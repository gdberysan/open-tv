package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/api/handlers"
	"github.com/gdberysan/open-tv/internal/stats"
)

func TestStatsHandlerRegistraYresume(t *testing.T) {
	h := handlers.NewStatsHandler(stats.NuevoAgregador(), nil /* sin agregado de catálogo en el test */)

	rec := httptest.NewRecorder()
	h.PostPlayback(rec, httptest.NewRequest(http.MethodPost, "/stats/playback",
		strings.NewReader(`{"canal_id":"c1","resultado":"iniciado","motor":"hlsjs","via":"proxy"}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST código %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	h.GetStats(rec2, httptest.NewRequest(http.MethodGet, "/stats", nil))
	var got map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &got)
	repro := got["reproduccion"].(map[string]any)
	if repro["iniciados"].(float64) != 1 {
		t.Errorf("iniciados = %v", repro["iniciados"])
	}
}

// Sin agregado de catálogo (nil, como en el test de arriba), /stats no debe
// reventar ni inventarse una clave "catalogo": simplemente no viene.
func TestStatsHandlerSinCatalogoNoRevienta(t *testing.T) {
	h := handlers.NewStatsHandler(stats.NuevoAgregador(), nil)

	rec := httptest.NewRecorder()
	h.GetStats(rec, httptest.NewRequest(http.MethodGet, "/stats", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET código %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if _, existe := got["catalogo"]; existe {
		t.Errorf("catalogo = %v, quiero que la clave no exista sin fuente de catálogo", got["catalogo"])
	}
}

// Un cuerpo por encima del límite (4 KiB) se rechaza: es la superficie de
// ataque más barata del endpoint, no hay razón para que un playback legítimo
// pese eso.
func TestStatsHandlerRechazaCuerpoGigante(t *testing.T) {
	h := handlers.NewStatsHandler(stats.NuevoAgregador(), nil)

	relleno := strings.Repeat("x", 5<<10)
	cuerpo := `{"canal_id":"c1","resultado":"iniciado","motivo":"` + relleno + `"}`
	rec := httptest.NewRecorder()
	h.PostPlayback(rec, httptest.NewRequest(http.MethodPost, "/stats/playback", strings.NewReader(cuerpo)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("código = %d, quiero 400 con cuerpo > 4KiB", rec.Code)
	}
}

// canal_id y resultado son los dos campos que el contrato marca como
// obligatorios (Tarea 14): sin ellos no hay nada que clasificar.
func TestStatsHandlerRechazaCamposObligatoriosFaltantes(t *testing.T) {
	h := handlers.NewStatsHandler(stats.NuevoAgregador(), nil)

	rec := httptest.NewRecorder()
	h.PostPlayback(rec, httptest.NewRequest(http.MethodPost, "/stats/playback",
		strings.NewReader(`{"motor":"hlsjs"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("código = %d, quiero 400 sin canal_id/resultado", rec.Code)
	}
}

// Bug real cazado en el gate en Chrome (2026-08-26): el cliente manda
// ms_primer_frame FRACCIONARIO (performance.now() devuelve milisegundos con
// decimales) y el decode a int lo rechazaba con 400 — TODOS los desenlaces
// 'iniciado' reales se perdían en silencio y /stats quedaba sesgado hacia el
// fallo. Un número JSON es un número: se acepta y se redondea.
func TestStatsHandlerAceptaMsPrimerFrameFraccionario(t *testing.T) {
	h := handlers.NewStatsHandler(stats.NuevoAgregador(), nil)

	rec := httptest.NewRecorder()
	h.PostPlayback(rec, httptest.NewRequest(http.MethodPost, "/stats/playback",
		strings.NewReader(`{"canal_id":"c1","resultado":"iniciado","motor":"hlsjs","via":"proxy","ms_primer_frame":3128.5}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST con ms_primer_frame fraccionario: código %d, se esperaba 204", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	h.GetStats(rec2, httptest.NewRequest(http.MethodGet, "/stats", nil))
	var got map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &got)
	repro := got["reproduccion"].(map[string]any)
	if repro["iniciados"].(float64) != 1 {
		t.Errorf("iniciados = %v, el desenlace fraccionario no se registró", repro["iniciados"])
	}
}
