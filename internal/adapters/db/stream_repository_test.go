package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	t.Cleanup(func() { _ = sqlDB.Close() })
	seedProviderOpensource(t, sqlDB)
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

// repoConCanal abre una DB de test con un canal ("ch-1") y dos streams
// ("s1", "s2") ya guardados, y devuelve además el *sql.DB crudo para que los
// tests puedan leer columnas (como web_ok) que el dominio no expone tal cual.
func repoConCanal(t *testing.T) (*db.SQLiteStreamRepository, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	seedProviderOpensource(t, sqlDB)

	chRepo := db.NewChannelRepository(sqlDB)
	stRepo := db.NewStreamRepository(sqlDB)
	seedChannel(t, chRepo, "ch-1")

	if err := stRepo.SaveBatch(context.Background(), []domain.Stream{
		makeStream("s1", "ch-1", "http://a.example/1.m3u8"),
		makeStream("s2", "ch-1", "http://b.example/1.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	return stRepo, sqlDB
}

// El veredicto web se guarda junto al de salud, en la misma transacción.
func TestMarkBatchPersisteWebOK(t *testing.T) {
	repo, sqlDB := repoConCanal(t)

	if err := repo.MarkBatch(context.Background(), []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, LatencyMs: 120, Web: domain.WebOK},
		{StreamID: "s2", IsAlive: true, LatencyMs: 300, Web: domain.WebNo},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}

	for _, c := range []struct {
		id     string
		quiero sql.NullInt64
	}{
		{"s1", sql.NullInt64{Int64: 1, Valid: true}},
		{"s2", sql.NullInt64{Int64: 0, Valid: true}},
	} {
		var got sql.NullInt64
		if err := sqlDB.QueryRow(`SELECT web_ok FROM streams WHERE id = ?`, c.id).Scan(&got); err != nil {
			t.Fatalf("leyendo web_ok de %s: %v", c.id, err)
		}
		if got != c.quiero {
			t.Errorf("web_ok de %s = %+v, quiero %+v", c.id, got, c.quiero)
		}
	}
}

// Los mirrors salen ORDENADOS: vivos primero, dentro de vivos por latencia
// ascendente, muertos al final. El cliente los recorre en ese orden al hacer
// failover, así que el orden es parte del contrato.
func TestFindMirrorsByChannelIDOrdenaPorSalud(t *testing.T) {
	repo, _ := repoConCanal(t) // ch-1 con streams s1, s2
	ctx := context.Background()

	// s3: tercer mirror, muerto tras agotar la histéresis.
	if err := repo.Save(ctx, makeStream("s3", "ch-1", "http://c.example/1.m3u8")); err != nil {
		t.Fatalf("Save s3: %v", err)
	}
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		if err := repo.MarkDead(ctx, "s3"); err != nil {
			t.Fatalf("MarkDead s3 #%d: %v", i, err)
		}
	}

	// s1: vivo 300ms webNo · s2: vivo 100ms webOK.
	if err := repo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, LatencyMs: 300, Web: domain.WebNo},
		{StreamID: "s2", IsAlive: true, LatencyMs: 100, Web: domain.WebOK},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}

	mirrors, err := repo.FindMirrorsByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindMirrorsByChannelID: %v", err)
	}
	if len(mirrors) != 3 {
		t.Fatalf("quiero 3 mirrors, tengo %d", len(mirrors))
	}
	// El de menor latencia va primero.
	if mirrors[0].LatencyMs != 100 || mirrors[1].LatencyMs != 300 {
		t.Errorf("orden por latencia mal: %d antes que %d", mirrors[0].LatencyMs, mirrors[1].LatencyMs)
	}
	if mirrors[0].WebOK != domain.WebOK {
		t.Errorf("web_ok del primer mirror = %v, quiero WebOK", mirrors[0].WebOK)
	}
	// El muerto va al final.
	if mirrors[2].IsAlive {
		t.Errorf("el mirror muerto debe ir último; IsAlive = %v", mirrors[2].IsAlive)
	}
}

