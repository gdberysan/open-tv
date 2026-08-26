package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

// ── fakes ────────────────────────────────────────────────────────────────────

// fakeSourceRepo implementa ports.SourceRepository en memoria. A diferencia
// de la implementación SQLite, aquí los tests fijan el ID de cada fuente a
// mano (Add no lo necesita para lo que estos tests ejercitan): lo que importa
// es que Syncer solo hable con el repo a través de la interfaz.
type fakeSourceRepo struct {
	mu      sync.Mutex
	fuentes []ports.Source
	touched map[string][]int64
}

func (f *fakeSourceRepo) List(context.Context) ([]ports.Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ports.Source(nil), f.fuentes...), nil
}

func (f *fakeSourceRepo) Add(_ context.Context, s ports.Source) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fuentes = append(f.fuentes, s)
	return nil
}

func (f *fakeSourceRepo) Remove(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.fuentes[:0]
	for _, s := range f.fuentes {
		if s.ID != id {
			out = append(out, s)
		}
	}
	f.fuentes = out
	return nil
}

func (f *fakeSourceRepo) TouchSync(_ context.Context, id string, cuando int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.touched == nil {
		f.touched = make(map[string][]int64)
	}
	f.touched[id] = append(f.touched[id], cuando)
	return nil
}

func (f *fakeSourceRepo) touchCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.touched[id])
}

type fakeChannelRepo struct {
	mu           sync.Mutex
	batches      [][]domain.Channel
	deleteStales []deleteStaleCall
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
func (f *fakeChannelRepo) CountFiltered(context.Context, ports.ChannelFilter) (int, error) {
	return 0, nil
}
func (f *fakeChannelRepo) Random(context.Context, ports.ChannelFilter) (domain.Channel, error) {
	return domain.Channel{}, nil
}

func (f *fakeChannelRepo) Countries(context.Context) ([]ports.Faceta, error)  { return nil, nil }
func (f *fakeChannelRepo) Categories(context.Context) ([]ports.Faceta, error) { return nil, nil }
func (f *fakeChannelRepo) Qualities(context.Context) ([]ports.Faceta, error)  { return nil, nil }

type deleteStaleCall struct {
	providerID string
	before     time.Time
}

func (f *fakeChannelRepo) DeleteStale(_ context.Context, providerID string, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteStales = append(f.deleteStales, deleteStaleCall{providerID, before})
	return 0, nil
}

func (f *fakeChannelRepo) staleCalls() []deleteStaleCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]deleteStaleCall(nil), f.deleteStales...)
}

func (f *fakeChannelRepo) batchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.batches)
}

// allChannelIDs junta los IDs de todos los batches persistidos, para
// comprobar la fusión de varias fuentes sin asumir cuántas llamadas a
// SaveBatch hizo cada una.
func (f *fakeChannelRepo) allChannelIDs() map[domain.ChannelID]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[domain.ChannelID]bool)
	for _, batch := range f.batches {
		for _, ch := range batch {
			out[ch.ID] = true
		}
	}
	return out
}

type fakeStreamRepo struct {
	mu      sync.Mutex
	batches [][]domain.Stream
}

func (f *fakeStreamRepo) Save(context.Context, domain.Stream) error { return nil }
func (f *fakeStreamRepo) FindAll(context.Context) ([]domain.Stream, error) {
	return nil, nil
}
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
func (f *fakeStreamRepo) FindMirrorsByChannelID(context.Context, domain.ChannelID) ([]ports.MirrorHealth, error) {
	return nil, nil
}
func (f *fakeStreamRepo) MarkAlive(context.Context, string, int64) error { return nil }
func (f *fakeStreamRepo) MarkDead(context.Context, string) error         { return nil }
func (f *fakeStreamRepo) MarkBatch(context.Context, []ports.StreamHealth) error {
	return nil
}

func (f *fakeStreamRepo) lastBatch() []domain.Stream {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.batches) == 0 {
		return nil
	}
	return f.batches[len(f.batches)-1]
}

func (f *fakeStreamRepo) batchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.batches)
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

// ── fixtures M3U servidas por httptest ──────────────────────────────────────

