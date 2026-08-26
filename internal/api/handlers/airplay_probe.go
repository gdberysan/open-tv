package handlers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

// presupuestoSondeo acota lo que el sondeo puede retrasar la respuesta. Si no
// da tiempo, se devuelve desconocido: la URL de reproducción nunca espera por
// un dato accesorio.
const presupuestoSondeo = time.Second

// maxCuerpoManifiesto limita la lectura. Un master playlist son unos cientos de
// bytes; leer más solo sirve para que un origen roto agote memoria.
const maxCuerpoManifiesto = 64 << 10

// userAgentSondeo identifica al gateway como un reproductor. Varios orígenes
// contestan 403 al default de Go, igual que le pasa al health-checker.
const userAgentSondeo = "VLC/3.0.20 LibVLC/3.0.20"

// maxRedireccionesSondeo: un manifiesto no necesita más de un par de saltos.
// Mismo tope que internal/proxy/handler.go.
const maxRedireccionesSondeo = 5

// AirplayProber resuelve la compatibilidad AirPlay de una URL bajo demanda y la
// recuerda en memoria.
//
// En memoria y no en SQLite a propósito: los handlers reciben un pool de solo
// lectura (cmd/server/main.go), separado del de escritura. Persistir desde aquí
// rompería esa separación por un dato que se recalcula en un segundo.
type AirplayProber struct {
	client      *http.Client
	ttl         time.Duration
	maxEntradas int
	privadasOK  bool

	mu    sync.RWMutex
	cache map[string]entradaAirplay
}

type entradaAirplay struct {
	veredicto domain.AirplaySupport
	expira    time.Time
}

// NewAirplayProber construye el sondeador.
//
// permitirDestinosPrivados solo es true en tests: los servidores de httptest
// viven en 127.0.0.1, que en producción es exactamente lo que hay que
// bloquear, porque url llega desde streams sincronizados de proveedores
// IPTV externos (ver channel_handler.go: resolveStreamURL) y no de nada que
// el usuario tecleé a mano. En el router se monta siempre con false.
func NewAirplayProber(client *http.Client, ttl time.Duration, maxEntradas int, permitirDestinosPrivados bool) *AirplayProber {
	p := &AirplayProber{
		ttl:         ttl,
		maxEntradas: maxEntradas,
		privadasOK:  permitirDestinosPrivados,
		cache:       make(map[string]entradaAirplay),
	}
	if client == nil {
		// CheckRedirect es imprescindible aquí: el http.Client por defecto
		// sigue hasta 10 redirecciones sin preguntar, y destinoPrivado (en
		// sondear) solo valida la URL de partida. Un origen que pase ese
		// primer filtro puede responder 302 hacia 127.0.0.1 o hacia el
		// enlace-local de metadatos de una nube; sin esto el sondeo lo
		// seguiría igual que si fuera un destino válido. Mismo criterio que
		// internal/proxy/handler.go: checkRedirect.
		client = &http.Client{
			Timeout:       presupuestoSondeo,
			CheckRedirect: p.checkRedirect,
		}
	}
	p.client = client
	return p
}

// checkRedirect se ejecuta en cada salto de una redirección 3xx, ANTES de
// que el cliente la siga. Solo se instala en el cliente por defecto (ver
// NewAirplayProber): si el caller inyecta su propio *http.Client (como
// hacen los tests), las redirecciones de ESE cliente no pasan por aquí.
func (p *AirplayProber) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedireccionesSondeo {
		return fmt.Errorf("demasiadas redirecciones (%d)", len(via))
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("esquema no permitido en redirección: %s", req.URL.Scheme)
	}
	if !p.privadasOK && destinoPrivado(req.Context(), req.URL.Hostname()) {
		return fmt.Errorf("redirección a destino no permitido: %s", req.URL.Hostname())
	}
	return nil
}

