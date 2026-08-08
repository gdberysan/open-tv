package validator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

type fakeStreamRepo struct {
	mu      sync.Mutex
	streams []domain.Stream
	alive   map[string]int64 // streamID → latencia
	dead    map[string]bool
}

func newFakeStreamRepo(streams []domain.Stream) *fakeStreamRepo {
	return &fakeStreamRepo{
		streams: streams,
		alive:   make(map[string]int64),
		dead:    make(map[string]bool),
	}
}

func (f *fakeStreamRepo) Save(context.Context, domain.Stream) error        { return nil }
func (f *fakeStreamRepo) SaveBatch(context.Context, []domain.Stream) error { return nil }
func (f *fakeStreamRepo) FindAll(context.Context) ([]domain.Stream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.streams, nil
}
func (f *fakeStreamRepo) FindByChannelID(context.Context, domain.ChannelID) ([]domain.Stream, error) {
	return nil, nil
}
func (f *fakeStreamRepo) FindBestByChannelID(context.Context, domain.ChannelID) (domain.Stream, error) {
	return domain.Stream{}, errors.New("no impl")
}
func (f *fakeStreamRepo) MarkAlive(_ context.Context, id string, latencyMs int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alive[id] = latencyMs
	return nil
}

// MarkBatch delega en MarkAlive/MarkDead para que las aserciones existentes
// sobre los mapas alive/dead sigan valiendo.
func (f *fakeStreamRepo) MarkBatch(ctx context.Context, resultados []ports.StreamHealth) error {
	for _, r := range resultados {
		var err error
		if r.IsAlive {
			err = f.MarkAlive(ctx, r.StreamID, r.LatencyMs)
		} else {
			err = f.MarkDead(ctx, r.StreamID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeStreamRepo) MarkDead(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dead[id] = true
	return nil
}

func (f *fakeStreamRepo) estado() (alive map[string]int64, dead map[string]bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	alive = make(map[string]int64, len(f.alive))
	for k, v := range f.alive {
		alive[k] = v
	}
	dead = make(map[string]bool, len(f.dead))
	for k, v := range f.dead {
		dead[k] = v
	}
	return alive, dead
}

func stream(id, channel, url string) domain.Stream {
	return domain.Stream{ID: id, ChannelID: domain.ChannelID(channel), URL: url, Protocol: domain.ProtocolHLS}
}

func testWorker(repo *fakeStreamRepo, interval time.Duration) *Worker {
	cfg := Config{MaxWorkers: 4, Timeout: 2 * time.Second}
	return NewWorker(repo, cfg, interval, nil)
}

func TestWorker_MarcaVivosYMuertos(t *testing.T) {
	var hits atomic.Int32
	aliveServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer aliveServer.Close()
	deadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer deadServer.Close()

	repo := newFakeStreamRepo([]domain.Stream{
		stream("st-vivo", "ch-1", aliveServer.URL+"/ok.m3u8"),
		stream("st-muerto", "ch-2", deadServer.URL+"/gone.m3u8"),
		// Mismo URL que st-vivo desde otro canal: debe chequearse UNA vez
		// y marcarse vivo en ambos streams.
		stream("st-vivo-dup", "ch-3", aliveServer.URL+"/ok.m3u8"),
	})

	w := testWorker(repo, time.Hour)
	w.checkOnce(context.Background())

	alive, dead := repo.estado()
	if _, ok := alive["st-vivo"]; !ok {
		t.Error("st-vivo debía marcarse vivo")
	}
	if _, ok := alive["st-vivo-dup"]; !ok {
		t.Error("st-vivo-dup comparte URL viva: debía marcarse vivo")
	}
	if !dead["st-muerto"] {
		t.Error("st-muerto debía marcarse muerto")
	}
	if dead["st-vivo"] || dead["st-vivo-dup"] {
		t.Error("streams vivos marcados muertos")
	}
	// Dedupe: la URL viva se chequea una sola vez (1 HEAD, sin fallback GET)
	if got := hits.Load(); got != 1 {
		t.Errorf("requests al server vivo = %d, want 1 (URL deduplicada)", got)
	}
}

// Sin límite por host, 50 workers golpeando al mismo servidor (jmp2.uk aloja
// 1400+ streams) disparan los limit_conn de nginx: el servidor rechaza
// conexiones ("connection refused") tanto al checker (falsos muertos) como al
// usuario reproduciendo en paralelo. Reproducido en vivo el 2026-08-06.
func TestWorker_LimitaConexionesPorHost(t *testing.T) {
	var current, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := current.Add(1)
		for {
			p := peak.Load()
			if c <= p || peak.CompareAndSwap(p, c) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		current.Add(-1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	streams := make([]domain.Stream, 24)
	for i := range streams {
		streams[i] = stream(fmt.Sprintf("st-%d", i), fmt.Sprintf("ch-%d", i),
			fmt.Sprintf("%s/stream-%d.m3u8", server.URL, i))
	}
	repo := newFakeStreamRepo(streams)

	w := NewWorker(repo, Config{MaxWorkers: 12, Timeout: 5 * time.Second}, time.Hour, nil)
	w.checkOnce(context.Background())

	alive, _ := repo.estado()
	if len(alive) != 24 {
		t.Fatalf("vivos = %d, want 24", len(alive))
	}
	if got := peak.Load(); got > maxConnsPerHost {
		t.Errorf("pico de conexiones simultáneas al mismo host = %d, want <= %d", got, maxConnsPerHost)
	}
}

func TestWorker_Start_EjecutaPeriodicamente(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := newFakeStreamRepo([]domain.Stream{stream("st-1", "ch-1", server.URL+"/a.m3u8")})
	w := testWorker(repo, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for hits.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hits.Load() < 2 {
		t.Fatalf("checks = %d, want >= 2 (pasada inicial + periódica)", hits.Load())
	}
}

func TestWorker_Start_TerminaAlCancelar(t *testing.T) {
	repo := newFakeStreamRepo(nil)
	w := testWorker(repo, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start no terminó tras cancelar el contexto")
	}
}
