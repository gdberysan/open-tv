# Filtros por modal, rejilla, favoritos y aleatorio — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hacer navegable un catálogo de 9 100 canales: filtros por modal con datos reales, rejilla de logos, favoritos y canal aleatorio.

**Architecture:** Seis fases. La 1 arregla el bug de categorías compuestas y añade los endpoints que alimentan los selectores — todo lo demás depende de esos datos. La 2 son los tres modales de filtro. La 3 la rejilla. La 4 favoritos. La 5 el aleatorio. La 6 verificación end-to-end.

Todo endpoint nuevo reutiliza `buildChannelWhere`; es el mismo principio que hizo honesto al contador.

**Tech Stack:** Go 1.25 (chi, modernc/sqlite), Flutter 3.44 (Riverpod, SharedPreferences). **Cero dependencias nuevas**: iconos de Material, banderas derivadas del código ISO, favoritos como `Set<String>` en SharedPreferences.

**Spec:** `docs/superpowers/specs/2026-08-08-filtros-rejilla-favoritos-design.md`

## Global Constraints

- Regla del ámbar: acento solo para señal viva — canal vivo, filtro activo, foco.
- Ningún widget hardcodea colores: todo del tema o de los tokens de `lib/theme/`.
- Tras cada tarea: `flutter analyze` limpio y `flutter test` verde; en el gateway
  `go build ./... && go vet ./... && gofmt -l . && go test -race ./...`.
- Comentarios y commits en español, como el resto del repo.
- Un commit por tarea.

---

# FASE 1 — Gateway: categorías reales y datos para los selectores

### Task 1: Categorías compuestas

**Files:** `gateway/internal/adapters/db/channel_repository.go`, `channel_repository_test.go`

**Contexto:** `category_id` guarda valores compuestos (`Animation;Kids`). El filtro compara con `=`, así que filtrar por `Kids` devuelve 253 en vez de 337.

- [ ] **Step 1: Test que falla**

```go
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
```

- [ ] **Step 2: Verlo fallar**

Run: `cd gateway && go test ./internal/adapters/db/ -run TestFindFilteredCategoriaCasaConCompuestas`
Esperado: FAIL, devuelve 1 en vez de 3.

- [ ] **Step 3: Comparar por pertenencia**

En `buildChannelWhere`, sustituir la cláusula de categoría:

```go
	if f.Category != "" {
		// Los category_id son compuestos ("Animation;Kids"), así que hay que
		// preguntar por pertenencia y no por igualdad. Los ';' de guarda evitan
		// que "Kid" case con "Kids". Medido: 7ms de escaneo sobre 12k filas, no
		// compensa índice. LIKE ya es insensible a mayúsculas para ASCII, lo que
		// además resuelve news/News.
		where = append(where, "';' || category_id || ';' LIKE ?")
		args = append(args, "%;"+f.Category+";%")
	}
```

- [ ] **Step 4: Verlo pasar y correr la suite**

Run: `go test -race -count=1 ./...` → todo verde. `TestCountFilteredCoincideConFindFiltered` sigue pasando: ambos usan el mismo helper.

- [ ] **Step 5: Commit**

```bash
git commit -am "gateway: el filtro de categoría casa con las compuestas"
```

---

### Task 2: Endpoints de países y categorías

**Files:** `gateway/internal/ports/channel_repository.go`, `adapters/db/channel_repository.go`, `api/handlers/channel_handler.go`, `api/router.go`, + tests

**Interfaces:**
- `ChannelRepository.Countries(ctx) ([]ports.Faceta, error)` y `Categories(ctx) ([]ports.Faceta, error)`, con `type Faceta struct { Valor string; Count int }`.
- `GET /channels/countries` → `[{"Valor":"US","Count":1935}, …]`
- `GET /channels/categories` → `[{"Valor":"News","Count":1002}, …]`

Ambos son los *quick wins* pendientes de `PROMPT_MAESTRO.md` §9.

- [ ] **Step 1: Tests que fallan**

```go
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
```

- [ ] **Step 2: Verlos fallar** → `Countries undefined`.

- [ ] **Step 3: Implementar**

En `ports/channel_repository.go`:

```go
// Faceta es un valor de filtro con su número de canales, para los selectores.
type Faceta struct {
	Valor string
	Count int
}
```

