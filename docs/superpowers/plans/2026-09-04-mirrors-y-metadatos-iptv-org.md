# Mirrors y metadatos desde la API de iptv-org — Plan de implementación

> **Para trabajadores agénticos:** SUB-SKILL OBLIGATORIA: usa
> `superpowers:subagent-driven-development` (recomendada) o
> `superpowers:executing-plans` para implementar tarea a tarea. Los pasos usan
> casillas (`- [ ]`) para el seguimiento.

**Goal:** Que un canal cuya única URL está muerta deje de serlo, aprovechando los
mirrors que la API de iptv-org publica y el `index.m3u` descarta.

**Architecture:** El proveedor `opensource` sigue siendo el parser genérico de
M3U y recibe **opcionalmente** un `Enriquecedor` (paquete nuevo `iptvorg`) que
aporta mirrors, cabeceras y categorías. `nil` = comportamiento de hoy, que es lo
que necesitan los M3U que sube el usuario. El syncer pasa a guardar N streams por
canal y a podar los que ya no aparecen.

**Tech Stack:** Go 1.x, `encoding/json` (sin dependencias nuevas), SQLite
(modernc), chi v5.

**Spec:** `docs/superpowers/specs/2026-09-04-mirrors-y-metadatos-iptv-org-design.md`

## Global Constraints

- **`mobile/` CERO diffs.** No tocar ningún fichero bajo `mobile/`.
- **`/channels` congelado en 15 claves.** `internal/domain/channel_test.go` lo
  asevera. No añadir ni quitar campos a `domain.Channel`.
- **Ninguna dependencia Go nueva.** `encoding/json` basta para ambos JSON.
- **Idioma:** código, comentarios y mensajes de commit en **español**.
- **Identidad de commits:** autor `Gerard <gdberysan@gmail.com>`. Trailer
  obligatorio: `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.
- **NUNCA `git add -A`.** Añadir por ruta explícita.
- **Gates verdes al final de CADA tarea:** `gofmt -l .` · `go vet ./...` ·
  `go build ./...` · `go test -race -count=1 ./...` · `golangci-lint run ./...` ·
  `go run ./tools/scrubcheck`.
- **`golangci-lint` NO honra `//nosec`** — usar `//nolint:gosec` si hiciera falta.
- **La guarda SSRF del proxy no se toca:** `controlConexion`, `checkRedirect`,
  tope de tamaño y solo-loopback siguen exactamente igual.

---

## Estructura de ficheros

**Crear:**
- `internal/adapters/providers/iptvorg/enriquecedor.go` — cliente de la API,
  índices en memoria y separación de `tvg_id`. Única responsabilidad: convertir
  los dos JSON de iptv-org en consultas baratas por `(canal, feed)`.
- `internal/adapters/providers/iptvorg/enriquecedor_test.go`
- `internal/adapters/providers/iptvorg/testdata/streams.json` — fixture pequeña.
- `internal/adapters/providers/iptvorg/testdata/channels.json` — fixture pequeña.

**Modificar:**
- `internal/domain/stream.go` — campos `Referrer`, `UserAgent`.
- `internal/adapters/db/schema.sql` — columnas nuevas en `streams`.
- `internal/adapters/db/db.go` — `alterMigrations`.
- `internal/adapters/db/stream_repository.go` — upsert con cabeceras y
  `last_seen_at`; `DeleteStale`; `CabecerasPorURL`.
- `internal/ports/stream_repository.go` — firmas nuevas.
- `internal/ports/provider_port.go` — `GetStreamsDeCanal`.
- `internal/adapters/providers/opensource/provider.go` — `WithEnriquecedor` y
  `GetStreamsDeCanal`.
- `internal/services/syncer.go` — N streams por canal, poda, categorías.
- `internal/proxy/handler.go` — cabeceras por stream.
- `internal/api/router.go` — inyección de la búsqueda de cabeceras.
- `internal/adapters/validator/checker.go` — cabeceras en el health-check.

---

### Task 1: Enriquecedor — cliente e índices de la API

**Files:**
- Create: `internal/adapters/providers/iptvorg/enriquecedor.go`
- Test: `internal/adapters/providers/iptvorg/enriquecedor_test.go`
- Create: `internal/adapters/providers/iptvorg/testdata/streams.json`
- Create: `internal/adapters/providers/iptvorg/testdata/channels.json`

**Interfaces:**
- Consumes: nada (paquete nuevo, autocontenido).
- Produces:
  - `type StreamExtra struct { URL, Referrer, UserAgent string }`
  - `func SepararTvgID(tvgID string) (canal, feed string)`
  - `type Enriquecedor struct{ ... }`
  - `func NuevoEnriquecedor(client *http.Client) *Enriquecedor`
  - `func (e *Enriquecedor) Cargar(ctx context.Context) error`
  - `func (e *Enriquecedor) Streams(canal, feed string) []StreamExtra`
  - `func (e *Enriquecedor) Categoria(canal string) (string, bool)`
  - `const URLStreams`, `const URLChannels`

- [ ] **Paso 1: Escribir las fixtures de test**

`internal/adapters/providers/iptvorg/testdata/streams.json`:

```json
[
  {"channel":"AndTV.in","feed":"HD","url":"https://uno.example/hd.m3u8","referrer":null,"user_agent":null},
  {"channel":"AndTV.in","feed":"HD","url":"https://dos.example/hd.m3u8","referrer":"https://ref.example/","user_agent":"VLC/3.0"},
  {"channel":"AndTV.in","feed":"SD","url":"https://tres.example/sd.m3u8","referrer":null,"user_agent":null},
  {"channel":"Solo.uk","feed":null,"url":"https://cuatro.example/x.m3u8","referrer":null,"user_agent":null},
  {"channel":null,"feed":null,"url":"https://huerfano.example/x.m3u8","referrer":null,"user_agent":null}
]
```

`internal/adapters/providers/iptvorg/testdata/channels.json`:

```json
[
  {"id":"AndTV.in","name":"& TV","country":"IN","categories":["entertainment","movies"]},
  {"id":"Solo.uk","name":"Solo","country":"GB","categories":[]}
]
```

- [ ] **Paso 2: Escribir el test que falla**

