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
	"github.com/gdberysan/open-tv/internal/proxy"
	"github.com/gdberysan/open-tv/internal/ui"
)

// RutaProxy es el prefijo con el que se reescriben las URIs del manifiesto y
// la ruta que las sirve. Una sola constante para que el reescritor y el router
// no puedan divergir.
const RutaProxy = "/proxy/hls?u="

// Options son los datos que el router necesita del proceso: qué puede montar
// y qué versión anunciar.
type Options struct {
	// ProxyActivo lo decide el listener real (loopback o no), nunca la
	// configuración: LISTEN_ADDR puede decir "localhost" y resolver a otra cosa.
	ProxyActivo bool
	Version     string
}

func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, sqlDB *sql.DB, syncer handlers.SyncStatus, opts Options) http.Handler {
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

	clienteWeb, hayClienteWeb := ui.Handler()

	hh := handlers.NewHealthHandler(sqlDB, syncer, handlers.Info{
		Version:      opts.Version,
		WebUI:        hayClienteWeb,
		ProxyEnabled: opts.ProxyActivo,
		ProxyRuta:    RutaProxy,
	})
	r.Get("/health", hh.Get)

	r.Route("/channels", func(r chi.Router) {
		r.Get("/", ch.GetChannels)
		r.Get("/stream", ch.GetStreamURL)       // ?id=<channelID>
		r.Get("/streams", ch.GetChannelStreams) // ?id=<channelID>
		r.Get("/countries", ch.GetCountries)
		r.Get("/categories", ch.GetCategories)
		r.Get("/random", ch.GetRandom)
		r.Get("/{id}/health", ch.GetHealth)
	})

	// El proxy HLS solo existe cuando escuchamos en loopback. No hay flag para
	// forzarlo: un proxy abierto a la red es un relay de vídeo de terceros con
	// la IP de quien lo levante, y eso no se ofrece ni por accidente. Los
	// builds del snapshot tampoco lo incluyen porque nunca son loopback.
	if opts.ProxyActivo {
		ph := proxy.NewHandler(RutaProxy, false)
		r.Get("/proxy/hls", ph.ServeHTTP)
	} else {
		// Sin proxy, /proxy/hls tiene que devolver un 404 explícito y no
		// caer en el fallback SPA de más abajo: una URL de proxy que
		// responde HTML con 200 es confusa aunque no insegura (nunca hay
		// relay de vídeo). Se registra ANTES del NotFound para que chi la
		// resuelva como ruta propia, no como comodín.
		r.Get("/proxy/hls", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	// La UI va la ÚLTIMA: NotFound solo se aplica a lo que ninguna ruta de API
	// haya reclamado, y así /channels/loquesea sigue siendo un 404 de API y no
	// devuelve el index.
	if hayClienteWeb {
		r.NotFound(clienteWeb.ServeHTTP)
	}

	return r
}