En el repositorio. Países sale directo de SQL; categorías se descomponen en Go
porque SQLite no trae `split`:

```go
func (r *SQLiteChannelRepository) Countries(ctx context.Context) ([]ports.Faceta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT country_code, COUNT(*) FROM channels
		WHERE country_code IS NOT NULL AND country_code != ''
		GROUP BY country_code ORDER BY COUNT(*) DESC, country_code`)
	if err != nil {
		return nil, fmt.Errorf("db.Countries: %w", err)
	}
	defer rows.Close()

	var out []ports.Faceta
	for rows.Next() {
		var f ports.Faceta
		if err := rows.Scan(&f.Valor, &f.Count); err != nil {
			return nil, fmt.Errorf("db.Countries (scan): %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Categories descompone los category_id compuestos ("Animation;Kids") en sus
// categorías atómicas y las cuenta por separado. SQLite no tiene split, así que
// el troceo va en Go; son 12k filas, es despreciable.
func (r *SQLiteChannelRepository) Categories(ctx context.Context) ([]ports.Faceta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category_id, COUNT(*) FROM channels
		WHERE category_id IS NOT NULL AND category_id != ''
		GROUP BY category_id`)
	if err != nil {
		return nil, fmt.Errorf("db.Categories: %w", err)
	}
	defer rows.Close()

	acum := map[string]int{}
	for rows.Next() {
		var compuesta string
		var n int
		if err := rows.Scan(&compuesta, &n); err != nil {
			return nil, fmt.Errorf("db.Categories (scan): %w", err)
		}
		for _, atomica := range strings.Split(compuesta, ";") {
			if atomica = strings.TrimSpace(atomica); atomica != "" {
				acum[atomica] += n
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.Categories (rows.Err): %w", err)
	}

	out := make([]ports.Faceta, 0, len(acum))
	for v, n := range acum {
		out = append(out, ports.Faceta{Valor: v, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Valor < out[j].Valor
	})
	return out, nil
}
```

Handlers `GetCountries` y `GetCategories` que devuelven el slice como JSON, con
el mismo patrón de log de error que el resto. Rutas en `router.go` dentro de
`/channels`: `r.Get("/countries", ch.GetCountries)` y
`r.Get("/categories", ch.GetCategories)`.

⚠️ Registrarlas **antes** que `/{id}/health` no hace falta —chi distingue rutas
literales de parámetros— pero conviene comprobarlo con el test de abajo.

- [ ] **Step 4: Test de routing**

```go
func TestRutasDeFacetasNoChocanConLosParametros(t *testing.T) {
	r := setupRouter()
	for _, ruta := range []string{"/channels/countries", "/channels/categories"} {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, ruta, nil))
		if rr.Code != http.StatusOK {
			t.Errorf("%s = %d, quiero 200 (¿la absorbió /{id}/health?)", ruta, rr.Code)
		}
	}
}
```

Actualizar los fakes de `ChannelRepository` con los dos métodos nuevos.

- [ ] **Step 5: Suite, verificación real y commit**

```bash
go test -race -count=1 ./... && go vet ./... && gofmt -l .
go build -o server ./cmd/server && ./server &
curl -s localhost:8080/channels/categories | python3 -m json.tool | head -12
curl -s localhost:8080/channels/countries | python3 -m json.tool | head -8
```

Esperado: `Undefined` 3350 y `General` 2821 arriba en categorías; `US` 1935 en
países. Comprobar que **no** aparece ninguna compuesta con `;`.

```bash
git commit -am "gateway: endpoints de países y categorías con recuentos reales"
```

---

### Task 3: Canal aleatorio y filtro por ids

**Files:** repositorio, handler, router, ports, + tests

**Interfaces:**
- `ChannelRepository.Random(ctx, f) (domain.Channel, error)` — un canal **vivo** al azar que case con el filtro.
- `ChannelFilter.IDs []string` — nuevo campo; cuando viene, `buildChannelWhere` añade `id IN (…)`.
- `GET /channels/random?country=&category=&quality=` → un canal, o 404 si no hay.
- `GET /channels?ids=a,b,c`

- [ ] **Step 1: Tests que fallan**

```go
// Un aleatorio muerto arruina la función: el sorteo es solo entre vivos.
func TestRandomSoloDevuelveCanalesVivos(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	chRepo.Save(ctx, makeChannel("vivo", "Vivo", "ES", "News"))
	chRepo.Save(ctx, makeChannel("muerto", "Muerto", "ES", "News"))
	stRepo.Save(ctx, makeStream("st-v", "vivo", "http://a/v.m3u8"))
	stRepo.Save(ctx, makeStream("st-m", "muerto", "http://a/m.m3u8"))
	stRepo.MarkAlive(ctx, "st-v", 100)
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		stRepo.MarkDead(ctx, "st-m")
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
		chRepo.Save(ctx, makeChannel(id, "C"+id, "ES", "News"))
		stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8"))
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
		chRepo.Save(ctx, makeChannel(id, "C", p, "News"))
		stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8"))
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

// Los favoritos son un concepto del cliente: se resuelven pidiendo sus ids.
func TestFindFilteredPorIDs(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	for i := 0; i < 5; i++ {
		id := "ch-" + strconv.Itoa(i)
		chRepo.Save(ctx, makeChannel(id, "C"+id, "ES", "News"))
		stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8"))
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
```

- [ ] **Step 2: Verlos fallar** → `Random undefined`, `IDs` desconocido.

- [ ] **Step 3: Implementar**

`IDs []string` en `ChannelFilter`, y en `buildChannelWhere`:

```go
	if len(f.IDs) > 0 {
		marcas := strings.Repeat("?,", len(f.IDs)-1) + "?"
		where = append(where, "id IN ("+marcas+")")
		for _, id := range f.IDs {
			args = append(args, id)
		}
	}
```

`Random` reutiliza el mismo helper y ordena al azar:

```go
// Random devuelve un canal al azar que case con el filtro. El sorteo va en SQL
// y no en el cliente: elegir entre las páginas ya cargadas sesgaría el
// resultado hacia el principio del catálogo.
func (r *SQLiteChannelRepository) Random(ctx context.Context, f ports.ChannelFilter) (domain.Channel, error) {
	f = f.Normalize()
	whereSQL, args := buildChannelWhere(f)

	q := "SELECT" + channelColumns + `,
		EXISTS(SELECT 1 FROM streams s WHERE s.channel_id = channels.id AND s.last_checked IS NOT NULL) AS any_checked,
		EXISTS(SELECT 1 FROM streams s WHERE s.channel_id = channels.id AND s.is_alive = 1) AS any_alive,
		(SELECT MIN(s.latency_ms) FROM streams s WHERE s.channel_id = channels.id AND s.is_alive = 1) AS best_latency
		FROM channels WHERE ` + whereSQL + " ORDER BY RANDOM() LIMIT 1"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return domain.Channel{}, fmt.Errorf("db.Random: %w", err)
	}
	defer rows.Close()

	canales, err := scanChannelsWithHealth(rows)
	if err != nil {
		return domain.Channel{}, fmt.Errorf("db.Random (scan): %w", err)
	}
	if len(canales) == 0 {
		return domain.Channel{}, fmt.Errorf("db.Random: ningún canal casa con el filtro")
	}
	return canales[0], nil
}
```

Handler `GetRandom` que arma el filtro igual que `GetChannels` (con
`AliveOnly: true` forzado) y devuelve 404 si no hay coincidencias. En
`GetChannels`, leer `ids` separados por coma y pasarlos al filtro.

- [ ] **Step 4: Verlos pasar, suite completa y commit**

```bash
go test -race -count=1 ./... && go vet ./... && gofmt -l .
git commit -am "gateway: canal aleatorio y filtro por ids para favoritos"
```

---

# FASE 2 — Los tres modales de filtro

### Task 4: Datos de país (nombres y banderas)

**Files:** `mobile/lib/domain/countries.dart`, `mobile/test/domain/countries_test.dart`

**Interfaces:** `nombrePais(String iso) -> String`, `banderaPais(String iso) -> String`.

- [ ] **Step 1: Test**

```dart
void main() {
  test('traduce los códigos ISO a nombre completo', () {
    expect(nombrePais('MX'), 'México');
    expect(nombrePais('US'), 'Estados Unidos');
    expect(nombrePais('GB'), 'Reino Unido');
    // Un código desconocido no debe romper la UI.
    expect(nombrePais('ZZ'), 'ZZ');
  });

  test('deriva la bandera de los indicadores regionales', () {
    expect(banderaPais('MX'), '🇲🇽');
    expect(banderaPais('ES'), '🇪🇸');
    expect(banderaPais('X'), '');
  });
}
```

- [ ] **Step 2: Verlo fallar** → no existe el archivo.

- [ ] **Step 3: Implementar**

Mapa `const Map<String, String>` con los ~250 códigos ISO 3166-1 alpha-2 y su
nombre en español, y:

```dart
/// Deriva la bandera del código ISO con los indicadores regionales Unicode.
/// Ojo: los lectores de pantalla los leen letra a letra, así que quien la pinte
/// debe acompañarla de un semanticLabel con el nombre real.
String banderaPais(String iso) {
  if (iso.length != 2) return '';
  final a = iso.toUpperCase().codeUnitAt(0) - 0x41 + 0x1F1E6;
  final b = iso.toUpperCase().codeUnitAt(1) - 0x41 + 0x1F1E6;
  return String.fromCharCode(a) + String.fromCharCode(b);
}

String nombrePais(String iso) => _nombres[iso.toUpperCase()] ?? iso;
```

- [ ] **Step 4: Verlo pasar y commit**

```bash
git commit -am "mobile: nombres y banderas de país desde el código ISO"
```

---

### Task 5: Proveedores de facetas

**Files:** `mobile/lib/data/repositories/facet_repository.dart`, `mobile/lib/presentation/providers/facet_provider.dart`, + tests

**Interfaces:** `Faceta({String valor, int count})`, `IFacetRepository.countries()`,
`.categories()`; `countriesProvider` y `categoriesProvider` como `FutureProvider`.

- [ ] **Step 1: Test del repositorio** con `MockClient` devolviendo
`[{"Valor":"US","Count":1935}]`, comprobando que parsea y que un 500 lanza `ApiError`.

- [ ] **Step 2: Verlo fallar.**

- [ ] **Step 3: Implementar** siguiendo el patrón de `ChannelRepository`
(`ApiConfig.baseUrl`, `.timeout`, `_get` con `ApiError.desde`).

- [ ] **Step 4: Verlo pasar y commit.**

---

### Task 6: Los tres botones y sus modales

**Files:** `mobile/lib/presentation/widgets/filter_bar.dart` (reescritura),
`filter_button.dart`, `country_picker.dart`, `resolution_picker.dart`,
`category_picker.dart`, `category_icons.dart`, + tests

**Interfaces:** `FilterButton({icon, label, value, onTap, onClear})`.

- [ ] **Step 1: Tests**

```dart
testWidgets('los tres botones se alinean en una línea', (tester) async {
  await _pump(tester, FakeRepo(total: 20));

  final pais = tester.getRect(find.byKey(const Key('filtro-pais')));
  final res = tester.getRect(find.byKey(const Key('filtro-resolucion')));
  final cat = tester.getRect(find.byKey(const Key('filtro-categoria')));

  // Misma línea: idéntico top.
  expect(res.top, pais.top);
  expect(cat.top, pais.top);
  // Y del mismo ancho, que es lo que pidió el diseño.
  expect(res.width, closeTo(pais.width, 1));
  expect(cat.width, closeTo(pais.width, 1));
});

testWidgets('el botón de país muestra bandera y nombre completo', (tester) async {
  final container = await _pump(tester, FakeRepo(total: 20));
  container.read(channelFilterProvider.notifier).state =
      const ChannelFilter(country: 'MX');
  await tester.pumpAndSettle();

  expect(find.textContaining('México'), findsOneWidget);
  expect(find.textContaining('🇲🇽'), findsOneWidget);
  // Nunca el código a secas.
  expect(find.text('MX'), findsNothing);
});

testWidgets('cada modal abre y aplica su valor', (tester) async {
  final container = await _pump(tester, FakeRepo(total: 20));

  await tester.tap(find.byKey(const Key('filtro-resolucion')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('4K'));
  await tester.pumpAndSettle();

  expect(container.read(channelFilterProvider).quality, '4k');
});
```

- [ ] **Step 2: Verlos fallar.**

- [ ] **Step 3: Implementar**

`FilterButton`: `Expanded` dentro de un `Row` para que los tres midan igual,
con borde hairline, icono, etiqueta neutra o valor activo, ámbar solo cuando hay
selección, y una `×` que limpia sin abrir el modal.

`country_picker.dart`: `showDialog` con `TextField` de búsqueda que casa por
nombre **y** por código, y una lista de `countriesProvider` con
`Text(banderaPais(c))` + `nombrePais(c)` + recuento, cada fila envuelta en
`Semantics(label: nombrePais(c))` porque el emoji se lee letra a letra.

`category_picker.dart`: igual, con `iconoCategoria(nombre)` de
`category_icons.dart`, un `Map<String, IconData>` de las 30 atómicas a iconos de
Material (`movie`, `sports_soccer`, `newspaper`, `child_care`, `music_note`,
`church`, `school`, `restaurant`, `directions_car`, `cloud`, `science`…) con
`Icons.tv` de reserva.

`resolution_picker.dart`: las cuatro opciones, sin búsqueda.

- [ ] **Step 4: Verlos pasar, `flutter analyze`, y verificación por mutación**
del test de alineación: dándole anchos distintos a los botones, debe fallar.

- [ ] **Step 5: Commit**

```bash
git commit -am "mobile: tres filtros por modal, alineados y con datos reales"
```

---

# FASE 3 — Rejilla

### Task 7: Tarjeta de canal y rejilla con toggle

**Files:** `mobile/lib/presentation/widgets/channel_card.dart`,
`channel_grid.dart`, `lib/presentation/providers/view_mode_provider.dart`,
`home_screen.dart`, + tests

- [ ] **Step 1: Tests**

```dart
testWidgets('la tarjeta pinta la inicial cuando no hay logo', (tester) async {
  // 1 de cada 6 canales no tiene logo: el marcador no es un detalle.
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(body: ChannelCard(channel: _sinLogo(), onTap: () {})),
  ));

  expect(find.text('B'), findsOneWidget); // inicial de "BBC One"
  expect(find.byType(Image), findsNothing);
});

testWidgets('el modo de vista persiste', (tester) async {
  SharedPreferences.setMockInitialValues({'view_mode': 'list'});
  final prefs = await SharedPreferences.getInstance();
  final c = ProviderContainer(
      overrides: [sharedPreferencesProvider.overrideWithValue(prefs)]);
  addTearDown(c.dispose);

  expect(c.read(viewModeProvider), ViewMode.list);
  c.read(viewModeProvider.notifier).toggle();
  expect(prefs.getString('view_mode'), 'grid');
});
```

- [ ] **Step 2: Verlos fallar.**

- [ ] **Step 3: Implementar**

`ChannelCard`: `AspectRatio` con el logo encuadrado sobre `surfaceInset`; si
`logoUrl` está vacío, la inicial en Space Grotesk sobre el hexágono de marca
(reutilizando el `_HexClipper` del wordmark, extraído a `korven_shapes.dart`).
Debajo, nombre a dos líneas y una línea mono con país · categoría · resolución.
Estrella arriba a la derecha. `cacheWidth` según el ancho de celda.

`ChannelGrid`: `GridView.builder` con
`SliverGridDelegateWithMaxCrossAxisExtent(maxCrossAxisExtent: 200)` para que las
columnas se adapten al ancho. Mismo `controller` y misma lógica de cola
(spinner / fila de error) que la lista.

`ViewModeNotifier` sobre `SharedPreferences`, clave `view_mode`.

Botón de toggle en `ConsoleBar` (`Icons.grid_view` / `Icons.view_list`).

- [ ] **Step 4: Verlos pasar y commit.**

---

# FASE 4 — Favoritos

### Task 8: Almacenamiento y estrella

**Files:** `mobile/lib/presentation/providers/favorites_provider.dart`,
`channel_card.dart`, `channel_row.dart`, + tests

- [ ] **Step 1: Tests**

```dart
test('los favoritos persisten', () async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  final c = ProviderContainer(
      overrides: [sharedPreferencesProvider.overrideWithValue(prefs)]);
  addTearDown(c.dispose);

  c.read(favoritesProvider.notifier).toggle('ch-1');
  expect(c.read(favoritesProvider), contains('ch-1'));
  expect(prefs.getStringList('favorites'), ['ch-1']);

  c.read(favoritesProvider.notifier).toggle('ch-1');
  expect(c.read(favoritesProvider), isEmpty);
});
```

- [ ] **Step 2-4:** implementar `FavoritesNotifier extends Notifier<Set<String>>`
sobre `SharedPreferences` (`getStringList`/`setStringList`), estrella en tarjeta
y fila (ámbar si es favorito, `borderStrong` si no), y commit.

---

### Task 9: Filtrar por favoritos

**Files:** `channel_filter.dart`, `channel_provider.dart`, `filter_bar.dart`, + tests

**Contexto:** el gateway no conoce los favoritos, así que filtrarlos no puede
resolverse paginando el catálogo — un favorito puede estar en la página 20. Con
el filtro activo la app pide exactamente esos IDs con el parámetro `ids`.

- [ ] **Step 1: Test**

```dart
test('el filtro de favoritos pide los ids, no pagina el catálogo', () async {
  final repo = FakeRepo(total: 1200);
  final container = await containerCon(repo);
  await container.read(channelListProvider.future);

  container.read(favoritesProvider.notifier).toggle('ch-900');
  container.read(channelFilterProvider.notifier).state =
      const ChannelFilter(onlyFavorites: true);
  await container.read(channelListProvider.future);

  // Un favorito de la página 2 tiene que aparecer sin haber scrolleado.
  expect(repo.lastFilter!.ids, contains('ch-900'));
});
```

- [ ] **Step 2-4:** `onlyFavorites` en `ChannelFilter`, `ids` en el repositorio
Dart, chip `★ favoritos` en la barra, y commit.

---

# FASE 5 — Canal aleatorio

### Task 10: Botón y navegación

**Files:** `mobile/lib/data/repositories/channel_repository.dart`,
`console_bar.dart`, `home_screen.dart`, + tests

- [ ] **Step 1: Test**

```dart
testWidgets('el aleatorio respeta los filtros activos', (tester) async {
  final repo = FakeRepo(total: 100);
  final container = await _pumpHome(tester, repo);
  container.read(channelFilterProvider.notifier).state =
      const ChannelFilter(country: 'MX');
  await tester.pumpAndSettle();

  await tester.tap(find.byTooltip('Canal aleatorio'));
  await tester.pumpAndSettle();

  expect(repo.lastRandomFilter!.country, 'MX');
});
```

- [ ] **Step 2-4:** `getRandomChannel(filter)` en el repositorio, botón
`Icons.casino` con tooltip `Canal aleatorio` en la barra que navega directo al
reproductor, y un `KorvenStateView` si el gateway devuelve 404 (ningún canal
casa). Commit.

---

# FASE 6 — Verificación end-to-end

### Task 11: Comprobar contra datos reales y fusionar

- [ ] **Step 1: Datos del gateway**

```bash
cd gateway && go build -o server ./cmd/server && ./server &
# La corrección de categorías, con el caso real
curl -s -D- -o /dev/null "localhost:8080/channels?category=Kids&alive=all" | grep -i x-total   # 337, no 253
curl -s -D- -o /dev/null "localhost:8080/channels?category=Movies&alive=all" | grep -i x-total # 655, no 573
# Facetas
curl -s localhost:8080/channels/categories | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d),'categorías'); print([x['Valor'] for x in d if ';' in x['Valor']] or 'ninguna compuesta')"
curl -s localhost:8080/channels/countries | python3 -c "import sys,json; print(len(json.load(sys.stdin)),'países')"
# Aleatorio: varía y respeta el filtro
for i in $(seq 1 8); do curl -s "localhost:8080/channels/random?country=MX" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['Name'],'|',d['CountryCode'],'| alive=',d['Alive'])"; done
```

Esperado: 30 categorías sin ninguna compuesta, 178 países, y ocho sorteos con
nombres distintos, todos `MX` y `alive=True`.

- [ ] **Step 2: A mano en la app**

Los tres modales abren y filtran; el de país busca por «Méx» y por «MX»; la
estrella persiste tras reiniciar; el toggle rejilla/lista persiste; la rejilla
se reflowea al estrechar la ventana; el aleatorio con México puesto abre un
canal mexicano.

- [ ] **Step 3: Merge**

CI verde, merge a `main` con `--no-ff`, borrar la rama.

## Fuera de alcance

Atajos de teclado, manejo global de errores, `autoDispose` de la lista
acumulada, pantalla de ajustes, y el resto de la Fase 8 (historial, retomar
reproducción, PiP).