```go
package iptvorg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// servidorFixtures sirve los dos JSON de testdata en las rutas que el
// Enriquecedor pide, para poder probar Cargar() sin salir a la red.
func servidorFixtures(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var fichero string
		switch r.URL.Path {
		case "/api/streams.json":
			fichero = "testdata/streams.json"
		case "/api/channels.json":
			fichero = "testdata/channels.json"
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		b, err := os.ReadFile(fichero)
		if err != nil {
			t.Errorf("leyendo %s: %v", fichero, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
}

func cargado(t *testing.T) *Enriquecedor {
	t.Helper()
	srv := servidorFixtures(t)
	t.Cleanup(srv.Close)
	e := NuevoEnriquecedor(srv.Client())
	e.urlStreams = srv.URL + "/api/streams.json"
	e.urlChannels = srv.URL + "/api/channels.json"
	if err := e.Cargar(context.Background()); err != nil {
		t.Fatalf("Cargar: %v", err)
	}
	return e
}

func TestSepararTvgID(t *testing.T) {
	casos := []struct{ in, canal, feed string }{
		{"AndTV.in@HD", "AndTV.in", "HD"},
		{"Solo.uk", "Solo.uk", ""},
		{"", "", ""},
		{"Raro.es@A@B", "Raro.es", "A@B"}, // solo se parte por el PRIMER @
	}
	for _, c := range casos {
		canal, feed := SepararTvgID(c.in)
		if canal != c.canal || feed != c.feed {
			t.Errorf("SepararTvgID(%q) = (%q,%q), quiero (%q,%q)", c.in, canal, feed, c.canal, c.feed)
		}
	}
}

// El feed DISCRIMINA: pedir HD no puede devolver el stream del SD. Es la trampa
// que el spec documenta (emparejar solo por canal mezcla calidades).
func TestStreamsRespetaElFeed(t *testing.T) {
	e := cargado(t)

	hd := e.Streams("AndTV.in", "HD")
	if len(hd) != 2 {
		t.Fatalf("HD: %d streams, quiero 2", len(hd))
	}
	if hd[0].URL != "https://uno.example/hd.m3u8" {
		t.Errorf("HD[0] = %q, quiero conservar el orden del JSON", hd[0].URL)
	}
	if hd[1].Referrer != "https://ref.example/" || hd[1].UserAgent != "VLC/3.0" {
		t.Errorf("HD[1] perdió las cabeceras: %+v", hd[1])
	}

	sd := e.Streams("AndTV.in", "SD")
	if len(sd) != 1 || sd[0].URL != "https://tres.example/sd.m3u8" {
		t.Errorf("SD = %+v, quiero solo el stream SD", sd)
	}
}

func TestStreamsFeedVacioYDesconocido(t *testing.T) {
	e := cargado(t)

	if got := e.Streams("Solo.uk", ""); len(got) != 1 {
		t.Errorf("feed vacío: %d streams, quiero 1", len(got))
	}
	if got := e.Streams("NoExiste.xx", "HD"); got != nil {
		t.Errorf("canal desconocido devolvió %+v, quiero nil", got)
	}
}

// Un stream sin 'channel' no se puede ligar a nada: se descarta en el índice.
func TestStreamsSinCanalSeDescartan(t *testing.T) {
	e := cargado(t)
	if n := e.totalIndexados(); n != 4 {
		t.Errorf("indexados %d, quiero 4 (el huérfano no cuenta)", n)
	}
}

func TestCategoria(t *testing.T) {
	e := cargado(t)

	got, ok := e.Categoria("AndTV.in")
	if !ok || got != "entertainment" {
		t.Errorf("Categoria = (%q,%v), quiero (entertainment,true): se toma la PRIMERA", got, ok)
	}
	if _, ok := e.Categoria("Solo.uk"); ok {
		t.Error("un canal con categories vacío no debe reportar categoría")
	}
	if _, ok := e.Categoria("NoExiste.xx"); ok {
		t.Error("un canal desconocido no debe reportar categoría")
	}
}
```

- [ ] **Paso 3: Ejecutar el test y verificar que falla**

Run: `go test ./internal/adapters/providers/iptvorg/ -run . -v`
Expected: FAIL — el paquete no compila (`undefined: NuevoEnriquecedor`, etc.).

- [ ] **Paso 4: Implementar el enriquecedor**

`internal/adapters/providers/iptvorg/enriquecedor.go`:

```go
// Package iptvorg lee la API pública de iptv-org (streams.json y channels.json)
// para aportar lo que el index.m3u descarta: los mirrors extra de cada canal,
// las cabeceras que algunos orígenes exigen, y la categoría real de los canales
// que el M3U deja en «General».
//
// Es un colaborador OPCIONAL del proveedor opensource: sin él, ese proveedor se
// comporta exactamente igual que siempre, que es lo que necesitan los M3U que
// sube el usuario (no tienen API detrás).
package iptvorg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	URLStreams  = "https://iptv-org.github.io/api/streams.json"
	URLChannels = "https://iptv-org.github.io/api/channels.json"

	// maxJSONBytes acota la descarga. Medido el 2026-09-04: streams.json pesa
	// ~3,5 MB y channels.json ~7,8 MB. 64 MB deja margen de sobra sin permitir
	// que un upstream roto agote la memoria.
	maxJSONBytes = 64 << 20

	// tiempoCarga cubre las dos descargas juntas.
	tiempoCarga = 2 * time.Minute
)

// StreamExtra es un stream de la API con las cabeceras que su origen exige.
// Referrer y UserAgent vacíos significan "usa los de siempre".
type StreamExtra struct {
	URL       string
	Referrer  string
	UserAgent string
}

// claveFeed identifica un feed concreto de un canal. El feed DISCRIMINA: el
// mismo canal tiene entradas distintas para HD y SD, y mezclarlas asigna al SD
// los streams del HD.
type claveFeed struct{ canal, feed string }

type Enriquecedor struct {
	client      *http.Client
	urlStreams  string
	urlChannels string

	mu         sync.RWMutex
	porFeed    map[claveFeed][]StreamExtra
	categorias map[string]string
}

func NuevoEnriquecedor(client *http.Client) *Enriquecedor {
	if client == nil {
		client = &http.Client{Timeout: tiempoCarga}
	}
	return &Enriquecedor{
		client:      client,
		urlStreams:  URLStreams,
		urlChannels: URLChannels,
		porFeed:     make(map[claveFeed][]StreamExtra),
		categorias:  make(map[string]string),
	}
}

// SepararTvgID parte el tvg_id del M3U en (canal, feed). El sufijo tras el
// PRIMER '@' es exactamente el campo `feed` de la API:
//
//	"AndTV.in@HD" -> ("AndTV.in", "HD")
//	"Solo.uk"     -> ("Solo.uk", "")
func SepararTvgID(tvgID string) (canal, feed string) {
	canal, feed, _ = strings.Cut(tvgID, "@")
	return canal, feed
}

// entradaStream refleja SOLO los campos que se usan de streams.json. El resto
// (title, quality, labels) se descarta al decodificar.
type entradaStream struct {
	Channel   *string `json:"channel"`
	Feed      *string `json:"feed"`
	URL       string  `json:"url"`
	Referrer  *string `json:"referrer"`
	UserAgent *string `json:"user_agent"`
}

// entradaCanal refleja SOLO lo que se usa de channels.json. OJO: este fichero
// NO tiene campo `languages` (comprobado sobre el JSON real), y pesa ~7,8 MB,
// así que se guarda únicamente la categoría y se tira el resto.
type entradaCanal struct {
	ID         string   `json:"id"`
	Categories []string `json:"categories"`
}

func valor(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Cargar descarga e indexa los dos ficheros. Se llama UNA vez por sync, nunca
// por canal.
func (e *Enriquecedor) Cargar(ctx context.Context) error {
	var streams []entradaStream
	if err := e.descargarJSON(ctx, e.urlStreams, &streams); err != nil {
		return fmt.Errorf("iptvorg.Cargar (streams): %w", err)
	}
	var canales []entradaCanal
	if err := e.descargarJSON(ctx, e.urlChannels, &canales); err != nil {
		return fmt.Errorf("iptvorg.Cargar (channels): %w", err)
	}

	porFeed := make(map[claveFeed][]StreamExtra, len(streams))
	for _, s := range streams {
		// Sin `channel` no hay con qué ligarlo a nuestro catálogo: se descarta.
		if s.Channel == nil || *s.Channel == "" || s.URL == "" {
			continue
		}
		k := claveFeed{canal: *s.Channel, feed: valor(s.Feed)}
		porFeed[k] = append(porFeed[k], StreamExtra{
			URL:       s.URL,
			Referrer:  valor(s.Referrer),
			UserAgent: valor(s.UserAgent),
		})
	}

	categorias := make(map[string]string, len(canales))
	for _, c := range canales {
		if c.ID == "" || len(c.Categories) == 0 {
			continue
		}
		categorias[c.ID] = c.Categories[0]
	}

	e.mu.Lock()
	e.porFeed = porFeed
	e.categorias = categorias
	e.mu.Unlock()
	return nil
}

func (e *Enriquecedor) descargarJSON(ctx context.Context, url string, destino any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("construyendo peticion: %w", err)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("descargando: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// LimitReader con N+1 para distinguir "justo en el límite" de "excedido",
	// igual que hace el parser de M3U.
	limitado := &io.LimitedReader{R: resp.Body, N: maxJSONBytes + 1}
	cuerpo, err := io.ReadAll(limitado)
	if err != nil {
		return fmt.Errorf("leyendo cuerpo: %w", err)
	}
	if int64(len(cuerpo)) > maxJSONBytes {
		return fmt.Errorf("respuesta mayor que el limite de %d bytes", maxJSONBytes)
	}
	if err := json.Unmarshal(cuerpo, destino); err != nil {
		return fmt.Errorf("decodificando JSON: %w", err)
	}
	return nil
}

// Streams devuelve los streams del par exacto (canal, feed), en el orden en que
// venían en la API. nil si no hay ninguno.
func (e *Enriquecedor) Streams(canal, feed string) []StreamExtra {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.porFeed[claveFeed{canal: canal, feed: feed}]
}

// Categoria devuelve la PRIMERA categoría upstream del canal. Se usa solo para
// rellenar las categorías débiles del M3U (ver el syncer).
func (e *Enriquecedor) Categoria(canal string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	c, ok := e.categorias[canal]
	return c, ok
}

// totalIndexados es para tests: cuántos streams quedaron en el índice.
func (e *Enriquecedor) totalIndexados() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	n := 0
	for _, v := range e.porFeed {
		n += len(v)
	}
	return n
}
```

