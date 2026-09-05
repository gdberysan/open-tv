package iptvorg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// servidorFixtures sirve el JSON de testdata en la ruta que el Enriquecedor
// pide, para poder probar Cargar() sin salir a la red.
func servidorFixtures(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fichero string
		switch r.URL.Path {
		case "/api/streams.json":
			fichero = "testdata/streams.json"
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		b, err := os.ReadFile(fichero) //nolint:gosec
		if err != nil {
			t.Errorf("leyendo %s: %v", fichero, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
}

func cargado(t *testing.T) *Enriquecedor {
	t.Helper()
	srv := servidorFixtures(t)
	t.Cleanup(srv.Close)
	e := NuevoEnriquecedor(srv.Client())
	e.urlStreams = srv.URL + "/api/streams.json"
	if err := e.Cargar(context.Background()); err != nil {
		t.Fatalf("Cargar: %v", err)
	}
	return e
}

func TestSepararTvgID(t *testing.T) {
	casos := []struct{ in, canal, feed string }{
		{"AndTV.in@HD", "AndTV.in", "HD"},
		{"Solo.uk", "Solo.uk", ""},
		{"", "", ""},
		{"Raro.es@A@B", "Raro.es", "A@B"}, // solo se parte por el PRIMER @
	}
	for _, c := range casos {
		canal, feed := SepararTvgID(c.in)
		if canal != c.canal || feed != c.feed {
			t.Errorf("SepararTvgID(%q) = (%q,%q), quiero (%q,%q)", c.in, canal, feed, c.canal, c.feed)
		}
	}
}

// El feed DISCRIMINA: pedir HD no puede devolver el stream del SD. Es la trampa
// que el spec documenta (emparejar solo por canal mezcla calidades).
func TestStreamsRespetaElFeed(t *testing.T) {
	e := cargado(t)

	hd := e.Streams("AndTV.in", "HD")
	if len(hd) != 2 {
		t.Fatalf("HD: %d streams, quiero 2", len(hd))
	}
	if hd[0].URL != "https://uno.example/hd.m3u8" {
		t.Errorf("HD[0] = %q, quiero conservar el orden del JSON", hd[0].URL)
	}
	if hd[1].Referrer != "https://ref.example/" || hd[1].UserAgent != "VLC/3.0" {
		t.Errorf("HD[1] perdió las cabeceras: %+v", hd[1])
	}

	sd := e.Streams("AndTV.in", "SD")
	if len(sd) != 1 || sd[0].URL != "https://tres.example/sd.m3u8" {
		t.Errorf("SD = %+v, quiero solo el stream SD", sd)
	}
}

func TestStreamsFeedVacioYDesconocido(t *testing.T) {
	e := cargado(t)

	if got := e.Streams("Solo.uk", ""); len(got) != 1 {
		t.Errorf("feed vacío: %d streams, quiero 1", len(got))
	}
	if got := e.Streams("NoExiste.xx", "HD"); got != nil {
		t.Errorf("canal desconocido devolvió %+v, quiero nil", got)
	}
}

// Un stream sin 'channel' no se puede ligar a nada: se descarta en el índice.
func TestStreamsSinCanalSeDescartan(t *testing.T) {
	e := cargado(t)
	if n := e.totalIndexados(); n != 4 {
		t.Errorf("indexados %d, quiero 4 (el huérfano no cuenta)", n)
	}
}
