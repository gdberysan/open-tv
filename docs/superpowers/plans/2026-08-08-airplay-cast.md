# Emisión por AirPlay — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emitir cualquier canal a un Apple TV o televisor con AirPlay 2, con sesión persistente: la rejilla sigue navegable y cambiar de canal no obliga a volver a elegir dispositivo.

**Architecture:** `media_kit`/libmpv se queda intacto para la reproducción local. Una capa Swift mínima expone un `AVPlayer` con `allowsExternalPlayback` y el `AVRoutePickerView` real de Apple detrás de dos canales de plataforma. Toda la decisión vive en Dart (máquina de estados) y en Go (clasificador de compatibilidad), que son las dos capas que CI sí compila.

**Tech Stack:** Go 1.22 + chi v5 · Flutter 3.44 + Riverpod 2 · Swift/AppKit + AVFoundation + AVKit · SQLite (sin cambios de esquema)

**Spec:** `docs/superpowers/specs/2026-08-08-airplay-cast-design.md`

## Global Constraints

- **Idioma:** todo el código, comentarios, mensajes de commit y texto de UI en **español**. Es la convención del repo entero.
- **CI compila en `ubuntu-latest`** (`.github/workflows/ci.yml`, ambos jobs). Ningún test Dart puede depender de un `MethodChannel` real. El código Swift no pasa por CI nunca.
- **Gates que deben quedar verdes al final de cada tarea:** `cd gateway && gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./...` · `cd mobile && flutter analyze && flutter test`
- **Cero `_` descartando errores en producción.** Errores envueltos con `fmt.Errorf("op: %w", err)`.
- **`slog` estructurado**, nunca `fmt.Println`.
- **`domain` sin imports externos.** Solo stdlib y tipos primitivos.
- **No se toca `PlaybackGuard` ni `player_screen.dart`** salvo lo que indique explícitamente la Tarea 12.
- **No se toca el esquema SQLite**, ni `domain.Channel`, ni las 14 claves congeladas de `domain/channel_test.go`.
- **`MACOSX_DEPLOYMENT_TARGET` sigue en 10.15.** `AVRoutePickerView` exige exactamente eso; no subirlo, y no usar API de macOS 11+ (`kAudioObjectPropertyElementMain` está prohibido, usar `...Master`).
- **Nombres de canal de plataforma, literales:** `dev.korven.opentv/airplay` (Method), `dev.korven.opentv/airplay/events` (Event), `dev.korven.opentv/route-picker` (PlatformView).

---

## Estructura de ficheros

| Fichero | Responsabilidad | Tarea |
|---|---|---|
| `gateway/internal/domain/airplay.go` | Clasificador puro de manifiestos HLS. Sin E/S. | 1 |
| `gateway/internal/adapters/validator/checker.go` | Clasificar el cuerpo que el fallback GET ya lee y descarta. | 2 |
| `gateway/internal/api/handlers/airplay_probe.go` | Sonda bajo demanda + caché en memoria con TTL. | 3 |
| `gateway/internal/api/handlers/channel_handler.go` | `airplay_ok` en la respuesta de `/channels/stream`. | 3 |
| `mobile/macos/Runner/AirPlay/RoutePickerFactory.swift` | `NSViewFactory` con el `AVRoutePickerView` real. | 4 |
| `mobile/macos/Runner/AirPlay/AirPlayPlugin.swift` | Registro de canales y factoría. Solo cableado. | 4, 5 |
| `mobile/macos/Runner/AirPlay/AirPlaySession.swift` | Posee el `AVPlayer`. KVO y clasificación de error. | 5 |
| `mobile/macos/Runner/AirPlay/RouteName.swift` | Nombre del dispositivo vía CoreAudio, best-effort. | 5 |
| `mobile/lib/domain/models/cast_session.dart` | Modelo puro + `CastState`. Sin imports. | 6 |
| `mobile/lib/data/airplay/airplay_platform.dart` | Interfaz + impl real. **Único** fichero que toca `MethodChannel`. | 6 |
| `mobile/lib/presentation/player/airplay_guard.dart` | Timeout de carga y fatalidad para la ruta `AVPlayer`. | 7 |
| `mobile/lib/presentation/providers/airplay_memory_provider.dart` | Memoria local de canales incompatibles. | 8 |
| `mobile/lib/presentation/providers/cast_provider.dart` | La máquina de estados. | 9 |
| `mobile/lib/presentation/widgets/airplay_button.dart` | `AppKitView` con el selector. | 10 |
| `mobile/lib/presentation/widgets/cast_bar.dart` | Barra persistente de sesión. | 11 |
| `mobile/lib/presentation/screens/home_screen.dart` | Cableado y toque condicional. | 12 |
| `mobile/lib/presentation/widgets/console_bar.dart` | Hueco para el botón AirPlay. | 12 |

---

## Tarea 1: Clasificador de compatibilidad AirPlay

**Files:**
- Create: `gateway/internal/domain/airplay.go`
- Test: `gateway/internal/domain/airplay_test.go`

**Interfaces:**
- Consumes: nada.
- Produces: `domain.AirplaySupport` (`AirplayUnknown`/`AirplayNo`/`AirplayOK`) y `domain.ClassifyManifest(url, body string) AirplaySupport`. Lo usan las tareas 2 y 3.

- [ ] **Step 1: Escribir el test que falla**

Crear `gateway/internal/domain/airplay_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
)

const masterSoportado = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.4d4028,mp4a.40.2"
720p.m3u8
`

const masterNoSoportado = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,CODECS="mp4v.20.9,mp4a.40.2"
low.m3u8
`

const masterMixto = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,CODECS="mp4v.20.9,mp4a.40.2"
low.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.64001f,mp4a.40.2"
high.m3u8
`

const masterSinCodecs = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=2000000
720p.m3u8
`

// Una variante declarada y otra sin declarar: no se puede afirmar que el canal
// sea incompatible, porque la no declarada podría reproducirse.
const masterParcial = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=800000,CODECS="mp4v.20.9"
low.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000000
high.m3u8
`

const playlistDeMedios = `#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.000,
seg1.ts
`

const widevine = `#EXTM3U
#EXT-X-KEY:METHOD=SAMPLE-AES,KEYFORMAT="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed",URI="skd://x"
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.4d4028,mp4a.40.2"
720p.m3u8
`

const fairplay = `#EXTM3U
#EXT-X-KEY:METHOD=SAMPLE-AES,KEYFORMAT="com.apple.streamingkeydelivery",URI="skd://x"
#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS="avc1.4d4028,mp4a.40.2"
720p.m3u8
`

func TestClassifyManifest(t *testing.T) {
	casos := []struct {
		nombre string
		url    string
		body   string
		quiero domain.AirplaySupport
	}{
		{"h264+aac es reproducible", "http://x/a.m3u8", masterSoportado, domain.AirplayOK},
		{"mpeg-4 visual no lo es", "http://x/a.m3u8", masterNoSoportado, domain.AirplayNo},
		{"basta una variante buena", "http://x/a.m3u8", masterMixto, domain.AirplayOK},
		{"master sin CODECS no se puede juzgar", "http://x/a.m3u8", masterSinCodecs, domain.AirplayUnknown},
		{"una variante sin declarar impide el veredicto negativo", "http://x/a.m3u8", masterParcial, domain.AirplayUnknown},
		{"playlist de medios no declara nada", "http://x/a.m3u8", playlistDeMedios, domain.AirplayUnknown},
		{"widevine no viaja a AirPlay", "http://x/a.m3u8", widevine, domain.AirplayNo},
		{"fairplay sí es de Apple", "http://x/a.m3u8", fairplay, domain.AirplayOK},
		{"dash queda descartado por la url", "http://x/a.mpd", masterSoportado, domain.AirplayNo},
		{"rtmp queda descartado por la url", "rtmp://x/live", "", domain.AirplayNo},
		{"mmsh queda descartado por la url", "mmsh://x/live", "", domain.AirplayNo},
		{"cuerpo vacío no dice nada", "http://x/a.m3u8", "", domain.AirplayUnknown},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := domain.ClassifyManifest(c.url, c.body); got != c.quiero {
				t.Errorf("ClassifyManifest = %v, quiero %v", got, c.quiero)
			}
		})
	}
}
```

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd gateway && go test ./internal/domain/ -run TestClassifyManifest -v
```

Esperado: FAIL de compilación, `undefined: domain.ClassifyManifest`.

- [ ] **Step 3: Implementar**

Crear `gateway/internal/domain/airplay.go`:

```go
package domain

import "strings"

// AirplaySupport clasifica si un stream puede enviarse a un receptor AirPlay.
//
// Tres estados y no dos: solo el 57,5 % de los manifiestos del catálogo declara
// CODECS (medido sobre 40 streams vivos), así que "no se sabe" es el caso más
// común después de "sí". Colapsarlo en "no" marcaría como rotos miles de
// canales que funcionan.
type AirplaySupport int

const (
	AirplayUnknown AirplaySupport = iota
	AirplayNo
	AirplayOK
)

// codecsSoportados son los prefijos de RFC 6381 que AVFoundation reproduce.
// Lista de permitidos, no de prohibidos: un códec desconocido no se asume
// reproducible.
var codecsSoportados = []string{
	"avc1.", "avc3.", "hvc1.", "hev1.", "dvh1.", "dvhe.",
	"mp4a.40.", "ac-3", "ec-3", "alac",
}

// keyformatApple es el único sistema de claves que AVFoundation abre por sí
// solo. Widevine y PlayReady sobre SAMPLE-AES no viajan a AirPlay.
const keyformatApple = "com.apple.streamingkeydelivery"

// ClassifyManifest decide si un stream es reproducible por AirPlay a partir de
// su URL y del cuerpo de su manifiesto. No hace E/S: el cuerpo llega ya leído.
func ClassifyManifest(url, body string) AirplaySupport {
	if urlNoReproducible(url) {
		return AirplayNo
	}
	if cifradoNoApple(body) {
		return AirplayNo
	}

	hayVarianteSinDeclarar := false
	hayVarianteDeclarada := false

	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if !strings.HasPrefix(linea, "#EXT-X-STREAM-INF:") {
			continue
		}
		codecs, ok := atributoEntreComillas(linea, "CODECS")
		if !ok {
			hayVarianteSinDeclarar = true
			continue
		}
		hayVarianteDeclarada = true
		if varianteSoportada(codecs) {
			return AirplayOK
		}
	}

	// Ninguna variante declarada sirve, pero alguna no se declaró: no hay base
	// para afirmar que el canal es incompatible.
	if hayVarianteDeclarada && !hayVarianteSinDeclarar {
		return AirplayNo
	}
	return AirplayUnknown
}

func urlNoReproducible(url string) bool {
	u := strings.ToLower(url)
	return strings.Contains(u, ".mpd") ||
		strings.HasPrefix(u, "rtmp://") ||
		strings.HasPrefix(u, "rtmps://") ||
		strings.HasPrefix(u, "mmsh://") ||
		strings.HasPrefix(u, "mms://")
}

// cifradoNoApple detecta SAMPLE-AES con un sistema de claves que no es el de
// Apple. Sin KEYFORMAT no se concluye nada: el default es de Apple.
func cifradoNoApple(body string) bool {
	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if !strings.HasPrefix(linea, "#EXT-X-KEY:") {
			continue
		}
		if !strings.Contains(linea, "METHOD=SAMPLE-AES") {
			continue
		}
		formato, ok := atributoEntreComillas(linea, "KEYFORMAT")
		if ok && !strings.EqualFold(formato, keyformatApple) {
			return true
		}
	}
	return false
}

// varianteSoportada exige que TODOS los códecs de la variante sean
// reproducibles: un vídeo válido con un audio que AVFoundation no abre deja la
// variante inservible igualmente.
func varianteSoportada(codecs string) bool {
	partes := strings.Split(codecs, ",")
	if len(partes) == 0 {
		return false
	}
	for _, c := range partes {
		if !codecSoportado(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

func codecSoportado(c string) bool {
	for _, p := range codecsSoportados {
		if strings.HasPrefix(c, p) {
			return true
		}
	}
	return false
}

// atributoEntreComillas extrae CLAVE="valor" de una línea de atributos HLS.
func atributoEntreComillas(linea, clave string) (string, bool) {
	i := strings.Index(linea, clave+`="`)
	if i < 0 {
		return "", false
	}
	resto := linea[i+len(clave)+2:]
	fin := strings.Index(resto, `"`)
	if fin < 0 {
		return "", false
	}
	return resto[:fin], true
}
```

- [ ] **Step 4: Ejecutar el test y comprobar que pasa**

```bash
cd gateway && go test ./internal/domain/ -run TestClassifyManifest -v && gofmt -l . && go vet ./...
```

Esperado: PASS en los 12 subtests, `gofmt -l` sin salida.

- [ ] **Step 5: Commit**

```bash
git add gateway/internal/domain/airplay.go gateway/internal/domain/airplay_test.go
git commit -m "gateway: clasificador de compatibilidad AirPlay sobre manifiestos HLS