- [ ] **Paso 5: Ejecutar el test y verificar que pasa**

Run: `go test ./internal/adapters/providers/iptvorg/ -v`
Expected: PASS (5 tests).

- [ ] **Paso 6: Gates y commit**

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
git add internal/adapters/providers/iptvorg/
git commit -m "feat(iptvorg): enriquecedor que indexa streams.json y channels.json

Empareja por el par exacto (channel, feed) porque el sufijo del tvg_id ES
el feed: emparejar solo por canal le asigna al SD los streams del HD.
Los streams sin 'channel' se descartan (no hay con qué ligarlos), y de
channels.json se guarda solo la categoría: pesa 7,8 MB y no trae
'languages'.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Esquema y repositorio de streams

**Files:**
- Modify: `internal/domain/stream.go`
- Modify: `internal/adapters/db/schema.sql:82-102`
- Modify: `internal/adapters/db/db.go` (`alterMigrations`)
- Modify: `internal/adapters/db/stream_repository.go`
- Modify: `internal/ports/stream_repository.go`
- Test: `internal/adapters/db/stream_repository_test.go`

**Interfaces:**
- Consumes: nada de la Task 1.
- Produces:
  - `domain.Stream` gana `Referrer string` y `UserAgent string`.
  - `ports.StreamRepository` gana:
    - `DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error)`
    - `CabecerasPorURL(ctx context.Context, url string) (referrer, userAgent string, err error)`

- [ ] **Paso 1: Escribir el test que falla**

Añadir a `internal/adapters/db/stream_repository_test.go`:

```go
// La poda de streams no existía: DeleteStale solo estaba en canales, así que
// una URL sustituida upstream se quedaba para siempre y el failover acababa
// gastando un intento entero en historia muerta.
func TestStreamDeleteStalePodaSoloLoViejoYNoCruzaFuentes(t *testing.T) {
	ctx := context.Background()
	base := nuevaDBDePrueba(t) // helper ya existente en este fichero
	canales := NewSQLiteChannelRepository(base)
	streams := NewSQLiteStreamRepository(base)

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
	canales := NewSQLiteChannelRepository(base)
	streams := NewSQLiteStreamRepository(base)

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
```

Si `nuevaDBDePrueba` no existe con ese nombre, usar el helper que ya use el
fichero para abrir una DB temporal.

- [ ] **Paso 2: Ejecutar el test y verificar que falla**

Run: `go test ./internal/adapters/db/ -run 'TestStreamDeleteStale|TestStreamCabeceras' -v`
Expected: FAIL — no compila (`DeleteStale`/`CabecerasPorURL` no definidos,
`domain.Stream` sin `Referrer`).

- [ ] **Paso 3: Añadir los campos al dominio**

En `internal/domain/stream.go`, dentro de `type Stream struct`, tras `Protocol`:

```go
	// Referrer y UserAgent son las cabeceras que el ORIGEN exige para servir
	// este stream (las publica la API de iptv-org). Vacías = usar las de
	// siempre. El navegador no puede ponerlas —Referer y User-Agent son
	// cabeceras prohibidas para fetch/XHR—, así que solo las usan el proxy de
	// loopback y el health-checker.
	Referrer  string `json:"Referrer"`
	UserAgent string `json:"UserAgent"`
```

`domain.Stream` no tiene prueba de contrato congelado (la de 15 claves es de
`Channel`), así que añadir campos aquí es seguro.

- [ ] **Paso 4: Migrar el esquema**

En `internal/adapters/db/schema.sql`, dentro de `CREATE TABLE IF NOT EXISTS
streams`, antes de `created_at`:

```sql
    -- Cabeceras que el origen exige (API de iptv-org). '' = usar las de siempre.
    referrer     TEXT    NOT NULL DEFAULT '',
    user_agent   TEXT    NOT NULL DEFAULT '',
    -- Frontera de la poda: los streams que no aparecen en un sync se borran.
    last_seen_at INTEGER NOT NULL DEFAULT 0,
```

Y añadir el índice al final del bloque de índices de `streams`:

```sql
CREATE INDEX IF NOT EXISTS idx_streams_url ON streams(url);
```

En `internal/adapters/db/db.go`, dentro de `alterMigrations`, al final del slice
`alters`:

```go
		// Cabeceras por stream y frontera de poda (spec de mirrors, 2026-09-04).
		"ALTER TABLE streams ADD COLUMN referrer TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE streams ADD COLUMN user_agent TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE streams ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0",
```

- [ ] **Paso 5: Ampliar el puerto**

En `internal/ports/stream_repository.go`, dentro de `type StreamRepository
interface`:

```go
	// DeleteStale borra los streams de la fuente cuyo last_seen_at sea anterior
	// a `before`. Mismo scoping por fuente que la poda de canales: la tabla
	// streams no tiene provider_id, así que se resuelve por su canal.
	DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error)

	// CabecerasPorURL devuelve las cabeceras que el origen exige para esa URL.
	// Una URL desconocida devuelve cadenas vacías y error nil: no es un fallo,
	// es "usa las de siempre".
	CabecerasPorURL(ctx context.Context, url string) (referrer, userAgent string, err error)
```

Asegurar que `time` está importado.

**OJO — esto rompe los dobles de test.** Todo implementador de
`ports.StreamRepository` debe ganar los dos métodos, incluido
`fakeStreamRepo` en `internal/services/syncer_test.go`. Añadirle:

```go
type deleteStaleStreamCall struct {
	providerID string
	before     time.Time
}

func (f *fakeStreamRepo) DeleteStale(_ context.Context, providerID string, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.staleStreams = append(f.staleStreams, deleteStaleStreamCall{providerID: providerID, before: before})
	return 0, nil
}

func (f *fakeStreamRepo) CabecerasPorURL(context.Context, string) (string, string, error) {
	return "", "", nil
}

func (f *fakeStreamRepo) staleStreamCalls() []deleteStaleStreamCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]deleteStaleStreamCall(nil), f.staleStreams...)
}
```

