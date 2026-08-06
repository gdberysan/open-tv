package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeProvider struct {
	mu        sync.Mutex
	calls     int
	failFirst int // cuántas llamadas iniciales a GetLiveChannels fallan
	channels  []domain.Channel
	urls      map[domain.ChannelID]string
}

func (f *fakeProvider) ID() string                        { return "fake" }
func (f *fakeProvider) Type() domain.ProviderType         { return domain.ProviderOpenSource }
func (f *fakeProvider) HealthCheck(context.Context) error { return nil }

func (f *fakeProvider) GetLiveChannels(context.Context) ([]domain.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failFirst {
		return nil, errors.New("provider caído")
	}
	return f.channels, nil
}

func (f *fakeProvider) GetStreamURL(_ context.Context, id domain.ChannelID) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.urls[id]
	if !ok {
		return "", errors.New("sin URL")
	}
	return u, nil
}

func (f *fakeProvider) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeChannelRepo struct {
	mu      sync.Mutex
	batches [][]domain.Channel
}

func (f *fakeChannelRepo) Save(context.Context, domain.Channel) error { return nil }
func (f *fakeChannelRepo) SaveBatch(_ context.Context, chs []domain.Channel) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, chs)
	return nil
}
func (f *fakeChannelRepo) FindByID(context.Context, domain.ChannelID) (domain.Channel, error) {
	return domain.Channel{}, nil
}
func (f *fakeChannelRepo) FindFiltered(context.Context, ports.ChannelFilter) ([]domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) Search(context.Context, string, int) ([]domain.Channel, error) {
	return nil, nil
}
func (f *fakeChannelRepo) Delete(context.Context, domain.ChannelID) error { return nil }

func (f *fakeChannelRepo) batchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.batches)
}

type fakeStreamRepo struct {
	mu      sync.Mutex
	batches [][]domain.Stream
}

func (f *fakeStreamRepo) Save(context.Context, domain.Stream) error { return nil }
func (f *fakeStreamRepo) SaveBatch(_ context.Context, ss []domain.Stream) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, ss)
	return nil
}
func (f *fakeStreamRepo) FindByChannelID(context.Context, domain.ChannelID) ([]domain.Stream, error) {
	return nil, nil
}
func (f *fakeStreamRepo) FindBestByChannelID(context.Context, domain.ChannelID) (domain.Stream, error) {
	return domain.Stream{}, errors.New("sin streams")
}
func (f *fakeStreamRepo) MarkAlive(context.Context, string, int64) error { return nil }
func (f *fakeStreamRepo) MarkDead(context.Context, string) error         { return nil }

func (f *fakeStreamRepo) lastBatch() []domain.Stream {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.batches) == 0 {
		return nil
	}
	return f.batches[len(f.batches)-1]
}

// waitFor sondea cond hasta que sea cierta o venza el deadline.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitFor: %s", msg)
}

// ── tests ────────────────────────────────────────────────────────────────────

func testChannels() ([]domain.Channel, map[domain.ChannelID]string) {
	chs := []domain.Channel{
		{ID: "fake-Uno", Name: "Uno", ProviderID: "fake", ProviderType: domain.ProviderOpenSource},
		{ID: "fake-Dos", Name: "Dos", ProviderID: "fake", ProviderType: domain.ProviderOpenSource},
		{ID: "fake-Tres", Name: "Tres", ProviderID: "fake", ProviderType: domain.ProviderOpenSource},
	}
	urls := map[domain.ChannelID]string{
		"fake-Uno":  "http://a.example/uno.m3u8",
		"fake-Dos":  "http://b.example/dos.mpd",
		"fake-Tres": "http://c.example/tres.ts", // extensión desconocida
	}
	return chs, urls
}

