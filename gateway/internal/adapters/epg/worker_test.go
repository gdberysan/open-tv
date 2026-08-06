package epg

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

type fakeEPGRepo struct {
	mu          sync.Mutex
	batches     [][]domain.EPGEntry
	prunedUntil time.Time
}

func (f *fakeEPGRepo) SaveBatch(_ context.Context, entries []domain.EPGEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	batch := make([]domain.EPGEntry, len(entries))
	copy(batch, entries)
	f.batches = append(f.batches, batch)
	return nil
}

func (f *fakeEPGRepo) FindByChannelAndWindow(context.Context, domain.ChannelID, time.Time, time.Time) ([]domain.EPGEntry, error) {
	return nil, nil
}

func (f *fakeEPGRepo) FindCurrentlyAiring(context.Context, time.Time) ([]domain.EPGEntry, error) {
	return nil, nil
}

func (f *fakeEPGRepo) DeleteEndedBefore(_ context.Context, t time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prunedUntil = t
	return 0, nil
}

func (f *fakeEPGRepo) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, b := range f.batches {
		n += len(b)
	}
	return n
}

// xmltvCon genera un XMLTV con n programmes de una hora.
func xmltvCon(n int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><tv>`)
	for i := 0; i < n; i++ {
		sb.WriteString(fmt.Sprintf(
			`<programme channel="X.uk" start="202608%02d000000 +0000" stop="202608%02d010000 +0000">`+
				`<title>Programa %d</title><desc>Desc</desc></programme>`,
			(i%27)+1, (i%27)+1, i))
	}
	sb.WriteString(`</tv>`)
	return sb.String()
}

func newTestWorker(url string, repo *fakeEPGRepo) *Worker {
	logger := slog.New(slog.DiscardHandler)
	return NewWorker(NewParser(), repo, url, time.Hour, logger)
}

func TestWorker_PersisteEntradasEnBatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xmltvCon(1200)))
	}))
	defer server.Close()

	repo := &fakeEPGRepo{}
	w := newTestWorker(server.URL, repo)
	w.runSync(context.Background())

	if got := repo.total(); got != 1200 {
		t.Errorf("entradas persistidas = %d, want 1200", got)
	}
	// 1200 con lotes de 500 → 500 + 500 + 200
	if len(repo.batches) != 3 {
		t.Errorf("batches = %d, want 3", len(repo.batches))
	}
	if repo.batches[0][0].Title != "Programa 0" {
		t.Errorf("primera entrada = %q", repo.batches[0][0].Title)
	}
	// El ChannelID de las entradas es el tvg-id del XMLTV
	if repo.batches[0][0].ChannelID != "X.uk" {
		t.Errorf("ChannelID = %q, want el tvg-id X.uk", repo.batches[0][0].ChannelID)
	}
}

func TestWorker_SoportaGzipTransparente(t *testing.T) {
	// Las fuentes EPG reales sirven .xml.gz; el worker debe descomprimir
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte(xmltvCon(3)))
	_ = gz.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	repo := &fakeEPGRepo{}
	w := newTestWorker(server.URL, repo)
	w.runSync(context.Background())

	if got := repo.total(); got != 3 {
		t.Errorf("entradas desde gzip = %d, want 3", got)
	}
}

func TestWorker_PurgaProgramasViejosTrasSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xmltvCon(1)))
	}))
	defer server.Close()

	repo := &fakeEPGRepo{}
	w := newTestWorker(server.URL, repo)
	antes := time.Now()
	w.runSync(context.Background())

	if repo.prunedUntil.IsZero() {
		t.Fatal("el worker debe purgar programas terminados tras el sync")
	}
	if repo.prunedUntil.After(antes) {
		t.Errorf("prunedUntil = %v: purgar el pasado, no el futuro", repo.prunedUntil)
	}
}
