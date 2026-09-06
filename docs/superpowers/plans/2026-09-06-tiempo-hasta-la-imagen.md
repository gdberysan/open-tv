# Tiempo hasta la imagen — Plan de implementación

> **Para trabajadores agénticos:** SUB-SKILL OBLIGATORIA: usa
> `superpowers:subagent-driven-development` (recomendada) o
> `superpowers:executing-plans` para implementar tarea a tarea. Los pasos usan
> casillas (`- [ ]`).

**Goal:** Que el reproductor reporte el desenlace real de cada mirror (tiempo
hasta la imagen o fallo), que el gateway lo persista por mirror, que el
failover salte los mirrors que ya fallaron aquí (con histéresis y «Probar de
todos modos»), que los mirrors sin audio se releguen y se etiqueten, y que la
tarjeta diga «Imagen en 2,1 s» o «Sin imagen desde aquí».

**Architecture:** Bucle de verdad cliente→servidor. El reproductor ya mide
`msPrimerFrame` y la clase de fallo; un reporter best-effort los manda a
`POST /streams/desenlace` (mismo origen, pool de escritura). `streams` gana
`audio_ok` (de la PMT que ya se parsea), `imagen_ms`, `fallos_reales`,
`ultimo_desenlace_at`, `ultimo_motivo`. La regla «sin imagen» (2 fallos
reales en 24 h) es una función pura de `domain`, se evalúa en el servidor y
viaja decidida en `/channels/streams`. `GET /channels/imagen` da a la tarjeta
el dato por canal solo para los canales sintonizados. Ninguna sonda de
velocidad: descartada con medidas (spec §1).

**Tech Stack:** Go 1.2x + SQLite modernc (ya presentes), chi; Svelte 5 +
Vitest + Testing Library. hls.js sin cambios.

**Spec:** `docs/superpowers/specs/2026-09-06-tiempo-hasta-la-imagen-design.md`

## Global Constraints

- Código, comentarios, commits y UI en **ESPAÑOL**; inglés con paridad de
  claves (`web/src/i18n/i18n.test.ts`).
- **Ninguna dependencia nueva.** **`mobile/` CERO diffs.** `/channels` y
  `/sources` NO cambian (15 claves: `internal/domain/channel_test.go`).
- **NUNCA `git add -A`.** `git config user.email` = `gdberysan@gmail.com`.
  Trailer: `Co-Authored-By: <modelo en uso> <noreply@anthropic.com>`.
- Gates Go por tarea: `gofmt -l .` · `go vet ./...` · `go build ./...` ·
  `go test -race -count=1 ./...` · `golangci-lint run ./...` ·
  `go run ./tools/scrubcheck`. Solo `//nolint:<linter> // razón`, nunca `#nosec`.
- Gates Web por tarea: `cd web && npm run check && npm test && npm run build`
  (bundle propio ≤ 80 KB gzip). No commitear `internal/ui/dist/`.
- Constantes de la spec, exactas: `UmbralFallosReales = 2`,
  `VentanaFallosReales = 24 * time.Hour`. Fallo REAL = motivo ∈
  {`desconocido`, `inestable`, `caido`, `caducado`}. Tipos de audio de la PMT:
  `0x03, 0x04, 0x0f, 0x11, 0x81, 0x87`.
- No se reporta un intento si: motor forzado (cast), pestaña oculta durante el
  intento, `navigator.onLine === false`, o `via === 'ninguna'`.
- Rama: `feat/tiempo-hasta-la-imagen` desde `spec/tiempo-hasta-la-imagen`.
- La DB real del gateway de dev es `DB_PATH=<repo>/.devdata/iptv.db`
  (plist de launchd), NO la de Application Support.

---

## Estructura de ficheros

**Crear:** `internal/domain/imagen.go` (+ test) · `internal/api/handlers/desenlace_handler.go` (+ test) · `web/src/estado/desenlaces.ts` (+ test) · `web/src/estado/imagen.ts` (+ test) · `docs/superpowers/evidencia/2026-09-06-sin-imagen.png`.

**Modificar:** `internal/domain/codec.go` (+ test) · `internal/ports/stream_repository.go` · `internal/adapters/db/schema.sql`, `db.go`, `stream_repository.go` (+ test) · `internal/adapters/validator/models.go`, `sonda.go`, `checker.go`, `worker.go` (+ tests) · `internal/api/handlers/channel_handler.go` (+ test), `stats_handler.go`, `catalogo_stats_test.go` · `internal/api/router.go` (+ test) · `cmd/open-tv/main.go` · fakes: `internal/adapters/validator/worker_test.go`, `internal/api/handlers/channel_handler_test.go`, `internal/api/router_test.go`, `internal/services/syncer_test.go` · `web/src/datos/catalogo.ts`, `http.ts` (+ test) · `web/src/reproductor/failover.ts` · `web/src/i18n/es.ts`, `en.ts` · `web/src/componentes/Reproductor.svelte` (+ test), `SenalCanal.svelte` (+ test), `ListaCanalesLateral.svelte`, `RejillaCanales.svelte`, `TarjetaCanal.svelte` · `web/src/App.svelte`.

---

### Task 0: Rama

- [ ] `cd "$(git rev-parse --show-toplevel)" && git checkout spec/tiempo-hasta-la-imagen && git checkout -b feat/tiempo-hasta-la-imagen && git config user.email`

---

### Task 1: Dominio — audio de la PMT, regla «sin imagen», motivos reales

**Files:**
- Modify: `internal/domain/codec.go`, `internal/domain/codec_test.go`
- Create: `internal/domain/imagen.go`, `internal/domain/imagen_test.go`

**Interfaces:**
- Produces:
  - `type AudioSupport int` — `AudioUnknown` (cero), `AudioNo`, `AudioOK`.
  - `func ClassifyAudio(streams []StreamTS) AudioSupport`: `AudioOK` si algún tipo ∈ {0x03,0x04,0x0f,0x11,0x81,0x87}; `AudioNo` si hay streams y ninguno es audio; `AudioUnknown` si la lista está vacía.
  - `const UmbralFallosReales = 2`, `const VentanaFallosReales = 24 * time.Hour`.
  - `func SinImagen(fallosReales int, ultimoDesenlace, ahora time.Time) bool` — `fallosReales >= UmbralFallosReales && !ultimoDesenlace.IsZero() && ahora.Sub(ultimoDesenlace) <= VentanaFallosReales`.
  - `func MotivoEsFalloReal(motivo string) bool` — true para `desconocido`, `inestable`, `caido`, `caducado`; false para todo lo demás (incluido `""`).

- [ ] **Paso 1: Tests (fallan)**

`codec_test.go`, añadir:

```go
func TestClassifyAudio(t *testing.T) {
	casos := []struct {
		nombre  string
		streams []domain.StreamTS
		quiero  domain.AudioSupport
	}{
		{"H.264 + AAC", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x0f}}, domain.AudioOK},
		{"MPEG-2 + MP2", []domain.StreamTS{{Tipo: 0x02}, {Tipo: 0x03}}, domain.AudioOK},
		{"H.264 + AC-3", []domain.StreamTS{{Tipo: 0x1b}, {Tipo: 0x81}}, domain.AudioOK},
		{"H.264 solo (AMC mirror 2)", []domain.StreamTS{{Tipo: 0x1b}}, domain.AudioNo},
		{"solo audio", []domain.StreamTS{{Tipo: 0x0f}}, domain.AudioOK},
		{"vacío", nil, domain.AudioUnknown},
	}
	for _, c := range casos {
		if got := domain.ClassifyAudio(c.streams); got != c.quiero {
			t.Errorf("%s: ClassifyAudio = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}
```

`imagen_test.go`:

```go
package domain_test

import (
	"testing"
	"time"

	"github.com/gdberysan/open-tv/internal/domain"
)

func TestSinImagen(t *testing.T) {
	ahora := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre string
		fallos int
		ultimo time.Time
		quiero bool
	}{
		{"nunca falló", 0, time.Time{}, false},
		{"un fallo reciente", 1, ahora.Add(-time.Hour), false},
		{"dos fallos recientes", 2, ahora.Add(-time.Hour), true},
		{"tres fallos recientes", 3, ahora.Add(-time.Minute), true},
		{"dos fallos hace exactamente 24 h: aún dentro", 2, ahora.Add(-domain.VentanaFallosReales), true},
		{"dos fallos hace 24 h y un segundo: otra oportunidad", 2, ahora.Add(-domain.VentanaFallosReales - time.Second), false},
		{"fallos sin fecha (fila corrupta): no se salta", 5, time.Time{}, false},
	}
	for _, c := range casos {
		if got := domain.SinImagen(c.fallos, c.ultimo, ahora); got != c.quiero {
			t.Errorf("%s: SinImagen = %v, quiero %v", c.nombre, got, c.quiero)
		}
	}
}

func TestMotivoEsFalloReal(t *testing.T) {
	reales := []string{"desconocido", "inestable", "caido", "caducado"}
	noReales := []string{"geo", "formato", "codec", "sinImagen", "", "otra"}
	for _, m := range reales {
		if !domain.MotivoEsFalloReal(m) {
			t.Errorf("%q debe ser fallo real", m)
		}
	}
	for _, m := range noReales {
		if domain.MotivoEsFalloReal(m) {
			t.Errorf("%q NO debe ser fallo real", m)
		}
	}
}
```

