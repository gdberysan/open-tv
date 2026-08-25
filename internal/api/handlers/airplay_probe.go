package handlers

import (
	"context"
	"io"
	"net/http"
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

	mu    sync.RWMutex
	cache map[string]entradaAirplay
}

type entradaAirplay struct {
	veredicto domain.AirplaySupport
	expira    time.Time
}

func NewAirplayProber(client *http.Client, ttl time.Duration, maxEntradas int) *AirplayProber {
	if client == nil {
		client = &http.Client{Timeout: presupuestoSondeo}
	}
	return &AirplayProber{
		client:      client,
		ttl:         ttl,
		maxEntradas: maxEntradas,
		cache:       make(map[string]entradaAirplay),
	}
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

func (p *AirplayProber) sondear(ctx context.Context, url string) domain.AirplaySupport {
	reqCtx, cancel := context.WithTimeout(ctx, presupuestoSondeo)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return domain.AirplayUnknown
	}
	req.Header.Set("User-Agent", userAgentSondeo)

	resp, err := p.client.Do(req)
	if err != nil {
		return domain.AirplayUnknown
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.AirplayUnknown
	}

	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxCuerpoManifiesto))
	if err != nil {
		return domain.AirplayUnknown
	}
	return domain.ClassifyManifest(url, string(cuerpo))
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
