package db_test

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/adapters/db"
	"github.com/gdberysan/open-tv/internal/domain"
	"github.com/gdberysan/open-tv/internal/ports"
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

// Sin desempate, dos canales con el mismo nombre pueden salir en distinto
// orden entre dos queries independientes: una fila se repite en una página y
// desaparece de la otra.
func TestFindFilteredPaginacionEstableConNombresRepetidos(t *testing.T) {
	ctx := context.Background()
	repo := openTestDB(t)

	// 6 canales, todos con el mismo nombre: solo el ID los distingue.
	for _, id := range []string{"f", "e", "d", "c", "b", "a"} {
		ch := makeChannel(id, "Canal Duplicado", "ES", "news")
		if err := repo.Save(ctx, ch); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}

	var vistos []string
	for offset := 0; offset < 6; offset += 2 {
		page, err := repo.FindFiltered(ctx, ports.ChannelFilter{Limit: 2, Offset: offset})
		if err != nil {
			t.Fatalf("FindFiltered(offset=%d): %v", offset, err)
		}
		for _, ch := range page {
			vistos = append(vistos, string(ch.ID))
		}
	}

	if len(vistos) != 6 {
		t.Fatalf("quiero 6 filas paginadas, tengo %d", len(vistos))
	}
	unicos := map[string]bool{}
	for _, id := range vistos {
		if unicos[id] {
			t.Errorf("el canal %q apareció en dos páginas distintas", id)
		}
		unicos[id] = true
	}
	if len(unicos) != 6 {
		t.Errorf("la paginación omitió canales: %d únicos de 6", len(unicos))
	}
}

