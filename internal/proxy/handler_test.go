package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/proxy"
)

func TestProxyReescribeElManifiestoQueRelaya(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sin ACAO: es justo el caso por el que existe el proxy.
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:6.0,\nseg1.ts\n"))
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/live.m3u8"), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "mpegurl") {
		t.Errorf("Content-Type = %q", ct)
	}
	quiero := "/proxy/hls?u=" + url.QueryEscape(origen.URL+"/seg1.ts")
	if !strings.Contains(rec.Body.String(), quiero) {
		t.Errorf("el segmento no se reescribió:\n%s", rec.Body.String())
	}
}

func TestProxyRelayaSegmentosYReenviaRange(t *testing.T) {
	var rangeVisto string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeVisto = r.Header.Get("Range")
		// El proxy no puede reenviar cabeceras del cliente a ciegas.
		if r.Header.Get("Origin") != "" || r.Header.Get("Referer") != "" {
			t.Error("el proxy reenvió Origin/Referer del cliente")
		}
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("bytes-de-video"))
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	req := httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg1.ts"), nil)
	req.Header.Set("Range", "bytes=0-1023")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Referer", "http://127.0.0.1:8080/")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	if rangeVisto != "bytes=0-1023" {
		t.Errorf("Range reenviado = %q, quiero bytes=0-1023", rangeVisto)
	}
	if rec.Body.String() != "bytes-de-video" {
		t.Errorf("cuerpo = %q", rec.Body.String())
	}
}

// El proxy escucha en loopback, así que solo lo alcanza esta máquina — pero
// eso no lo convierte en un pasadizo hacia el router de casa.
func TestProxyRechazaDestinosPrivados(t *testing.T) {
	h := proxy.NewHandler("/proxy/hls?u=", false)
	for _, destino := range []string{
		"http://127.0.0.1:8080/health",
		"http://192.168.1.1/",
		"http://[::1]:8080/",
		"file:///etc/passwd",
		"http://169.254.169.254/latest/meta-data/",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
			"/proxy/hls?u="+url.QueryEscape(destino), nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s → %d, quiero 403", destino, rec.Code)
		}
	}
}

func TestProxySinParametroU(t *testing.T) {
	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("código %d, quiero 400", rec.Code)
	}
}

func TestProxyPropagaElErrorDelOrigen(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/live.m3u8"), nil))

	if rec.Code != http.StatusForbidden {
		t.Errorf("código %d, quiero 403 (el del origen)", rec.Code)
	}
}

// El proxy no puede convertirse en un descargador infinito.
func TestProxyCortaSegmentosGigantes(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		bloque := make([]byte, 1<<20)
		for i := 0; i < 60; i++ { // 60 MB > el tope de 50
			if _, err := w.Write(bloque); err != nil {
				return
			}
		}
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil))

	if n := int64(rec.Body.Len()); n > proxy.MaxSegmentoBytes {
		t.Errorf("relayó %d bytes, el tope es %d", n, proxy.MaxSegmentoBytes)
	}
}

func TestProxyNoAnunciaCORS(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = io.WriteString(w, "#EXTM3U\n")
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/live.m3u8"), nil))

	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("el proxy copió el ACAO del origen (%q); la UI es del mismo origen y no lo necesita", v)
	}
}
