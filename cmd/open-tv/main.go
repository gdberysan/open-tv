package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/gdberysan/open-tv/internal/ports"
	"github.com/gdberysan/open-tv/internal/services"
	"github.com/gdberysan/open-tv/internal/stats"
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

	// 1. Puerto y detección de instancia viva. Esto va ANTES de tocar la DB
	// (Ruling R14): si ya hay un Open TV escuchando, el 2º proceso tiene que
	// enfocar esa ventana y volver SIN abrir el SQLite compartido ni
	// arrancar un Syncer/health-worker contra él. Abrir la DB primero
	// significaba que la 2ª instancia creaba/tocaba el fichero y disparaba
	// un sync abortado antes de darse cuenta de que sobraba.
	//
	// Loopback por defecto: la API no tiene auth y solo la consume la app
	// local. El gateway nunca proxya video (solo devuelve JSON con la URL),
	// así que un WriteTimeout corto es seguro (se fija más abajo, con el
	// router).
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

	// 2. Base de datos SQLite. La ruta viene del directorio de datos del
	// sistema salvo que DB_PATH diga otra cosa. A partir de aquí ya sabemos
	// que somos la única instancia, así que cualquier error de aquí en
	// adelante tiene que cerrar `ln` antes de volver.
	dbPath, err := datadir.RutaDB()
	if err != nil {
		_ = ln.Close()
		return fmt.Errorf("resolviendo el directorio de datos: %w", err)
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		_ = ln.Close()
		return fmt.Errorf("abriendo la base de datos: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	// Pool aparte de solo lectura para los handlers. El de escritura está
	// limitado a una conexión (evita SQLITE_BUSY), así que compartirlo haría
	// que cada request se encolara detrás del sync o del health-check en curso.
	lecturaDB, err := db.OpenReadOnly(dbPath)
	if err != nil {
		_ = ln.Close()
		return fmt.Errorf("abriendo el pool de lectura: %w", err)
	}
	defer func() { _ = lecturaDB.Close() }()
	logger.Info("SQLite abierta", slog.String("path", dbPath))

	// 3. Repositorios y proveedor IPTV-org.
	// Escritura: los usan el syncer y el health-worker.
	channelRepo := db.NewChannelRepository(sqlDB)
	streamRepo := db.NewStreamRepository(sqlDB)

	// Lectura: los usan los handlers HTTP.
	channelRepoRO := db.NewChannelRepository(lecturaDB)
	streamRepoRO := db.NewStreamRepository(lecturaDB)

	sourceRepo := db.NewSourceRepository(sqlDB)

	// Directorio permitido para fuentes file:// (ficheros M3U subidos por el
	// usuario, guardado a cargo de la Tarea 4). Se deriva del propio path de
	// la DB —y no de datadir.Default()— para que respete DB_PATH cuando algo
	// (tests, una instalación con datos en otro sitio) lo fija a mano.
	fuentesDir := filepath.Join(filepath.Dir(dbPath), "fuentes")
	if err := os.MkdirAll(fuentesDir, 0o700); err != nil {
		_ = ln.Close()
		return fmt.Errorf("creando el directorio de fuentes: %w", err)
	}

	// IPTV_ORG_URL es un atajo de dev, opt-in: solo si está fijada se da de
	// alta esa fuente antes de arrancar (mismo INSERT ... ON CONFLICT DO
	// NOTHING del viejo seed, ahora vía SourceRepository.Add). Sin la
	// variable, una instalación limpia arranca con cero fuentes: el catálogo
	// depende por completo de lo que el usuario dé de alta.
	if iptvOrgURL := os.Getenv("IPTV_ORG_URL"); iptvOrgURL != "" {
		err := sourceRepo.Add(context.Background(), ports.Source{
			URL: iptvOrgURL, Label: "IPTV-org (dev)", Kind: "url", IsActive: true,
		})
		if err != nil && !errors.Is(err, ports.ErrFuenteDuplicada) {
			_ = ln.Close()
			return fmt.Errorf("dando de alta IPTV_ORG_URL: %w", err)
		}
	}

	// 4. Sync periódico en background: itera las fuentes activas, reintenta
	// con backoff si el ciclo falla y persiste los streams para que
	// /channels/stream sobreviva reinicios.
	syncer := services.NewSyncer(logger, sourceRepo, channelRepo, streamRepo, fuentesDir, services.Config{
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

	// 4b. Health-check de streams (Fase 7): valida las URLs con el pool
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

	// 4c. Agregador de estadísticas de reproducción (Tarea 13): uno por
	// proceso, en memoria, nunca a disco. Lo que el cliente reporta en
	// /stats/playback vive aquí y solo aquí — se pierde al reiniciar, que es
	// exactamente lo que le corresponde a observabilidad que no rastrea a
	// nadie.
	agregador := stats.NuevoAgregador()

	// api.NewRouter todavía pide un ports.ProviderPort fijo: lo usa
	// ChannelHandler.GetStreamURL como último recurso para un canal que la DB
	// no conoce (arranque en frío antes del primer sync). Con N fuentes ya no
	// hay UN provider que sirva de fallback natural, y resolver eso de verdad
	// es tarea de la Tarea 4 (que retira este parámetro de router.go). Hasta
	// entonces, un provider nunca sincronizado (URL vacía, caché de streams
	// siempre vacío) es un shim inocuo: GetStreamURL sigue devolviendo el
	// mismo "canal no encontrado" que ya devolvía antes de tener catálogo.
	providerFallback := opensource.NewProvider("legacy-fallback", "", nil)

	// 5. Router y servidor HTTP, sobre el listener ya resuelto en el paso 1.
	url := "http://" + ln.Addr().String()
	srv := &http.Server{
		Handler: api.NewRouter(logger, channelRepoRO, providerFallback, streamRepoRO, lecturaDB, syncer, sourceRepo, fuentesDir, api.Options{
			ProxyActivo:              esLoopback(ln),
			Version:                  version,
			HostsPermitidos:          hostsPermitidos(ln),
			Agregador:                agregador,
			PermitirDestinosPrivados: permitirDestinosPrivados(),
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

	// 6. Apagado ordenado
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

// permitirDestinosPrivados decide si el proxy HLS puede relayar loopback/red
// privada. El default (variable ausente o distinta de "1") es el seguro:
// bloquear, que es la protección SSRF de producción y lo que corre en
// cualquier build o instalación real. La única razón por la que esta puerta
// existe es que el e2e de Playwright (web/tests/e2e/global-setup.ts) sirve
// sus fixtures HLS de prueba en 127.0.0.1 y necesita ejercitar el camino real
// del proxy (hls.js → proxy → origen) en vez de que el 403 de SSRF lo tape
// siempre; ese script es el único sitio del repo que fija esta variable.
func permitirDestinosPrivados() bool {
	return os.Getenv("OPEN_TV_PERMITIR_DESTINOS_PRIVADOS") == "1"
}

// hostsPermitidos construye la lista blanca de Host para el middleware
// anti-rebinding (Ruling R13) a partir del puerto REAL con el que se acabó
// enlazando (no LISTEN_ADDR: pudo haber saltado a un puerto de fallback). Los
// tres literales de loopback son los que un navegador puede usar para llegar
// aquí; un dominio atacante que resuelve a 127.0.0.1 llega con su propio
// Host y no está en esta lista.
func hostsPermitidos(ln net.Listener) []string {
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return nil
	}
	puerto := fmt.Sprintf("%d", addr.Port)
	return []string{
		"127.0.0.1:" + puerto,
		"localhost:" + puerto,
		"[::1]:" + puerto,
	}
}
