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
	"syscall"
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
		_, n, _ := net.ParseCIDR(cidr)
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

// Handler relaya HLS al navegador de la misma máquina.
type Handler struct {
	client     *http.Client
	prefijo    string
	privadasOK bool
}

// NewHandler construye el proxy. prefijo es la ruta con la que se reescriben
// las URIs del manifiesto, normalmente "/proxy/hls?u=".
//
// permitirDestinosPrivados solo es true en tests: los servidores de httptest
// viven en 127.0.0.1, que en producción es exactamente lo que hay que
// bloquear. En el router se monta siempre con false.
func NewHandler(prefijo string, permitirDestinosPrivados bool) *Handler {
	h := &Handler{
		prefijo:    prefijo,
		privadasOK: permitirDestinosPrivados,
	}

	dialer := &net.Dialer{
		Timeout: tiempoPeticion,
		// Control se ejecuta con la IP YA resuelta, justo antes de que el
		// kernel abra la conexión — no la que destinoPrivado comprobó antes
		// de que el propio Transport volviera a resolver el hostname por su
		// cuenta. Sin esto, un DNS que cambie de respuesta entre esa
		// comprobación y la conexión real (rebinding) esquiva el filtro por
		// hostname sin que el proxy se entere.
		Control: h.controlConexion,
	}

	h.client = &http.Client{
		// Sin timeout de cliente: lo pone el contexto por petición, que
		// además cancela la copia en curso si el navegador cierra.
		Transport: &http.Transport{
			MaxConnsPerHost:   4,
			DisableKeepAlives: true,
			DialContext:       dialer.DialContext,
		},
		// El http.Client por defecto sigue redirecciones (hasta 10) sin
		// preguntar. Un origen que en principio pasó el filtro puede
		// responder 302 hacia 127.0.0.1 o hacia el enlace-local de metadatos
		// de una nube, y sin esto el proxy lo seguiría y relayaría la
		// respuesta interna al navegador.
		CheckRedirect: h.checkRedirect,
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, destino.String(), nil)
	if err != nil {
		http.Error(w, "no se pudo construir la petición", http.StatusBadGateway)
		return
	}
	req.Header.Set("User-Agent", userAgent)
	// Range sí viaja: EXT-X-BYTERANGE hace que el reproductor pida trozos.
	if rango := r.Header.Get("Range"); rango != "" {
		req.Header.Set("Range", rango)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "el origen no respondió", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

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

// checkRedirect se ejecuta en cada salto de una redirección 3xx, ANTES de que
// el cliente la siga.
func (h *Handler) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirecciones {
		return fmt.Errorf("demasiadas redirecciones (%d)", len(via))
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("esquema no permitido en redirección: %s", req.URL.Scheme)
	}
	if !h.privadasOK && destinoPrivado(req.Context(), req.URL.Hostname()) {
		return fmt.Errorf("redirección a destino no permitido: %s", req.URL.Hostname())
	}
	return nil
}

// controlConexion se ejecuta justo antes de que el sistema operativo abra la
// conexión TCP, con la dirección YA resuelta. Es la única comprobación que
// mira la IP con la que el kernel conecta de verdad, así que es la que de
// verdad cierra el DNS-rebinding: destinoPrivado (arriba) y checkRedirect
// comprueban el hostname en un instante anterior, y nada les garantiza que el
// Transport resuelva ese mismo hostname a la misma IP al conectar.
func (h *Handler) controlConexion(_, address string, _ syscall.RawConn) error {
	if h.privadasOK {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("dirección de conexión inválida: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("dirección de conexión no es una IP: %s", host)
	}
	if ipPrivada(ip) {
		return fmt.Errorf("conexión bloqueada a destino privado: %s", ip)
	}
	return nil
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
