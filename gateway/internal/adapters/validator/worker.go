package validator

import (
	"context"
	"log/slog"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

// Worker ejecuta el health-check periódico de streams: valida cada URL con
// el pool concurrente (HEAD→GET) y persiste el resultado vía MarkAlive /
// MarkDead, alimentando FindBestByChannelID y el filtro AliveOnly.
type Worker struct {
	repo      ports.StreamRepository
	validator *Validator
	interval  time.Duration
	logger    *slog.Logger
}

func NewWorker(repo ports.StreamRepository, cfg Config, interval time.Duration, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Worker{
		repo:      repo,
		validator: NewValidator(cfg, nil),
		interval:  interval,
		logger:    logger,
	}
}

// Start ejecuta una pasada inmediata y luego repite cada interval hasta que
// ctx se cancele.
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("Health-check worker iniciado", slog.Duration("interval", w.interval))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.checkOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Health-check worker detenido")
			return
		case <-ticker.C:
			w.checkOnce(ctx)
		}
	}
}

// checkOnce valida todos los streams del catálogo. Cada URL única se chequea
// una sola vez y el resultado se aplica a todos los streams que la comparten.
func (w *Worker) checkOnce(ctx context.Context) {
	streams, err := w.repo.FindAll(ctx)
	if err != nil {
		w.logger.Error("Health-check: error listando streams", slog.Any("error", err))
		return
	}
	if len(streams) == 0 {
		return
	}

	byURL := make(map[string][]string, len(streams))
	for _, s := range streams {
		byURL[s.URL] = append(byURL[s.URL], s.ID)
	}

	urls := make(chan string)
	go func() {
		defer close(urls)
		for u := range byURL {
			select {
			case urls <- u:
			case <-ctx.Done():
				return
			}
		}
	}()

	var alive, dead int
	resultados := make([]ports.StreamHealth, 0, len(streams))
	for res := range w.validator.Start(ctx, urls) {
		for _, id := range byURL[res.URL] {
			resultados = append(resultados, ports.StreamHealth{
				StreamID:  id,
				IsAlive:   res.IsAlive,
				LatencyMs: res.LatencyMs,
			})
			if res.IsAlive {
				alive++
			} else {
				dead++
			}
		}
	}

	// Una transacción para toda la pasada en vez de ~12k UPDATEs sueltos: con
	// MaxOpenConns(1), cada escritura suelta es tiempo en el que la API no lee.
	if err := w.repo.MarkBatch(ctx, resultados); err != nil {
		w.logger.Error("Health-check: fallo persistiendo resultados", slog.Any("error", err))
		return
	}

	w.logger.Info("Health-check completado",
		slog.Int("streams", len(streams)),
		slog.Int("urls_unicas", len(byURL)),
		slog.Int("vivos", alive),
		slog.Int("muertos", dead))
}
