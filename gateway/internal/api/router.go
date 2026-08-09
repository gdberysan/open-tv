package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/gdberysan/open-tv/gateway/internal/api/handlers"
	"github.com/gdberysan/open-tv/gateway/internal/api/middleware"
	"github.com/gdberysan/open-tv/gateway/internal/ports"
)

func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, sqlDB *sql.DB, syncer handlers.SyncStatus) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CORS)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recover(logger))
	r.Use(middleware.RateLimiter(100))

	ch := handlers.NewChannelHandler(logger, repo, provider, streams)

	hh := handlers.NewHealthHandler(sqlDB, syncer)
	r.Get("/health", hh.Get)

	r.Route("/channels", func(r chi.Router) {
		r.Get("/", ch.GetChannels)
		r.Get("/stream", ch.GetStreamURL) // ?id=<channelID>
		r.Get("/{id}/health", ch.GetHealth)
	})

	return r
}
