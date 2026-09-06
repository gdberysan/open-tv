package proxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/proxy"
)

// httptest vive en 127.0.0.1: con la guardia activa NO se llega ni a abrir
// la conexión; con permitirDestinosPrivados=true (solo tests) sí.
func TestNuevoClienteGuardadoBloqueaLoopbackSalvoEnTests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/x.ts", nil)

	if resp, err := proxy.NuevoClienteGuardado(false).Do(req); err == nil {
		_ = resp.Body.Close()
		t.Fatal("con la guardia activa, loopback debe bloquearse antes de conectar")
	} else if !strings.Contains(err.Error(), "destino privado") {
		t.Errorf("el error debe nombrar la causa: %v", err)
	}

	resp, err := proxy.NuevoClienteGuardado(true).Do(req)
	if err != nil {
		t.Fatalf("con permitirDestinosPrivados=true debe pasar: %v", err)
	}
	_ = resp.Body.Close()
}

func TestNuevoClienteGuardadoSinKeepAliveYConTopePorHost(t *testing.T) {
	tr, ok := proxy.NuevoClienteGuardado(false).Transport.(*http.Transport)
	if !ok {
		t.Fatal("el transporte debe ser *http.Transport")
	}
	if !tr.DisableKeepAlives || tr.MaxConnsPerHost != 4 {
		t.Errorf("DisableKeepAlives=%v MaxConnsPerHost=%d; quiero true y 4", tr.DisableKeepAlives, tr.MaxConnsPerHost)
	}
}

// La sonda de códecs necesita un tope MENOR que el del proxy/manifiesto: los
// dos comparten el mismo host y sumados no deben superar el presupuesto que
// evita los falsos muertos por limit_conn de nginx.
func TestNuevoClienteGuardadoConTopePersonalizado(t *testing.T) {
	tr, ok := proxy.NuevoClienteGuardadoConTope(false, 2).Transport.(*http.Transport)
	if !ok {
		t.Fatal("el transporte debe ser *http.Transport")
	}
	if tr.MaxConnsPerHost != 2 {
		t.Errorf("MaxConnsPerHost = %d; quiero 2", tr.MaxConnsPerHost)
	}
}
