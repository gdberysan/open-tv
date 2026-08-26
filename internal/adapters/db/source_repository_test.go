package db_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/ports"
)

func openSourceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

// (a) Una instalación limpia (DB nueva) no trae ninguna fuente precargada:
// el catálogo depende por completo de lo que el usuario dé de alta.
func TestSourceRepository_InstalacionLimpiaSinFuentes(t *testing.T) {
	w := openSourceTestDB(t)
	repo := db.NewSourceRepository(w)

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List() = %d fuentes, quiero 0 en una instalación limpia", len(got))
	}
}

// (b) Una fila de providers preexistente (de una DB de antes de este cambio)
// sobrevive a Open: la migración no puede ser destructiva.
func TestSourceRepository_MigracionNoBorraProvidersPreexistentes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open (primera vez): %v", err)
	}
	now := time.Now().Unix()
	if _, err := sqlDB.Exec(`
		INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
		VALUES ('preexistente', 'opensource', 'http://viejo.example/index.m3u', 100, 1, ?, ?)
	`, now, now); err != nil {
		t.Fatalf("insertando provider preexistente: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reabrir: dispara alterMigrations sobre una DB que ya tiene la fila.
	sqlDB2, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open (segunda vez): %v", err)
	}
	defer func() { _ = sqlDB2.Close() }()

	repo := db.NewSourceRepository(sqlDB2)
	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != "preexistente" {
		t.Fatalf("List() = %+v, quiero la fila preexistente intacta", got)
	}
	if got[0].URL != "http://viejo.example/index.m3u" {
		t.Errorf("URL = %q, quiero la URL original preservada", got[0].URL)
	}
}

// (c) Add + List roundtrip con label y kind.
func TestSourceRepository_AddYListRoundtrip(t *testing.T) {
	w := openSourceTestDB(t)
	repo := db.NewSourceRepository(w)
	ctx := context.Background()

	s := ports.Source{
		Label:    "Mi lista",
		URL:      "http://ejemplo.example/canales.m3u",
		Kind:     "url",
		IsActive: true,
	}
	if err := repo.Add(ctx, s); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() = %d fuentes, quiero 1", len(got))
	}
	if got[0].Label != "Mi lista" {
		t.Errorf("Label = %q, quiero %q", got[0].Label, "Mi lista")
	}
	if got[0].URL != s.URL {
		t.Errorf("URL = %q, quiero %q", got[0].URL, s.URL)
	}
	if got[0].Kind != "url" {
		t.Errorf("Kind = %q, quiero %q", got[0].Kind, "url")
	}
	if !got[0].IsActive {
		t.Error("IsActive = false, quiero true")
	}
	if got[0].ID == "" {
		t.Error("ID vacío tras Add")
	}
}

// (d) Añadir la misma URL dos veces (mismo id fnv64a) debe fallar con un
// error reconocible, no crear una segunda fuente ni pisar la primera.
func TestSourceRepository_AddDuplicadaFalla(t *testing.T) {
	w := openSourceTestDB(t)
	repo := db.NewSourceRepository(w)
	ctx := context.Background()

	s := ports.Source{Label: "Uno", URL: "http://dup.example/x.m3u", Kind: "url"}
	if err := repo.Add(ctx, s); err != nil {
		t.Fatalf("primer Add: %v", err)
	}

	err := repo.Add(ctx, ports.Source{Label: "Dos", URL: s.URL, Kind: "url"})
	if err == nil {
		t.Fatal("segundo Add con la misma URL debería fallar")
	}
	if !errors.Is(err, ports.ErrFuenteDuplicada) {
		t.Errorf("err = %v, quiero que envuelva ports.ErrFuenteDuplicada", err)
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() = %d fuentes tras el duplicado, quiero 1", len(got))
	}
}

// (e) Remove borra la fuente y sus canales/streams, sin tocar los de otra
// fuente distinta.
func TestSourceRepository_RemoveBorraSoloSusDatos(t *testing.T) {
	w := openSourceTestDB(t)
	repo := db.NewSourceRepository(w)
	chRepo := db.NewChannelRepository(w)
	stRepo := db.NewStreamRepository(w)
	ctx := context.Background()

	uno := ports.Source{Label: "Uno", URL: "http://a.example/1.m3u", Kind: "url"}
	dos := ports.Source{Label: "Dos", URL: "http://b.example/2.m3u", Kind: "url"}
	if err := repo.Add(ctx, uno); err != nil {
		t.Fatalf("Add uno: %v", err)
	}
	if err := repo.Add(ctx, dos); err != nil {
		t.Fatalf("Add dos: %v", err)
	}

	fuentes, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var idUno, idDos string
	for _, f := range fuentes {
		switch f.URL {
		case uno.URL:
			idUno = f.ID
		case dos.URL:
			idDos = f.ID
		}
	}
	if idUno == "" || idDos == "" {
		t.Fatalf("no se resolvieron los IDs de las fuentes: %+v", fuentes)
	}

	chUno := makeChannel("ch-uno", "Canal Uno", "ES", "news")
	chUno.ProviderID = idUno
	if err := chRepo.Save(ctx, chUno); err != nil {
		t.Fatalf("Save canal de uno: %v", err)
	}
	if err := stRepo.Save(ctx, makeStream("st-uno", "ch-uno", "http://a.example/stream.m3u8")); err != nil {
		t.Fatalf("Save stream de uno: %v", err)
	}

	chDos := makeChannel("ch-dos", "Canal Dos", "ES", "news")
	chDos.ProviderID = idDos
	if err := chRepo.Save(ctx, chDos); err != nil {
		t.Fatalf("Save canal de dos: %v", err)
	}
	if err := stRepo.Save(ctx, makeStream("st-dos", "ch-dos", "http://b.example/stream.m3u8")); err != nil {
		t.Fatalf("Save stream de dos: %v", err)
	}

	if err := repo.Remove(ctx, idUno); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	restantes, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List tras Remove: %v", err)
	}
	if len(restantes) != 1 || restantes[0].ID != idDos {
		t.Fatalf("List() tras Remove = %+v, quiero solo la fuente 'dos'", restantes)
	}

	if _, err := chRepo.FindByID(ctx, "ch-uno"); err == nil {
		t.Error("el canal de la fuente borrada debería haber desaparecido")
	}
	streamsUno, err := stRepo.FindByChannelID(ctx, "ch-uno")
	if err != nil {
		t.Fatalf("FindByChannelID('ch-uno'): %v", err)
	}
	if len(streamsUno) != 0 {
		t.Errorf("los streams del canal borrado deben caer por cascada; quedan %d", len(streamsUno))
	}

	chDosRestante, err := chRepo.FindByID(ctx, "ch-dos")
	if err != nil {
		t.Fatalf("el canal de la fuente NO borrada debería seguir ahí: %v", err)
	}
	if chDosRestante.ID != "ch-dos" {
		t.Errorf("canal inesperado: %+v", chDosRestante)
	}
	streamsDos, err := stRepo.FindByChannelID(ctx, "ch-dos")
	if err != nil {
		t.Fatalf("FindByChannelID('ch-dos'): %v", err)
	}
	if len(streamsDos) != 1 {
		t.Errorf("el stream de la fuente NO borrada debería seguir ahí; tengo %d", len(streamsDos))
	}
}

