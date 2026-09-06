package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxSegmentoBytes acota lo que el proxy relaya de una sola pieza. Un segmento
// HLS de 6 s son unos pocos MB; 50 MB es holgado y a la vez impide que un
// origen roto —o un "segmento" que en realidad es un fichero de horas— use el
// proxy como descargador infinito.
const MaxSegmentoBytes int64 = 50 << 20

// Redes que net.IP.IsPrivate() no cubre pero que tampoco deben alcanzarse por
// el relay: CGNAT (Tailscale y muchos ISP) y el rango de benchmark.
var redesExtraPrivadas = func() []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range []string{"100.64.0.0/10", "198.18.0.0/15"} {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("CIDR inválido %q: %v", cidr, err))
		}
		nets = append(nets, n)
	}
	return nets
}()

// maxManifiestoBytes: un manifiesto de miles de segmentos ronda los cientos de
// KB. Se lee entero porque hay que reescribirlo.
const maxManifiestoBytes int64 = 5 << 20

// tiempoPeticion acota cada relay. Un stream en vivo que no manda un segmento
// en 30 s está roto.
const tiempoPeticion = 30 * time.Second

// userAgent: varios orígenes contestan 403 al default de Go, igual que le pasa
// al health-checker.
const userAgent = "VLC/3.0.20 LibVLC/3.0.20"

// maxRedirecciones: un manifiesto o un segmento no necesitan más de un par de
// saltos. Un origen que redirige en bucle, o que encadena saltos para llegar a
// donde el filtro de destinos no le deja, se corta aquí.
const maxRedirecciones = 5

// BuscadorCabeceras resuelve las cabeceras que el ORIGEN exige para una URL.
// Devuelve vacías si no hay ninguna o si la URL no está en el catálogo. Nunca
// devuelve error: un fallo de lectura se traduce en "usa las de siempre", que
// es el comportamiento anterior y siempre es seguro.
type BuscadorCabeceras func(ctx context.Context, url string) (referrer, userAgent string)

// Option configura parámetros opcionales del Handler, igual que hace
// opensource.Option con su Provider.
type Option func(*Handler)

// ConBuscadorCabeceras conecta la búsqueda de cabeceras por stream. Sin ella el
// proxy se comporta como siempre: User-Agent fijo y ningún Referer.
func ConBuscadorCabeceras(b BuscadorCabeceras) Option {
	return func(h *Handler) { h.cabeceras = b }
}

// Handler relaya HLS al navegador de la misma máquina.
type Handler struct {
	client     *http.Client
	prefijo    string
	privadasOK bool
	// cabeceras resuelve el Referer/User-Agent que el origen exige, cuando el
	// catálogo lo sabe. nil (el zero value, o cualquier caso donde devuelva
	// cadenas vacías) cae al User-Agent fijo de siempre.
	cabeceras BuscadorCabeceras
}

