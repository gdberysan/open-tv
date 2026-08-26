package ui_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/ui"
)

func TestHandlerSirveIndexEnLaRaiz(t *testing.T) {
	h, ok := ui.Handler()
	if !ok {
		t.Skip("no hay cliente construido en internal/ui/dist")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	// index.html NUNCA se cachea: es lo que apunta a los assets con hash, y
	// una copia vieja deja al usuario en una versión anterior sin saberlo.
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("Cache-Control de index = %q, quiero no-cache", cc)
	}
}

// Fallback SPA: una ruta del cliente que el servidor no conoce devuelve el
// index, no un 404. Sin esto, recargar en /canal/x rompe la app.
func TestHandlerFallbackSPA(t *testing.T) {
	h, ok := ui.Handler()
	if !ok {
		t.Skip("no hay cliente construido en internal/ui/dist")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/canal/bbc-one", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("código %d, quiero 200 con el index", rec.Code)
	}
}

// La CSP y las cabeceras de endurecimiento (hallazgo M1) van en TODA respuesta
// del cliente. Lo que de verdad protege es script-src 'self' SIN 'unsafe-inline':
// si alguien afloja eso, el vector de XSS se reabre y este test debe fallar. Se
// comprueba también que img-src sigue permitiendo hosts externos (los logos FTA
// vienen de cualquier sitio; una CSP que los bloquee rompería la app).
func TestHandlerCabecerasSeguridad(t *testing.T) {
	h, ok := ui.Handler()
	if !ok {
		t.Skip("no hay cliente construido en internal/ui/dist")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("falta Content-Security-Policy")
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP sin script-src 'self': %q", csp)
	}
	// La protección clave: ningún script inline/externo ajeno. Extraemos la
	// directiva script-src y exigimos que NO lleve 'unsafe-inline'.
	for _, dir := range strings.Split(csp, ";") {
		dir = strings.TrimSpace(dir)
		if strings.HasPrefix(dir, "script-src") && strings.Contains(dir, "unsafe-inline") {
			t.Errorf("script-src no debe llevar 'unsafe-inline': %q", dir)
		}
	}
	if !strings.Contains(csp, "object-src 'none'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP sin object-src/frame-ancestors 'none': %q", csp)
	}
	// Los logos FTA vienen de cualquier host: img-src debe permitir externos.
	if !strings.Contains(csp, "img-src") || !strings.Contains(csp, "https:") {
		t.Errorf("CSP debe permitir imágenes externas (logos): %q", csp)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, quiero nosniff", got)
	}
}

// Un fichero con extensión que no existe SÍ es 404: devolver el index para
// /favicon.ico o /assets/roto.js convierte errores de red en HTML silencioso.
func TestHandlerFicheroInexistenteEs404(t *testing.T) {
	h, ok := ui.Handler()
	if !ok {
		t.Skip("no hay cliente construido en internal/ui/dist")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/no-existe.js", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("código %d, quiero 404", rec.Code)
	}
}