// (f) TouchSync actualiza el timestamp de última sincronización que List reporta.
func TestSourceRepository_TouchSync(t *testing.T) {
	w := openSourceTestDB(t)
	repo := db.NewSourceRepository(w)
	ctx := context.Background()

	if err := repo.Add(ctx, ports.Source{Label: "X", URL: "http://x.example/x.m3u", Kind: "url"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	fuentes, err := repo.List(ctx)
	if err != nil || len(fuentes) != 1 {
		t.Fatalf("List: %v / %+v", err, fuentes)
	}
	id := fuentes[0].ID
	if fuentes[0].UltimoSync != 0 {
		t.Errorf("UltimoSync antes de TouchSync = %d, quiero 0", fuentes[0].UltimoSync)
	}

	cuando := time.Now().Unix()
	if err := repo.TouchSync(ctx, id, cuando); err != nil {
		t.Fatalf("TouchSync: %v", err)
	}

	fuentes, err = repo.List(ctx)
	if err != nil || len(fuentes) != 1 {
		t.Fatalf("List tras TouchSync: %v / %+v", err, fuentes)
	}
	if fuentes[0].UltimoSync != cuando {
		t.Errorf("UltimoSync = %d, quiero %d", fuentes[0].UltimoSync, cuando)
	}
}

// (g) SetTvgURL persiste la url-tvg declarada por la cabecera M3U de la
// fuente; por defecto (sin fijarla) es "".
func TestSourceRepository_SetTvgURL(t *testing.T) {
	w := openSourceTestDB(t)
	repo := db.NewSourceRepository(w)
	ctx := context.Background()

	if err := repo.Add(ctx, ports.Source{Label: "X", URL: "http://tvg.example/x.m3u", Kind: "url"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	fuentes, err := repo.List(ctx)
	if err != nil || len(fuentes) != 1 {
		t.Fatalf("List: %v / %+v", err, fuentes)
	}
	id := fuentes[0].ID
	if fuentes[0].TvgURL != "" {
		t.Errorf("TvgURL antes de SetTvgURL = %q, quiero \"\"", fuentes[0].TvgURL)
	}

	if err := repo.SetTvgURL(ctx, id, "https://e/g.xml"); err != nil {
		t.Fatalf("SetTvgURL: %v", err)
	}

	fuentes, err = repo.List(ctx)
	if err != nil || len(fuentes) != 1 {
		t.Fatalf("List tras SetTvgURL: %v / %+v", err, fuentes)
	}
	if fuentes[0].TvgURL != "https://e/g.xml" {
		t.Errorf("TvgURL = %q, quiero %q", fuentes[0].TvgURL, "https://e/g.xml")
	}
}

// Canales en List refleja el recuento real de canales de esa fuente.
func TestSourceRepository_ListCuentaCanales(t *testing.T) {
	w := openSourceTestDB(t)
	repo := db.NewSourceRepository(w)
	chRepo := db.NewChannelRepository(w)
	ctx := context.Background()

	if err := repo.Add(ctx, ports.Source{Label: "X", URL: "http://cont.example/x.m3u", Kind: "url"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	fuentes, err := repo.List(ctx)
	if err != nil || len(fuentes) != 1 {
		t.Fatalf("List: %v / %+v", err, fuentes)
	}
	id := fuentes[0].ID
	if fuentes[0].Canales != 0 {
		t.Errorf("Canales antes de sync = %d, quiero 0", fuentes[0].Canales)
	}

	for _, chID := range []string{"c1", "c2", "c3"} {
		ch := makeChannel(chID, "Canal "+chID, "ES", "news")
		ch.ProviderID = id
		if err := chRepo.Save(ctx, ch); err != nil {
			t.Fatalf("Save(%s): %v", chID, err)
		}
	}

	fuentes, err = repo.List(ctx)
	if err != nil || len(fuentes) != 1 {
		t.Fatalf("List: %v / %+v", err, fuentes)
	}
	if fuentes[0].Canales != 3 {
		t.Errorf("Canales = %d, quiero 3", fuentes[0].Canales)
	}
}
