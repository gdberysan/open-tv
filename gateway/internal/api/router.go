package api

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/api/handlers"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/api/middleware"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, epg ports.EPGRepository, sqlDB *sql.DB, syncer handlers.SyncStatus) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CORS)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recover(logger))
	r.Use(middleware.RateLimiter(100))

	ch := handlers.NewChannelHandler(logger, repo, provider, streams)
	eh := handlers.NewEPGHandler(logger, epg)

	hh := handlers.NewHealthHandler(sqlDB, syncer)
	r.Get("/health", hh.Get)

	r.Route("/channels", func(r chi.Router) {
		r.Get("/", ch.GetChannels)
		r.Get("/stream", ch.GetStreamURL) // ?id=<channelID>
		r.Get("/{id}/epg", eh.GetByChannel)
		r.Get("/{id}/health", ch.GetHealth)
	})

	r.Route("/epg", func(r chi.Router) {
		r.Get("/", eh.GetWindow) // ?channel=<id>&from=<RFC3339>&to=<RFC3339>
		r.Get("/now", eh.GetNow)
	})

	return r
}
