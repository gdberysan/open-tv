package proxy

// Tests de caja blanca: checkRedirect y controlConexion son las dos defensas
// nuevas contra el SSRF por redirección y el DNS-rebinding, y ninguna de las
// dos se puede forzar desde fuera del paquete sin montar un origen que
// resida fuera de loopback y redirija hacia dentro — algo que un test no
// puede hacer sin depender de red real. Llamarlas directamente prueba la
// lógica en sí, sin esa limitación.

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCheckRedirectRechazaDestinoPrivado(t *testing.T) {
	h := guardiaRed{privadasOK: false}

	casos := []struct {
		nombre      string
		req         *http.Request
		via         []*http.Request
		quieroError bool
	}{
		{
			nombre:      "demasiadas redirecciones",
			req:         httptest.NewRequest(http.MethodGet, "http://93.184.216.34/", nil),
			via:         make([]*http.Request, maxRedirecciones),
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

	for _, c := range casos {
		err := h.checkRedirect(c.req, c.via)
		if (err != nil) != c.quieroError {
			t.Errorf("%s: err = %v, quiero error=%v", c.nombre, err, c.quieroError)
		}
	}
}

// Con privadasOK=true (solo en tests de relay) checkRedirect no debe bloquear
// destinos privados: es la misma vía de escape que necesita ServeHTTP para
// poder testear contra httptest.Server.
func TestCheckRedirectPermiteDestinoPrivadoConPrivadasOK(t *testing.T) {
	h := guardiaRed{privadasOK: true}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9/", nil)
	if err := h.checkRedirect(req, nil); err != nil {
		t.Errorf("con privadasOK=true no debería bloquear: %v", err)
	}
}

// controlConexion es lo que de verdad cierra el DNS-rebinding: se ejecuta con
// la IP YA resuelta por el Transport, justo antes de que el kernel abra la
// conexión — el mismo camino que recorre cada conexión real del proxy,
// incluidas las de una redirección.
func TestControlConexionRechazaIPPrivada(t *testing.T) {
	h := guardiaRed{privadasOK: false}
	for _, addr := range []string{"127.0.0.1:9999", "169.254.169.254:80", "192.168.1.1:443", "[::1]:8080"} {
		if err := h.controlConexion("tcp", addr, nil); err == nil {
			t.Errorf("%s: debería rechazar la conexión, dejó pasar", addr)
		}
	}
}

func TestControlConexionPermiteIPPublica(t *testing.T) {
	h := guardiaRed{privadasOK: false}
	if err := h.controlConexion("tcp", "93.184.216.34:443", nil); err != nil {
		t.Errorf("una IP pública no debería bloquearse: %v", err)
	}
}

// Con privadasOK=true, controlConexion no debe rechazar 127.0.0.1: es
// exactamente la conexión que hacen los tests de relay contra httptest.Server.
func TestControlConexionPermiteConPrivadasOK(t *testing.T) {
	h := guardiaRed{privadasOK: true}
	if err := h.controlConexion("tcp", "127.0.0.1:9999", nil); err != nil {
		t.Errorf("con privadasOK=true no debería bloquear: %v", err)
	}
}

func TestControlConexionExigeDireccionResoluble(t *testing.T) {
	h := guardiaRed{privadasOK: false}
	if err := h.controlConexion("tcp", "no-es-host-puerto", nil); err == nil {
		t.Error("debería rechazar una dirección sin host:puerto")
	}
}

func TestIpPrivadaBloqueaCGNATyBenchmark(t *testing.T) {
	for _, ip := range []string{"100.64.0.1", "100.127.255.254", "198.18.0.1", "198.19.255.254"} {
		if !ipPrivada(net.ParseIP(ip)) {
			t.Errorf("ipPrivada(%s) = false, quiero true", ip)
		}
	}
	if ipPrivada(net.ParseIP("1.1.1.1")) {
		t.Error("ipPrivada(1.1.1.1) = true, quiero false")
	}
}

// Un origen con protección de hotlink solo sirve si le llega el Referer que
// espera. El navegador NO puede ponerlo (Referer es cabecera prohibida para
// fetch/XHR), así que es el proxy quien tiene que hacerlo. Esta es también la
// rama de producción de verdad: el router SIEMPRE monta un BuscadorCabeceras
// (nunca nil), así que la invariante "las cabeceras del cliente no viajan"
// tiene que probarse AQUÍ, con un buscador no-nil y el cliente mandando su
// propio Origin/Referer/Cookie — no solo en la rama h.cabeceras == nil, que
// en producción no se ejecuta nunca.
func TestProxyMandaLasCabecerasDelStream(t *testing.T) {
	var gotRef, gotUA, gotCookie string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRef = r.Header.Get("Referer")
		gotUA = r.Header.Get("User-Agent")
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("datos"))
	}))
	defer origen.Close()

	h := NewHandler("/proxy/hls?u=", true, ConBuscadorCabeceras(
		func(_ context.Context, _ string) (string, string) {
			return "https://ref.example/", "UA-Especial/1"
		}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil)
	// Cabeceras del CLIENTE: si algo dentro de la rama h.cabeceras != nil las
	// leyera y las reenviara (aunque sea como fallback), este test lo pilla.
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Referer", "http://cliente-malicioso.example/")
	req.Header.Set("Cookie", "sesion=secreta")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, quiero 200", rec.Code)
	}
	if gotRef != "https://ref.example/" {
		t.Errorf("Referer = %q, quiero el del stream (NUNCA el que mandó el cliente)", gotRef)
	}
	if gotUA != "UA-Especial/1" {
		t.Errorf("User-Agent = %q, quiero el del stream", gotUA)
	}
	if gotCookie != "" {
		t.Errorf("Cookie = %q, quiero ninguna: la del cliente nunca debe viajar al origen", gotCookie)
	}
}

// Sin cabeceras propias se manda el User-Agent de siempre y ningún Referer:
// mandar un Referer inventado podría romper orígenes que hoy funcionan.
func TestProxySinCabecerasUsaElDeSiempre(t *testing.T) {
	var gotRef, gotUA string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRef = r.Header.Get("Referer")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("datos"))
	}))
	defer origen.Close()

	h := NewHandler("/proxy/hls?u=", true, ConBuscadorCabeceras(
		func(_ context.Context, _ string) (string, string) {
			return "", ""
		}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil)
	h.ServeHTTP(rec, req)

	if gotUA != userAgent {
		t.Errorf("User-Agent = %q, quiero el de siempre (%q)", gotUA, userAgent)
	}
	if gotRef != "" {
		t.Errorf("Referer = %q, quiero ninguno", gotRef)
	}
}

// Un buscador nil (o un fallo de DB, que se traduce en cadenas vacías) no puede
// romper el proxy: se cae al comportamiento anterior.
func TestProxyBuscadorNil(t *testing.T) {
	var gotUA string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("datos"))
	}))
	defer origen.Close()

	h := NewHandler("/proxy/hls?u=", true) // sin opciones: como todas las llamadas de siempre
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, quiero 200: un buscador nil no puede romper el relay", rec.Code)
	}
	if gotUA != userAgent {
		t.Errorf("User-Agent = %q, quiero el de siempre", gotUA)
	}
}
