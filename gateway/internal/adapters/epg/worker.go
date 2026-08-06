package epg

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

type Worker struct {
	epgPort ports.EPGPort
	url     string
	ticker  *time.Ticker
	logger  *slog.Logger
	client  *http.Client
}

func NewWorker(epgPort ports.EPGPort, url string, interval time.Duration, logger *slog.Logger) *Worker {
	return &Worker{
		epgPort: epgPort,
		url:     url,
		ticker:  time.NewTicker(interval),
		logger:  logger,
		client:  &http.Client{Timeout: 30 * time.Second}, // Timeout para la descarga inicial
	}
}

// Start arranca el ciclo del worker en background.
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("EPG Worker iniciado", slog.String("interval", "ticker"))

	// Disparo inicial inmediato (opcional)
	go w.runSync(ctx)

	for {
		select {
		case <-ctx.Done():
			w.ticker.Stop()
			w.logger.Info("EPG Worker detenido")
			return
		case <-w.ticker.C:
			w.runSync(ctx)
		}
	}
}

func (w *Worker) runSync(ctx context.Context) {
	w.logger.Info("Iniciando descarga y parseo EPG", slog.String("url", w.url))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.url, nil)
	if err != nil {
		w.logger.Error("Error creando request", slog.Any("error", err))
		return
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Error("Error descargando XMLTV", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.logger.Error("Error XMLTV HTTP Status", slog.Int("status", resp.StatusCode))
		return
	}

	// Lote para insertar en batch (ej. cada 500)
	const batchSize = 500
	var batch []domain.EPGEntry
	var totalProcessed int

	onEntry := func(entry domain.EPGEntry) error {
		batch = append(batch, entry)
		
		if len(batch) >= batchSize {
			// AQUI: En un sistema real llamaríamos a h.repository.SaveBatch(ctx, batch)
			// w.repo.SaveBatch(ctx, batch)
			
			totalProcessed += len(batch)
			batch = batch[:0] // Limpiar sin reasignar memoria
		}
		return nil
	}

	if err := w.epgPort.ParseStream(ctx, resp.Body, onEntry); err != nil {
		w.logger.Error("Error parseando flujo XMLTV", slog.Any("error", err))
		return
	}

	// Insertar los restantes
	if len(batch) > 0 {
		// w.repo.SaveBatch(ctx, batch)
		totalProcessed += len(batch)
	}

	w.logger.Info("Sincronización EPG finalizada con éxito", slog.Int("entries_processed", totalProcessed))
}
