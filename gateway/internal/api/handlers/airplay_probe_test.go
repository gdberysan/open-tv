package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
)

const masterOK = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS=\"avc1.4d4028,mp4a.40.2\"\n720p.m3u8\n"

func TestProberClasificaYCachea(t *testing.T) {
	var peticiones atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		peticiones.Add(1)
		_, _ = w.Write([]byte(masterOK))
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10)
	url := srv.URL + "/a.m3u8"

	if got := p.Veredicto(context.Background(), url); got != domain.AirplayOK {
		t.Fatalf("primer veredicto = %v, quiero AirplayOK", got)
	}
	if got := p.Veredicto(context.Background(), url); got != domain.AirplayOK {
		t.Fatalf("segundo veredicto = %v, quiero AirplayOK", got)
	}
	if n := peticiones.Load(); n != 1 {
		t.Errorf("el origen recibió %d peticiones, quiero 1: la caché no está sirviendo", n)
	}
}

func TestProberDevuelveDesconocidoSiElOrigenFalla(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10)
	if got := p.Veredicto(context.Background(), srv.URL+"/a.m3u8"); got != domain.AirplayUnknown {
		t.Errorf("veredicto = %v, quiero AirplayUnknown", got)
	}
}

func TestProberNoSondeaLoQueLaURLYaDescarta(t *testing.T) {
	var peticiones atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		peticiones.Add(1)
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10)
	if got := p.Veredicto(context.Background(), srv.URL+"/a.mpd"); got != domain.AirplayNo {
		t.Errorf("veredicto = %v, quiero AirplayNo", got)
	}
	if n := peticiones.Load(); n != 0 {
		t.Errorf("el origen recibió %d peticiones, quiero 0: DASH se descarta sin red", n)
	}
}

func TestProberRespetaElTopeDeEntradas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(masterOK))
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 2)
	for _, s := range []string{"/a.m3u8", "/b.m3u8", "/c.m3u8"} {
		p.Veredicto(context.Background(), srv.URL+s)
	}
	if n := p.entradas(); n > 2 {
		t.Errorf("la caché guarda %d entradas, el tope es 2", n)
	}
}
