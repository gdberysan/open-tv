package api_test

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/acceso"
	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/adapters/providers/iptvorg"
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

func (provVacio) GetStreamsDeCanal(context.Context, domain.ChannelID) ([]iptvorg.StreamExtra, error) {
	return nil, errors.New("caché vacía")
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
func (streamsVacio) DeleteStale(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (streamsVacio) ImagenPorCanal(context.Context, time.Time) ([]ports.ImagenCanal, error) {
	return nil, nil
}
func (streamsVacio) CabecerasPorURL(context.Context, string) (string, string, error) {
	return "", "", nil
}

type syncVacio struct{}

func (syncVacio) LastSuccess() time.Time                { return time.Time{} }
func (syncVacio) SyncOne(context.Context, string) error { return nil }

type sourcesVacio struct{}

func (sourcesVacio) List(context.Context) ([]ports.Source, error)    { return nil, nil }
func (sourcesVacio) Add(context.Context, ports.Source) error         { return nil }
func (sourcesVacio) Remove(context.Context, string) error            { return nil }
func (sourcesVacio) TouchSync(context.Context, string, int64) error  { return nil }
func (sourcesVacio) SetTvgURL(context.Context, string, string) error { return nil }

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
		streamsVacio{}, sqlDB, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{})

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
		// /imagen es otra literal que convive con /{id}/health: tiene que
		// resolverse antes de que chi la confunda con un ID de canal.
		{"/channels/imagen", http.StatusOK},
		{"/channels/stream?id=x", http.StatusNotFound},
		{"/channels/loquesea/health", http.StatusNotFound},
		// /sources* (Tarea 4): alcanzables a través del router REAL, no de un
		// mock de handler. Igual que arriba, "sugeridas" es una ruta literal
		// que convive con /{id}/sync — si chi la absorbiera como parámetro,
		// esto dejaría de ser 200 con la constante y pasaría a intentar un
		// sync sobre una fuente inexistente.
		{"/sources", http.StatusOK},
		{"/sources/sugeridas", http.StatusOK},
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
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{})

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
		r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{ProxyActivo: c.activo})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls", nil))
		if rec.Code != c.quiero {
			t.Errorf("proxyActivo=%v → %d, quiero %d", c.activo, rec.Code, c.quiero)
		}
	}
}

type catalogoFijo map[string]bool

func (c catalogoFijo) ExisteURL(_ context.Context, u string) (bool, error) { return c[u], nil }

// Una URL que no es del catálogo ni lleva firma no se relaya: el proxy ya no
// es un relé abierto. Se comprueba por el código, sin red: el 403 llega antes
// de resolver nada.
func TestProxyNoRelayaURLsAjenasAlCatalogo(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil,
		api.Options{ProxyActivo: true, Catalogo: catalogoFijo{"https://origen.example/canal.m3u8": true}})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape("https://atacante.example/x"), nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("URL ajena = %d, quiero 403", rec.Code)
	}
}

// Una URL del catálogo SÍ se relaya, y el manifiesto que devuelve el origen
// sale con sus URLs hijas firmadas: prueba de extremo a extremo de que
// api.Options.Catalogo llega de verdad hasta el proxy montado por el router,
// no solo que rechaza lo ajeno (ver TestProxyNoRelayaURLsAjenasAlCatalogo).
func TestProxyRelayaURLsDelCatalogo(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:6.0,\nseg1.ts\n"))
	}))
	defer origen.Close()

	canalURL := origen.URL + "/canal.m3u8"
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil,
		// httptest.NewServer escucha en 127.0.0.1: hay que permitir destinos
		// privados para que este test ejercite el relay real, no solo el 403
		// de SSRF.
		api.Options{ProxyActivo: true, PermitirDestinosPrivados: true, Catalogo: catalogoFijo{canalURL: true}})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape(canalURL), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "&f=") {
		t.Errorf("el segmento hijo no salió firmado:\n%s", rec.Body.String())
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

	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{ProxyActivo: false})

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

func routerModoRed(t *testing.T) http.Handler {
	t.Helper()
	return api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil,
		api.Options{ProxyActivo: true, Sesiones: acceso.NuevasSesiones("la-clave", nil)})
}

func TestModoRedExigeSesionEnAPIYProxy(t *testing.T) {
	r := routerModoRed(t)
	for _, c := range []struct {
		ruta   string
		quiero int
	}{
		{"/channels", http.StatusUnauthorized},
		{"/sources", http.StatusUnauthorized},
		{"/proxy/hls?u=x", http.StatusUnauthorized},
		{"/acceso", http.StatusOK},
		// Encoded-path bypass (ruling de la Tarea 4): chi enruta por
		// r.URL.RawPath cuando está fijado, así que "/acces%6F" no coincide
		// con la ruta /acceso y cae al fallback SPA — a menos que exenta()
		// también exija RawPath == "". Sin ese fix, esto daría 200/404 en vez
		// de 401.
		{"/acces%6F", http.StatusUnauthorized},
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.ruta, nil))
		if rec.Code != c.quiero {
			t.Errorf("%s = %d, quiero %d", c.ruta, rec.Code, c.quiero)
		}
	}
}

func TestLoopbackNoMontaAcceso(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))
	if rec.Code == http.StatusUnauthorized {
		t.Error("sin Sesiones el router exige acceso: el modo loopback cambió")
	}
}

// POST /streams/desenlace es mutante: MismoOrigen (global) lo corta en
// cross-site antes de llegar al handler; en mismo origen, sin registrador (el
// caso de este test, api.Options{} sin Desenlaces) responde 503, nunca panic.
func TestStreamsDesenlaceEsMutanteProtegido(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{})
	cuerpo := `{"url":"http://o/x.m3u8","resultado":"fallo","motivo":"caido"}`
	// Cross-site: lo corta MismoOrigen antes de llegar al handler.
	req := httptest.NewRequest(http.MethodPost, "/streams/desenlace", strings.NewReader(cuerpo))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site → %d, quiero 403", rec.Code)
	}
	// Mismo origen sin registrador (tests): 503, no panic.
	req = httptest.NewRequest(http.MethodPost, "/streams/desenlace", strings.NewReader(cuerpo))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("sin registrador → %d, quiero 503", rec.Code)
	}
}
