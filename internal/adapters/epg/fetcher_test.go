package epg

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

const guiaXMLPlano = `<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <programme start="20260826200000 +0000" stop="20260826210000 +0000" channel="cnn.us">
    <title>The Lead</title>
    <sub-title>Breaking News</sub-title>
    <desc>Cobertura de las noticias del día.</desc>
  </programme>
</tv>
`

func TestFetcher_Descargar_XMLPlano(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(guiaXMLPlano))
	}))
	defer server.Close()

	f := NewFetcher(server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rc, err := f.Descargar(ctx, server.URL+"/guia.xml")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("error leyendo body: %v", err)
	}
	if string(got) != guiaXMLPlano {
		t.Errorf("body no coincide con el XML plano esperado.\ngot: %s", got)
	}
}

func TestFetcher_Descargar_XMLGzipPorContentEncoding(t *testing.T) {
	gz, err := os.ReadFile("testdata/guia.xml.gz")
	if err != nil {
		t.Fatalf("no se pudo leer fixture gzip: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(gz)
	}))
	defer server.Close()

	f := NewFetcher(server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rc, err := f.Descargar(ctx, server.URL+"/guia")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("error leyendo body: %v", err)
	}
	if string(got) != guiaXMLPlano {
		t.Errorf("body descomprimido no coincide con el XML plano esperado.\ngot: %s", got)
	}
}

func TestFetcher_Descargar_XMLGzipPorExtensionURL(t *testing.T) {
	gz, err := os.ReadFile("testdata/guia.xml.gz")
	if err != nil {
		t.Fatalf("no se pudo leer fixture gzip: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberadamente SIN Content-Encoding: el servidor sirve el fichero
		// .xml.gz tal cual, como haría un hosting estático de EPG real.
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(gz)
	}))
	defer server.Close()

	f := NewFetcher(server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rc, err := f.Descargar(ctx, server.URL+"/guia.xml.gz")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("error leyendo body: %v", err)
	}
	if string(got) != guiaXMLPlano {
		t.Errorf("body descomprimido (vía extensión de URL) no coincide.\ngot: %s", got)
	}
}

func TestFetcher_Descargar_Error404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	f := NewFetcher(server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := f.Descargar(ctx, server.URL+"/no-existe.xml")
	if err == nil {
		t.Fatal("esperaba error por HTTP 404, obtuve nil")
	}
}

func TestFetcher_Descargar_RespetaContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	f := NewFetcher(server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := f.Descargar(ctx, server.URL+"/lento.xml")
	if err == nil {
		t.Fatal("esperaba error por timeout/cancelación de contexto, obtuve nil")
	}
}

func TestFetcher_Descargar_ErrorDeRedURLInvalida(t *testing.T) {
	f := NewFetcher(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := f.Descargar(ctx, "http://127.0.0.1:0/no-hay-nadie-escuchando.xml")
	if err == nil {
		t.Fatal("esperaba error de red, obtuve nil")
	}
}

func TestNewFetcher_NilClientGetsDefault(t *testing.T) {
	f := NewFetcher(nil)
	if f == nil {
		t.Fatal("NewFetcher(nil) no debería devolver nil")
	}
}
