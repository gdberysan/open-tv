package api

import (
	"context"
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
	"github.com/gdberysan/open-tv/internal/stats"
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
	// HostsPermitidos cierra el DNS-rebinding (Ruling R13): si no está vacía,
	// solo se sirven peticiones cuyo Host esté en la lista. Vacía = middleware
	// deshabilitado, que es lo que usan los tests con Host arbitrario.
	HostsPermitidos []string
	// Agregador acumula los desenlaces de reproducción que reporta /stats/
	// playback. Uno por proceso, creado en cmd/open-tv/main.go. nil (el caso
	// de los tests de router existentes, que no lo necesitan) se resuelve a
	// un agregador nuevo y vacío: /stats nunca debe nil-pointer-panicar.
	Agregador *stats.Agregador
	// PermitirDestinosPrivados se traslada tal cual a proxy.NewHandler. Falso
	// en todo despliegue real (Ruling de seguridad: el proxy nunca debe
	// relayar loopback/red privada, protección SSRF); cmd/open-tv/main.go
	// solo lo pone a true si OPEN_TV_PERMITIR_DESTINOS_PRIVADOS=1, una
	// puerta pensada exclusivamente para que el e2e de Playwright (que sirve
	// sus fixtures HLS en 127.0.0.1) pueda ejercitar el camino real del
	// proxy. El default de este campo (false, el zero value) ya es el seguro,
	// así que cualquier caller que no lo fije explícitamente —incluidos
	// todos los tests de router existentes— se queda con el proxy
	// bloqueando destinos privados.
	PermitirDestinosPrivados bool
	// Desenlaces es el puerto de ESCRITURA de POST /streams/desenlace. Va
	// aparte de `streams` (el StreamRepository de arriba, de solo lectura)
	// porque RegistrarDesenlace muta la tabla streams: cmd/open-tv/main.go
	// inyecta aquí el repo abierto sobre el pool de escritura. nil (el caso
	// de todos los tests de router existentes, que no lo necesitan) hace que
	// el handler responda 503 en vez de panicar.
	Desenlaces ports.RegistradorDesenlaces
	// Catalogo autoriza en el proxy las URLs de nivel superior. nil (tests que
	// no lo necesitan) deja pasar solo URLs firmadas, que es el caso seguro.
	Catalogo ports.VerificadorURLs
	// Firmador firma las URLs hijas del proxy. nil: el router crea uno
	// aleatorio. cmd/open-tv lo crea él para poder fallar al arrancar si no hay
	// aleatoriedad, en vez de degradar en silencio.
	Firmador *proxy.Firmador
}

// Syncer es lo que el router necesita del *services.Syncer para cablear tanto
// /health (SyncStatus: LastSuccess) como /sources/{id}/sync (SyncOne bajo
// demanda). Una sola interfaz para las dos en vez de dos parámetros separados:
// en producción los satisface el mismo *services.Syncer, y aquí se declara el
// mínimo común para que los dobles de test puedan implementar solo lo que usan.
type Syncer interface {
	handlers.SyncStatus
	SyncOne(ctx context.Context, id string) error
}

