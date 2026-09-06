package validator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/proxy"
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

// maxConnsPorHostSonda acota, POR SEPARADO, el cliente de la sonda de
// códecs (manifiesto intermedio + segmento). El checker de manifiestos ya
// gasta hasta maxConnsPerHost (4) contra ese mismo host; si la sonda usara
// el mismo tope, un host con muchos streams podría ver hasta 4+4=8
// conexiones simultáneas, duplicando el presupuesto que existe justo para
// evitar los falsos muertos de nginx. El presupuesto total por host queda
// en 4+2=6.
const maxConnsPorHostSonda = 2

// userAgentPorDefecto identifica al checker como un reproductor. Algunos
// orígenes filtran el default de Go (Go-http-client/2.0) con un 403. Se usa
// cuando el stream no exige un User-Agent propio.
const userAgentPorDefecto = "VLC/3.0.20 LibVLC/3.0.20"

// Checker se encarga de validar una única URL.
type Checker struct {
	client    HTTPChecker
	transport *http.Transport
	timeout   time.Duration
	// sonda es el cliente GUARDADO (proxy.NuevoClienteGuardado) con el que se
	// piden las URLs que dicta un manifiesto de terceros: media playlist y
	// segmento. El manifiesto en sí sigue yendo por client, como siempre.
	sonda HTTPChecker
	// transporteSonda es el *http.Transport del cliente por defecto de la
	// sonda (nil si se inyectó un HTTPChecker desde fuera). Solo para tests.
	transporteSonda *http.Transport
}

// NewChecker instancia un Checker con la configuración dada.
func NewChecker(client HTTPChecker, timeout time.Duration) *Checker {
	return NewCheckerConSonda(client, nil, timeout)
}

// NewCheckerConSonda deja inyectar el cliente de la sonda. nil = el guardado
// de producción (bloquea destinos privados).
func NewCheckerConSonda(client, sonda HTTPChecker, timeout time.Duration) *Checker {
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
	var trSonda *http.Transport
	if sonda == nil {
		clienteSonda := proxy.NuevoClienteGuardadoConTope(false, maxConnsPorHostSonda)
		trSonda, _ = clienteSonda.Transport.(*http.Transport)
		sonda = clienteSonda
	}
	return &Checker{client: client, transport: tr, timeout: timeout, sonda: sonda, transporteSonda: trSonda}
}

// Transport expone el transporte propio del checker, o nil si se le inyectó un
// cliente desde fuera. Solo para tests.
func (c *Checker) Transport() *http.Transport { return c.transport }

// TransporteSonda expone el transporte propio del cliente de la sonda de
// códecs, o nil si se le inyectó un HTTPChecker desde fuera. Solo para tests.
func (c *Checker) TransporteSonda() *http.Transport { return c.transporteSonda }

// Check valida una URL sin cabeceras propias. Envoltorio compatible para los
// call sites (y tests) que no tienen Referrer/UserAgent a mano.
func (c *Checker) Check(ctx context.Context, url string) StreamResult {
	return c.CheckConCabeceras(ctx, url, "", "")
}

// CheckConCabeceras valida una URL, mandando el Referer/User-Agent que su
// origen exige cuando los tiene (referrer/userAgent vacíos = usar los de
// siempre). Sin ellos, un origen con protección de hotlink responde error
// aunque el stream funcione perfectamente. Para HLS va directo al GET: el
// HEAD no trae el manifiesto, y sin manifiesto no hay veredicto de
// compatibilidad. Es una petición en lugar de dos, no una más. El resto
// conserva HEAD→GET. Nunca sondea códecs: eso es CheckTarea.
func (c *Checker) CheckConCabeceras(ctx context.Context, url, referrer, userAgentStream string) StreamResult {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, _, _ := c.comprobar(reqCtx, url, referrer, userAgentStream)
	return res
}

// CheckTarea es lo que ejecuta el pool: el chequeo de siempre y, si la
// tarea lo pide y el manifiesto está vivo, la sonda de códecs — dentro del
// MISMO timeout, para que la pasada no se alargue.
func (c *Checker) CheckTarea(ctx context.Context, t TareaCheck) StreamResult {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, cuerpo, final := c.comprobar(reqCtx, t.URL, t.Referrer, t.UserAgent)
	if t.CodecCaducado && res.IsAlive && res.Protocol == "HLS" && cuerpo != "" {
		res.CodecSondeado = true
		res.Codec, res.Codecs = c.sondearCodecs(reqCtx, final, cuerpo, t.Referrer, t.UserAgent)
	}
	return res
}