// Un veredicto desconocido NO puede pisar uno bueno: si el origen no contestó
// esta pasada, lo que sabíamos de la anterior sigue siendo lo mejor que hay.
func TestMarkBatchUnknownNoPisaElVeredictoAnterior(t *testing.T) {
	repo, sqlDB := repoConCanal(t)
	ctx := context.Background()

	if err := repo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, LatencyMs: 100, Web: domain.WebOK},
	}); err != nil {
		t.Fatalf("MarkBatch (1): %v", err)
	}
	if err := repo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: false, Web: domain.WebUnknown},
	}); err != nil {
		t.Fatalf("MarkBatch (2): %v", err)
	}

	var got sql.NullInt64
	if err := sqlDB.QueryRow(`SELECT web_ok FROM streams WHERE id = 's1'`).Scan(&got); err != nil {
		t.Fatalf("leyendo web_ok: %v", err)
	}
	if !got.Valid || got.Int64 != 1 {
		t.Errorf("web_ok = %+v, quiero que conserve 1", got)
	}
}

// nuevaDBDePrueba abre una DB de test con los providers "p1" y "p2" ya
// sembrados (channels.provider_id tiene FK contra providers(id)), y devuelve
// el *sql.DB crudo para que el test construya los repos que necesite.
func nuevaDBDePrueba(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	seedProvider(t, sqlDB, "p1")
	seedProvider(t, sqlDB, "p2")
	return sqlDB
}

// La poda de streams no existía: DeleteStale solo estaba en canales, así que
// una URL sustituida upstream se quedaba para siempre y el failover acababa
// gastando un intento entero en historia muerta.
func TestStreamDeleteStalePodaSoloLoViejoYNoCruzaFuentes(t *testing.T) {
	ctx := context.Background()
	base := nuevaDBDePrueba(t) // helper ya existente en este fichero
	canales := db.NewChannelRepository(base)
	streams := db.NewStreamRepository(base)

	// Dos canales de DOS proveedores distintos.
	if err := canales.SaveBatch(ctx, []domain.Channel{
		{ID: "p1-c1", Name: "C1", ProviderID: "p1", ProviderType: domain.ProviderOpenSource},
		{ID: "p2-c1", Name: "C1", ProviderID: "p2", ProviderType: domain.ProviderOpenSource},
	}); err != nil {
		t.Fatalf("SaveBatch canales: %v", err)
	}

	viejo := domain.Stream{ID: "st-viejo", ChannelID: "p1-c1", URL: "https://viejo/x.m3u8", Protocol: domain.ProtocolHLS}
	ajeno := domain.Stream{ID: "st-ajeno", ChannelID: "p2-c1", URL: "https://ajeno/x.m3u8", Protocol: domain.ProtocolHLS}
	if err := streams.SaveBatch(ctx, []domain.Stream{viejo, ajeno}); err != nil {
		t.Fatalf("SaveBatch streams viejos: %v", err)
	}

	time.Sleep(1100 * time.Millisecond) // last_seen_at tiene resolución de segundos
	frontera := time.Now()

	// Re-sync de p1: solo aparece un stream NUEVO.
	nuevo := domain.Stream{ID: "st-nuevo", ChannelID: "p1-c1", URL: "https://nuevo/x.m3u8", Protocol: domain.ProtocolHLS}
	if err := streams.SaveBatch(ctx, []domain.Stream{nuevo}); err != nil {
		t.Fatalf("SaveBatch stream nuevo: %v", err)
	}

	n, err := streams.DeleteStale(ctx, "p1", frontera)
	if err != nil {
		t.Fatalf("DeleteStale: %v", err)
	}
	if n != 1 {
		t.Errorf("podados %d, quiero 1 (solo el viejo de p1)", n)
	}

	quedan, err := streams.FindByChannelID(ctx, "p1-c1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	if len(quedan) != 1 || quedan[0].ID != "st-nuevo" {
		t.Errorf("en p1 quedan %+v, quiero solo st-nuevo", quedan)
	}

	// La poda NUNCA cruza fuentes.
	ajenos, err := streams.FindByChannelID(ctx, "p2-c1")
	if err != nil {
		t.Fatalf("FindByChannelID p2: %v", err)
	}
	if len(ajenos) != 1 {
		t.Errorf("la poda de p1 se llevó streams de p2: quedan %d, quiero 1", len(ajenos))
	}
}