- [ ] **Paso 2:** `go test ./internal/domain/ -run 'ClassifyAudio|SinImagen|MotivoEsFalloReal'` → error de compilación.

- [ ] **Paso 3: Implementar**

En `codec.go`, tras `ClassifyCodecs`:

```go
// AudioSupport dice si la PMT trae alguna pista de audio. AudioNo no es un
// defecto de códec: es un origen que emite vídeo mudo (AMC mirror 2). El
// cliente lo relega y lo etiqueta («Sin audio en este origen»), no lo salta.
type AudioSupport int

const (
	AudioUnknown AudioSupport = iota
	AudioNo
	AudioOK
)

// ClassifyAudio: basta un stream de audio conocido. Sin streams no se juzga.
func ClassifyAudio(streams []StreamTS) AudioSupport {
	if len(streams) == 0 {
		return AudioUnknown
	}
	for _, s := range streams {
		switch s.Tipo {
		case tsAudioMPEG1, tsAudioMPEG2, tsAudioAAC, tsAudioLATM, tsAudioAC3, tsAudioEAC3:
			return AudioOK
		}
	}
	return AudioNo
}
```

`imagen.go`:

```go
package domain

import "time"

// Regla «sin imagen desde aquí» (spec tiempo-hasta-la-imagen §3.3): misma
// filosofía que la histéresis del health-check (3 chequeos para declarar
// muerto). Dos fallos REALES consecutivos dentro de 24 h y el mirror se
// salta; pasada la ventana vuelve a tener una oportunidad.
const (
	UmbralFallosReales  = 2
	VentanaFallosReales = 24 * time.Hour
)

// SinImagen se evalúa en el SERVIDOR y viaja decidida en el cable: el
// cliente nunca rederiva la regla. Una fila sin fecha de desenlace nunca se
// salta, tenga los fallos que tenga: sin fecha no hay ventana.
func SinImagen(fallosReales int, ultimoDesenlace, ahora time.Time) bool {
	if fallosReales < UmbralFallosReales || ultimoDesenlace.IsZero() {
		return false
	}
	return ahora.Sub(ultimoDesenlace) <= VentanaFallosReales
}

// MotivoEsFalloReal separa «el origen no dio imagen desde aquí» de lo que no
// es culpa del origen o ya tiene su propio veredicto: geo (es dónde estamos),
// formato y codec (veredictos propios), sinImagen (no hubo intento).
func MotivoEsFalloReal(motivo string) bool {
	switch motivo {
	case "desconocido", "inestable", "caido", "caducado":
		return true
	}
	return false
}
```

- [ ] **Paso 4:** gates Go. **Paso 5:** commit `feat(domain): audio de la PMT, regla «sin imagen» y motivos de fallo real` con `internal/domain/codec.go internal/domain/codec_test.go internal/domain/imagen.go internal/domain/imagen_test.go`.

---

### Task 2: Persistencia — columnas, `RegistrarDesenlace`, `ImagenPorCanal`, orden

**Files:**
- Modify: `internal/ports/stream_repository.go`
- Modify: `internal/adapters/db/schema.sql` (bloque `streams`), `internal/adapters/db/db.go` (`alterMigrations`), `internal/adapters/db/stream_repository.go`
- Test: `internal/adapters/db/stream_repository_test.go`
- Modify (fakes, solo para compilar): `internal/adapters/validator/worker_test.go`, `internal/api/handlers/channel_handler_test.go`, `internal/api/router_test.go`, `internal/services/syncer_test.go`

**Interfaces:**
- Consumes: `domain.AudioSupport`, `domain.SinImagen`, `domain.MotivoEsFalloReal` (Task 1).
- Produces:
  - `ports.StreamHealth` gana `Audio domain.AudioSupport` (Unknown nunca pisa).
  - `ports.MirrorHealth` gana `Audio domain.AudioSupport`, `ImagenMs int64`, `FallosReales int`, `UltimoDesenlace time.Time`, `UltimoMotivo string`.
  - `type ports.DesenlaceMirror struct { Resultado string; Motivo string; MsPrimerFrame int64 }` (`Resultado` ∈ `iniciado|fallo`).
  - `type ports.ImagenCanal struct { ChannelID domain.ChannelID; ImagenMs int64; SinImagen bool }`.
  - `type ports.RegistradorDesenlaces interface { RegistrarDesenlace(ctx context.Context, url string, d DesenlaceMirror) error }` — lo implementa `*SQLiteStreamRepository`; se inyecta al router por `Options` con el repo del pool de ESCRITURA.
  - `StreamRepository` gana `ImagenPorCanal(ctx context.Context, ahora time.Time) ([]ImagenCanal, error)` (lectura).
  - Columnas: `audio_ok INTEGER CHECK (audio_ok IN (0,1))`, `imagen_ms INTEGER NOT NULL DEFAULT 0`, `fallos_reales INTEGER NOT NULL DEFAULT 0`, `ultimo_desenlace_at INTEGER NOT NULL DEFAULT 0`, `ultimo_motivo TEXT NOT NULL DEFAULT ''`.

- [ ] **Paso 1: Tests (fallan)** — añadir a `stream_repository_test.go` (usa `repoConCanal`: canal `ch-1`, streams `s1`=`http://a.example/1.m3u8`, `s2`=`http://b.example/1.m3u8`):

