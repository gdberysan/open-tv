package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstanciaVivaReconoceOtroOpenTV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","db":"ok","web_ui":true}`))
	}))
	defer srv.Close()

	if !InstanciaViva(context.Background(), srv.URL) {
		t.Error("un /health de Open TV debe reconocerse como instancia viva")
	}
}

// Cualquier otra cosa escuchando en el puerto NO es Open TV. Abrir el
// navegador contra el panel de otro programa sería peor que fallar.
func TestInstanciaVivaRechazaOtroServicio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Grafana"))
	}))
	defer srv.Close()

	if InstanciaViva(context.Background(), srv.URL) {
		t.Error("un servicio ajeno no puede pasar por Open TV")
	}
}

func TestInstanciaVivaConPuertoMuerto(t *testing.T) {
	if InstanciaViva(context.Background(), "http://127.0.0.1:1") {
		t.Error("un puerto que no contesta no es una instancia viva")
	}
}