// m3uServer levanta un servidor HTTP que sirve un M3U con un canal por nombre
// en names. Se usa como "fuente" real: Syncer construye un
// opensource.Provider de verdad contra esta URL, exactamente como hará en
// producción.
func m3uServer(t *testing.T, names ...string) *httptest.Server {
	t.Helper()
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, n := range names {
		fmt.Fprintf(&b, "#EXTINF:-1,%s\nhttp://stream.example/%s.m3u8\n", n, n)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// failingServer simula una fuente caída (500 en cada petición).
func failingServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "caído a propósito", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fuente(id, url string) ports.Source {
	return ports.Source{ID: id, Label: id, URL: url, Kind: "url", IsActive: true}
}

// channelID reproduce el esquema id = "<fuenteID>-<nombre>" del parser M3U
// (opensource.parseM3UStream), para que los tests puedan comprobar de quién
// es cada canal persistido sin depender de internals del provider.
func channelID(fuenteID, nombre string) domain.ChannelID {
	return domain.ChannelID(fuenteID + "-" + nombre)
}

// ── tests: ciclo multi-fuente (SyncOnce) ────────────────────────────────────

// (a) Dos fuentes activas: un ciclo sincroniza ambas y el catálogo fusiona
// (canales de ambas presentes, namespaced por fuente).
func TestSyncOnce_DosFuentesActivas_SincronizaAmbasYFusiona(t *testing.T) {
	srvA := m3uServer(t, "Uno", "Dos")
	srvB := m3uServer(t, "Tres")

	sources := &fakeSourceRepo{fuentes: []ports.Source{
		fuente("src-a", srvA.URL),
		fuente("src-b", srvB.URL),
	}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, "", Config{})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	if chRepo.batchCount() != 2 {
		t.Fatalf("batches de canales = %d, quiero 2 (uno por fuente)", chRepo.batchCount())
	}
	ids := chRepo.allChannelIDs()
	for _, want := range []domain.ChannelID{
		channelID("src-a", "Uno"), channelID("src-a", "Dos"), channelID("src-b", "Tres"),
	} {
		if !ids[want] {
			t.Errorf("catálogo fusionado sin %q: %v", want, ids)
		}
	}

	if n := sources.touchCount("src-a"); n != 1 {
		t.Errorf("TouchSync(src-a) = %d veces, quiero 1", n)
	}
	if n := sources.touchCount("src-b"); n != 1 {
		t.Errorf("TouchSync(src-b) = %d veces, quiero 1", n)
	}

	// La poda queda scoped por fuente: cada DeleteStale lleva el id de SU
	// fuente, nunca el de la otra.
	stale := chRepo.staleCalls()
	if len(stale) != 2 {
		t.Fatalf("llamadas a DeleteStale = %d, quiero 2", len(stale))
	}
	got := map[string]bool{stale[0].providerID: true, stale[1].providerID: true}
	if !got["src-a"] || !got["src-b"] {
		t.Errorf("DeleteStale providerIDs = %v, quiero {src-a, src-b}", got)
	}
}

// (b) Cero fuentes: el ciclo es un no-op sin error, y no debe podar ni
// escribir nada (una instalación limpia no tiene catálogo que vaciar).
func TestSyncOnce_CeroFuentes_NoOpSinError(t *testing.T) {
	sources := &fakeSourceRepo{}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, "", Config{})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce con 0 fuentes debería ser un no-op, no un error: %v", err)
	}
	if n := chRepo.batchCount(); n != 0 {
		t.Errorf("batches de canales = %d, quiero 0", n)
	}
	if n := len(chRepo.staleCalls()); n != 0 {
		t.Errorf("llamadas a DeleteStale = %d, quiero 0 (nada que podar)", n)
	}
	if n := stRepo.batchCount(); n != 0 {
		t.Errorf("batches de streams = %d, quiero 0", n)
	}
}

// Una fuente inactiva no se sincroniza en el ciclo periódico.
func TestSyncOnce_FuenteInactiva_SeOmite(t *testing.T) {
	srv := m3uServer(t, "Uno")
	sources := &fakeSourceRepo{fuentes: []ports.Source{
		{ID: "src-a", URL: srv.URL, Kind: "url", IsActive: false},
	}}
	chRepo := &fakeChannelRepo{}
	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{})

	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}
	if n := chRepo.batchCount(); n != 0 {
		t.Errorf("una fuente inactiva no debe sincronizarse; batches = %d", n)
	}
	if n := sources.touchCount("src-a"); n != 0 {
		t.Errorf("TouchSync no debe llamarse para una fuente inactiva; llamadas = %d", n)
	}
}

