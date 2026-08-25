package proxy

import (
	"context"
	"io"
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

// maxManifiestoBytes: un manifiesto de miles de segmentos ronda los cientos de
// KB. Se lee entero porque hay que reescribirlo.
const maxManifiestoBytes int64 = 5 << 20

// tiempoPeticion acota cada relay. Un stream en vivo que no manda un segmento
// en 30 s está roto.
const tiempoPeticion = 30 * time.Second

// userAgent: varios orígenes contestan 403 al default de Go, igual que le pasa
// al health-checker.
const userAgent = "VLC/3.0.20 LibVLC/3.0.20"

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
	return &Handler{
		prefijo:    prefijo,
		privadasOK: permitirDestinosPrivados,
		client: &http.Client{
			// Sin timeout de cliente: lo pone el contexto por petición, que
			// además cancela la copia en curso si el navegador cierra.
			Transport: &http.Transport{
				MaxConnsPerHost:   4,
				DisableKeepAlives: true,
			},
		},
	}
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

	if esManifiesto(destino, resp.Header.Get("Content-Type")) {
		h.relayarManifiesto(w, resp, destino)
		return
	}
	h.relayarBytes(w, resp)
}

func (h *Handler) relayarManifiesto(w http.ResponseWriter, resp *http.Response, destino *url.URL) {
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxManifiestoBytes))
	if err != nil {
		http.Error(w, "manifiesto ilegible", http.StatusBadGateway)
		return
	}
	// La base es la URL FINAL: si hubo redirección, las relativas se resuelven
	// contra el sitio al que se llegó, no contra el que se pidió.
	base := destino
	if resp.Request != nil && resp.Request.URL != nil {
		base = resp.Request.URL
	}
	salida := ReescribirManifiesto(base, string(cuerpo), h.prefijo)

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.WriteString(w, salida)
}

func (h *Handler) relayarBytes(w http.ResponseWriter, resp *http.Response) {
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
	_, _ = io.Copy(w, io.LimitReader(resp.Body, MaxSegmentoBytes))
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
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