func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, sqlDB *sql.DB, syncer Syncer, sources ports.SourceRepository, fuentesDir string, epg ports.EPGRepository, opts Options) http.Handler {
	r := chi.NewRouter()

	// Primero: si el Host no es el loopback enlazado, se corta antes de que
	// nada más (logger, rate limiter, handlers) toque la petición.
	r.Use(middleware.MismoOrigen(opts.HostsPermitidos))
	r.Use(chimiddleware.RequestID)
	// Sin RealIP a propósito: este gateway nunca vive detrás de un proxy
	// inverso de confianza (solo loopback, consumido por la app local), así
	// que fiarse de X-Forwarded-For/X-Real-IP/True-Client-IP solo abriría la
	// puerta a que cualquiera falsifique el remote_addr que ve el logger.
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recover(logger))
	r.Use(middleware.RateLimiter(100))

	// TTL de 12 h: los códecs de un canal no cambian en una tarde, y la caché
	// se pierde igualmente al reiniciar el gateway. permitirDestinosPrivados
	// va siempre false aquí: los streams sondeados llegan de proveedores
	// externos, nunca de la propia app.
	prober := handlers.NewAirplayProber(nil, 12*time.Hour, 2000, false)
	ch := handlers.NewChannelHandler(logger, repo, provider, streams, prober)
	eh := handlers.NewEPGHandler(logger, epg)

	clienteWeb, hayClienteWeb := ui.Handler()

	hh := handlers.NewHealthHandler(sqlDB, syncer, handlers.Info{
		Version:      opts.Version,
		WebUI:        hayClienteWeb,
		ProxyEnabled: opts.ProxyActivo,
		ProxyRuta:    RutaProxy,
	})
	r.Get("/health", hh.Get)

	// El agregado del catálogo se calcula contra el mismo pool de solo
	// lectura que ya usa /health (sqlDB): no hay un query de este agregado en
	// ningún repositorio existente, así que reutilizar el pool y hacer la
	// consulta en el propio handler es más simple que ensanchar
	// ports.StreamRepository por un único endpoint de observabilidad.
	agregador := opts.Agregador
	if agregador == nil {
		agregador = stats.NuevoAgregador()
	}
	sh := handlers.NewStatsHandler(agregador, handlers.NewDBCatalogoStats(sqlDB))
	r.Post("/stats/playback", sh.PostPlayback)
	r.Get("/stats", sh.GetStats)

	// El desenlace real de un intento de reproducción (spec
	// tiempo-hasta-la-imagen §3.1): opts.Desenlaces es nil en los tests de
	// router que no lo necesitan, y el handler responde 503 en ese caso.
	dh := handlers.NewDesenlaceHandler(logger, opts.Desenlaces)
	r.Post("/streams/desenlace", dh.Post)

	// /sources* es exclusivo del cliente web (Flutter no lo conoce, ver
	// contrato congelado): alta/baja/resync de las fuentes "bring your own"
	// del usuario. "sources" tiene que ser el pool de ESCRITURA: Add y Remove
	// mutan la tabla providers, y el pool de solo lectura del resto de
	// handlers de este router (sqlDB de arriba) está abierto en modo
	// read-only — cmd/open-tv/main.go es quien decide cuál pasa aquí.
	fh := handlers.NewSourceHandler(logger, sources, syncer, fuentesDir)
	r.Route("/sources", func(r chi.Router) {
		r.Get("/", fh.Get)
		r.Post("/", fh.Post)
		// Ruta literal antes que la de parámetro por claridad de lectura (chi
		// no necesita el orden para no confundir "sugeridas" con un {id}, pero
		// el resto del router ya documenta este mismo cuidado en /channels).
		r.Get("/sugeridas", fh.Sugeridas)
		r.Delete("/{id}", fh.Delete)
		r.Post("/{id}/sync", fh.Sync)
	})

	r.Route("/channels", func(r chi.Router) {
		r.Get("/", ch.GetChannels)
		r.Get("/stream", ch.GetStreamURL)       // ?id=<channelID>
		r.Get("/streams", ch.GetChannelStreams) // ?id=<channelID>
		r.Get("/countries", ch.GetCountries)
		r.Get("/categories", ch.GetCategories)
		r.Get("/qualities", ch.GetQualities)
		r.Get("/imagen", ch.GetImagen) // literal antes de /{id}/health, mismo cuidado que el resto
		r.Get("/random", ch.GetRandom)
		r.Get("/epg", eh.GetEPGLote) // ?ids=<a,b,c>
		r.Get("/{id}/health", ch.GetHealth)
		r.Get("/{id}/epg", eh.GetEPGCanal) // ?limit=<n>
	})

	// El proxy HLS solo relaya URLs del catálogo o firmadas por este proceso
	// (internal/proxy/firma.go), así que no es un relé abierto. En modo red,
	// además, todo lo que hay detrás del middleware de acceso exige sesión.
	// ProxyActivo sigue existiendo para los tests que prueban el router sin él.
	if opts.ProxyActivo {
		firmador := opts.Firmador
		if firmador == nil {
			var err error
			firmador, err = proxy.NuevoFirmadorAleatorio()
			if err != nil {
				// crypto/rand no falla en ningún sistema soportado; si lo
				// hiciera, mejor sin proxy que con uno sin firma.
				logger.Error("proxy desactivado: sin fuente de aleatoriedad", slog.Any("error", err))
			}
		}
		if firmador == nil {
			r.Get("/proxy/hls", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
		} else {
			opciones := []proxy.Option{
				proxy.ConBuscadorCabeceras(func(ctx context.Context, u string) (string, string) {
					ref, ua, err := streams.CabecerasPorURL(ctx, u)
					if err != nil {
						// Un fallo de lectura no puede tumbar la reproducción:
						// se cae a las cabeceras de siempre.
						return "", ""
					}
					return ref, ua
				}),
			}
			if opts.Catalogo != nil {
				catalogo := opts.Catalogo
				opciones = append(opciones, proxy.ConCatalogo(func(ctx context.Context, u string) bool {
					ok, err := catalogo.ExisteURL(ctx, u)
					if err != nil {
						logger.Warn("proxy: no se pudo consultar el catálogo", slog.Any("error", err))
						return false
					}
					return ok
				}))
			}
			ph := proxy.NewHandler(RutaProxy, firmador, opts.PermitirDestinosPrivados, opciones...)
			r.Get("/proxy/hls", ph.ServeHTTP)
		}
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
