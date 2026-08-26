package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/domain"
)

// openEPGTestRepos abre una DB de test con el provider "opensource" ya dado
// de alta (ver seedProviderOpensource) y devuelve el *sql.DB crudo (para
// contar filas directamente), el repo de canales (para sembrar canales con
// tvg_id) y el repo de EPG bajo prueba.
func openEPGTestRepos(t *testing.T) (*sql.DB, *db.SQLiteChannelRepository, *db.SQLiteEPGRepository) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	seedProviderOpensource(t, sqlDB)
	return sqlDB, db.NewChannelRepository(sqlDB), db.NewEPGRepository(sqlDB)
}

// seedProvider da de alta un provider adicional con el id dado. Hace falta
// para los tests de aislamiento: dos fuentes, cada una con SU guía propia.
func seedProvider(t *testing.T, sqlDB *sql.DB, id string) {
	t.Helper()
	if _, err := sqlDB.Exec(`
		INSERT INTO providers (id, type, base_url, priority, is_active, created_at, updated_at)
		VALUES (?, 'opensource', 'http://test.invalid/'||?||'.m3u', 100, 1, 0, 0)
	`, id, id); err != nil {
		t.Fatalf("seedProvider(%s): %v", id, err)
	}
}

// epgChannel crea un canal con tvg_id explícito, atado al provider dado.
// makeChannel (de channel_repository_test.go) fija ProviderID a "opensource"
// siempre; estos tests necesitan providers arbitrarios para el aislamiento.
func epgChannel(id, tvgID, providerID string) domain.Channel {
	return domain.Channel{
		ID:           domain.ChannelID(id),
		TvgID:        tvgID,
		Name:         "Canal " + id,
		ProviderID:   providerID,
		ProviderType: domain.ProviderOpenSource,
	}
}

func programa(tvgChannelID string, inicio, fin int64, titulo string) domain.Programa {
	return domain.Programa{
		ChannelID: tvgChannelID,
		InicioUTC: inicio,
		FinUTC:    fin,
		Titulo:    titulo,
	}
}