y el campo `staleStreams []deleteStaleStreamCall` al struct.

- [ ] **Paso 6: Implementar en el repositorio**

En `internal/adapters/db/stream_repository.go`, sustituir `upsertStreamSQL`:

```go
// El upsert preserva latency_ms/is_alive/last_checked: son resultado del
// health-check, no del sync, y un re-sync no debe borrarlos. last_seen_at SÍ se
// refresca en cada sync: es la frontera que usa DeleteStale.
const upsertStreamSQL = `
	INSERT INTO streams (id, channel_id, url, protocol, referrer, user_agent,
	                     latency_ms, is_alive, last_checked, last_seen_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, NULL, 0, NULL, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		channel_id   = excluded.channel_id,
		url          = excluded.url,
		protocol     = excluded.protocol,
		referrer     = excluded.referrer,
		user_agent   = excluded.user_agent,
		last_seen_at = excluded.last_seen_at,
		updated_at   = excluded.updated_at`
```

Actualizar las dos llamadas que lo usan (`Save` y `SaveBatch`) para pasar los
argumentos nuevos, en este orden:

```go
	s.ID, string(s.ChannelID), s.URL, string(s.Protocol), s.Referrer, s.UserAgent, now, now, now,
```

Y añadir los dos métodos:

```go
// DeleteStale borra los streams de la fuente que no aparecieron en este sync.
// streams no tiene provider_id, así que el scoping va por el canal — igual de
// estricto: la poda de una fuente nunca toca los streams de otra.
func (r *SQLiteStreamRepository) DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM streams
		WHERE last_seen_at < ?
		  AND channel_id IN (SELECT id FROM channels WHERE provider_id = ?)`,
		before.Unix(), providerID)
	if err != nil {
		return 0, fmt.Errorf("db.Stream.DeleteStale (provider=%s): %w", providerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("db.Stream.DeleteStale (RowsAffected): %w", err)
	}
	return n, nil
}

// CabecerasPorURL busca las cabeceras del stream con esa URL. Una URL que no
// está en el catálogo devuelve vacías sin error: significa "usa las de siempre".
func (r *SQLiteStreamRepository) CabecerasPorURL(ctx context.Context, url string) (string, string, error) {
	var referrer, userAgent string
	err := r.db.QueryRowContext(ctx,
		"SELECT referrer, user_agent FROM streams WHERE url = ? LIMIT 1", url).
		Scan(&referrer, &userAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("db.Stream.CabecerasPorURL: %w", err)
	}
	return referrer, userAgent, nil
}
```

Asegurar que `errors` y `database/sql` están importados.

- [ ] **Paso 7: Ejecutar los tests y verificar que pasan**

Run: `go test ./internal/adapters/db/ -run 'TestStream' -v`
Expected: PASS.

- [ ] **Paso 8: Gates y commit**

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
git add internal/domain/stream.go internal/adapters/db/schema.sql internal/adapters/db/db.go internal/adapters/db/stream_repository.go internal/adapters/db/stream_repository_test.go internal/ports/stream_repository.go
git commit -m "feat(db): poda de streams y cabeceras por stream

DeleteStale solo existía para canales, así que una URL sustituida
upstream se quedaba para siempre: los 254 canales que hoy tienen 2+
mirrors son justo eso, restos sin validar. Con los mirrors ya
intencionados, cada resto muerto le cuesta al usuario un intento entero.

referrer/user_agent se guardan por stream para que el proxy y el
health-checker puedan mandarlas: el navegador no puede (son cabeceras
prohibidas para fetch/XHR).

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: El proveedor opensource acepta el enriquecedor

**Files:**
- Modify: `internal/adapters/providers/opensource/provider.go`
- Modify: `internal/ports/provider_port.go`
- Test: `internal/adapters/providers/opensource/provider_test.go`

**Interfaces:**
- Consumes de Task 1: `iptvorg.StreamExtra`, `iptvorg.SepararTvgID`.
- Produces:
  - `type Enriquecedor interface { Streams(canal, feed string) []iptvorg.StreamExtra; Categoria(canal string) (string, bool) }`
  - `func WithEnriquecedor(e Enriquecedor) Option`
  - `func (p *Provider) GetStreamsDeCanal(ctx context.Context, channelID domain.ChannelID) ([]iptvorg.StreamExtra, error)`
  - `func (p *Provider) CategoriaDe(tvgID string) (string, bool)`

- [ ] **Paso 1: Escribir el test que falla**

Añadir a `internal/adapters/providers/opensource/provider_test.go`:

```go
// enriquecedorFalso permite probar la fusión sin salir a la red.
type enriquecedorFalso struct {
	streams    map[string][]iptvorg.StreamExtra // clave "canal|feed"
	categorias map[string]string
}

func (e *enriquecedorFalso) Streams(canal, feed string) []iptvorg.StreamExtra {
	return e.streams[canal+"|"+feed]
}
func (e *enriquecedorFalso) Categoria(canal string) (string, bool) {
	c, ok := e.categorias[canal]
	return c, ok
}

const m3uUnCanal = `#EXTM3U
#EXTINF:-1 tvg-id="AndTV.in@HD" tvg-logo="l.png" group-title="General",AndTV HD
https://delm3u.example/x.m3u8
`

// SIN enriquecedor el comportamiento es el de siempre: exactamente un stream.
// Es la prueba que protege a las fuentes bring-your-own, que no tienen API.
func TestGetStreamsDeCanalSinEnriquecedor(t *testing.T) {
	p := providerConM3U(t, m3uUnCanal) // helper: sirve el M3U y llama a GetLiveChannels
	got, err := p.GetStreamsDeCanal(context.Background(), "opensource-AndTV HD")
	if err != nil {
		t.Fatalf("GetStreamsDeCanal: %v", err)
	}
	if len(got) != 1 || got[0].URL != "https://delm3u.example/x.m3u8" {
		t.Errorf("got %+v, quiero solo la URL del M3U", got)
	}
}

// Con enriquecedor: la URL del M3U va PRIMERA y los mirrors detrás, sin
// duplicar la que ya venía.
func TestGetStreamsDeCanalOrdenYDeduplicacion(t *testing.T) {
	e := &enriquecedorFalso{streams: map[string][]iptvorg.StreamExtra{
		"AndTV.in|HD": {
			{URL: "https://delm3u.example/x.m3u8"}, // duplicada a propósito
			{URL: "https://mirror.example/x.m3u8", Referrer: "https://ref/"},
		},
	}}
	p := providerConM3UYOpts(t, m3uUnCanal, WithEnriquecedor(e))

	got, err := p.GetStreamsDeCanal(context.Background(), "opensource-AndTV HD")
	if err != nil {
		t.Fatalf("GetStreamsDeCanal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d streams, quiero 2 (la duplicada se colapsa): %+v", len(got), got)
	}
	if got[0].URL != "https://delm3u.example/x.m3u8" {
		t.Errorf("got[0] = %q, la del M3U va primera", got[0].URL)
	}
	if got[1].URL != "https://mirror.example/x.m3u8" || got[1].Referrer != "https://ref/" {
		t.Errorf("got[1] = %+v, quiero el mirror con su referrer", got[1])
	}
}