// (d) El fallo de una fuente no aborta el ciclo: las demás se intentan y
// tienen éxito igualmente; el ciclo completo se reporta como fallido (para
// que Run() aplique el backoff existente), pero SOLO a la fuente sana se le
// hace TouchSync.
func TestSyncOnce_FalloDeUnaFuente_NoAbortaLasDemas(t *testing.T) {
	srvMala := failingServer(t)
	srvBuena := m3uServer(t, "Uno")

	sources := &fakeSourceRepo{fuentes: []ports.Source{
		fuente("src-mala", srvMala.URL),
		fuente("src-buena", srvBuena.URL),
	}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, "", Config{})
	err := s.SyncOnce(context.Background())
	if err == nil {
		t.Fatal("SyncOnce debería reportar el ciclo como fallido si una fuente falló")
	}

	if chRepo.batchCount() != 1 {
		t.Fatalf("batches de canales = %d, quiero 1 (solo la fuente buena)", chRepo.batchCount())
	}
	ids := chRepo.allChannelIDs()
	if !ids[channelID("src-buena", "Uno")] {
		t.Errorf("la fuente buena no persistió su canal: %v", ids)
	}

	if n := sources.touchCount("src-mala"); n != 0 {
		t.Errorf("TouchSync(src-mala) = %d, quiero 0 (falló)", n)
	}
	if n := sources.touchCount("src-buena"); n != 1 {
		t.Errorf("TouchSync(src-buena) = %d, quiero 1", n)
	}
}

// ── tests: SyncOne ───────────────────────────────────────────────────────────

// (c) SyncOne(id) sincroniza solo la fuente pedida, dejando la otra intacta.
func TestSyncOne_SincronizaSoloEsaFuente(t *testing.T) {
	srvA := m3uServer(t, "Uno")
	srvB := m3uServer(t, "Dos")

	sources := &fakeSourceRepo{fuentes: []ports.Source{
		fuente("src-a", srvA.URL),
		fuente("src-b", srvB.URL),
	}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, "", Config{})
	if err := s.SyncOne(context.Background(), "src-a"); err != nil {
		t.Fatalf("SyncOne: %v", err)
	}

	if chRepo.batchCount() != 1 {
		t.Fatalf("batches de canales = %d, quiero 1", chRepo.batchCount())
	}
	ids := chRepo.allChannelIDs()
	if !ids[channelID("src-a", "Uno")] {
		t.Errorf("SyncOne no persistió el canal de src-a: %v", ids)
	}
	if ids[channelID("src-b", "Dos")] {
		t.Errorf("SyncOne no debía tocar src-b: %v", ids)
	}
	if n := sources.touchCount("src-a"); n != 1 {
		t.Errorf("TouchSync(src-a) = %d, quiero 1", n)
	}
	if n := sources.touchCount("src-b"); n != 0 {
		t.Errorf("TouchSync(src-b) = %d, quiero 0", n)
	}
}

// SyncOne sincroniza una fuente aunque esté marcada inactiva: quien pide un
// id concreto ya sabe cuál quiere (a diferencia del ciclo periódico).
func TestSyncOne_SincronizaAunqueLaFuenteEsteInactiva(t *testing.T) {
	srv := m3uServer(t, "Uno")
	sources := &fakeSourceRepo{fuentes: []ports.Source{
		{ID: "src-a", URL: srv.URL, Kind: "url", IsActive: false},
	}}
	chRepo := &fakeChannelRepo{}
	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{})

	if err := s.SyncOne(context.Background(), "src-a"); err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if n := chRepo.batchCount(); n != 1 {
		t.Errorf("batches de canales = %d, quiero 1", n)
	}
}

// (e) SyncOne con un id desconocido devuelve un error reconocible, sin tocar
// ninguna fuente existente.
func TestSyncOne_IDDesconocido_DevuelveErrorReconocible(t *testing.T) {
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("src-a", "http://a.example/x.m3u")}}
	chRepo := &fakeChannelRepo{}
	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{})

	err := s.SyncOne(context.Background(), "no-existe")
	if !errors.Is(err, ErrFuenteNoEncontrada) {
		t.Fatalf("SyncOne(id inexistente) = %v, quiero que envuelva ErrFuenteNoEncontrada", err)
	}
	if n := chRepo.batchCount(); n != 0 {
		t.Errorf("un id desconocido no debe tocar ninguna fuente; batches = %d", n)
	}
}

// ── tests: persistencia, protocolo e IDs deterministas (vía un provider real) ─