Tres estados en vez de dos porque solo el 57,5 % de los manifiestos del
catálogo declara CODECS: colapsar el desconocido en \"no\" marcaría como
rotos miles de canales que funcionan.

Vive en domain y no en el validador porque lo consumen tanto el validador
como el handler; dejarlo en un adaptador obligaría a api a importar
adapters."
```

---

## Tarea 2: Aprovechar el GET de fallback del checker

El fallback GET ya lee 64 KB y los descarta (`checker.go:122`). Clasificarlos en vez de tirarlos no cuesta ni una petición.

**Files:**
- Modify: `gateway/internal/adapters/validator/models.go` (campo nuevo en `StreamResult`)
- Modify: `gateway/internal/adapters/validator/checker.go:102-125`
- Test: `gateway/internal/adapters/validator/checker_test.go`

**Interfaces:**
- Consumes: `domain.ClassifyManifest`, `domain.AirplaySupport` (Tarea 1).
- Produces: `StreamResult.Airplay domain.AirplaySupport`. Nadie más lo consume todavía; queda disponible para un relleno de fondo futuro.

- [ ] **Step 1: Escribir el test que falla**

Añadir a `gateway/internal/adapters/validator/checker_test.go`:

```go
func TestCheckClasificaAirplayEnElFallbackGET(t *testing.T) {
	const master = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS=\"avc1.4d4028,mp4a.40.2\"\n720p.m3u8\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HEAD rechazado a propósito: es lo que fuerza el fallback a GET, que
		// es el único camino donde hay cuerpo que clasificar.
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(master))
	}))
	defer srv.Close()

	c := NewChecker(nil, 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/a.m3u8")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, quiero true")
	}
	if res.Airplay != domain.AirplayOK {
		t.Errorf("Airplay = %v, quiero AirplayOK", res.Airplay)
	}
}

func TestCheckDejaAirplayDesconocidoSiElHEADBasta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker(nil, 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/a.m3u8")

	// Sin GET no hay cuerpo, y sin cuerpo no se inventa un veredicto.
	if res.Airplay != domain.AirplayUnknown {
		t.Errorf("Airplay = %v, quiero AirplayUnknown", res.Airplay)
	}
}
```

Añadir al bloque de imports del fichero, si faltan: `"context"`, `"net/http"`, `"net/http/httptest"`, `"time"`, `"github.com/gdberysan/open-tv/gateway/internal/domain"`.

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd gateway && go test ./internal/adapters/validator/ -run TestCheckClasifica -v
```

Esperado: FAIL de compilación, `res.Airplay undefined`.

- [ ] **Step 3: Implementar**

En `models.go`, añadir el campo a `StreamResult` (y el import de `domain`):

```go
// StreamResult representa el resultado de una validación de stream.
type StreamResult struct {
	URL       string
	IsAlive   bool
	LatencyMs int64
	Protocol  string
	Error     error

	// Compatibilidad con AirPlay, cuando se ha podido determinar. Solo la
	// rellena el camino con fallback a GET: es el único que lee cuerpo.
	Airplay domain.AirplaySupport
}
```

En `checker.go`, sustituir el descarte del cuerpo (líneas 120-122) por una lectura que se aproveche:

```go
		defer respGet.Body.Close()
		// Drenar el cuerpo es obligatorio: sin leerlo, la conexión queda
		// inutilizable y el servidor la ve abortada a media respuesta. Ya que
		// hay que leerlo, se clasifica en vez de tirarlo — cero peticiones
		// extra por un veredicto de compatibilidad AirPlay.
		cuerpo, err := io.ReadAll(io.LimitReader(respGet.Body, 64<<10))
		if err != nil {
			result.Error = fmt.Errorf("leyendo cuerpo del GET: %w", err)
			return result
		}
		result.Airplay = domain.ClassifyManifest(url, string(cuerpo))

		result.IsAlive = respGet.StatusCode >= 200 && respGet.StatusCode < 300
```

Añadir `"github.com/gdberysan/open-tv/gateway/internal/domain"` a los imports de `checker.go`.

- [ ] **Step 4: Ejecutar los tests y comprobar que pasan**

```bash
cd gateway && go test -race -count=1 ./internal/adapters/validator/ -v && gofmt -l . && go vet ./...
```

Esperado: PASS, incluidos los tests preexistentes del validador.

- [ ] **Step 5: Commit**

```bash
git add gateway/internal/adapters/validator/
git commit -m "gateway: el fallback GET del checker clasifica en vez de descartar

El cuerpo ya se leía y se tiraba para no dejar la conexión inservible.
Clasificarlo sale gratis: ni una petición más por un veredicto de
compatibilidad AirPlay."
```

---

## Tarea 3: Sonda bajo demanda y `airplay_ok` en `/channels/stream`

Los handlers reciben un pool de **solo lectura** (`main.go:62`), así que el veredicto se cachea en memoria, no en SQLite.

**Files:**
- Create: `gateway/internal/api/handlers/airplay_probe.go`
- Modify: `gateway/internal/api/handlers/channel_handler.go:74-89` (`GetStreamURL`) y el constructor `NewChannelHandler:22-27`
- Modify: `gateway/internal/api/router.go:26`
- Test: `gateway/internal/api/handlers/airplay_probe_test.go`, `gateway/internal/api/handlers/channel_handler_test.go`

**Interfaces:**
- Consumes: `domain.ClassifyManifest`, `domain.AirplaySupport` (Tarea 1).
- Produces: `handlers.NewAirplayProber(client *http.Client, ttl time.Duration, maxEntradas int) *AirplayProber` con `Veredicto(ctx context.Context, url string) domain.AirplaySupport`. `NewChannelHandler` gana un quinto parámetro `prober *AirplayProber`. `GET /channels/stream?id=` responde `{"url": string, "airplay_ok": bool|null}`.

- [ ] **Step 1: Escribir el test que falla**

Crear `gateway/internal/api/handlers/airplay_probe_test.go`:

```go
package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
)

const masterOK = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000000,CODECS=\"avc1.4d4028,mp4a.40.2\"\n720p.m3u8\n"

func TestProberClasificaYCachea(t *testing.T) {
	var peticiones atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		peticiones.Add(1)
		_, _ = w.Write([]byte(masterOK))
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10)
	url := srv.URL + "/a.m3u8"

	if got := p.Veredicto(context.Background(), url); got != domain.AirplayOK {
		t.Fatalf("primer veredicto = %v, quiero AirplayOK", got)
	}
	if got := p.Veredicto(context.Background(), url); got != domain.AirplayOK {
		t.Fatalf("segundo veredicto = %v, quiero AirplayOK", got)
	}
	if n := peticiones.Load(); n != 1 {
		t.Errorf("el origen recibió %d peticiones, quiero 1: la caché no está sirviendo", n)
	}
}

func TestProberDevuelveDesconocidoSiElOrigenFalla(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10)
	if got := p.Veredicto(context.Background(), srv.URL+"/a.m3u8"); got != domain.AirplayUnknown {
		t.Errorf("veredicto = %v, quiero AirplayUnknown", got)
	}
}

func TestProberNoSondeaLoQueLaURLYaDescarta(t *testing.T) {
	var peticiones atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		peticiones.Add(1)
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 10)
	if got := p.Veredicto(context.Background(), srv.URL+"/a.mpd"); got != domain.AirplayNo {
		t.Errorf("veredicto = %v, quiero AirplayNo", got)
	}
	if n := peticiones.Load(); n != 0 {
		t.Errorf("el origen recibió %d peticiones, quiero 0: DASH se descarta sin red", n)
	}
}

func TestProberRespetaElTopeDeEntradas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(masterOK))
	}))
	defer srv.Close()

	p := NewAirplayProber(srv.Client(), time.Hour, 2)
	for _, s := range []string{"/a.m3u8", "/b.m3u8", "/c.m3u8"} {
		p.Veredicto(context.Background(), srv.URL+s)
	}
	if n := p.entradas(); n > 2 {
		t.Errorf("la caché guarda %d entradas, el tope es 2", n)
	}
}
```

Añadir a `channel_handler_test.go`:

```go
func TestGetStreamURLIncluyeAirplayOK(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(masterOK))
	}))
	defer origen.Close()

	// mockStreamRepo guarda un map[ChannelID][]Stream y FindBestByChannelID
	// escoge el vivo de menor latencia, así que IsAlive es obligatorio.
	streams := &mockStreamRepo{
		streams: map[domain.ChannelID][]domain.Stream{
			"1": {{URL: origen.URL + "/a.m3u8", IsAlive: true, LatencyMs: 10}},
		},
	}
	h := NewChannelHandler(nil, &mockRepo{}, &mockProvider{}, streams,
		NewAirplayProber(origen.Client(), time.Hour, 10))

	req := httptest.NewRequest(http.MethodGet, "/channels/stream?id=1", nil)
	rec := httptest.NewRecorder()
	h.GetStreamURL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("código = %d, quiero 200", rec.Code)
	}
	var body struct {
		URL       string `json:"url"`
		AirplayOK *bool  `json:"airplay_ok"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if body.URL == "" {
		t.Error("url vacía: el sondeo no puede romper la respuesta principal")
	}
	if body.AirplayOK == nil || !*body.AirplayOK {
		t.Errorf("airplay_ok = %v, quiero true", body.AirplayOK)
	}
}
```

> **Nota para quien implemente:** `mockStreamRepo` (línea 93) y `mockProvider` ya existen en `channel_handler_test.go`; no hay que tocarlos. **Todas** las llamadas existentes a `NewChannelHandler` en ese fichero necesitan el quinto argumento — pásales `NewAirplayProber(http.DefaultClient, time.Hour, 10)`.

- [ ] **Step 2: Ejecutar los tests y comprobar que fallan**

```bash
cd gateway && go test ./internal/api/handlers/ -run 'TestProber|TestGetStreamURLIncluye' -v
```

Esperado: FAIL de compilación, `undefined: NewAirplayProber`.

- [ ] **Step 3: Implementar la sonda**

Crear `gateway/internal/api/handlers/airplay_probe.go`:

```go
package handlers

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gdberysan/open-tv/gateway/internal/domain"
)

