package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseLocalSiempreApuntaALoopback(t *testing.T) {
	for entrada, quiero := range map[string]string{
		"":               "http://127.0.0.1:8080",
		"0.0.0.0:9000":   "http://127.0.0.1:9000",
		"127.0.0.1:8081": "http://127.0.0.1:8081",
		"192.168.1.5:80": "http://127.0.0.1:80",
		"[::]:7000":      "http://127.0.0.1:7000",
	} {
		if got := baseLocal(entrada); got != quiero {
			t.Errorf("baseLocal(%q) = %q, quiero %q", entrada, got, quiero)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	sano := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","web_ui":true}`))
	}))
	defer sano.Close()
	if code := healthcheck(context.Background(), strings.TrimPrefix(sano.URL, "http://")); code != 0 {
		t.Errorf("servidor sano → %d, quiero 0", code)
	}
	if code := healthcheck(context.Background(), "127.0.0.1:1"); code != 1 {
		t.Errorf("nada escuchando → %d, quiero 1", code)
	}
}

func TestMostrarClaveLeeEntornoOFicheroSinGenerar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(dir, "open-tv.db"))
	t.Setenv("OPEN_TV_ACCESS_KEY", "")

	var out bytes.Buffer
	if code := mostrarClave(&out); code != 1 {
		t.Errorf("sin clave → %d, quiero 1", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "access-key")); !os.IsNotExist(err) {
		t.Error("mostrarClave generó una clave: solo debe leer")
	}

	if err := os.WriteFile(filepath.Join(dir, "access-key"), []byte("del-fichero\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := mostrarClave(&out); code != 0 || strings.TrimSpace(out.String()) != "del-fichero" {
		t.Errorf("fichero → code %d, salida %q", code, out.String())
	}

	t.Setenv("OPEN_TV_ACCESS_KEY", "del-entorno")
	out.Reset()
	if code := mostrarClave(&out); code != 0 || strings.TrimSpace(out.String()) != "del-entorno" {
		t.Errorf("entorno → code %d, salida %q", code, out.String())
	}
}

func TestMetaComandosNuevosNoArrancanServidor(t *testing.T) {
	var out bytes.Buffer
	t.Setenv("OPEN_TV_ACCESS_KEY", "x")
	if manejado, _ := manejaMetaComando([]string{"access-key"}, &out); !manejado {
		t.Error("access-key no se reconoce como meta-comando")
	}
	t.Setenv("LISTEN_ADDR", "127.0.0.1:1")
	if manejado, code := manejaMetaComando([]string{"healthcheck"}, &out); !manejado || code != 1 {
		t.Errorf("healthcheck → manejado=%v code=%d, quiero true/1", manejado, code)
	}
}
