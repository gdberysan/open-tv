package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/adapters/providers/opensource"
	"github.com/gdberysan/open-tv/internal/adapters/validator"
	"github.com/gdberysan/open-tv/internal/api"
	"github.com/gdberysan/open-tv/internal/datadir"
	"github.com/gdberysan/open-tv/internal/netx"
	"github.com/gdberysan/open-tv/internal/services"
)

// version la inyecta el linker en las releases (-ldflags "-X main.version=…").
// En desarrollo se queda en "dev", que es exactamente lo que es.
var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	// El paquete log GLOBAL de la stdlib lo usan dependencias como
	// net/http.Transport para avisos que no pasan por slog (p.ej. "Unsolicited
	// response received on idle HTTP channel"). Redirigirlo evita líneas sin
	// estructurar mezcladas con el JSON, y de paso las hace buscables.
	log.SetFlags(0)
	log.SetOutput(slogWriter{logger})

	// Subcomandos: `serve` es el default. Se acepta explícito para que el
	// LaunchAgent y los scripts de arranque no dependan del default.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	sinNavegador := fs.Bool("no-browser", false, "no abrir el navegador al arrancar")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, *sinNavegador); err != nil {
		logger.Error("Fallo fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

// run monta el stack completo y bloquea hasta que ctx se cancele. Separado de
// main para que sea testeable: main solo traduce el error a un exit code, y así
// el cableado de workers y el orden de apagado quedan bajo test.
func run(ctx context.Context, logger *slog.Logger, sinNavegador bool) error {
	logger.Info("Iniciando Korven Open TV — gateway")

	// 1. Base de datos SQLite. La ruta viene del directorio de datos del
	// sistema salvo que DB_PATH diga otra cosa.
	dbPath, err := datadir.RutaDB()
	if err != nil {
		return fmt.Errorf("resolviendo el directorio de datos: %w", err)
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("abriendo la base de datos: %w", err)
	}
	defer sqlDB.Close()

	// Pool aparte de solo lectura para los handlers. El de escritura está
	// limitado a una conexión (evita SQLITE_BUSY), así que compartirlo haría
	// que cada request se encolara detrás del sync o del health-check en curso.
	lecturaDB, err := db.OpenReadOnly(dbPath)
	if err != nil {
		return fmt.Errorf("abriendo el pool de lectura: %w", err)
	}
	defer lecturaDB.Close()
	logger.Info("SQLite abierta", slog.String("path", dbPath))

	// 2. Repositorios y proveedor IPTV-org.
	// Escritura: los usan el syncer y el health-worker.
	channelRepo := db.NewChannelRepository(sqlDB)
	streamRepo := db.NewStreamRepository(sqlDB)

	// Lectura: los usan los handlers HTTP.
	channelRepoRO := db.NewChannelRepository(lecturaDB)
	streamRepoRO := db.NewStreamRepository(lecturaDB)

	iptvOrgURL := os.Getenv("IPTV_ORG_URL")
	if iptvOrgURL == "" {
		iptvOrgURL = "https://iptv-org.github.io/iptv/index.m3u"
	}
	provider := opensource.NewProvider("opensource", iptvOrgURL, nil)

	// 3. Sync periódico en background: reintenta con backoff si el proveedor
	// falla y persiste los streams para que /channels/stream sobreviva reinicios.
	syncer := services.NewSyncer(logger, provider, channelRepo, streamRepo, services.Config{
		Interval: durationEnv(logger, "SYNC_INTERVAL", 12*time.Hour),
	})

	syncCtx, stopSync := context.WithCancel(context.Background())
	defer stopSync()

	// Un WaitGroup por cada worker: el apagado tiene que esperarlos antes de
	// que el defer de sqlDB.Close() se desenrolle, o la DB se cierra mientras
	// alguno sigue dentro de un ExecContext.
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		syncer.Run(syncCtx)
	}()

	// 3b. Health-check de streams (Fase 7): valida las URLs con el pool
	// HEAD→GET y marca is_alive/latency en DB. Espera al primer sync para
	// tener el catálogo de streams. HEALTH_INTERVAL default 60m.
	healthWorker := validator.NewWorker(streamRepo, validator.DefaultConfig(),
		durationEnv(logger, "HEALTH_INTERVAL", 60*time.Minute), logger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-syncCtx.Done():
		case <-syncer.FirstSyncDone():
			healthWorker.Start(syncCtx)
		}
	}()

	// 4. Router y servidor HTTP
	// Loopback por defecto: la API no tiene auth y solo la consume la app
	// local. El gateway nunca proxya video (solo devuelve JSON con la URL),
	// así que un WriteTimeout corto es seguro.
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8080"
	}

	// Se prueba el puerto configurado a pelo ANTES de barajar fallbacks: si
	// está libre, nada cambia. Si está ocupado, hay que decidir por qué antes
	// de saltar de puerto — puede ser otro Open TV (llevar al usuario a esa
	// ventana, NUNCA arrancar un segundo catálogo contra la misma SQLite) o
	// un servicio ajeno (entonces sí tiene sentido probar los siguientes
	// puertos, que es el comportamiento de siempre).
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		base := "http://" + listenAddr
		if InstanciaViva(ctx, base) {
			logger.Info("Ya hay un Open TV escuchando; abriendo esa ventana",
				slog.String("url", base))
			if !sinNavegador {
				if err := AbrirNavegador(base); err != nil {
					logger.Warn("No se pudo abrir el navegador", slog.Any("error", err))
				}
			}
			return nil
		}
		// No es Open TV: el puerto configurado lo tiene otra cosa, así que
		// probamos los siguientes.
		ln, err = netx.EscuchaConFallback(listenAddr, 8)
		if err != nil {
			return fmt.Errorf("escuchando en %s: %w", listenAddr, err)
		}
	}

	url := "http://" + ln.Addr().String()
	srv := &http.Server{
		Handler: api.NewRouter(logger, channelRepoRO, provider, streamRepoRO, lecturaDB, syncer, api.Options{
			ProxyActivo: esLoopback(ln),
			Version:     version,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() {
		logger.Info("Servidor escuchando", slog.String("url", url))
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	if !sinNavegador {
		if err := AbrirNavegador(url); err != nil {
			logger.Warn("No se pudo abrir el navegador; abre la URL a mano",
				slog.String("url", url), slog.Any("error", err))
		}
	}

	select {
	case err := <-srvErr:
		return fmt.Errorf("servidor HTTP: %w", err)
	case <-ctx.Done():
	}

	// 5. Apagado ordenado
	logger.Info("Apagando servidor...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error en apagado forzado", slog.Any("error", err))
	}

	// Parar los workers y esperarlos ANTES de que el defer de sqlDB.Close()
	// se desenrolle.
	stopSync()
	wg.Wait()

	logger.Info("Servidor detenido limpiamente")
	return nil
}

// durationEnv lee una duración de entorno con fallback y aviso si es inválida.
// Extraída porque el mismo patrón estaba repetido tres veces.
func durationEnv(logger *slog.Logger, key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		logger.Warn("Duración inválida, usando default",
			slog.String("var", key), slog.String("valor", v), slog.Duration("default", def))
		return def
	}
	return d
}

// slogWriter reencamina lo que escriba el paquete log global hacia slog, para
// que no haya dos formatos de salida distintos en el mismo stdout.
type slogWriter struct{ l *slog.Logger }

func (w slogWriter) Write(p []byte) (int, error) {
	w.l.Warn("stdlib log", slog.String("msg", strings.TrimSpace(string(p))))
	return len(p), nil
}

// esLoopback decide si el proxy HLS puede montarse. Se pregunta al listener
// real y no a la cadena de configuración: LISTEN_ADDR puede decir "localhost",
// un nombre puede resolver a otra cosa, y lo que importa es la IP que quedó.
func esLoopback(ln net.Listener) bool {
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return false
	}
	return addr.IP.IsLoopback()
}