// presupuestoSondeo acota lo que el sondeo puede retrasar la respuesta. Si no
// da tiempo, se devuelve desconocido: la URL de reproducción nunca espera por
// un dato accesorio.
const presupuestoSondeo = time.Second

// maxCuerpoManifiesto limita la lectura. Un master playlist son unos cientos de
// bytes; leer más solo sirve para que un origen roto agote memoria.
const maxCuerpoManifiesto = 64 << 10

// AirplayProber resuelve la compatibilidad AirPlay de una URL bajo demanda y la
// recuerda en memoria.
//
// En memoria y no en SQLite a propósito: los handlers reciben un pool de solo
// lectura (cmd/server/main.go), separado del de escritura. Persistir desde aquí
// rompería esa separación por un dato que se recalcula en un segundo.
type AirplayProber struct {
	client      *http.Client
	ttl         time.Duration
	maxEntradas int

	mu    sync.RWMutex
	cache map[string]entradaAirplay
}

type entradaAirplay struct {
	veredicto domain.AirplaySupport
	expira    time.Time
}

func NewAirplayProber(client *http.Client, ttl time.Duration, maxEntradas int) *AirplayProber {
	if client == nil {
		client = &http.Client{Timeout: presupuestoSondeo}
	}
	return &AirplayProber{
		client:      client,
		ttl:         ttl,
		maxEntradas: maxEntradas,
		cache:       make(map[string]entradaAirplay),
	}
}

// Veredicto nunca devuelve error: un sondeo que falla es AirplayUnknown, que es
// exactamente lo que significa "no se pudo determinar".
func (p *AirplayProber) Veredicto(ctx context.Context, url string) domain.AirplaySupport {
	// Lo que la URL ya descarta no necesita red.
	if v := domain.ClassifyManifest(url, ""); v == domain.AirplayNo {
		return v
	}
	if v, ok := p.leerCache(url); ok {
		return v
	}

	v := p.sondear(ctx, url)
	p.guardar(url, v)
	return v
}

func (p *AirplayProber) sondear(ctx context.Context, url string) domain.AirplaySupport {
	reqCtx, cancel := context.WithTimeout(ctx, presupuestoSondeo)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return domain.AirplayUnknown
	}
	// El mismo User-Agent que el validador: varios orígenes contestan 403 al
	// default de Go.
	req.Header.Set("User-Agent", "VLC/3.0.20 LibVLC/3.0.20")

	resp, err := p.client.Do(req)
	if err != nil {
		return domain.AirplayUnknown
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.AirplayUnknown
	}

	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxCuerpoManifiesto))
	if err != nil {
		return domain.AirplayUnknown
	}
	return domain.ClassifyManifest(url, string(cuerpo))
}

func (p *AirplayProber) leerCache(url string) (domain.AirplaySupport, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.cache[url]
	if !ok || time.Now().After(e.expira) {
		return domain.AirplayUnknown, false
	}
	return e.veredicto, true
}

func (p *AirplayProber) guardar(url string, v domain.AirplaySupport) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Al llegar al tope se vacía entera en vez de desalojar por antigüedad: son
	// unos cientos de entradas de texto y una LRU aquí sería complejidad sin
	// beneficio medible.
	if len(p.cache) >= p.maxEntradas {
		p.cache = make(map[string]entradaAirplay, p.maxEntradas)
	}
	p.cache[url] = entradaAirplay{veredicto: v, expira: time.Now().Add(p.ttl)}
}

// entradas expone el tamaño de la caché. Solo para tests.
func (p *AirplayProber) entradas() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.cache)
}

// aBool traduce el veredicto al JSON del cable: null cuando no se sabe.
func aBool(v domain.AirplaySupport) *bool {
	switch v {
	case domain.AirplayOK:
		t := true
		return &t
	case domain.AirplayNo:
		f := false
		return &f
	default:
		return nil
	}
}
```

- [ ] **Step 4: Cablear el handler**

En `channel_handler.go`, añadir el campo y el parámetro:

```go
type ChannelHandler struct {
	logger   *slog.Logger
	repo     ports.ChannelRepository
	provider ports.ProviderPort
	streams  ports.StreamRepository
	prober   *AirplayProber
}

func NewChannelHandler(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, prober *AirplayProber) *ChannelHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &ChannelHandler{logger: logger, repo: repo, provider: provider, streams: streams, prober: prober}
}
```

Y sustituir el cuerpo de `GetStreamURL`:

```go
// GetStreamURL resuelve la URL de reproducción para un canal.
// Ruta: GET /channels/stream?id=<channelID>
// Usa query param para soportar IDs con "/" (ej: "24/7 News").
//
// airplay_ok viaja aquí y no en /channels porque el sondeo cuesta una petición
// al origen: se paga solo por los canales que de verdad se van a reproducir.
// null significa "no se pudo determinar", que es el caso más común.
func (h *ChannelHandler) GetStreamURL(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	url, err := h.resolveStreamURL(r, domain.ChannelID(id))
	if err != nil {
		h.writeError(w, http.StatusNotFound, "Stream no encontrado")
		return
	}

	respuesta := map[string]any{"url": url}
	if h.prober != nil {
		respuesta["airplay_ok"] = aBool(h.prober.Veredicto(r.Context(), url))
	}
	h.writeJSON(w, http.StatusOK, respuesta)
}
```

En `router.go:26`, construir la sonda:

```go
	// TTL de 12 h: los códecs de un canal no cambian en una tarde, y la caché
	// se pierde igualmente al reiniciar el gateway.
	prober := handlers.NewAirplayProber(nil, 12*time.Hour, 2000)
	ch := handlers.NewChannelHandler(logger, repo, provider, streams, prober)
```

Añadir `"time"` a los imports de `router.go`.

- [ ] **Step 5: Ejecutar los tests y comprobar que pasan**

```bash
cd gateway && go test -race -count=1 ./... && gofmt -l . && go vet ./... && go build ./...
```

Esperado: PASS en todo el módulo. `domain/channel_test.go` sigue verde en 14 claves — no se ha tocado `domain.Channel`.

- [ ] **Step 6: Commit**

```bash
git add gateway/internal/api/
git commit -m "gateway: airplay_ok bajo demanda en /channels/stream

El veredicto se cachea en memoria con TTL de 12h, no en SQLite: los
handlers reciben un pool de solo lectura y persistir desde aquí rompería
esa separación por un dato que se recalcula en un segundo.

El sondeo tiene presupuesto de 1s y nunca bloquea la respuesta: si no da
tiempo, airplay_ok sale null y la URL viaja igual."
```

---

## Tarea 4: Selector de ruta nativo (`AVRoutePickerView`)

Primera mitad de la capa Swift. Se verifica **viendo el botón** en la app: es la prueba de que el registro del platform view funciona antes de construir nada encima.

**Files:**
- Create: `mobile/macos/Runner/AirPlay/RoutePickerFactory.swift`
- Create: `mobile/macos/Runner/AirPlay/AirPlayPlugin.swift`
- Modify: `mobile/macos/Runner/MainFlutterWindow.swift`
- Modify: `mobile/macos/Runner/Info.plist`

**Interfaces:**
- Consumes: nada.
- Produces: platform view registrado con id `dev.korven.opentv/route-picker`. Lo consume la Tarea 10.

- [ ] **Step 1: Crear la factoría del selector**

Crear `mobile/macos/Runner/AirPlay/RoutePickerFactory.swift`:

```swift
import AVKit
import Cocoa
import FlutterMacOS

/// Devuelve el AVRoutePickerView real de Apple. El descubrimiento de
/// dispositivos, el emparejamiento y los cambios de protocolo de tvOS son
/// problema de AVFoundation, no nuestro: por eso no hay ni mDNS ni sockets en
/// todo este módulo.
final class RoutePickerFactory: NSObject, FlutterPlatformViewFactory {
  func create(withViewIdentifier viewId: Int64, arguments args: Any?) -> NSView {
    let picker = AVRoutePickerView()
    picker.isRoutePickerButtonBordered = false
    // Paleta Korven: gris de texto en reposo, acento cuando hay ruta activa.
    picker.setRoutePickerButtonColor(
      NSColor(srgbRed: 0.62, green: 0.64, blue: 0.66, alpha: 1), for: .normal)
    picker.setRoutePickerButtonColor(
      NSColor(srgbRed: 0.36, green: 0.85, blue: 0.62, alpha: 1), for: .active)
    return picker
  }
}
```

- [ ] **Step 2: Crear el plugin con el registro**

Crear `mobile/macos/Runner/AirPlay/AirPlayPlugin.swift`:

```swift
import Cocoa
import FlutterMacOS

/// Solo cableado: registra la factoría de vistas y, a partir de la Tarea 5,
/// los dos canales de la sesión. Cero lógica de negocio — esta capa no pasa
/// nunca por CI (los dos jobs corren en ubuntu-latest), así que todo lo que
/// pueda decidirse en Dart se decide en Dart.
enum AirPlayPlugin {
  static func register(with registrar: FlutterPluginRegistrar) {
    registrar.register(
      RoutePickerFactory(),
      withId: "dev.korven.opentv/route-picker")
  }
}
```

- [ ] **Step 3: Llamar al registro**

En `mobile/macos/Runner/MainFlutterWindow.swift`, después de
`RegisterGeneratedPlugins(registry: flutterViewController)`, añadir:

```swift
    AirPlayPlugin.register(
      with: flutterViewController.registrar(forPlugin: "AirPlayPlugin"))
```

- [ ] **Step 4: Permitir los streams en claro**

En `mobile/macos/Runner/Info.plist`, dentro del `<dict>` raíz, añadir:

```xml
	<key>NSAppTransportSecurity</key>
	<dict>
		<!-- El 19 % del catálogo son URLs http:// sin TLS. La clave estrecha,
		     limitada a cargas de medios: NSAllowsArbitraryLoads abriría también
		     las peticiones al gateway y a los logos, que no lo necesitan. -->
		<key>NSAllowsArbitraryLoadsInMedia</key>
		<true/>
	</dict>
```

- [ ] **Step 5: Añadir los ficheros al proyecto Xcode y compilar**

```bash
cd mobile && open macos/Runner.xcworkspace
```

En Xcode: arrastrar la carpeta `Runner/AirPlay` al grupo `Runner` del navegador, con **Create groups** y el target `Runner` marcado. Luego:

```bash
cd mobile && flutter build macos --debug
```

Esperado: BUILD SUCCEEDED. Si falla con `cannot find 'AirPlayPlugin' in scope`, los ficheros no están añadidos al target.

- [ ] **Step 6: Verificación manual**

```bash
cd mobile && flutter run -d macos
```

La app arranca igual que antes (todavía no hay botón en la UI; eso es la Tarea 10). Esta tarea se da por buena si compila y arranca sin excepciones en consola.

- [ ] **Step 7: Commit**

```bash
git add mobile/macos/
git commit -m "macos: selector de ruta AirPlay nativo y excepción ATS para medios

El AVRoutePickerView de Apple resuelve descubrimiento, emparejamiento y
los cambios de protocolo de tvOS. Por eso no hay mDNS ni sockets aquí: la
única vía sancionada para emitir es AVFoundation.

NSAllowsArbitraryLoadsInMedia y no NSAllowsArbitraryLoads: el 19 % de los
streams son http:// en claro, pero el gateway y los logos no necesitan la
excepción."
```

---

## Tarea 5: Sesión `AVPlayer` y canales de eventos

**Files:**
- Create: `mobile/macos/Runner/AirPlay/AirPlaySession.swift`
- Create: `mobile/macos/Runner/AirPlay/RouteName.swift`
- Modify: `mobile/macos/Runner/AirPlay/AirPlayPlugin.swift`

**Interfaces:**
- Consumes: registro del plugin (Tarea 4).
- Produces: MethodChannel `dev.korven.opentv/airplay` con `start(url:String, title:String)` y `stop()`. EventChannel `dev.korven.opentv/airplay/events` emitiendo `["type":"route","active":Bool,"name":String?]` y `["type":"status","state":"loading"|"playing"|"failed","error":String?,"formatError":Bool]`. Lo consume la Tarea 6.

- [ ] **Step 1: Nombre del dispositivo**

Crear `mobile/macos/Runner/AirPlay/RouteName.swift`:

```swift
import CoreAudio
import Foundation

