package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

// Mocks rápidos para los tests unitarios
type mockRepo struct{}

func (m *mockRepo) Save(ctx context.Context, ch domain.Channel) error              { return nil }
func (m *mockRepo) SaveBatch(ctx context.Context, channels []domain.Channel) error { return nil }
func (m *mockRepo) FindByID(ctx context.Context, id domain.ChannelID) (domain.Channel, error) {
	return domain.Channel{}, nil
}
func (m *mockRepo) FindFiltered(ctx context.Context, f ports.ChannelFilter) ([]domain.Channel, error) {
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
func (m *mockStreamRepo) FindByChannelID(ctx context.Context, id domain.ChannelID) ([]domain.Stream, error) {
	return m.streams[id], nil
}
func (m *mockStreamRepo) FindBestByChannelID(ctx context.Context, id domain.ChannelID) (domain.Stream, error) {
	return domain.Stream{}, errors.New("sin streams vivos")
}
func (m *mockStreamRepo) MarkAlive(ctx context.Context, id string, latencyMs int64) error { return nil }
func (m *mockStreamRepo) MarkDead(ctx context.Context, id string) error                   { return nil }

func setupRouter() http.Handler {
	return setupRouterWith(&mockProvider{}, &mockStreamRepo{})
}

func setupRouterWith(provider ports.ProviderPort, streams ports.StreamRepository) http.Handler {
	r := chi.NewRouter()
	h := NewChannelHandler(&mockRepo{}, provider, streams)
	r.Get("/channels", h.GetChannels)
	r.Get("/channels/stream", h.GetStreamURL)
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
