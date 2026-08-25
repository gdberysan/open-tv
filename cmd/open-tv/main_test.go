package main

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// run debe montar el stack, servir, y apagar limpiamente al cancelar el
// contexto — sin dejar la DB cerrada bajo los workers ni colgarse esperándolos.
func TestRunArrancaYApagaLimpio(t *testing.T) {
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("LISTEN_ADDR", "127.0.0.1:18080")
	// Provider inalcanzable: el sync falla y reintenta, que es justo el estado
	// en el que un apagado mal ordenado se cuelga o cierra la DB por debajo.
	t.Setenv("IPTV_ORG_URL", "http://127.0.0.1:1/index.m3u")

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- run(ctx, slog.New(slog.DiscardHandler)) }()

	var resp *http.Response
	var err error
	for i := 0; i < 60; i++ {
		resp, err = http.Get("http://127.0.0.1:18080/health")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("el servidor no llegó a escuchar: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/health = %d, quiero 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("run devolvió error en un apagado limpio: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("run no terminó en 20s tras cancelar el contexto: algún worker ignora la cancelación")
	}
}

// Un puerto ocupado debe devolver error en vez de matar el proceso con
// os.Exit(1) desde dentro de una goroutine, que se saltaba todos los defers.
func TestRunDevuelveErrorSiElPuertoEstaOcupado(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:18081", Handler: http.NotFoundHandler()}
	go func() { _ = srv.ListenAndServe() }()
	defer srv.Close()
	time.Sleep(300 * time.Millisecond)

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("LISTEN_ADDR", "127.0.0.1:18081")
	t.Setenv("IPTV_ORG_URL", "http://127.0.0.1:1/index.m3u")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := run(ctx, slog.New(slog.DiscardHandler)); err == nil {
		t.Error("run debería devolver error si no puede escuchar")
	}
}

func TestDurationEnv(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("vacío usa el default", func(t *testing.T) {
		t.Setenv("PRUEBA_DUR", "")
		if got := durationEnv(logger, "PRUEBA_DUR", time.Hour); got != time.Hour {
			t.Errorf("got %v, quiero 1h", got)
		}
	})
	t.Run("valor válido se respeta", func(t *testing.T) {
		t.Setenv("PRUEBA_DUR", "45m")
		if got := durationEnv(logger, "PRUEBA_DUR", time.Hour); got != 45*time.Minute {
			t.Errorf("got %v, quiero 45m", got)
		}
	})
	t.Run("basura cae al default", func(t *testing.T) {
		t.Setenv("PRUEBA_DUR", "no-es-una-duración")
		if got := durationEnv(logger, "PRUEBA_DUR", time.Hour); got != time.Hour {
			t.Errorf("got %v, quiero 1h", got)
		}
	})
	t.Run("negativo cae al default", func(t *testing.T) {
		t.Setenv("PRUEBA_DUR", "-5m")
		if got := durationEnv(logger, "PRUEBA_DUR", time.Hour); got != time.Hour {
			t.Errorf("got %v, quiero 1h", got)
		}
	})
}
