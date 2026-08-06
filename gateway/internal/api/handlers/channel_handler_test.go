package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

// Mocks rápidos para los tests unitarios
type mockRepo struct{}

func (m *mockRepo) Save(ctx context.Context, ch domain.Channel) error { return nil }
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
func (m *mockProvider) ID() string { return "mock" }
func (m *mockProvider) Type() domain.ProviderType { return domain.ProviderOpenSource }
func (m *mockProvider) GetLiveChannels(ctx context.Context) ([]domain.Channel, error) { return nil, nil }
func (m *mockProvider) GetStreamURL(ctx context.Context, channelID domain.ChannelID) (string, error) {
	return "http://mock.com/" + string(channelID) + ".ts", nil
}
func (m *mockProvider) GetEPGData(ctx context.Context, channelID domain.ChannelID) ([]domain.EPGEntry, error) {
	return []domain.EPGEntry{{Title: "Mock Program"}}, nil
}
func (m *mockProvider) HealthCheck(ctx context.Context) error { return nil }

func setupRouter() http.Handler {
	r := chi.NewRouter()
	h := NewChannelHandler(&mockRepo{}, &mockProvider{})
	r.Get("/channels", h.GetChannels)
	r.Get("/channels/stream", h.GetStreamURL)
	r.Get("/channels/{id}/epg", h.GetEPG)
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

func TestChannelHandler_GetEPG(t *testing.T) {
	r := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/channels/123/epg", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rr.Code, http.StatusOK)
	}

	var epg []domain.EPGEntry
	json.NewDecoder(rr.Body).Decode(&epg)
	if len(epg) != 1 || epg[0].Title != "Mock Program" {
		t.Errorf("Unexpected epg: %v", epg)
	}
}
