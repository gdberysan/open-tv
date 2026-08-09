package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
	"github.com/gdberysan/open-tv/gateway/internal/ports"
	"github.com/go-chi/chi/v5"
)

// Mocks rápidos para los tests unitarios
type mockRepo struct {
	lastFilter ports.ChannelFilter
	err        error
}

func (m *mockRepo) Save(ctx context.Context, ch domain.Channel) error              { return nil }
func (m *mockRepo) SaveBatch(ctx context.Context, channels []domain.Channel) error { return nil }
func (m *mockRepo) FindByID(ctx context.Context, id domain.ChannelID) (domain.Channel, error) {
	return domain.Channel{}, nil
}
func (m *mockRepo) FindFiltered(ctx context.Context, f ports.ChannelFilter) ([]domain.Channel, error) {
	m.lastFilter = f
	if m.err != nil {
		return nil, m.err
	}
	return []domain.Channel{
		{ID: "1", Name: "MockChannel"},
	}, nil
}
func (m *mockRepo) Search(ctx context.Context, query string, limit int) ([]domain.Channel, error) {
	return []domain.Channel{
		{ID: "1", Name: "MockChannel"},
	}, nil
}
func (m *mockRepo) Delete(ctx context.Context, id domain.ChannelID) error { return nil }

// Devuelve el número de canales que FindFiltered entregaría, para que el test
// del contador compruebe algo real.
func (m *mockRepo) CountFiltered(context.Context, ports.ChannelFilter) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return 1, nil
}
func (m *mockRepo) DeleteStale(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}

type mockProvider struct{}

func (m *mockProvider) ID() string                { return "mock" }
func (m *mockProvider) Type() domain.ProviderType { return domain.ProviderOpenSource }
func (m *mockProvider) GetLiveChannels(ctx context.Context) ([]domain.Channel, error) {
	return nil, nil
}
func (m *mockProvider) GetStreamURL(ctx context.Context, channelID domain.ChannelID) (string, error) {
	return "http://mock.com/" + string(channelID) + ".ts", nil
}
func (m *mockProvider) HealthCheck(ctx context.Context) error { return nil }

// mockProviderSinCache simula un provider recién reiniciado: el caché en
// memoria de stream URLs está vacío hasta que complete el primer sync.
type mockProviderSinCache struct{ mockProvider }

func (m *mockProviderSinCache) GetStreamURL(ctx context.Context, channelID domain.ChannelID) (string, error) {
	return "", errors.New("sync pendiente")
}

type mockStreamRepo struct {
	streams map[domain.ChannelID][]domain.Stream
}

func (m *mockStreamRepo) Save(ctx context.Context, s domain.Stream) error         { return nil }
func (m *mockStreamRepo) SaveBatch(ctx context.Context, ss []domain.Stream) error { return nil }
func (m *mockStreamRepo) FindAll(ctx context.Context) ([]domain.Stream, error)    { return nil, nil }
func (m *mockStreamRepo) FindByChannelID(ctx context.Context, id domain.ChannelID) ([]domain.Stream, error) {
	return m.streams[id], nil
}

// FindBestByChannelID replica la semántica del repo real: solo streams vivos,
// el de menor latencia. Antes devolvía siempre error, lo que ocultaba que el
// handler ni siquiera lo llamaba.
func (m *mockStreamRepo) FindBestByChannelID(ctx context.Context, id domain.ChannelID) (domain.Stream, error) {
	var mejor domain.Stream
	encontrado := false
	for _, s := range m.streams[id] {
		if !s.IsAlive {
			continue
		}
		if !encontrado || s.LatencyMs < mejor.LatencyMs {
			mejor, encontrado = s, true
		}
	}
	if !encontrado {
		return domain.Stream{}, errors.New("sin streams vivos")
	}
	return mejor, nil
}
func (m *mockStreamRepo) MarkAlive(ctx context.Context, id string, latencyMs int64) error { return nil }
func (m *mockStreamRepo) MarkDead(ctx context.Context, id string) error                   { return nil }
func (m *mockStreamRepo) MarkBatch(context.Context, []ports.StreamHealth) error           { return nil }

func setupRouter() http.Handler {
	return setupRouterWith(&mockProvider{}, &mockStreamRepo{})
}

func setupRouterWith(provider ports.ProviderPort, streams ports.StreamRepository) http.Handler {
	return setupRouterFull(&mockRepo{}, provider, streams)
}

