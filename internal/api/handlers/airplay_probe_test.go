package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

const masterOK = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS=\"avc1.4d4028,mp4a.40.2\"\n720p.m3u8\n"

func TestProberClasificaYCachea(t *testing.T) {
	var peticiones atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		peticiones.Add(1)
		_, _ = w.Write([]byte(masterOK))
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10, true)
	url := srv.URL + "/a.m3u8"

	if got := p.Veredicto(context.Background(), url); got != domain.AirplayOK {
		t.Fatalf("primer veredicto = %v, quiero AirplayOK", got)
	}
	if got := p.Veredicto(context.Background(), url); got != domain.AirplayOK {
		t.Fatalf("segundo veredicto = %v, quiero AirplayOK", got)
	}
	if n := peticiones.Load(); n != 1 {
		t.Errorf("el origen recibió %d peticiones, quiero 1: la caché no está sirviendo", n)
	}
}

func TestProberDevuelveDesconocidoSiElOrigenFalla(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10, true)
	if got := p.Veredicto(context.Background(), srv.URL+"/a.m3u8"); got != domain.AirplayUnknown {
		t.Errorf("veredicto = %v, quiero AirplayUnknown", got)
	}
}

func TestProberNoSondeaLoQueLaURLYaDescarta(t *testing.T) {
	var peticiones atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		peticiones.Add(1)
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10, true)
	if got := p.Veredicto(context.Background(), srv.URL+"/a.mpd"); got != domain.AirplayNo {
		t.Errorf("veredicto = %v, quiero AirplayNo", got)
	}
	if n := peticiones.Load(); n != 0 {
		t.Errorf("el origen recibió %d peticiones, quiero 0: DASH se descarta sin red", n)
	}
}

func TestProberRespetaElTopeDeEntradas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(masterOK))
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 2, true)
	for _, s := range []string{"/a.m3u8", "/b.m3u8", "/c.m3u8"} {
		p.Veredicto(context.Background(), srv.URL+s)
	}
	if n := p.entradas(); n > 2 {
		t.Errorf("la caché guarda %d entradas, el tope es 2", n)
	}
}

// SSRF por redirección: un origen puede pasar la comprobación de destino
// inicial (sondear) y luego responder 302 hacia un destino privado — la LAN,
// o el enlace-local de metadatos de una nube —, y el http.Client por
// defecto lo seguiría sin preguntar si nadie revisa cada salto. checkRedirect
// es esa revisión; se prueba en caja blanca (igual que
// internal/proxy/handler_internal_test.go: TestCheckRedirectRechazaDestinoPrivado)
// porque montar un origen "público" de verdad no es algo que un test pueda
// hacer sin red real.
func TestAirplayCheckRedirectRechazaDestinoPrivado(t *testing.T) {
	casos := []struct {
		nombre      string
		req         *http.Request
		via         []*http.Request
		quieroError bool
	}{
		{
			nombre:      "demasiadas redirecciones",
			req:         httptest.NewRequest(http.MethodGet, "http://93.184.216.34/", nil),
			via:         make([]*http.Request, maxRedireccionesSondeo),
			quieroError: true,
		},
		{
			nombre:      "esquema no permitido",
			req:         httptest.NewRequest(http.MethodGet, "file:///etc/passwd", nil),
			quieroError: true,
		},
		{
			nombre:      "destino privado (loopback)",
			req:         httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9/", nil),
			quieroError: true,
		},
		{
			nombre:      "destino privado (metadatos de nube)",
			req:         httptest.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil),
			quieroError: true,
		},
		{
			nombre:      "destino público, dentro del límite de saltos",
			req:         httptest.NewRequest(http.MethodGet, "http://93.184.216.34/", nil),
			quieroError: false,
		},
	}

	p := &AirplayProber{privadasOK: false}
	for _, c := range casos {
		if err := p.checkRedirect(c.req, c.via); (err != nil) != c.quieroError {
			t.Errorf("%s: err = %v, quiero error=%v", c.nombre, err, c.quieroError)
		}
	}

	// privadasOK=true (solo tests reales) tiene que dejar pasar lo que
	// privadasOK=false bloquea, o los tests que sondean httptest (loopback)
	// con permitirDestinosPrivados=true se romperían en cuanto usaran el
	// cliente por defecto.
	pPrivadasOK := &AirplayProber{privadasOK: true}
	loopback := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9/", nil)
	if err := pPrivadasOK.checkRedirect(loopback, nil); err != nil {
		t.Errorf("privadasOK=true: checkRedirect rechazó loopback, err = %v", err)
	}
}

// Prueba de cableado: NewAirplayProber(nil, ...) —el camino real que usa
// producción, ver internal/api/router.go— tiene que instalar checkRedirect
// en el cliente por defecto. Si esto se rompe, sondear vuelve a ser
// vulnerable a SSRF por redirección aunque TestAirplayCheckRedirectRechazaDestinoPrivado
// siga en verde, porque esa prueba llama al método directamente.
func TestNewAirplayProberClienteDefaultInstalaCheckRedirect(t *testing.T) {
	p := NewAirplayProber(nil, time.Hour, 10, false)
	if p.client.CheckRedirect == nil {
		t.Fatal("el cliente por defecto no tiene CheckRedirect: las redirecciones se seguirían sin validar")
	}
	req := httptest.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	if err := p.client.CheckRedirect(req, nil); err == nil {
		t.Error("CheckRedirect del cliente por defecto no rechazó un destino privado")
	}
}
