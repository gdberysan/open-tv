package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/ports"
)

func openTestDB(t *testing.T) *db.SQLiteChannelRepository {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return db.NewChannelRepository(sqlDB)
}

func makeChannel(id, name, country, category string) domain.Channel {
	return domain.Channel{
		ID:           domain.ChannelID(id),
		Name:         name,
		LogoURL:      "https://example.com/logo.png",
		CategoryID:   category,
		LanguageCode: "en",
		CountryCode:  country,
		ProviderID:   "opensource",
		ProviderType: domain.ProviderOpenSource,
	}
}

func TestChannelRepository_SaveAndFindByID(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	ch := makeChannel("ch-001", "BBC One", "GB", "news")
	if err := repo.Save(ctx, ch); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, ch.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if got.Name != ch.Name {
		t.Errorf("Name = %q, want %q", got.Name, ch.Name)
	}
	if got.CountryCode != "GB" {
		t.Errorf("CountryCode = %q, want %q", got.CountryCode, "GB")
	}
	if got.ProviderType != domain.ProviderOpenSource {
		t.Errorf("ProviderType = %q, want %q", got.ProviderType, domain.ProviderOpenSource)
	}
}

func TestChannelRepository_SaveIsUpsert(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	ch := makeChannel("ch-002", "France 2", "FR", "general")
	if err := repo.Save(ctx, ch); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	ch.Name = "France 2 HD"
	if err := repo.Save(ctx, ch); err != nil {
		t.Fatalf("upsert Save: %v", err)
	}

	got, err := repo.FindByID(ctx, ch.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "France 2 HD" {
		t.Errorf("Name after upsert = %q, want %q", got.Name, "France 2 HD")
	}
}

func TestChannelRepository_SaveBatch(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	const n = 500
	channels := make([]domain.Channel, n)
	for i := range channels {
		channels[i] = makeChannel(
			string(rune('a'+i%26))+"-batch-"+itoa(i),
			"Canal "+itoa(i),
			"FR", "general",
		)
	}

	start := time.Now()
	if err := repo.SaveBatch(ctx, channels); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}
	t.Logf("SaveBatch(%d canales) en %v", n, time.Since(start))

	results, err := repo.Search(ctx, "Canal", n)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != n {
		t.Errorf("Search devolvió %d resultados, esperaba %d", len(results), n)
	}
}

