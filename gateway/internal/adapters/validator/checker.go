package validator

import (
	"context"
	"fmt"
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

// Checker se encarga de validar una única URL.
type Checker struct {
	client  HTTPChecker
	timeout time.Duration
}

// NewChecker instancia un Checker con la configuración dada.
func NewChecker(client HTTPChecker, timeout time.Duration) *Checker {
	if client == nil {
		client = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxConnsPerHost: maxConnsPerHost,
			},
		}
	}
	return &Checker{
		client:  client,
		timeout: timeout,
	}
}

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

		respGet, errGet := c.client.Do(reqGet)
		if errGet != nil {
			result.Error = fmt.Errorf("error en GET request: %w", errGet)
			return result
		}
		defer respGet.Body.Close()

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
