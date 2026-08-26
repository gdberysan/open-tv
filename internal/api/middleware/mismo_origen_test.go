package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gdberysan/open-tv/internal/api/middleware"
)

func TestMismoOrigenRechazaHostAjeno(t *testing.T) {
	mw := middleware.MismoOrigen([]string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	casos := []struct {
		host   string
		quiero int
	}{
		{"127.0.0.1:8080", 200},
		{"localhost:8080", 200},
		{"evil.example:8080", http.StatusForbidden},
		{"127.0.0.1:9999", http.StatusForbidden},
	}
	for _, c := range casos {
		req := httptest.NewRequest(http.MethodGet, "http://x/channels", nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.quiero {
			t.Errorf("Host %q → %d, quiero %d", c.host, rec.Code, c.quiero)
		}
	}
}

func TestMismoOrigenVacioPasaTodo(t *testing.T) {
	mw := middleware.MismoOrigen(nil)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	req := httptest.NewRequest(http.MethodGet, "http://x/channels", nil)
	req.Host = "loquesea:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("con lista vacía debe pasar; código %d", rec.Code)
	}
}

// TestMismoOrigenSecFetchSiteEnMutantes cubre F1: un POST (u otro método
// mutante) con Sec-Fetch-Site cross-site se corta con 403 aunque el Host sea
// el correcto (o la lista de hosts esté vacía, como en los tests de este
// paquete): ese chequeo de Sec-Fetch-Site es CSRF, no DNS-rebinding, y no
// debe depender de si hostsPermitidos está configurada.
func TestMismoOrigenSecFetchSiteEnMutantes(t *testing.T) {
	mw := middleware.MismoOrigen(nil)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	casos := []struct {
		nombre string
		header string // "" = cabecera ausente
		quiero int
	}{
		{"ausente", "", 200},
		{"same-origin", "same-origin", 200},
		{"none", "none", 200},
		{"cross-site", "cross-site", http.StatusForbidden},
	}
	for _, c := range casos {
		req := httptest.NewRequest(http.MethodPost, "http://x/sources", nil)
		if c.header != "" {
			req.Header.Set("Sec-Fetch-Site", c.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.quiero {
			t.Errorf("Sec-Fetch-Site=%q (%s) → %d, quiero %d", c.header, c.nombre, rec.Code, c.quiero)
		}
	}
}

// TestMismoOrigenSecFetchSiteNoAfectaAGetHead confirma que el chequeo de
// Sec-Fetch-Site solo aplica a métodos mutantes: un GET/HEAD cross-site (la
// SPA cargando sus propios assets, o cualquier navegación normal) debe pasar
// igual, incluso con Sec-Fetch-Site: cross-site.
func TestMismoOrigenSecFetchSiteNoAfectaAGetHead(t *testing.T) {
	mw := middleware.MismoOrigen(nil)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	for _, metodo := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(metodo, "http://x/channels", nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("%s con Sec-Fetch-Site cross-site → %d, quiero 200 (no debe afectar a GET/HEAD)", metodo, rec.Code)
		}
	}
}
