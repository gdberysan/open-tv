package api_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/api"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
	"github.com/gdberysan/open-tv/internal/ui"
)

type repoVacio struct{}

func (repoVacio) Save(context.Context, domain.Channel) error        { return nil }
func (repoVacio) SaveBatch(context.Context, []domain.Channel) error { return nil }
func (repoVacio) FindByID(context.Context, domain.ChannelID) (domain.Channel, error) {
	return domain.Channel{}, nil
}
func (repoVacio) FindFiltered(context.Context, ports.ChannelFilter) ([]domain.Channel, error) {
	return nil, nil
}
func (repoVacio) Search(context.Context, string, int) ([]domain.Channel, error) { return nil, nil }
func (repoVacio) Delete(context.Context, domain.ChannelID) error                { return nil }
func (repoVacio) DeleteStale(context.Context, string, time.Time) (int64, error) { return 0, nil }
func (repoVacio) CountFiltered(context.Context, ports.ChannelFilter) (int, error) {
	return 0, nil
}
func (repoVacio) Random(context.Context, ports.ChannelFilter) (domain.Channel, error) {
	return domain.Channel{}, errors.New("vacío")
}

func (repoVacio) Countries(context.Context) ([]ports.Faceta, error)  { return nil, nil }
func (repoVacio) Categories(context.Context) ([]ports.Faceta, error) { return nil, nil }
func (repoVacio) Qualities(context.Context) ([]ports.Faceta, error)  { return nil, nil }

type provVacio struct{}

func (provVacio) ID() string                        { return "test" }
func (provVacio) Type() domain.ProviderType         { return domain.ProviderOpenSource }
func (provVacio) HealthCheck(context.Context) error { return nil }
func (provVacio) GetLiveChannels(context.Context) ([]domain.Channel, error) {
	return nil, nil
}

// Una caché de provider vacía da error, no cadena vacía: devolver ("", nil)
// haría que el handler sirviera una URL vacía como si fuese válida.
func (provVacio) GetStreamURL(context.Context, domain.ChannelID) (string, error) {
	return "", errors.New("caché vacía")
}

type streamsVacio struct{}

func (streamsVacio) Save(context.Context, domain.Stream) error        { return nil }
func (streamsVacio) SaveBatch(context.Context, []domain.Stream) error { return nil }
func (streamsVacio) FindAll(context.Context) ([]domain.Stream, error) { return nil, nil }
func (streamsVacio) FindByChannelID(context.Context, domain.ChannelID) ([]domain.Stream, error) {
	return nil, nil
}
func (streamsVacio) FindBestByChannelID(context.Context, domain.ChannelID) (domain.Stream, error) {
	return domain.Stream{}, sql.ErrNoRows
}
func (streamsVacio) FindMirrorsByChannelID(context.Context, domain.ChannelID) ([]ports.MirrorHealth, error) {
	return nil, nil
}
func (streamsVacio) MarkAlive(context.Context, string, int64) error        { return nil }
func (streamsVacio) MarkDead(context.Context, string) error                { return nil }
func (streamsVacio) MarkBatch(context.Context, []ports.StreamHealth) error { return nil }

type syncVacio struct{}

func (syncVacio) LastSuccess() time.Time { return time.Time{} }

