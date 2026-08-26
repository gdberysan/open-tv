package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	_ = resp.Body.Close()
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
			_ = ln.Close()
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
	defer func() { _ = ocupado.Close() }()

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
	_ = resp.Body.Close()

	cancel()
	if err := <-errc; err != nil {
		t.Errorf("run devolvió error: %v", err)
	}
}

// Deviación #7 del plan: si el puerto configurado ya lo tiene OTRO Open TV
// (no un servicio cualquiera), run() no debe arrancar un segundo catálogo —
// tiene que detectarlo por InstanciaViva y volver sin tocar ningún puerto de
// fallback. Arrancar un segundo Syncer + health-worker contra la misma
// SQLite es exactamente el bug que este fix cierra.
func TestRunEnfocaInstanciaExistenteEnVezDeArrancarSegunda(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("preparando el puerto ocupado: %v", err)
	}
	addr := ln.Addr().String()

	// Un Open TV "de mentira": un httptest.Server pegado al puerto que
	// normalmente usaría el segundo run(), con un /health que marca web_ui
	// tal y como lo hace el router real (ver instancia_test.go).
	falso := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","db":"ok","web_ui":true}`))
	}))
	falso.Listener = ln
	falso.Start()
	defer falso.Close()

	_, puertoStr, _ := net.SplitHostPort(addr)
	puerto, _ := strconv.Atoi(puertoStr)
	siguiente := "127.0.0.1:" + strconv.Itoa(puerto+1)

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("LISTEN_ADDR", addr)
	t.Setenv("IPTV_ORG_URL", "http://127.0.0.1:1/index.m3u")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errc := make(chan error, 1)
	go func() { errc <- run(ctx, slog.New(slog.DiscardHandler), true) }()

	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("run devolvió error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run no volvió: parece haber arrancado un segundo catálogo en vez de detectar la instancia existente")
	}

	// Nada debería haber arrancado un segundo Open TV en el puerto de
	// fallback. Un dial TCP a pelo es poco fiable aquí — el SO puede tener
	// algún otro proceso ajeno ocupando ese puerto efímero por pura
	// coincidencia — así que se pregunta con la misma vara que usa la
	// producción: InstanciaViva exige un /health con web_ui, que ningún
	// proceso ajeno va a servir por casualidad.
	if InstanciaViva(context.Background(), "http://"+siguiente) {
		t.Error("no debería haber un segundo Open TV escuchando en el puerto de fallback")
	}
}

// Ruling R14: el early-return de "ya hay instancia" tiene que ocurrir ANTES
// de abrir la DB. Si el orden se invierte, el 2º proceso crea/abre el SQLite
// compartido y arranca un sync abortado antes de darse cuenta de que ya había
// una instancia — este test comprueba que el fichero de DB ni se crea.
func TestRunNoAbreLaDBSiYaHayInstancia(t *testing.T) {
	viva := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","web_ui":true}`))
	}))
	defer viva.Close()
	_, puerto, _ := net.SplitHostPort(strings.TrimPrefix(viva.URL, "http://"))

	dbPath := filepath.Join(t.TempDir(), "no-debe-existir.db")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("LISTEN_ADDR", "127.0.0.1:"+puerto)

	if err := run(context.Background(), slog.New(slog.DiscardHandler), true); err != nil {
		t.Fatalf("run devolvió error: %v", err)
	}
	if _, err := os.Stat(dbPath); err == nil {
		t.Error("la 2ª instancia abrió/creó el SQLite; debía enfocar sin tocar la DB")
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
