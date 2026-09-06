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

	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

type fakeStreamRepo struct {
	mu      sync.Mutex
	streams []domain.Stream
	alive   map[string]int64 // streamID → latencia
	dead    map[string]bool
	salud   map[string]ports.StreamHealth // último resultado por stream
}

func newFakeStreamRepo(streams []domain.Stream) *fakeStreamRepo {
	return &fakeStreamRepo{
		streams: streams,
		alive:   make(map[string]int64),
		dead:    make(map[string]bool),
		salud:   make(map[string]ports.StreamHealth),
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
func (f *fakeStreamRepo) FindMirrorsByChannelID(context.Context, domain.ChannelID) ([]ports.MirrorHealth, error) {
	return nil, errors.New("no impl")
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
		f.mu.Lock()
		f.salud[r.StreamID] = r
		f.mu.Unlock()

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

func (f *fakeStreamRepo) saludDe(id string) ports.StreamHealth {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.salud[id]
}

func (f *fakeStreamRepo) MarkDead(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dead[id] = true
	return nil
}

func (f *fakeStreamRepo) DeleteStale(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeStreamRepo) CabecerasPorURL(context.Context, string) (string, string, error) {
	return "", "", nil
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

func streamConCabeceras(id, channel, url, referrer, userAgent string) domain.Stream {
	s := stream(id, channel, url)
	s.Referrer = referrer
	s.UserAgent = userAgent
	return s
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

// Sin las cabeceras del stream, el health-check las manda vacías y este
// origen con protección de hotlink devuelve 403: el stream se marcaría
// muerto por una razón falsa. Es el mismo escenario de TestCheckConCabeceras
// pero verificando el tramo real FindAll→byURL→canal→checker, no solo el
// checker en aislado.
func TestWorker_UsaLasCabecerasDelStreamParaElHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://ref.example/" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n"))
	}))
	defer srv.Close()

	repo := newFakeStreamRepo([]domain.Stream{
		streamConCabeceras("st-hotlink", "ch-1", srv.URL+"/x.m3u8", "https://ref.example/", ""),
	})

	w := testWorker(repo, time.Hour)
	w.checkOnce(context.Background())

	alive, dead := repo.estado()
	if _, ok := alive["st-hotlink"]; !ok {
		t.Errorf("st-hotlink debía marcarse vivo (el Referer del stream debía llegar al checker); dead=%v", dead)
	}
}

// Dos streams comparten URL (mismo mirror físico bajo canales distintos) y
// solo uno declara cabeceras. La deduplicación por URL no puede perder esas
// cabeceras ni dejar que la fila sin ellas las pise: se resuelve
// deterministamente por "primera fila con cabeceras no vacías gana, y una
// fila vacía nunca sobrescribe una que ya las traía".
func TestWorker_DedupPorURLConservaLasCabecerasAunqueUnaFilaVengaVacia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://ref.example/" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	url := srv.URL + "/compartido.m3u8"
	repo := newFakeStreamRepo([]domain.Stream{
		stream("st-sin-cabeceras", "ch-1", url),
		streamConCabeceras("st-con-cabeceras", "ch-2", url, "https://ref.example/", ""),
	})

	w := testWorker(repo, time.Hour)
	w.checkOnce(context.Background())

	alive, dead := repo.estado()
	if _, ok := alive["st-sin-cabeceras"]; !ok {
		t.Error("st-sin-cabeceras debía marcarse vivo: comparte URL con st-con-cabeceras y el check único debe usar el Referer")
	}
	if _, ok := alive["st-con-cabeceras"]; !ok {
		t.Errorf("st-con-cabeceras debía marcarse vivo; dead=%v", dead)
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

// Prefijo TS mínimo con PAT + PMT (MPEG-2 + MP2), para no depender de los
// helpers del paquete externo de tests.
func prefijoMPEG2() []byte {
	paquete := func(pid int, seccion []byte) []byte {
		p := make([]byte, 188)
		for i := range p {
			p[i] = 0xff
		}
		//nolint:gosec // pid es un literal de test (0 o 0x1000): cabe en 13 bits, sin overflow real
		p[0], p[1], p[2], p[3], p[4] = 0x47, 0x40|byte(pid>>8), byte(pid), 0x10, 0
		copy(p[5:], seccion)
		return p
	}
	seccion := func(tableID byte, cuerpo []byte) []byte {
		largo := len(cuerpo) + 4
		//nolint:gosec // largo es el tamaño de una sección de test, muy por debajo de 255
		s := append([]byte{tableID, 0xb0 | byte(largo>>8), byte(largo)}, cuerpo...)
		return append(s, 0, 0, 0, 0)
	}
	pat := seccion(0x00, []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0x00, 0x01, 0xf0, 0x00})
	pmt := seccion(0x02, []byte{0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00,
		0x02, 0xe1, 0x00, 0xf0, 0x00, 0x03, 0xe1, 0x01, 0xf0, 0x00})
	return append(paquete(0, pat), paquete(0x1000, pmt)...)
}

func origenHLS(t *testing.T, hitsSeg *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/media.m3u8":
			_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:4.0,\nseg.ts\n"))
		case "/seg.ts":
			hitsSeg.Add(1)
			w.Header().Set("Content-Type", "video/MP2T")
			_, _ = w.Write(prefijoMPEG2())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWorker_SondeaSoloLosCaducados(t *testing.T) {
	var hitsSeg atomic.Int32
	srv := origenHLS(t, &hitsSeg)
	ahora := time.Now()

	nunca := stream("st-nunca", "ch-1", srv.URL+"/media.m3u8?a")
	reciente := stream("st-reciente", "ch-2", srv.URL+"/media.m3u8?b")
	reciente.IsAlive = true
	reciente.CodecCheckedAt = ahora.Add(-time.Hour)
	viejo := stream("st-viejo", "ch-3", srv.URL+"/media.m3u8?c")
	viejo.IsAlive = true
	viejo.CodecCheckedAt = ahora.Add(-25 * time.Hour)
	resucitado := stream("st-resucitado", "ch-4", srv.URL+"/media.m3u8?d")
	resucitado.IsAlive = false
	resucitado.CodecCheckedAt = ahora.Add(-time.Hour)

	repo := newFakeStreamRepo([]domain.Stream{nunca, reciente, viejo, resucitado})
	cfg := Config{MaxWorkers: 4, Timeout: 2 * time.Second, PermitirDestinosPrivados: true}
	w := NewWorker(repo, cfg, time.Hour, nil)
	w.checkOnce(context.Background())

	if got := hitsSeg.Load(); got != 3 {
		t.Errorf("segmentos pedidos = %d, quiero 3 (nunca, viejo, resucitado)", got)
	}
	for _, id := range []string{"st-nunca", "st-viejo", "st-resucitado"} {
		s := repo.saludDe(id)
		if !s.CodecSondeado || s.Codec != domain.CodecNo || s.Codecs != "mpeg2video,mp2" {
			t.Errorf("%s: %+v", id, s)
		}
	}
	if s := repo.saludDe("st-reciente"); s.CodecSondeado {
		t.Errorf("st-reciente no debía sondearse: %+v", s)
	}
}

// Dos filas con la MISMA URL: si cualquiera está caducada, la URL se sondea
// (una vez) y el veredicto se aplica a las dos.
func TestWorker_DedupPorURLSondeaSiAlgunaFilaEstaCaducada(t *testing.T) {
	var hitsSeg atomic.Int32
	srv := origenHLS(t, &hitsSeg)
	a := stream("st-a", "ch-1", srv.URL+"/media.m3u8")
	a.IsAlive = true
	a.CodecCheckedAt = time.Now()
	b := stream("st-b", "ch-2", srv.URL+"/media.m3u8") // nunca sondeado

	repo := newFakeStreamRepo([]domain.Stream{a, b})
	w := NewWorker(repo, Config{MaxWorkers: 4, Timeout: 2 * time.Second, PermitirDestinosPrivados: true}, time.Hour, nil)
	w.checkOnce(context.Background())

	if hitsSeg.Load() != 1 {
		t.Errorf("segmentos pedidos = %d, quiero 1", hitsSeg.Load())
	}
	for _, id := range []string{"st-a", "st-b"} {
		if s := repo.saludDe(id); !s.CodecSondeado || s.Codec != domain.CodecNo {
			t.Errorf("%s: %+v", id, s)
		}
	}
}

func TestCodecCaducado(t *testing.T) {
	ahora := time.Now()
	casos := []struct {
		nombre string
		s      domain.Stream
		quiero bool
	}{
		{"nunca sondeado", domain.Stream{IsAlive: true}, true},
		{"reciente y vivo", domain.Stream{IsAlive: true, CodecCheckedAt: ahora.Add(-time.Hour)}, false},
		{"más de 24 h", domain.Stream{IsAlive: true, CodecCheckedAt: ahora.Add(-CaducidadCodec - time.Minute)}, true},
		{"venía de muerto", domain.Stream{IsAlive: false, CodecCheckedAt: ahora.Add(-time.Hour)}, true},
	}
	for _, c := range casos {
		if got := codecCaducado(c.s, ahora); got != c.quiero {
			t.Errorf("%s: codecCaducado = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}
