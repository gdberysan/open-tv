// Package services contiene los casos de uso que orquestan adapters y ports.
package services

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

// ErrCatalogoSospechoso se devuelve cuando el proveedor entrega muchos menos
// canales que en el último sync aceptado. Un 200 con una página de error, un
// portal cautivo o un cuerpo truncado produce un M3U válido pero casi vacío;
// sin este suelo eso contaría como éxito, resetearía el backoff a la cadencia
// normal y —con la poda activa— borraría el catálogo entero.
var ErrCatalogoSospechoso = errors.New("catálogo anómalamente pequeño")

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
	// SyncTimeout acota cada intento completo. Default: 5min.
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

// Syncer mantiene la DB de canales y streams al día con el proveedor.
// Sustituye al sync único de boot: reintenta con backoff exponencial si el
// proveedor falla y re-sincroniza periódicamente si tiene éxito.
type Syncer struct {
	logger   *slog.Logger
	provider ports.ProviderPort
	channels ports.ChannelRepository
	streams  ports.StreamRepository
	cfg      Config

	firstDone chan struct{}
	firstOnce sync.Once

	mu          sync.RWMutex
	lastSuccess time.Time

	// ultimoConteo es el número de canales del último sync aceptado. Cero
	// significa que aún no hay referencia con la que comparar. Solo lo toca
	// SyncOnce, que corre en serie dentro del bucle de Run.
	ultimoConteo int
}

func NewSyncer(logger *slog.Logger, provider ports.ProviderPort, channels ports.ChannelRepository, streams ports.StreamRepository, cfg Config) *Syncer {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Syncer{
		logger:    logger,
		provider:  provider,
		channels:  channels,
		streams:   streams,
		cfg:       cfg.withDefaults(),
		firstDone: make(chan struct{}),
	}
}

// FirstSyncDone se cierra tras el primer sync exitoso. Permite a trabajos
// dependientes (p.ej. el worker EPG, que necesita tvg_id en DB) esperar a
// que exista el catálogo de canales antes de arrancar.
func (s *Syncer) FirstSyncDone() <-chan struct{} {
	return s.firstDone
}

// LastSuccess devuelve el instante del último sync exitoso. Cero si aún no ha
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
			s.logger.Error("Sync fallido, reintentando",
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

// SyncOnce descarga los canales del proveedor y persiste canales y streams.
func (s *Syncer) SyncOnce(ctx context.Context) error {
	// Frontera de la poda: todo canal cuyo last_seen_at quede por debajo de
	// este instante es que no apareció en este sync.
	inicio := time.Now()

	channels, err := s.provider.GetLiveChannels(ctx)
	if err != nil {
		return fmt.Errorf("services.SyncOnce (GetLiveChannels): %w", err)
	}

	// Rechazar ANTES de escribir nada: si el catálogo es sospechoso no se hace
	// upsert ni poda, y el backoff lo trata como el fallo que es.
	if s.ultimoConteo > 0 &&
		float64(len(channels)) < float64(s.ultimoConteo)*minRatioCatalogo {
		return fmt.Errorf("services.SyncOnce: %w (recibidos %d, anterior %d)",
			ErrCatalogoSospechoso, len(channels), s.ultimoConteo)
	}

	if err := s.channels.SaveBatch(ctx, channels); err != nil {
		return fmt.Errorf("services.SyncOnce (canales): %w", err)
	}

	streams := make([]domain.Stream, 0, len(channels))
	for _, ch := range channels {
		url, err := s.provider.GetStreamURL(ctx, ch.ID)
		if err != nil {
			// Canal sin URL en el caché del provider: raro pero no fatal
			s.logger.Warn("Canal sin stream URL, omitido", slog.String("channel", string(ch.ID)))
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
		return fmt.Errorf("services.SyncOnce (streams): %w", err)
	}

	podados, err := s.channels.DeleteStale(ctx, s.provider.ID(), inicio)
	if err != nil {
		return fmt.Errorf("services.SyncOnce (poda): %w", err)
	}

	s.ultimoConteo = len(channels)

	s.logger.Info("Sync completado",
		slog.Int("canales", len(channels)),
		slog.Int("streams", len(streams)),
		slog.Int64("podados", podados))
	return nil
}

// streamID genera un ID determinista por (canal, URL): el mismo par produce
// el mismo ID en cada sync, de modo que el upsert actualiza en vez de duplicar.
func streamID(channelID domain.ChannelID, url string) string {
	h := fnv.New64a()
	h.Write([]byte(string(channelID)))
	h.Write([]byte{0})
	h.Write([]byte(url))
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