// El feed discrimina también aquí: un canal @HD no recibe los streams del SD.
func TestGetStreamsDeCanalRespetaElFeed(t *testing.T) {
	e := &enriquecedorFalso{streams: map[string][]iptvorg.StreamExtra{
		"AndTV.in|SD": {{URL: "https://sd.example/x.m3u8"}},
	}}
	p := providerConM3UYOpts(t, m3uUnCanal, WithEnriquecedor(e))

	got, err := p.GetStreamsDeCanal(context.Background(), "opensource-AndTV HD")
	if err != nil {
		t.Fatalf("GetStreamsDeCanal: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %+v, el canal HD no debe heredar el stream SD", got)
	}
}
```

Si no existen helpers como `providerConM3U`, crearlos en el fichero de test
levantando un `httptest.Server` que sirva la cadena y llamando a
`GetLiveChannels` antes de devolver el provider.

- [ ] **Paso 2: Ejecutar el test y verificar que falla**

Run: `go test ./internal/adapters/providers/opensource/ -run TestGetStreamsDeCanal -v`
Expected: FAIL — `undefined: WithEnriquecedor`, `GetStreamsDeCanal`.

- [ ] **Paso 3: Implementar en el provider**

En `internal/adapters/providers/opensource/provider.go`:

```go
// Enriquecedor aporta lo que el M3U no trae. Es OPCIONAL: sin él (el caso de un
// M3U subido por el usuario, que no tiene API detrás) el provider se comporta
// exactamente igual que siempre.
type Enriquecedor interface {
	Streams(canal, feed string) []iptvorg.StreamExtra
	Categoria(canal string) (string, bool)
}

// WithEnriquecedor conecta la API de iptv-org. Solo debe usarse cuando la
// fuente ES iptv-org: para cualquier otro M3U los identificadores no casan.
func WithEnriquecedor(e Enriquecedor) Option {
	return func(p *Provider) {
		p.enriquecedor = e
	}
}
```

Añadir el campo al struct `Provider`, junto a `allowedFileDir`:

```go
	// enriquecedor es nil salvo que la fuente sea iptv-org.
	enriquecedor Enriquecedor
```

Guardar también el `tvg_id` por canal durante el parseo, para poder consultar la
API. Junto a `streamURLs`, añadir al struct:

```go
	// tvgIDs guarda el tvg-id de cada canal del último GetLiveChannels: es la
	// clave con la que se consulta la API. Protegido por mu.
	tvgIDs map[domain.ChannelID]string
```

Inicializarlo en `NewProvider` (`tvgIDs: make(map[domain.ChannelID]string)`),
poblarlo en el mismo sitio donde se rellena `streamURLs` tras el parseo, y añadir
un campo `tvgIDs` a `m3uParseResult` que `parseM3UStream` rellene con
`currentChannel.TvgID` al cerrar cada canal.

Y los métodos nuevos:

```go
// GetStreamsDeCanal devuelve la URL del M3U PRIMERO y, si hay enriquecedor, los
// mirrors de la API detrás, sin duplicados. El orden de inserción solo decide el
// arranque en frío: a partir de ahí manda la salud (FindMirrorsByChannelID).
func (p *Provider) GetStreamsDeCanal(_ context.Context, channelID domain.ChannelID) ([]iptvorg.StreamExtra, error) {
	p.mu.RLock()
	url, ok := p.streamURLs[channelID]
	tvgID := p.tvgIDs[channelID]
	enr := p.enriquecedor
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("opensource.GetStreamsDeCanal: canal %s no encontrado (sync pendiente?)", channelID)
	}

	salida := []iptvorg.StreamExtra{{URL: url}}
	if enr == nil || tvgID == "" {
		return salida, nil
	}

	vistas := map[string]bool{url: true}
	canal, feed := iptvorg.SepararTvgID(tvgID)
	for _, s := range enr.Streams(canal, feed) {
		if s.URL == "" || vistas[s.URL] {
			continue
		}
		vistas[s.URL] = true
		salida = append(salida, s)
	}
	return salida, nil
}

// CategoriaDe devuelve la categoría upstream del canal cuyo tvg_id se pasa.
// Sin enriquecedor, o sin tvg_id, no hay categoría.
func (p *Provider) CategoriaDe(tvgID string) (string, bool) {
	p.mu.RLock()
	enr := p.enriquecedor
	p.mu.RUnlock()
	if enr == nil || tvgID == "" {
		return "", false
	}
	canal, _ := iptvorg.SepararTvgID(tvgID)
	return enr.Categoria(canal)
}
```

En `internal/ports/provider_port.go`, añadir al interface:

```go
	// GetStreamsDeCanal devuelve la url principal y, si la fuente tiene API
	// detrás, los mirrors. Siempre al menos un elemento si el canal existe.
	GetStreamsDeCanal(ctx context.Context, channelID domain.ChannelID) ([]iptvorg.StreamExtra, error)
```

con el import de `iptvorg`.

- [ ] **Paso 4: Ejecutar los tests y verificar que pasan**

Run: `go test ./internal/adapters/providers/... -v`
Expected: PASS.

- [ ] **Paso 5: Gates y commit**

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
git add internal/adapters/providers/opensource/ internal/ports/provider_port.go
git commit -m "feat(opensource): GetStreamsDeCanal con mirrors del enriquecedor

El provider guardaba UNA url por canal en un map[ChannelID]string, así
que las entradas repetidas se pisaban: de ahí que el 96% de los canales
tenga un solo mirror. Ahora devuelve la del M3U primero y los mirrors de
la API detrás, deduplicados y respetando el feed.

El enriquecedor es opcional: sin él el comportamiento es idéntico al de
siempre, que es lo que necesitan los M3U que sube el usuario.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: El syncer guarda N streams y poda los viejos

**Files:**
- Modify: `internal/services/syncer.go:360-420`
- Test: `internal/services/syncer_test.go`

**Interfaces:**
- Consumes de Task 1-3: `iptvorg.NuevoEnriquecedor`, `opensource.WithEnriquecedor`,
  `GetStreamsDeCanal`, `streams.DeleteStale`.
- Produces: nada nuevo hacia fuera.

**Nota de diseño — por qué el enriquecedor se inyecta por `Config`:** el syncer
construye el provider él mismo a partir de `fuente.URL`, y la detección de
iptv-org va por host. Un `httptest.Server` nunca tiene ese host, así que **sin
inyección los tests no podrían ejercitar el camino enriquecido en absoluto**. Por
eso `Config` gana una fábrica que los tests sustituyen por un doble, y cuyo
default es el de producción.

- [ ] **Paso 1: Escribir el test que falla**

Añadir a `internal/services/syncer_test.go`:

```go
const m3uMirrors = `#EXTM3U
#EXTINF:-1 tvg-id="AndTV.in@HD" group-title="General",AndTV HD
https://delm3u.example/x.m3u8
`

type enriquecedorFalso struct {
	streams    map[string][]iptvorg.StreamExtra // clave "canal|feed"
	categorias map[string]string
}

func (e *enriquecedorFalso) Streams(canal, feed string) []iptvorg.StreamExtra {
	return e.streams[canal+"|"+feed]
}

func (e *enriquecedorFalso) Categoria(canal string) (string, bool) {
	c, ok := e.categorias[canal]
	return c, ok
}

