package proxy

// Tests de caja blanca: checkRedirect y controlConexion son las dos defensas
// nuevas contra el SSRF por redirección y el DNS-rebinding, y ninguna de las
// dos se puede forzar desde fuera del paquete sin montar un origen que
// resida fuera de loopback y redirija hacia dentro — algo que un test no
// puede hacer sin depender de red real. Llamarlas directamente prueba la
// lógica en sí, sin esa limitación.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckRedirectRechazaDestinoPrivado(t *testing.T) {
	h := &Handler{privadasOK: false}

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
	h := &Handler{privadasOK: true}
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
	h := &Handler{privadasOK: false}
	for _, addr := range []string{"127.0.0.1:9999", "169.254.169.254:80", "192.168.1.1:443", "[::1]:8080"} {
		if err := h.controlConexion("tcp", addr, nil); err == nil {
			t.Errorf("%s: debería rechazar la conexión, dejó pasar", addr)
		}
	}
}

func TestControlConexionPermiteIPPublica(t *testing.T) {
	h := &Handler{privadasOK: false}
	if err := h.controlConexion("tcp", "93.184.216.34:443", nil); err != nil {
		t.Errorf("una IP pública no debería bloquearse: %v", err)
	}
}

// Con privadasOK=true, controlConexion no debe rechazar 127.0.0.1: es
// exactamente la conexión que hacen los tests de relay contra httptest.Server.
func TestControlConexionPermiteConPrivadasOK(t *testing.T) {
	h := &Handler{privadasOK: true}
	if err := h.controlConexion("tcp", "127.0.0.1:9999", nil); err != nil {
		t.Errorf("con privadasOK=true no debería bloquear: %v", err)
	}
}

func TestControlConexionExigeDireccionResoluble(t *testing.T) {
	h := &Handler{privadasOK: false}
	if err := h.controlConexion("tcp", "no-es-host-puerto", nil); err == nil {
		t.Error("debería rechazar una dirección sin host:puerto")
	}
}
