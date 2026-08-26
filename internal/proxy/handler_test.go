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

// Una página de otro sitio no puede usar el proxy como relay: Sec-Fetch-Site
// lo manda el navegador y el atacante no puede falsificarlo desde JS.
func TestProxyRechazaSecFetchSiteCruzado(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("bytes-de-video"))
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)

	for _, c := range []struct {
		site   string
		quiero int
	}{
		{"cross-site", http.StatusForbidden},
		{"same-site", http.StatusForbidden},
		{"same-origin", http.StatusOK},
		{"none", http.StatusOK},
		{"", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodGet,
			"/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg1.ts"), nil)
		if c.site != "" {
			req.Header.Set("Sec-Fetch-Site", c.site)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.quiero {
			t.Errorf("Sec-Fetch-Site %q → %d, quiero %d", c.site, rec.Code, c.quiero)
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

// SSRF por redirección: un origen puede responder 302 hacia un destino
// privado (la LAN, el enlace-local de metadatos de una nube) y el
// http.Client por defecto lo sigue sin preguntar. Aquí origen e interno viven
// los dos en 127.0.0.1 —es lo único que un test puede montar sin red real—
// así que privadasOK=false ya bloquea la petición al origen antes incluso de
// llegar a la redirección; TestCheckRedirectRechazaDestinoPrivado, en
// handler_internal_test.go, fuerza el camino de checkRedirect en concreto,
// sin depender de que un origen "público" viva fuera de loopback.
func TestProxyNoSigueRedireccionesHaciaDestinosPrivados(t *testing.T) {
	interno := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("el proxy llegó a contactar al servidor interno: la redirección no se bloqueó")
		_, _ = w.Write([]byte("SECRETO-INTERNO"))
	}))
	defer interno.Close()

	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, interno.URL+"/secreto", http.StatusFound)
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/live.m3u8"), nil))

	if rec.Code == http.StatusOK {
		t.Errorf("código %d: relayó algo cuando debía bloquear", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "SECRETO-INTERNO") {
		t.Errorf("el cuerpo del servidor interno se filtró: %s", rec.Body.String())
	}
}