func TestChannelRepository_Search(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	channels := []domain.Channel{
		makeChannel("s-001", "BBC One", "GB", "general"),
		makeChannel("s-002", "BBC Two", "GB", "general"),
		makeChannel("s-003", "ITV", "GB", "general"),
		makeChannel("s-004", "TF1", "FR", "general"),
	}
	if err := repo.SaveBatch(ctx, channels); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	results, err := repo.Search(ctx, "BBC", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Search('BBC') = %d resultados, esperaba 2", len(results))
	}

	// Búsqueda vacía devuelve todos
	all, err := repo.Search(ctx, "", 10)
	if err != nil {
		t.Fatalf("Search vacío: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("Search('') = %d resultados, esperaba 4", len(all))
	}
}

func TestChannelRepository_FindFiltered_Country(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	channels := []domain.Channel{
		makeChannel("c-001", "BBC One (1080p)", "GB", "general"),
		makeChannel("c-002", "ITV (1080p)", "GB", "general"),
		makeChannel("c-003", "TF1 (1080p)", "FR", "general"),
	}
	if err := repo.SaveBatch(ctx, channels); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	uk, err := repo.FindFiltered(ctx, ports.ChannelFilter{Country: "GB", Limit: 10})
	if err != nil {
		t.Fatalf("FindFiltered(country=GB): %v", err)
	}
	if len(uk) != 2 {
		t.Errorf("FindFiltered(country=GB) = %d, esperaba 2", len(uk))
	}
}

func TestChannelRepository_FindFiltered_Quality(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	channels := []domain.Channel{
		makeChannel("q-001", "Canal HD (1080p)", "FR", "news"),
		makeChannel("q-002", "Canal SD (576p)", "FR", "news"),
		makeChannel("q-003", "Canal 4K (2160p)", "FR", "news"),
	}
	if err := repo.SaveBatch(ctx, channels); err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	fhd, err := repo.FindFiltered(ctx, ports.ChannelFilter{Country: "FR", MinQuality: "fhd", Limit: 10})
	if err != nil {
		t.Fatalf("FindFiltered(quality=fhd): %v", err)
	}
	if len(fhd) != 2 {
		t.Errorf("FindFiltered(quality=fhd) = %d, esperaba 2 (1080p + 2160p)", len(fhd))
	}
}

func TestChannelRepository_Delete(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	ch := makeChannel("d-001", "Arte", "FR", "culture")
	if err := repo.Save(ctx, ch); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := repo.Delete(ctx, ch.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(ctx, ch.ID)
	if err == nil {
		t.Error("FindByID después de Delete debería retornar error, obtuvo nil")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// Un canal sin ninguna fila en streams es injugable: /channels/stream
// devuelve 404 al pulsarlo. No debe listarse cuando AliveOnly está activo.
func TestFindFilteredAliveOnlyOcultaCanalesSinStreams(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	seedChannel(t, chRepo, "con-stream")
	seedChannel(t, chRepo, "sin-stream")

	if err := stRepo.Save(ctx, makeStream("st-1", "con-stream", "http://a/1.m3u8")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{AliveOnly: true, Limit: 100})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}

	for _, ch := range got {
		if ch.ID == "sin-stream" {
			t.Errorf("el canal sin streams no debería listarse con AliveOnly")
		}
	}
	if len(got) != 1 || got[0].ID != "con-stream" {
		t.Errorf("quiero solo [con-stream], tengo %d canales", len(got))
	}
}

// Antes de la primera pasada del health-worker nada está "vivo". Un canal con
// streams sin chequear debe seguir visible para no vaciar la app al arrancar.
func TestFindFilteredAliveOnlyMuestraStreamsSinChequear(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	seedChannel(t, chRepo, "sin-chequear")
	if err := stRepo.Save(ctx, makeStream("st-1", "sin-chequear", "http://a/1.m3u8")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{AliveOnly: true, Limit: 100})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("un canal con streams sin chequear debe seguir visible; tengo %d", len(got))
	}
}

// El ID de canal se deriva del nombre, así que un renombrado aguas arriba deja
// huérfana la fila anterior. DeleteStale la barre tras un sync exitoso.
func TestDeleteStaleBorraCanalesNoVistosEnElUltimoSync(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	seedChannel(t, chRepo, "viejo")
	if err := stRepo.Save(ctx, makeStream("st-viejo", "viejo", "http://a/v.m3u8")); err != nil {
		t.Fatalf("Save stream: %v", err)
	}

	// Frontera del sync: last_seen_at tiene resolución de segundos, así que hay
	// que cruzar un segundo entero para que el corte distinga las dos tandas.
	time.Sleep(1100 * time.Millisecond)
	corte := time.Now()

	seedChannel(t, chRepo, "nuevo")

	n, err := chRepo.DeleteStale(ctx, "opensource", corte)
	if err != nil {
		t.Fatalf("DeleteStale: %v", err)
	}
	if n != 1 {
		t.Errorf("quiero 1 canal borrado, tengo %d", n)
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{Limit: 100})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}
	if len(got) != 1 || got[0].ID != "nuevo" {
		t.Errorf("quiero solo [nuevo], tengo %d canales", len(got))
	}

	// El stream del canal borrado se va por ON DELETE CASCADE.
	streams, err := stRepo.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(streams) != 0 {
		t.Errorf("los streams del canal borrado deben caer por cascada; quedan %d", len(streams))
	}
}