// comprobar hace el chequeo HEAD→GET (o GET directo para HLS) sobre reqCtx,
// que YA trae el timeout puesto: lo recibe en vez de crearlo, porque
// CheckTarea necesita compartirlo con la sonda. Devuelve además el cuerpo
// del manifiesto y su URL final cuando hubo GET (cadenas vacías si solo
// hubo HEAD, o si algo falló antes de llegar a clasificar).
func (c *Checker) comprobar(reqCtx context.Context, url, referrer, userAgentStream string) (result StreamResult, cuerpoManifiesto, urlManifiesto string) {
	start := time.Now()

	ua := userAgentPorDefecto
	if userAgentStream != "" {
		ua = userAgentStream
	}

	result = StreamResult{
		URL:      url,
		Protocol: inferProtocol(url),
	}

	needsGetFallback := result.Protocol == "HLS"

	if !needsGetFallback {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, url, nil)
		if err != nil {
			result.Error = fmt.Errorf("creando HEAD request: %w", err)
			return result, "", ""
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Origin", domain.OrigenWeb)
		if referrer != "" {
			req.Header.Set("Referer", referrer)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			result.Error = fmt.Errorf("error en HEAD request: %w", err)
			needsGetFallback = true
		} else {
			defer func() { _ = resp.Body.Close() }()
			result.StatusCode = resp.StatusCode
			if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode >= 400 {
				needsGetFallback = true
			} else {
				result.IsAlive = resp.StatusCode >= 200 && resp.StatusCode < 300
				// Sin cuerpo, pero con esquema final y CORS ya se decide todo lo
				// que un .ts o un .mp4 necesitan.
				result.Web = domain.ClassifyWeb(urlFinal(resp, url),
					resp.Header.Get("Access-Control-Allow-Origin"), "")
			}
		}
	}

	if needsGetFallback {
		result.Error = nil
		reqGet, errGet := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if errGet != nil {
			result.Error = fmt.Errorf("creando GET request: %w", errGet)
			return result, "", ""
		}
		reqGet.Header.Set("User-Agent", ua)
		// Origin va a propósito: los orígenes que reflejan el origen del
		// solicitante solo contestan un ACAO si se les manda uno.
		reqGet.Header.Set("Origin", domain.OrigenWeb)
		if referrer != "" {
			reqGet.Header.Set("Referer", referrer)
		}

		respGet, errGet := c.client.Do(reqGet)
		if errGet != nil {
			result.Error = fmt.Errorf("error en GET request: %w", errGet)
			return result, "", ""
		}
		defer func() { _ = respGet.Body.Close() }()
		result.StatusCode = respGet.StatusCode

		// Drenar el cuerpo es obligatorio: sin leerlo, la conexión queda
		// inutilizable y el servidor la ve abortada a media respuesta. Ya que
		// hay que leerlo, se clasifica dos veces en vez de tirarlo — cero
		// peticiones extra por dos veredictos de compatibilidad.
		cuerpo, errLectura := io.ReadAll(io.LimitReader(respGet.Body, 64<<10))
		if errLectura != nil {
			result.Error = fmt.Errorf("leyendo cuerpo del GET: %w", errLectura)
			return result, "", ""
		}
		final := urlFinal(respGet, url)
		result.Airplay = domain.ClassifyManifest(final, string(cuerpo))
		result.Web = domain.ClassifyWeb(final,
			respGet.Header.Get("Access-Control-Allow-Origin"), string(cuerpo))

		result.IsAlive = respGet.StatusCode >= 200 && respGet.StatusCode < 300

		result.LatencyMs = time.Since(start).Milliseconds()
		return result, string(cuerpo), final
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	return result, "", ""
}

// urlFinal devuelve la URL tras las redirecciones. Importa: un http:// que
// redirige a https:// SÍ es reproducible desde una página segura, y juzgarlo
// por la URL de partida lo descartaría sin motivo.
func urlFinal(resp *http.Response, porDefecto string) string {
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return porDefecto
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