func countEPGRows(t *testing.T, sqlDB *sql.DB, providerID string) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM epg_programmes WHERE provider_id = ?`, providerID).Scan(&n); err != nil {
		t.Fatalf("countEPGRows(%s): %v", providerID, err)
	}
	return n
}

// ── ReemplazarVentana ────────────────────────────────────────────────────────

func TestEPGRepository_ReemplazarVentanaNoAcumula(t *testing.T) {
	sqlDB, _, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()

	primeros := []domain.Programa{
		programa("TVG1", 1000, 2000, "Uno"),
		programa("TVG1", 2000, 3000, "Dos"),
	}
	if err := epgRepo.ReemplazarVentana(ctx, "opensource", primeros); err != nil {
		t.Fatalf("ReemplazarVentana (1): %v", err)
	}
	if n := countEPGRows(t, sqlDB, "opensource"); n != 2 {
		t.Fatalf("tras la 1ª ventana, filas = %d, want 2", n)
	}

	segunda := []domain.Programa{
		programa("TVG1", 5000, 6000, "Solo"),
	}
	if err := epgRepo.ReemplazarVentana(ctx, "opensource", segunda); err != nil {
		t.Fatalf("ReemplazarVentana (2): %v", err)
	}
	if n := countEPGRows(t, sqlDB, "opensource"); n != 1 {
		t.Fatalf("tras la 2ª ventana, filas = %d, want 1 (no debe acumular sobre la anterior)", n)
	}
}

func TestEPGRepository_ReemplazarVentanaVaciaBorraLaGuia(t *testing.T) {
	sqlDB, _, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()

	if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
		programa("TVG1", 1, 2, "X"),
	}); err != nil {
		t.Fatalf("ReemplazarVentana: %v", err)
	}
	if err := epgRepo.ReemplazarVentana(ctx, "opensource", nil); err != nil {
		t.Fatalf("ReemplazarVentana vacía: %v", err)
	}
	if n := countEPGRows(t, sqlDB, "opensource"); n != 0 {
		t.Errorf("filas tras ventana vacía = %d, want 0", n)
	}
}

// Reemplazar la ventana de un provider no puede tocar la guía de otro: cada
// sync de fuente es independiente.
func TestEPGRepository_ReemplazarVentanaNoTocaOtroProvider(t *testing.T) {
	sqlDB, _, epgRepo := openEPGTestRepos(t)
	seedProvider(t, sqlDB, "otro")
	ctx := context.Background()

	if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
		programa("TVG1", 1, 2, "A"),
	}); err != nil {
		t.Fatalf("ReemplazarVentana opensource: %v", err)
	}
	if err := epgRepo.ReemplazarVentana(ctx, "otro", []domain.Programa{
		programa("TVG1", 1, 2, "B"),
	}); err != nil {
		t.Fatalf("ReemplazarVentana otro: %v", err)
	}

	if err := epgRepo.ReemplazarVentana(ctx, "opensource", nil); err != nil {
		t.Fatalf("ReemplazarVentana opensource vacía: %v", err)
	}

	if n := countEPGRows(t, sqlDB, "otro"); n != 1 {
		t.Errorf("filas de 'otro' = %d, want 1 (reemplazar la ventana de opensource no debe tocarlo)", n)
	}
}

// ── Podar ────────────────────────────────────────────────────────────────────

func TestEPGRepository_Podar(t *testing.T) {
	sqlDB, _, epgRepo := openEPGTestRepos(t)
	seedProvider(t, sqlDB, "otro")
	ctx := context.Background()

	if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
		programa("TVG1", 1000, 2000, "Pasado"), // stop 2000 < corte 2500 → se poda
		programa("TVG1", 2000, 3000, "Futuro"), // stop 3000 >= corte → se queda
	}); err != nil {
		t.Fatalf("ReemplazarVentana opensource: %v", err)
	}
	if err := epgRepo.ReemplazarVentana(ctx, "otro", []domain.Programa{
		programa("TVG1", 500, 1500, "PasadoOtro"), // stop 1500 < corte → se poda
	}); err != nil {
		t.Fatalf("ReemplazarVentana otro: %v", err)
	}

	if err := epgRepo.Podar(ctx, 2500); err != nil {
		t.Fatalf("Podar: %v", err)
	}

	if n := countEPGRows(t, sqlDB, "opensource"); n != 1 {
		t.Errorf("opensource: filas tras podar = %d, want 1", n)
	}
	if n := countEPGRows(t, sqlDB, "otro"); n != 0 {
		t.Errorf("otro: filas tras podar = %d, want 0 (Podar cruza providers)", n)
	}
}

// ── AhoraDespuesPorCanales: aislamiento por fuente ──────────────────────────

// Dos providers distintos traen el MISMO tvg_id con guías distintas; cada
// canal de la app (uno por provider) debe recibir SOLO el programa de SU
// fuente. Es el requisito central de la Tarea 3.
func TestEPGRepository_AhoraDespuesPorCanalesAislaPorProvider(t *testing.T) {
	sqlDB, chRepo, epgRepo := openEPGTestRepos(t)
	seedProvider(t, sqlDB, "p2")
	ctx := context.Background()

	if err := chRepo.Save(ctx, epgChannel("ch1", "TVGX", "opensource")); err != nil {
		t.Fatalf("Save ch1: %v", err)
	}
	if err := chRepo.Save(ctx, epgChannel("ch2", "TVGX", "p2")); err != nil {
		t.Fatalf("Save ch2: %v", err)
	}

	if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
		programa("TVGX", 500, 1500, "P1-Ahora"),
	}); err != nil {
		t.Fatalf("ReemplazarVentana opensource: %v", err)
	}
	if err := epgRepo.ReemplazarVentana(ctx, "p2", []domain.Programa{
		programa("TVGX", 500, 1500, "P2-Ahora"),
	}); err != nil {
		t.Fatalf("ReemplazarVentana p2: %v", err)
	}

	m, err := epgRepo.AhoraDespuesPorCanales(ctx, []string{"ch1", "ch2"}, 1000)
	if err != nil {
		t.Fatalf("AhoraDespuesPorCanales: %v", err)
	}

	ad1, ok := m["ch1"]
	if !ok || ad1.Ahora == nil {
		t.Fatalf("ch1: ausente o sin Ahora (ok=%v): %+v", ok, ad1)
	}
	if ad1.Ahora.Titulo != "P1-Ahora" {
		t.Errorf("ch1.Ahora.Titulo = %q, want P1-Ahora (JAMÁS cruzado con la guía de p2)", ad1.Ahora.Titulo)
	}

	ad2, ok := m["ch2"]
	if !ok || ad2.Ahora == nil {
		t.Fatalf("ch2: ausente o sin Ahora (ok=%v): %+v", ok, ad2)
	}
	if ad2.Ahora.Titulo != "P2-Ahora" {
		t.Errorf("ch2.Ahora.Titulo = %q, want P2-Ahora (JAMÁS cruzado con la guía de opensource)", ad2.Ahora.Titulo)
	}
}

// ── AhoraDespuesPorCanales: límites y casos borde ───────────────────────────

func TestEPGRepository_AhoraDespuesPorCanalesLimites(t *testing.T) {
	_, chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()

	if err := chRepo.Save(ctx, epgChannel("ch1", "TVG1", "opensource")); err != nil {
		t.Fatalf("Save ch1: %v", err)
	}

	t.Run("ahora justo en start cuenta como Ahora", func(t *testing.T) {
		if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
			programa("TVG1", 1000, 2000, "EnCurso"),
		}); err != nil {
			t.Fatalf("ReemplazarVentana: %v", err)
		}
		m, err := epgRepo.AhoraDespuesPorCanales(ctx, []string{"ch1"}, 1000)
		if err != nil {
			t.Fatalf("AhoraDespuesPorCanales: %v", err)
		}
		if m["ch1"].Ahora == nil || m["ch1"].Ahora.Titulo != "EnCurso" {
			t.Errorf("ahora=start: Ahora = %+v, want EnCurso", m["ch1"].Ahora)
		}
	})

	t.Run("ahora justo en stop NO cuenta como Ahora; hueco tras él", func(t *testing.T) {
		if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
			programa("TVG1", 1000, 2000, "Anterior"),
			// hueco entre 2000 y 3000: nada empieza justo donde acaba "Anterior"
			programa("TVG1", 3000, 4000, "Posterior"),
		}); err != nil {
			t.Fatalf("ReemplazarVentana: %v", err)
		}
		m, err := epgRepo.AhoraDespuesPorCanales(ctx, []string{"ch1"}, 2000)
		if err != nil {
			t.Fatalf("AhoraDespuesPorCanales: %v", err)
		}
		ad := m["ch1"]
		if ad.Ahora != nil {
			t.Errorf("ahora=stop de 'Anterior': Ahora = %+v, want nil (stop es exclusivo)", ad.Ahora)
		}
		if ad.Siguiente == nil || ad.Siguiente.Titulo != "Posterior" {
			t.Errorf("Siguiente = %+v, want Posterior", ad.Siguiente)
		}
	})

	t.Run("ahora justo en stop pero contiguo: el que empieza ahí SÍ es Ahora", func(t *testing.T) {
		if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
			programa("TVG1", 1000, 2000, "Anterior"),
			programa("TVG1", 2000, 3000, "Contiguo"), // empieza justo donde acaba el anterior
		}); err != nil {
			t.Fatalf("ReemplazarVentana: %v", err)
		}
		m, err := epgRepo.AhoraDespuesPorCanales(ctx, []string{"ch1"}, 2000)
		if err != nil {
			t.Fatalf("AhoraDespuesPorCanales: %v", err)
		}
		if m["ch1"].Ahora == nil || m["ch1"].Ahora.Titulo != "Contiguo" {
			t.Errorf("Ahora = %+v, want Contiguo", m["ch1"].Ahora)
		}
	})

	t.Run("hueco entre programas: Ahora nil, Siguiente el próximo por start", func(t *testing.T) {
		if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
			programa("TVG1", 1000, 2000, "Antes"),
			programa("TVG1", 5000, 6000, "Después"),
		}); err != nil {
			t.Fatalf("ReemplazarVentana: %v", err)
		}
		m, err := epgRepo.AhoraDespuesPorCanales(ctx, []string{"ch1"}, 3000)
		if err != nil {
			t.Fatalf("AhoraDespuesPorCanales: %v", err)
		}
		ad := m["ch1"]
		if ad.Ahora != nil {
			t.Errorf("hueco: Ahora = %+v, want nil", ad.Ahora)
		}
		if ad.Siguiente == nil || ad.Siguiente.Titulo != "Después" {
			t.Errorf("Siguiente = %+v, want Después", ad.Siguiente)
		}
	})

	t.Run("sin filas para el canal: presente en el mapa con Ahora y Siguiente nil", func(t *testing.T) {
		if err := epgRepo.ReemplazarVentana(ctx, "opensource", nil); err != nil {
			t.Fatalf("ReemplazarVentana vacía: %v", err)
		}
		m, err := epgRepo.AhoraDespuesPorCanales(ctx, []string{"ch1"}, 1000)
		if err != nil {
			t.Fatalf("AhoraDespuesPorCanales: %v", err)
		}
		ad, ok := m["ch1"]
		if !ok {
			t.Fatal("ch1 tiene tvg_id pero ninguna fila de guía: debe estar PRESENTE en el mapa")
		}
		if ad.Ahora != nil || ad.Siguiente != nil {
			t.Errorf("sin filas: AhoraDespues = %+v, want ambos nil", ad)
		}
	})

	t.Run("tvg_id vacío en el canal: AUSENTE del mapa (sin guía posible)", func(t *testing.T) {
		if err := chRepo.Save(ctx, epgChannel("ch-sin-tvg", "", "opensource")); err != nil {
			t.Fatalf("Save ch-sin-tvg: %v", err)
		}
		m, err := epgRepo.AhoraDespuesPorCanales(ctx, []string{"ch-sin-tvg"}, 1000)
		if err != nil {
			t.Fatalf("AhoraDespuesPorCanales: %v", err)
		}
		if ad, ok := m["ch-sin-tvg"]; ok {
			t.Errorf("canal sin tvg_id: debe estar AUSENTE del mapa, pero apareció: %+v", ad)
		}
	})
}

// ── ProximosDeCanal ──────────────────────────────────────────────────────────

func TestEPGRepository_ProximosDeCanal(t *testing.T) {
	_, chRepo, epgRepo := openEPGTestRepos(t)
	ctx := context.Background()

	if err := chRepo.Save(ctx, epgChannel("ch1", "TVG1", "opensource")); err != nil {
		t.Fatalf("Save ch1: %v", err)
	}
	if err := epgRepo.ReemplazarVentana(ctx, "opensource", []domain.Programa{
		programa("TVG1", 1000, 2000, "EnCurso"),
		programa("TVG1", 2000, 3000, "Siguiente1"),
		programa("TVG1", 3000, 4000, "Siguiente2"),
		programa("TVG1", 4000, 5000, "Siguiente3"),
	}); err != nil {
		t.Fatalf("ReemplazarVentana: %v", err)
	}

	got, err := epgRepo.ProximosDeCanal(ctx, "ch1", 1500, 2)
	if err != nil {
		t.Fatalf("ProximosDeCanal: %v", err)
	}
	want := []string{"EnCurso", "Siguiente1", "Siguiente2"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Titulo != w {
			t.Errorf("got[%d].Titulo = %q, want %q", i, got[i].Titulo, w)
		}
	}
}

func TestEPGRepository_ProximosDeCanalCanalDesconocido(t *testing.T) {
	_, _, epgRepo := openEPGTestRepos(t)
	got, err := epgRepo.ProximosDeCanal(context.Background(), "no-existe", 1000, 5)
	if err != nil {
		t.Fatalf("ProximosDeCanal: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
