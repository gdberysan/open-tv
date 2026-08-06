package opensource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

const mockM3U = `#EXTM3U
#EXTINF:-1 tvg-id="1" tvg-logo="http://logo.com/1.png" group-title="News",News Channel
http://stream.com/news.m3u8
#EXTINF:-1 tvg-id="2" tvg-logo="http://logo.com/2.png" group-title="Sports",Sports Channel
http://stream.com/sports.m3u8
`

func TestProvider_GetLiveChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockM3U))
	}))
	defer server.Close()

	client := server.Client()
	p := NewProvider("prov_1", server.URL, client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	channels, err := p.GetLiveChannels(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(channels) != 2 {
		t.Fatalf("Expected 2 channels, got %d", len(channels))
	}

	if channels[0].Name != "News Channel" {
		t.Errorf("Expected 'News Channel', got '%s'", channels[0].Name)
	}
	if channels[0].LogoURL != "http://logo.com/1.png" {
		t.Errorf("Expected logo 'http://logo.com/1.png', got '%s'", channels[0].LogoURL)
	}
	if channels[0].CategoryID != "News" {
		t.Errorf("Expected group 'News', got '%s'", channels[0].CategoryID)
	}
	if channels[0].ProviderType != domain.ProviderOpenSource {
		t.Errorf("Expected provider type '%s', got '%s'", domain.ProviderOpenSource, channels[0].ProviderType)
	}
}

func TestProvider_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer server.Close()

	p := NewProvider("prov_1", server.URL, server.Client())
	ctx := context.Background()

	err := p.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}
