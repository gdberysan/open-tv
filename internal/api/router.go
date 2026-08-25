package api

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/gdberysan/open-tv/internal/api/handlers"
	"github.com/gdberysan/open-tv/internal/api/middleware"
	"github.com/gdberysan/open-tv/internal/ports"
)

func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, sqlDB *sql.DB, syncer handlers.SyncStatus, proxyActivo bool) http.Handler {
	// proxyActivo lo decide el listener real (loopback o no) y lo consumen el
	// proxy HLS (montaje) y /health (proxy_enabled).
	_ = proxyActivo

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recover(logger))
	r.Use(middleware.RateLimiter(100))

	// TTL de 12 h: los códecs de un canal no cambian en una tarde, y la caché
	// se pierde igualmente al reiniciar el gateway.
	prober := handlers.NewAirplayProber(nil, 12*time.Hour, 2000)
	ch := handlers.NewChannelHandler(logger, repo, provider, streams, prober)

	hh := handlers.NewHealthHandler(sqlDB, syncer)
	r.Get("/health", hh.Get)

	r.Route("/channels", func(r chi.Router) {
		r.Get("/", ch.GetChannels)
		r.Get("/stream", ch.GetStreamURL) // ?id=<channelID>
		r.Get("/countries", ch.GetCountries)
		r.Get("/categories", ch.GetCategories)
		r.Get("/random", ch.GetRandom)
		r.Get("/{id}/health", ch.GetHealth)
	})

	return r
}
