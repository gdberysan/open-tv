package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

type mockEPGRepo struct {
	entries     []domain.EPGEntry
	lastFrom    time.Time
	lastTo      time.Time
	lastChannel domain.ChannelID
}

func (m *mockEPGRepo) SaveBatch(context.Context, []domain.EPGEntry) error { return nil }

func (m *mockEPGRepo) FindByChannelAndWindow(_ context.Context, ch domain.ChannelID, from, to time.Time) ([]domain.EPGEntry, error) {
	m.lastChannel, m.lastFrom, m.lastTo = ch, from, to
	return m.entries, nil
}

func (m *mockEPGRepo) FindCurrentlyAiring(context.Context, time.Time) ([]domain.EPGEntry, error) {
	return m.entries, nil
}

func (m *mockEPGRepo) DeleteEndedBefore(context.Context, time.Time) (int64, error) { return 0, nil }

func setupEPGRouter(repo *mockEPGRepo) http.Handler {
	r := chi.NewRouter()
	h := NewEPGHandler(repo)
	r.Get("/epg", h.GetWindow)
	r.Get("/epg/now", h.GetNow)
	r.Get("/channels/{id}/epg", h.GetByChannel)
	return r
}

func entradaDePrueba() domain.EPGEntry {
	return domain.EPGEntry{
		ChannelID: "opensource-BBC News",
		Title:     "Noticias",
		StartAt:   time.Now().Truncate(time.Hour),
		EndAt:     time.Now().Truncate(time.Hour).Add(time.Hour),
	}
}

func TestEPGHandler_GetWindow(t *testing.T) {
	repo := &mockEPGRepo{entries: []domain.EPGEntry{entradaDePrueba()}}
	r := setupEPGRouter(repo)

	from := time.Now().UTC().Format(time.RFC3339)
	to := time.Now().UTC().Add(6 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet,
		"/epg?channel=opensource-BBC+News&from="+from+"&to="+to, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	var entries []domain.EPGEntry
	if err := json.NewDecoder(rr.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Title != "Noticias" {
		t.Errorf("entries = %+v", entries)
	}
	if repo.lastChannel != "opensource-BBC News" {
		t.Errorf("channel = %q", repo.lastChannel)
	}
}

func TestEPGHandler_GetWindow_DefaultsProximas6Horas(t *testing.T) {
	repo := &mockEPGRepo{}
	r := setupEPGRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/epg?channel=x", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	ventana := repo.lastTo.Sub(repo.lastFrom)
	if ventana != 6*time.Hour {
		t.Errorf("ventana default = %v, want 6h", ventana)
	}
	if time.Since(repo.lastFrom) > time.Minute {
		t.Errorf("from default = %v, quiere ~ahora", repo.lastFrom)
	}
}

func TestEPGHandler_GetWindow_SinCanalEs400(t *testing.T) {
	r := setupEPGRouter(&mockEPGRepo{})

	req := httptest.NewRequest(http.MethodGet, "/epg", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEPGHandler_GetWindow_VentanaMayorA24hEs400(t *testing.T) {
	r := setupEPGRouter(&mockEPGRepo{})

	from := time.Now().UTC().Format(time.RFC3339)
	to := time.Now().UTC().Add(25 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet,
		"/epg?channel=x&from="+from+"&to="+to, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (ventana máxima 24h)", rr.Code)
	}
}

func TestEPGHandler_GetWindow_FechaInvalidaEs400(t *testing.T) {
	r := setupEPGRouter(&mockEPGRepo{})

	req := httptest.NewRequest(http.MethodGet, "/epg?channel=x&from=ayer", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEPGHandler_GetNow(t *testing.T) {
	repo := &mockEPGRepo{entries: []domain.EPGEntry{entradaDePrueba()}}
	r := setupEPGRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/epg/now", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var entries []domain.EPGEntry
	if err := json.NewDecoder(rr.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %+v", entries)
	}
}

// Ruta legacy /channels/{id}/epg: ahora servida desde la DB (antes un stub
// del provider que siempre devolvía []).
func TestEPGHandler_GetByChannel(t *testing.T) {
	repo := &mockEPGRepo{entries: []domain.EPGEntry{entradaDePrueba()}}
	r := setupEPGRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/channels/opensource-BBC%20News/epg", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var entries []domain.EPGEntry
	if err := json.NewDecoder(rr.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Title != "Noticias" {
		t.Errorf("entries = %+v", entries)
	}
	if repo.lastChannel != "opensource-BBC News" {
		t.Errorf("channel = %q", repo.lastChannel)
	}
}

// Sobre una conexión HTTP real (no httptest.NewRequest) chi entrega los path
// params aún escapados: "BBC%20News" en vez de "BBC News". El handler debe
// des-escapar antes de consultar la DB.
func TestEPGHandler_GetByChannel_DecodificaPathEscapado(t *testing.T) {
	repo := &mockEPGRepo{}
	server := httptest.NewServer(setupEPGRouter(repo))
	defer server.Close()

	// La mezcla de %20, paréntesis sin escapar y %5B hace que Go conserve
	// RawPath y chi entregue el param escapado (reproducido con curl real)
	resp, err := http.Get(server.URL + "/channels/opensource-BBC%20Alba%20(1080p)%20(HEVC)%20%5BGeo-blocked%5D/epg")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if repo.lastChannel != "opensource-BBC Alba (1080p) (HEVC) [Geo-blocked]" {
		t.Errorf("channel = %q, quiere el ID des-escapado", repo.lastChannel)
	}
}

func TestEPGHandler_GetNow_SinDatosDevuelveListaVacia(t *testing.T) {
	r := setupEPGRouter(&mockEPGRepo{})

	req := httptest.NewRequest(http.MethodGet, "/epg/now", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	// `[]`, no `null`: los clientes JSON no deben lidiar con null
	if got := rr.Body.String(); got != "[]\n" && got != "[]" {
		t.Errorf("body = %q, want []", got)
	}
}