// Este test usa el router REAL, no una tabla de rutas duplicada en el test: si
// el helper de los tests de handlers construye su propia tabla, puede derivar
// de la de producción sin que nadie se entere.
func TestTablaDeRutas(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{},
		streamsVacio{}, sqlDB, syncVacio{}, api.Options{})

	// El fallback SPA solo existe cuando internal/ui/dist tiene un cliente
	// construido (ui.Handler() ok=true): eso pasa en el job "Cliente web" de
	// CI (que construye web/ antes de este test) y en un checkout local
	// donde alguien ya corrió el build a mano, pero NO en un checkout limpio
	// ni en el job "Gateway (Go)" de CI, que nunca construye el cliente. Sin
	// esta comprobación el caso de /epg/now es no determinista: pasa o falla
	// según el estado del working tree, no según el código. Mismo patrón que
	// TestProxyApagadoNoRompeFallbackSPA en internal/ui/ui_test.go.
	_, hayClienteWeb := ui.Handler()
	epgNowQuiero := http.StatusNotFound
	if hayClienteWeb {
		epgNowQuiero = http.StatusOK
	}

	casos := []struct {
		ruta   string
		quiero int
	}{
		{"/health", http.StatusOK},
		{"/channels", http.StatusOK},
		// Rutas literales que conviven con /{id}/health: si chi las absorbiera
		// como parámetro, los selectores recibirían basura en vez de facetas.
		{"/channels/countries", http.StatusOK},
		{"/channels/categories", http.StatusOK},
		{"/channels/stream?id=x", http.StatusNotFound},
		{"/channels/loquesea/health", http.StatusNotFound},
		// La guía se retiró. Ya no es 404 de API: el cliente web (Tarea 9) se
		// monta como NotFound del router, así que cualquier ruta que ninguna
		// API reclame cae al fallback SPA (200 con el index) SI hay cliente
		// construido; si no, chi no tiene NotFound propio y responde 404. El
		// 404 de verdad para un fichero inexistente lo prueba internal/ui.
		{"/epg/now", epgNowQuiero},
	}
	for _, c := range casos {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, c.ruta, nil))
		if rr.Code != c.quiero {
			t.Errorf("%s = %d, quiero %d", c.ruta, rr.Code, c.quiero)
		}
	}
}

// El gateway local dejó de ser legible por webs de terceros. Sin este test,
// alguien reintroduce el middleware "porque el navegador se quejaba" y la
// regresión no se ve: todo sigue funcionando, solo que para todos.
func TestRouterNoAnunciaCORS(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, api.Options{})

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, quiero vacío", v)
	}
}

// La regla estructural: sin loopback no hay proxy. Si alguien la relaja, este
// test es el que lo dice.
//
// El caso proxyActivo=false exige un 404 EXPLÍCITO de /proxy/hls, no el que
// caiga en el fallback SPA del cliente web (Tarea 9). Antes de la Tarea 17
// este caso esperaba 200 (el índice del SPA) porque nada distinguía
// /proxy/hls de cualquier otra ruta desconocida del cliente: confuso, aunque
// no inseguro (nunca se sirve el relay). Ver TestProxyApagadoNoRompeFallbackSPA
// para la prueba de que el fallback SPA del resto de rutas sigue intacto.
func TestProxySoloExisteEnLoopback(t *testing.T) {
	for _, c := range []struct {
		activo bool
		quiero int
	}{
		{true, http.StatusBadRequest}, // montado: se queja de que falta u
		{false, http.StatusNotFound},  // apagado: 404 explícito, no el índice del SPA
	} {
		r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, api.Options{ProxyActivo: c.activo})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls", nil))
		if rec.Code != c.quiero {
			t.Errorf("proxyActivo=%v → %d, quiero %d", c.activo, rec.Code, c.quiero)
		}
	}
}

// Con el cliente web REAL montado (el caso realista: un binario con web/
// construido), el 404 de /proxy/hls tiene que ser específico de esa ruta:
// cualquier otra ruta desconocida del cliente (p.ej. /canal/x) sigue cayendo
// en el fallback SPA con 200. Si el fix se hiciera desmontando el fallback
// SPA entero cuando el proxy está apagado, este test lo detectaría.
//
// Si el binario de test no tiene web/ construido en internal/ui/dist (dist
// vacío salvo .gitkeep), ui.Handler() devuelve ok=false y este test se salta:
// ese caso no distingue nada porque tampoco hay fallback SPA que proteger. El
// gate manual/e2e de la Tarea 17 cubre ese escenario con el binario real.
func TestProxyApagadoNoRompeFallbackSPA(t *testing.T) {
	if _, ok := ui.Handler(); !ok {
		t.Skip("sin cliente web construido en internal/ui/dist: nada que distinguir")
	}

	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, api.Options{ProxyActivo: false})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls?u=x", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("/proxy/hls con proxy apagado = %d, quiero %d", rec.Code, http.StatusNotFound)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/canal/x", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/canal/x (ruta del cliente, no de la API) = %d, quiero %d (fallback SPA)", rec.Code, http.StatusOK)
	}
}