/// Nombre del destino AirPlay, best-effort.
///
/// macOS no expone públicamente el nombre de la ruta de un AVPlayer. Cuando
/// AirPlay se activa, el dispositivo de salida por defecto del sistema pasa a
/// ser el receptor, así que CoreAudio da el nombre correcto en la práctica. Si
/// falla, la UI dice "AirPlay" y no se pierde nada.
enum RouteName {
  static func salidaPorDefecto() -> String? {
    var deviceID = AudioDeviceID(0)
    var size = UInt32(MemoryLayout<AudioDeviceID>.size)
    // kAudioObjectPropertyElementMain exige macOS 12 y el target es 10.15.
    var addr = AudioObjectPropertyAddress(
      mSelector: kAudioHardwarePropertyDefaultOutputDevice,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    guard AudioObjectGetPropertyData(
      AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size, &deviceID) == noErr
    else { return nil }

    var nombre: CFString = "" as CFString
    var nombreSize = UInt32(MemoryLayout<CFString>.size)
    var nombreAddr = AudioObjectPropertyAddress(
      mSelector: kAudioObjectPropertyName,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    guard AudioObjectGetPropertyData(
      deviceID, &nombreAddr, 0, nil, &nombreSize, &nombre) == noErr
    else { return nil }

    let s = nombre as String
    return s.isEmpty ? nil : s
  }
}
```

- [ ] **Step 2: La sesión**

Crear `mobile/macos/Runner/AirPlay/AirPlaySession.swift`:

```swift
import AVFoundation
import Foundation

/// Posee el AVPlayer que emite a AirPlay. Existe solo mientras hay sesión: la
/// reproducción local sigue siendo de media_kit, y los dos nunca están abiertos
/// a la vez.
final class AirPlaySession: NSObject {
  private var player: AVPlayer?
  private var observaciones: [NSKeyValueObservation] = []
  private let emitir: ([String: Any]) -> Void

  init(emitir: @escaping ([String: Any]) -> Void) {
    self.emitir = emitir
  }

  func start(url: String, title: String) {
    stop()

    guard let u = URL(string: url) else {
      emitir(["type": "status", "state": "failed",
              "error": "URL inválida", "formatError": false])
      return
    }

    let item = AVPlayerItem(url: u)
    let p = AVPlayer(playerItem: item)
    p.allowsExternalPlayback = true
    p.usesExternalPlaybackWhileExternalScreenIsActive = true
    player = p

    emitir(["type": "status", "state": "loading", "formatError": false])

    observaciones.append(item.observe(\.status, options: [.new]) { [weak self] it, _ in
      guard let self = self else { return }
      switch it.status {
      case .readyToPlay:
        p.play()
      case .failed:
        let err = it.error
        self.emitir([
          "type": "status", "state": "failed",
          "error": err?.localizedDescription ?? "Fallo de reproducción",
          "formatError": Self.esErrorDeFormato(err),
        ])
      default:
        break
      }
    })

    // timeControlStatus == .playing es la prueba de reproducción real. El
    // equivalente de por qué PlaybackGuard no se fía de `playing` en mpv.
    observaciones.append(p.observe(\.timeControlStatus, options: [.new]) { [weak self] pl, _ in
      guard let self = self, pl.timeControlStatus == .playing else { return }
      self.emitir(["type": "status", "state": "playing", "formatError": false])
    })

    observaciones.append(p.observe(\.isExternalPlaybackActive, options: [.new]) { [weak self] pl, _ in
      guard let self = self else { return }
      self.emitir([
        "type": "route",
        "active": pl.isExternalPlaybackActive,
        "name": RouteName.salidaPorDefecto() as Any,
      ])
    })
  }

  func stop() {
    player?.pause()
    player?.replaceCurrentItem(with: nil)
    player = nil
    observaciones.forEach { $0.invalidate() }
    observaciones.removeAll()
  }

  /// Distingue "este stream no lo puedo decodificar" de "no llegué al servidor".
  /// Es la diferencia entre recordar el canal como incompatible para siempre y
  /// no escribir nada: un Apple TV dormido y un stream MPEG-2 afloran los dos
  /// como .failed.
  private static func esErrorDeFormato(_ error: Error?) -> Bool {
    guard let e = error as NSError?, e.domain == AVFoundationErrorDomain else {
      return false
    }
    switch e.code {
    case AVError.Code.decodeFailed.rawValue,
         AVError.Code.fileFormatNotRecognized.rawValue,
         AVError.Code.failedToLoadMediaData.rawValue,
         AVError.Code.decoderNotFound.rawValue:
      return true
    default:
      return false
    }
  }
}
```

- [ ] **Step 3: Cablear los canales**

Reemplazar `mobile/macos/Runner/AirPlay/AirPlayPlugin.swift` por:

```swift
import Cocoa
import FlutterMacOS

/// Solo cableado: registra la factoría de vistas y los dos canales de la
/// sesión. Cero lógica de negocio — esta capa no pasa nunca por CI (los dos
/// jobs corren en ubuntu-latest), así que todo lo que pueda decidirse en Dart
/// se decide en Dart.
final class AirPlayPlugin: NSObject, FlutterStreamHandler {
  private var sink: FlutterEventSink?
  private var session: AirPlaySession?
  private static var instancia: AirPlayPlugin?

  static func register(with registrar: FlutterPluginRegistrar) {
    registrar.register(
      RoutePickerFactory(),
      withId: "dev.korven.opentv/route-picker")

    let plugin = AirPlayPlugin()
    instancia = plugin

    let metodos = FlutterMethodChannel(
      name: "dev.korven.opentv/airplay",
      binaryMessenger: registrar.messenger)
    metodos.setMethodCallHandler { call, result in
      plugin.atender(call, result)
    }

    let eventos = FlutterEventChannel(
      name: "dev.korven.opentv/airplay/events",
      binaryMessenger: registrar.messenger)
    eventos.setStreamHandler(plugin)
  }

  private func atender(_ call: FlutterMethodCall, _ result: FlutterResult) {
    switch call.method {
    case "start":
      guard let args = call.arguments as? [String: Any],
            let url = args["url"] as? String
      else {
        result(FlutterError(code: "args", message: "url requerida", details: nil))
        return
      }
      sesionViva().start(url: url, title: args["title"] as? String ?? "")
      result(nil)
    case "stop":
      session?.stop()
      result(nil)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func sesionViva() -> AirPlaySession {
    if let s = session { return s }
    let s = AirPlaySession { [weak self] evento in
      DispatchQueue.main.async { self?.sink?(evento) }
    }
    session = s
    return s
  }

  func onListen(withArguments _: Any?, eventSink: @escaping FlutterEventSink) -> FlutterError? {
    sink = eventSink
    return nil
  }

  func onCancel(withArguments _: Any?) -> FlutterError? {
    sink = nil
    return nil
  }

  /// Sin esto, el AVPlayer retiene la ruta después de cerrar la app.
  static func alTerminar() {
    instancia?.session?.stop()
  }
}
```

En `mobile/macos/Runner/AppDelegate.swift`, dentro de la clase, añadir:

```swift
  override func applicationWillTerminate(_ notification: Notification) {
    AirPlayPlugin.alTerminar()
  }
```

- [ ] **Step 4: Compilar**

```bash
cd mobile && flutter build macos --debug
```

Esperado: BUILD SUCCEEDED. Añadir los dos ficheros nuevos al target en Xcode si el build se queja de símbolos no encontrados.

- [ ] **Step 5: Commit**

```bash
git add mobile/macos/
git commit -m "macos: sesión AVPlayer para AirPlay tras dos canales de plataforma

El AVPlayer existe solo mientras hay sesión; media_kit sigue siendo el
único reproductor local y los dos nunca están abiertos a la vez.

esErrorDeFormato separa \"no puedo decodificar esto\" de \"no llegué al
servidor\": un Apple TV dormido y un stream MPEG-2 afloran los dos como
.failed, y persistir incompatibilidad ante un fallo de red etiquetaría
mal canales buenos para siempre."
```

---

## Tarea 6: Modelo de sesión y costura de plataforma en Dart

**Files:**
- Create: `mobile/lib/domain/models/cast_session.dart`
- Create: `mobile/lib/data/airplay/airplay_platform.dart`
- Test: `mobile/test/domain/cast_session_test.dart`

**Interfaces:**
- Consumes: contrato de canales de la Tarea 5.
- Produces: `CastState`, `CastSession`, `AirplayEvent` (`AirplayRouteEvent`/`AirplayStatusEvent`), `AirplayPlaybackState`, `AirplayPlatform`, `airplayPlatformProvider`. Los consumen las tareas 7, 9, 10, 11.

- [ ] **Step 1: Escribir el test que falla**

Crear `mobile/test/domain/cast_session_test.dart`:

```dart
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';

void main() {
  test('idle no se dibuja ni intercepta toques', () {
    const s = CastSession();
    expect(s.visible, isFalse);
    expect(s.intercepta, isFalse);
  });

  test('armed se dibuja e intercepta: hay dispositivo esperando', () {
    const s = CastSession(state: CastState.armed, deviceName: 'Salón Apple TV');
    expect(s.visible, isTrue);
    expect(s.intercepta, isTrue);
    expect(s.etiquetaDispositivo, 'Salón Apple TV');
  });

  test('casting se dibuja e intercepta', () {
    const s = CastSession(state: CastState.casting, channelName: 'BBC News');
    expect(s.visible, isTrue);
    expect(s.intercepta, isTrue);
  });

  test('failed se dibuja pero ya no intercepta: el traspaso va a local', () {
    const s = CastSession(state: CastState.failed, error: 'sin códec');
    expect(s.visible, isTrue);
    expect(s.intercepta, isFalse);
  });

  test('sin nombre de dispositivo la etiqueta cae a AirPlay', () {
    const s = CastSession(state: CastState.armed);
    expect(s.etiquetaDispositivo, 'AirPlay');
  });

  test('copyWith limpia el error cuando se le pide', () {
    const s = CastSession(state: CastState.failed, error: 'x');
    final limpio = s.copyWith(state: CastState.armed, clearError: true);
    expect(limpio.error, isNull);
    expect(limpio.state, CastState.armed);
  });
}
```

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd mobile && flutter test test/domain/cast_session_test.dart
```

Esperado: FAIL, `Error: Couldn't resolve the package 'korven_open_tv'... cast_session.dart`.

- [ ] **Step 3: Implementar el modelo**

Crear `mobile/lib/domain/models/cast_session.dart`:

```dart
/// Estados de la sesión de emisión.
///
/// [armed] existe porque elegir dispositivo y elegir canal son dos actos
/// distintos: con una ruta seleccionada y nada reproduciendo, la barra ya debe
/// verse y el toque en la rejilla ya debe emitir en vez de abrir el reproductor.
enum CastState { idle, armed, connecting, casting, failed }

class CastSession {
  const CastSession({
    this.state = CastState.idle,
    this.deviceName,
    this.channelId,
    this.channelName,
    this.error,
  });

  final CastState state;
  final String? deviceName;
  final String? channelId;
  final String? channelName;
  final String? error;

  /// Si se dibuja la CastBar.
  bool get visible => state != CastState.idle;

  /// Si un toque en la rejilla emite en vez de abrir el reproductor. En
  /// [CastState.failed] es false a propósito: ese estado significa que el
  /// canal se va a reproducir en local.
  bool get intercepta =>
      state == CastState.armed ||
      state == CastState.connecting ||
      state == CastState.casting;

  /// macOS no expone públicamente el nombre de la ruta; cuando CoreAudio no lo
  /// da, decir "AirPlay" es mejor que dejar el hueco vacío.
  String get etiquetaDispositivo => deviceName ?? 'AirPlay';

  CastSession copyWith({
    CastState? state,
    String? deviceName,
    String? channelId,
    String? channelName,
    String? error,
    bool clearError = false,
    bool clearChannel = false,
  }) =>
      CastSession(
        state: state ?? this.state,
        deviceName: deviceName ?? this.deviceName,
        channelId: clearChannel ? null : (channelId ?? this.channelId),
        channelName: clearChannel ? null : (channelName ?? this.channelName),
        error: clearError ? null : (error ?? this.error),
      );
}
```

- [ ] **Step 4: Implementar la costura de plataforma**

Crear `mobile/lib/data/airplay/airplay_platform.dart`:

```dart
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

enum AirplayPlaybackState { loading, playing, failed }

sealed class AirplayEvent {
  const AirplayEvent();
}

/// La ruta del sistema cambió. [active] false a mitad de emisión significa que
/// el televisor se apagó o salió de la red.
class AirplayRouteEvent extends AirplayEvent {
  const AirplayRouteEvent({required this.active, this.name});
  final bool active;
  final String? name;
}

class AirplayStatusEvent extends AirplayEvent {
  const AirplayStatusEvent({
    required this.state,
    this.error,
    this.formatError = false,
  });

  final AirplayPlaybackState state;
  final String? error;

  /// True solo si AVFoundation falló por formato o códec, nunca por red. Es lo
  /// único que autoriza a recordar el canal como incompatible.
  final bool formatError;
}

/// La frontera con la capa nativa. Es una interfaz para que la máquina de
/// estados se pueda testear con un doble: CI corre en ubuntu-latest y allí no
/// hay ningún MethodChannel que responda.
abstract interface class AirplayPlatform {
  Stream<AirplayEvent> get events;
  Future<void> start({required String url, required String title});
  Future<void> stop();
}

class MethodChannelAirplay implements AirplayPlatform {
  static const _metodos = MethodChannel('dev.korven.opentv/airplay');
  static const _eventos = EventChannel('dev.korven.opentv/airplay/events');

  @override
  Stream<AirplayEvent> get events =>
      _eventos.receiveBroadcastStream().map(_traducir).where((e) => e != null).cast();

  @override
  Future<void> start({required String url, required String title}) =>
      _metodos.invokeMethod<void>('start', {'url': url, 'title': title});

  @override
  Future<void> stop() => _metodos.invokeMethod<void>('stop');

  static AirplayEvent? _traducir(dynamic raw) {
    if (raw is! Map) return null;
    switch (raw['type']) {
      case 'route':
        return AirplayRouteEvent(
          active: raw['active'] as bool? ?? false,
          name: raw['name'] as String?,
        );
      case 'status':
        final estado = switch (raw['state']) {
          'playing' => AirplayPlaybackState.playing,
          'failed' => AirplayPlaybackState.failed,
          _ => AirplayPlaybackState.loading,
        };
        return AirplayStatusEvent(
          state: estado,
          error: raw['error'] as String?,
          formatError: raw['formatError'] as bool? ?? false,
        );
      default:
        return null;
    }
  }
}

final airplayPlatformProvider =
    Provider<AirplayPlatform>((_) => MethodChannelAirplay());
```

- [ ] **Step 5: Ejecutar los tests y comprobar que pasan**

```bash
cd mobile && flutter test test/domain/cast_session_test.dart && flutter analyze
```

Esperado: PASS en los 6 tests, `flutter analyze` sin issues.

- [ ] **Step 6: Commit**

```bash
git add mobile/lib/domain/models/cast_session.dart mobile/lib/data/airplay/ mobile/test/domain/cast_session_test.dart
git commit -m "app: modelo de sesión de emisión y costura con la capa nativa

AirplayPlatform es una interfaz y no una clase suelta porque CI corre en
ubuntu-latest: sin doble, la máquina de estados de la Tarea 9 sería
inverificable.

El estado armed existe porque elegir dispositivo y elegir canal son dos
actos distintos: con ruta puesta y nada sonando, la barra ya se ve y el
toque en la rejilla ya emite."
```

---

## Tarea 7: `AirplayGuard`

**Files:**
- Create: `mobile/lib/presentation/player/airplay_guard.dart`
- Test: `mobile/test/presentation/airplay_guard_test.dart`

**Interfaces:**
- Consumes: `AirplayStatusEvent`, `AirplayPlaybackState` (Tarea 6).
- Produces: `AirplayGuard({required void Function(String, {bool formatError}) onFatal, void Function()? onPlaybackConfirmed, Duration loadTimeout})` con `armLoadTimeout()`, `onStatus(AirplayStatusEvent)`, `dispose()`. Lo consume la Tarea 9.

- [ ] **Step 1: Escribir el test que falla**

Crear `mobile/test/presentation/airplay_guard_test.dart`:

```dart
import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/data/airplay/airplay_platform.dart';
import 'package:korven_open_tv/presentation/player/airplay_guard.dart';

void main() {
  test('sin reproducción dentro del presupuesto, fatal', () {
    fakeAsync((async) {
      String? fatal;
      final g = AirplayGuard(onFatal: (m, {formatError = false}) => fatal = m);
      g.armLoadTimeout();

      async.elapse(const Duration(seconds: 14));
      expect(fatal, isNull, reason: 'aún dentro del presupuesto');

      async.elapse(const Duration(seconds: 2));
      expect(fatal, isNotNull);
      g.dispose();
    });
  });

  test('playing confirma y desarma el watchdog', () {
    fakeAsync((async) {
      String? fatal;
      var confirmado = 0;
      final g = AirplayGuard(
        onFatal: (m, {formatError = false}) => fatal = m,
        onPlaybackConfirmed: () => confirmado++,
      );
      g.armLoadTimeout();
      g.onStatus(const AirplayStatusEvent(state: AirplayPlaybackState.playing));

      async.elapse(const Duration(seconds: 30));
      expect(fatal, isNull);
      expect(confirmado, 1);
      g.dispose();
    });
  });

  test('un fallo es terminal a la primera, incluso ya reproduciendo', () {
    fakeAsync((async) {
      String? fatal;
      final g = AirplayGuard(onFatal: (m, {formatError = false}) => fatal = m);
      g.armLoadTimeout();
      g.onStatus(const AirplayStatusEvent(state: AirplayPlaybackState.playing));
      g.onStatus(const AirplayStatusEvent(
          state: AirplayPlaybackState.failed, error: 'roto'));

      expect(fatal, 'roto',
          reason: 'AVPlayer no emite errores transitorios como mpv');
      g.dispose();
    });
  });

  test('propaga si el fallo fue de formato', () {
    fakeAsync((async) {
      bool? formato;
      final g = AirplayGuard(
          onFatal: (m, {formatError = false}) => formato = formatError);
      g.armLoadTimeout();
      g.onStatus(const AirplayStatusEvent(
        state: AirplayPlaybackState.failed,
        error: 'códec',
        formatError: true,
      ));
      expect(formato, isTrue);
      g.dispose();
    });
  });

  test('tras dispose no llama a nadie', () {
    fakeAsync((async) {
      var llamadas = 0;
      final g = AirplayGuard(onFatal: (m, {formatError = false}) => llamadas++);
      g.armLoadTimeout();
      g.dispose();
      async.elapse(const Duration(seconds: 30));
      expect(llamadas, 0);
    });
  });
}
```

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd mobile && flutter test test/presentation/airplay_guard_test.dart
```

Esperado: FAIL, no se resuelve `airplay_guard.dart`.

- [ ] **Step 3: Implementar**

Crear `mobile/lib/presentation/player/airplay_guard.dart`:

```dart
import 'dart:async';

import '../../data/airplay/airplay_platform.dart';

/// Vigila la carga de una emisión AirPlay.
///
/// Hermano de PlaybackGuard, no una refactorización suya: los dos clasifican al
/// revés. mpv escupe errores recuperables constantemente en HLS en vivo y
/// tratarlos como fatales mataba el vídeo, así que PlaybackGuard los ignora
/// mientras la posición avance. AVPlayer no hace eso: un
/// AVPlayerItem.status == .failed es terminal a la primera. Fundir ambos
/// corrompería el comportamiento de uno de los dos.
class AirplayGuard {
  AirplayGuard({
    required this.onFatal,
    this.onPlaybackConfirmed,
    this.loadTimeout = const Duration(seconds: 15),
  });

  /// [formatError] distingue "no puedo decodificar esto" de "no llegué al
  /// servidor". Solo lo primero autoriza a recordar el canal como incompatible.
  final void Function(String message, {bool formatError}) onFatal;

  final void Function()? onPlaybackConfirmed;
  final Duration loadTimeout;

  Timer? _loadTimer;
  bool _confirmado = false;
  bool _disposed = false;

  /// Armar ANTES de invocar start(): resolver la URL contra el gateway puede
  /// colgarse igual que la carga del stream, y el presupuesto cubre todo el
  /// proceso. Misma razón que en PlaybackGuard.
  void armLoadTimeout() {
    _loadTimer?.cancel();
    _loadTimer = Timer(loadTimeout, () {
      if (_disposed || _confirmado) return;
      onFatal(
        'El canal no llegó a reproducirse en el televisor '
        'en ${loadTimeout.inSeconds}s.',
        formatError: false,
      );
    });
  }

  void onStatus(AirplayStatusEvent e) {
    if (_disposed) return;
    switch (e.state) {
      case AirplayPlaybackState.playing:
        if (_confirmado) return;
        _confirmado = true;
        _loadTimer?.cancel();
        onPlaybackConfirmed?.call();
      case AirplayPlaybackState.failed:
        _loadTimer?.cancel();
        onFatal(e.error ?? 'Fallo de reproducción en el televisor.',
            formatError: e.formatError);
      case AirplayPlaybackState.loading:
        break;
    }
  }

  void dispose() {
    _disposed = true;
    _loadTimer?.cancel();
  }
}
```

- [ ] **Step 4: Ejecutar los tests y comprobar que pasan**

```bash
cd mobile && flutter test test/presentation/airplay_guard_test.dart && flutter analyze
```

Esperado: PASS en los 5 tests.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/presentation/player/airplay_guard.dart mobile/test/presentation/airplay_guard_test.dart
git commit -m "app: AirplayGuard, hermano de PlaybackGuard y no refactor suyo

Los dos clasifican al revés. PlaybackGuard ignora los errores de mpv
mientras la posición avance porque en HLS en vivo son ruido; AVPlayer no
emite ese ruido y un .failed suyo es terminal a la primera. Fundirlos
corrompería el comportamiento de uno de los dos."
```

---

## Tarea 8: Memoria local de canales incompatibles

**Files:**
- Create: `mobile/lib/presentation/providers/airplay_memory_provider.dart`
- Test: `mobile/test/presentation/airplay_memory_test.dart`

**Interfaces:**
- Consumes: `sharedPreferencesProvider` (`presentation/providers/channel_provider.dart:14`).
- Produces: `airplayMemoryProvider` → `NotifierProvider<AirplayMemoryNotifier, Set<String>>` con `void marcarIncompatible(String channelId)` y `bool esIncompatible(String channelId)`. Lo consumen las tareas 9 y 12.

- [ ] **Step 1: Escribir el test que falla**

Crear `mobile/test/presentation/airplay_memory_test.dart`:

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/presentation/providers/airplay_memory_provider.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> contenedor([Map<String, Object> inicial = const {}]) async {
    SharedPreferences.setMockInitialValues(inicial);
    final prefs = await SharedPreferences.getInstance();
    return ProviderContainer(
      overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
    );
  }

  test('arranca vacío', () async {
    final c = await contenedor();
    expect(c.read(airplayMemoryProvider), isEmpty);
  });

  test('marcar persiste y se consulta', () async {
    final c = await contenedor();
    c.read(airplayMemoryProvider.notifier).marcarIncompatible('bbc');
    expect(c.read(airplayMemoryProvider), contains('bbc'));
    expect(c.read(airplayMemoryProvider.notifier).esIncompatible('bbc'), isTrue);
    expect(c.read(airplayMemoryProvider.notifier).esIncompatible('cnn'), isFalse);
  });

  test('lee lo persistido en un arranque anterior', () async {
    final c = await contenedor({
      'airplay_incompatibles': <String>['cnn'],
    });
    expect(c.read(airplayMemoryProvider), contains('cnn'));
  });

  test('marcar dos veces no duplica ni rompe', () async {
    final c = await contenedor();
    final n = c.read(airplayMemoryProvider.notifier);
    n.marcarIncompatible('bbc');
    n.marcarIncompatible('bbc');
    expect(c.read(airplayMemoryProvider).length, 1);
  });
}
```

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd mobile && flutter test test/presentation/airplay_memory_test.dart
```

Esperado: FAIL, no se resuelve `airplay_memory_provider.dart`.

- [ ] **Step 3: Implementar**

Crear `mobile/lib/presentation/providers/airplay_memory_provider.dart`:

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'channel_provider.dart';

/// Canales que AVFoundation ya rechazó por formato, como conjunto de IDs en
/// SharedPreferences. Mismo patrón que FavoritesNotifier: para un conjunto de
/// identificadores no hace falta sqflite.
///
/// Vive en el cliente y no en el gateway a propósito. Los handlers reciben un
/// pool de solo lectura, y abrir un endpoint de escritura en una API sin
/// autenticación para guardar esto sería un mal cambio. Además es la única
/// fuente que tendrá datos: sin relleno de fondo, /channels no trae
/// compatibilidad para casi ningún canal.
class AirplayMemoryNotifier extends Notifier<Set<String>> {
  static const _key = 'airplay_incompatibles';

  @override
  Set<String> build() =>
      ref.watch(sharedPreferencesProvider).getStringList(_key)?.toSet() ??
      <String>{};

  void marcarIncompatible(String channelId) {
    if (state.contains(channelId)) return;
    final nuevo = Set<String>.from(state)..add(channelId);
    state = nuevo;
    ref.read(sharedPreferencesProvider).setStringList(_key, nuevo.toList());
  }

  bool esIncompatible(String channelId) => state.contains(channelId);
}

final airplayMemoryProvider =
    NotifierProvider<AirplayMemoryNotifier, Set<String>>(
  AirplayMemoryNotifier.new,
);
```

- [ ] **Step 4: Ejecutar los tests y comprobar que pasan**

```bash
cd mobile && flutter test test/presentation/airplay_memory_test.dart && flutter analyze
```

Esperado: PASS en los 4 tests.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/presentation/providers/airplay_memory_provider.dart mobile/test/presentation/airplay_memory_test.dart
git commit -m "app: memoria local de canales que AirPlay rechaza por formato

En el cliente y no en el gateway: los handlers tienen pool de solo
lectura, y abrir escritura en una API sin auth para guardar esto sería un
mal cambio. Además es la única fuente con datos — sin relleno de fondo,
/channels no trae compatibilidad para casi ningún canal."
```

---

## Tarea 9: La máquina de estados

**Files:**
- Create: `mobile/lib/presentation/providers/cast_provider.dart`
- Test: `mobile/test/presentation/cast_provider_test.dart`

**Interfaces:**
- Consumes: `AirplayPlatform`/`airplayPlatformProvider`/`AirplayEvent` (T6), `CastSession`/`CastState` (T6), `AirplayGuard` (T7), `airplayMemoryProvider` (T8), `channelRepositoryProvider` (`channel_provider.dart:9`), `Channel` (`domain/models/channel.dart`).
- Produces: `castProvider` → `NotifierProvider<CastNotifier, CastSession>` con `Future<void> reproducir(Channel ch)`, `Future<void> detener()`, `void reconocerFallo()`. Lo consumen las tareas 11 y 12.

- [ ] **Step 1: Escribir el test que falla**

Crear `mobile/test/presentation/cast_provider_test.dart`:

```dart
import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/data/airplay/airplay_platform.dart';
import 'package:korven_open_tv/data/repositories/channel_repository.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';
import 'package:korven_open_tv/domain/models/channel.dart';
import 'package:korven_open_tv/domain/models/channel_filter.dart';
import 'package:korven_open_tv/presentation/providers/airplay_memory_provider.dart';
import 'package:korven_open_tv/presentation/providers/cast_provider.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

class FakeAirplayPlatform implements AirplayPlatform {
  final _ctrl = StreamController<AirplayEvent>.broadcast();
  final List<String> iniciados = [];
  int paradas = 0;

  @override
  Stream<AirplayEvent> get events => _ctrl.stream;

  @override
  Future<void> start({required String url, required String title}) async {
    iniciados.add(url);
  }

  @override
  Future<void> stop() async {
    paradas++;
  }

  void emitir(AirplayEvent e) => _ctrl.add(e);
  void cerrar() => _ctrl.close();
}

class FakeChannelRepo implements IChannelRepository {
  Object? error;

  @override
  Future<String> getStreamUrl(String channelId) async {
    if (error != null) throw error!;
    return 'http://stream/$channelId.m3u8';
  }

  // filter lleva default y no `required`: en un override no se puede endurecer
  // un parámetro opcional de la interfaz.
  @override
  Future<ChannelPage> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit = 500,
    int offset = 0,
    List<String>? ids,
  }) async =>
      const ChannelPage(channels: [], total: 0);

  @override
  Future<Channel> getRandomChannel(ChannelFilter filter) async =>
      const Channel(
        id: 'x',
        name: 'x',
        logoUrl: '',
        categoryId: '',
        languageCode: '',
        countryCode: '',
        providerType: '',
      );
}

const _bbc = Channel(
  id: 'bbc',
  name: 'BBC News',
  logoUrl: '',
  categoryId: '',
  languageCode: '',
  countryCode: 'GB',
  providerType: 'opensource',
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late FakeAirplayPlatform plataforma;
  late FakeChannelRepo repo;

  Future<ProviderContainer> contenedor() async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    plataforma = FakeAirplayPlatform();
    repo = FakeChannelRepo();
    final c = ProviderContainer(overrides: [
      sharedPreferencesProvider.overrideWithValue(prefs),
      airplayPlatformProvider.overrideWithValue(plataforma),
      channelRepositoryProvider.overrideWithValue(repo),
    ]);
    addTearDown(() {
      c.dispose();
      plataforma.cerrar();
    });
    // Forzar build() para que se suscriba a los eventos.
    c.read(castProvider);
    return c;
  }

  test('una ruta activa arma la sesión', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await Future<void>.delayed(Duration.zero);

    final s = c.read(castProvider);
    expect(s.state, CastState.armed);
    expect(s.deviceName, 'Salón');
  });

  test('reproducir resuelve la URL y arranca la emisión', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);

    expect(plataforma.iniciados, ['http://stream/bbc.m3u8']);
    final s = c.read(castProvider);
    expect(s.state, CastState.connecting);
    expect(s.channelName, 'BBC News');
  });

  test('playing pasa a casting', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(
        const AirplayStatusEvent(state: AirplayPlaybackState.playing));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.casting);
  });

  test('cambiar de canal no vuelve a pedir dispositivo', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await Future<void>.delayed(Duration.zero);

    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(
        const AirplayStatusEvent(state: AirplayPlaybackState.playing));
    await Future<void>.delayed(Duration.zero);

    const cnn = Channel(
      id: 'cnn',
      name: 'CNN',
      logoUrl: '',
      categoryId: '',
      languageCode: '',
      countryCode: 'US',
      providerType: 'opensource',
    );
    await c.read(castProvider.notifier).reproducir(cnn);

    expect(plataforma.iniciados.length, 2);
    expect(c.read(castProvider).deviceName, 'Salón',
        reason: 'el dispositivo sobrevive al cambio de canal');
  });

  test('un fallo de formato pasa a failed y recuerda el canal', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayStatusEvent(
      state: AirplayPlaybackState.failed,
      error: 'códec no soportado',
      formatError: true,
    ));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.failed);
    expect(plataforma.paradas, greaterThan(0));
    expect(c.read(airplayMemoryProvider), contains('bbc'));
  });

  test('un fallo de red NO marca el canal como incompatible', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayStatusEvent(
      state: AirplayPlaybackState.failed,
      error: 'sin conexión',
      formatError: false,
    ));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.failed);
    expect(c.read(airplayMemoryProvider), isEmpty,
        reason: 'un Apple TV dormido no vuelve incompatible un canal bueno');
  });

  test('perder la ruta a mitad de emisión devuelve a idle', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayRouteEvent(active: false));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.idle);
  });

  test('detener para la emisión y conserva la ruta armada', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await c.read(castProvider.notifier).reproducir(_bbc);
    await c.read(castProvider.notifier).detener();

    expect(plataforma.paradas, greaterThan(0));
    expect(c.read(castProvider).state, CastState.armed);
  });

  test('un gateway caído deja la sesión en failed', () async {
    final c = await contenedor();
    repo.error = Exception('conexión rechazada');
    await c.read(castProvider.notifier).reproducir(_bbc);

    final s = c.read(castProvider);
    expect(s.state, CastState.failed);
    expect(s.error, isNotNull);
  });

  test('reconocerFallo devuelve a armed si la ruta sigue puesta', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayStatusEvent(
        state: AirplayPlaybackState.failed, error: 'x'));
    await Future<void>.delayed(Duration.zero);

    c.read(castProvider.notifier).reconocerFallo();
    expect(c.read(castProvider).state, CastState.armed);
  });
}
```

> **Nota:** si `IChannelRepository` tiene métodos distintos de los del doble, copia sus firmas exactas de `mobile/lib/data/repositories/channel_repository.dart`. El doble debe implementar la interfaz entera.

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd mobile && flutter test test/presentation/cast_provider_test.dart
```

Esperado: FAIL, no se resuelve `cast_provider.dart`.

- [ ] **Step 3: Implementar**

Crear `mobile/lib/presentation/providers/cast_provider.dart`:

```dart
import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/airplay/airplay_platform.dart';
import '../../data/api_error.dart';
import '../../domain/models/cast_session.dart';
import '../../domain/models/channel.dart';
import '../player/airplay_guard.dart';
import 'airplay_memory_provider.dart';
import 'channel_provider.dart';

/// Sesión de emisión AirPlay a nivel de app.
///
/// No navega nunca. Publica [CastState.failed] con el motivo y es la pantalla
/// activa quien decide qué hacer con eso. Mantener la navegación fuera de aquí
/// es lo que permite testear el traspaso sin WidgetTester ni árbol de widgets,
/// que es obligatorio: CI corre en ubuntu-latest.
class CastNotifier extends Notifier<CastSession> {
  late final AirplayPlatform _plataforma;
  StreamSubscription<AirplayEvent>? _sub;
  AirplayGuard? _guard;

  /// Generación de carga, por el mismo motivo que en PlayerScreen: dos toques
  /// rápidos en canales distintos dejaban que la resolución más lenta pisara el
  /// estado de la más reciente al volver de su await.
  int _generacion = 0;

  @override
  CastSession build() {
    _plataforma = ref.watch(airplayPlatformProvider);
    _sub = _plataforma.events.listen(_alEvento);
    ref.onDispose(() {
      _sub?.cancel();
      _guard?.dispose();
    });
    return const CastSession();
  }

  Future<void> reproducir(Channel ch) async {
    final generacion = ++_generacion;

    _guard?.dispose();
    final guard = AirplayGuard(
      onFatal: (mensaje, {formatError = false}) =>
          _alFallo(ch, mensaje, formatError: formatError),
    );
    _guard = guard;
    // Armar antes de resolver la URL: el gateway puede colgarse igual que la
    // carga del stream, y el presupuesto cubre todo el proceso.
    guard.armLoadTimeout();

    state = state.copyWith(
      state: CastState.connecting,
      channelId: ch.id,
      channelName: ch.name,
      clearError: true,
    );

    try {
      final url = await ref.read(channelRepositoryProvider).getStreamUrl(ch.id);
      if (generacion != _generacion) return;
      await _plataforma.start(url: url, title: ch.name);
    } catch (e) {
      if (generacion != _generacion) return;
      guard.dispose();
      _alFallo(ch, ApiError.desde(e).mensaje, formatError: false);
    }
  }

  Future<void> detener() async {
    _guard?.dispose();
    _guard = null;
    await _plataforma.stop();
    // A armed y no a idle: la ruta del sistema sigue seleccionada — una app no
    // puede deseleccionarla, esa UI es de Apple — así que decir "desconectado"
    // sería mentir.
    state = state.copyWith(
      state: state.deviceName == null ? CastState.idle : CastState.armed,
      clearChannel: true,
      clearError: true,
    );
  }

  /// La pantalla llama a esto cuando ya ha consumido el traspaso y ha puesto el
  /// canal en local.
  void reconocerFallo() {
    if (state.state != CastState.failed) return;
    state = state.copyWith(
      state: state.deviceName == null ? CastState.idle : CastState.armed,
      clearChannel: true,
      clearError: true,
    );
  }

  void _alFallo(Channel ch, String mensaje, {required bool formatError}) {
    // Solo los fallos de formato enseñan algo sobre el canal. Un Apple TV
    // dormido y un stream MPEG-2 llegan los dos como fallo, y marcar por red
    // etiquetaría canales buenos para siempre.
    if (formatError) {
      ref.read(airplayMemoryProvider.notifier).marcarIncompatible(ch.id);
    }
    unawaited(_plataforma.stop());
    state = state.copyWith(state: CastState.failed, error: mensaje);
  }

  void _alEvento(AirplayEvent e) {
    switch (e) {
      case AirplayRouteEvent(active: final activa, name: final nombre):
        if (!activa) {
          _guard?.dispose();
          _guard = null;
          state = const CastSession();
          return;
        }
        state = state.copyWith(
          state: state.state == CastState.idle ? CastState.armed : state.state,
          deviceName: nombre,
        );
      case AirplayStatusEvent():
        _guard?.onStatus(e);
        if (e.state == AirplayPlaybackState.playing) {
          state = state.copyWith(state: CastState.casting, clearError: true);
        }
    }
  }
}

final castProvider =
    NotifierProvider<CastNotifier, CastSession>(CastNotifier.new);
```

- [ ] **Step 4: Ejecutar los tests y comprobar que pasan**

```bash
cd mobile && flutter test test/presentation/cast_provider_test.dart && flutter analyze
```

Esperado: PASS en los 10 tests.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/presentation/providers/cast_provider.dart mobile/test/presentation/cast_provider_test.dart
git commit -m "app: máquina de estados de la sesión de emisión

No navega nunca: publica failed con el motivo y la pantalla decide. Sacar
la navegación del notifier es lo que permite testear el traspaso sin
árbol de widgets, que es obligatorio con CI en ubuntu-latest.

detener() vuelve a armed y no a idle porque la ruta del sistema sigue
puesta — una app no puede deseleccionarla, esa UI es de Apple."
```

---

## Tarea 10: Botón de AirPlay

**Files:**
- Create: `mobile/lib/presentation/widgets/airplay_button.dart`
- Test: `mobile/test/presentation/airplay_button_test.dart`

**Interfaces:**
- Consumes: platform view `dev.korven.opentv/route-picker` (Tarea 4).
- Produces: `const AirplayButton({super.key})`. Lo consume la Tarea 12.

- [ ] **Step 1: Escribir el test que falla**

Crear `mobile/test/presentation/airplay_button_test.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/presentation/widgets/airplay_button.dart';

void main() {
  testWidgets('fuera de macOS no ocupa sitio', (tester) async {
    // El test corre en el binding de test, donde defaultTargetPlatform no es
    // macOS: es exactamente el caso que debe colapsar. Sin esto, CI (Linux)
    // reventaría al intentar crear un AppKitView.
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: AirplayButton())),
    );
    expect(find.byType(AirplayButton), findsOneWidget);
    final caja = tester.getSize(find.byType(AirplayButton));
    expect(caja.width, 0);
    expect(caja.height, 0);
  });
}
```

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd mobile && flutter test test/presentation/airplay_button_test.dart
```

Esperado: FAIL, no se resuelve `airplay_button.dart`.

- [ ] **Step 3: Implementar**

Crear `mobile/lib/presentation/widgets/airplay_button.dart`:

```dart
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

/// El AVRoutePickerView nativo, embebido.
///
/// Es el botón real de Apple y no una réplica: el descubrimiento de
/// dispositivos, el emparejamiento con tvOS y la lista de rutas los resuelve
/// AVFoundation. Reimplementarlo exigiría el handshake HAP, que es frágil y no
/// tiene soporte.
///
/// Fuera de macOS colapsa a cero para que los targets de la Fase 9 compilen sin
/// tocarlo — y para que CI, que corre en Linux, no intente crear un AppKitView.
class AirplayButton extends StatelessWidget {
  const AirplayButton({super.key});

  static const _viewType = 'dev.korven.opentv/route-picker';
  static const _lado = 28.0;

  @override
  Widget build(BuildContext context) {
    if (defaultTargetPlatform != TargetPlatform.macOS || kIsWeb) {
      return const SizedBox.shrink();
    }
    return const SizedBox(
      width: _lado,
      height: _lado,
      // Sin creationParams: la vista nativa no recibe argumentos. El selector
      // abre su propio popover al hacer clic, y hitTestBehavior.opaque, que es
      // el valor por defecto, ya deja que los clics lleguen al NSView.
      child: AppKitView(viewType: _viewType),
    );
  }
}
```

- [ ] **Step 4: Ejecutar los tests y comprobar que pasan**

```bash
cd mobile && flutter test test/presentation/airplay_button_test.dart && flutter analyze
```

Esperado: PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/presentation/widgets/airplay_button.dart mobile/test/presentation/airplay_button_test.dart
git commit -m "app: botón de AirPlay con el selector nativo embebido

Es el botón real de Apple, no una réplica: descubrimiento, emparejamiento
con tvOS y lista de rutas los resuelve AVFoundation. Fuera de macOS
colapsa a cero para que CI en Linux no intente crear un AppKitView."
```

---

## Tarea 11: Barra de sesión

**Files:**
- Create: `mobile/lib/presentation/widgets/cast_bar.dart`
- Test: `mobile/test/presentation/cast_bar_test.dart`

**Interfaces:**
- Consumes: `castProvider`, `CastSession`, `CastState` (T6, T9), `KorvenColors`, `KorvenSpacing`.
- Produces: `const CastBar({super.key})`. Lo consume la Tarea 12.

- [ ] **Step 1: Escribir el test que falla**

Crear `mobile/test/presentation/cast_bar_test.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';
import 'package:korven_open_tv/presentation/providers/cast_provider.dart';
import 'package:korven_open_tv/presentation/widgets/cast_bar.dart';

class _CastFijo extends CastNotifier {
  _CastFijo(this._inicial);
  final CastSession _inicial;

  @override
  CastSession build() => _inicial;
}

Future<void> _montar(WidgetTester tester, CastSession sesion) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [castProvider.overrideWith(() => _CastFijo(sesion))],
      child: const MaterialApp(home: Scaffold(body: CastBar())),
    ),
  );
}

void main() {
  testWidgets('en idle no se dibuja', (tester) async {
    await _montar(tester, const CastSession());
    expect(find.byType(SizedBox), findsWidgets);
    expect(find.textContaining('AirPlay'), findsNothing);
  });

  testWidgets('armed muestra el dispositivo', (tester) async {
    await _montar(tester,
        const CastSession(state: CastState.armed, deviceName: 'Salón Apple TV'));
    expect(find.textContaining('Salón Apple TV'), findsOneWidget);
  });

  testWidgets('casting muestra dispositivo y canal', (tester) async {
    await _montar(
      tester,
      const CastSession(
        state: CastState.casting,
        deviceName: 'Salón Apple TV',
        channelName: 'BBC News',
      ),
    );
    expect(find.textContaining('Salón Apple TV'), findsOneWidget);
    expect(find.textContaining('BBC News'), findsOneWidget);
  });

  testWidgets('sin nombre de dispositivo cae a AirPlay', (tester) async {
    await _montar(tester, const CastSession(state: CastState.armed));
    expect(find.textContaining('AirPlay'), findsOneWidget);
  });
}
```

- [ ] **Step 2: Ejecutar el test y comprobar que falla**

```bash
cd mobile && flutter test test/presentation/cast_bar_test.dart
```

Esperado: FAIL, no se resuelve `cast_bar.dart`.

- [ ] **Step 3: Implementar**

Crear `mobile/lib/presentation/widgets/cast_bar.dart`:

```dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/models/cast_session.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../providers/cast_provider.dart';

/// Barra persistente de la sesión de emisión. Se dibuja en cuanto hay una ruta
/// puesta, aunque todavía no suene nada: elegir dispositivo y elegir canal son
/// dos actos distintos.
class CastBar extends ConsumerWidget {
  const CastBar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sesion = ref.watch(castProvider);
    if (!sesion.visible) return const SizedBox.shrink();

    return Container(
      height: 44,
      padding: const EdgeInsets.symmetric(horizontal: KorvenSpacing.s4),
      decoration: const BoxDecoration(
        color: KorvenColors.surfaceBase,
        border: Border(top: BorderSide(color: KorvenColors.borderSubtle)),
      ),
      child: Row(
        children: [
          const Icon(Icons.airplay, size: 16, color: KorvenColors.textMuted),
          const SizedBox(width: KorvenSpacing.s3),
          Expanded(
            child: Text(
              _texto(sesion),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontFamily: 'JetBrainsMono',
                fontSize: 12,
                color: KorvenColors.textMuted,
              ),
            ),
          ),
          IconButton(
            iconSize: 18,
            splashRadius: 16,
            tooltip: 'Terminar sesión',
            icon: const Icon(Icons.stop, color: KorvenColors.textMuted),
            onPressed: () => ref.read(castProvider.notifier).detener(),
          ),
        ],
      ),
    );
  }

  String _texto(CastSession s) {
    final destino = s.etiquetaDispositivo;
    return switch (s.state) {
      CastState.armed => '$destino · listo para emitir',
      CastState.connecting => '$destino · abriendo ${s.channelName ?? ''}',
      CastState.casting => '$destino · ${s.channelName ?? ''}',
      CastState.failed => '$destino · ${s.error ?? 'fallo'}',
      CastState.idle => destino,
    };
  }
}
```

> **Nota:** `surfaceBase`, `borderSubtle` y `textMuted` existen en `korven_colors.dart` (líneas 44, 56, 52). No inventes tokens nuevos ni uses literales de color.

- [ ] **Step 4: Ejecutar los tests y comprobar que pasan**

```bash
cd mobile && flutter test test/presentation/cast_bar_test.dart && flutter analyze
```

Esperado: PASS en los 4 tests.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/presentation/widgets/cast_bar.dart mobile/test/presentation/cast_bar_test.dart
git commit -m "app: barra persistente de sesión de emisión

Se dibuja en cuanto hay ruta puesta, aunque no suene nada: elegir
dispositivo y elegir canal son dos actos distintos."
```

---

## Tarea 12: Cableado en las pantallas y toque condicional

**Files:**
- Modify: `mobile/lib/presentation/widgets/console_bar.dart` (hueco para acciones)
- Modify: `mobile/lib/presentation/screens/home_screen.dart:79-89, 112-126`
- Modify: `mobile/lib/presentation/screens/player_screen.dart:152-178`
- Test: `mobile/test/presentation/home_cast_test.dart`

**Interfaces:**
- Consumes: `castProvider`/`CastNotifier` (T9), `AirplayButton` (T10), `CastBar` (T11), `airplayMemoryProvider` (T8).
- Produces: comportamiento final. Nada aguas abajo.

- [ ] **Step 1: Escribir el test que falla**

Crear `mobile/test/presentation/home_cast_test.dart`:

```dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';

void main() {
  test('con sesión interceptando, el toque emite en vez de abrir el player', () {
    // El comportamiento condicional del toque vive en CastSession.intercepta:
    // home_screen lo consulta en _abrirCanal. Verificarlo aquí, sobre el
    // modelo, evita montar el árbol entero con gateway falso.
    const armada = CastSession(state: CastState.armed, deviceName: 'Salón');
    const emitiendo = CastSession(state: CastState.casting, deviceName: 'Salón');
    const parada = CastSession();
    const fallida = CastSession(state: CastState.failed, error: 'x');

    expect(armada.intercepta, isTrue);
    expect(emitiendo.intercepta, isTrue);
    expect(parada.intercepta, isFalse);
    expect(fallida.intercepta, isFalse,
        reason: 'tras un traspaso el canal va a local, no al televisor');
  });
}
```

- [ ] **Step 2: Ejecutar el test**

```bash
cd mobile && flutter test test/presentation/home_cast_test.dart
```

Esperado: PASS (el modelo ya existe desde la Tarea 6). Este test fija el contrato antes de cablearlo.

- [ ] **Step 3: Hueco para acciones en `ConsoleBar`**

En `mobile/lib/presentation/widgets/console_bar.dart`, añadir un parámetro
opcional al constructor y al campo:

```dart
    this.leadingActions = const <Widget>[],
```

```dart
  /// Acciones que van antes de las propias de la barra. El botón de AirPlay
  /// entra por aquí en vez de hardcodearse: ConsoleBar no debe saber que
  /// existe una sesión de emisión.
  final List<Widget> leadingActions;
```

En el `Row` del `build`, dentro de la rama `else ...[` que contiene las acciones
(la que empieza con `_AccionBarra(icon: Icons.search, ...)`), insertar
`...leadingActions,` como **primer** elemento, antes de la acción de búsqueda.
No tocar la rama `if (searching)`: durante la búsqueda la barra solo muestra el
botón de cerrar, y meter ahí el selector le robaría sitio al campo de texto.

- [ ] **Step 4: Cablear `HomeScreen`**

En `mobile/lib/presentation/screens/home_screen.dart`, sustituir `_abrirCanal`
(líneas 79-89) por:

```dart
  /// Con sesión de emisión activa el toque va al televisor y la rejilla no se
  /// abandona: es el punto entero de la segunda pantalla. Sin sesión, se
  /// comporta como siempre.
  void _abrirCanal(BuildContext context, Channel ch) {
    if (ref.read(castProvider).intercepta) {
      ref.read(castProvider.notifier).reproducir(ch);
      return;
    }
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => PlayerScreen(
          channelId: ch.id,
          channelName: ch.name,
          countryCode: ch.countryCode,
        ),
      ),
    );
  }
```

En `build`, añadir el botón a la barra y la `CastBar` al scaffold:

```dart
      appBar: ConsoleBar(
        leadingActions: const [AirplayButton()],
        searching: _searching,
```

```dart
      bottomNavigationBar: const CastBar(),
```

Y escuchar el traspaso, dentro de `build` antes del `return`:

```dart
    // El notifier no navega: publica failed y aquí se decide. Al fallar la
    // emisión, el canal se abre en local con media_kit y se reconoce el fallo
    // para que la barra vuelva a "listo para emitir".
    ref.listen(castProvider, (anterior, actual) {
      if (actual.state != CastState.failed) return;
      final id = actual.channelId;
      final nombre = actual.channelName;
      ref.read(castProvider.notifier).reconocerFallo();
      if (id == null || nombre == null) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('// este canal no viaja a AirPlay — '
            'reproduciendo en local')),
      );
      Navigator.of(context).push(
        MaterialPageRoute(
          builder: (_) => PlayerScreen(channelId: id, channelName: nombre),
        ),
      );
    });
```

Añadir los imports que falten: `../providers/cast_provider.dart`,
`../widgets/airplay_button.dart`, `../widgets/cast_bar.dart`,
`../../domain/models/cast_session.dart`.

- [ ] **Step 5: Cablear `PlayerScreen`**

En `mobile/lib/presentation/screens/player_screen.dart`, en `actions` del
`AppBar` (línea 167), añadir el botón como primer elemento:

```dart
        actions: [
          const AirplayButton(),
          if (!_isLoading)
```

Y en el `Scaffold` (línea 152), añadir:

```dart
      bottomNavigationBar: const CastBar(),
```

Añadir los imports de `../widgets/airplay_button.dart` y `../widgets/cast_bar.dart`.

- [ ] **Step 6: Marca `sin airplay` en la fila de canal**

En `mobile/lib/presentation/widgets/channel_row.dart`, dentro del `Row` de
metadatos, añadir:

```dart
        if (ref.watch(castProvider).intercepta &&
            ref.watch(airplayMemoryProvider).contains(widget.channel.id))
          const Padding(
            padding: EdgeInsets.only(left: KorvenSpacing.s2),
            child: Text(
              'sin airplay',
              style: TextStyle(
                fontFamily: 'JetBrainsMono',
                fontSize: 10,
                color: KorvenColors.textMuted,
              ),
            ),
          ),
```

> **Nota:** solo con `intercepta` **y** memoria de fallo. Nunca por ausencia de
> dato: el 42 % del catálogo no se puede clasificar y marcarlo sería ruido. Si
> `ChannelRow` no es un `ConsumerStatefulWidget`, conviértelo — es el mismo
> cambio que ya hizo `FavoriteStar` para leer providers.

- [ ] **Step 7: Ejecutar toda la suite**

```bash
cd mobile && flutter analyze && flutter test
```

Esperado: PASS en toda la suite, incluidos `home_screen_test.dart` y
`channel_row_test.dart` preexistentes. Si alguno rompe por falta del
`ProviderScope` con `sharedPreferencesProvider`, añádeselo con el mismo patrón
que usa `favorites_provider_test.dart`.

- [ ] **Step 8: Compilar y arrancar**

```bash
cd mobile && flutter build macos --debug && flutter run -d macos
```

Esperado: el botón de AirPlay aparece en la barra superior.

- [ ] **Step 9: Commit**

```bash
git add mobile/lib/presentation/ mobile/test/presentation/home_cast_test.dart
git commit -m "app: cableado de la sesión de emisión en las dos pantallas

Con sesión activa el toque en la rejilla emite y no abandona la lista:
es el punto entero de la segunda pantalla.

La marca 'sin airplay' solo sale con memoria de fallo y sesión activa,
nunca por ausencia de dato: el 42 % del catálogo no se puede clasificar y
marcarlo convertiría la rejilla en ruido."
```

---

## Tarea 13: Verificación manual y documentación

Nada de esto lo cubre CI. Los siete puntos son el único gate real de la capa Swift.

**Files:**
- Modify: `PROMPT_MAESTRO.md` (sección 8, estado del proyecto)
- Modify: `README.md` (sección nueva de emisión)

- [ ] **Step 1: Ejecutar la verificación manual**

Con un Apple TV o televisor AirPlay 2 encendido en la misma red:

```bash
cd gateway && go run ./cmd/server   # terminal 1
cd mobile  && flutter run -d macos  # terminal 2
```

Comprobar y anotar el resultado de cada punto:

1. [ ] El selector lista el dispositivo. **Si la lista sale vacía, parar aquí**: es el riesgo #4 del spec (permiso de red local en macOS 15+) y bloquea todo lo demás.
2. [ ] Emitir un canal conocido bueno: vídeo en el televisor, la barra muestra el nombre del dispositivo.
3. [ ] Cambiar de canal desde la rejilla: el televisor cambia sin volver a pedir dispositivo.
4. [ ] Emitir un canal `.mpd`: traspaso a local en menos de 15 s, con el aviso.
5. [ ] Apagar el televisor a mitad de emisión: la barra desaparece.
6. [ ] Cerrar la app emitiendo: la ruta queda liberada.
7. [ ] Emitir un canal `http://`: reproduce, la excepción ATS funciona.

Para el punto 4, sacar un canal DASH del catálogo:

```bash
cd gateway && sqlite3 iptv.db \
  "select c.name from channels c join streams s on s.channel_id = c.id
   where s.url like '%.mpd%' limit 5;"
```

- [ ] **Step 2: Documentar en el README**

Añadir a `README.md`, tras la sección de configuración de la app:

```markdown
## Emisión a un televisor

Con un Apple TV o un televisor con AirPlay 2 en la misma red, el botón de
AirPlay de la barra superior abre el selector del sistema. Al elegir destino, la
barra inferior queda fija y los toques en la rejilla emiten al televisor en vez
de abrir el reproductor.

Solo AirPlay: no hay Chromecast, DLNA ni Roku.

Algunos canales se ven en el Mac y no viajan a AirPlay. AVFoundation es mucho
más estricto que libmpv y rechaza manifiestos y códecs que mpv reproduce sin
quejarse. Cuando pasa, la app se da cuenta en 15 s, reproduce el canal en local
y lo recuerda para marcarlo en la lista.
```

- [ ] **Step 3: Actualizar el estado en `PROMPT_MAESTRO.md`**

En la tabla "Completado ✓" de la sección 8, añadir:

```markdown
| **Emisión por AirPlay** | ✓ | AVPlayer nativo tras AVRoutePickerView; media_kit intacto para local |
```

En "Pendiente prioritario", dejar constancia de lo que se descartó:

```markdown
| Relleno de fondo de compatibilidad AirPlay | ✗ Descartado; el veredicto se sondea bajo demanda y se cachea en memoria |
| Google Cast / DLNA / Roku | ✗ Fuera de alcance |
```

- [ ] **Step 4: Ejecutar los gates completos**

```bash
cd gateway && gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./...
cd ../mobile && flutter analyze && flutter test
```

Esperado: todo verde.

- [ ] **Step 5: Commit**

```bash
git add README.md PROMPT_MAESTRO.md
git commit -m "docs: emisión por AirPlay en el README y en el estado del proyecto

Deja escrito lo que se descartó y por qué: sin relleno de fondo, sin
Cast, sin DLNA y sin Roku."
```