func servidorM3U(t *testing.T, cuerpo string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, cuerpo)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Un canal con mirrors produce VARIAS filas de stream, en orden y con sus
// cabeceras. Antes solo se guardaba una: de ahí el 96 % de canales con un
// único mirror.
func TestSyncGuardaTodosLosMirrors(t *testing.T) {
	srv := servidorM3U(t, m3uMirrors)
	enr := &enriquecedorFalso{streams: map[string][]iptvorg.StreamExtra{
		"AndTV.in|HD": {
			{URL: "https://delm3u.example/x.m3u8"}, // duplicada a propósito
			{URL: "https://mirror.example/x.m3u8", Referrer: "https://ref/", UserAgent: "UA/1"},
		},
	}}

	sources := &fakeSourceRepo{fuentes: []ports.Source{
		{ID: "src-a", URL: srv.URL, Kind: "url", IsActive: true},
	}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, nil, nil, "", Config{
		NuevoEnriquecedor: func(string) opensource.Enriquecedor { return enr },
	})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	got := stRepo.lastBatch()
	if len(got) != 2 {
		t.Fatalf("guardados %d streams, quiero 2 (la del M3U + el mirror, sin duplicar la repetida): %+v", len(got), got)
	}
	if got[0].URL != "https://delm3u.example/x.m3u8" {
		t.Errorf("got[0].URL = %q, la del M3U va primera", got[0].URL)
	}
	if got[1].URL != "https://mirror.example/x.m3u8" {
		t.Errorf("got[1].URL = %q, quiero el mirror", got[1].URL)
	}
	if got[1].Referrer != "https://ref/" || got[1].UserAgent != "UA/1" {
		t.Errorf("got[1] perdió las cabeceras: %+v", got[1])
	}
}

// Sin enriquecedor (fuente que no es iptv-org, o API caída) el sync NO falla:
// guarda el catálogo del M3U con su único stream, como hasta ahora.
func TestSyncSinEnriquecedorSigueFuncionando(t *testing.T) {
	srv := servidorM3U(t, m3uMirrors)
	sources := &fakeSourceRepo{fuentes: []ports.Source{
		{ID: "src-a", URL: srv.URL, Kind: "url", IsActive: true},
	}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, nil, nil, "", Config{
		NuevoEnriquecedor: func(string) opensource.Enriquecedor { return nil },
	})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce sin enriquecedor debe funcionar: %v", err)
	}

	got := stRepo.lastBatch()
	if len(got) != 1 || got[0].URL != "https://delm3u.example/x.m3u8" {
		t.Errorf("got %+v, quiero solo la URL del M3U", got)
	}
}

// La poda de streams corre por fuente, igual que la de canales.
func TestSyncPodaLosStreamsQueYaNoAparecen(t *testing.T) {
	srv := servidorM3U(t, m3uMirrors)
	sources := &fakeSourceRepo{fuentes: []ports.Source{
		{ID: "src-a", URL: srv.URL, Kind: "url", IsActive: true},
	}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, nil, nil, "", Config{
		NuevoEnriquecedor: func(string) opensource.Enriquecedor { return nil },
	})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	llamadas := stRepo.staleStreamCalls()
	if len(llamadas) != 1 {
		t.Fatalf("llamadas a DeleteStale de streams = %d, quiero 1", len(llamadas))
	}
	if llamadas[0].providerID != "src-a" {
		t.Errorf("podado con providerID %q, quiero src-a: la poda NUNCA cruza fuentes", llamadas[0].providerID)
	}
}
```

Imports nuevos en el fichero de test: `io`, `net/http`, `net/http/httptest`,
`github.com/gdberysan/open-tv/internal/adapters/providers/iptvorg`,
`github.com/gdberysan/open-tv/internal/adapters/providers/opensource`.

- [ ] **Paso 2: Ejecutar y verificar que falla**

Run: `go test ./internal/services/ -run TestSync -v`
Expected: FAIL.

- [ ] **Paso 3: Implementar en el syncer**

Primero, la fábrica inyectable. En `Config`:

```go
	// NuevoEnriquecedor construye el enriquecedor de una fuente, o devuelve nil
	// si esa fuente no tiene API detrás. Inyectable porque el syncer construye
	// el provider él solo y la detección va por host: sin esto los tests no
	// podrían ejercitar el camino enriquecido (un httptest.Server nunca es
	// iptv-org.github.io). Default: enriquecedorDeProduccion.
	NuevoEnriquecedor func(fuenteURL string) opensource.Enriquecedor
```

En `withDefaults`:

```go
	if c.NuevoEnriquecedor == nil {
		c.NuevoEnriquecedor = enriquecedorDeProduccion
	}
