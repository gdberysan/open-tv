package opensource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/providers/iptvorg"
	"github.com/gdberysan/open-tv/internal/domain"
)

const mockM3U = `#EXTM3U
#EXTINF:-1 tvg-id="1" tvg-logo="http://logo.com/1.png" group-title="News",News Channel
http://stream.com/news.m3u8
#EXTINF:-1 tvg-id="2" tvg-logo="http://logo.com/2.png" group-title="Sports",Sports Channel
http://stream.com/sports.m3u8
`

const mockM3UWithTvgURL = `#EXTM3U url-tvg="https://e/g.xml.gz"
#EXTINF:-1 tvg-id="1",News Channel
http://stream.com/news.m3u8
`

const mockM3UWithTvgURLList = `#EXTM3U url-tvg="https://a,https://b"
#EXTINF:-1 tvg-id="1",News Channel
http://stream.com/news.m3u8
`

const mockM3UWithXTvgURL = `#EXTM3U x-tvg-url="https://e/g.xml"
#EXTINF:-1 tvg-id="1",News Channel
http://stream.com/news.m3u8
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

// ── fuentes locales (file://) ───────────────────────────────────────────────

func TestProvider_GetLiveChannels_FileSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lista.m3u")
	if err := os.WriteFile(path, []byte(mockM3U), 0o600); err != nil {
		t.Fatalf("no se pudo escribir el fixture: %v", err)
	}

	p := NewProvider("prov_1", "file://"+path, nil, WithAllowedFileDir(dir))

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

	// GetStreamURL debe quedar poblado igual que en la vía HTTP.
	streamURL, err := p.GetStreamURL(ctx, channels[0].ID)
	if err != nil {
		t.Fatalf("Unexpected error en GetStreamURL: %v", err)
	}
	if streamURL != "http://stream.com/news.m3u8" {
		t.Errorf("Expected stream URL 'http://stream.com/news.m3u8', got '%s'", streamURL)
	}
}

func TestProvider_GetLiveChannels_FileDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lista.m3u")
	if err := os.WriteFile(path, []byte(mockM3U), 0o600); err != nil {
		t.Fatalf("no se pudo escribir el fixture: %v", err)
	}

	// Sin WithAllowedFileDir: el soporte file:// debe quedar deshabilitado.
	p := NewProvider("prov_1", "file://"+path, nil)

	_, err := p.GetLiveChannels(context.Background())
	if err == nil {
		t.Fatal("Se esperaba error: soporte file:// deshabilitado por defecto")
	}
}

func TestProvider_GetLiveChannels_FileOutsideAllowedDir(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()
	path := filepath.Join(outsideDir, "lista.m3u")
	if err := os.WriteFile(path, []byte(mockM3U), 0o600); err != nil {
		t.Fatalf("no se pudo escribir el fixture: %v", err)
	}

	p := NewProvider("prov_1", "file://"+path, nil, WithAllowedFileDir(allowedDir))

	_, err := p.GetLiveChannels(context.Background())
	if err == nil {
		t.Fatal("Se esperaba error por ruta fuera del directorio permitido")
	}
}

func TestProvider_GetLiveChannels_FilePathTraversal(t *testing.T) {
	allowedDir := t.TempDir()
	secretDir := t.TempDir()
	secretFile := filepath.Join(secretDir, "secreto.m3u")
	if err := os.WriteFile(secretFile, []byte(mockM3U), 0o600); err != nil {
		t.Fatalf("no se pudo escribir el fixture: %v", err)
	}

	// allowedDir/../<secretDir base>/secreto.m3u: tras filepath.Clean apunta
	// fuera de allowedDir (ambos t.TempDir() comparten el mismo padre).
	traversal := filepath.Join(allowedDir, "..", filepath.Base(secretDir), "secreto.m3u")

	p := NewProvider("prov_1", "file://"+traversal, nil, WithAllowedFileDir(allowedDir))

	_, err := p.GetLiveChannels(context.Background())
	if err == nil {
		t.Fatal("Se esperaba error por path traversal (../)")
	}
}

func TestProvider_GetLiveChannels_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-existe.m3u")

	p := NewProvider("prov_1", "file://"+path, nil, WithAllowedFileDir(dir))

	_, err := p.GetLiveChannels(context.Background())
	if err == nil {
		t.Fatal("Se esperaba error por fichero inexistente")
	}
}

// ── url-tvg / x-tvg-url (guía EPG declarada en la cabecera) ────────────────

func TestProvider_TvgURLs(t *testing.T) {
	tests := []struct {
		name string
		m3u  string
		want []string
	}{
		{
			name: "una URL en url-tvg",
			m3u:  mockM3UWithTvgURL,
			want: []string{"https://e/g.xml.gz"},
		},
		{
			name: "lista separada por comas en url-tvg",
			m3u:  mockM3UWithTvgURLList,
			want: []string{"https://a", "https://b"},
		},
		{
			name: "alias x-tvg-url",
			m3u:  mockM3UWithXTvgURL,
			want: []string{"https://e/g.xml"},
		},
		{
			name: "cabecera sin atributo de guia",
			m3u:  mockM3U,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.m3u))
			}))
			defer server.Close()

			p := NewProvider("prov_1", server.URL, server.Client())

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if _, err := p.GetLiveChannels(ctx); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			got := p.TvgURLs()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TvgURLs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TvgURLs también debe poblarse por la vía file://, no solo HTTP.
func TestProvider_TvgURLs_FileSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lista.m3u")
	if err := os.WriteFile(path, []byte(mockM3UWithTvgURL), 0o600); err != nil {
		t.Fatalf("no se pudo escribir el fixture: %v", err)
	}

	p := NewProvider("prov_1", "file://"+path, nil, WithAllowedFileDir(dir))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := p.GetLiveChannels(ctx); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	want := []string{"https://e/g.xml.gz"}
	got := p.TvgURLs()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TvgURLs() = %#v, want %#v", got, want)
	}
}

// ── GetStreamsDeCanal / enriquecedor ────────────────────────────────────────

// providerConM3U sirve m3u desde un httptest.Server, llama a GetLiveChannels y
// devuelve el provider ya poblado. El ID de fuente es "opensource", para que
// coincida con los IDs de canal que usan los tests de este bloque.
func providerConM3U(t *testing.T, m3u string) *Provider {
	t.Helper()
	return providerConM3UYOpts(t, m3u)
}

// providerConM3UYOpts es providerConM3U pero admitiendo Option (para
// WithEnriquecedor y similares).
func providerConM3UYOpts(t *testing.T, m3u string, opts ...Option) *Provider {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(m3u))
	}))
	t.Cleanup(server.Close)

	p := NewProvider("opensource", server.URL, server.Client(), opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := p.GetLiveChannels(ctx); err != nil {
		t.Fatalf("GetLiveChannels: %v", err)
	}
	return p
}

// enriquecedorFalso permite probar la fusión sin salir a la red.
type enriquecedorFalso struct {
	streams    map[string][]iptvorg.StreamExtra // clave "canal|feed"
	categorias map[string]string
}

func (e *enriquecedorFalso) Streams(canal, feed string) []iptvorg.StreamExtra {
	return e.streams[canal+"|"+feed]
}
func (e *enriquecedorFalso) Categoria(canal string) (string, bool) {
	c, ok := e.categorias[canal]
	return c, ok
}

const m3uUnCanal = `#EXTM3U
#EXTINF:-1 tvg-id="AndTV.in@HD" tvg-logo="l.png" group-title="General",AndTV HD
https://delm3u.example/x.m3u8
`

// SIN enriquecedor el comportamiento es el de siempre: exactamente un stream.
// Es la prueba que protege a las fuentes bring-your-own, que no tienen API.
func TestGetStreamsDeCanalSinEnriquecedor(t *testing.T) {
	p := providerConM3U(t, m3uUnCanal) // helper: sirve el M3U y llama a GetLiveChannels
	got, err := p.GetStreamsDeCanal(context.Background(), "opensource-AndTV HD")
	if err != nil {
		t.Fatalf("GetStreamsDeCanal: %v", err)
	}
	if len(got) != 1 || got[0].URL != "https://delm3u.example/x.m3u8" {
		t.Errorf("got %+v, quiero solo la URL del M3U", got)
	}
}

// Con enriquecedor: la URL del M3U va PRIMERA y los mirrors detrás, sin
// duplicar la que ya venía.
func TestGetStreamsDeCanalOrdenYDeduplicacion(t *testing.T) {
	e := &enriquecedorFalso{streams: map[string][]iptvorg.StreamExtra{
		"AndTV.in|HD": {
			{URL: "https://delm3u.example/x.m3u8"}, // duplicada a propósito
			{URL: "https://mirror.example/x.m3u8", Referrer: "https://ref/"},
		},
	}}
	p := providerConM3UYOpts(t, m3uUnCanal, WithEnriquecedor(e))

	got, err := p.GetStreamsDeCanal(context.Background(), "opensource-AndTV HD")
	if err != nil {
		t.Fatalf("GetStreamsDeCanal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d streams, quiero 2 (la duplicada se colapsa): %+v", len(got), got)
	}
	if got[0].URL != "https://delm3u.example/x.m3u8" {
		t.Errorf("got[0] = %q, la del M3U va primera", got[0].URL)
	}
	if got[1].URL != "https://mirror.example/x.m3u8" || got[1].Referrer != "https://ref/" {
		t.Errorf("got[1] = %+v, quiero el mirror con su referrer", got[1])
	}
}

// El feed discrimina también aquí: un canal @HD no recibe los streams del SD.
func TestGetStreamsDeCanalRespetaElFeed(t *testing.T) {
	e := &enriquecedorFalso{streams: map[string][]iptvorg.StreamExtra{
		"AndTV.in|SD": {{URL: "https://sd.example/x.m3u8"}},
	}}
	p := providerConM3UYOpts(t, m3uUnCanal, WithEnriquecedor(e))

	got, err := p.GetStreamsDeCanal(context.Background(), "opensource-AndTV HD")
	if err != nil {
		t.Fatalf("GetStreamsDeCanal: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %+v, el canal HD no debe heredar el stream SD", got)
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