// Veredicto nunca devuelve error: un sondeo que falla es AirplayUnknown, que es
// exactamente lo que significa "no se pudo determinar".
func (p *AirplayProber) Veredicto(ctx context.Context, url string) domain.AirplaySupport {
	// Lo que la URL ya descarta no necesita red.
	if v := domain.ClassifyManifest(url, ""); v == domain.AirplayNo {
		return v
	}
	if v, ok := p.leerCache(url); ok {
		return v
	}

	v := p.sondear(ctx, url)
	p.guardar(url, v)
	return v
}

func (p *AirplayProber) sondear(ctx context.Context, destino string) domain.AirplaySupport {
	reqCtx, cancel := context.WithTimeout(ctx, presupuestoSondeo)
	defer cancel()

	// destino sale de streams sincronizados de proveedores IPTV externos
	// (resolveStreamURL), así que se valida como cualquier destino de red
	// ajeno antes de pedirlo: mismo criterio que el proxy HLS
	// (internal/proxy/handler.go).
	u, err := url.Parse(destino)
	if err != nil {
		return domain.AirplayUnknown
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return domain.AirplayUnknown
	}
	if !p.privadasOK && destinoPrivado(reqCtx, u.Hostname()) {
		return domain.AirplayUnknown
	}

	// #nosec G704 -- destino ya pasó la comprobación de esquema y de destino
	// privado justo arriba; el análisis de taint de gosec no ve esa
	// validación, ni tampoco checkRedirect (que repite el mismo filtro en
	// cada salto cuando el cliente es el que construye NewAirplayProber por
	// defecto).
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, destino, nil)
	if err != nil {
		return domain.AirplayUnknown
	}
	req.Header.Set("User-Agent", userAgentSondeo)

	resp, err := p.client.Do(req) // #nosec G704 -- misma petición ya filtrada, ver comentario arriba
	if err != nil {
		return domain.AirplayUnknown
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.AirplayUnknown
	}

	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxCuerpoManifiesto))
	if err != nil {
		return domain.AirplayUnknown
	}
	return domain.ClassifyManifest(destino, string(cuerpo))
}

// destinoPrivado bloquea loopback, red privada, link-local y direcciones sin
// especificar. Mismo criterio que internal/proxy/handler.go: este sondeo no
// tiene la protección de dial-time contra DNS-rebinding que sí tiene el
// proxy (aquí p.client puede venir de fuera, en tests), así que esta
// comprobación por hostname es la única línea de defensa — motivo de más
// para no relajarla.
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

// redesExtraPrivadas: rangos que net.IP.IsPrivate() no cubre pero que
// tampoco deben alcanzarse — CGNAT (Tailscale y muchos ISP) y el rango de
// benchmark. Igual que internal/proxy/handler.go.
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

func (p *AirplayProber) leerCache(url string) (domain.AirplaySupport, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.cache[url]
	if !ok || time.Now().After(e.expira) {
		return domain.AirplayUnknown, false
	}
	return e.veredicto, true
}

func (p *AirplayProber) guardar(url string, v domain.AirplaySupport) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Al llegar al tope se vacía entera en vez de desalojar por antigüedad: son
	// unos cientos de entradas de texto y una LRU aquí sería complejidad sin
	// beneficio medible.
	if len(p.cache) >= p.maxEntradas {
		p.cache = make(map[string]entradaAirplay, p.maxEntradas)
	}
	p.cache[url] = entradaAirplay{veredicto: v, expira: time.Now().Add(p.ttl)}
}

// entradas expone el tamaño de la caché. Solo para tests.
func (p *AirplayProber) entradas() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.cache)
}

// aBool traduce el veredicto al JSON del cable: null cuando no se sabe.
func aBool(v domain.AirplaySupport) *bool {
	switch v {
	case domain.AirplayOK:
		t := true
		return &t
	case domain.AirplayNo:
		f := false
		return &f
	default:
		return nil
	}
}