```go
func TestRegistrarDesenlaceExitoYFallos(t *testing.T) {
	ctx := context.Background()
	stRepo, sqlDB := repoConCanal(t)
	url := "http://a.example/1.m3u8"

	leer := func() (imagen, fallos, ultimo int64, motivo string) {
		t.Helper()
		if err := sqlDB.QueryRow(`SELECT imagen_ms, fallos_reales, ultimo_desenlace_at, ultimo_motivo FROM streams WHERE id = 's1'`).
			Scan(&imagen, &fallos, &ultimo, &motivo); err != nil {
			t.Fatal(err)
		}
		return
	}

	// Fallo real, dos veces.
	for i := 0; i < 2; i++ {
		if err := stRepo.RegistrarDesenlace(ctx, url, ports.DesenlaceMirror{Resultado: "fallo", Motivo: "desconocido"}); err != nil {
			t.Fatalf("RegistrarDesenlace fallo %d: %v", i, err)
		}
	}
	imagen, fallos, ultimo, motivo := leer()
	if imagen != 0 || fallos != 2 || ultimo == 0 || motivo != "desconocido" {
		t.Errorf("tras 2 fallos reales: imagen=%d fallos=%d ultimo=%d motivo=%q", imagen, fallos, ultimo, motivo)
	}

	// Fallo NO real: no toca fallos_reales, sí el motivo.
	if err := stRepo.RegistrarDesenlace(ctx, url, ports.DesenlaceMirror{Resultado: "fallo", Motivo: "geo"}); err != nil {
		t.Fatal(err)
	}
	if _, fallos, _, motivo = leer(); fallos != 2 || motivo != "geo" {
		t.Errorf("fallo geo: fallos=%d motivo=%q", fallos, motivo)
	}

	// Éxito: resetea y guarda la imagen.
	if err := stRepo.RegistrarDesenlace(ctx, url, ports.DesenlaceMirror{Resultado: "iniciado", MsPrimerFrame: 2100}); err != nil {
		t.Fatal(err)
	}
	if imagen, fallos, _, motivo = leer(); imagen != 2100 || fallos != 0 || motivo != "" {
		t.Errorf("éxito: imagen=%d fallos=%d motivo=%q", imagen, fallos, motivo)
	}

	// Un fallo posterior NO borra la última imagen vista.
	if err := stRepo.RegistrarDesenlace(ctx, url, ports.DesenlaceMirror{Resultado: "fallo", Motivo: "inestable"}); err != nil {
		t.Fatal(err)
	}
	if imagen, fallos, _, _ = leer(); imagen != 2100 || fallos != 1 {
		t.Errorf("fallo tras éxito: imagen=%d fallos=%d", imagen, fallos)
	}
}

func TestRegistrarDesenlaceURLDesconocidaNoFalla(t *testing.T) {
	stRepo, _ := repoConCanal(t)
	if err := stRepo.RegistrarDesenlace(context.Background(), "http://nadie.example/x.m3u8", ports.DesenlaceMirror{Resultado: "fallo", Motivo: "caido"}); err != nil {
		t.Errorf("URL desconocida debe ser no-op: %v", err)
	}
}

func TestRegistrarDesenlaceAplicaATodasLasFilasConEsaURL(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")
	seedChannel(t, chRepo, "ch-2")
	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("s1", "ch-1", "http://c.example/x.m3u8"),
		makeStream("s2", "ch-2", "http://c.example/x.m3u8"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := stRepo.RegistrarDesenlace(ctx, "http://c.example/x.m3u8", ports.DesenlaceMirror{Resultado: "iniciado", MsPrimerFrame: 900}); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []domain.ChannelID{"ch-1", "ch-2"} {
		m, err := stRepo.FindMirrorsByChannelID(ctx, ch)
		if err != nil || len(m) != 1 || m[0].ImagenMs != 900 {
			t.Errorf("%s: %+v (%v)", ch, m, err)
		}
	}
}

func TestMarkBatchPersisteAudio(t *testing.T) {
	ctx := context.Background()
	stRepo, sqlDB := repoConCanal(t)
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, Codec: domain.CodecOK, Codecs: "h264", CodecSondeado: true, Audio: domain.AudioNo},
		{StreamID: "s2", IsAlive: true, Codec: domain.CodecOK, Codecs: "h264,aac", CodecSondeado: true, Audio: domain.AudioOK},
	}); err != nil {
		t.Fatal(err)
	}
	var a1, a2 sql.NullInt64
	_ = sqlDB.QueryRow(`SELECT audio_ok FROM streams WHERE id='s1'`).Scan(&a1)
	_ = sqlDB.QueryRow(`SELECT audio_ok FROM streams WHERE id='s2'`).Scan(&a2)
	if !a1.Valid || a1.Int64 != 0 || !a2.Valid || a2.Int64 != 1 {
		t.Errorf("audio_ok s1=%v s2=%v", a1, a2)
	}
	// Unknown no pisa.
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{{StreamID: "s1", IsAlive: true, CodecSondeado: true}}); err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.QueryRow(`SELECT audio_ok FROM streams WHERE id='s1'`).Scan(&a1)
	if !a1.Valid || a1.Int64 != 0 {
		t.Errorf("Unknown pisó audio_ok: %v", a1)
	}
}

// Orden: vivos; con audio antes que sin audio; imagen conocida ascendente
// antes que desconocida; luego latencia.
func TestFindMirrorsByChannelIDOrdenaPorAudioImagenYLatencia(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")
	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("rapido-mudo", "ch-1", "http://o/1.m3u8"),
		makeStream("lento-con-imagen", "ch-1", "http://o/2.m3u8"),
		makeStream("medio-sin-imagen", "ch-1", "http://o/3.m3u8"),
		makeStream("muerto", "ch-1", "http://o/4.m3u8"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "rapido-mudo", IsAlive: true, LatencyMs: 50, Codec: domain.CodecOK, CodecSondeado: true, Audio: domain.AudioNo},
		{StreamID: "lento-con-imagen", IsAlive: true, LatencyMs: 900, Codec: domain.CodecOK, CodecSondeado: true, Audio: domain.AudioOK},
		{StreamID: "medio-sin-imagen", IsAlive: true, LatencyMs: 300, Codec: domain.CodecOK, CodecSondeado: true, Audio: domain.AudioOK},
		{StreamID: "muerto", IsAlive: false},
	}); err != nil {
		t.Fatal(err)
	}
	if err := stRepo.RegistrarDesenlace(ctx, "http://o/2.m3u8", ports.DesenlaceMirror{Resultado: "iniciado", MsPrimerFrame: 1500}); err != nil {
		t.Fatal(err)
	}
	mirrors, err := stRepo.FindMirrorsByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatal(err)
	}
	var urls []string
	for _, m := range mirrors {
		urls = append(urls, m.URL)
	}
	quiero := []string{"http://o/2.m3u8", "http://o/3.m3u8", "http://o/1.m3u8", "http://o/4.m3u8"}
	if fmt.Sprint(urls) != fmt.Sprint(quiero) {
		t.Errorf("orden = %v, quiero %v", urls, quiero)
	}
	if mirrors[0].Audio != domain.AudioOK || mirrors[2].Audio != domain.AudioNo || mirrors[0].ImagenMs != 1500 {
		t.Errorf("campos: %+v", mirrors[:3])
	}
}

func TestImagenPorCanal(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	for _, ch := range []string{"visto", "sin-imagen", "por-codec", "nunca"} {
		seedChannel(t, chRepo, ch)
	}
	if err := stRepo.SaveBatch(ctx, []domain.Stream{
		makeStream("v1", "visto", "http://v/1.m3u8"), makeStream("v2", "visto", "http://v/2.m3u8"),
		makeStream("f1", "sin-imagen", "http://f/1.m3u8"), makeStream("f2", "sin-imagen", "http://f/2.m3u8"),
		makeStream("c1", "por-codec", "http://c/1.m3u8"),
		makeStream("n1", "nunca", "http://n/1.m3u8"),
	}); err != nil {
		t.Fatal(err)
	}
	vivos := []ports.StreamHealth{}
	for _, id := range []string{"v1", "v2", "f1", "f2", "c1", "n1"} {
		vivos = append(vivos, ports.StreamHealth{StreamID: id, IsAlive: true, LatencyMs: 100})
	}
	vivos[4].Codec, vivos[4].CodecSondeado = domain.CodecNo, true // c1: MPEG-2
	if err := stRepo.MarkBatch(ctx, vivos); err != nil {
		t.Fatal(err)
	}
	ok := func(url string, d ports.DesenlaceMirror) {
		t.Helper()
		if err := stRepo.RegistrarDesenlace(ctx, url, d); err != nil {
			t.Fatal(err)
		}
	}
	ok("http://v/1.m3u8", ports.DesenlaceMirror{Resultado: "iniciado", MsPrimerFrame: 3000})
	ok("http://v/2.m3u8", ports.DesenlaceMirror{Resultado: "iniciado", MsPrimerFrame: 1200})
	for _, u := range []string{"http://f/1.m3u8", "http://f/2.m3u8"} {
		ok(u, ports.DesenlaceMirror{Resultado: "fallo", Motivo: "desconocido"})
		ok(u, ports.DesenlaceMirror{Resultado: "fallo", Motivo: "inestable"})
	}
	// por-codec: un fallo real sobre un mirror que además es CodecNo — el canal
	// entero está saltado (por códec), aunque no llegue al umbral de fallos.
	ok("http://c/1.m3u8", ports.DesenlaceMirror{Resultado: "fallo", Motivo: "desconocido"})

	got, err := stRepo.ImagenPorCanal(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	porCanal := map[domain.ChannelID]ports.ImagenCanal{}
	for _, ic := range got {
		porCanal[ic.ChannelID] = ic
	}
	if _, hay := porCanal["nunca"]; hay {
		t.Error("un canal sin desenlaces no debe aparecer")
	}
	if v := porCanal["visto"]; v.ImagenMs != 1200 || v.SinImagen {
		t.Errorf("visto: %+v (quiero el menor imagen_ms, 1200, y sin_imagen=false)", v)
	}
	if f := porCanal["sin-imagen"]; !f.SinImagen || f.ImagenMs != 0 {
		t.Errorf("sin-imagen: %+v", f)
	}
	if c := porCanal["por-codec"]; !c.SinImagen {
		t.Errorf("por-codec: %+v (todos sus mirrors vivos están saltados por códec)", c)
	}
}
```

Añade `"time"` y `"fmt"` a los imports del test si faltan.

- [ ] **Paso 2:** `go test ./internal/adapters/db/ -run 'Desenlace|Audio|Imagen|OrdenaPorAudio'` → error de compilación.

- [ ] **Paso 3: Ports**

```go
// DesenlaceMirror es lo que el reproductor cuenta de un intento REAL sobre un
// mirror (spec tiempo-hasta-la-imagen §3.1). Resultado: "iniciado" | "fallo".
type DesenlaceMirror struct {
	Resultado     string
	Motivo        string
	MsPrimerFrame int64
}

// ImagenCanal es el resumen por canal para la tarjeta (GET /channels/imagen):
// solo existe para canales con algún desenlace registrado.
type ImagenCanal struct {
	ChannelID domain.ChannelID
	ImagenMs  int64 // menor imagen_ms > 0 entre sus mirrors vivos; 0 = ninguno
	SinImagen bool  // TODOS sus mirrors vivos están saltados (códec o §3.3)
}

// RegistradorDesenlaces es el puerto de ESCRITURA del bucle de verdad. Va
// aparte de StreamRepository porque el router recibe el repo de solo lectura;
// cmd/open-tv/main.go inyecta el del pool de escritura por api.Options.
type RegistradorDesenlaces interface {
	// RegistrarDesenlace aplica el desenlace a TODAS las filas con esa URL.
	// URL desconocida = no-op sin error (un mirror podado no rompe nada).
	RegistrarDesenlace(ctx context.Context, url string, d DesenlaceMirror) error
}
```

