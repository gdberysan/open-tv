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
	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/providers/opensource"
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

	// 4. Router y servidor HTTP
	// Loopback por defecto: la API no tiene auth y solo la consume la app
	// local. El gateway nunca proxya video (solo devuelve JSON con la URL),
	// así que un WriteTimeout corto es seguro.
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8080"
	}
	handler := api.NewRouter(logger, channelRepo, provider, streamRepo)
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
