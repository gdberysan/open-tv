package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

func openEPGTestRepos(t *testing.T) (*db.SQLiteChannelRepository, *db.SQLiteEPGRepository) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return db.NewChannelRepository(sqlDB), db.NewEPGRepository(sqlDB)
}

// seedChannelTvg crea un canal con tvg_id, requisito para que el EPG matchee.
func seedChannelTvg(t *testing.T, repo *db.SQLiteChannelRepository, id, tvgID string) {
	t.Helper()
	ch := makeChannel(id, "Canal "+id, "GB", "news")
	ch.TvgID = tvgID
	if err := repo.Save(context.Background(), ch); err != nil {
		t.Fatalf("seedChannelTvg(%s): %v", id, err)
	}
}

func epgEntry(tvgID string, start, end time.Time, title string) domain.EPGEntry {
	return domain.EPGEntry{
		ChannelID: domain.ChannelID(tvgID), // el parser XMLTV emite el tvg-id aquí
		Title:     title,
		StartAt:   start,
		EndAt:     end,
	}
}

func TestEPGRepository_SaveBatch_ExpandePorTvgID(t *testing.T) {
	chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)

	// Dos canales comparten tvg-id (variante SD/HD); un tercero es distinto
	seedChannelTvg(t, chRepo, "ch-sd", "BBCNews.uk")
	seedChannelTvg(t, chRepo, "ch-hd", "BBCNews.uk")
	seedChannelTvg(t, chRepo, "ch-otro", "France2.fr")

	err := epgRepo.SaveBatch(ctx, []domain.EPGEntry{
		epgEntry("BBCNews.uk", base, base.Add(time.Hour), "Noticias"),
	})
	if err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	for _, chID := range []string{"ch-sd", "ch-hd"} {
		got, err := epgRepo.FindByChannelAndWindow(ctx, domain.ChannelID(chID), base, base.Add(time.Hour))
		if err != nil {
			t.Fatalf("FindByChannelAndWindow(%s): %v", chID, err)
		}
		if len(got) != 1 || got[0].Title != "Noticias" {
			t.Errorf("%s: got %+v, quiere 1 entrada 'Noticias'", chID, got)
		}
		if got[0].ChannelID != domain.ChannelID(chID) {
			t.Errorf("%s: ChannelID = %q, debe ser el ID propio del canal", chID, got[0].ChannelID)
		}
	}

	otro, err := epgRepo.FindByChannelAndWindow(ctx, "ch-otro", base, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("FindByChannelAndWindow(ch-otro): %v", err)
	}
	if len(otro) != 0 {
		t.Errorf("ch-otro no debe tener EPG, got %+v", otro)
	}
}

func TestEPGRepository_SaveBatch_IgnoraTvgIDDesconocido(t *testing.T) {
	chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	seedChannelTvg(t, chRepo, "ch-1", "Conocido.uk")

	err := epgRepo.SaveBatch(ctx, []domain.EPGEntry{
		epgEntry("NoExiste.xx", base, base.Add(time.Hour), "Fantasma"),
		epgEntry("Conocido.uk", base, base.Add(time.Hour), "Real"),
	})
	if err != nil {
		t.Fatalf("SaveBatch no debe fallar por tvg-ids sin canal: %v", err)
	}

	got, _ := epgRepo.FindByChannelAndWindow(ctx, "ch-1", base, base.Add(time.Hour))
	if len(got) != 1 || got[0].Title != "Real" {
		t.Errorf("got %+v, quiere solo la entrada 'Real'", got)
	}
}

