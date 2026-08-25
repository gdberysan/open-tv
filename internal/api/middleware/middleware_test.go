package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gdberysan/open-tv/internal/api/middleware"
)

// El limitador es un semáforo global: al llenarse debe responder 429 en vez de
// encolar. Si esto regresa en silencio, la API se cuelga bajo carga en lugar de
// rechazar rápido.
func TestRateLimiterDevuelve429AlLlenarse(t *testing.T) {
	bloquear := make(chan struct{})
	dentro := make(chan struct{})

	h := middleware.RateLimiter(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dentro <- struct{}{}
		<-bloquear
	}))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()

	<-dentro // el único slot está ocupado

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, quiero 429 con el semáforo lleno", rec.Code)
	}

	close(bloquear)
	wg.Wait()
}

// El slot debe liberarse al terminar cada petición, o el gateway se degrada
// hasta rechazarlo todo tras las primeras N peticiones.
func TestRateLimiterLiberaElSlotAlTerminar(t *testing.T) {
	h := middleware.RateLimiter(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("petición %d: status = %d, quiero 200", i, rec.Code)
		}
	}
}

// Un panic en un handler no puede tumbar el proceso ni dejar al cliente sin
// respuesta, y debe quedar registrado para poder diagnosticarlo.
func TestRecoverConvierteElPanicEn500YLoRegistra(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := middleware.Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, quiero 500", rec.Code)
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("el panic debe quedar registrado; log:\n%s", buf.String())
	}
}

func TestRecoverNoInterfiereConRespuestasNormales(t *testing.T) {
	h := middleware.Recover(slog.New(slog.DiscardHandler))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte("ok"))
		}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTeapot || rec.Body.String() != "ok" {
		t.Errorf("status = %d, body = %q; quiero 418 / \"ok\"", rec.Code, rec.Body.String())
	}
}

// El logger estructurado es la única traza de cada petición: si deja de
// registrar método, ruta o status, los incidentes se diagnostican a ciegas.
func TestLoggerRegistraMetodoRutaYStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := middleware.Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/channels", nil))

	salida := buf.String()
	for _, quiero := range []string{`"method":"GET"`, `"path":"/channels"`, `"status":404`} {
		if !strings.Contains(salida, quiero) {
			t.Errorf("falta %s en el log; salida:\n%s", quiero, salida)
		}
	}
}

// El preflight OPTIONS debe cortarse en el middleware: si llegase al handler,
// respondería 405 y el navegador bloquearía la petición real.
func TestCORSCortaElPreflight(t *testing.T) {
	llamado := false
	h := middleware.CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamado = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/channels", nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, quiero 204", rec.Code)
	}
	if llamado {
		t.Error("el preflight no debe llegar al handler")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("falta la cabecera Access-Control-Allow-Origin")
	}
}