`StreamHealth` gana `Audio domain.AudioSupport` (comentario: sale de la misma PMT que Codec; Unknown no pisa). `MirrorHealth` gana:

```go
	Audio           domain.AudioSupport
	ImagenMs        int64     // último tiempo real hasta la imagen; 0 = nunca
	FallosReales    int
	UltimoDesenlace time.Time // cero = nunca
	UltimoMotivo    string
```

`StreamRepository` gana `ImagenPorCanal(ctx context.Context, ahora time.Time) ([]ImagenCanal, error)`.

- [ ] **Paso 4: Esquema + migración** (`schema.sql` tras `codec_checked_at`; `alterMigrations` cinco `ALTER TABLE streams ADD COLUMN …` con los mismos tipos/defaults; comentario en español citando la spec).

- [ ] **Paso 5: Repositorio**

`MarkBatch`, sentencia de vivos: añade `audio_ok = COALESCE(?, audio_ok)` tras `codecs` y el argumento `argAudioOK(res.Audio)` (mismo patrón que `argCodecOK`).

`RegistrarDesenlace`:

```go
func (r *SQLiteStreamRepository) RegistrarDesenlace(ctx context.Context, url string, d ports.DesenlaceMirror) error {
	now := time.Now().Unix()
	var q string
	var args []any
	switch {
	case d.Resultado == "iniciado":
		q = `UPDATE streams SET imagen_ms = ?, fallos_reales = 0, ultimo_desenlace_at = ?, ultimo_motivo = '', updated_at = ? WHERE url = ?`
		args = []any{d.MsPrimerFrame, now, now, url}
	case domain.MotivoEsFalloReal(d.Motivo):
		q = `UPDATE streams SET fallos_reales = fallos_reales + 1, ultimo_desenlace_at = ?, ultimo_motivo = ?, updated_at = ? WHERE url = ?`
		args = []any{now, d.Motivo, now, url}
	default:
		// geo/formato/codec: se anota el motivo para stats, no cuenta como
		// fallo del origen.
		q = `UPDATE streams SET ultimo_desenlace_at = ?, ultimo_motivo = ?, updated_at = ? WHERE url = ?`
		args = []any{now, d.Motivo, now, url}
	}
	if _, err := r.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("db.Stream.RegistrarDesenlace (%s): %w", d.Resultado, err)
	}
	return nil
}
```

`FindMirrorsByChannelID`: SELECT añade `audio_ok, imagen_ms, fallos_reales, ultimo_desenlace_at, ultimo_motivo`; ORDER BY:

```sql
ORDER BY is_alive DESC,
         CASE WHEN audio_ok = 0 THEN 1 ELSE 0 END,
         CASE WHEN imagen_ms > 0 THEN 0 ELSE 1 END,
         imagen_ms ASC,
         CASE WHEN latency_ms IS NULL THEN 1 ELSE 0 END,
         latency_ms ASC
```

Scan con `audioOK sql.NullInt64`, `ultimo int64` → `time.Unix` si ≠ 0; mapea audio tri-estado como codec.

`ImagenPorCanal` (agrega en Go, con la regla de dominio):

```go
func (r *SQLiteStreamRepository) ImagenPorCanal(ctx context.Context, ahora time.Time) ([]ports.ImagenCanal, error) {
	const q = `SELECT channel_id, is_alive, codec_ok, imagen_ms, fallos_reales, ultimo_desenlace_at
	           FROM streams
	           WHERE channel_id IN (SELECT DISTINCT channel_id FROM streams WHERE ultimo_desenlace_at > 0)
	           ORDER BY channel_id`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db.Stream.ImagenPorCanal: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type acum struct {
		imagen   int64
		vivos    int
		saltados int
	}
	orden := []domain.ChannelID{}
	porCanal := map[domain.ChannelID]*acum{}
	for rows.Next() {
		var (
			ch           string
			alive        int
			codecOK      sql.NullInt64
			imagen       int64
			fallos       int
			ultimo       int64
		)
		if err := rows.Scan(&ch, &alive, &codecOK, &imagen, &fallos, &ultimo); err != nil {
			return nil, fmt.Errorf("db.Stream.ImagenPorCanal (scan): %w", err)
		}
		id := domain.ChannelID(ch)
		a, ya := porCanal[id]
		if !ya {
			a = &acum{}
			porCanal[id] = a
			orden = append(orden, id)
		}
		if alive != 1 {
			continue
		}
		a.vivos++
		var ultimoT time.Time
		if ultimo != 0 {
			ultimoT = time.Unix(ultimo, 0)
		}
		porCodec := codecOK.Valid && codecOK.Int64 == 0
		if porCodec || domain.SinImagen(fallos, ultimoT, ahora) {
			a.saltados++
		} else if imagen > 0 && (a.imagen == 0 || imagen < a.imagen) {
			a.imagen = imagen
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db.Stream.ImagenPorCanal (rows.Err): %w", err)
	}
	out := make([]ports.ImagenCanal, 0, len(orden))
	for _, id := range orden {
		a := porCanal[id]
		out = append(out, ports.ImagenCanal{ChannelID: id, ImagenMs: a.imagen, SinImagen: a.vivos > 0 && a.saltados == a.vivos})
	}
	return out, nil
}
```

(Nota: en `TestImagenPorCanal`, «visto» tiene v1=3000 y v2=1200 → 1200; «sin-imagen» dos mirrors con 2 fallos cada uno → saltados==vivos; «por-codec» → saltado por códec.)

- [ ] **Paso 6: Fakes** — a cada implementación de `ports.StreamRepository` en tests (`fakeStreamRepo` en `validator/worker_test.go`, `mockStreamRepo` en `handlers/channel_handler_test.go`, `streamsVacio` en `api/router_test.go`, el fake de `services/syncer_test.go`) añade:

```go
func (…) ImagenPorCanal(context.Context, time.Time) ([]ports.ImagenCanal, error) { return nil, nil }
```

(`mockStreamRepo` guarda además un campo `imagen []ports.ImagenCanal` y lo devuelve, para la Task 4.)

- [ ] **Paso 7:** gates Go (todo el repo). **Paso 8:** commit `feat(db): desenlaces reales por mirror, audio_ok y orden por imagen` con ports, schema, db.go, stream_repository.go (+test) y los cuatro ficheros de fakes.

---

### Task 3: La sonda también decide el audio

**Files:**
- Modify: `internal/adapters/validator/models.go` (`StreamResult` gana `Audio domain.AudioSupport`), `sonda.go`, `checker.go:122-123`, `worker.go` (mapeo)
- Test: `internal/adapters/validator/sonda_test.go`, `worker_test.go`

**Interfaces:**
- Consumes: `domain.ClassifyAudio` (Task 1), `ports.StreamHealth.Audio` (Task 2).
- Produces: `sondearCodecs` devuelve `(domain.CodecSupport, domain.AudioSupport, string)`; `StreamResult.Audio`; el worker copia `Audio` a `StreamHealth`.

- [ ] **Paso 1: Tests (fallan)** — en `sonda_test.go`, `TestSondaMasterMediaSegmentoClasificaMPEG2` asevera además `res.Audio == domain.AudioOK`; nuevo test:

```go
func TestSondaH264SinAudioDaAudioNo(t *testing.T) {
	o := nuevoOrigen(t)
	o.segmento = segmentoTS(0x1b)
	res := checkerDeTest().CheckTarea(context.Background(), validator.TareaCheck{URL: o.srv.URL + "/media.m3u8", CodecCaducado: true})
	if res.Codec != domain.CodecOK || res.Audio != domain.AudioNo {
		t.Errorf("codec=%v audio=%v, quiero OK y AudioNo", res.Codec, res.Audio)
	}
}
```

y en `TestSondaFMP4QuedaDesconocidoPeroSondeado`: `res.Audio == domain.AudioUnknown`. En `worker_test.go`, `TestWorker_SondeaSoloLosCaducados` asevera `s.Audio == domain.AudioOK` para los sondeados (el prefijo de test lleva MP2).

