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
	// Del tvg-id se deriva CountryCode (formato "Nombre.cc@Feed")
	if channels[0].TvgID != "1" {
		t.Errorf("Expected TvgID '1', got '%s'", channels[0].TvgID)
	}
}

func TestNewProvider_NilClientGetsTimeout(t *testing.T) {
	p := NewProvider("prov_1", "http://example.com", nil)

	if p.client == http.DefaultClient {
		t.Fatal("NewProvider(nil) no debe usar http.DefaultClient (sin timeout)")
	}
	if p.client.Timeout <= 0 {
		t.Fatalf("El cliente por defecto debe tener timeout > 0, tiene %v", p.client.Timeout)
	}
}

func TestProvider_GetLiveChannels_RejectsOversizedM3U(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#EXTM3U\n"))
		for i := 0; i < 100; i++ {
			_, _ = w.Write([]byte(mockM3U))
		}
	}))
	defer server.Close()

	p := NewProvider("prov_1", server.URL, server.Client())
	p.maxBodyBytes = 1024 // forzar un límite pequeño para el test

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	channels, err := p.GetLiveChannels(ctx)
	if err == nil {
		t.Fatalf("Se esperaba error por M3U demasiado grande, se obtuvieron %d canales", len(channels))
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
