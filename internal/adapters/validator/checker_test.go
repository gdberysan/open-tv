package validator_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/validator"
	"github.com/gdberysan/open-tv/internal/domain"
)

// Los orígenes IPTV rotos responden a un HEAD escribiendo cuerpo igualmente.
// Con keep-alive, esa conexión vuelve al pool con bytes sin leer y el
// transporte de Go escupe "Unsolicited response received on idle HTTP channel"
// por el paquete log global, saltándose el handler JSON de slog.
func TestCheckerNoReutilizaConexionesInactivas(t *testing.T) {
	c := validator.NewChecker(nil, time.Second)
	tr := c.Transport()
	if tr == nil {
		t.Fatal("el checker debe exponer su transporte propio")
	}
	if !tr.DisableKeepAlives {
		t.Error("DisableKeepAlives debe estar activo: sin pool de inactivas no hay peek fallido")
	}
}

func TestCheckerMandaUserAgentDeReproductor(t *testing.T) {
	var recibido string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recibido = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := validator.NewChecker(nil, 2*time.Second)
	res := c.Check(context.Background(), srv.URL+"/stream.m3u8")
	if !res.IsAlive {
		t.Fatalf("el stream de prueba debería dar vivo: %v", res.Error)
	}
	if recibido == "" || strings.HasPrefix(recibido, "Go-http-client") {
		t.Errorf("User-Agent = %q; algunos orígenes filtran el default de Go", recibido)
	}
}

// HLS va directo a GET: el checker nunca manda un HEAD sobre esta URL, así
// que el único camino posible es el que trae cuerpo que clasificar.
func TestCheckClasificaAirplayEnElFallbackGET(t *testing.T) {
	const master = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS=\"avc1.4d4028,mp4a.40.2\"\n720p.m3u8\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			t.Error("no debe haber HEAD sobre una URL HLS")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(master))
	}))
	defer srv.Close()

	c := validator.NewChecker(nil, 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/a.m3u8")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, quiero true")
	}
	if res.Airplay != domain.AirplayOK {
		t.Errorf("Airplay = %v, quiero AirplayOK", res.Airplay)
	}
}

// Lo no-HLS conserva HEAD→GET: si el HEAD ya basta (2xx), no se manda ningún
// GET, y sin GET no hay cuerpo que clasificar — Airplay se queda en
// Unknown en vez de inventar un veredicto.
func TestCheckDejaAirplayDesconocidoSiElHEADBasta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			t.Error("el HEAD ya basta: no debería haber ningún GET")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := validator.NewChecker(nil, 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/a.ts")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, quiero true")
	}
	if res.Airplay != domain.AirplayUnknown {
		t.Errorf("Airplay = %v, quiero AirplayUnknown", res.Airplay)
	}
}

// El veredicto web sale del MISMO GET que decide si el stream está vivo. Si
// alguien reintroduce el HEAD para HLS, este test cae: el cuerpo no llega y
// no hay veredicto.
func TestCheckClasificaWebEnHLSSinPeticionExtra(t *testing.T) {
	var peticiones int32
	var vioOrigin atomic.Bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&peticiones, 1)
		if r.Header.Get("Origin") != "" {
			vioOrigin.Store(true)
		}
		if r.Method == http.MethodHead {
			t.Error("no debe haber HEAD sobre una URL HLS")
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f,mp4a.40.2\"\nchunk.m3u8\n"))
	}))
	defer srv.Close()

	c := validator.NewChecker(srv.Client(), 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/live.m3u8")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, error = %v", res.Error)
	}
	if res.Web != domain.WebOK {
		t.Errorf("Web = %v, quiero WebOK", res.Web)
	}
	if n := atomic.LoadInt32(&peticiones); n != 1 {
		t.Errorf("%d peticiones, quiero exactamente 1", n)
	}
	if !vioOrigin.Load() {
		t.Error("el checker debe mandar Origin para que los orígenes que lo reflejan contesten un ACAO reconocible")
	}
}

// Sin ACAO el stream sigue VIVO —el health-check no cambia— pero no es
// reproducible desde una página. Los dos veredictos son independientes.
func TestCheckVivoPeroNoWebSinCORS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f\"\nchunk.m3u8\n"))
	}))
	defer srv.Close()

	c := validator.NewChecker(srv.Client(), 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/live.m3u8")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, error = %v", res.Error)
	}
	if res.Web != domain.WebNo {
		t.Errorf("Web = %v, quiero WebNo", res.Web)
	}
}

// Un origen inalcanzable deja el veredicto en Unknown, nunca en No: "no pude
// preguntar" y "pregunté y no" son cosas distintas y la DB las distingue.
func TestCheckOrigenCaidoDejaWebUnknown(t *testing.T) {
	c := validator.NewChecker(nil, 500*time.Millisecond)
	res := c.Check(context.Background(), "http://127.0.0.1:1/live.m3u8")

	if res.IsAlive {
		t.Error("IsAlive = true sobre un puerto muerto")
	}
	if res.Web != domain.WebUnknown {
		t.Errorf("Web = %v, quiero WebUnknown", res.Web)
	}
}

// Lo no-HLS conserva HEAD→GET; el veredicto sale de las cabeceras del HEAD.
func TestCheckNoHLSClasificaDesdeElHEAD(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := validator.NewChecker(srv.Client(), 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/canal.ts")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, error = %v", res.Error)
	}
	if res.Web != domain.WebOK {
		t.Errorf("Web = %v, quiero WebOK", res.Web)
	}
}