func setupRouterFull(repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository) http.Handler {
	r := chi.NewRouter()
	h := NewChannelHandler(slog.New(slog.DiscardHandler), repo, provider, streams)
	r.Get("/channels", h.GetChannels)
	r.Get("/channels/stream", h.GetStreamURL)
	r.Get("/channels/{id}/health", h.GetHealth)
	return r
}

func TestChannelHandler_GetChannels(t *testing.T) {
	r := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("status = %v, want %v", status, http.StatusOK)
	}

	var channels []domain.Channel
	if err := json.NewDecoder(rr.Body).Decode(&channels); err != nil {
		t.Fatal(err)
	}

	if len(channels) != 1 || channels[0].Name != "MockChannel" {
		t.Errorf("Unexpected response: %v", channels)
	}
}

func TestChannelHandler_GetStreamURL(t *testing.T) {
	r := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/channels/stream?id=123", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}

	var res map[string]string
	json.NewDecoder(rr.Body).Decode(&res)
	if res["url"] != "http://mock.com/123.ts" {
		t.Errorf("Unexpected url: %v", res["url"])
	}
}

// AliveOnly es el default (Fase 7): sin param se ocultan canales muertos;
// ?alive=all los muestra todos.
func TestChannelHandler_GetChannels_AliveOnlyPorDefecto(t *testing.T) {
	repo := &mockRepo{}
	r := setupRouterFull(repo, &mockProvider{}, &mockStreamRepo{})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels", nil))
	if !repo.lastFilter.AliveOnly {
		t.Error("sin param alive, AliveOnly debe ser true")
	}

	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels?alive=all", nil))
	if repo.lastFilter.AliveOnly {
		t.Error("con alive=all, AliveOnly debe ser false")
	}
}