```

Y la implementación de producción, junto a `protocolFromURL`:

```go
// enriquecedorDeProduccion conecta la API de iptv-org SOLO para su lista
// oficial: es la única que comparte identificadores con ella. La decisión va por
// HOST de la URL ya parseada, no por substring.
//
// Un fallo de la API NO tumba el sync: devuelve nil y se sincroniza solo con el
// M3U. El enriquecimiento es una mejora, jamás un requisito — un corte de
// iptv-org.github.io no puede dejar al usuario sin catálogo.
func enriquecedorDeProduccion(fuenteURL string) opensource.Enriquecedor {
	u, err := url.Parse(fuenteURL)
	if err != nil || !strings.EqualFold(u.Hostname(), "iptv-org.github.io") {
		return nil
	}
	enr := iptvorg.NuevoEnriquecedor(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := enr.Cargar(ctx); err != nil {
		return nil
	}
	return enr
}
```

Y en `sincronizarFuente`, sustituir la construcción del provider:

```go
	opts := []opensource.Option{opensource.WithAllowedFileDir(s.allowedFileDir)}
	// Interface nil-safe: NuevoEnriquecedor devuelve nil para una fuente sin
	// API o si la carga falló, y WithEnriquecedor(nil) deja el provider con el
	// comportamiento de siempre.
	if enr := s.cfg.NuevoEnriquecedor(fuente.URL); enr != nil {
		opts = append(opts, opensource.WithEnriquecedor(enr))
	} else {
		s.logger.Info("Fuente sin enriquecedor, solo M3U", slog.String("fuente", fuente.ID))
	}
	provider := opensource.NewProvider(fuente.ID, fuente.URL, nil, opts...)
```

Imports nuevos en `syncer.go`: `net/url` y los dos paquetes de providers.
Comprobar cómo se llama el campo de config dentro del Syncer (`s.cfg` o
equivalente) y usar ese nombre.

Sustituir el bucle que arma `streams`:

```go
	streams := make([]domain.Stream, 0, len(channels))
	for _, ch := range channels {
		extras, err := provider.GetStreamsDeCanal(ctx, ch.ID)
		if err != nil {
			s.logger.Warn("Canal sin stream URL, omitido",
				slog.String("fuente", fuente.ID), slog.String("channel", string(ch.ID)))
			continue
		}
		for _, ex := range extras {
			streams = append(streams, domain.Stream{
				ID:        streamID(ch.ID, ex.URL),
				ChannelID: ch.ID,
				URL:       ex.URL,
				Protocol:  protocolFromURL(ex.URL),
				Referrer:  ex.Referrer,
				UserAgent: ex.UserAgent,
			})
		}
	}
```

Y añadir la poda de streams **antes** de la de canales:

```go
	// Los streams se podan ANTES que los canales: así un canal que se va no
	// deja streams huérfanos. Primera pasada tras el despliegue: se limpian los
	// ~254 restos históricos, así que el conteo bajará antes de subir.
	streamsPodados, err := s.streams.DeleteStale(ctx, fuente.ID, inicio)
	if err != nil {
		return fmt.Errorf("services.sincronizarFuente (%s) poda de streams: %w", fuente.ID, err)
	}
	s.logger.Info("Streams podados", slog.String("fuente", fuente.ID), slog.Int64("n", streamsPodados))
```

- [ ] **Paso 4: Ejecutar y verificar que pasan**

Run: `go test ./internal/services/ -v`
Expected: PASS.

- [ ] **Paso 5: Gates y commit**

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
git add internal/services/syncer.go internal/services/syncer_test.go
git commit -m "feat(syncer): guardar todos los mirrors y podar los que se van

2.112 canales pasan de tener cero alternativas a tener al menos una
(de 254 a 2.112, 8,3x), con 4.180 filas de stream extra. La poda entra a
la vez porque sin ella los mirrors muertos se acumulan para siempre y
cada uno cuesta un intento entero al usuario.

Un fallo de la API de iptv-org NO tumba el sync: se sincroniza solo con
el M3U, como hasta ahora.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Categorías desde channels.json

**Files:**
- Modify: `internal/services/syncer.go` (fusión de categoría)
- Test: `internal/services/syncer_test.go`

**Interfaces:**
- Consumes de Task 3: `provider.CategoriaDe(tvgID string) (string, bool)`.

- [ ] **Paso 1: Escribir el test que falla**

```go
const m3uCategorias = `#EXTM3U
#EXTINF:-1 tvg-id="Uno.in@HD" group-title="General",Uno
https://uno.example/x.m3u8
#EXTINF:-1 tvg-id="Dos.in@HD" group-title="",Dos
https://dos.example/x.m3u8
#EXTINF:-1 tvg-id="Tres.in@HD" group-title="News",Tres
https://tres.example/x.m3u8
`

// Se rellena SOLO lo débil: vacío, «General» o «Undefined» (los cajones de
// sastre). Una categoría real del M3U gana siempre — si no, este cambio
// reescribiría de golpe la taxonomía de 12.000 canales.
func TestSyncRellenaCategoriasDebiles(t *testing.T) {
	srv := servidorM3U(t, m3uCategorias)
	enr := &enriquecedorFalso{categorias: map[string]string{
		"Uno.in": "movies", "Dos.in": "movies", "Tres.in": "movies",
	}}

	sources := &fakeSourceRepo{fuentes: []ports.Source{
		{ID: "src-a", URL: srv.URL, Kind: "url", IsActive: true},
	}}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := NewSyncer(nil, sources, chRepo, stRepo, nil, nil, "", Config{
		NuevoEnriquecedor: func(string) opensource.Enriquecedor { return enr },
	})
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	porNombre := map[string]string{}
	for _, c := range chRepo.lastBatch() { // añadir este helper si no existe
		porNombre[c.Name] = c.CategoryID
	}

	if got := porNombre["Uno"]; got != "movies" {
		t.Errorf("«General» quedó en %q, quiero movies", got)
	}
	if got := porNombre["Dos"]; got != "movies" {
		t.Errorf("categoría vacía quedó en %q, quiero movies", got)
	}
	if got := porNombre["Tres"]; got != "News" {
		t.Errorf("«News» quedó en %q: una categoría real del M3U NO se pisa", got)
	}
}
```

Si `fakeChannelRepo` no tiene `lastBatch()`, añadirlo con el mismo patrón que
`fakeStreamRepo.lastBatch()`.

- [ ] **Paso 2: Ejecutar y verificar que falla**

Run: `go test ./internal/services/ -run TestSyncRellenaCategorias -v`
Expected: FAIL.

- [ ] **Paso 3: Implementar**

En `sincronizarFuente`, entre `GetLiveChannels` y `channels.SaveBatch`:

```go
	// Categorías: 2.606 canales caen hoy en «General», el cajón de sastre de
	// iptv-org. Se rellena SOLO lo débil; una categoría real del M3U gana.
	// El país NO se toca: los 1.683 canales sin country_code son exactamente
	// los que no tienen tvg_id, así que no hay clave con la que unirlos (ver
	// §2.2 del spec) — no reintentarlo.
	for i := range channels {
		if !categoriaDebil(channels[i].CategoryID) {
			continue
		}
		if cat, ok := provider.CategoriaDe(channels[i].TvgID); ok {
			channels[i].CategoryID = cat
		}
	}
```

Y el helper:

```go
// categoriaDebil marca las categorías que no dicen nada y conviene sustituir.
func categoriaDebil(c string) bool {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "", "general", "undefined":
		return true
	default:
		return false
	}
}
```

- [ ] **Paso 4: Ejecutar y verificar que pasan**

Run: `go test ./internal/services/ -v`
Expected: PASS.

- [ ] **Paso 5: Gates y commit**

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
git add internal/services/syncer.go internal/services/syncer_test.go
git commit -m "feat(syncer): rellenar las categorías débiles desde channels.json

2.606 canales caen hoy en «General». Se rellena solo lo vacío o
«General»/«Undefined»; una categoría real del M3U gana siempre.

El país NO se toca a propósito: los 1.683 canales sin country_code son
EXACTAMENTE los que no tienen tvg_id, así que channels.json no puede
resolver ninguno (medido: 0 de 1.683, y por nombre solo 81 únicos con
109 ambiguos). Está en el spec para que nadie lo reintente.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: El proxy manda las cabeceras del stream

**Files:**
- Modify: `internal/proxy/handler.go`
- Modify: `internal/api/router.go:147`
- Test: `internal/proxy/handler_test.go`

**Interfaces:**
- Consumes de Task 2: `streams.CabecerasPorURL`.
- Produces: `type BuscadorCabeceras func(ctx context.Context, url string) (referrer, userAgent string)`
  y `proxy.NewHandler(prefijo string, permitirDestinosPrivados bool, cabeceras BuscadorCabeceras)`.

- [ ] **Paso 1: Escribir el test que falla**

```go
// Un origen con protección de hotlink solo sirve si le llega el Referer que
// espera. El navegador NO puede ponerlo (Referer es cabecera prohibida para
// fetch/XHR), así que es el proxy quien tiene que hacerlo.
func TestProxyMandaLasCabecerasDelStream(t *testing.T) {
	var gotRef, gotUA string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRef = r.Header.Get("Referer")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("datos"))
	}))
	defer origen.Close()

	h := NewHandler("/proxy/hls?u=", true, func(_ context.Context, _ string) (string, string) {
		return "https://ref.example/", "UA-Especial/1"
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, quiero 200", rec.Code)
	}
	if gotRef != "https://ref.example/" {
		t.Errorf("Referer = %q, quiero el del stream", gotRef)
	}
	if gotUA != "UA-Especial/1" {
		t.Errorf("User-Agent = %q, quiero el del stream", gotUA)
	}
}

// Sin cabeceras propias se manda el User-Agent de siempre y ningún Referer:
// mandar un Referer inventado podría romper orígenes que hoy funcionan.
func TestProxySinCabecerasUsaElDeSiempre(t *testing.T) {
	var gotRef, gotUA string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRef = r.Header.Get("Referer")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("datos"))
	}))
	defer origen.Close()

	h := NewHandler("/proxy/hls?u=", true, func(_ context.Context, _ string) (string, string) {
		return "", ""
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil)
	h.ServeHTTP(rec, req)

	if gotUA != userAgent {
		t.Errorf("User-Agent = %q, quiero el de siempre (%q)", gotUA, userAgent)
	}
	if gotRef != "" {
		t.Errorf("Referer = %q, quiero ninguno", gotRef)
	}
}

