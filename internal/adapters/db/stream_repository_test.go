package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
)

// openStreamTestRepos abre una DB de test y devuelve ambos repos: los streams
// tienen FK hacia channels, así que los tests necesitan crear canales primero.
func openStreamTestRepos(t *testing.T) (*db.SQLiteChannelRepository, *db.SQLiteStreamRepository) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return db.NewChannelRepository(sqlDB), db.NewStreamRepository(sqlDB)
}

func seedChannel(t *testing.T, repo *db.SQLiteChannelRepository, id string) {
	t.Helper()
	ch := makeChannel(id, "Canal "+id, "ES", "news")
	if err := repo.Save(context.Background(), ch); err != nil {
		t.Fatalf("seedChannel(%s): %v", id, err)
	}
}

func makeStream(id, channelID, url string) domain.Stream {
	return domain.Stream{
		ID:        id,
		ChannelID: domain.ChannelID(channelID),
		URL:       url,
		Protocol:  domain.ProtocolHLS,
	}
}

// De tvg_id se deriva el país del canal, así que debe sobrevivir el roundtrip.
func TestChannelRepository_PersisteTvgID(t *testing.T) {
	chRepo, _ := openStreamTestRepos(t)
	ctx := context.Background()

	ch := makeChannel("ch-tvg", "BBC News", "GB", "news")
	ch.TvgID = "BBCNews.uk"
	if err := chRepo.Save(ctx, ch); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := chRepo.FindByID(ctx, "ch-tvg")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.TvgID != "BBCNews.uk" {
		t.Errorf("TvgID = %q, want BBCNews.uk", got.TvgID)
	}
}

func TestStreamRepository_SaveBatchAndFindByChannelID(t *testing.T) {
	chRepo, stRepo := openStreamTestRepos(t)
	ctx := context.Background()
	seedChannel(t, chRepo, "ch-1")

	streams := []domain.Stream{
		makeStream("st-1", "ch-1", "http://a.example/1.m3u8"),
		makeStream("st-2", "ch-1", "http://b.example/1.m3u8"),
	}
	if err := stRepo.SaveBatch(ctx, streams); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	got, err := stRepo.FindByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].URL == "" || got[0].Protocol != domain.ProtocolHLS {
		t.Errorf("stream mal persistido: %+v", got[0])
	}
}

func TestStreamRepository_SaveBatchIsUpsert(t *testing.T) {
	chRepo, stRepo := openStreamTestRepos(t)
	ctx := context.Background()
	seedChannel(t, chRepo, "ch-1")

	s := makeStream("st-1", "ch-1", "http://a.example/1.m3u8")
	if err := stRepo.SaveBatch(ctx, []domain.Stream{s}); err != nil {
		t.Fatalf("SaveBatch 1: %v", err)
	}

	// Re-sync con URL nueva para el mismo stream ID: debe actualizar, no duplicar
	s.URL = "http://a.example/nueva.m3u8"
	if err := stRepo.SaveBatch(ctx, []domain.Stream{s}); err != nil {
		t.Fatalf("SaveBatch 2: %v", err)
	}

	got, err := stRepo.FindByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (upsert no debe duplicar)", len(got))
	}
	if got[0].URL != "http://a.example/nueva.m3u8" {
		t.Errorf("URL = %q, quiere la actualizada", got[0].URL)
	}
}

// El health-worker necesita el listado completo para chequear cada stream.
func TestStreamRepository_FindAll(t *testing.T) {
	chRepo, stRepo := openStreamTestRepos(t)
	ctx := context.Background()
	seedChannel(t, chRepo, "ch-1")
	seedChannel(t, chRepo, "ch-2")

	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("st-1", "ch-1", "http://a.example/1.m3u8"),
		makeStream("st-2", "ch-2", "http://b.example/2.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	got, err := stRepo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].URL == "" || got[0].ID == "" {
		t.Errorf("stream incompleto: %+v", got[0])
	}
}

// AliveOnly oculta los canales con TODOS sus streams chequeados y muertos, y
// también los que no tienen ningún stream (injugables: /channels/stream da
// 404). Los que tienen streams sin chequear (last_checked NULL) siguen
// visibles, para no vaciar la app antes de la primera pasada del health-worker.
func TestChannelRepository_FindFiltered_AliveOnly(t *testing.T) {
	chRepo, stRepo := openStreamTestRepos(t)
	ctx := context.Background()

	seedChannel(t, chRepo, "ch-vivo")      // stream chequeado y vivo
	seedChannel(t, chRepo, "ch-muerto")    // stream chequeado y muerto
	seedChannel(t, chRepo, "ch-pendiente") // stream sin chequear
	seedChannel(t, chRepo, "ch-sin-streams")

	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("st-v", "ch-vivo", "http://v.example/1.m3u8"),
		makeStream("st-m", "ch-muerto", "http://m.example/1.m3u8"),
		makeStream("st-p", "ch-pendiente", "http://p.example/1.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	if err := stRepo.MarkAlive(ctx, "st-v", 100); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}
	// Hasta agotar la histéresis: un solo fallo ya no basta para darlo por muerto.
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-m"); err != nil {
			t.Fatalf("MarkDead #%d: %v", i, err)
		}
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{AliveOnly: true, MinQuality: "none"})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}
	ids := make(map[string]bool)
	for _, ch := range got {
		ids[string(ch.ID)] = true
	}
	if !ids["ch-vivo"] || !ids["ch-pendiente"] {
		t.Errorf("visibles = %v; vivo y pendiente deben verse", ids)
	}
	if ids["ch-muerto"] {
		t.Error("ch-muerto (todos sus streams chequeados y muertos) debe ocultarse")
	}
	if ids["ch-sin-streams"] {
		t.Error("ch-sin-streams no tiene URL que reproducir; debe ocultarse")
	}

	// Sin AliveOnly todos son visibles
	all, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{MinQuality: "none"})
	if err != nil {
		t.Fatalf("FindFiltered sin AliveOnly: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("sin AliveOnly len = %d, want 4", len(all))
	}
}