func TestChannelHandler_GetHealth(t *testing.T) {
	checked := time.Now().Truncate(time.Second)
	streams := &mockStreamRepo{streams: map[domain.ChannelID][]domain.Stream{
		"123": {
			{ID: "st-1", ChannelID: "123", URL: "http://a/1.m3u8", IsAlive: true, LatencyMs: 120, LastChecked: checked},
			{ID: "st-2", ChannelID: "123", URL: "http://b/1.m3u8", IsAlive: false, LastChecked: checked.Add(-time.Hour)},
		},
	}}
	r := setupRouterWith(&mockProvider{}, streams)

	req := httptest.NewRequest(http.MethodGet, "/channels/123/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rr.Code, rr.Body.String())
	}
	var res struct {
		Alive     bool   `json:"alive"`
		LatencyMs int64  `json:"latency_ms"`
		CheckedAt string `json:"checked_at"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if !res.Alive || res.LatencyMs != 120 {
		t.Errorf("res = %+v, quiere alive con la latencia del stream vivo", res)
	}
	if res.CheckedAt == "" {
		t.Error("checked_at vacío; quiere el último chequeo")
	}
}

func TestChannelHandler_GetHealth_SinStreamsEs404(t *testing.T) {
	r := setupRouterWith(&mockProvider{}, &mockStreamRepo{})

	req := httptest.NewRequest(http.MethodGet, "/channels/123/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

// Tras un reinicio el caché del provider está vacío hasta el primer sync;
// el handler debe caer a los streams persistidos en DB en vez de dar 404.
func TestChannelHandler_GetStreamURL_FallbackADB(t *testing.T) {
	streams := &mockStreamRepo{streams: map[domain.ChannelID][]domain.Stream{
		"123": {{ID: "st-1", ChannelID: "123", URL: "http://db.example/123.m3u8"}},
	}}
	r := setupRouterWith(&mockProviderSinCache{}, streams)

	req := httptest.NewRequest(http.MethodGet, "/channels/stream?id=123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %v, want %v", rr.Code, http.StatusOK)
	}
	var res map[string]string
	json.NewDecoder(rr.Body).Decode(&res)
	if res["url"] != "http://db.example/123.m3u8" {
		t.Errorf("url = %q, quiere la persistida en DB", res["url"])
	}
}

func TestChannelHandler_GetStreamURL_SinCacheNiDB(t *testing.T) {
	r := setupRouterWith(&mockProviderSinCache{}, &mockStreamRepo{})

	req := httptest.NewRequest(http.MethodGet, "/channels/stream?id=123", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusNotFound)
	}
}

// Un 500 sin rastro en los logs hace indistinguible un fallo de DB de un
// deadline o de un WAL corrupto: el operador ve el mismo cuerpo opaco y nada
// en la salida estructurada.
func TestGetChannelsLogueaLaCausaDelError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := NewChannelHandler(logger, &mockRepo{err: errors.New("disco en llamas")},
		&mockProvider{}, &mockStreamRepo{})

	rec := httptest.NewRecorder()
	h.GetChannels(rec, httptest.NewRequest(http.MethodGet, "/channels", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiero 500", rec.Code)
	}
	if !strings.Contains(buf.String(), "disco en llamas") {
		t.Errorf("la causa del 500 debe aparecer en los logs; log:\n%s", buf.String())
	}
}

// El health-worker sabe qué stream está vivo y cuál no; servir persisted[0]
// (el primero por ID) tiraba esa información a la basura. En la DB real hay 19
// canales con un stream vivo y otro muerto.
func TestChannelHandler_GetStreamURL_PrefiereElStreamVivo(t *testing.T) {
	streams := &mockStreamRepo{streams: map[domain.ChannelID][]domain.Stream{
		// st-a es el primero por ID pero está muerto; st-b es el bueno.
		"123": {
			{ID: "st-a", ChannelID: "123", URL: "http://muerto/1.m3u8", IsAlive: false},
			{ID: "st-b", ChannelID: "123", URL: "http://vivo/1.m3u8", IsAlive: true, LatencyMs: 120},
		},
	}}
	r := setupRouterWith(&mockProvider{}, streams)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels/stream?id=123", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200 (body %s)", rr.Code, rr.Body.String())
	}
	var res map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&res)
	if res["url"] != "http://vivo/1.m3u8" {
		t.Errorf("url = %q, quiero la del stream vivo", res["url"])
	}
}

// Entre varios vivos, el de menor latencia.
func TestChannelHandler_GetStreamURL_PrefiereMenorLatencia(t *testing.T) {
	streams := &mockStreamRepo{streams: map[domain.ChannelID][]domain.Stream{
		"123": {
			{ID: "st-a", ChannelID: "123", URL: "http://lento/1.m3u8", IsAlive: true, LatencyMs: 900},
			{ID: "st-b", ChannelID: "123", URL: "http://rapido/1.m3u8", IsAlive: true, LatencyMs: 80},
		},
	}}
	r := setupRouterWith(&mockProvider{}, streams)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels/stream?id=123", nil))

	var res map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&res)
	if res["url"] != "http://rapido/1.m3u8" {
		t.Errorf("url = %q, quiero la de menor latencia", res["url"])
	}
}

// Antes de la primera pasada del health-worker ningún stream está vivo, pero el
// canal sí se puede reproducir: hay que servir alguno en vez de dar 404.
func TestChannelHandler_GetStreamURL_SinChequearSirveIgual(t *testing.T) {
	streams := &mockStreamRepo{streams: map[domain.ChannelID][]domain.Stream{
		"123": {{ID: "st-a", ChannelID: "123", URL: "http://sinchequear/1.m3u8"}},
	}}
	r := setupRouterWith(&mockProvider{}, streams)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels/stream?id=123", nil))

	var res map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&res)
	if res["url"] != "http://sinchequear/1.m3u8" {
		t.Errorf("url = %q, quiero el stream sin chequear", res["url"])
	}
}

// Arranque en frío: la DB aún no sabe nada del canal, pero el provider ya
// parseó el M3U. Es el único caso en que la caché en memoria manda.
func TestChannelHandler_GetStreamURL_ArranqueEnFrioUsaLaCache(t *testing.T) {
	r := setupRouterWith(&mockProvider{}, &mockStreamRepo{})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels/stream?id=123", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200", rr.Code)
	}
	var res map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&res)
	if res["url"] != "http://mock.com/123.ts" {
		t.Errorf("url = %q, quiero la de la caché del provider", res["url"])
	}
}

// La app necesita el total para que su contador no mienta.
func TestGetChannelsDevuelveElTotalEnCabecera(t *testing.T) {
	r := setupRouter()
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels", nil))

	if got := rr.Header().Get("X-Total-Count"); got == "" {
		t.Error("falta X-Total-Count; la app lo necesita para el contador")
	}
}
