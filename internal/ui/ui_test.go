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
