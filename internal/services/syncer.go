// Package services contiene los casos de uso que orquestan adapters y ports.
package services

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/providers/opensource"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

// ErrCatalogoSospechoso se devuelve cuando una fuente entrega muchos menos
// canales que en el último sync aceptado. Un 200 con una página de error, un
// portal cautivo o un cuerpo truncado produce un M3U válido pero casi vacío;
// sin este suelo eso contaría como éxito, resetearía el backoff a la cadencia
// normal y —con la poda activa— borraría el catálogo entero de esa fuente.
var ErrCatalogoSospechoso = errors.New("catálogo anómalamente pequeño")

// ErrFuenteNoEncontrada se devuelve por SyncOne cuando el id pedido no
// corresponde a ninguna fuente dada de alta.
var ErrFuenteNoEncontrada = errors.New("fuente no encontrada")

// minRatioCatalogo es la fracción del catálogo anterior por debajo de la cual
// un sync se rechaza. Los catálogos FTA fluctúan algo entre syncs; perder más
// de la mitad de golpe es un fallo de la fuente, no una actualización.
const minRatioCatalogo = 0.5

// Config parametriza la cadencia del sync. Los ceros toman los defaults.
type Config struct {
	// Interval entre syncs exitosos. Default: 12h.
	Interval time.Duration
	// RetryBase es la espera tras el primer fallo; se duplica en cada
	// fallo consecutivo hasta RetryMax. Defaults: 30s y 10min.
	RetryBase time.Duration
	RetryMax  time.Duration
	// SyncTimeout acota cada intento completo (todas las fuentes). Default: 5min.
	SyncTimeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.Interval <= 0 {
		c.Interval = 12 * time.Hour
	}
	if c.RetryBase <= 0 {
		c.RetryBase = 30 * time.Second
	}
	if c.RetryMax <= 0 {
		c.RetryMax = 10 * time.Minute
	}
	if c.SyncTimeout <= 0 {
		c.SyncTimeout = 5 * time.Minute
	}
	return c
}

// Syncer mantiene la DB de canales y streams al día con las fuentes que el
// usuario ha dado de alta. Sustituye al sync de un único provider fijo: cada
// ciclo relee SourceRepository.List y sincroniza todas las fuentes activas,
// fusionando sus catálogos (los IDs de canal ya llevan namespace por fuente).
// Reintenta con backoff exponencial si el ciclo falla y re-sincroniza
// periódicamente si tiene éxito.
type Syncer struct {
	logger         *slog.Logger
	sources        ports.SourceRepository
	allowedFileDir string
	httpClient     *http.Client
	channels       ports.ChannelRepository
	streams        ports.StreamRepository
	cfg            Config

	firstDone chan struct{}
	firstOnce sync.Once

	mu          sync.RWMutex
	lastSuccess time.Time

	// syncMu serializa toda ejecución de sincronizarFuente, del ciclo
	// periódico o de un SyncOne bajo demanda. Es una guarda simple y
	// deliberadamente de grano grueso (una fuente a la vez en todo el
	// proceso, no solo por-fuente): un futuro handler HTTP puede invocar
	// SyncOne mientras el ciclo de Run está en marcha, y esto basta para que
	// dos sincronizaciones nunca corran a la vez ni pisen ultimoConteo.
	syncMu sync.Mutex

	// ultimoConteo es el número de canales del último sync aceptado, por
	// fuente. Ausente/cero significa que aún no hay referencia con la que
	// comparar. Solo lo toca sincronizarFuente, que corre bajo syncMu.
	ultimoConteo map[string]int
}

func NewSyncer(logger *slog.Logger, sources ports.SourceRepository, channels ports.ChannelRepository, streams ports.StreamRepository, allowedFileDir string, cfg Config) *Syncer {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Syncer{
		logger:         logger,
		sources:        sources,
		allowedFileDir: allowedFileDir,
		channels:       channels,
		streams:        streams,
		cfg:            cfg.withDefaults(),
		firstDone:      make(chan struct{}),
		ultimoConteo:   make(map[string]int),
	}
}

// FirstSyncDone se cierra tras el primer ciclo exitoso (aunque no haya
// fuentes: un ciclo con cero fuentes es un no-op que se considera éxito).
// Permite a trabajos dependientes (hoy el health-worker, que necesita el
// catálogo de streams) esperar a que el primer ciclo haya corrido antes de
// arrancar, en vez de colgarse para siempre en una instalación limpia.
func (s *Syncer) FirstSyncDone() <-chan struct{} {
	return s.firstDone
}

// LastSuccess devuelve el instante del último ciclo exitoso. Cero si aún no ha
// habido ninguno. Lo consume /health para reportar la edad del catálogo: un
// syncer que lleva días fallando no debe verse como un gateway sano.
func (s *Syncer) LastSuccess() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSuccess
}