func TestStreamCabecerasPorURL(t *testing.T) {
	ctx := context.Background()
	base := nuevaDBDePrueba(t)
	canales := db.NewChannelRepository(base)
	streams := db.NewStreamRepository(base)

	if err := canales.SaveBatch(ctx, []domain.Channel{
		{ID: "p1-c1", Name: "C1", ProviderID: "p1", ProviderType: domain.ProviderOpenSource},
	}); err != nil {
		t.Fatalf("SaveBatch canales: %v", err)
	}
	if err := streams.SaveBatch(ctx, []domain.Stream{
		{ID: "st-1", ChannelID: "p1-c1", URL: "https://con/x.m3u8", Protocol: domain.ProtocolHLS,
			Referrer: "https://ref/", UserAgent: "UA/1"},
		{ID: "st-2", ChannelID: "p1-c1", URL: "https://sin/x.m3u8", Protocol: domain.ProtocolHLS},
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	ref, ua, err := streams.CabecerasPorURL(ctx, "https://con/x.m3u8")
	if err != nil {
		t.Fatalf("CabecerasPorURL: %v", err)
	}
	if ref != "https://ref/" || ua != "UA/1" {
		t.Errorf("(%q,%q), quiero (https://ref/, UA/1)", ref, ua)
	}

	ref, ua, err = streams.CabecerasPorURL(ctx, "https://sin/x.m3u8")
	if err != nil {
		t.Fatalf("CabecerasPorURL sin cabeceras: %v", err)
	}
	if ref != "" || ua != "" {
		t.Errorf("(%q,%q), quiero vacías", ref, ua)
	}

	// Una URL desconocida no hereda cabeceras de otra ni es un error.
	ref, ua, err = streams.CabecerasPorURL(ctx, "https://desconocida/x.m3u8")
	if err != nil {
		t.Fatalf("CabecerasPorURL desconocida: %v", err)
	}
	if ref != "" || ua != "" {
		t.Errorf("(%q,%q), quiero vacías para una URL que no está", ref, ua)
	}
}

// Los segmentos y claves (EXT-X-KEY/EXT-X-MAP) que ReescribirManifiesto
// reescribe NUNCA son una fila propia de streams: solo el manifiesto de
// nivel superior lo es. Sin esta caída al origen, el proxy pediría las
// cabeceras de cada segmento y siempre fallaría — el hotlink/agent-filter
// del origen sigue aplicando a esas URLs hijas igual que al manifiesto.
func TestStreamCabecerasPorURLCaeAlOrigenParaHijosSinFilaPropia(t *testing.T) {
	ctx := context.Background()
	base := nuevaDBDePrueba(t)
	canales := db.NewChannelRepository(base)
	streams := db.NewStreamRepository(base)

	if err := canales.SaveBatch(ctx, []domain.Channel{
		{ID: "p1-c1", Name: "C1", ProviderID: "p1", ProviderType: domain.ProviderOpenSource},
	}); err != nil {
		t.Fatalf("SaveBatch canales: %v", err)
	}
	if err := streams.SaveBatch(ctx, []domain.Stream{
		// El único registro real: el manifiesto de nivel superior.
		{ID: "st-1", ChannelID: "p1-c1", URL: "https://con.example/live/master.m3u8",
			Protocol: domain.ProtocolHLS, Referrer: "https://ref.example/", UserAgent: "UA/1"},
		// Un stream de otro origen, sin cabeceras: no debe poder "prestarlas"
		// a nadie (ni siquiera lo intenta, porque no comparte origen).
		{ID: "st-2", ChannelID: "p1-c1", URL: "https://otro.example/x.m3u8", Protocol: domain.ProtocolHLS},
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	// Caso 1: exacto GANA sobre el origen. seg1.ts SÍ tiene fila propia (sin
	// cabeceras): si el fallback por origen "ganara", vendría contaminado con
	// las cabeceras de master.m3u8; el contrato exige lo contrario.
	if err := streams.SaveBatch(ctx, []domain.Stream{
		{ID: "st-3", ChannelID: "p1-c1", URL: "https://con.example/live/seg1.ts", Protocol: domain.ProtocolHLS},
	}); err != nil {
		t.Fatalf("SaveBatch seg1: %v", err)
	}
	ref, ua, err := streams.CabecerasPorURL(ctx, "https://con.example/live/seg1.ts")
	if err != nil {
		t.Fatalf("CabecerasPorURL seg1 (fila propia): %v", err)
	}
	if ref != "" || ua != "" {
		t.Errorf("seg1.ts con fila propia = (%q,%q), quiero vacías (el exacto gana, no hereda del origen)", ref, ua)
	}

	// Caso 2: un segmento SIN fila propia, mismo origen que master.m3u8,
	// hereda sus cabeceras.
	ref, ua, err = streams.CabecerasPorURL(ctx, "https://con.example/live/seg2.ts")
	if err != nil {
		t.Fatalf("CabecerasPorURL seg2 (hereda del origen): %v", err)
	}
	if ref != "https://ref.example/" || ua != "UA/1" {
		t.Errorf("seg2.ts = (%q,%q), quiero heredar (https://ref.example/, UA/1) del origen", ref, ua)
	}

	// Caso 3: mismo esquema y path, pero OTRO host — nada que heredar.
	ref, ua, err = streams.CabecerasPorURL(ctx, "https://con.example.evil.com/live/seg3.ts")
	if err != nil {
		t.Fatalf("CabecerasPorURL host distinto: %v", err)
	}
	if ref != "" || ua != "" {
		t.Errorf("host distinto (con.example.evil.com) = (%q,%q), quiero vacías", ref, ua)
	}

	// Caso 4: un host que ni siquiera comparte origen con nada en la tabla.
	ref, ua, err = streams.CabecerasPorURL(ctx, "https://nada-que-ver.example/seg.ts")
	if err != nil {
		t.Fatalf("CabecerasPorURL host ajeno: %v", err)
	}
	if ref != "" || ua != "" {
		t.Errorf("host ajeno = (%q,%q), quiero vacías", ref, ua)
	}
}

// EXPLAIN QUERY PLAN del camino de fallback: tiene que ser un range scan por
// idx_streams_url, nunca un escaneo completo — a ~17k streams la diferencia
// es justo lo que este fallback no puede permitirse pagar por cada segmento.
func TestStreamCabecerasPorURLFallbackUsaElIndice(t *testing.T) {
	ctx := context.Background()
	base := nuevaDBDePrueba(t)

	filas, err := base.QueryContext(ctx, `
		EXPLAIN QUERY PLAN
		SELECT referrer, user_agent FROM streams
		WHERE url >= ? AND url < ?
		  AND (referrer <> '' OR user_agent <> '')
		LIMIT 1`, "https://con.example/", "https://con.example0")
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer func() { _ = filas.Close() }()

	var plan []string
	for filas.Next() {
		var id, parent, notUsed int
		var detail string
		if err := filas.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan del plan: %v", err)
		}
		plan = append(plan, detail)
		t.Logf("plan: %s", detail)
	}
	if err := filas.Err(); err != nil {
		t.Fatalf("iterando el plan: %v", err)
	}

	huboIndice := false
	for _, l := range plan {
		if strings.Contains(l, "USING INDEX idx_streams_url") {
			huboIndice = true
		}
		if strings.Contains(strings.ToUpper(l), "SCAN TABLE STREAMS") &&
			!strings.Contains(l, "USING INDEX") {
			t.Errorf("el plan hace un scan completo de streams: %q", l)
		}
	}
	if !huboIndice {
		t.Errorf("el plan no usa idx_streams_url: %v", plan)
	}
}

// Las cabeceras tienen que sobrevivir el ROUNDTRIP por los finders, no solo
// por CabecerasPorURL: el health-check lee el catálogo con FindAll y, si el
// SELECT no trae referrer/user_agent, chequea pelados justo los streams cuyo
// origen los exige — 403, tres pasadas, y muertos. Este es el punto de unión
// del que depende toda la cadena de cabeceras y no tenía cobertura.
func TestStreamCabecerasSobrevivenALosFinders(t *testing.T) {
	ctx := context.Background()
	base := nuevaDBDePrueba(t)
	canales := db.NewChannelRepository(base)
	streams := db.NewStreamRepository(base)

	if err := canales.SaveBatch(ctx, []domain.Channel{
		{ID: "p1-c1", Name: "C1", ProviderID: "p1", ProviderType: domain.ProviderOpenSource},
	}); err != nil {
		t.Fatalf("SaveBatch canales: %v", err)
	}
	if err := streams.SaveBatch(ctx, []domain.Stream{
		{ID: "st-1", ChannelID: "p1-c1", URL: "https://con/x.m3u8", Protocol: domain.ProtocolHLS,
			Referrer: "https://ref/", UserAgent: "UA/1"},
	}); err != nil {
		t.Fatalf("SaveBatch streams: %v", err)
	}

	todos, err := streams.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("FindAll devolvió %d streams, quiero 1", len(todos))
	}
	if todos[0].Referrer != "https://ref/" || todos[0].UserAgent != "UA/1" {
		t.Errorf("FindAll: (%q,%q), quiero (https://ref/, UA/1)", todos[0].Referrer, todos[0].UserAgent)
	}

	delCanal, err := streams.FindByChannelID(ctx, "p1-c1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	if len(delCanal) != 1 || delCanal[0].Referrer != "https://ref/" || delCanal[0].UserAgent != "UA/1" {
		t.Errorf("FindByChannelID: %+v, quiero las cabeceras intactas", delCanal)
	}

	// FindBestByChannelID solo mira los vivos: hay que chequearlo antes.
	if err := streams.MarkAlive(ctx, "st-1", 42); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}
	mejor, err := streams.FindBestByChannelID(ctx, "p1-c1")
	if err != nil {
		t.Fatalf("FindBestByChannelID: %v", err)
	}
	if mejor.Referrer != "https://ref/" || mejor.UserAgent != "UA/1" {
		t.Errorf("FindBestByChannelID: (%q,%q), quiero las cabeceras intactas", mejor.Referrer, mejor.UserAgent)
	}
}

// Un mirror recién sincronizado entra con fail_count 0 y last_checked NULL.
// Con la persistencia de todos los mirrors, cada sync mete filas nuevas a
// puñados: si contaran como disponibles, 631 canales sin un solo stream que
// funcione volverían a la lista durante las ~3 h que tardan tres pasadas del
// health-check en agotar el umbral, y la app promete «Comprobado en vivo».
// Un mirror sin verificar NO cuenta; en cuanto se verifica vivo, cuenta.
func TestFindFilteredAliveOnlyIgnoraMirrorsSinVerificar(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	seedChannel(t, chRepo, "ch-muerto")  // uno probado muerto + un mirror nuevo
	seedChannel(t, chRepo, "ch-en-frio") // nada chequeado todavía

	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("st-muerto", "ch-muerto", "http://a/viejo.m3u8"),
		makeStream("st-nuevo", "ch-muerto", "http://a/mirror-nuevo.m3u8"),
		makeStream("st-frio", "ch-en-frio", "http://b/1.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	// El viejo agota la histéresis: probado muerto.
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-muerto"); err != nil {
			t.Fatalf("MarkDead #%d: %v", i, err)
		}
	}

	visibles := func() map[string]bool {
		t.Helper()
		got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{AliveOnly: true, Limit: 100})
		if err != nil {
			t.Fatalf("FindFiltered: %v", err)
		}
		ids := map[string]bool{}
		for _, ch := range got {
			ids[string(ch.ID)] = true
		}
		return ids
	}

	ids := visibles()
	if ids["ch-muerto"] {
		t.Error("un mirror nuevo SIN VERIFICAR no puede hacer disponible un canal probado muerto")
	}
	// Arranque en frío: si NINGÚN stream del canal se ha chequeado aún no hay
	// evidencia en contra, así que sigue visible. Sin esto la app aparecería
	// vacía entre el primer sync y la primera pasada del health-worker.
	if !ids["ch-en-frio"] {
		t.Error("un canal cuyos streams no se han chequeado todavía debe seguir visible")
	}

	// Y en cuanto el mirror nuevo se verifica vivo, el canal vuelve.
	if err := stRepo.MarkAlive(ctx, "st-nuevo", 80); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}
	if !visibles()["ch-muerto"] {
		t.Error("un mirror verificado vivo debe hacer visible el canal con normalidad")
	}
}

// El contador tiene que contar lo mismo que la lista enseña: si CountFiltered
// y FindFiltered discreparan, la app pintaría un número que no corresponde.
func TestCountFilteredAliveOnlyCuentaLoMismoQueLaLista(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	seedChannel(t, chRepo, "ch-muerto")
	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("st-muerto", "ch-muerto", "http://a/viejo.m3u8"),
		makeStream("st-nuevo", "ch-muerto", "http://a/mirror-nuevo.m3u8"),
	}); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-muerto"); err != nil {
			t.Fatalf("MarkDead #%d: %v", i, err)
		}
	}

	n, err := chRepo.CountFiltered(ctx, ports.ChannelFilter{AliveOnly: true})
	if err != nil {
		t.Fatalf("CountFiltered: %v", err)
	}
	if n != 0 {
		t.Errorf("CountFiltered = %d, quiero 0: el mirror sin verificar no cuenta", n)
	}
}
