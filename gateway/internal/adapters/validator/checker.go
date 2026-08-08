package validator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPChecker define la interfaz para realizar peticiones HTTP, útil para testing.
type HTTPChecker interface {
	Do(req *http.Request) (*http.Response, error)
}

// maxConnsPerHost acota las conexiones simultáneas al mismo servidor.
// Hosts como jmp2.uk alojan 1400+ streams: sin límite, el pool de 50
// workers dispara los limit_conn de nginx y el servidor rechaza conexiones
// tanto al checker (falsos muertos) como al usuario reproduciendo.
const maxConnsPerHost = 4

// userAgent identifica al checker como un reproductor. Algunos orígenes
// filtran el default de Go (Go-http-client/2.0) con un 403.
const userAgent = "VLC/3.0.20 LibVLC/3.0.20"

// Checker se encarga de validar una única URL.
type Checker struct {
	client    HTTPChecker
	transport *http.Transport
	timeout   time.Duration
}

// NewChecker instancia un Checker con la configuración dada.
func NewChecker(client HTTPChecker, timeout time.Duration) *Checker {
	var tr *http.Transport
	if client == nil {
		// DisableKeepAlives: los orígenes IPTV rotos (MistServer sobre todo)
		// responden a un HEAD escribiendo cuerpo igualmente, o escriben una
		// segunda respuesta completa en el mismo socket. Si esa conexión vuelve
		// al pool de inactivas con bytes pendientes, el transporte lo detecta al
		// siguiente peek y lo registra con el paquete log GLOBAL, fuera de slog:
		// de ahí las líneas "Unsolicited response received on idle HTTP channel"
		// sueltas entre el JSON. Sin pool de inactivas, ese camino no existe.
		tr = &http.Transport{
			MaxConnsPerHost:   maxConnsPerHost,
			DisableKeepAlives: true,
		}
		client = &http.Client{Timeout: timeout, Transport: tr}
	}
	return &Checker{
		client:    client,
		transport: tr,
		timeout:   timeout,
	}
}

// Transport expone el transporte propio del checker, o nil si se le inyectó un
// cliente desde fuera. Solo para tests.
func (c *Checker) Transport() *http.Transport { return c.transport }

// Check valida una URL usando una estrategia HTTP HEAD con fallback a HTTP GET.
func (c *Checker) Check(ctx context.Context, url string) StreamResult {
	start := time.Now()

	// 1. Contexto con timeout para evitar goroutine leaks (Riesgo #6 mitigado)
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result := StreamResult{
		URL:      url,
		Protocol: inferProtocol(url),
	}

	// Intento 1: HTTP HEAD
	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, url, nil)
	if err != nil {
		result.Error = fmt.Errorf("creando HEAD request: %w", err)
		return result
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := c.client.Do(req)

	needsGetFallback := false
	if err != nil {
		// Evaluamos si el error es de timeout u otra cosa
		result.Error = fmt.Errorf("error en HEAD request: %w", err)
		// En algunos casos, se prefiere fallback ante cualquier error que no sea timeout, pero
		// para evitar ahogar servidores, limitamos el fallback a errores específicos de método si tenemos resp.
	} else {
		defer resp.Body.Close()
		// Si el servidor responde 405 Method Not Allowed u otros errores, intentamos con GET
		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode >= 400 {
			needsGetFallback = true
		} else {
			result.IsAlive = resp.StatusCode >= 200 && resp.StatusCode < 300
		}
	}

	// Intento 2: Fallback a HTTP GET si HEAD falla explícitamente o es rechazado
	if err != nil || needsGetFallback {
		// Limpiamos el error previo
		result.Error = nil
		reqGet, errGet := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if errGet != nil {
			result.Error = fmt.Errorf("creando GET request: %w", errGet)
			return result
		}

		reqGet.Header.Set("User-Agent", userAgent)

		respGet, errGet := c.client.Do(reqGet)
		if errGet != nil {
			result.Error = fmt.Errorf("error en GET request: %w", errGet)
			return result
		}
		defer respGet.Body.Close()
		// Drenar un poco del cuerpo: sin leerlo, la conexión queda inutilizable
		// y el servidor la ve abortada a media respuesta.
		_, _ = io.Copy(io.Discard, io.LimitReader(respGet.Body, 64<<10))

		result.IsAlive = respGet.StatusCode >= 200 && respGet.StatusCode < 300
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	return result
}

// inferProtocol adivina el protocolo según la extensión de la URL
func inferProtocol(url string) string {
	lowerURL := strings.ToLower(url)
	if strings.Contains(lowerURL, ".m3u8") {
		return "HLS"
	} else if strings.Contains(lowerURL, ".mpd") {
		return "DASH"
	} else if strings.HasPrefix(lowerURL, "rtmp://") {
		return "RTMP"
	}
	return "UNKNOWN"
}
