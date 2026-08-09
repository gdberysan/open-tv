package validator_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/adapters/validator"
)

// Los orígenes IPTV rotos responden a un HEAD escribiendo cuerpo igualmente.
// Con keep-alive, esa conexión vuelve al pool con bytes sin leer y el
// transporte de Go escupe "Unsolicited response received on idle HTTP channel"
// por el paquete log global, saltándose el handler JSON de slog.
func TestCheckerNoReutilizaConexionesInactivas(t *testing.T) {
	c := validator.NewChecker(nil, time.Second)
	tr := c.Transport()
	if tr == nil {
		t.Fatal("el checker debe exponer su transporte propio")
	}
	if !tr.DisableKeepAlives {
		t.Error("DisableKeepAlives debe estar activo: sin pool de inactivas no hay peek fallido")
	}
}

func TestCheckerMandaUserAgentDeReproductor(t *testing.T) {
	var recibido string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recibido = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := validator.NewChecker(nil, 2*time.Second)
	res := c.Check(context.Background(), srv.URL+"/stream.m3u8")
	if !res.IsAlive {
		t.Fatalf("el stream de prueba debería dar vivo: %v", res.Error)
	}
	if recibido == "" || strings.HasPrefix(recibido, "Go-http-client") {
		t.Errorf("User-Agent = %q; algunos orígenes filtran el default de Go", recibido)
	}
}
