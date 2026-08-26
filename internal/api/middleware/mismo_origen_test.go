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
