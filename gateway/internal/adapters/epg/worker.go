package epg

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

type Worker struct {
	epgPort ports.EPGPort
	repo    ports.EPGRepository
	url     string
	ticker  *time.Ticker
	logger  *slog.Logger
	client  *http.Client
}

func NewWorker(epgPort ports.EPGPort, repo ports.EPGRepository, url string, interval time.Duration, logger *slog.Logger) *Worker {
	return &Worker{
		epgPort: epgPort,
		repo:    repo,
		url:     url,
		ticker:  time.NewTicker(interval),
		logger:  logger,
		// Los XMLTV combinados pueden pesar cientos de MB: el timeout debe
		// cubrir la descarga completa, no solo el primer byte.
		client: &http.Client{Timeout: 10 * time.Minute},
	}
}

// Start arranca el ciclo del worker en background.
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("EPG Worker iniciado", slog.String("url", w.url))

	// Disparo inicial inmediato
	w.runSync(ctx)

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

	body, err := maybeGunzip(resp.Body)
	if err != nil {
		w.logger.Error("Error abriendo gzip XMLTV", slog.Any("error", err))
		return
	}

	// Lote para insertar en batch cada 500 (ADR-001)
	const batchSize = 500
	var batch []domain.EPGEntry
	var totalPersisted int

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := w.repo.SaveBatch(ctx, batch); err != nil {
			return err
		}
		totalPersisted += len(batch)
		batch = batch[:0] // Limpiar sin reasignar memoria
		return nil
	}

	onEntry := func(entry domain.EPGEntry) error {
		batch = append(batch, entry)
		if len(batch) >= batchSize {
			return flush()
		}
		return nil
	}

	if err := w.epgPort.ParseStream(ctx, body, onEntry); err != nil {
		w.logger.Error("Error parseando flujo XMLTV", slog.Any("error", err))
		return
	}

	// Insertar los restantes
	if err := flush(); err != nil {
		w.logger.Error("Error persistiendo lote EPG final", slog.Any("error", err))
		return
	}

	// Purgar programas terminados hace más de 24h: el EPG crece sin límite
	// si no se poda tras cada sync.
	pruned, err := w.repo.DeleteEndedBefore(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		w.logger.Error("Error purgando EPG viejo", slog.Any("error", err))
	}

	w.logger.Info("Sincronización EPG finalizada",
		slog.Int("entries_persisted", totalPersisted), slog.Int64("pruned", pruned))
}

// maybeGunzip detecta por los magic bytes (1f 8b) si el cuerpo viene
// comprimido — las fuentes EPG reales sirven .xml.gz — y lo descomprime.
func maybeGunzip(r io.Reader) (io.Reader, error) {
	br := bufio.NewReader(r)
	magic, err := br.Peek(2)
	if err != nil {
		// Cuerpo más corto que 2 bytes: que el parser XML reporte el error
		return br, nil
	}
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return gzip.NewReader(br)
	}
	return br, nil
}
