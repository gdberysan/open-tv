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

	// 2. Repositorio y proveedor IPTV-org
	channelRepo := db.NewChannelRepository(sqlDB)

	iptvOrgURL := os.Getenv("IPTV_ORG_URL")
	if iptvOrgURL == "" {
		iptvOrgURL = "https://iptv-org.github.io/iptv/index.m3u"
	}
	provider := opensource.NewProvider("opensource", iptvOrgURL, nil)

	// 3. Sync inicial en background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		logger.Info("Iniciando sync de canales desde IPTV-org...")
		channels, err := provider.GetLiveChannels(ctx)
		if err != nil {
			logger.Error("Sync IPTV-org fallido", slog.Any("error", err))
			return
		}
		if err := channelRepo.SaveBatch(ctx, channels); err != nil {
			logger.Error("SaveBatch fallido", slog.Any("error", err))
			return
		}
		logger.Info("Sync completado", slog.Int("canales", len(channels)))
	}()

	// 4. Router y servidor HTTP
	handler := api.NewRouter(logger, channelRepo, provider)
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // streams continuos — sin timeout de escritura
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