// FindFiltered incrusta la salud agregada para que la lista pinte el
// indicador de señal sin hacer N+1 requests a /channels/{id}/health.
func TestChannelRepository_FindFiltered_IncluyeSalud(t *testing.T) {
	chRepo, stRepo := openStreamTestRepos(t)
	ctx := context.Background()

	seedChannel(t, chRepo, "ch-vivo")
	seedChannel(t, chRepo, "ch-muerto")
	seedChannel(t, chRepo, "ch-pendiente")

	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("st-v1", "ch-vivo", "http://v.example/1.m3u8"),
		makeStream("st-v2", "ch-vivo", "http://v.example/2.m3u8"),
		makeStream("st-m", "ch-muerto", "http://m.example/1.m3u8"),
		makeStream("st-p", "ch-pendiente", "http://p.example/1.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	if err := stRepo.MarkAlive(ctx, "st-v1", 500); err != nil {
		t.Fatalf("MarkAlive v1: %v", err)
	}
	if err := stRepo.MarkAlive(ctx, "st-v2", 90); err != nil {
		t.Fatalf("MarkAlive v2: %v", err)
	}
	if err := stRepo.MarkDead(ctx, "st-m"); err != nil {
		t.Fatalf("MarkDead: %v", err)
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}
	byID := make(map[string]domain.Channel)
	for _, ch := range got {
		byID[string(ch.ID)] = ch
	}

	vivo := byID["ch-vivo"]
	if vivo.Alive == nil || !*vivo.Alive {
		t.Errorf("ch-vivo: Alive = %v, want true", vivo.Alive)
	}
	if vivo.LatencyMs != 90 {
		t.Errorf("ch-vivo: LatencyMs = %d, want 90 (la mejor entre vivos)", vivo.LatencyMs)
	}

	muerto := byID["ch-muerto"]
	if muerto.Alive == nil || *muerto.Alive {
		t.Errorf("ch-muerto: Alive = %v, want false", muerto.Alive)
	}

	// Sin chequear: Alive nil, para que el cliente distinga "muerto" de
	// "aún sin datos" (barras grises vs apagadas)
	pendiente := byID["ch-pendiente"]
	if pendiente.Alive != nil {
		t.Errorf("ch-pendiente: Alive = %v, want nil (sin chequear)", pendiente.Alive)
	}
}

func TestStreamRepository_FindBestByChannelID(t *testing.T) {
	chRepo, stRepo := openStreamTestRepos(t)
	ctx := context.Background()
	seedChannel(t, chRepo, "ch-1")

	streams := []domain.Stream{
		makeStream("st-lento", "ch-1", "http://lento.example/1.m3u8"),
		makeStream("st-rapido", "ch-1", "http://rapido.example/1.m3u8"),
		makeStream("st-muerto", "ch-1", "http://muerto.example/1.m3u8"),
	}
	if err := stRepo.SaveBatch(ctx, streams); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	if err := stRepo.MarkAlive(ctx, "st-lento", 900); err != nil {
		t.Fatalf("MarkAlive lento: %v", err)
	}
	if err := stRepo.MarkAlive(ctx, "st-rapido", 50); err != nil {
		t.Fatalf("MarkAlive rapido: %v", err)
	}
	if err := stRepo.MarkDead(ctx, "st-muerto"); err != nil {
		t.Fatalf("MarkDead: %v", err)
	}

	best, err := stRepo.FindBestByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindBestByChannelID: %v", err)
	}
	if best.ID != "st-rapido" {
		t.Errorf("best = %q, want st-rapido (menor latencia entre vivos)", best.ID)
	}
	if !best.IsAlive || best.LatencyMs != 50 {
		t.Errorf("best mal escaneado: %+v", best)
	}
}

func TestStreamRepository_FindBestByChannelID_SinVivos(t *testing.T) {
	chRepo, stRepo := openStreamTestRepos(t)
	ctx := context.Background()
	seedChannel(t, chRepo, "ch-1")

	// Streams recién sincronizados, aún sin health-check (is_alive=0)
	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("st-1", "ch-1", "http://a.example/1.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	if _, err := stRepo.FindBestByChannelID(ctx, "ch-1"); err == nil {
		t.Error("se esperaba error cuando no hay streams vivos")
	}
}