// CountFiltered tiene que aplicar exactamente los mismos filtros que
// FindFiltered, o el contador de la app mentiría.
func TestCountFilteredCoincideConFindFiltered(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	for i, pais := range []string{"ES", "ES", "MX", "GB"} {
		id := "ch-" + strconv.Itoa(i)
		ch := makeChannel(id, "Canal "+id+" (1080p)", pais, "news")
		if err := chRepo.Save(ctx, ch); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	casos := []ports.ChannelFilter{
		{},
		{Country: "ES"},
		{Country: "ES", MinQuality: "fhd"},
		{Query: "Canal", AliveOnly: true},
		{Country: "NO-EXISTE"},
	}
	for _, f := range casos {
		conLimite := f
		conLimite.Limit = 1000
		encontrados, err := chRepo.FindFiltered(ctx, conLimite)
		if err != nil {
			t.Fatalf("FindFiltered(%+v): %v", f, err)
		}
		total, err := chRepo.CountFiltered(ctx, f)
		if err != nil {
			t.Fatalf("CountFiltered(%+v): %v", f, err)
		}
		if total != len(encontrados) {
			t.Errorf("filtro %+v: count = %d, FindFiltered devolvió %d", f, total, len(encontrados))
		}
	}
}

// El contador ignora paginación: es el total que casa, no la página.
func TestCountFilteredIgnoraLimitYOffset(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	for i := 0; i < 5; i++ {
		id := "ch-" + strconv.Itoa(i)
		if err := chRepo.Save(ctx, makeChannel(id, "Canal "+id, "ES", "news")); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	total, err := chRepo.CountFiltered(ctx, ports.ChannelFilter{Limit: 2, Offset: 3})
	if err != nil {
		t.Fatalf("CountFiltered: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, quiero 5 (limit y offset no deben afectar)", total)
	}
}

// Las categorías de IPTV-org vienen capitalizadas ("News", "Sports"), así que
// un filtro sensible a mayúsculas devolvía cero ante lo que cualquiera teclearía.
func TestFindFilteredCategoriaIgnoraMayusculas(t *testing.T) {
	ctx := context.Background()
	repo := openTestDB(t)

	if err := repo.Save(ctx, makeChannel("ch-1", "Canal Uno", "ES", "News")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	for _, consulta := range []string{"News", "news", "NEWS"} {
		got, err := repo.FindFiltered(ctx, ports.ChannelFilter{Category: consulta, Limit: 10})
		if err != nil {
			t.Fatalf("FindFiltered(%q): %v", consulta, err)
		}
		if len(got) != 1 {
			t.Errorf("categoría %q: %d canales, quiero 1", consulta, len(got))
		}
	}
}

// Los category_id vienen compuestos ("Animation;Kids"), así que comparar por
// igualdad descartaba en silencio una cuarta parte de los canales infantiles.
func TestFindFilteredCategoriaCasaConCompuestas(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	casos := map[string]string{
		"ch-1": "Kids",
		"ch-2": "Animation;Kids",
		"ch-3": "Animation;Kids;Religious",
		"ch-4": "Movies",
	}
	for id, cat := range casos {
		if err := chRepo.Save(ctx, makeChannel(id, "Canal "+id, "ES", cat)); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{Category: "Kids", Limit: 100})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("categoría Kids devolvió %d, quiero 3 (incluye las compuestas)", len(got))
	}

	// No debe casar por subcadena suelta: "Kid" no es "Kids".
	parcial, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{Category: "Kid", Limit: 100})
	if err != nil {
		t.Fatalf("FindFiltered parcial: %v", err)
	}
	if len(parcial) != 0 {
		t.Errorf("'Kid' casó con %d canales; debe exigir la categoría completa", len(parcial))
	}
}

func TestCountriesDevuelveRecuentosOrdenados(t *testing.T) {
	ctx := context.Background()
	chRepo, _ := openStreamTestRepos(t)
	for i, p := range []string{"ES", "ES", "ES", "MX", "MX", "GB"} {
		if err := chRepo.Save(ctx, makeChannel("ch-"+strconv.Itoa(i), "C", p, "News")); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	got, err := chRepo.Countries(ctx)
	if err != nil {
		t.Fatalf("Countries: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("quiero 3 países, tengo %d", len(got))
	}
	// Ordenados por volumen: el selector los enseña así.
	if got[0].Valor != "ES" || got[0].Count != 3 {
		t.Errorf("primero = %+v, quiero ES con 3", got[0])
	}
}

// Las categorías se devuelven ATÓMICAS: "Animation;Kids" alimenta a las dos.
func TestCategoriesDescomponeLasCompuestas(t *testing.T) {
	ctx := context.Background()
	chRepo, _ := openStreamTestRepos(t)
	for i, c := range []string{"Kids", "Animation;Kids", "Movies"} {
		if err := chRepo.Save(ctx, makeChannel("ch-"+strconv.Itoa(i), "C", "ES", c)); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	got, err := chRepo.Categories(ctx)
	if err != nil {
		t.Fatalf("Categories: %v", err)
	}
	porNombre := map[string]int{}
	for _, f := range got {
		porNombre[f.Valor] = f.Count
	}
	if porNombre["Kids"] != 2 {
		t.Errorf("Kids = %d, quiero 2 (la compuesta cuenta)", porNombre["Kids"])
	}
	if porNombre["Animation"] != 1 {
		t.Errorf("Animation = %d, quiero 1", porNombre["Animation"])
	}
	if _, hay := porNombre["Animation;Kids"]; hay {
		t.Error("no debe aparecer la compuesta como categoría propia")
	}
}

// Un aleatorio muerto arruina la función: el sorteo es solo entre vivos.
func TestRandomSoloDevuelveCanalesVivos(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	for _, id := range []string{"vivo", "muerto"} {
		if err := chRepo.Save(ctx, makeChannel(id, "C-"+id, "ES", "News")); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}
	if err := stRepo.MarkAlive(ctx, "st-vivo", 100); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-muerto"); err != nil {
			t.Fatalf("MarkDead: %v", err)
		}
	}

	for i := 0; i < 20; i++ {
		got, err := chRepo.Random(ctx, ports.ChannelFilter{AliveOnly: true})
		if err != nil {
			t.Fatalf("Random: %v", err)
		}
		if got.ID != "vivo" {
			t.Fatalf("devolvió %q, que está muerto", got.ID)
		}
	}
}

// Sortear mal es fácil y silencioso: si siempre sale el mismo, no es aleatorio.
func TestRandomVaria(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	for i := 0; i < 10; i++ {
		id := "ch-" + strconv.Itoa(i)
		if err := chRepo.Save(ctx, makeChannel(id, "C"+id, "ES", "News")); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	vistos := map[string]bool{}
	for i := 0; i < 40; i++ {
		got, err := chRepo.Random(ctx, ports.ChannelFilter{AliveOnly: true})
		if err != nil {
			t.Fatalf("Random: %v", err)
		}
		vistos[string(got.ID)] = true
	}
	if len(vistos) < 3 {
		t.Errorf("40 sorteos dieron %d canales distintos; no parece aleatorio", len(vistos))
	}
}

func TestRandomRespetaElFiltro(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	for i, p := range []string{"ES", "MX", "MX"} {
		id := "ch-" + strconv.Itoa(i)
		if err := chRepo.Save(ctx, makeChannel(id, "C", p, "News")); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	for i := 0; i < 15; i++ {
		got, err := chRepo.Random(ctx, ports.ChannelFilter{Country: "MX", AliveOnly: true})
		if err != nil {
			t.Fatalf("Random: %v", err)
		}
		if got.CountryCode != "MX" {
			t.Fatalf("devolvió un canal de %q con filtro MX", got.CountryCode)
		}
	}
}

// Los favoritos son un concepto del cliente: se resuelven pidiendo sus ids, no
// paginando el catálogo — un favorito puede estar en la página 20.
func TestFindFilteredPorIDs(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	for i := 0; i < 5; i++ {
		id := "ch-" + strconv.Itoa(i)
		if err := chRepo.Save(ctx, makeChannel(id, "C"+id, "ES", "News")); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	got, err := chRepo.FindFiltered(ctx, ports.ChannelFilter{
		IDs:   []string{"ch-1", "ch-3"},
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("quiero 2 canales, tengo %d", len(got))
	}
}