func TestSyncOnce_PersisteCanalesYStreamsConProtocoloInferido(t *testing.T) {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXTINF:-1,Uno\nhttp://a.example/uno.m3u8\n")
	b.WriteString("#EXTINF:-1,Dos\nhttp://b.example/dos.mpd\n")
	b.WriteString("#EXTINF:-1,Tres\nhttp://c.example/tres.ts\n") // extensión desconocida
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(b.String()))
	}))
	t.Cleanup(srv.Close)

	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, "", Config{})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	if chRepo.batchCount() != 1 || len(chRepo.batches[0]) != 3 {
		t.Fatalf("canales: %d batches, quiero 1 batch de 3", chRepo.batchCount())
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
	if byChannel[channelID("fake", "Uno")].Protocol != domain.ProtocolHLS {
		t.Errorf("Uno: protocol = %q, want HLS", byChannel[channelID("fake", "Uno")].Protocol)
	}
	if byChannel[channelID("fake", "Dos")].Protocol != domain.ProtocolDASH {
		t.Errorf("Dos: protocol = %q, want DASH", byChannel[channelID("fake", "Dos")].Protocol)
	}
	// Extensión desconocida → HLS por defecto (el CHECK de la tabla solo
	// admite HLS/DASH/RTMP y las listas FTA son HLS en su inmensa mayoría)
	if byChannel[channelID("fake", "Tres")].Protocol != domain.ProtocolHLS {
		t.Errorf("Tres: protocol = %q, want HLS por defecto", byChannel[channelID("fake", "Tres")].Protocol)
	}
}

func TestSyncOnce_StreamIDsDeterministas(t *testing.T) {
	srv := m3uServer(t, "Uno", "Dos")
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, "", Config{})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce 1: %v", err)
	}
	first := stRepo.lastBatch()
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce 2: %v", err)
	}
	second := stRepo.lastBatch()

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

// ── tests: Run, backoff, periodicidad, FirstSyncDone, cancelación ──────────

func TestSyncer_Run_ReintentaConBackoffTrasFallo(t *testing.T) {
	var intentos int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		intentos++
		n := intentos
		mu.Unlock()
		if n <= 2 {
			http.Error(w, "caído", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:-1,Uno\nhttp://a.example/uno.m3u8\n"))
	}))
	t.Cleanup(srv.Close)

	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}
	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{
		Interval:  time.Hour, // que no re-sincronice durante el test
		RetryBase: time.Millisecond,
		RetryMax:  5 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	waitFor(t, 2*time.Second, func() bool { return chRepo.batchCount() >= 1 },
		"el sync nunca tuvo éxito pese a los reintentos")

	mu.Lock()
	got := intentos
	mu.Unlock()
	if got < 3 {
		t.Errorf("intentos = %d, want >= 3 (2 fallos + 1 éxito)", got)
	}
}