// Un fallo aislado (blip de red, 503 momentáneo) no debe ocultar un canal
// durante una hora entera. Solo N fallos consecutivos lo dan por muerto.
func TestMarkDeadRequiereFallosConsecutivos(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")

	if err := stRepo.Save(ctx, makeStream("st-1", "ch-1", "http://a/1.m3u8")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := stRepo.MarkAlive(ctx, "st-1", 120); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}

	isAlive := func() bool {
		t.Helper()
		streams, err := stRepo.FindByChannelID(ctx, "ch-1")
		if err != nil {
			t.Fatalf("FindByChannelID: %v", err)
		}
		if len(streams) != 1 {
			t.Fatalf("quiero 1 stream, tengo %d", len(streams))
		}
		return streams[0].IsAlive
	}

	for i := int64(1); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
			t.Fatalf("MarkDead #%d: %v", i, err)
		}
		if !isAlive() {
			t.Fatalf("tras %d fallo(s) el stream ya está muerto; el umbral es %d", i, db.DeadFailThreshold)
		}
	}

	if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
		t.Fatalf("MarkDead final: %v", err)
	}
	if isAlive() {
		t.Errorf("tras %d fallos consecutivos el stream debería estar muerto", db.DeadFailThreshold)
	}
}

// Un chequeo exitoso borra el historial de fallos: dos fallos hoy y uno
// mañana no deben sumar tres.
func TestMarkAliveReseteaElContadorDeFallos(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")

	if err := stRepo.Save(ctx, makeStream("st-1", "ch-1", "http://a/1.m3u8")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	for i := int64(1); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
			t.Fatalf("MarkDead #%d: %v", i, err)
		}
	}
	if err := stRepo.MarkAlive(ctx, "st-1", 90); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}
	if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
		t.Fatalf("MarkDead tras reset: %v", err)
	}

	streams, err := stRepo.FindByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	if !streams[0].IsAlive {
		t.Errorf("MarkAlive debe resetear el contador; un solo fallo posterior no puede matarlo")
	}
}

// La histéresis no sirve de nada si el filtro ignora fail_count: un stream que
// nunca llegó a estar vivo arranca con is_alive=0, así que su PRIMER fallo ya
// lo ocultaba pese a que fail_count fuese 1. Un canal solo desaparece cuando
// está probado muerto, es decir al alcanzar el umbral.
func TestFindFilteredAliveOnlyRespetaLaHisteresis(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	seedChannel(t, chRepo, "ch-un-fallo")
	seedChannel(t, chRepo, "ch-agotado")

	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("st-1", "ch-un-fallo", "http://a/1.m3u8"),
		makeStream("st-2", "ch-agotado", "http://b/1.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	// Un solo fallo: aún no está probado muerto.
	if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
		t.Fatalf("MarkDead: %v", err)
	}
	// Fallos hasta agotar el umbral.
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-2"); err != nil {
			t.Fatalf("MarkDead st-2 #%d: %v", i, err)
		}
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{AliveOnly: true, Limit: 100})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}
	ids := map[string]bool{}
	for _, ch := range got {
		ids[string(ch.ID)] = true
	}
	if !ids["ch-un-fallo"] {
		t.Errorf("un solo fallo no prueba que el canal esté muerto; debe seguir visible")
	}
	if ids["ch-agotado"] {
		t.Errorf("tras %d fallos consecutivos el canal debe ocultarse", db.DeadFailThreshold)
	}
}

// MarkBatch debe aplicar todos los resultados en una transacción y respetar la
// misma histéresis que MarkDead.
func TestMarkBatchAplicaVivosYMuertosConHisteresis(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")

	for _, id := range []string{"st-vivo", "st-muerto"} {
		if err := stRepo.Save(ctx, makeStream(id, "ch-1", "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}

	for i := int64(0); i < db.DeadFailThreshold; i++ {
		err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
			{StreamID: "st-vivo", IsAlive: true, LatencyMs: 42},
			{StreamID: "st-muerto", IsAlive: false},
		})
		if err != nil {
			t.Fatalf("MarkBatch #%d: %v", i, err)
		}
	}

	streams, err := stRepo.FindByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	porID := map[string]domain.Stream{}
	for _, s := range streams {
		porID[s.ID] = s
	}

	if !porID["st-vivo"].IsAlive {
		t.Error("st-vivo debe seguir vivo")
	}
	if porID["st-vivo"].LatencyMs != 42 {
		t.Errorf("latencia = %d, quiero 42", porID["st-vivo"].LatencyMs)
	}
	if porID["st-muerto"].IsAlive {
		t.Errorf("st-muerto debe estar muerto tras %d fallos", db.DeadFailThreshold)
	}
}

func TestMarkBatchVacioNoFalla(t *testing.T) {
	_, stRepo := openStreamTestRepos(t)
	if err := stRepo.MarkBatch(context.Background(), nil); err != nil {
		t.Errorf("MarkBatch(nil) = %v, quiero nil", err)
	}
}
