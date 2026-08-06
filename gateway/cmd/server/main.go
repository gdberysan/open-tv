package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/epg"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/providers/opensource"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/validator"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/api"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("Iniciando IPTV Ecosystem API Gateway")

	// 1. Base de datos SQLite
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "iptv.db"
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		logger.Error("No se pudo abrir la base de datos", slog.Any("error", err))
		os.Exit(1)
	}
	defer sqlDB.Close()
	logger.Info("SQLite abierta", slog.String("path", dbPath))

	// 2. Repositorios y proveedor IPTV-org
	channelRepo := db.NewChannelRepository(sqlDB)
	streamRepo := db.NewStreamRepository(sqlDB)
	epgRepo := db.NewEPGRepository(sqlDB)

	iptvOrgURL := os.Getenv("IPTV_ORG_URL")
	if iptvOrgURL == "" {
		iptvOrgURL = "https://iptv-org.github.io/iptv/index.m3u"
	}
	provider := opensource.NewProvider("opensource", iptvOrgURL, nil)

	// 3. Sync periódico en background: reintenta con backoff si el proveedor
	// falla y persiste los streams para que /channels/stream sobreviva reinicios.
	syncInterval := 12 * time.Hour
	if v := os.Getenv("SYNC_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			syncInterval = d
		} else {
			logger.Warn("SYNC_INTERVAL inválido, usando default 12h", slog.String("valor", v))
		}
	}
	syncer := services.NewSyncer(logger, provider, channelRepo, streamRepo, services.Config{
		Interval: syncInterval,
	})
	syncCtx, stopSync := context.WithCancel(context.Background())
	defer stopSync()
	go syncer.Run(syncCtx)

	// 3b. Health-check de streams (Fase 7): valida las URLs con el pool
	// HEAD→GET y marca is_alive/latency en DB. Espera al primer sync para
	// tener el catálogo de streams. HEALTH_INTERVAL default 60m.
	healthInterval := 60 * time.Minute
	if v := os.Getenv("HEALTH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			healthInterval = d
		} else {
			logger.Warn("HEALTH_INTERVAL inválido, usando default 60m", slog.String("valor", v))
		}
	}
	healthWorker := validator.NewWorker(streamRepo, validator.DefaultConfig(), healthInterval, logger)
	go func() {
		select {
		case <-syncCtx.Done():
		case <-syncer.FirstSyncDone():
			healthWorker.Start(syncCtx)
		}
	}()

	// 3c. Worker EPG (opt-in): no existe una URL XMLTV pública canónica para
	// el catálogo completo de IPTV-org, así que la fuente se configura vía
	// EPG_URL (acepta .xml y .xml.gz). Sin ella el worker no arranca y los
	// endpoints /epg responden vacío.
	if epgURL := os.Getenv("EPG_URL"); epgURL != "" {
		epgInterval := 12 * time.Hour
		if v := os.Getenv("EPG_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				epgInterval = d
			} else {
				logger.Warn("EPG_INTERVAL inválido, usando default 12h", slog.String("valor", v))
			}
		}
		epgWorker := epg.NewWorker(epg.NewParser(), epgRepo, epgURL, epgInterval, logger)
		// Esperar al primer sync de canales: sin tvg_id en DB, el EPG
		// descartaría todas las entradas en silencio.
		go func() {
			select {
			case <-syncCtx.Done():
			case <-syncer.FirstSyncDone():
				epgWorker.Start(syncCtx)
			}
		}()
	} else {
		logger.Info("EPG_URL no configurada; worker EPG desactivado")
	}

	// 4. Router y servidor HTTP
	// Loopback por defecto: la API no tiene auth y solo la consume la app
	// local. El gateway nunca proxya video (solo devuelve JSON con la URL),
	// así que un WriteTimeout corto es seguro.
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8080"
	}
	handler := api.NewRouter(logger, channelRepo, provider, streamRepo, epgRepo)
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("Servidor escuchando", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Fallo en el servidor", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// 5. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Apagando servidor...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Error en apagado forzado", slog.Any("error", err))
	}
	logger.Info("Servidor detenido limpiamente")
}