func TestSyncer_Run_ResincronizaPeriodicamente(t *testing.T) {
	srv := m3uServer(t, "Uno")
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}

	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{
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

// El health-worker debe esperar al primer ciclo: sin streams en DB no tendría
// nada que chequear y la primera pasada se desperdiciaría.
func TestSyncer_FirstSyncDone_SeCierraTrasElPrimerCicloExitoso(t *testing.T) {
	var intentos int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		intentos++
		n := intentos
		mu.Unlock()
		if n <= 1 {
			http.Error(w, "caído", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:-1,Uno\nhttp://a.example/uno.m3u8\n"))
	}))
	t.Cleanup(srv.Close)

	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	s := NewSyncer(nil, sources, &fakeChannelRepo{}, &fakeStreamRepo{}, "", Config{
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
		t.Fatal("FirstSyncDone no se cerró tras el primer ciclo exitoso")
	}
}

// FirstSyncDone tiene que dispararse tras el primer ciclo AUNQUE haya cero
// fuentes: si no, el health-worker de una instalación limpia se cuelga para
// siempre esperando un catálogo que nunca llega.
func TestSyncer_FirstSyncDone_ConCeroFuentes(t *testing.T) {
	s := NewSyncer(nil, &fakeSourceRepo{}, &fakeChannelRepo{}, &fakeStreamRepo{}, "", Config{
		Interval: time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	select {
	case <-s.FirstSyncDone():
	case <-time.After(2 * time.Second):
		t.Fatal("FirstSyncDone no se cerró tras el primer ciclo (0 fuentes) aunque debe considerarse éxito")
	}
}

func TestSyncer_Run_TerminaAlCancelarContexto(t *testing.T) {
	srv := m3uServer(t, "Uno")
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}

	s := NewSyncer(nil, sources, &fakeChannelRepo{}, &fakeStreamRepo{}, "", Config{
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

// ── poda y suelo de cordura (por fuente) ─────────────────────────────────────

// El ID de canal se deriva del nombre: un renombrado aguas arriba deja una fila
// huérfana que ya no se puede reproducir. Cada sync exitoso la barre.
func TestSincronizarFuente_PodaCanalesObsoletos(t *testing.T) {
	srv := m3uServer(t, "a", "b")
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}
	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{})

	antes := time.Now()
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	llamadas := chRepo.staleCalls()
	if len(llamadas) != 1 {
		t.Fatalf("quiero 1 llamada a DeleteStale, tengo %d", len(llamadas))
	}
	if llamadas[0].providerID != "fake" {
		t.Errorf("providerID = %q, quiero %q", llamadas[0].providerID, "fake")
	}
	if llamadas[0].before.Before(antes) {
		t.Errorf("el corte de poda (%v) debe ser posterior al arranque del sync (%v)",
			llamadas[0].before, antes)
	}
}

// Si la fuente falla no hay catálogo con el que comparar: podar borraría
// canales buenos por un fallo de red.
func TestSincronizarFuente_NoPodaSiLaFuenteFalla(t *testing.T) {
	srv := failingServer(t)
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}
	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{})

	if err := s.SyncOnce(context.Background()); err == nil {
		t.Fatal("SyncOnce debería fallar si la fuente falla")
	}
	if n := len(chRepo.staleCalls()); n != 0 {
		t.Errorf("no se debe podar tras un sync fallido; hubo %d llamadas", n)
	}
}

// Un 200 con una página HTML de error, un portal cautivo o un cuerpo truncado
// produce cero (o muy pocos) canales. Sin suelo de cordura eso cuenta como
// sync exitoso y, con la poda activa, borraría el catálogo entero de la fuente.
func TestSincronizarFuente_RechazaUnCatalogoAnomalamentePequeno(t *testing.T) {
	var chico bool
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		esChico := chico
		mu.Unlock()
		if esChico {
			_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:-1,a\nhttp://a/1.m3u8\n"))
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n" +
			"#EXTINF:-1,a\nhttp://a/1.m3u8\n" +
			"#EXTINF:-1,b\nhttp://b/1.m3u8\n" +
			"#EXTINF:-1,c\nhttp://c/1.m3u8\n" +
			"#EXTINF:-1,d\nhttp://d/1.m3u8\n"))
	}))
	t.Cleanup(srv.Close)

	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}
	s := NewSyncer(nil, sources, chRepo, &fakeStreamRepo{}, "", Config{})

	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("primer SyncOnce: %v", err)
	}
	llamadasTrasPrimero := len(chRepo.staleCalls())
	lotesTrasPrimero := chRepo.batchCount()

	mu.Lock()
	chico = true
	mu.Unlock()

	err := s.SyncOnce(context.Background())
	if !errors.Is(err, ErrCatalogoSospechoso) {
		t.Fatalf("SyncOnce = %v, quiero que envuelva ErrCatalogoSospechoso", err)
	}
	if n := len(chRepo.staleCalls()); n != llamadasTrasPrimero {
		t.Errorf("un catálogo sospechoso no debe podar nada; llamadas nuevas: %d", n-llamadasTrasPrimero)
	}
	if n := chRepo.batchCount(); n != lotesTrasPrimero {
		t.Errorf("un catálogo sospechoso no debe escribir nada; lotes nuevos: %d", n-lotesTrasPrimero)
	}
}

// El primer sync de la vida de una fuente no tiene con qué comparar.
func TestSincronizarFuente_PrimerSyncSiempreSeAcepta(t *testing.T) {
	srv := m3uServer(t, "uno")
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	s := NewSyncer(nil, sources, &fakeChannelRepo{}, &fakeStreamRepo{}, "", Config{})

	if err := s.SyncOnce(context.Background()); err != nil {
		t.Errorf("el primer sync no tiene referencia previa; debe aceptarse: %v", err)
	}
}

// ── concurrencia: SyncOne y el ciclo periódico no deben pisarse ─────────────

// El futuro handler HTTP de re-sync manual llamará SyncOne mientras Run()
// sigue su ciclo en background. syncMu tiene que bastar para que -race no
// encuentre ningún acceso concurrente a ultimoConteo ni a los repos.
func TestSyncer_SyncOneConcurrenteConElCicloPeriodico_SinCarreras(t *testing.T) {
	srv := m3uServer(t, "Uno", "Dos")
	sources := &fakeSourceRepo{fuentes: []ports.Source{fuente("fake", srv.URL)}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, "", Config{
		Interval:  2 * time.Millisecond,
		RetryBase: time.Millisecond,
		RetryMax:  time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)

	var wg sync.WaitGroup
	var erroresInesperados atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if err := s.SyncOne(context.Background(), "fake"); err != nil {
					erroresInesperados.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	cancel()

	if n := erroresInesperados.Load(); n != 0 {
		t.Errorf("SyncOne concurrente devolvió %d errores inesperados", n)
	}
}