- [ ] **Paso 2:** ver el fallo. **Paso 3:** implementar: todos los `return domain.CodecUnknown, ""` de `sondearCodecs` pasan a `return domain.CodecUnknown, domain.AudioUnknown, ""`; el final `return domain.ClassifyCodecs(streams), domain.ClassifyAudio(streams), domain.NombreCodecs(streams)`; `checker.go`: `res.Codec, res.Audio, res.Codecs = c.sondearCodecs(...)`; `worker.go`: `Audio: res.Audio` en el `StreamHealth`.

- [ ] **Paso 4:** gates Go. **Paso 5:** commit `feat(validator): la sonda anota si la PMT trae audio`.

---

### Task 4: API — `/channels/streams`, `POST /streams/desenlace`, `GET /channels/imagen`, `/stats`

**Files:**
- Modify: `internal/api/handlers/channel_handler.go` (+ `channel_handler_test.go`), `internal/api/handlers/stats_handler.go` (+ `catalogo_stats_test.go`)
- Create: `internal/api/handlers/desenlace_handler.go`, `internal/api/handlers/desenlace_handler_test.go`
- Modify: `internal/api/router.go` (`Options.Desenlaces`, rutas), `internal/api/router_test.go`, `cmd/open-tv/main.go`

**Interfaces:**
- Consumes: `ports.RegistradorDesenlaces`, `ports.DesenlaceMirror`, `ports.ImagenCanal`, `MirrorHealth` nuevos campos (Task 2); `domain.SinImagen`.
- Produces (cable):
  - `/channels/streams`: `audio_ok` (`*bool`), `imagen_ms` (int64), `sin_imagen` (bool), `ultimo_fallo_hace_s` (int64: segundos desde `UltimoDesenlace` si `FallosReales > 0`, si no 0).
  - `POST /streams/desenlace` `{url, resultado, motivo, ms_primer_frame}` → 204; 400 body inválido/`url` o `resultado` vacíos/`resultado` ∉ {iniciado,fallo}/cuerpo > 4 KB; 503 si no hay registrador.
  - `GET /channels/imagen` → `{"<id>": {"imagen_ms": n, "sin_imagen": b}}`.
  - `/stats.catalogo`: `sin_imagen` (mirrors con `SinImagen` verdadera ahora) e `imagen_p50_ms` (mediana de `imagen_ms > 0`, 0 si no hay).
  - `api.Options.Desenlaces ports.RegistradorDesenlaces`.

- [ ] **Paso 1: Tests (fallan)**

`desenlace_handler_test.go` (paquete `handlers`, con un `registradorFalso` que guarda `(url, d)` y puede devolver error):

```go
package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/ports"
)

type registradorFalso struct {
	urls []string
	des  []ports.DesenlaceMirror
}

func (r *registradorFalso) RegistrarDesenlace(_ context.Context, url string, d ports.DesenlaceMirror) error {
	r.urls = append(r.urls, url)
	r.des = append(r.des, d)
	return nil
}

func postDesenlace(h *DesenlaceHandler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/streams/desenlace", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.Post(rec, req)
	return rec
}

func TestDesenlaceHandlerRegistra(t *testing.T) {
	reg := &registradorFalso{}
	h := NewDesenlaceHandler(slog.New(slog.DiscardHandler), reg)
	rec := postDesenlace(h, `{"url":"http://o/x.m3u8","resultado":"iniciado","motivo":"","ms_primer_frame":2100.4}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	if len(reg.urls) != 1 || reg.urls[0] != "http://o/x.m3u8" || reg.des[0].Resultado != "iniciado" || reg.des[0].MsPrimerFrame != 2100 {
		t.Errorf("registrado: %v %+v", reg.urls, reg.des)
	}
}

func TestDesenlaceHandlerRechazaCuerposInvalidos(t *testing.T) {
	h := NewDesenlaceHandler(slog.New(slog.DiscardHandler), &registradorFalso{})
	for _, body := range []string{
		`no json`,
		`{"resultado":"fallo"}`,
		`{"url":"http://o/x.m3u8"}`,
		`{"url":"http://o/x.m3u8","resultado":"cortado"}`,
		`{"url":"http://o/x.m3u8","resultado":"fallo","motivo":"` + strings.Repeat("x", 5000) + `"}`,
	} {
		if rec := postDesenlace(h, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%.40s → %d, quiero 400", body, rec.Code)
		}
	}
}

func TestDesenlaceHandlerSinRegistradorEs503(t *testing.T) {
	h := NewDesenlaceHandler(slog.New(slog.DiscardHandler), nil)
	if rec := postDesenlace(h, `{"url":"http://o/x.m3u8","resultado":"fallo","motivo":"caido"}`); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("código %d, quiero 503", rec.Code)
	}
}
```

En `channel_handler_test.go`: `TestGetChannelStreamsDevuelveMirrorsOrdenados` da al primer mirror `Audio: domain.AudioNo, ImagenMs: 2100, FallosReales: 2, UltimoDesenlace: time.Now().Add(-time.Hour), UltimoMotivo: "desconocido"` y asevera `got[0]["audio_ok"] == false`, `got[0]["imagen_ms"] == float64(2100)`, `got[0]["sin_imagen"] == true`, `got[0]["ultimo_fallo_hace_s"]` ≈ 3600 (entre 3590 y 3610); el segundo (sin datos) `audio_ok == nil`, `sin_imagen == false`, `ultimo_fallo_hace_s == float64(0)`. Nuevo test `TestGetImagenDevuelveSoloCanalesConDatos`: `mockStreamRepo{imagen: []ports.ImagenCanal{{ChannelID: "c1", ImagenMs: 1200}, {ChannelID: "c2", SinImagen: true}}}` → `GET /channels/imagen` 200 con `{"c1":{"imagen_ms":1200,"sin_imagen":false},"c2":{"imagen_ms":0,"sin_imagen":true}}`; con lista vacía → `{}` (no `null`).

En `catalogo_stats_test.go`: añade streams con `RegistrarDesenlace` (dos éxitos 1000 y 3000 → `imagen_p50_ms` = 1000 o 3000 según la mediana definida abajo: con n par se toma el elemento `n/2` ordenado, es decir 3000; documenta la elección en el test) y un mirror con 2 fallos reales → `sin_imagen == 1`.

En `router_test.go`: `TestTablaDeRutas` gana `{"/channels/imagen", http.StatusOK}`; nuevo:

```go
func TestStreamsDesenlaceEsMutanteProtegido(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncVacio{}, sourcesVacio{}, t.TempDir(), nil, api.Options{})
	cuerpo := `{"url":"http://o/x.m3u8","resultado":"fallo","motivo":"caido"}`
	// Cross-site: lo corta MismoOrigen antes de llegar al handler.
	req := httptest.NewRequest(http.MethodPost, "/streams/desenlace", strings.NewReader(cuerpo))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site → %d, quiero 403", rec.Code)
	}
	// Mismo origen sin registrador (tests): 503, no panic.
	req = httptest.NewRequest(http.MethodPost, "/streams/desenlace", strings.NewReader(cuerpo))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("sin registrador → %d, quiero 503", rec.Code)
	}
}
```

- [ ] **Paso 2:** ver los fallos. **Paso 3: Implementar**

`desenlace_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"

	"github.com/gdberysan/open-tv/internal/ports"
)

// DesenlaceHandler recibe del reproductor el desenlace REAL de un intento
// sobre un mirror (spec tiempo-hasta-la-imagen §3.1/§3.5). Mismo origen
// (MismoOrigen es global), best-effort para el cliente, escritura por el
// pool de escritura que main.go inyecta.
type DesenlaceHandler struct {
	logger      *slog.Logger
	registrador ports.RegistradorDesenlaces
}

func NewDesenlaceHandler(logger *slog.Logger, registrador ports.RegistradorDesenlaces) *DesenlaceHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &DesenlaceHandler{logger: logger, registrador: registrador}
}

type desenlaceBody struct {
	URL           string  `json:"url"`
	Resultado     string  `json:"resultado"`
	Motivo        string  `json:"motivo"`
	MsPrimerFrame float64 `json:"ms_primer_frame"` // float: performance.now() trae decimales
}

