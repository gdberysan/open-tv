package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/providers/opensource"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/validator"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/api"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	// El paquete log GLOBAL de la stdlib lo usan dependencias como
	// net/http.Transport para avisos que no pasan por slog (p.ej. "Unsolicited
	// response received on idle HTTP channel"). Redirigirlo evita líneas sin
	// estructurar mezcladas con el JSON, y de paso las hace buscables.
	log.SetFlags(0)
	log.SetOutput(slogWriter{logger})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("Fallo fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

// run monta el stack completo y bloquea hasta que ctx se cancele. Separado de
// main para que sea testeable: main solo traduce el error a un exit code, y así
// el cableado de workers y el orden de apagado quedan bajo test.
func run(ctx context.Context, logger *slog.Logger) error {
	logger.Info("Iniciando IPTV Ecosystem API Gateway")

	// 1. Base de datos SQLite
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "iptv.db"
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
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           api.NewRouter(logger, channelRepoRO, provider, streamRepoRO, lecturaDB, syncer),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// El fallo del servidor viaja por un canal en vez de os.Exit(1): así el
	// apagado ordenado se ejecuta igual y no se saltan los defers.
	srvErr := make(chan error, 1)
	go func() {
		logger.Info("Servidor escuchando", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

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
