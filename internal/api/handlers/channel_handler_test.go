package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/adapters/providers/iptvorg"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
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
func (m *mockRepo) Random(context.Context, ports.ChannelFilter) (domain.Channel, error) {
	if m.err != nil {
		return domain.Channel{}, m.err
	}
	return domain.Channel{ID: "1", Name: "MockChannel"}, nil
}

func (m *mockRepo) Countries(context.Context) ([]ports.Faceta, error) {
	return []ports.Faceta{{Valor: "ES", Count: 3}}, nil
}
func (m *mockRepo) Categories(context.Context) ([]ports.Faceta, error) {
	return []ports.Faceta{{Valor: "News", Count: 2}}, nil
}
func (m *mockRepo) Qualities(context.Context) ([]ports.Faceta, error) {
	return []ports.Faceta{{Valor: "hd", Count: 5}, {Valor: "fhd", Count: 3}, {Valor: "4k", Count: 1}}, nil
}

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
func (m *mockProvider) GetStreamsDeCanal(ctx context.Context, channelID domain.ChannelID) ([]iptvorg.StreamExtra, error) {
	return nil, nil
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
	// mirrors, si no es nil, gana sobre el cálculo a partir de streams: es lo
	// único que permite fijar WebOK a un valor concreto en el test, porque
	// domain.Stream no tiene campo Web (ver FindMirrorsByChannelID).
	mirrors []ports.MirrorHealth
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

// FindMirrorsByChannelID replica el orden del repo real (vivos primero, por
// latencia ascendente) a partir de los mismos domain.Stream de m.streams.
func (m *mockStreamRepo) FindMirrorsByChannelID(ctx context.Context, id domain.ChannelID) ([]ports.MirrorHealth, error) {
	if m.mirrors != nil {
		return m.mirrors, nil
	}
	streams := append([]domain.Stream(nil), m.streams[id]...)
	sort.SliceStable(streams, func(i, j int) bool {
		if streams[i].IsAlive != streams[j].IsAlive {
			return streams[i].IsAlive
		}
		return streams[i].LatencyMs < streams[j].LatencyMs
	})
	mirrors := make([]ports.MirrorHealth, 0, len(streams))
	for _, s := range streams {
		mirrors = append(mirrors, ports.MirrorHealth{
			URL:       s.URL,
			IsAlive:   s.IsAlive,
			LatencyMs: s.LatencyMs,
		})
	}
	return mirrors, nil
}

func (m *mockStreamRepo) MarkAlive(ctx context.Context, id string, latencyMs int64) error { return nil }
func (m *mockStreamRepo) MarkDead(ctx context.Context, id string) error                   { return nil }
func (m *mockStreamRepo) MarkBatch(context.Context, []ports.StreamHealth) error           { return nil }
func (m *mockStreamRepo) DeleteStale(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (m *mockStreamRepo) CabecerasPorURL(context.Context, string) (string, string, error) {
	return "", "", nil
}

func setupRouter() http.Handler {
	return setupRouterWith(&mockProvider{}, &mockStreamRepo{})
}

func setupRouterWith(provider ports.ProviderPort, streams ports.StreamRepository) http.Handler {
	return setupRouterFull(&mockRepo{}, provider, streams)
}

func setupRouterFull(repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository) http.Handler {
	r := chi.NewRouter()
	h := NewChannelHandler(slog.New(slog.DiscardHandler), repo, provider, streams,
		NewAirplayProber(http.DefaultClient, time.Hour, 10, false))
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
	_ = json.NewDecoder(rr.Body).Decode(&res)
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
	_ = json.NewDecoder(rr.Body).Decode(&res)
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
		&mockProvider{}, &mockStreamRepo{},
		NewAirplayProber(http.DefaultClient, time.Hour, 10, false))

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

func TestGetStreamURLIncluyeAirplayOK(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(masterOK))
	}))
	defer origen.Close()

	// mockStreamRepo guarda un map[ChannelID][]Stream y FindBestByChannelID
	// escoge el vivo de menor latencia, así que IsAlive es obligatorio.
	streams := &mockStreamRepo{
		streams: map[domain.ChannelID][]domain.Stream{
			"1": {{URL: origen.URL + "/a.m3u8", IsAlive: true, LatencyMs: 10}},
		},
	}
	h := NewChannelHandler(nil, &mockRepo{}, &mockProvider{}, streams,
		NewAirplayProber(origen.Client(), time.Hour, 10, true))

	req := httptest.NewRequest(http.MethodGet, "/channels/stream?id=1", nil)
	rec := httptest.NewRecorder()
	h.GetStreamURL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d, quiero 200", rec.Code)
	}
	var body struct {
		URL       string `json:"url"`
		AirplayOK *bool  `json:"airplay_ok"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if body.URL == "" {
		t.Error("url vacía: el sondeo no puede romper la respuesta principal")
	}
	if body.AirplayOK == nil || !*body.AirplayOK {
		t.Errorf("airplay_ok = %v, quiero true", body.AirplayOK)
	}
}

// /channels/streams es endpoint nuevo (no lo consume Flutter): expone los
// mirrors ordenados por salud de la Tarea 1, con web_ok mapeado a bool/null.
func TestGetChannelStreamsDevuelveMirrorsOrdenados(t *testing.T) {
	streams := &mockStreamRepo{mirrors: []ports.MirrorHealth{
		{URL: "https://a/x.m3u8", IsAlive: true, LatencyMs: 100, WebOK: domain.WebOK, Codec: domain.CodecNo, Codecs: "mpeg2video,mp2"},
		{URL: "https://b/x.m3u8", IsAlive: true, LatencyMs: 300, WebOK: domain.WebNo},
	}}
	h := NewChannelHandler(slog.New(slog.DiscardHandler), &mockRepo{}, &mockProvider{}, streams, nil)

	rec := httptest.NewRecorder()
	h.GetChannelStreams(rec, httptest.NewRequest(http.MethodGet, "/channels/streams?id=c1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got) != 2 || got[0]["url"] != "https://a/x.m3u8" {
		t.Fatalf("mirrors = %v", got)
	}
	if got[0]["web_ok"] != true || got[1]["web_ok"] != false {
		t.Errorf("web_ok mal mapeado: %v", got)
	}
	if got[0]["codec_ok"] != false || got[0]["codecs"] != "mpeg2video,mp2" {
		t.Errorf("codec_ok/codecs mal mapeados en el primero: %v", got[0])
	}
	if got[1]["codec_ok"] != nil || got[1]["codecs"] != "" {
		t.Errorf("sin sondear debe ser null y '': %v", got[1])
	}
}

// GetQualities debe reutilizar el MISMO predicado que /channels?quality=,
// no una copia: se prueba contra el repo SQLite real (no el mock) para que
// el test verifique el predicado de verdad y no una expectativa inventada.
// hd (>=720p) es un superconjunto de fhd (>=1080p), que a su vez lo es de 4k
// —así es como qualityWhereClause construye los tramos—, y un canal cuyo
// nombre no lleva ningún token de resolución reconocible no debe contar en
// ninguno.
func TestChannelHandler_GetQualities(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
	// Sin seed de IPTV-org (retirado en P0.7/SourceRepository), channels.provider_id
	// exige una fila en providers: se da de alta explícitamente para este test.
	if _, err := sqlDB.Exec(`
		INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
		VALUES ('opensource', 'opensource', 'http://test.invalid/index.m3u', 100, 1, 0, 0)
	`); err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	repo := db.NewChannelRepository(sqlDB)

	ctx := context.Background()
	channels := []domain.Channel{
		{ID: "c-720", Name: "Canal HD (720p)", ProviderID: "opensource", ProviderType: domain.ProviderOpenSource},
		{ID: "c-1080", Name: "Canal FHD (1080p)", ProviderID: "opensource", ProviderType: domain.ProviderOpenSource},
		{ID: "c-2160", Name: "Canal 4K (2160p)", ProviderID: "opensource", ProviderType: domain.ProviderOpenSource},
		// Sin token de resolución reconocible: no debe contar en ningún tramo.
		{ID: "c-sd", Name: "Canal SD (576p)", ProviderID: "opensource", ProviderType: domain.ProviderOpenSource},
	}
	if err := repo.SaveBatch(ctx, channels); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	h := NewChannelHandler(slog.New(slog.DiscardHandler), repo, &mockProvider{}, &mockStreamRepo{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/channels/qualities", nil)
	rec := httptest.NewRecorder()
	h.GetQualities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d, quiero 200 (body %s)", rec.Code, rec.Body.String())
	}

	var got []ports.Faceta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("esperaba 3 tramos (hd/fhd/4k), tengo %d: %+v", len(got), got)
	}

	porValor := map[string]int{}
	for _, f := range got {
		porValor[f.Valor] = f.Count
	}

	if porValor["hd"] != 3 {
		t.Errorf("hd = %d, quiero 3 (720p + 1080p + 4k)", porValor["hd"])
	}
	if porValor["fhd"] != 2 {
		t.Errorf("fhd = %d, quiero 2 (1080p + 4k)", porValor["fhd"])
	}
	if porValor["4k"] != 1 {
		t.Errorf("4k = %d, quiero 1", porValor["4k"])
	}
}

func TestGetChannelStreamsSinIdEs400(t *testing.T) {
	h := NewChannelHandler(slog.New(slog.DiscardHandler), &mockRepo{}, &mockProvider{}, &mockStreamRepo{}, nil)
	rec := httptest.NewRecorder()
	h.GetChannelStreams(rec, httptest.NewRequest(http.MethodGet, "/channels/streams", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("código %d, quiero 400", rec.Code)
	}
}
