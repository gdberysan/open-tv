package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/ports"
)

type registradorFalso struct {
	urls []string
	des  []ports.DesenlaceMirror
}

func (r *registradorFalso) RegistrarDesenlace(_ context.Context, url string, d ports.DesenlaceMirror) error {
	r.urls = append(r.urls, url)
	r.des = append(r.des, d)
	return nil
}

func postDesenlace(h *DesenlaceHandler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/streams/desenlace", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Post(rec, req)
	return rec
}

func TestDesenlaceHandlerRegistra(t *testing.T) {
	reg := &registradorFalso{}
	h := NewDesenlaceHandler(slog.New(slog.DiscardHandler), reg)
	rec := postDesenlace(h, `{"url":"http://o/x.m3u8","resultado":"iniciado","motivo":"","ms_primer_frame":2100.4}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	if len(reg.urls) != 1 || reg.urls[0] != "http://o/x.m3u8" || reg.des[0].Resultado != "iniciado" || reg.des[0].MsPrimerFrame != 2100 {
		t.Errorf("registrado: %v %+v", reg.urls, reg.des)
	}
}

func TestDesenlaceHandlerRechazaCuerposInvalidos(t *testing.T) {
	h := NewDesenlaceHandler(slog.New(slog.DiscardHandler), &registradorFalso{})
	for _, body := range []string{
		`no json`,
		`{"resultado":"fallo"}`,
		`{"url":"http://o/x.m3u8"}`,
		`{"url":"http://o/x.m3u8","resultado":"cortado"}`,
		`{"url":"http://o/x.m3u8","resultado":"fallo","motivo":"` + strings.Repeat("x", 5000) + `"}`,
	} {
		if rec := postDesenlace(h, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%.40s → %d, quiero 400", body, rec.Code)
		}
	}
}

func TestDesenlaceHandlerSinRegistradorEs503(t *testing.T) {
	h := NewDesenlaceHandler(slog.New(slog.DiscardHandler), nil)
	if rec := postDesenlace(h, `{"url":"http://o/x.m3u8","resultado":"fallo","motivo":"caido"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("código %d, quiero 503", rec.Code)
	}
}