// Run ejecuta el bucle de sincronización hasta que ctx se cancele.
func (s *Syncer) Run(ctx context.Context) {
	backoff := s.cfg.RetryBase
	for {
		err := s.syncWithTimeout(ctx)
		if ctx.Err() != nil {
			return
		}

		var wait time.Duration
		if err != nil {
			s.logger.Error("Ciclo de sync fallido, reintentando",
				slog.Any("error", err), slog.Duration("backoff", backoff))
			wait = backoff
			backoff = min(backoff*2, s.cfg.RetryMax)
		} else {
			s.mu.Lock()
			s.lastSuccess = time.Now()
			s.mu.Unlock()

			s.firstOnce.Do(func() { close(s.firstDone) })
			wait = s.cfg.Interval
			backoff = s.cfg.RetryBase
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Syncer) syncWithTimeout(ctx context.Context) error {
	syncCtx, cancel := context.WithTimeout(ctx, s.cfg.SyncTimeout)
	defer cancel()
	return s.SyncOnce(syncCtx)
}

// SyncOnce ejecuta un ciclo completo: relee las fuentes activas y sincroniza
// cada una. El fallo de una fuente se registra vía slog y NO aborta el
// ciclo — las demás se intentan igualmente.
//
// Semántica de éxito/fallo del CICLO (no de cada fuente): un ciclo es útil
// si el catálogo avanzó. Basta con que UNA fuente intentada haya
// sincronizado bien —o con que no hubiera ninguna fuente activa que
// intentar— para que el ciclo cuente como éxito de cara a Run(): LastSuccess
// se estampa y FirstSyncDone se cierra aunque el resto fallara. Una fuente
// rota entre varias buenas NUNCA debe hacer pasar el catálogo entero por
// "no sincronizado": eso cuelga /health (last_sync:null para siempre) y con
// él al cliente, aunque miles de canales de las fuentes sanas ya se estén
// sirviendo. SyncOnce solo devuelve error (el errors.Join de los fallos, así
// errors.Is/As sigue funcionando, p.ej. para ErrCatalogoSospechoso) cuando
// TODAS las fuentes intentadas fallaron — ahí, y solo ahí, tiene sentido que
// Run() aplique su backoff al ciclo completo. TouchSync sigue siendo
// estrictamente por-fuente: solo se marca la que sincronizó bien.
func (s *Syncer) SyncOnce(ctx context.Context) error {
	fuentes, err := s.sources.List(ctx)
	if err != nil {
		return fmt.Errorf("services.SyncOnce (List): %w", err)
	}

	var (
		errores  []error
		intentos int
		exitos   int
	)
	for _, fuente := range fuentes {
		if !fuente.IsActive {
			continue
		}
		intentos++
		if err := s.sincronizarFuenteYMarcar(ctx, fuente); err != nil {
			s.logger.Error("Fallo sincronizando fuente, se continúa con las demás",
				slog.String("fuente", fuente.ID), slog.Any("error", err))
			errores = append(errores, err)
			continue
		}
		exitos++
	}

	if intentos > 0 && exitos == 0 {
		return errors.Join(errores...)
	}
	return nil
}

// SyncOne sincroniza únicamente la fuente indicada, bajo demanda (pensado
// para un futuro handler HTTP de re-sync manual). A diferencia del ciclo
// periódico, no filtra por IsActive: quien pide un id concreto ya sabe cuál
// quiere. Con un id que no corresponde a ninguna fuente dada de alta,
// devuelve un error que envuelve ErrFuenteNoEncontrada.
func (s *Syncer) SyncOne(ctx context.Context, id string) error {
	fuentes, err := s.sources.List(ctx)
	if err != nil {
		return fmt.Errorf("services.SyncOne (List): %w", err)
	}
	for _, fuente := range fuentes {
		if fuente.ID != id {
			continue
		}
		if err := s.sincronizarFuenteYMarcar(ctx, fuente); err != nil {
			return fmt.Errorf("services.SyncOne (fuente=%s): %w", id, err)
		}
		return nil
	}
	return fmt.Errorf("services.SyncOne (id=%s): %w", id, ErrFuenteNoEncontrada)
}

// sincronizarFuenteYMarcar sincroniza una fuente y, solo si tuvo éxito, sella
// su TouchSync. Un fallo en TouchSync se trata como fallo de la sincronización
// completa: mejor reintentar de más (sincronizarFuente es idempotente) que
// dejar una fuente sincronizada sin su marca de tiempo actualizada.
func (s *Syncer) sincronizarFuenteYMarcar(ctx context.Context, fuente ports.Source) error {
	if err := s.sincronizarFuente(ctx, fuente); err != nil {
		return err
	}
	if err := s.sources.TouchSync(ctx, fuente.ID, time.Now().Unix()); err != nil {
		return fmt.Errorf("TouchSync: %w", err)
	}
	return nil
}

// sincronizarFuente descarga los canales de UNA fuente y persiste canales y
// streams. Construye un provider opensource efímero por fuente (mismo cliente
// HTTP y mismo directorio permitido para file:// en todas), lo usa una vez y
// lo descarta: el caché de streamURLs del provider solo hace falta durante
// esta llamada, porque GetStreamURL se consulta aquí mismo, justo después del
// GetLiveChannels que lo pobló.
//
// syncMu serializa esta función: el ciclo periódico y un SyncOne bajo demanda
// nunca deben tocar ultimoConteo[fuente.ID] ni escribir en DB para la misma
// (o distinta) fuente al mismo tiempo.
func (s *Syncer) sincronizarFuente(ctx context.Context, fuente ports.Source) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	provider := opensource.NewProvider(fuente.ID, fuente.URL, s.httpClient,
		opensource.WithAllowedFileDir(s.allowedFileDir))

	// Frontera de la poda: todo canal de esta fuente cuyo last_seen_at quede
	// por debajo de este instante es que no apareció en este sync.
	inicio := time.Now()

	channels, err := provider.GetLiveChannels(ctx)
	if err != nil {
		return fmt.Errorf("services.sincronizarFuente (%s) GetLiveChannels: %w", fuente.ID, err)
	}

	// Rechazar ANTES de escribir nada: si el catálogo es sospechoso no se hace
	// upsert ni poda, y el backoff lo trata como el fallo que es. La
	// referencia es por fuente: el tamaño de una fuente no dice nada sobre lo
	// que debería tener otra.
	anterior := s.ultimoConteo[fuente.ID]
	if anterior > 0 && float64(len(channels)) < float64(anterior)*minRatioCatalogo {
		return fmt.Errorf("services.sincronizarFuente (%s): %w (recibidos %d, anterior %d)",
			fuente.ID, ErrCatalogoSospechoso, len(channels), anterior)
	}

	if err := s.channels.SaveBatch(ctx, channels); err != nil {
		return fmt.Errorf("services.sincronizarFuente (%s) canales: %w", fuente.ID, err)
	}

	streams := make([]domain.Stream, 0, len(channels))
	for _, ch := range channels {
		url, err := provider.GetStreamURL(ctx, ch.ID)
		if err != nil {
			// Canal sin URL en el caché del provider: raro pero no fatal.
			s.logger.Warn("Canal sin stream URL, omitido",
				slog.String("fuente", fuente.ID), slog.String("channel", string(ch.ID)))
			continue
		}
		streams = append(streams, domain.Stream{
			ID:        streamID(ch.ID, url),
			ChannelID: ch.ID,
			URL:       url,
			Protocol:  protocolFromURL(url),
		})
	}

	if err := s.streams.SaveBatch(ctx, streams); err != nil {
		return fmt.Errorf("services.sincronizarFuente (%s) streams: %w", fuente.ID, err)
	}

	// DeleteStale se scoped por el id de ESTA fuente: la poda nunca cruza
	// fuentes, así que un fallo o una fuente pequeña no puede arrastrar el
	// catálogo de otra.
	podados, err := s.channels.DeleteStale(ctx, fuente.ID, inicio)
	if err != nil {
		return fmt.Errorf("services.sincronizarFuente (%s) poda: %w", fuente.ID, err)
	}

	s.ultimoConteo[fuente.ID] = len(channels)

	s.logger.Info("Fuente sincronizada",
		slog.String("fuente", fuente.ID),
		slog.Int("canales", len(channels)),
		slog.Int("streams", len(streams)),
		slog.Int64("podados", podados))
	return nil
}

// streamID genera un ID determinista por (canal, URL): el mismo par produce
// el mismo ID en cada sync, de modo que el upsert actualiza en vez de duplicar.
func streamID(channelID domain.ChannelID, url string) string {
	h := fnv.New64a()
	// hash.Hash.Write nunca devuelve error (garantía de la interfaz io.Writer
	// para fnv): se descarta explícitamente en vez de comprobarlo.
	_, _ = h.Write([]byte(string(channelID)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(url))
	return fmt.Sprintf("st-%016x", h.Sum64())
}

// protocolFromURL infiere el protocolo por la URL. Las extensiones no
// reconocidas se tratan como HLS: el CHECK de la tabla streams solo admite
// HLS/DASH/RTMP y las listas FTA son HLS en su inmensa mayoría.
func protocolFromURL(url string) domain.Protocol {
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, ".mpd"):
		return domain.ProtocolDASH
	case strings.HasPrefix(lower, "rtmp://"):
		return domain.ProtocolRTMP
	default:
		return domain.ProtocolHLS
	}
}
