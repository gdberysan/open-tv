package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
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
	go func() { errc <- run(ctx, slog.New(slog.DiscardHandler), true) }()

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

// Con el fallback de puerto (Tarea 2), un único puerto ocupado ya no basta
// para que run() falle: salta al siguiente. Solo devuelve error cuando se
// agotan los 8 intentos, así que aquí se ocupan los 8 puertos consecutivos.
func TestRunDevuelveErrorSiNoQuedaPuertoLibre(t *testing.T) {
	const base = 18090
	var ocupados []net.Listener
	for i := 0; i < 8; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(base+i))
		if err != nil {
			t.Fatalf("ocupando el puerto %d: %v", base+i, err)
		}
		ocupados = append(ocupados, ln)
	}
	defer func() {
		for _, ln := range ocupados {
			ln.Close()
		}
	}()

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("LISTEN_ADDR", "127.0.0.1:"+strconv.Itoa(base))
	t.Setenv("IPTV_ORG_URL", "http://127.0.0.1:1/index.m3u")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := run(ctx, slog.New(slog.DiscardHandler), true); err == nil {
		t.Error("run debería devolver error si no queda ningún puerto libre")
	}
}

// El fallback de puerto es la diferencia entre "no arranca" y "arranca en el
// 8081". Se comprueba con el 8080 realmente ocupado.
func TestRunSaltaDePuertoSiEstaOcupado(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("preparando el puerto ocupado: %v", err)
	}
	defer ocupado.Close()

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("LISTEN_ADDR", ocupado.Addr().String())
	t.Setenv("IPTV_ORG_URL", "http://127.0.0.1:1/index.m3u")

	_, puertoStr, _ := net.SplitHostPort(ocupado.Addr().String())
	puerto, _ := strconv.Atoi(puertoStr)
	siguiente := "http://127.0.0.1:" + strconv.Itoa(puerto+1)

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- run(ctx, slog.New(slog.DiscardHandler), true) }()

	var resp *http.Response
	for i := 0; i < 60; i++ {
		resp, err = http.Get(siguiente + "/health")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("no escuchó en el puerto siguiente: %v", err)
	}
	resp.Body.Close()

	cancel()
	if err := <-errc; err != nil {
		t.Errorf("run devolvió error: %v", err)
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