func TestSyncer_SyncOnce_PersistsChannelsAndStreams(t *testing.T) {
	chs, urls := testChannels()
	prov := &fakeProvider{channels: chs, urls: urls}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, prov, chRepo, stRepo, Config{})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	if chRepo.batchCount() != 1 || len(chRepo.batches[0]) != 3 {
		t.Fatalf("canales: %d batches, quiere 1 batch de 3", chRepo.batchCount())
	}

	streams := stRepo.lastBatch()
	if len(streams) != 3 {
		t.Fatalf("streams persistidos = %d, want 3", len(streams))
	}
	byChannel := make(map[domain.ChannelID]domain.Stream)
	for _, st := range streams {
		if st.ID == "" {
			t.Errorf("stream sin ID: %+v", st)
		}
		byChannel[st.ChannelID] = st
	}
	if byChannel["fake-Uno"].Protocol != domain.ProtocolHLS {
		t.Errorf("Uno: protocol = %q, want HLS", byChannel["fake-Uno"].Protocol)
	}
	if byChannel["fake-Dos"].Protocol != domain.ProtocolDASH {
		t.Errorf("Dos: protocol = %q, want DASH", byChannel["fake-Dos"].Protocol)
	}
	// Extensión desconocida → HLS por defecto (el CHECK de la tabla solo
	// admite HLS/DASH/RTMP y las listas FTA son HLS en su inmensa mayoría)
	if byChannel["fake-Tres"].Protocol != domain.ProtocolHLS {
		t.Errorf("Tres: protocol = %q, want HLS por defecto", byChannel["fake-Tres"].Protocol)
	}
	if byChannel["fake-Uno"].URL != "http://a.example/uno.m3u8" {
		t.Errorf("Uno: URL = %q", byChannel["fake-Uno"].URL)
	}
}

func TestSyncer_SyncOnce_StreamIDsDeterministas(t *testing.T) {
	chs, urls := testChannels()
	prov := &fakeProvider{channels: chs, urls: urls}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, prov, chRepo, stRepo, Config{})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce 1: %v", err)
	}
	first := stRepo.lastBatch()
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce 2: %v", err)
	}
	second := stRepo.lastBatch()

	// Mismo canal+URL debe producir el mismo ID en cada sync,
	// para que el upsert en DB actualice en vez de duplicar.
	ids := func(ss []domain.Stream) map[domain.ChannelID]string {
		m := make(map[domain.ChannelID]string)
		for _, st := range ss {
			m[st.ChannelID] = st.ID
		}
		return m
	}
	f, sec := ids(first), ids(second)
	for ch, id := range f {
		if sec[ch] != id {
			t.Errorf("canal %s: ID cambió entre syncs (%q → %q)", ch, id, sec[ch])
		}
	}
}

func TestSyncer_Run_ReintentaConBackoffTrasFallo(t *testing.T) {
	chs, urls := testChannels()
	prov := &fakeProvider{channels: chs, urls: urls, failFirst: 2}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, prov, chRepo, stRepo, Config{
		Interval:  time.Hour, // que no re-sincronice durante el test
		RetryBase: time.Millisecond,
		RetryMax:  5 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	waitFor(t, 2*time.Second, func() bool { return chRepo.batchCount() >= 1 },
		"el sync nunca tuvo éxito pese a los reintentos")

	if got := prov.callCount(); got < 3 {
		t.Errorf("intentos = %d, want >= 3 (2 fallos + 1 éxito)", got)
	}
}

func TestSyncer_Run_ResincronizaPeriodicamente(t *testing.T) {
	chs, urls := testChannels()
	prov := &fakeProvider{channels: chs, urls: urls}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, prov, chRepo, stRepo, Config{
		Interval:  5 * time.Millisecond,
		RetryBase: time.Millisecond,
		RetryMax:  time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	waitFor(t, 2*time.Second, func() bool { return chRepo.batchCount() >= 2 },
		"no hubo re-sync periódico tras el primer éxito")
}

// El worker EPG debe esperar al primer sync de canales: sin tvg_id en DB,
// todas las entradas EPG se descartarían en silencio.
func TestSyncer_FirstSyncDone_SeCierraTrasElPrimerExito(t *testing.T) {
	chs, urls := testChannels()
	prov := &fakeProvider{channels: chs, urls: urls, failFirst: 1}
	s := NewSyncer(nil, prov, &fakeChannelRepo{}, &fakeStreamRepo{}, Config{
		Interval:  time.Hour,
		RetryBase: time.Millisecond,
		RetryMax:  time.Millisecond,
	})

	select {
	case <-s.FirstSyncDone():
		t.Fatal("FirstSyncDone no debe estar cerrado antes de sincronizar")
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	select {
	case <-s.FirstSyncDone():
	case <-time.After(2 * time.Second):
		t.Fatal("FirstSyncDone no se cerró tras el primer sync exitoso")
	}
}

func TestSyncer_Run_TerminaAlCancelarContexto(t *testing.T) {
	chs, urls := testChannels()
	prov := &fakeProvider{channels: chs, urls: urls}

	s := NewSyncer(nil, prov, &fakeChannelRepo{}, &fakeStreamRepo{}, Config{
		Interval: time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run no terminó tras cancelar el contexto")
	}
}