// Un buscador nil (o un fallo de DB, que se traduce en cadenas vacías) no puede
// romper el proxy: se cae al comportamiento anterior.
func TestProxyBuscadorNil(t *testing.T) {
	var gotUA string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("datos"))
	}))
	defer origen.Close()

	h := NewHandler("/proxy/hls?u=", true, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, quiero 200: un buscador nil no puede romper el relay", rec.Code)
	}
	if gotUA != userAgent {
		t.Errorf("User-Agent = %q, quiero el de siempre", gotUA)
	}
}
```

`NewHandler` se llama aquí con `permitirDestinosPrivados = true` porque los
`httptest.Server` viven en 127.0.0.1, que en producción es justo lo que la guarda
SSRF bloquea. Es el mismo patrón que ya usan los tests existentes del proxy.

- [ ] **Paso 2: Ejecutar y verificar que falla**

Run: `go test ./internal/proxy/ -run TestProxy -v`
Expected: FAIL — `NewHandler` no acepta tres argumentos.

- [ ] **Paso 3: Implementar**

En `internal/proxy/handler.go`:

```go
// BuscadorCabeceras resuelve las cabeceras que el ORIGEN exige para una URL.
// Devuelve vacías si no hay ninguna o si la URL no está en el catálogo. Nunca
// devuelve error: un fallo de lectura se traduce en "usa las de siempre", que
// es el comportamiento anterior y siempre es seguro.
type BuscadorCabeceras func(ctx context.Context, url string) (referrer, userAgent string)
```

Añadir el campo `cabeceras BuscadorCabeceras` al struct `Handler`, aceptarlo en
`NewHandler` y usarlo en `ServeHTTP`, sustituyendo la línea del User-Agent fijo:

```go
	// Cabeceras del ORIGEN, no del cliente: las del navegador siguen sin
	// viajar. Si el stream no declara ninguna, se usan las de siempre.
	ua := userAgent
	if h.cabeceras != nil {
		ref, uaStream := h.cabeceras(ctx, destino.String())
		if uaStream != "" {
			ua = uaStream
		}
		if ref != "" {
			req.Header.Set("Referer", ref)
		}
	}
	req.Header.Set("User-Agent", ua)
```

**No se toca nada más:** `destinoPrivado`, `controlConexion`, `checkRedirect`,
el tope de tamaño y el filtro `Sec-Fetch-Site` quedan exactamente igual.

En `internal/api/router.go:147`:

```go
		ph := proxy.NewHandler(RutaProxy, opts.PermitirDestinosPrivados,
			func(ctx context.Context, u string) (string, string) {
				ref, ua, err := streamRepo.CabecerasPorURL(ctx, u)
				if err != nil {
					// Un fallo de lectura no puede tumbar la reproducción:
					// se cae a las cabeceras de siempre.
					return "", ""
				}
				return ref, ua
			})
```

pasando `streamRepo` a la construcción del router si aún no está disponible ahí.

- [ ] **Paso 4: Ejecutar y verificar que pasan**

Run: `go test ./internal/proxy/ ./internal/api/ -v`
Expected: PASS.

- [ ] **Paso 5: Gates y commit**

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
git add internal/proxy/handler.go internal/proxy/handler_test.go internal/api/router.go
git commit -m "feat(proxy): mandar el referrer y user-agent que exige cada origen

1.046 streams del catálogo declaran cabeceras propias. Son parte de los
403 que NO son geo: protección de hotlink y filtros de agente. El
navegador no puede ponerlas (Referer y User-Agent son cabeceras
prohibidas para fetch/XHR), así que el único sitio que puede es este
proxy, que ya construye una petición nueva desde cero.

La guarda SSRF no se toca: destinoPrivado, controlConexion,
checkRedirect, el tope de tamaño y el filtro Sec-Fetch-Site quedan igual.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: El health-checker respeta las cabeceras

**Files:**
- Modify: `internal/adapters/validator/checker.go`
- Modify: `internal/adapters/validator/worker.go`
- Test: `internal/adapters/validator/checker_test.go`

**Interfaces:**
- Consumes de Task 2: `domain.Stream.Referrer`, `domain.Stream.UserAgent`.
- Produces: `func (c *Checker) CheckConCabeceras(ctx context.Context, url, referrer, userAgent string) StreamResult`.

- [ ] **Paso 1: Escribir el test que falla**

```go
// Sin las cabeceras del origen, un stream con hotlink se marcaba muerto
// aunque funcionase perfectamente con ellas.
func TestCheckConCabeceras(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://ref.example/" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = w.Write([]byte("#EXTM3U\n"))
	}))
	defer srv.Close()

	c := NewChecker(nil, 5*time.Second)

	sin := c.Check(context.Background(), srv.URL+"/x.m3u8")
	if sin.StatusCode != http.StatusForbidden {
		t.Errorf("sin cabeceras: status %d, quiero 403", sin.StatusCode)
	}

	con := c.CheckConCabeceras(context.Background(), srv.URL+"/x.m3u8", "https://ref.example/", "")
	if con.StatusCode != http.StatusOK {
		t.Errorf("con cabeceras: status %d, quiero 200", con.StatusCode)
	}
}
```

- [ ] **Paso 2: Ejecutar y verificar que falla**

Run: `go test ./internal/adapters/validator/ -run TestCheckConCabeceras -v`
Expected: FAIL — `undefined: CheckConCabeceras`.

- [ ] **Paso 3: Implementar**

En `checker.go`, renombrar el cuerpo actual de `Check` a `CheckConCabeceras(ctx,
url, referrer, userAgent string)`, sustituyendo las dos llamadas
`req.Header.Set("User-Agent", userAgent)` por:

```go
	ua := userAgentPorDefecto
	if userAgentStream != "" {
		ua = userAgentStream
	}
	req.Header.Set("User-Agent", ua)
	if referrer != "" {
		req.Header.Set("Referer", referrer)
	}
```

(renombrando la constante `userAgent` a `userAgentPorDefecto` para no chocar con
el parámetro), y dejar `Check` como envoltorio compatible:

```go
// Check mantiene la firma de siempre para los call sites que no tienen
// cabeceras a mano.
func (c *Checker) Check(ctx context.Context, url string) StreamResult {
	return c.CheckConCabeceras(ctx, url, "", "")
}
```

En `worker.go`, donde se recorre `FindAll` y se llama al checker, pasar
`s.Referrer` y `s.UserAgent` del stream.

- [ ] **Paso 4: Ejecutar y verificar que pasan**

Run: `go test ./internal/adapters/validator/ -v`
Expected: PASS.

- [ ] **Paso 5: Gates finales y commit**

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./... && golangci-lint run ./... && go run ./tools/scrubcheck
cd web && npm run check && npm test && npm run build && cd ..
git status --porcelain mobile/   # DEBE estar vacío
git add internal/adapters/validator/
git commit -m "feat(validator): usar las cabeceras del stream en el health-check

Sin ellas, un origen con protección de hotlink se marcaba muerto aunque
funcionara perfectamente, y el canal quedaba al final de la cola de
mirrors por una razón falsa.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Verificación final (tras la última tarea)

- [ ] `go test -race -count=1 ./...` en verde.
- [ ] `golangci-lint run ./...` → 0 issues.
- [ ] `go run ./tools/scrubcheck` limpio.
- [ ] `cd web && npm run check && npm test && npm run build` en verde.
- [ ] `git status --porcelain mobile/` **vacío**.
- [ ] `go test ./internal/domain/ -run TestChannel` en verde (las 15 claves).
- [ ] **Verificación real, no solo gates:** arrancar el gateway y comprobar en la
      DB que los mirrors llegaron:

```bash
go build -o open-tv ./cmd/open-tv
# tras un sync completo:
sqlite3 .devdata/iptv.db \
  "SELECT n, COUNT(*) FROM (SELECT channel_id, COUNT(*) n FROM streams GROUP BY channel_id) GROUP BY n ORDER BY n;"
```

Esperado: los canales con 2+ mirrors pasan de 254 a del orden de 2.000. Si sigue
en 254, el enriquecedor no se está conectando — comprobar `esIPTVOrg` y el aviso
«API de iptv-org no disponible» en el log.

- [ ] Comprobar las categorías:

```bash
sqlite3 .devdata/iptv.db "SELECT COUNT(*) FROM channels WHERE category_id='General';"
```

Esperado: baja de 2.606 hacia 0. Los 3.147 «Undefined» **no** deben moverse.
