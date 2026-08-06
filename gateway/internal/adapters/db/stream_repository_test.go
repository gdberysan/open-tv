package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
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