func TestEPGRepository_SaveBatch_EsUpsert(t *testing.T) {
	chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	seedChannelTvg(t, chRepo, "ch-1", "X.uk")

	e := epgEntry("X.uk", base, base.Add(time.Hour), "Título v1")
	if err := epgRepo.SaveBatch(ctx, []domain.EPGEntry{e}); err != nil {
		t.Fatalf("SaveBatch 1: %v", err)
	}
	// Re-sync: mismo canal+inicio, título corregido
	e.Title = "Título v2"
	if err := epgRepo.SaveBatch(ctx, []domain.EPGEntry{e}); err != nil {
		t.Fatalf("SaveBatch 2: %v", err)
	}

	got, _ := epgRepo.FindByChannelAndWindow(ctx, "ch-1", base, base.Add(time.Hour))
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (upsert no debe duplicar)", len(got))
	}
	if got[0].Title != "Título v2" {
		t.Errorf("Title = %q, quiere el actualizado", got[0].Title)
	}
}

func TestEPGRepository_FindByChannelAndWindow_SolapaYOrdena(t *testing.T) {
	chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	seedChannelTvg(t, chRepo, "ch-1", "X.uk")

	err := epgRepo.SaveBatch(ctx, []domain.EPGEntry{
		epgEntry("X.uk", base.Add(2*time.Hour), base.Add(3*time.Hour), "Tercero"),
		epgEntry("X.uk", base, base.Add(time.Hour), "Primero"),
		epgEntry("X.uk", base.Add(time.Hour), base.Add(2*time.Hour), "Segundo"),
		epgEntry("X.uk", base.Add(10*time.Hour), base.Add(11*time.Hour), "FueraDeVentana"),
	})
	if err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	// Ventana que solapa parcialmente el primero y el segundo
	got, err := epgRepo.FindByChannelAndWindow(ctx, "ch-1",
		base.Add(30*time.Minute), base.Add(90*time.Minute))
	if err != nil {
		t.Fatalf("FindByChannelAndWindow: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (solape parcial)", len(got))
	}
	if got[0].Title != "Primero" || got[1].Title != "Segundo" {
		t.Errorf("orden incorrecto: %q, %q", got[0].Title, got[1].Title)
	}
}

func TestEPGRepository_FindCurrentlyAiring(t *testing.T) {
	chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	seedChannelTvg(t, chRepo, "ch-1", "X.uk")
	seedChannelTvg(t, chRepo, "ch-2", "Y.fr")

	err := epgRepo.SaveBatch(ctx, []domain.EPGEntry{
		epgEntry("X.uk", base, base.Add(time.Hour), "EnEmision-X"),
		epgEntry("X.uk", base.Add(time.Hour), base.Add(2*time.Hour), "Siguiente-X"),
		epgEntry("Y.fr", base, base.Add(time.Hour), "EnEmision-Y"),
	})
	if err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	got, err := epgRepo.FindCurrentlyAiring(ctx, base.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("FindCurrentlyAiring: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (uno por canal en emisión)", len(got))
	}
	titles := map[string]bool{got[0].Title: true, got[1].Title: true}
	if !titles["EnEmision-X"] || !titles["EnEmision-Y"] {
		t.Errorf("got %+v, quiere EnEmision-X y EnEmision-Y", got)
	}
}

func TestEPGRepository_DeleteEndedBefore(t *testing.T) {
	chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Hour)
	seedChannelTvg(t, chRepo, "ch-1", "X.uk")

	err := epgRepo.SaveBatch(ctx, []domain.EPGEntry{
		epgEntry("X.uk", base.Add(-3*time.Hour), base.Add(-2*time.Hour), "Viejo"),
		epgEntry("X.uk", base, base.Add(time.Hour), "Vigente"),
	})
	if err != nil {
		t.Fatalf("SaveBatch: %v", err)
	}

	n, err := epgRepo.DeleteEndedBefore(ctx, base.Add(-time.Hour))
	if err != nil {
		t.Fatalf("DeleteEndedBefore: %v", err)
	}
	if n != 1 {
		t.Errorf("eliminadas = %d, want 1", n)
	}

	got, _ := epgRepo.FindByChannelAndWindow(ctx, "ch-1", base.Add(-4*time.Hour), base.Add(2*time.Hour))
	if len(got) != 1 || got[0].Title != "Vigente" {
		t.Errorf("got %+v, quiere solo 'Vigente'", got)
	}
}