// NewHandler construye el proxy. prefijo es la ruta con la que se reescriben
// las URIs del manifiesto, normalmente "/proxy/hls?u=".
//
// permitirDestinosPrivados solo es true en tests: los servidores de httptest
// viven en 127.0.0.1, que en producción es exactamente lo que hay que
// bloquear. En el router se monta siempre con false.
func NewHandler(prefijo string, permitirDestinosPrivados bool, opts ...Option) *Handler {
	h := &Handler{
		prefijo:    prefijo,
		privadasOK: permitirDestinosPrivados,
	}

	h.client = NuevoClienteGuardado(permitirDestinosPrivados)

	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	crudo := r.URL.Query().Get("u")
	if crudo == "" {
		http.Error(w, "falta el parámetro u", http.StatusBadRequest)
		return
	}
	destino, err := url.Parse(crudo)
	if err != nil {
		http.Error(w, "u no es una URL", http.StatusBadRequest)
		return
	}
	if destino.Scheme != "http" && destino.Scheme != "https" {
		http.Error(w, "esquema no permitido", http.StatusForbidden)
		return
	}

	// Sec-Fetch-Site lo manda el navegador y el atacante no puede falsificarlo
	// desde JS. Si viene y no es same-origin/none, la petición la disparó otro
	// sitio (una página maliciosa cargando esta URL de proxy), y el relay no
	// tiene por qué obedecerla aunque el Host ya haya pasado el filtro de
	// MismoOrigen (ese filtro no ve de qué origen viene el fetch).
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		http.Error(w, "origen cruzado no permitido", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tiempoPeticion)
	defer cancel()

	if !h.privadasOK && destinoPrivado(ctx, destino.Hostname()) {
		http.Error(w, "destino no permitido", http.StatusForbidden)
		return
	}

	// Petición NUEVA, no un reenvío: las cabeceras del cliente (Origin,
	// Referer, Cookie) no tienen por qué viajar a un tercero.
	//
	// #nosec G704 -- destino ya pasó destinoPrivado (arriba) y el esquema está
	// restringido a http/https; el análisis de taint de gosec no ve esas
	// comprobaciones ni tampoco controlConexion/checkRedirect, que cierran
	// redirecciones y DNS-rebinding más abajo.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, destino.String(), nil)
	if err != nil {
		http.Error(w, "no se pudo construir la petición", http.StatusBadGateway)
		return
	}
	// Cabeceras del ORIGEN, no del cliente: las del navegador siguen sin
	// viajar. Si el stream no declara ninguna, se usan las de siempre.
	ua := userAgent
	if h.cabeceras != nil {
		ref, uaStream := h.cabeceras(ctx, destino.String())
		if uaStream != "" {
			ua = uaStream
		}
		if ref != "" {
			req.Header.Set("Referer", ref)
		}
	}
	req.Header.Set("User-Agent", ua)
	// Range sí viaja: EXT-X-BYTERANGE hace que el reproductor pida trozos.
	if rango := r.Header.Get("Range"); rango != "" {
		req.Header.Set("Range", rango)
	}

	resp, err := h.client.Do(req) // #nosec G704 -- misma petición ya filtrada, ver comentario arriba
	if err != nil {
		http.Error(w, "el origen no respondió", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// La URL FINAL: si hubo redirección, las relativas del manifiesto se
	// resuelven contra el sitio al que se llegó, no contra el que se pidió,
	// y la extensión que decide si esto es un manifiesto se mira ahí también.
	urlFinal := destino
	if resp.Request != nil && resp.Request.URL != nil {
		urlFinal = resp.Request.URL
	}

	if esManifiesto(urlFinal, resp.Header.Get("Content-Type")) {
		h.relayarManifiesto(w, resp, urlFinal)
		return
	}
	h.relayarBytes(w, resp, urlFinal)
}

func (h *Handler) relayarManifiesto(w http.ResponseWriter, resp *http.Response, base *url.URL) {
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxManifiestoBytes))
	if err != nil {
		http.Error(w, "manifiesto ilegible", http.StatusBadGateway)
		return
	}
	if int64(len(cuerpo)) == maxManifiestoBytes {
		slog.Warn("proxy: manifiesto truncado al alcanzar el tope",
			"url", base.String(), "tope_bytes", maxManifiestoBytes)
	}
	salida := ReescribirManifiesto(base, string(cuerpo), h.prefijo)

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.WriteString(w, salida)
}

func (h *Handler) relayarBytes(w http.ResponseWriter, resp *http.Response, destino *url.URL) {
	// Solo las cabeceras que el reproductor necesita. Nada de copiar el juego
	// entero: ahí viajan Set-Cookie y el ACAO del origen, y la UI es del mismo
	// origen y no quiere ninguno de los dos.
	for _, k := range []string{"Content-Type", "Content-Range", "Accept-Ranges", "Content-Length"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	// El contexto de la petición cancela esta copia si el navegador se va.
	n, _ := io.Copy(w, io.LimitReader(resp.Body, MaxSegmentoBytes))
	if n == MaxSegmentoBytes {
		slog.Warn("proxy: segmento truncado al alcanzar el tope",
			"url", destino.String(), "tope_bytes", MaxSegmentoBytes)
	}
}

// esManifiesto mira el tipo de contenido y, si el origen no lo declara bien
// —cosa habitual—, la extensión.
func esManifiesto(u *url.URL, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "mpegurl") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".m3u8")
}

// destinoPrivado bloquea loopback, red privada, link-local y direcciones sin
// especificar. El proxy solo escucha en loopback, así que solo lo alcanza esta
// máquina — pero eso no lo convierte en un pasadizo hacia el router de casa o
// hacia los metadatos de una nube.
func destinoPrivado(ctx context.Context, host string) bool {
	if host == "" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ipPrivada(ip)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return true
	}
	for _, a := range ips {
		if ipPrivada(a.IP) {
			return true
		}
	}
	return false
}

func ipPrivada(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, n := range redesExtraPrivadas {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