func (h *DesenlaceHandler) Post(w http.ResponseWriter, r *http.Request) {
	if h.registrador == nil {
		http.Error(w, "registro de desenlaces no disponible", http.StatusServiceUnavailable)
		return
	}
	var body desenlaceBody
	if err := json.NewDecoder(io.LimitReader(r.Body, maxCuerpoPlayback)).Decode(&body); err != nil {
		http.Error(w, "cuerpo invalido o demasiado grande", http.StatusBadRequest)
		return
	}
	if body.URL == "" || (body.Resultado != "iniciado" && body.Resultado != "fallo") {
		http.Error(w, "url y resultado (iniciado|fallo) son obligatorios", http.StatusBadRequest)
		return
	}
	d := ports.DesenlaceMirror{Resultado: body.Resultado, Motivo: body.Motivo, MsPrimerFrame: int64(math.Round(body.MsPrimerFrame))}
	if err := h.registrador.RegistrarDesenlace(r.Context(), body.URL, d); err != nil {
		h.logger.Warn("desenlace: fallo registrando", slog.Any("error", err))
		http.Error(w, "no se pudo registrar", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

(`maxCuerpoPlayback` ya existe en `stats_handler.go`, mismo paquete. El motivo de 5.000 bytes cae en el 400 por el `LimitReader`: el decoder falla al truncarse el JSON.)

`channel_handler.go`: `mirrorJSON` gana `AudioOK *bool json:"audio_ok"`, `ImagenMs int64 json:"imagen_ms"`, `SinImagen bool json:"sin_imagen"`, `UltimoFalloHaceS int64 json:"ultimo_fallo_hace_s"`; en el bucle:

```go
		ahora := time.Now()
		…
		var haceS int64
		if m.FallosReales > 0 && !m.UltimoDesenlace.IsZero() {
			haceS = int64(ahora.Sub(m.UltimoDesenlace).Seconds())
		}
		salida = append(salida, mirrorJSON{
			…,
			AudioOK: audioOKaPtr(m.Audio), ImagenMs: m.ImagenMs,
			SinImagen: domain.SinImagen(m.FallosReales, m.UltimoDesenlace, ahora),
			UltimoFalloHaceS: haceS,
		})
```

`audioOKaPtr` como `codecOKaPtr`. Nuevo `GetImagen`:

```go
// GetImagen: GET /channels/imagen → {id: {imagen_ms, sin_imagen}} solo para
// canales con algún desenlace. Objeto vacío, nunca null.
func (h *ChannelHandler) GetImagen(w http.ResponseWriter, r *http.Request) {
	lista, err := h.streams.ImagenPorCanal(r.Context(), time.Now())
	if err != nil {
		h.logger.Error("GetImagen: fallo agregando", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error agregando la imagen por canal")
		return
	}
	type imagenJSON struct {
		ImagenMs  int64 `json:"imagen_ms"`
		SinImagen bool  `json:"sin_imagen"`
	}
	salida := make(map[string]imagenJSON, len(lista))
	for _, ic := range lista {
		salida[string(ic.ChannelID)] = imagenJSON{ImagenMs: ic.ImagenMs, SinImagen: ic.SinImagen}
	}
	h.writeJSON(w, http.StatusOK, salida)
}
```

`stats_handler.go` `Resumen`: tras el SELECT existente, dos consultas más:
`SELECT imagen_ms FROM streams WHERE imagen_ms > 0 ORDER BY imagen_ms` → mediana en Go (elemento `n/2` de la lista ordenada; 0 si vacía); `SELECT fallos_reales, ultimo_desenlace_at FROM streams WHERE fallos_reales >= ?` (con `domain.UmbralFallosReales`) → cuenta las filas con `domain.SinImagen(..., time.Now())`. Claves `"imagen_p50_ms"` y `"sin_imagen"`.

`router.go`: `Options` gana `Desenlaces ports.RegistradorDesenlaces` (comentario: escritura; nil en tests → 503); `dh := handlers.NewDesenlaceHandler(logger, opts.Desenlaces)`; `r.Post("/streams/desenlace", dh.Post)`; dentro de `/channels`: `r.Get("/imagen", ch.GetImagen)` ANTES de `/{id}/health` (mismo cuidado que las otras literales). `main.go`: `Desenlaces: streamRepo` en `api.Options{…}` (el repo del pool de escritura, línea ~172).

- [ ] **Paso 4:** gates Go + `go test -race -count=1 ./internal/domain/ -run Channel` (15 claves). **Paso 5:** commit `feat(api): desenlaces por mirror, imagen por canal y audio_ok en el cable`.

---

### Task 5: Cliente — datos, reporter de desenlaces y store de imagen

**Files:**
- Modify: `web/src/datos/catalogo.ts` (`Mirror`, `CatalogSource.imagen()`), `web/src/datos/http.ts` (+ `http.test.ts`), `web/src/reproductor/failover.ts`
- Create: `web/src/estado/desenlaces.ts` (+ test), `web/src/estado/imagen.ts` (+ test)

**Interfaces:**
- Consumes: cable de la Task 4.
- Produces:
  - `Mirror` gana `audioOk?: boolean | null`, `imagenMs?: number`, `sinImagen?: boolean`, `ultimoFalloHaceS?: number`.
  - `CatalogSource.imagen(): Promise<Record<string, { imagenMs: number; sinImagen: boolean }>>` (GET `/channels/imagen`; el mock/`CatalogSource` de tests que no lo implemente: `fuente.imagen?.()` con `?.` en el llamador).
  - `DesenlaceReproduccion` gana `url: string`, `oculto?: boolean` (la pestaña estuvo oculta durante el intento), `motorForzado?: boolean`.
  - `reportarDesenlaceMirror(o: DesenlaceReproduccion, base = ''): boolean` — devuelve si se reportó; no reporta cuando `o.via === 'ninguna'`, `o.motorForzado`, `o.oculto`, `navigator.onLine === false` o `!o.url`; POST a `/streams/desenlace` con `{url, resultado, motivo, ms_primer_frame}` (`resultado: 'cortado'` NO se reporta: un corte tras verse no es un intento).
  - `crearImagen(fuente)`: store `Readable<Map<string, {imagenMs, sinImagen}>>` con `cargar()` y `refrescar()` (debounce 1 s; `refrescar` durante el debounce no acumula fetches).

- [ ] **Paso 1: Tests (fallan)**

`http.test.ts`: el test de mirrors gana `audio_ok:false, imagen_ms:2100, sin_imagen:true, ultimo_fallo_hace_s:3600` en el primero y asevera el mapeo; un mirror sin las claves → `audioOk: null, imagenMs: 0, sinImagen: false, ultimoFalloHaceS: 0`. Nuevo: `imagen() lee GET /channels/imagen y devuelve el mapa` (`{"c1":{"imagen_ms":1200,"sin_imagen":false}}` → `{ c1: { imagenMs: 1200, sinImagen: false } }`).

`desenlaces.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reportarDesenlaceMirror } from './desenlaces'
import type { DesenlaceReproduccion } from '../reproductor/failover'

const base: DesenlaceReproduccion = {
  canalId: 'c1', resultado: 'fallo', motivo: 'desconocido', motor: 'hlsjs', via: 'proxy', mirrorIndex: 0, url: 'http://o/x.m3u8',
}
let espia: ReturnType<typeof vi.fn>
beforeEach(() => {
  espia = vi.fn(async (..._args: unknown[]) => new Response(null, { status: 204 }))
  vi.stubGlobal('fetch', espia)
  Object.defineProperty(navigator, 'onLine', { value: true, configurable: true })
})
afterEach(() => vi.unstubAllGlobals())

describe('reportarDesenlaceMirror', () => {
  it('reporta un fallo con su url y motivo', () => {
    expect(reportarDesenlaceMirror(base, '')).toBe(true)
    const [url, opts] = espia.mock.calls[0]
    expect(String(url)).toContain('/streams/desenlace')
    expect(JSON.parse((opts as RequestInit).body as string)).toEqual({ url: 'http://o/x.m3u8', resultado: 'fallo', motivo: 'desconocido', ms_primer_frame: 0 })
  })
  it('reporta un éxito con ms_primer_frame entero', () => {
    expect(reportarDesenlaceMirror({ ...base, resultado: 'iniciado', motivo: undefined, msPrimerFrame: 2100.6 }, '')).toBe(true)
    expect(JSON.parse((espia.mock.calls[0][1] as RequestInit).body as string).ms_primer_frame).toBe(2101)
  })
  it.each([
    ['sin intento (via ninguna)', { ...base, via: 'ninguna' as const }],
    ['motor forzado (cast)', { ...base, motorForzado: true }],
    ['pestaña oculta durante el intento', { ...base, oculto: true }],
    ['corte tras verse', { ...base, resultado: 'cortado' as const }],
    ['sin url', { ...base, url: '' }],
  ])('NO reporta: %s', (_n, d) => {
    expect(reportarDesenlaceMirror(d, '')).toBe(false)
    expect(espia).not.toHaveBeenCalled()
  })
  it('NO reporta sin conexión', () => {
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true })
    expect(reportarDesenlaceMirror(base, '')).toBe(false)
  })
  it('nunca lanza aunque fetch reviente', () => {
    vi.stubGlobal('fetch', vi.fn(() => { throw new Error('boom') }))
    expect(() => reportarDesenlaceMirror(base, '')).not.toThrow()
  })
})
```

`imagen.test.ts`: `crearImagen(fuente)` con `fuente.imagen` espía: `cargar()` rellena el mapa; dos `refrescar()` seguidos dentro del debounce provocan UNA sola llamada (con `vi.useFakeTimers()`); un `imagen()` que rechaza deja el mapa anterior intacto.

- [ ] **Paso 2:** ver los fallos. **Paso 3: Implementar** (`desenlaces.ts` con el mismo patrón try/catch + `void fetch(...).catch(() => {})` de `estadisticas.ts`; `imagen.ts` con `writable` de `svelte/store` como `favoritos.ts`, sin localStorage). **Paso 4:** gates web. **Paso 5:** commit `feat(web): reporter de desenlaces por mirror y store de imagen por canal`.

---

### Task 6: Cliente — reproductor: saltar «sin imagen», «Probar de todos modos», «Sin audio», desenlace con url

**Files:**
- Modify: `web/src/componentes/Reproductor.svelte`, `web/src/i18n/es.ts`, `web/src/i18n/en.ts`
- Test: `web/src/componentes/Reproductor.test.ts`

**Interfaces:**
- Consumes: `Mirror.sinImagen/audioOk/ultimoFalloHaceS`, `DesenlaceReproduccion.url/oculto/motorForzado` (Task 5).
- Produces: claves i18n `reproductor.error.sinImagen` («Ningún origen de este canal llega a dar imagen desde aquí (último intento hace {hace}).» / «None of this channel's origins delivers a picture from here (last tried {hace} ago).»), `reproductor.error.probarIgual` («Probar de todos modos» / «Try anyway»), `reproductor.sinAudio` («Sin audio en este origen» / «No audio on this origin»), `tiempo.haceMin` («hace {n} min» / «{n} min ago»), `tiempo.haceH` («hace {n} h» / «{n} h ago»); `reproducir(opts?: { ignorarSinImagen?: boolean })`; cada `alDesenlace` lleva `url`, `oculto`, `motorForzado`.

- [ ] **Paso 1: Tests (fallan)** — en `Reproductor.test.ts`:

```ts
  it('salta los mirrors con sinImagen y solo prueba los demás', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://falla/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, sinImagen: true, ultimoFalloHaceS: 600 },
      { url: 'https://bueno/x.m3u8', vivo: true, latenciaMs: 200, webOk: true },
    ]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const intentadas: string[] = []
    render(Reproductor, { canal, fuente: fuente as any, alIntentar: (url: string) => intentadas.push(url) })
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(intentadas).toEqual(['https://bueno/x.m3u8'])
  })

  it('con todos los mirrors sin imagen muestra el mensaje con «hace» y el botón «Probar de todos modos», que sí los intenta', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://falla/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, sinImagen: true, ultimoFalloHaceS: 3 * 3600 },
    ]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const desenlaces: DesenlaceReproduccion[] = []
    const intentadas: string[] = []
    render(Reproductor, { canal, fuente: fuente as any, alIntentar: (u: string) => intentadas.push(u), alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d) })

    const esperado = t('reproductor.error.sinImagen', { hace: t('tiempo.haceH', { n: 3 }) })
    await vi.waitFor(() => expect(screen.queryAllByText(esperado).length).toBeGreaterThan(0))
    expect(intentadas).toEqual([])
    expect(desenlaces).toEqual([{ canalId: 'c1', resultado: 'fallo', motivo: 'sinImagen', motor: 'hlsjs', via: 'ninguna', mirrorIndex: 0, url: '', oculto: false, motorForzado: false }])
    expect(screen.queryByText(t('reproductor.error.reintentar'))).toBeNull()

    await fireEvent.click(screen.getByText(t('reproductor.error.probarIgual')))
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(intentadas).toEqual(['https://falla/x.m3u8'])
  })

  it('«Probar de todos modos» ignora sinImagen pero NO los saltos por códec', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://mpeg2/x.m3u8', vivo: true, latenciaMs: 50, webOk: true, codecOk: false, codecs: 'mpeg2video' },
      { url: 'https://falla/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, sinImagen: true, ultimoFalloHaceS: 60 },
    ]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const intentadas: string[] = []
    render(Reproductor, { canal, fuente: fuente as any, alIntentar: (u: string) => intentadas.push(u) })
    await vi.waitFor(() => expect(screen.queryByText(t('reproductor.error.probarIgual'))).not.toBeNull())
    await fireEvent.click(screen.getByText(t('reproductor.error.probarIgual')))
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(intentadas).toEqual(['https://falla/x.m3u8'])
  })

  it('el desenlace de cada intento lleva la url del mirror', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [{ url: 'https://uno/x.m3u8', vivo: true, latenciaMs: 100, webOk: true }]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const desenlaces: DesenlaceReproduccion[] = []
    render(Reproductor, { canal, fuente: fuente as any, alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d) })
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].fallar('manifestLoadError')
    await vi.waitFor(() => expect(desenlaces).toHaveLength(1))
    expect(desenlaces[0].url).toBe('https://uno/x.m3u8')
    expect(desenlaces[0].oculto).toBe(false)
    expect(desenlaces[0].motorForzado).toBe(false)
  })

  it('reproduciendo un mirror sin audio se muestra «Sin audio en este origen»', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [{ url: 'https://mudo/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, audioOk: false }]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const { container } = render(Reproductor, { canal, fuente: fuente as any })
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    // Confirmar reproducción como hacen los tests existentes del guard:
    // dos timeupdate con posición distinta sobre el <video>.
    const video = container.querySelector('video')!
    Object.defineProperty(video, 'currentTime', { value: 1, configurable: true, writable: true })
    video.dispatchEvent(new Event('timeupdate'))
    ;(video as any).currentTime = 2
    video.dispatchEvent(new Event('timeupdate'))
    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.sinAudio')).length).toBeGreaterThan(0))
  })
```

(Si el fichero ya tiene un helper para «confirmar reproducción», úsalo en vez de los dos `timeupdate` a mano; `fireEvent` viene de `@testing-library/svelte`.)

- [ ] **Paso 2:** ver los fallos. **Paso 3: Implementar**

En `Reproductor.svelte`:

- Estado: `let ocultoEnIntento = $state(false)`; `let mirrorActual = $state<Mirror | null>(null)` (el mirror del intento vigente, para la nota de audio); `let ignorandoSinImagen = false`.
- `onVisibilidad`: al pasar a oculta, `ocultoEnIntento = true` (además de `guard.pausar()`).
- `intentar(intento, motor)`: al empezar, `ocultoEnIntento = document.hidden`; `mirrorActual = mirrorDe(intento)` (búscalo por `intento.url === m.url || urlProxy(m.url) === intento.url` en la lista de mirrors del `reproducir()` vigente; guarda esa lista en una variable del componente). Todos los `alDesenlace({...})` del fichero llevan `url: intento.url` (o `''` en los desenlaces sin intento), `oculto: ocultoEnIntento`, `motorForzado: motorForzado !== null`.
- `reproducir(opts: { ignorarSinImagen?: boolean } = {})`: tras el filtro de códec:

```ts
        const conImagen = opts.ignorarSinImagen ? reproducibles : reproducibles.filter((m) => m.sinImagen !== true)
        if (conImagen.length === 0) {
          // (mismo teardown de cast que arriba, factorizado en una función local)
          cargando = false
          errorSinReintento = false
          errorProbarIgual = true
          const haceS = Math.max(...mirrors.map((m) => m.ultimoFalloHaceS ?? 0))
          mensajeError = t('reproductor.error.sinImagen', { hace: formatearHace(haceS) })
          alDesenlace({ canalId: canal.id, resultado: 'fallo', motivo: 'sinImagen', motor, via: 'ninguna', mirrorIndex: 0, url: '', oculto: false, motorForzado: false })
          return
        }
        totalMirrors = conImagen.length
        …
        intentos = planDeFailover(conImagen, motor, proxyDisp)
```

  con `let errorProbarIgual = $state(false)` (reset a `false` al inicio de `reproducir`) y `function probarIgual() { limpiarIntento(); reproducir({ ignorarSinImagen: true }) }`. `formatearHace(s)`: `< 3600` → `t('tiempo.haceMin', { n: Math.max(1, Math.round(s / 60)) })`, si no `t('tiempo.haceH', { n: Math.round(s / 3600) })`.
- Marcado del error: tras el botón de reintentar,
  `{#if errorProbarIgual}<button type="button" class="probar-mirror" onclick={probarIgual}>{t('reproductor.error.probarIgual')}</button>{/if}`, y el de reintentar pasa a `{#if !errorSinReintento && !errorProbarIgual}`.
- Nota de audio: en el overlay, junto a la insignia «En vivo»: `{#if !cargando && !mensajeError && mirrorActual?.audioOk === false}<span class="overlay-sin-audio">{t('reproductor.sinAudio')}</span>{/if}`; y en la región polite persistente (línea ~1125) la expresión pasa a `cargando ? … : estadoCast === 'emitiendo' ? … : mirrorActual?.audioOk === false ? t('reproductor.sinAudio') : ''`. Sin animación, sin región nueva.

- [ ] **Paso 4:** gates web (bundle). **Paso 5:** commit `feat(web): el reproductor salta los mirrors sin imagen, ofrece «Probar de todos modos» y avisa «Sin audio»`.

---

### Task 7: Cliente — tarjeta y cableado en App

**Files:**
- Modify: `web/src/componentes/SenalCanal.svelte` (+ test), `ListaCanalesLateral.svelte`, `RejillaCanales.svelte`, `TarjetaCanal.svelte`, `web/src/App.svelte`, `web/src/i18n/es.ts`, `en.ts`

**Interfaces:**
- Consumes: `crearImagen`, `reportarDesenlaceMirror` (Task 5).
- Produces: `SenalCanal` props `imagenMs?: number`, `sinImagen?: boolean`; claves `senal.sinImagen` («Sin imagen desde aquí» / «No picture from here»), `senal.imagenEn` («Imagen en {s} s» / «Picture in {s} s»); contexto Svelte `imagen` (store) que los tres llamadores leen con `getContext`.

- [ ] **Paso 1: Tests (fallan)** — `SenalCanal.test.ts`:

```ts
  it('con imagenMs pinta «Imagen en 2,1 s» y lo dice en el aria-label', () => {
    const { container } = render(SenalCanal, { vivo: true, latenciaMs: 120, imagenMs: 2140 })
    expect(container.textContent).toContain('Imagen en 2,1 s')
    expect(container.querySelector('.punto')?.getAttribute('aria-label')).toBe('Señal viva, imagen en 2,1 s')
    expect(container.textContent).not.toContain('120 ms')
  })
  it('con sinImagen pinta el punto de error y «Sin imagen desde aquí»', () => {
    const { container } = render(SenalCanal, { vivo: true, latenciaMs: 120, sinImagen: true })
    expect(container.querySelector('.punto')?.classList.contains('sin-imagen')).toBe(true)
    expect(container.querySelector('.punto')?.getAttribute('aria-label')).toBe('Sin imagen desde aquí')
    expect(container.textContent).toContain('Sin imagen desde aquí')
  })
  it('sin datos de imagen sigue como hoy', () => {
    const { container } = render(SenalCanal, { vivo: true, latenciaMs: 120 })
    expect(container.textContent).toContain('120 ms')
  })
```

(Formato del decimal: `(ms / 1000).toFixed(1).replace('.', ',')` en `es`, con punto en `en` — usa `idioma.actual` como hace `formatearHoraLocal` si ya distingue; si no, un `toLocaleString` con el idioma actual.)

- [ ] **Paso 2:** ver los fallos. **Paso 3: Implementar**
  - `SenalCanal.svelte`: nuevos props opcionales; `estado` gana `'sin-imagen'` (clase `.punto.sin-imagen { background: var(--signal-error); }`); texto: `sinImagen` → `t('senal.sinImagen')`; si no, `imagenMs > 0` → `t('senal.imagenEn', { s })`; si no, los ms de hoy. `aria-label`: `sinImagen` → `senal.sinImagen`; `imagenMs` → `t(clave) + ', ' + t('senal.imagenEn', { s }).toLowerCase()`; si no, `t(clave)`.
  - `App.svelte`: `const imagen = crearImagen(fuente)` tras crear `fuente`; `setContext('imagen', imagen)`; `imagen.cargar()` al arrancar (donde se cargan favoritos/historial); el prop del reproductor pasa a `alDesenlace={(d) => { reportarDesenlace(d); if (reportarDesenlaceMirror(d)) imagen.refrescar() }}`.
  - Los tres llamadores: `const imagen = getContext<ReturnType<typeof crearImagen>>('imagen')` y `<SenalCanal vivo={canal.vivo} latenciaMs={canal.latenciaMs} imagenMs={$imagen.get(canal.id)?.imagenMs} sinImagen={$imagen.get(canal.id)?.sinImagen} />`. Si un test de esos componentes no provee el contexto, `getContext` devuelve `undefined`: usa `imagen?.` / un store vacío por defecto (`writable(new Map())`) para que los tests existentes no cambien.

- [ ] **Paso 4:** gates web. **Paso 5:** commit `feat(web): la tarjeta dice «Imagen en N s» o «Sin imagen desde aquí»`.

---

### Task 8: Integración real contra AMC (720p) y cierre

**Files:** `internal/ui/dist/.gitkeep` (restaurar si el build lo borra), `docs/superpowers/evidencia/2026-09-06-sin-imagen.png`.

- [ ] **Paso 1:** gates completos Go + Web + `cd mobile && flutter analyze && flutter test`; `git status --short mobile/` vacío.
- [ ] **Paso 2:** `go build -o open-tv ./cmd/open-tv` desde la raíz; `git checkout -- internal/ui/dist/.gitkeep`; `pkill -f "open-tv serve"`; comprobar `curl -s http://127.0.0.1:8080/health` y el hash `index-*.js` servido = `ls internal/ui/dist/assets/`.
- [ ] **Paso 3 (evidencia SQL, DB `.devdata/iptv.db`):** en Chrome VISIBLE (`?fresh=imagen1`, comprobar `document.visibilityState`), sintonizar «AMC (720p)» y esperar al error; repetir una segunda vez. Consultar:
  `SELECT url, audio_ok, imagen_ms, fallos_reales, ultimo_desenlace_at, ultimo_motivo FROM streams WHERE channel_id='opensource-AMC (720p)';`
  Esperado: `41.205.93.154` con `audio_ok=0`, `fallos_reales=2`, `ultimo_motivo` ∈ {desconocido, inestable}. Y `curl -s "http://127.0.0.1:8080/channels/streams?id=opensource-AMC%20(720p)"` con `sin_imagen:true` en ese mirror.
- [ ] **Paso 4:** tercera sintonización: el mensaje «Ningún origen… (último intento hace N min)» sale AL INSTANTE con «Probar de todos modos» y sin «Reintentar»; `curl -s http://127.0.0.1:8080/channels/imagen` incluye `"opensource-AMC (720p)":{"imagen_ms":0,"sin_imagen":true}`; la tarjeta de AMC en la lateral dice «Sin imagen desde aquí» (snapshot de accesibilidad: `aria-label`). Captura SOLO de la región del reproductor → `docs/superpowers/evidencia/2026-09-06-sin-imagen.png` (sin tira «Continuar viendo», sin lateral).
- [ ] **Paso 5:** sintonizar un canal bueno (p. ej. «90s Throwback»): tras verse, `/channels/imagen` trae su `imagen_ms > 0` y su tarjeta dice «Imagen en N,N s». Pegar todo (comandos + salidas) en el ledger de SDD.
- [ ] **Paso 6:** commit `docs(evidencia): AMC sin imagen desde aquí y tarjeta con tiempo real` con SOLO la captura (y `.gitkeep` si cambió). No mergear ni pushear.

---

## Autorrevisión del plan

- **Cobertura de la spec:** §3.1 (qué es real) → Task 1 (`MotivoEsFalloReal`) + Task 5 (exclusiones del reporter); §3.2 → Task 2 + Task 3 (`audio_ok`); §3.3 → Task 1 + Task 4 (`sin_imagen` en el cable); §3.4 → Task 2 (ORDER BY); §3.5 → Task 4; §4.1 → Task 5; §4.2 y §4.3 → Task 6; §4.4 → Task 7; §5 → cada tarea + Task 8; §7 → nada lo implementa.
- **Tipos consistentes:** `AudioSupport/AudioNo/AudioOK/AudioUnknown`; `SinImagen(fallos, ultimo, ahora)`; `MotivoEsFalloReal`; `ports.DesenlaceMirror{Resultado, Motivo, MsPrimerFrame}`; `ports.ImagenCanal{ChannelID, ImagenMs, SinImagen}`; `ports.RegistradorDesenlaces.RegistrarDesenlace(ctx, url, d)`; `StreamRepository.ImagenPorCanal(ctx, ahora)`; `MirrorHealth.Audio/ImagenMs/FallosReales/UltimoDesenlace/UltimoMotivo`; cable `audio_ok/imagen_ms/sin_imagen/ultimo_fallo_hace_s`; `Mirror.audioOk/imagenMs/sinImagen/ultimoFalloHaceS`; `DesenlaceReproduccion.url/oculto/motorForzado`; `reproducir({ ignorarSinImagen })`; `api.Options.Desenlaces`.
- **Sin placeholders.** Los pasos que dicen «como X» apuntan a código existente con nombre.
