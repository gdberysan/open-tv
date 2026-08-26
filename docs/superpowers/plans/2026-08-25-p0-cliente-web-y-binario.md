# P0 — Cliente web y binario único · Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que `go run ./cmd/open-tv` levante un solo proceso que sirve el catálogo Y un cliente web propio, y que los canales se reproduzcan en Chrome, Firefox y Safari en el Mac del autor, con `mobile/` intacto y CI verde.

**Architecture:** El módulo Go sube a la raíz del repo y gana un cliente Svelte 5 embebido con `go:embed`. El health-checker aprende un veredicto nuevo —`web_ok`— sobre el mismo manifiesto que ya descarga, y lo persiste por stream. Un proxy HLS que **solo existe cuando se escucha en loopback** cubre a Chrome y Firefox cuando el emisor no manda CORS. El cliente decide "directo primero, proxy si falla" y nunca se cuelga: el `PlaybackGuard` de la app Flutter se porta 1:1 a TypeScript.

**Tech Stack:** Go 1.25 + chi v5 + modernc/sqlite · Svelte 5 + TypeScript + Vite + hls.js · Vitest + Playwright (Chromium/Firefox/WebKit) · ffmpeg (fixtures HLS) · GitHub Actions

**Spec:** `docs/superpowers/specs/2026-08-22-open-tv-web-product-design.md` (§2.1, §3, §4, §6.3, §9, §10 P0). Para las trampas del código que este plan toca, `docs/superpowers/specs/2026-08-08-korven-open-tv-design.md` sigue siendo la referencia del catálogo.

## Global Constraints

- **Idioma:** todo el código, comentarios, mensajes de commit y texto de UI en **español**. Es la convención del repo entero. El texto de UI además existe en inglés (ver i18n).
- **Gates que deben quedar verdes al final de CADA tarea:**
  - `gofmt -l .` (vacío) · `go vet ./...` · `go build ./...` · `go test -race -count=1 ./...`
  - Desde la Tarea 10 en adelante, además: `cd web && npm run check && npm test`
  - `cd mobile && flutter analyze && flutter test`
- **`mobile/` no se toca.** Ni un diff bajo `mobile/` en todo P0. El job de Flutter de CI sigue existiendo y sigue verde. Si una tarea parece necesitar tocar Flutter, está mal planteada.
- **El contrato JSON con la app Flutter es aditivo y nada más.** `domain.Channel` gana exactamente un campo (`WebOK`); las 14 claves existentes conservan nombre, tipo y semántica. `domain/channel_test.go` pasa de exigir 14 claves a exigir 15, y eso ocurre **una sola vez**, en la Tarea 6.
- **`domain` sin imports externos.** Solo stdlib.
- **Cero `_` descartando errores en producción.** Errores envueltos con `fmt.Errorf("op: %w", err)`.
- **`slog` estructurado**, nunca `fmt.Println`.
- **El proxy solo se monta si la dirección de escucha es loopback.** No hay flag para forzarlo. La comprobación es del `net.Listener` real, no de la cadena de configuración.
- **Ninguna dependencia Go nueva.** El proxy, el embed y el veredicto web se hacen con stdlib + chi, que ya está.
- **Dependencias JS fijadas exactas** (`npm i --save-exact`). `web/package-lock.json` se versiona.
- **El ámbar `#FF8A2B` significa señal viva** —canal en directo, filtro activo, foco— y nada más. Un modo o un botón neutro NO lleva ámbar. Los tokens se copian **tal cual** desde `~/Dev/korven/design/design_handoff_korven_sitio/tokens/*.css`.
- **Presupuesto del cliente:** ≤ 80 KB gzip de JS propio. `hls.js` se carga con `import()` perezoso, solo al reproducir.
- **Identidad git:** todo commit como `gdberysan@gmail.com`. Antes de empujar: `git log --format='%ae' <rango> | sort -u`.
- **El repo sigue PRIVADO.** Nada en este plan lo hace público (eso es P1).
- Commits terminan con `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

### Desviaciones de la spec, decididas aquí

1. **§4.2 dice que `web_ok` es "hermano de `airplay_ok`" y que se guarda en el stream. `airplay_ok` HOY NO SE GUARDA.** Se sondea bajo demanda en `/channels/stream` (`handlers.AirplayProber`, caché en memoria, TTL 12 h) y el health-worker **calcula `res.Airplay` y lo tira** (`worker.go` solo persiste `IsAlive`/`LatencyMs`). Son hermanos en **semántica** —tri-estado, `null` = sin comprobar, lista de permitidos— no en almacenamiento. `web_ok` sí se persiste, porque la rejilla necesita pintarlo para 500 canales a la vez y un sondeo bajo demanda por canal no sirve para eso. `airplay_ok` se queda exactamente como está: cambiarlo tocaría el camino que consume la app Flutter, y `mobile/` no se toca.
2. **§4.2 dice "sin peticiones extra". Para conseguirlo, el checker pasa a GET directo en URLs HLS** en vez de HEAD→GET. Hoy el cuerpo del manifiesto solo se lee en el camino de fallback, así que clasificar "sobre el manifiesto que el checker ya descarga" cubriría únicamente a los orígenes que rechazan HEAD. Un GET directo es **una** petición, no dos: no añade tráfico de red, lo reduce. Lo no-HLS conserva HEAD→GET.
3. **§3.2 pide una marca `web_ok` en la tarjeta y el mensaje "este canal se ve en la app instalada / en Safari". Eso implica que `web_ok` es el veredicto ESTRICTO (hls.js), no el de Safari.** El censo lo confirma: Safari reproduce el 85 % (todo lo que sea HTTPS, sin necesitar CORS) y Chrome/Firefox el 67 % (HTTPS **y** CORS). Con un solo campo booleano se guarda el estricto, y **el cliente decide la política de intento según el motor**: en HLS nativo (Safari) se intenta directo siempre que el esquema sea HTTPS, aunque `web_ok` sea falso. Esa lógica es una función pura y testeada (`planDeReproduccion`, Tarea 13).
4. **§3.3 pide fixtures dorados generados por la suite Go para demostrar que `HttpCatalog` y `StaticCatalog` son equivalentes. En P0 solo existe `HttpCatalog`.** Se crea la costura `CatalogSource` completa (es la que hace posible P3) y se deja el generador de fixtures para P3, donde hay dos implementaciones que comparar. Comparar una implementación consigo misma no es un gate.
5. **§2.1 dice `internal/ui/` con "go:embed de web/dist". `go:embed` no admite `..`,** así que el cliente se construye directamente en `internal/ui/dist/` (`build.outDir` de Vite). El directorio lleva un `.gitkeep` versionado —sin al menos un fichero, `go:embed` falla en tiempo de compilación— y el resto se ignora.
6. **§9 pide Playwright en tres motores contra un stream real. La prueba de REPRODUCCIÓN no corre en WebKit dentro de CI.** El WebKit de Playwright sobre Linux no es Safari y su soporte de H.264/HLS depende de plugins de GStreamer que no están garantizados en `ubuntu-latest`. En CI: los tres motores corren catálogo, filtros, i18n y estados; la reproducción corre en Chromium y Firefox. **La reproducción en WebKit es un gate MANUAL en el Mac del autor, con Safari de verdad** (Tarea 17), que además es el navegador que importa. Si se declarase verde un WebKit de Linux que no decodifica, sería una puerta verde que no prueba nada.
7. **§6.3 dice que un segundo `open-tv` con uno ya corriendo "solo abre el navegador a la instancia existente". Se implementa en P0**, no en P2, porque es el mismo camino de código que el fallback de puerto y separarlos duplicaría la lógica.

---

## File Structure

Todas las rutas son relativas a la raíz del repo **después** de la Tarea 1.

**Movidos (Tarea 1, mecánica, `git mv`)**
- `gateway/{cmd,internal,go.mod,go.sum}` → raíz. `cmd/server` → `cmd/open-tv`.
- Módulo: `github.com/gdberysan/open-tv/gateway` → `github.com/gdberysan/open-tv`.

**Creados**
- `internal/datadir/datadir.go` — `Default()`, `RutaDB()`. Puro salvo `os.MkdirAll`. Sin dependencias del resto del proyecto.
- `internal/netx/escucha.go` — `EscuchaConFallback(addr string, intentos int) (net.Listener, error)`.
- `cmd/open-tv/navegador.go` — `AbrirNavegador(url string) error`, por `runtime.GOOS`.
- `cmd/open-tv/instancia.go` — `InstanciaViva(ctx, base string) bool`: pregunta a `/health` si el puerto ocupado es otro Open TV.
- `internal/domain/web.go` — `WebSupport`, `ClassifyWeb(finalURL, acao, body string) WebSupport`. Puro, sin E/S.
- `internal/proxy/manifiesto.go` — `ReescribirManifiesto(base *url.URL, cuerpo, prefijo string) string`. Puro.
- `internal/proxy/handler.go` — `Handler`, `NewHandler(...)`, ruta `GET /proxy/hls?u=`.
- `internal/ui/ui.go` — `//go:embed all:dist`, `Handler() (http.Handler, bool)`.
- `internal/ui/dist/.gitkeep` — versionado; el resto de `dist/` ignorado.
- `web/` — cliente Svelte 5. Detalle en la Tarea 10.
- `web/tests/e2e/` — Playwright: `global-setup.ts`, `catalogo.spec.ts`, `reproduccion.spec.ts`.
- `web/tests/fixtures/` — M3U local + HLS generado con ffmpeg (ignorado; lo genera el setup).
- `.github/workflows/ci.yml` — job `web` nuevo (se modifica el fichero existente).

**Modificados**
- `internal/api/router.go` — fuera CORS; monta UI, proxy y API en un solo `chi.Mux`.
- `internal/api/middleware/cors.go` — **borrado**; y sus casos en `middleware_test.go`.
- `internal/domain/channel.go` — campo `WebOK *bool` (clave 15).
- `internal/domain/channel_test.go` — 14 → 15 claves, una sola vez.
- `internal/domain/airplay.go` — `urlNoReproducible` y `atributoEntreComillas` pasan a compartirse con `web.go` (mismo paquete, sin cambios de firma).
- `internal/adapters/validator/{models.go,checker.go,worker.go}` — `StreamResult.Web`; GET directo para HLS; el worker propaga el veredicto.
- `internal/adapters/db/{schema.sql,db.go,stream_repository.go,channel_repository.go}` — columna `streams.web_ok`, ALTER idempotente, `MarkBatch` la escribe, `FindFiltered` la agrega por canal.
- `internal/ports/stream_repository.go` — `StreamHealth.Web`.
- `internal/api/handlers/health_handler.go` — `version`, `web_ui`, `proxy_enabled`.
- `cmd/open-tv/main.go` — subcomando `serve` por defecto, `--no-browser`, datadir, fallback de puerto, apertura del navegador.
- `.gitignore`, `README.md`, `tools/README.md`, `tools/dev.korven.opentv.gateway.plist`, `PROMPT_MAESTRO.md` §3.

---

## Tarea 1: El módulo Go sube a la raíz

Un único commit mecánico. CI es el juez: si compila, vetea, formatea y los tests pasan, el movimiento fue correcto. Nada de lógica nueva en esta tarea.

**Files:**
- Move: `gateway/cmd` → `cmd`, `gateway/internal` → `internal`, `gateway/go.mod`, `gateway/go.sum` → raíz
- Move: `cmd/server` → `cmd/open-tv`
- Modify: `go.mod` (línea `module`), todos los `.go` con imports del módulo
- Modify: `.gitignore`, `.github/workflows/ci.yml`, `README.md`, `tools/README.md`, `tools/dev.korven.opentv.gateway.plist`, `PROMPT_MAESTRO.md`

**Interfaces:**
- Produces: el prefijo de import `github.com/gdberysan/open-tv/internal/...` que usan TODAS las tareas siguientes, y el entrypoint `./cmd/open-tv`.

- [ ] **Step 1: Parar el LaunchAgent antes de mover nada**

El gateway está corriendo bajo launchd desde un binario y un `WorkingDirectory` que van a dejar de existir. Si no se para, `KeepAlive` lo revive contra rutas rotas y deja logs confusos durante toda la tarea.

```bash
launchctl bootout gui/$(id -u)/dev.korven.opentv.gateway 2>/dev/null || true
lsof -iTCP:8080 -sTCP:LISTEN -n -P || echo "puerto 8080 libre"
```

- [ ] **Step 2: Mover los artefactos no versionados fuera del camino**

`gateway/` contiene la DB de desarrollo y el binario compilado, ambos ignorados. La DB se conserva: reconstruirla cuesta un sync completo.

```bash
mkdir -p .devdata
mv gateway/iptv.db gateway/iptv.db-shm gateway/iptv.db-wal .devdata/ 2>/dev/null || true
rm -f gateway/server
ls gateway
```

Esperado: solo quedan `cmd`, `internal`, `go.mod`, `go.sum`.

- [ ] **Step 3: Mover el módulo con `git mv` y renombrar el entrypoint**

```bash
git mv gateway/cmd gateway/internal gateway/go.mod gateway/go.sum .
git mv cmd/server cmd/open-tv
rmdir gateway
git status --short
```

- [ ] **Step 4: Reescribir el path del módulo y todos los imports**

```bash
sed -i '' 's|^module github.com/gdberysan/open-tv/gateway$|module github.com/gdberysan/open-tv|' go.mod
grep -rl 'github.com/gdberysan/open-tv/gateway/' --include='*.go' . \
  | xargs sed -i '' 's|github.com/gdberysan/open-tv/gateway/|github.com/gdberysan/open-tv/|g'
grep -rn 'open-tv/gateway' --include='*.go' . || echo "sin restos"
head -1 go.mod
```

Esperado: `sin restos` y `module github.com/gdberysan/open-tv`.

- [ ] **Step 5: Correr los gates de Go desde la raíz**

Run:

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=1 ./...
```

Esperado: `gofmt -l .` no imprime nada y los tests pasan. `cmd/open-tv/main_test.go` levanta el stack en `127.0.0.1:18080`; si falla ahí, el movimiento rompió algo real.

- [ ] **Step 6: Actualizar `.gitignore` a las rutas nuevas**

Sustituir el bloque `# Go` de `.gitignore` por:

```
# Go
/open-tv
/server
iptv.db
*.db-journal
*.db-wal
*.db-shm
.devdata/

# Cliente web construido: lo produce `npm run build` en web/ y lo embebe Go.
# El .gitkeep se versiona a propósito: go:embed falla al compilar si el
# directorio no existe.
/internal/ui/dist/*
!/internal/ui/dist/.gitkeep
```

- [ ] **Step 7: Actualizar CI a la raíz**

En `.github/workflows/ci.yml`, en el job `gateway`: borrar el bloque `defaults.run.working-directory: gateway` entero, y cambiar las dos rutas del `setup-go`:

```yaml
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache-dependency-path: go.sum
```

El job `mobile` no se toca.

- [ ] **Step 8: Actualizar la plantilla del LaunchAgent y su README**

En `tools/dev.korven.opentv.gateway.plist`, sustituir `ProgramArguments`, `WorkingDirectory` y `DB_PATH`:

```xml
	<key>ProgramArguments</key>
	<array>
		<string>__RUTA_AL_REPO__/open-tv</string>
		<string>serve</string>
		<string>--no-browser</string>
	</array>

	<key>WorkingDirectory</key>
	<string>__RUTA_AL_REPO__</string>

	<key>EnvironmentVariables</key>
	<dict>
		<key>DB_PATH</key>
		<string>__RUTA_AL_REPO__/.devdata/iptv.db</string>
		<key>LISTEN_ADDR</key>
		<string>127.0.0.1:8080</string>
	</dict>
```

En `tools/README.md`, en la sección del plist, sustituir los dos bloques de comandos que compilan:

```bash
go build -o open-tv ./cmd/open-tv
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/dev.korven.opentv.gateway.plist
```

```bash
go build -o open-tv ./cmd/open-tv
launchctl kickstart -k gui/$(id -u)/dev.korven.opentv.gateway
```

Y añadir al final de esa sección: `--no-browser` es obligatorio bajo launchd: un agente de fondo que abre el navegador al iniciar sesión es hostil.

- [ ] **Step 9: Actualizar el README y PROMPT_MAESTRO**

En `README.md`, sustituir el bloque "### 1. Gateway" por:

````markdown
### 1. Gateway

```bash
go run ./cmd/open-tv
```
````

y en "## Desarrollo" sustituir la primera línea por `go test -race ./... && go vet ./... && gofmt -l .` (sin `cd gateway`). En `PROMPT_MAESTRO.md` §3, sustituir cualquier mención a `gateway/` como raíz del módulo Go por la disposición nueva (`cmd/open-tv`, `internal/`, `web/`).

- [ ] **Step 10: Reinstalar el LaunchAgent y comprobar que revive**

```bash
go build -o open-tv ./cmd/open-tv
sed -e "s|__RUTA_AL_REPO__|$PWD|g" -e "s|__RUTA_HOME__|$HOME|g" \
  tools/dev.korven.opentv.gateway.plist > ~/Library/LaunchAgents/dev.korven.opentv.gateway.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/dev.korven.opentv.gateway.plist
sleep 2 && curl -s localhost:8080/health
```

Esperado: JSON con `"db":"ok"` y un `last_sync` no nulo (la DB de `.devdata/` conserva el catálogo).

- [ ] **Step 11: Commit**

```bash
git add -A
git commit -m "refactor: el módulo Go sube a la raíz como github.com/gdberysan/open-tv

Movimiento mecánico, sin lógica nueva: es lo que hace posible
\`go install github.com/gdberysan/open-tv/cmd/open-tv@latest\` y la
historia de un solo binario. mobile/ no participa.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 2: Directorio de datos, puerto con fallback, y abrir el navegador

Un desconocido ejecuta `open-tv` desde donde le apetezca. Hoy `DB_PATH` es relativo al directorio de trabajo, así que ejecutarlo desde dos sitios distintos son dos catálogos distintos, y desde `/` es una DB vacía que responde 200 con cero canales.

**Files:**
- Create: `internal/datadir/datadir.go`, `internal/datadir/datadir_test.go`
- Create: `internal/netx/escucha.go`, `internal/netx/escucha_test.go`
- Create: `cmd/open-tv/navegador.go`, `cmd/open-tv/instancia.go`, `cmd/open-tv/instancia_test.go`
- Modify: `cmd/open-tv/main.go`

**Interfaces:**
- Produces: `datadir.RutaDB() (string, error)`; `netx.EscuchaConFallback(addr string, intentos int) (net.Listener, error)`; `AbrirNavegador(url string) error`; `InstanciaViva(ctx context.Context, base string) bool`.
- Consumes: nada de tareas anteriores.

- [ ] **Step 1: Escribir el test del directorio de datos**

Crear `internal/datadir/datadir_test.go`:

```go
package datadir_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/datadir"
)

func TestDefaultPorSistema(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	got, err := datadir.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}

	var quiero string
	switch runtime.GOOS {
	case "darwin":
		quiero = filepath.Join(home, "Library", "Application Support", "Korven Open TV")
	case "windows":
		quiero = filepath.Join(home, "AppData", "Roaming", "Korven Open TV")
	default:
		quiero = filepath.Join(home, ".local", "share", "korven-open-tv")
	}
	if got != quiero {
		t.Errorf("Default() = %q, quiero %q", got, quiero)
	}
}

func TestXDGDataHomeGanaEnLinux(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG_DATA_HOME solo aplica al camino por defecto")
	}
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	got, err := datadir.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if got != filepath.Join(xdg, "korven-open-tv") {
		t.Errorf("Default() = %q, quiero %q", got, filepath.Join(xdg, "korven-open-tv"))
	}
}

// DB_PATH es la vía de escape documentada y sigue mandando: la usan el
// LaunchAgent, los tests del stack completo y quien quiera dos catálogos.
func TestRutaDBRespetaDBPath(t *testing.T) {
	explicita := filepath.Join(t.TempDir(), "mia.db")
	t.Setenv("DB_PATH", explicita)

	got, err := datadir.RutaDB()
	if err != nil {
		t.Fatalf("RutaDB: %v", err)
	}
	if got != explicita {
		t.Errorf("RutaDB() = %q, quiero %q", got, explicita)
	}
}

// Sin DB_PATH, RutaDB tiene que DEJAR EL DIRECTORIO CREADO: sqlite no crea
// directorios intermedios y el fallo llega como "unable to open database file",
// que no dice nada del directorio que falta.
func TestRutaDBCreaElDirectorio(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DB_PATH", "")
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "datos"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	got, err := datadir.RutaDB()
	if err != nil {
		t.Fatalf("RutaDB: %v", err)
	}
	if !strings.HasSuffix(got, "iptv.db") {
		t.Errorf("RutaDB() = %q, quiero que termine en iptv.db", got)
	}
	if _, err := filepath.Abs(got); err != nil {
		t.Fatalf("ruta no absoluta: %v", err)
	}
	dir := filepath.Dir(got)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("el directorio %q no quedó creado (err=%v)", dir, err)
	}
}
```

Los imports del fichero son exactamente: `os`, `path/filepath`, `runtime`, `strings`, `testing` y `github.com/gdberysan/open-tv/internal/datadir`.

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `go test ./internal/datadir/...`
Esperado: FAIL — el paquete `datadir` no existe todavía.

- [ ] **Step 3: Implementar `datadir`**

Crear `internal/datadir/datadir.go`:

```go
// Package datadir resuelve dónde vive la base de datos del catálogo.
//
// Existe porque DB_PATH es relativo al directorio de trabajo: ejecutar
// `open-tv` desde dos sitios distintos son dos catálogos distintos, y
// ejecutarlo desde / crea una DB vacía que responde 200 con cero canales.
// Para un binario que un desconocido lanza desde donde sea, eso no vale.
package datadir

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// nombreApp es el del directorio en macOS y Windows, donde la convención es
// el nombre legible del producto. En Linux la convención es en minúsculas
// y con guiones.
const (
	nombreApp   = "Korven Open TV"
	nombreUnix  = "korven-open-tv"
	nombreFichero = "iptv.db"
)

// Default devuelve el directorio de datos del sistema. No crea nada.
func Default() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("datadir.Default: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", nombreApp), nil

	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, nombreApp), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("datadir.Default: %w", err)
		}
		return filepath.Join(home, "AppData", "Roaming", nombreApp), nil

	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, nombreUnix), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("datadir.Default: %w", err)
		}
		return filepath.Join(home, ".local", "share", nombreUnix), nil
	}
}

// RutaDB devuelve la ruta del fichero SQLite y deja su directorio creado.
// DB_PATH manda si está puesta: es la vía de escape documentada.
func RutaDB() (string, error) {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p, nil
	}
	dir, err := Default()
	if err != nil {
		return "", err
	}
	// 0o700 y no 0o755: aquí no hay secretos, pero tampoco hay motivo para que
	// otro usuario de la máquina lea qué canales ves.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("datadir.RutaDB (MkdirAll %s): %w", dir, err)
	}
	return filepath.Join(dir, nombreFichero), nil
}
```

- [ ] **Step 4: Correr el test y verificar que pasa**

Run: `go test ./internal/datadir/... -v`
Esperado: PASS en los cuatro tests (uno con SKIP en macOS).

- [ ] **Step 5: Escribir el test del fallback de puerto**

Crear `internal/netx/escucha_test.go`:

```go
package netx_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/gdberysan/open-tv/internal/netx"
)

func TestEscuchaConFallbackSaltaAlSiguientePuerto(t *testing.T) {
	// Ocupamos un puerto real y pedimos ESE. El fallback debe darnos el +1.
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("preparando el puerto ocupado: %v", err)
	}
	defer ocupado.Close()

	_, puertoStr, _ := net.SplitHostPort(ocupado.Addr().String())
	puerto, _ := strconv.Atoi(puertoStr)

	ln, err := netx.EscuchaConFallback("127.0.0.1:"+puertoStr, 5)
	if err != nil {
		t.Fatalf("EscuchaConFallback: %v", err)
	}
	defer ln.Close()

	_, obtenidoStr, _ := net.SplitHostPort(ln.Addr().String())
	obtenido, _ := strconv.Atoi(obtenidoStr)
	if obtenido == puerto {
		t.Fatal("devolvió el puerto ocupado")
	}
	if obtenido < puerto+1 || obtenido > puerto+5 {
		t.Errorf("puerto %d fuera del rango de fallback [%d,%d]", obtenido, puerto+1, puerto+5)
	}
}

func TestEscuchaConFallbackSeRindeYDevuelveError(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("preparando el puerto ocupado: %v", err)
	}
	defer ocupado.Close()

	// Un solo intento sobre un puerto ocupado: no hay a dónde saltar.
	if ln, err := netx.EscuchaConFallback(ocupado.Addr().String(), 1); err == nil {
		ln.Close()
		t.Fatal("quiero error cuando no queda puerto libre")
	}
}

// El puerto 0 significa "el que sea" y ya lo resuelve el sistema: el fallback
// no debe intentar 1, 2, 3...
func TestEscuchaConFallbackRespetaElPuertoCero(t *testing.T) {
	ln, err := netx.EscuchaConFallback("127.0.0.1:0", 3)
	if err != nil {
		t.Fatalf("EscuchaConFallback: %v", err)
	}
	defer ln.Close()
	if _, p, _ := net.SplitHostPort(ln.Addr().String()); p == "0" {
		t.Error("el sistema debía asignar un puerto real")
	}
}
```

- [ ] **Step 6: Correr el test y verificar que falla**

Run: `go test ./internal/netx/...`
Esperado: FAIL — el paquete `netx` no existe.

- [ ] **Step 7: Implementar `netx`**

Crear `internal/netx/escucha.go`:

```go
// Package netx contiene el poco código de red que no encaja en un adaptador.
package netx

import (
	"fmt"
	"net"
	"strconv"
)

// EscuchaConFallback abre un listener en addr y, si el puerto está ocupado,
// prueba los intentos-1 siguientes.
//
// Un binario de escritorio no puede rendirse porque el 8080 esté cogido: la
// mitad de las herramientas de desarrollo lo usan. Con puerto 0 no hay
// fallback que valga — el sistema ya elige uno libre.
func EscuchaConFallback(addr string, intentos int) (net.Listener, error) {
	host, puertoStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("netx.EscuchaConFallback (%q): %w", addr, err)
	}
	puerto, err := strconv.Atoi(puertoStr)
	if err != nil {
		return nil, fmt.Errorf("netx.EscuchaConFallback (puerto %q): %w", puertoStr, err)
	}
	if intentos < 1 {
		intentos = 1
	}
	if puerto == 0 {
		intentos = 1
	}

	var ultimo error
	for i := 0; i < intentos; i++ {
		candidato := net.JoinHostPort(host, strconv.Itoa(puerto+i))
		ln, err := net.Listen("tcp", candidato)
		if err == nil {
			return ln, nil
		}
		ultimo = err
	}
	return nil, fmt.Errorf("netx.EscuchaConFallback: ningún puerto libre entre %d y %d: %w",
		puerto, puerto+intentos-1, ultimo)
}
```

- [ ] **Step 8: Correr el test y verificar que pasa**

Run: `go test ./internal/netx/... -v`
Esperado: PASS en los tres.

- [ ] **Step 9: Escribir el test de "ya hay una instancia viva"**

Crear `cmd/open-tv/instancia_test.go`:

```go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstanciaVivaReconoceOtroOpenTV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","db":"ok","web_ui":true}`))
	}))
	defer srv.Close()

	if !InstanciaViva(context.Background(), srv.URL) {
		t.Error("un /health de Open TV debe reconocerse como instancia viva")
	}
}

// Cualquier otra cosa escuchando en el puerto NO es Open TV. Abrir el
// navegador contra el panel de otro programa sería peor que fallar.
func TestInstanciaVivaRechazaOtroServicio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Grafana"))
	}))
	defer srv.Close()

	if InstanciaViva(context.Background(), srv.URL) {
		t.Error("un servicio ajeno no puede pasar por Open TV")
	}
}

func TestInstanciaVivaConPuertoMuerto(t *testing.T) {
	if InstanciaViva(context.Background(), "http://127.0.0.1:1") {
		t.Error("un puerto que no contesta no es una instancia viva")
	}
}
```

- [ ] **Step 10: Correr el test y verificar que falla**

Run: `go test ./cmd/open-tv/ -run TestInstanciaViva`
Esperado: FAIL — `InstanciaViva` no está definida.

- [ ] **Step 11: Implementar `InstanciaViva` y `AbrirNavegador`**

Crear `cmd/open-tv/instancia.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// InstanciaViva dice si en base hay OTRO Open TV escuchando.
//
// Se comprueba antes de rendirse por "puerto ocupado": lo más probable cuando
// el 8080 está cogido es que el usuario ya tenga Open TV abierto, y entonces
// lo correcto es llevarle a esa pestaña, no arrancar un segundo catálogo.
// La marca es web_ui: solo la pone este binario.
func InstanciaViva(ctx context.Context, base string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	var cuerpo struct {
		WebUI *bool `json:"web_ui"`
	}
	datos, err := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if err != nil {
		return false
	}
	if err := json.Unmarshal(datos, &cuerpo); err != nil {
		return false
	}
	return cuerpo.WebUI != nil
}
```

Crear `cmd/open-tv/navegador.go`:

```go
package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// AbrirNavegador abre url en el navegador por defecto del sistema.
//
// No se usa una dependencia para esto: son tres comandos y ninguna librería
// va a saber más que el propio sistema. El error se registra pero nunca es
// fatal — el servidor ya está escuchando y la URL se imprime igualmente.
func AbrirNavegador(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("abriendo el navegador: %w", err)
	}
	// Release y no Wait: `open` en macOS termina enseguida, pero xdg-open puede
	// quedarse vivo mientras dure el navegador. Esperarlo colgaría el arranque.
	return cmd.Process.Release()
}
```

- [ ] **Step 12: Correr los tests y verificar que pasan**

Run: `go test ./cmd/open-tv/ -run TestInstanciaViva -v`
Esperado: PASS en los tres.

- [ ] **Step 13: Cablearlo todo en `main.go`**

En `cmd/open-tv/main.go`, añadir a los imports `"flag"`, `"net"`, y los paquetes nuevos `"github.com/gdberysan/open-tv/internal/datadir"` y `"github.com/gdberysan/open-tv/internal/netx"`.

Sustituir el cuerpo de `main()` por:

```go
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	log.SetFlags(0)
	log.SetOutput(slogWriter{logger})

	// Subcomandos: `serve` es el default. Se acepta explícito para que el
	// LaunchAgent y los scripts de arranque no dependan del default.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	sinNavegador := fs.Bool("no-browser", false, "no abrir el navegador al arrancar")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, *sinNavegador); err != nil {
		logger.Error("Fallo fatal", slog.Any("error", err))
		os.Exit(1)
	}
}
```

En `run`, cambiar la firma a `func run(ctx context.Context, logger *slog.Logger, sinNavegador bool) error` y sustituir el bloque `// 1. Base de datos SQLite` por:

```go
	// 1. Base de datos SQLite. La ruta viene del directorio de datos del
	// sistema salvo que DB_PATH diga otra cosa.
	dbPath, err := datadir.RutaDB()
	if err != nil {
		return fmt.Errorf("resolviendo el directorio de datos: %w", err)
	}
	sqlDB, err := db.Open(dbPath)
```

Sustituir el bloque `// 4. Router y servidor HTTP` (desde `listenAddr := ...` hasta el `go func()` del `ListenAndServe`) por:

```go
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8080"
	}

	ln, err := netx.EscuchaConFallback(listenAddr, 8)
	if err != nil {
		// Puerto ocupado: lo más probable es que ya haya un Open TV abierto.
		base := "http://" + listenAddr
		if InstanciaViva(ctx, base) {
			logger.Info("Ya hay un Open TV escuchando; abriendo esa ventana",
				slog.String("url", base))
			if !sinNavegador {
				if err := AbrirNavegador(base); err != nil {
					logger.Warn("No se pudo abrir el navegador", slog.Any("error", err))
				}
			}
			return nil
		}
		return fmt.Errorf("escuchando en %s: %w", listenAddr, err)
	}

	url := "http://" + ln.Addr().String()
	srv := &http.Server{
		Handler:           api.NewRouter(logger, channelRepoRO, provider, streamRepoRO, lecturaDB, syncer, esLoopback(ln)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() {
		logger.Info("Servidor escuchando", slog.String("url", url))
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	if !sinNavegador {
		if err := AbrirNavegador(url); err != nil {
			logger.Warn("No se pudo abrir el navegador; abre la URL a mano",
				slog.String("url", url), slog.Any("error", err))
		}
	}
```

Nótese que `srv.Addr` desaparece: con `Serve(ln)` la dirección la manda el listener, que es el único que sabe qué puerto acabó tocando.

Añadir al final de `main.go`:

```go
// esLoopback decide si el proxy HLS puede montarse. Se pregunta al listener
// real y no a la cadena de configuración: LISTEN_ADDR puede decir "localhost",
// un nombre puede resolver a otra cosa, y lo que importa es la IP que quedó.
func esLoopback(ln net.Listener) bool {
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return false
	}
	return addr.IP.IsLoopback()
}
```

`api.NewRouter` todavía no acepta el séptimo parámetro: eso lo añade la Tarea 3. Hasta entonces el build falla, así que **este step y el siguiente van en el mismo commit que la Tarea 3**. Para no dejar el árbol roto entre tareas, aplicar el cambio de firma de `NewRouter` ahora, en la Tarea 3, y commitear las dos juntas.

- [ ] **Step 14: Ajustar `main_test.go` a la firma nueva**

En `cmd/open-tv/main_test.go`, la llamada `run(ctx, slog.New(slog.DiscardHandler))` pasa a `run(ctx, slog.New(slog.DiscardHandler), true)`. El `true` es obligatorio: un test que abre el navegador del autor cada vez que corre es inaceptable.

- [ ] **Step 15: Añadir el test de que un puerto ocupado no mata el arranque**

Añadir a `cmd/open-tv/main_test.go`:

```go
// El fallback de puerto es la diferencia entre "no arranca" y "arranca en el
// 8081". Se comprueba con el 8080 realmente ocupado.
func TestRunSaltaDePuertoSiEstaOcupado(t *testing.T) {
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("preparando el puerto ocupado: %v", err)
	}
	defer ocupado.Close()

	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("LISTEN_ADDR", ocupado.Addr().String())
	t.Setenv("IPTV_ORG_URL", "http://127.0.0.1:1/index.m3u")

	_, puertoStr, _ := net.SplitHostPort(ocupado.Addr().String())
	puerto, _ := strconv.Atoi(puertoStr)
	siguiente := "http://127.0.0.1:" + strconv.Itoa(puerto+1)

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- run(ctx, slog.New(slog.DiscardHandler), true) }()

	var resp *http.Response
	for i := 0; i < 60; i++ {
		resp, err = http.Get(siguiente + "/health")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("no escuchó en el puerto siguiente: %v", err)
	}
	resp.Body.Close()

	cancel()
	if err := <-errc; err != nil {
		t.Errorf("run devolvió error: %v", err)
	}
}
```

Añadir `"net"` y `"strconv"` a los imports del test.

- [ ] **Step 16: Correr toda la suite**

Run: `gofmt -l . && go vet ./... && go test -race -count=1 ./...`
Esperado: todo verde (después de aplicar la Tarea 3, que cierra la firma de `NewRouter`).

---

## Tarea 3: Fuera el CORS `*`

Hoy cualquier página que visites puede leer `http://127.0.0.1:8080/channels` desde tu navegador. Existía para una app Flutter web que nunca se construyó; la app de macOS es nativa y jamás lo necesitó, y el cliente nuevo es del mismo origen.

**Files:**
- Delete: `internal/api/middleware/cors.go`
- Modify: `internal/api/middleware/middleware_test.go:121-140`
- Modify: `internal/api/router.go`
- Modify: `internal/api/router_test.go`

**Interfaces:**
- Produces: `api.NewRouter(logger, repo, provider, streams, sqlDB, syncer, proxyActivo bool) http.Handler` — la firma que consumen la Tarea 2 (`main.go`), la Tarea 8 (proxy) y la Tarea 9 (UI embebida).

- [ ] **Step 1: Escribir el test que exige que NO haya cabecera CORS**

En `internal/api/router_test.go`, añadir:

```go
// El gateway local dejó de ser legible por webs de terceros. Sin este test,
// alguien reintroduce el middleware "porque el navegador se quejaba" y la
// regresión no se ve: todo sigue funcionando, solo que para todos.
func TestRouterNoAnunciaCORS(t *testing.T) {
	r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncerFalso{}, false)

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, quiero vacío", v)
	}
}
```

Si `syncerFalso` no existe con ese nombre en el fichero, usar el tipo que ya esté implementando `handlers.SyncStatus` en `router_test.go`.

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `go test ./internal/api/ -run TestRouterNoAnunciaCORS`
Esperado: FAIL con `Access-Control-Allow-Origin = "*", quiero vacío` (y un error de compilación por el séptimo parámetro, que se resuelve en el step siguiente).

- [ ] **Step 3: Borrar el middleware y su uso**

```bash
git rm internal/api/middleware/cors.go
```

En `internal/api/middleware/middleware_test.go`, borrar entera la función `TestCORSCortaElPreflight` (líneas 121–140 aproximadamente; el bloque completo desde `func TestCORSCortaElPreflight` hasta su `}` de cierre).

En `internal/api/router.go`, borrar la línea `r.Use(middleware.CORS)` y cambiar la firma:

```go
func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, sqlDB *sql.DB, syncer handlers.SyncStatus, proxyActivo bool) http.Handler {
```

Por ahora `proxyActivo` no se usa (lo consume la Tarea 8). Para que `go vet` no proteste por un parámetro sin usar —no lo hace, pero para que quede constancia— añadir justo debajo de la firma:

```go
	// proxyActivo lo decide el listener real (loopback o no) y lo consumen el
	// proxy HLS (montaje) y /health (proxy_enabled).
	_ = proxyActivo
```

Esa línea se borra en la Tarea 8.

- [ ] **Step 4: Actualizar el resto de llamadas a `NewRouter`**

```bash
grep -rn 'NewRouter(' --include='*.go' .
```

Actualizar cada llamada añadiendo el argumento final. En `router_test.go` los tests existentes pasan `false`; en `cmd/open-tv/main.go` pasa `esLoopback(ln)` (ya escrito en la Tarea 2).

- [ ] **Step 5: Correr los tests y verificar que pasan**

Run: `go test -race -count=1 ./...`
Esperado: PASS, incluido `TestRouterNoAnunciaCORS` y los tests de `cmd/open-tv` de la Tarea 2.

- [ ] **Step 6: Verificar el efecto de verdad, no solo el test**

Con el gateway corriendo (`go run ./cmd/open-tv --no-browser` o el LaunchAgent recompilado):

```bash
curl -s -D- -o /dev/null -H 'Origin: https://evil.example' http://127.0.0.1:8080/channels?limit=1 | grep -i 'access-control' || echo "sin cabeceras CORS — correcto"
```

Esperado: `sin cabeceras CORS — correcto`.

- [ ] **Step 7: Commit (cierra también la Tarea 2)**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./... && (cd mobile && flutter analyze && flutter test)
git add -A
git commit -m "feat(binario): directorio de datos, fallback de puerto, navegador; fuera el CORS *

El binario deja de depender del directorio de trabajo (DB_PATH sigue
mandando), sobrevive a un 8080 ocupado, y si ya hay otro Open TV
escuchando abre esa ventana en vez de fallar.

El CORS * se retira: existía para una Flutter web que nunca se
construyó, y mientras tanto cualquier página podía leer tu catálogo
local desde tu propio navegador.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 4: `web_ok` — el clasificador puro

Tres estados, como AirPlay, y por el mismo motivo: colapsar "no se sabe" en "no" marcaría como rotos miles de canales que funcionan. Sin E/S: el cuerpo llega ya leído.

**Files:**
- Create: `internal/domain/web.go`, `internal/domain/web_test.go`
- Modify: ninguno (reutiliza `urlNoReproducible` y `atributoEntreComillas` de `internal/domain/airplay.go`, mismo paquete)

**Interfaces:**
- Produces: `domain.WebSupport` (`WebUnknown`/`WebNo`/`WebOK`, cero valor = Unknown), `domain.ClassifyWeb(finalURL, acao, body string) WebSupport`, `domain.OrigenWeb`.
- Consumes: `domain.ClassifyManifest` como precedente de forma; no lo llama.

- [ ] **Step 1: Escribir el test**

Crear `internal/domain/web_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/gdberysan/open-tv/internal/domain"
)

const masterOK = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS="avc1.4d401f,mp4a.40.2"
chunk.m3u8
`

const masterHEVC = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=6000000,CODECS="hvc1.1.6.L93.B0,mp4a.40.2"
chunk.m3u8
`

const masterSinCodecs = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1200000
chunk.m3u8
`

func TestClassifyWeb(t *testing.T) {
	casos := []struct {
		nombre string
		url    string
		acao   string
		cuerpo string
		quiero domain.WebSupport
	}{
		{"https con ACAO comodín y códecs de navegador",
			"https://cdn.example/live.m3u8", "*", masterOK, domain.WebOK},

		{"https con ACAO exactamente nuestro origen",
			"https://cdn.example/live.m3u8", domain.OrigenWeb, masterOK, domain.WebOK},

		// El caso Pluto TV: 1 359 streams del catálogo mandan ACAO pero solo
		// para su propio dominio. Desde nuestra página el navegador los corta.
		{"ACAO de otro origen no sirve",
			"https://cdn.example/live.m3u8", "https://pluto.tv", masterOK, domain.WebNo},

		{"sin ACAO no hay fetch posible",
			"https://cdn.example/live.m3u8", "", masterOK, domain.WebNo},

		// El sitio hospedado es HTTPS: contenido mixto bloqueado. En el binario
		// local el proxy de loopback cubre justo este caso.
		{"http es contenido mixto",
			"http://cdn.example/live.m3u8", "*", masterOK, domain.WebNo},

		{"códecs que ningún navegador decodifica",
			"https://cdn.example/live.m3u8", "*", masterHEVC, domain.WebNo},

		// Solo el 57,5 % de los manifiestos declara CODECS. Sin declaración no
		// hay base para decir que no.
		{"sin CODECS declarados se acepta",
			"https://cdn.example/live.m3u8", "*", masterSinCodecs, domain.WebOK},

		{"cuerpo vacío: manda esquema y CORS",
			"https://cdn.example/canal.ts", "*", "", domain.WebOK},

		{"DASH no lo toca ningún navegador sin MSE propio",
			"https://cdn.example/manifest.mpd", "*", "", domain.WebNo},

		{"rtmp no existe en el navegador",
			"rtmp://cdn.example/live", "*", "", domain.WebNo},

		// hls.js abre AES-128 pero no FairPlay: SAMPLE-AES es no.
		{"SAMPLE-AES no se abre en el navegador",
			"https://cdn.example/live.m3u8", "*",
			"#EXTM3U\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://x\"\n" + masterOK,
			domain.WebNo},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := domain.ClassifyWeb(c.url, c.acao, c.cuerpo); got != c.quiero {
				t.Errorf("ClassifyWeb(%q, %q, …) = %v, quiero %v", c.url, c.acao, got, c.quiero)
			}
		})
	}
}

// El cero valor tiene que ser Unknown: StreamResult.Web nace así cuando el
// checker no llegó a hablar con el origen, y eso es exactamente "no se sabe".
func TestWebSupportCeroEsUnknown(t *testing.T) {
	var v domain.WebSupport
	if v != domain.WebUnknown {
		t.Errorf("el cero valor es %v, quiero WebUnknown", v)
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `go test ./internal/domain/ -run 'TestClassifyWeb|TestWebSupportCero'`
Esperado: FAIL — `undefined: domain.ClassifyWeb`.

- [ ] **Step 3: Implementar el clasificador**

Crear `internal/domain/web.go`:

```go
package domain

import "strings"

// WebSupport clasifica si un stream se reproduce DIRECTAMENTE en un navegador,
// desde una página de otro origen servida por HTTPS.
//
// Hermano de AirplaySupport en semántica —tres estados, lista de permitidos,
// "no se sabe" es un veredicto legítimo— pero con reglas distintas: al
// navegador le importa el esquema y el CORS, cosas que a AVFoundation le dan
// igual porque no es una página web.
//
// Es el veredicto ESTRICTO, el de hls.js (Chrome, Firefox). Safari reproduce
// HLS de forma nativa sin necesitar CORS, así que reproduce más de lo que este
// veredicto admite; esa diferencia la resuelve el cliente al elegir motor, no
// este clasificador. Censo 2026-08-22: Safari 85 %, Chrome/Firefox 67 %.
type WebSupport int

const (
	WebUnknown WebSupport = iota
	WebNo
	WebOK
)

// OrigenWeb es el origen del sitio hospedado. El health-checker lo manda como
// cabecera Origin para que los servidores que reflejan el origen del
// solicitante contesten con un ACAO que podamos reconocer.
const OrigenWeb = "https://opentv.korven.dev"

// codecsWeb son los prefijos RFC 6381 que decodifica cualquier navegador
// moderno en HLS. Lista de permitidos: un códec desconocido no se asume
// reproducible. HEVC queda fuera a propósito — es el 0,9 % del catálogo y solo
// Safari lo abre.
var codecsWeb = []string{"avc1.", "avc3.", "mp4a.40."}

// ClassifyWeb decide sobre la URL FINAL (tras redirecciones), la cabecera
// Access-Control-Allow-Origin de esa respuesta y el cuerpo del manifiesto.
// El cuerpo puede venir vacío: entonces solo mandan esquema y CORS.
func ClassifyWeb(finalURL, acao, body string) WebSupport {
	if urlNoReproducible(finalURL) {
		return WebNo
	}
	// Contenido mixto: una página HTTPS no puede cargar medios por HTTP.
	if !strings.HasPrefix(strings.ToLower(finalURL), "https://") {
		return WebNo
	}
	if !corsPermisivo(acao) {
		return WebNo
	}
	if cifradoNoAbrible(body) {
		return WebNo
	}
	return veredictoCodecsWeb(body)
}

// corsPermisivo acepta el comodín o exactamente nuestro origen. Cualquier otro
// origen concreto —el caso Pluto TV, 1 359 streams— no nos sirve.
func corsPermisivo(acao string) bool {
	acao = strings.TrimSpace(acao)
	return acao == "*" || strings.EqualFold(acao, OrigenWeb)
}

// cifradoNoAbrible: hls.js descifra AES-128 por su cuenta, pero SAMPLE-AES es
// FairPlay y necesita EME con un servidor de licencias que no tenemos.
func cifradoNoAbrible(body string) bool {
	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if strings.HasPrefix(linea, "#EXT-X-KEY:") && strings.Contains(linea, "METHOD=SAMPLE-AES") {
			return true
		}
	}
	return false
}

// veredictoCodecsWeb aplica la misma regla que AirPlay: basta una variante
// reproducible; si TODAS las declaradas son inservibles y ninguna se quedó sin
// declarar, es que no. Sin declaraciones, se acepta.
func veredictoCodecsWeb(body string) WebSupport {
	hayDeclarada := false
	haySinDeclarar := false

	for _, linea := range strings.Split(body, "\n") {
		linea = strings.TrimSpace(linea)
		if !strings.HasPrefix(linea, "#EXT-X-STREAM-INF:") {
			continue
		}
		codecs, ok := atributoEntreComillas(linea, "CODECS")
		if !ok {
			haySinDeclarar = true
			continue
		}
		hayDeclarada = true
		if varianteWebSoportada(codecs) {
			return WebOK
		}
	}

	if hayDeclarada && !haySinDeclarar {
		return WebNo
	}
	return WebOK
}

// varianteWebSoportada exige que TODOS los códecs de la variante sirvan: un
// vídeo H.264 con audio AC-3 deja la variante inservible en el navegador.
func varianteWebSoportada(codecs string) bool {
	partes := strings.Split(codecs, ",")
	if len(partes) == 0 {
		return false
	}
	for _, c := range partes {
		if !codecWebSoportado(strings.TrimSpace(c)) {
			return false
		}
	}
	return true
}

func codecWebSoportado(c string) bool {
	for _, p := range codecsWeb {
		if strings.HasPrefix(c, p) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Correr el test y verificar que pasa**

Run: `go test ./internal/domain/ -run 'TestClassifyWeb|TestWebSupportCero' -v`
Esperado: PASS en los once subcasos y en el del cero valor.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./...
git add internal/domain/web.go internal/domain/web_test.go
git commit -m "feat(web-ok): clasificador de reproducibilidad en navegador

Tres estados como AirPlay, reglas distintas: al navegador le importan
el esquema (contenido mixto) y el CORS, que a AVFoundation le dan
igual. Es el veredicto estricto de hls.js; Safari reproduce más y esa
diferencia la resuelve el cliente, no el clasificador.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 5: El checker emite el veredicto web sobre el manifiesto que ya descarga

Hoy el cuerpo solo se lee en el camino de fallback a GET, así que "clasificar sobre el manifiesto que ya se descarga" cubriría únicamente a los orígenes que rechazan HEAD. Para HLS —el 99 % del catálogo— el HEAD no aporta nada que necesitemos: se va directo al GET. Es **una** petición en vez de dos.

**Files:**
- Modify: `internal/adapters/validator/models.go`
- Modify: `internal/adapters/validator/checker.go`
- Test: `internal/adapters/validator/checker_test.go`

**Interfaces:**
- Produces: `validator.StreamResult.Web domain.WebSupport`, rellenado por `Checker.Check`.
- Consumes: `domain.ClassifyWeb`, `domain.OrigenWeb` (Tarea 4).

- [ ] **Step 1: Escribir los tests**

Añadir a `internal/adapters/validator/checker_test.go`:

```go
// El veredicto web sale del MISMO GET que decide si el stream está vivo. Si
// alguien reintroduce el HEAD para HLS, este test cae: el cuerpo no llega y
// no hay veredicto.
func TestCheckClasificaWebEnHLSSinPeticionExtra(t *testing.T) {
	var peticiones int32
	var vioOrigin atomic.Bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&peticiones, 1)
		if r.Header.Get("Origin") != "" {
			vioOrigin.Store(true)
		}
		if r.Method == http.MethodHead {
			t.Error("no debe haber HEAD sobre una URL HLS")
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f,mp4a.40.2\"\nchunk.m3u8\n"))
	}))
	defer srv.Close()

	c := validator.NewChecker(srv.Client(), 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/live.m3u8")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, error = %v", res.Error)
	}
	if res.Web != domain.WebOK {
		t.Errorf("Web = %v, quiero WebOK", res.Web)
	}
	if n := atomic.LoadInt32(&peticiones); n != 1 {
		t.Errorf("%d peticiones, quiero exactamente 1", n)
	}
	if !vioOrigin.Load() {
		t.Error("el checker debe mandar Origin para que los orígenes que lo reflejan contesten un ACAO reconocible")
	}
}

// Sin ACAO el stream sigue VIVO —el health-check no cambia— pero no es
// reproducible desde una página. Los dos veredictos son independientes.
func TestCheckVivoPeroNoWebSinCORS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f\"\nchunk.m3u8\n"))
	}))
	defer srv.Close()

	c := validator.NewChecker(srv.Client(), 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/live.m3u8")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, error = %v", res.Error)
	}
	if res.Web != domain.WebNo {
		t.Errorf("Web = %v, quiero WebNo", res.Web)
	}
}

// Un origen inalcanzable deja el veredicto en Unknown, nunca en No: "no pude
// preguntar" y "pregunté y no" son cosas distintas y la DB las distingue.
func TestCheckOrigenCaidoDejaWebUnknown(t *testing.T) {
	c := validator.NewChecker(nil, 500*time.Millisecond)
	res := c.Check(context.Background(), "http://127.0.0.1:1/live.m3u8")

	if res.IsAlive {
		t.Error("IsAlive = true sobre un puerto muerto")
	}
	if res.Web != domain.WebUnknown {
		t.Errorf("Web = %v, quiero WebUnknown", res.Web)
	}
}

// Lo no-HLS conserva HEAD→GET; el veredicto sale de las cabeceras del HEAD.
func TestCheckNoHLSClasificaDesdeElHEAD(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := validator.NewChecker(srv.Client(), 5*time.Second)
	res := c.Check(context.Background(), srv.URL+"/canal.ts")

	if !res.IsAlive {
		t.Fatalf("IsAlive = false, error = %v", res.Error)
	}
	if res.Web != domain.WebOK {
		t.Errorf("Web = %v, quiero WebOK", res.Web)
	}
}
```

Añadir a los imports del fichero de test: `"sync/atomic"` y `"github.com/gdberysan/open-tv/internal/domain"` si no están.

- [ ] **Step 2: Correr los tests y verificar que fallan**

Run: `go test ./internal/adapters/validator/ -run TestCheck`
Esperado: FAIL — `res.Web undefined` y, una vez añadido el campo, `no debe haber HEAD sobre una URL HLS`.

- [ ] **Step 3: Añadir el campo al resultado**

En `internal/adapters/validator/models.go`, dentro de `StreamResult`, debajo de `Airplay`:

```go
	// Reproducibilidad directa en un navegador. Sale del mismo GET que decide
	// IsAlive: esquema final, ACAO de la respuesta y CODECS del manifiesto.
	// Cero valor = WebUnknown = no se pudo preguntar.
	Web domain.WebSupport
```

- [ ] **Step 4: Reescribir `Check`**

En `internal/adapters/validator/checker.go`, sustituir el cuerpo de `Check` por:

```go
// Check valida una URL. Para HLS va directo al GET: el HEAD no trae el
// manifiesto, y sin manifiesto no hay veredicto de compatibilidad. Es una
// petición en lugar de dos, no una más. El resto conserva HEAD→GET.
func (c *Checker) Check(ctx context.Context, url string) StreamResult {
	start := time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result := StreamResult{
		URL:      url,
		Protocol: inferProtocol(url),
	}

	needsGetFallback := result.Protocol == "HLS"

	if !needsGetFallback {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, url, nil)
		if err != nil {
			result.Error = fmt.Errorf("creando HEAD request: %w", err)
			return result
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Origin", domain.OrigenWeb)

		resp, err := c.client.Do(req)
		if err != nil {
			result.Error = fmt.Errorf("error en HEAD request: %w", err)
			needsGetFallback = true
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode >= 400 {
				needsGetFallback = true
			} else {
				result.IsAlive = resp.StatusCode >= 200 && resp.StatusCode < 300
				// Sin cuerpo, pero con esquema final y CORS ya se decide todo lo
				// que un .ts o un .mp4 necesitan.
				result.Web = domain.ClassifyWeb(urlFinal(resp, url),
					resp.Header.Get("Access-Control-Allow-Origin"), "")
			}
		}
	}

	if needsGetFallback {
		result.Error = nil
		reqGet, errGet := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
		if errGet != nil {
			result.Error = fmt.Errorf("creando GET request: %w", errGet)
			return result
		}
		reqGet.Header.Set("User-Agent", userAgent)
		// Origin va a propósito: los orígenes que reflejan el origen del
		// solicitante solo contestan un ACAO si se les manda uno.
		reqGet.Header.Set("Origin", domain.OrigenWeb)

		respGet, errGet := c.client.Do(reqGet)
		if errGet != nil {
			result.Error = fmt.Errorf("error en GET request: %w", errGet)
			return result
		}
		defer respGet.Body.Close()

		// Drenar el cuerpo es obligatorio: sin leerlo, la conexión queda
		// inutilizable y el servidor la ve abortada a media respuesta. Ya que
		// hay que leerlo, se clasifica dos veces en vez de tirarlo — cero
		// peticiones extra por dos veredictos de compatibilidad.
		cuerpo, errLectura := io.ReadAll(io.LimitReader(respGet.Body, 64<<10))
		if errLectura != nil {
			result.Error = fmt.Errorf("leyendo cuerpo del GET: %w", errLectura)
			return result
		}
		final := urlFinal(respGet, url)
		result.Airplay = domain.ClassifyManifest(final, string(cuerpo))
		result.Web = domain.ClassifyWeb(final,
			respGet.Header.Get("Access-Control-Allow-Origin"), string(cuerpo))

		result.IsAlive = respGet.StatusCode >= 200 && respGet.StatusCode < 300
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	return result
}

// urlFinal devuelve la URL tras las redirecciones. Importa: un http:// que
// redirige a https:// SÍ es reproducible desde una página segura, y juzgarlo
// por la URL de partida lo descartaría sin motivo.
func urlFinal(resp *http.Response, porDefecto string) string {
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return porDefecto
}
```

Añadir `"github.com/gdberysan/open-tv/internal/domain"` a los imports si el linter lo pide (ya está: `ClassifyManifest` se usaba).

- [ ] **Step 5: Correr los tests y verificar que pasan**

Run: `go test -race -count=1 ./internal/adapters/validator/ -v`
Esperado: PASS, incluidos los cuatro nuevos y los de latencia y `MaxConnsPerHost` que ya estaban.

- [ ] **Step 6: Medir el coste real del cambio antes de darlo por bueno**

El cambio HEAD→GET para HLS descarga hasta 64 KB por stream donde antes podían ser cero. Sobre el catálogo real es lo que hay que comprobar, no estimar.

```bash
go build -o open-tv ./cmd/open-tv
DB_PATH=$PWD/.devdata/iptv.db HEALTH_INTERVAL=5m ./open-tv --no-browser 2>&1 | grep -m1 'Health-check completado'
```

Anotar aquí el resultado (streams, urls_unicas, vivos, muertos) y el tiempo que tardó la pasada. Si la pasada pasa de ~10 minutos, el `MaxConnsPerHost=4` sigue siendo el cuello y no el cambio de método; no tocar nada sin medir cuál de los dos es.

**Medición (rellenar al ejecutar):** Ejecutado 2026-08-25 sobre el catálogo real de `.devdata/iptv.db` (LaunchAgent parado durante la medición, restaurado después). Sync: 12 837 canales / 12 837 streams. Primera pasada de salud: `streams=12225, urls_unicas=12225, vivos=8866, muertos=3359`, completada en **5 min 21 s** (worker iniciado 02:34:51.766, "Health-check completado" 02:40:12.849) — muy por debajo del umbral de ~10 min del Step 6, así que el GET directo para HLS no es el cuello; `MaxConnsPerHost=4` tampoco lo evidencia aquí. Tres pasadas posteriores (intervalo 5 min) dieron cifras estables: 8856/3369, 8862/3363, 8858/3367 vivos/muertos — la variación (~0,1 %) es ruido normal de orígenes intermitentes, no una regresión del cambio de método.

- [ ] **Step 7: Commit**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./...
git add internal/adapters/validator/
git commit -m "feat(web-ok): el checker emite el veredicto web sobre el manifiesto que ya lee

Para HLS se va directo al GET: el HEAD no trae manifiesto y sin
manifiesto no hay veredicto. Es una peticion en vez de dos, no una
mas. Origin se manda a proposito, para que los origenes que reflejan
el origen del solicitante contesten un ACAO reconocible.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 6: Persistir `web_ok` y exponerlo por canal

La rejilla pinta 500 canales de golpe: el veredicto tiene que venir en la misma consulta, como ya viene `Alive`. Aquí es donde el contrato JSON pasa de 14 a 15 claves, **una sola vez en todo P0**.

**Files:**
- Modify: `internal/adapters/db/schema.sql`, `internal/adapters/db/db.go:alterMigrations`
- Modify: `internal/ports/stream_repository.go`
- Modify: `internal/adapters/db/stream_repository.go` (`MarkBatch`)
- Modify: `internal/adapters/db/channel_repository.go` (`FindFiltered`, `scanChannelsWithHealth`)
- Modify: `internal/adapters/validator/worker.go`
- Modify: `internal/domain/channel.go`, `internal/domain/channel_test.go`
- Test: `internal/adapters/db/stream_repository_test.go`, `internal/adapters/db/channel_repository_test.go`

**Interfaces:**
- Produces: `domain.Channel.WebOK *bool` (clave JSON `WebOK`, `null` = sin comprobar); `ports.StreamHealth.Web domain.WebSupport`; columna `streams.web_ok INTEGER`.
- Consumes: `validator.StreamResult.Web` (Tarea 5).

- [ ] **Step 1: Escribir el test de persistencia**

Añadir a `internal/adapters/db/stream_repository_test.go`:

```go
// El veredicto web se guarda junto al de salud, en la misma transacción.
func TestMarkBatchPersisteWebOK(t *testing.T) {
	repo, sqlDB := repoConCanal(t) // helper existente del fichero

	if err := repo.MarkBatch(context.Background(), []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, LatencyMs: 120, Web: domain.WebOK},
		{StreamID: "s2", IsAlive: true, LatencyMs: 300, Web: domain.WebNo},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}

	for _, c := range []struct {
		id     string
		quiero sql.NullInt64
	}{
		{"s1", sql.NullInt64{Int64: 1, Valid: true}},
		{"s2", sql.NullInt64{Int64: 0, Valid: true}},
	} {
		var got sql.NullInt64
		if err := sqlDB.QueryRow(`SELECT web_ok FROM streams WHERE id = ?`, c.id).Scan(&got); err != nil {
			t.Fatalf("leyendo web_ok de %s: %v", c.id, err)
		}
		if got != c.quiero {
			t.Errorf("web_ok de %s = %+v, quiero %+v", c.id, got, c.quiero)
		}
	}
}

// Un veredicto desconocido NO puede pisar uno bueno: si el origen no contestó
// esta pasada, lo que sabíamos de la anterior sigue siendo lo mejor que hay.
func TestMarkBatchUnknownNoPisaElVeredictoAnterior(t *testing.T) {
	repo, sqlDB := repoConCanal(t)
	ctx := context.Background()

	if err := repo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: true, LatencyMs: 100, Web: domain.WebOK},
	}); err != nil {
		t.Fatalf("MarkBatch (1): %v", err)
	}
	if err := repo.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "s1", IsAlive: false, Web: domain.WebUnknown},
	}); err != nil {
		t.Fatalf("MarkBatch (2): %v", err)
	}

	var got sql.NullInt64
	if err := sqlDB.QueryRow(`SELECT web_ok FROM streams WHERE id = 's1'`).Scan(&got); err != nil {
		t.Fatalf("leyendo web_ok: %v", err)
	}
	if !got.Valid || got.Int64 != 1 {
		t.Errorf("web_ok = %+v, quiero que conserve 1", got)
	}
}
```

Si el fichero no tiene un helper `repoConCanal(t)`, escribirlo junto a los tests siguiendo el patrón que ya use el fichero para crear una DB temporal con un canal y dos streams `s1`/`s2`.

- [ ] **Step 2: Escribir el test de agregación por canal**

Añadir a `internal/adapters/db/channel_repository_test.go`:

```go
// El canal se ve en la web si ALGUNO de sus streams se ve. Sin comprobar
// sigue siendo null, igual que Alive: la app no puede distinguir "no" de
// "todavía no lo sé" si los colapsamos.
func TestFindFilteredAgregaWebOK(t *testing.T) {
	// Preparar: canal "c1" con un stream WebNo y otro WebOK; canal "c2" con
	// un único stream sin comprobar.
	repo, streams := reposConDosCanales(t)
	ctx := context.Background()

	if err := streams.MarkBatch(ctx, []ports.StreamHealth{
		{StreamID: "c1-a", IsAlive: true, LatencyMs: 100, Web: domain.WebNo},
		{StreamID: "c1-b", IsAlive: true, LatencyMs: 200, Web: domain.WebOK},
	}); err != nil {
		t.Fatalf("MarkBatch: %v", err)
	}

	canales, err := repo.FindFiltered(ctx, ports.ChannelFilter{})
	if err != nil {
		t.Fatalf("FindFiltered: %v", err)
	}

	porID := map[domain.ChannelID]domain.Channel{}
	for _, c := range canales {
		porID[c.ID] = c
	}

	c1 := porID["c1"]
	if c1.WebOK == nil || !*c1.WebOK {
		t.Errorf("c1.WebOK = %v, quiero true (uno de sus streams se ve)", c1.WebOK)
	}
	c2 := porID["c2"]
	if c2.WebOK != nil {
		t.Errorf("c2.WebOK = %v, quiero nil (sin comprobar)", c2.WebOK)
	}
}
```

Escribir `reposConDosCanales(t)` siguiendo el patrón de helpers que ya tenga el fichero.

- [ ] **Step 3: Correr los tests y verificar que fallan**

Run: `go test ./internal/adapters/db/ -run 'WebOK'`
Esperado: FAIL — `unknown field Web in struct literal` y `c.WebOK undefined`.

- [ ] **Step 4: Añadir la columna al esquema**

En `internal/adapters/db/schema.sql`, dentro de `CREATE TABLE IF NOT EXISTS streams`, tras la línea de `fail_count`:

```sql
    -- Reproducibilidad directa en navegador (veredicto estricto de hls.js).
    -- NULL = sin comprobar, y hay que distinguirlo de 0: la tarjeta pinta
    -- cosas distintas para "no se ve" y "todavía no lo sé".
    web_ok       INTEGER CHECK (web_ok IN (0,1)),
```

En `internal/adapters/db/db.go`, añadir a la lista `alters` de `alterMigrations`:

```go
		"ALTER TABLE streams ADD COLUMN web_ok INTEGER",
```

El ALTER va sin `CHECK`: SQLite no admite añadir una restricción con `ALTER TABLE ADD COLUMN` si no es constante, y la restricción solo protegería a las DBs nuevas de todas formas. La escritura la controla `MarkBatch`.

- [ ] **Step 5: Ampliar `StreamHealth` y `MarkBatch`**

En `internal/ports/stream_repository.go`, dentro de `StreamHealth`:

```go
	// Web es el veredicto de reproducibilidad en navegador. WebUnknown
	// (el cero valor) significa "no se pudo preguntar" y NO pisa lo que ya
	// hubiera guardado.
	Web domain.WebSupport
```

En `internal/adapters/db/stream_repository.go`, en `MarkBatch`, cambiar las dos sentencias preparadas y el bucle:

```go
	stmtVivo, err := tx.PrepareContext(ctx,
		`UPDATE streams
		 SET is_alive = 1, fail_count = 0, latency_ms = ?, last_checked = ?, updated_at = ?,
		     web_ok = COALESCE(?, web_ok)
		 WHERE id = ?`)
```

```go
	stmtMuerto, err := tx.PrepareContext(ctx,
		`UPDATE streams
		 SET fail_count   = fail_count + 1,
		     is_alive     = CASE WHEN fail_count + 1 >= ? THEN 0 ELSE is_alive END,
		     last_checked = ?,
		     updated_at   = ?,
		     web_ok       = COALESCE(?, web_ok)
		 WHERE id = ?`)
```

```go
	now := time.Now().Unix()
	for _, res := range resultados {
		if res.IsAlive {
			_, err = stmtVivo.ExecContext(ctx, res.LatencyMs, now, now, argWebOK(res.Web), res.StreamID)
		} else {
			_, err = stmtMuerto.ExecContext(ctx, DeadFailThreshold, now, now, argWebOK(res.Web), res.StreamID)
		}
		if err != nil {
			return fmt.Errorf("db.Stream.MarkBatch (Exec id=%s): %w", res.StreamID, err)
		}
	}
```

Y añadir al final del fichero:

```go
// argWebOK traduce el veredicto al argumento del COALESCE: nil para
// "no se sabe", que deja intacto lo que ya hubiera en la columna.
func argWebOK(v domain.WebSupport) any {
	switch v {
	case domain.WebOK:
		return int64(1)
	case domain.WebNo:
		return int64(0)
	default:
		return nil
	}
}
```

Añadir `"github.com/gdberysan/open-tv/internal/domain"` a los imports del fichero si no estuviera.

- [ ] **Step 6: Propagar el veredicto desde el worker**

En `internal/adapters/validator/worker.go`, dentro de `checkOnce`, en la construcción de `resultados`:

```go
			resultados = append(resultados, ports.StreamHealth{
				StreamID:  id,
				IsAlive:   res.IsAlive,
				LatencyMs: res.LatencyMs,
				Web:       res.Web,
			})
```

- [ ] **Step 7: Agregar por canal en `FindFiltered`**

En `internal/adapters/db/channel_repository.go`, en `FindFiltered`, añadir la subconsulta:

```go
	q := "SELECT" + channelColumns + `,
		EXISTS(SELECT 1 FROM streams s WHERE s.channel_id = channels.id AND s.last_checked IS NOT NULL) AS any_checked,
		EXISTS(SELECT 1 FROM streams s WHERE s.channel_id = channels.id AND s.is_alive = 1) AS any_alive,
		(SELECT MIN(s.latency_ms) FROM streams s WHERE s.channel_id = channels.id AND s.is_alive = 1) AS best_latency,
		(SELECT MAX(s.web_ok) FROM streams s WHERE s.channel_id = channels.id AND s.web_ok IS NOT NULL) AS web_ok
		FROM channels WHERE ` + whereSQL +
		" ORDER BY name COLLATE NOCASE, id LIMIT ? OFFSET ?"
```

`MAX` y no `MIN`: basta que UNO de los streams del canal se vea en el navegador para que el canal se vea. Y `NULL` sale solo si ninguno está comprobado, que es justo la semántica que queremos.

En `scanChannelsWithHealth`, añadir la variable, el destino del `Scan` y la traducción:

```go
			anyChecked, anyAlive                                  int
			bestLatency                                           sql.NullInt64
			webOK                                                 sql.NullInt64
```

```go
			&anyChecked, &anyAlive, &bestLatency, &webOK,
```

```go
		if webOK.Valid {
			v := webOK.Int64 == 1
			ch.WebOK = &v
		}
```

- [ ] **Step 8: Añadir el campo al dominio y mover el contrato a 15 claves**

En `internal/domain/channel.go`, tras `LatencyMs`:

```go
	// WebOK dice si el canal se reproduce DIRECTAMENTE en un navegador
	// (veredicto estricto: HTTPS + CORS + códecs de navegador). nil = ningún
	// stream comprobado aún. Campo ADITIVO: la app Flutter lo ignora, porque
	// Channel.fromJson lee claves por nombre.
	WebOK *bool `json:"WebOK"`
```

En `internal/domain/channel_test.go`, dentro de `TestChannelJSONCongelaElContratoConLaApp`: añadir `"WebOK"` a la lista de claves obligatorias, y cambiar el recuento:

```go
	if len(got) != 15 {
		t.Errorf("el JSON tiene %d claves, quiero 15: añadir un campo al dominio lo filtra al cable", len(got))
	}
```

Añadir además, en el mismo fichero, el test que documenta por qué esto es seguro:

```go
// El campo nuevo es ADITIVO. La app Flutter (mobile/lib/domain/models/
// channel.dart) construye Channel leyendo claves por nombre con
// `json['X'] as T?`, así que una clave de más la ignora. Lo que la rompería es
// quitar o renombrar una de las 14 originales — de eso se encarga la lista de
// arriba.
func TestWebOKEsNullCuandoNoSeHaComprobado(t *testing.T) {
	raw, err := json.Marshal(domain.Channel{ID: "x", Name: "X", ProviderType: domain.ProviderOpenSource})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v, ok := got["WebOK"]; !ok || v != nil {
		t.Errorf("WebOK = %v (presente=%v), quiero null", v, ok)
	}
}
```

- [ ] **Step 9: Correr los tests y verificar que pasan**

Run: `go test -race -count=1 ./internal/... -v -run 'WebOK|Contrato'`
Esperado: PASS. Después, la suite entera: `go test -race -count=1 ./...`.

- [ ] **Step 10: Verificar el fallo de verdad — que la app Flutter sigue leyendo bien**

Un test de Go no prueba que Dart tolere la clave nueva. Con el gateway compilado y una pasada de salud hecha:

```bash
curl -s 'http://127.0.0.1:8080/channels?limit=3' | python3 -m json.tool | head -40
cd mobile && flutter analyze && flutter test && cd ..
```

Esperado: el JSON trae `"WebOK"` (probablemente `null` antes de la primera pasada del health-worker), y `flutter test` sigue verde con **cero diffs bajo `mobile/`** (`git status --short mobile/` vacío).

- [ ] **Step 11: Commit**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./... && (cd mobile && flutter analyze && flutter test)
git status --short mobile/   # debe estar vacío
git add -A
git commit -m "feat(web-ok): persistir el veredicto web y agregarlo por canal

streams.web_ok (NULL = sin comprobar) y Channel.WebOK, la clave 15 del
contrato. Aditiva: la app Flutter lee claves por nombre y la ignora.
MAX() en la agregacion porque basta que UNO de los streams del canal se
vea; COALESCE en la escritura para que un 'no pude preguntar' no pise
lo que ya sabiamos.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 7: El reescritor de manifiestos HLS (puro)

Un manifiesto que pasa por el proxy es inútil si sus segmentos siguen apuntando al origen: el navegador los pediría directo y volvería a chocar con el CORS. Hay que reescribir **todas** las URIs. Esta tarea es la función pura; el handler es la siguiente.

**Files:**
- Create: `internal/proxy/manifiesto.go`, `internal/proxy/manifiesto_test.go`

**Interfaces:**
- Produces: `proxy.ReescribirManifiesto(base *url.URL, cuerpo, prefijo string) string`.

- [ ] **Step 1: Escribir el test**

Crear `internal/proxy/manifiesto_test.go`:

```go
package proxy_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/proxy"
)

const prefijo = "/proxy/hls?u="

func base(t *testing.T) *url.URL {
	t.Helper()
	u, err := url.Parse("https://cdn.example/live/master.m3u8")
	if err != nil {
		t.Fatalf("base: %v", err)
	}
	return u
}

func TestReescribeVariantesRelativasYAbsolutas(t *testing.T) {
	entrada := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f\"",
		"720/chunk.m3u8",
		"#EXT-X-STREAM-INF:BANDWIDTH=400000",
		"https://otro.example/480/chunk.m3u8?t=9",
		"",
	}, "\n")

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo)

	if !strings.Contains(got, prefijo+url.QueryEscape("https://cdn.example/live/720/chunk.m3u8")) {
		t.Errorf("la variante relativa no se resolvió contra la base:\n%s", got)
	}
	if !strings.Contains(got, prefijo+url.QueryEscape("https://otro.example/480/chunk.m3u8?t=9")) {
		t.Errorf("la variante absoluta no se envolvió:\n%s", got)
	}
	// Las líneas de etiqueta que no llevan URI se dejan intactas.
	if !strings.Contains(got, "#EXT-X-STREAM-INF:BANDWIDTH=1200000,CODECS=\"avc1.4d401f\"") {
		t.Errorf("se tocó una etiqueta sin URI:\n%s", got)
	}
}

// EXT-X-KEY y EXT-X-MAP llevan la URI en un atributo. Olvidarlos deja el
// stream cifrado sin clave y el fMP4 sin cabecera: silencio con manifiesto OK.
func TestReescribeAtributosURI(t *testing.T) {
	entrada := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-KEY:METHOD=AES-128,URI=\"clave.key\",IV=0x00",
		"#EXT-X-MAP:URI=\"init.mp4\"",
		"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"a\",URI=\"audio/es.m3u8\"",
		"seg1.ts",
		"",
	}, "\n")

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo)

	for _, quiero := range []string{
		"https://cdn.example/live/clave.key",
		"https://cdn.example/live/init.mp4",
		"https://cdn.example/live/audio/es.m3u8",
		"https://cdn.example/live/seg1.ts",
	} {
		if !strings.Contains(got, prefijo+url.QueryEscape(quiero)) {
			t.Errorf("falta %q reescrito:\n%s", quiero, got)
		}
	}
	// El resto del atributo sobrevive: IV se pierde y el descifrado falla.
	if !strings.Contains(got, "IV=0x00") {
		t.Errorf("se perdió IV:\n%s", got)
	}
	if !strings.Contains(got, "METHOD=AES-128") {
		t.Errorf("se perdió METHOD:\n%s", got)
	}
}

// EXT-X-BYTERANGE se aplica a la URI de la línea siguiente, que sí se
// reescribe. La etiqueta en sí no se toca; el Range lo reenvía el handler.
func TestNoTocaByterangeNiComentarios(t *testing.T) {
	entrada := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-TARGETDURATION:6",
		"#EXTINF:6.0,",
		"#EXT-X-BYTERANGE:75232@0",
		"seg.ts",
		"#EXT-X-ENDLIST",
		"",
	}, "\n")

	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo)

	for _, intacta := range []string{"#EXT-X-TARGETDURATION:6", "#EXTINF:6.0,", "#EXT-X-BYTERANGE:75232@0", "#EXT-X-ENDLIST"} {
		if !strings.Contains(got, intacta) {
			t.Errorf("se tocó %q:\n%s", intacta, got)
		}
	}
	if !strings.Contains(got, prefijo+url.QueryEscape("https://cdn.example/live/seg.ts")) {
		t.Errorf("no se reescribió el segmento:\n%s", got)
	}
}

// Una URI que no se puede resolver se deja como estaba: un manifiesto con una
// línea rara sigue reproduciendo el resto; uno al que le hemos comido una
// línea, no.
func TestURIIlegibleSeDejaIntacta(t *testing.T) {
	entrada := "#EXTM3U\n://esto no es una URL\n"
	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo)
	if !strings.Contains(got, "://esto no es una URL") {
		t.Errorf("se perdió la línea ilegible:\n%s", got)
	}
}

func TestConservaElNumeroDeLineas(t *testing.T) {
	entrada := "#EXTM3U\n\nseg1.ts\nseg2.ts\n"
	got := proxy.ReescribirManifiesto(base(t), entrada, prefijo)
	if a, b := strings.Count(entrada, "\n"), strings.Count(got, "\n"); a != b {
		t.Errorf("líneas: entrada %d, salida %d", a, b)
	}
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `go test ./internal/proxy/`
Esperado: FAIL — el paquete `proxy` no existe.

- [ ] **Step 3: Implementar el reescritor**

Crear `internal/proxy/manifiesto.go`:

```go
// Package proxy relaya HLS para el navegador de la MISMA máquina.
//
// Existe porque Chrome y Firefox necesitan CORS y el 32 % de los streams vivos
// del catálogo no lo mandan (censo 2026-08-22). Revierte, de forma deliberada
// y acotada, la regla "el gateway nunca proxya vídeo": es el precio de esos
// dos navegadores. Solo se monta si la dirección de escucha es loopback, y
// nunca forma parte del sitio hospedado.
package proxy

import (
	"net/url"
	"strings"
)

// ReescribirManifiesto reescribe TODAS las URIs de un manifiesto HLS para que
// pasen por prefijo (p.ej. "/proxy/hls?u=").
//
// Reescribir solo los segmentos no sirve: el navegador pediría la clave, el
// mapa de inicialización y las pistas de audio directamente al origen, y
// volvería a chocar con el mismo CORS por el que existe el proxy.
//
// base es la URL absoluta del manifiesto, necesaria para resolver las
// referencias relativas.
func ReescribirManifiesto(base *url.URL, cuerpo, prefijo string) string {
	lineas := strings.Split(cuerpo, "\n")
	for i, linea := range lineas {
		recortada := strings.TrimSpace(linea)
		switch {
		case recortada == "":
			// Se deja como está, con su \r si lo tenía.
		case strings.HasPrefix(recortada, "#"):
			lineas[i] = reescribirAtributoURI(base, linea, prefijo)
		default:
			lineas[i] = envolver(base, recortada, prefijo)
		}
	}
	return strings.Join(lineas, "\n")
}

// reescribirAtributoURI cambia el valor de URI="..." en una línea de etiqueta.
// Genérico a propósito: vale para EXT-X-KEY, EXT-X-MAP, EXT-X-MEDIA,
// EXT-X-I-FRAME-STREAM-INF, EXT-X-PART, EXT-X-PRELOAD-HINT y cualquier
// etiqueta futura que use el mismo atributo. Una lista de etiquetas conocidas
// se quedaría corta en silencio.
func reescribirAtributoURI(base *url.URL, linea, prefijo string) string {
	const marca = `URI="`
	i := strings.Index(linea, marca)
	if i < 0 {
		return linea
	}
	inicio := i + len(marca)
	fin := strings.Index(linea[inicio:], `"`)
	if fin < 0 {
		return linea
	}
	valor := linea[inicio : inicio+fin]
	return linea[:inicio] + envolver(base, valor, prefijo) + linea[inicio+fin:]
}

// envolver resuelve ref contra base y la mete en el prefijo del proxy. Si la
// referencia no se puede interpretar se devuelve tal cual: un manifiesto con
// una línea rara reproduce el resto; uno al que le hemos comido una línea, no.
func envolver(base *url.URL, ref, prefijo string) string {
	abs, err := base.Parse(ref)
	if err != nil {
		return ref
	}
	return prefijo + url.QueryEscape(abs.String())
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `go test ./internal/proxy/ -v`
Esperado: PASS en los cinco.

- [ ] **Step 5: Commit**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./...
git add internal/proxy/
git commit -m "feat(proxy): reescritor de manifiestos HLS

Reescribe TODAS las URIs, no solo los segmentos: la clave, el mapa de
inicializacion y las pistas de audio irian al origen y chocarian con el
mismo CORS por el que existe el proxy. Una URI ilegible se deja intacta:
mejor un manifiesto con una linea rara que uno con una linea menos.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 8: El proxy HLS de loopback

**Files:**
- Create: `internal/proxy/handler.go`, `internal/proxy/handler_test.go`
- Modify: `internal/api/router.go`

**Interfaces:**
- Produces: `proxy.NewHandler(prefijo string) *proxy.Handler` (implementa `http.Handler`), montado en `GET /proxy/hls?u=<url>`.
- Consumes: `proxy.ReescribirManifiesto` (Tarea 7), el parámetro `proxyActivo` de `api.NewRouter` (Tarea 3).

- [ ] **Step 1: Escribir los tests**

Crear `internal/proxy/handler_test.go`:

```go
package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/proxy"
)

func TestProxyReescribeElManifiestoQueRelaya(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sin ACAO: es justo el caso por el que existe el proxy.
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:6.0,\nseg1.ts\n"))
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/live.m3u8"), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("código %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "mpegurl") {
		t.Errorf("Content-Type = %q", ct)
	}
	quiero := "/proxy/hls?u=" + url.QueryEscape(origen.URL+"/seg1.ts")
	if !strings.Contains(rec.Body.String(), quiero) {
		t.Errorf("el segmento no se reescribió:\n%s", rec.Body.String())
	}
}

func TestProxyRelayaSegmentosYReenviaRange(t *testing.T) {
	var rangeVisto string
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeVisto = r.Header.Get("Range")
		// El proxy no puede reenviar cabeceras del cliente a ciegas.
		if r.Header.Get("Origin") != "" || r.Header.Get("Referer") != "" {
			t.Error("el proxy reenvió Origin/Referer del cliente")
		}
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write([]byte("bytes-de-video"))
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	req := httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg1.ts"), nil)
	req.Header.Set("Range", "bytes=0-1023")
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Referer", "http://127.0.0.1:8080/")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	if rangeVisto != "bytes=0-1023" {
		t.Errorf("Range reenviado = %q, quiero bytes=0-1023", rangeVisto)
	}
	if rec.Body.String() != "bytes-de-video" {
		t.Errorf("cuerpo = %q", rec.Body.String())
	}
}

// El proxy escucha en loopback, así que solo lo alcanza esta máquina — pero
// eso no lo convierte en un pasadizo hacia el router de casa.
func TestProxyRechazaDestinosPrivados(t *testing.T) {
	h := proxy.NewHandler("/proxy/hls?u=", false)
	for _, destino := range []string{
		"http://127.0.0.1:8080/health",
		"http://192.168.1.1/",
		"http://[::1]:8080/",
		"file:///etc/passwd",
		"http://169.254.169.254/latest/meta-data/",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
			"/proxy/hls?u="+url.QueryEscape(destino), nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s → %d, quiero 403", destino, rec.Code)
		}
	}
}

func TestProxySinParametroU(t *testing.T) {
	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("código %d, quiero 400", rec.Code)
	}
}

func TestProxyPropagaElErrorDelOrigen(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/live.m3u8"), nil))

	if rec.Code != http.StatusForbidden {
		t.Errorf("código %d, quiero 403 (el del origen)", rec.Code)
	}
}

// El proxy no puede convertirse en un descargador infinito.
func TestProxyCortaSegmentosGigantes(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		bloque := make([]byte, 1<<20)
		for i := 0; i < 60; i++ { // 60 MB > el tope de 50
			if _, err := w.Write(bloque); err != nil {
				return
			}
		}
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/seg.ts"), nil))

	if n := int64(rec.Body.Len()); n > proxy.MaxSegmentoBytes {
		t.Errorf("relayó %d bytes, el tope es %d", n, proxy.MaxSegmentoBytes)
	}
}

func TestProxyNoAnunciaCORS(t *testing.T) {
	origen := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = io.WriteString(w, "#EXTM3U\n")
	}))
	defer origen.Close()

	h := proxy.NewHandler("/proxy/hls?u=", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/proxy/hls?u="+url.QueryEscape(origen.URL+"/live.m3u8"), nil))

	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("el proxy copió el ACAO del origen (%q); la UI es del mismo origen y no lo necesita", v)
	}
}
```

- [ ] **Step 2: Correr los tests y verificar que fallan**

Run: `go test ./internal/proxy/ -run TestProxy`
Esperado: FAIL — `undefined: proxy.NewHandler`.

- [ ] **Step 3: Implementar el handler**

Crear `internal/proxy/handler.go`:

```go
package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxSegmentoBytes acota lo que el proxy relaya de una sola pieza. Un segmento
// HLS de 6 s son unos pocos MB; 50 MB es holgado y a la vez impide que un
// origen roto —o un "segmento" que en realidad es un fichero de horas— use el
// proxy como descargador infinito.
const MaxSegmentoBytes int64 = 50 << 20

// maxManifiestoBytes: un manifiesto de miles de segmentos ronda los cientos de
// KB. Se lee entero porque hay que reescribirlo.
const maxManifiestoBytes int64 = 5 << 20

// tiempoPeticion acota cada relay. Un stream en vivo que no manda un segmento
// en 30 s está roto.
const tiempoPeticion = 30 * time.Second

// userAgent: varios orígenes contestan 403 al default de Go, igual que le pasa
// al health-checker.
const userAgent = "VLC/3.0.20 LibVLC/3.0.20"

// Handler relaya HLS al navegador de la misma máquina.
type Handler struct {
	client      *http.Client
	prefijo     string
	privadasOK  bool
}

// NewHandler construye el proxy. prefijo es la ruta con la que se reescriben
// las URIs del manifiesto, normalmente "/proxy/hls?u=".
//
// permitirDestinosPrivados solo es true en tests: los servidores de httptest
// viven en 127.0.0.1, que en producción es exactamente lo que hay que
// bloquear. En el router se monta siempre con false.
func NewHandler(prefijo string, permitirDestinosPrivados bool) *Handler {
	return &Handler{
		prefijo:    prefijo,
		privadasOK: permitirDestinosPrivados,
		client: &http.Client{
			// Sin timeout de cliente: lo pone el contexto por petición, que
			// además cancela la copia en curso si el navegador cierra.
			Transport: &http.Transport{
				MaxConnsPerHost:   4,
				DisableKeepAlives: true,
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	crudo := r.URL.Query().Get("u")
	if crudo == "" {
		http.Error(w, "falta el parámetro u", http.StatusBadRequest)
		return
	}
	destino, err := url.Parse(crudo)
	if err != nil {
		http.Error(w, "u no es una URL", http.StatusBadRequest)
		return
	}
	if destino.Scheme != "http" && destino.Scheme != "https" {
		http.Error(w, "esquema no permitido", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), tiempoPeticion)
	defer cancel()

	if !h.privadasOK && destinoPrivado(ctx, destino.Hostname()) {
		http.Error(w, "destino no permitido", http.StatusForbidden)
		return
	}

	// Petición NUEVA, no un reenvío: las cabeceras del cliente (Origin,
	// Referer, Cookie) no tienen por qué viajar a un tercero.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, destino.String(), nil)
	if err != nil {
		http.Error(w, "no se pudo construir la petición", http.StatusBadGateway)
		return
	}
	req.Header.Set("User-Agent", userAgent)
	// Range sí viaja: EXT-X-BYTERANGE hace que el reproductor pida trozos.
	if rango := r.Header.Get("Range"); rango != "" {
		req.Header.Set("Range", rango)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, "el origen no respondió", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if esManifiesto(destino, resp.Header.Get("Content-Type")) {
		h.relayarManifiesto(w, resp, destino)
		return
	}
	h.relayarBytes(w, resp)
}

func (h *Handler) relayarManifiesto(w http.ResponseWriter, resp *http.Response, destino *url.URL) {
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, maxManifiestoBytes))
	if err != nil {
		http.Error(w, "manifiesto ilegible", http.StatusBadGateway)
		return
	}
	// La base es la URL FINAL: si hubo redirección, las relativas se resuelven
	// contra el sitio al que se llegó, no contra el que se pidió.
	base := destino
	if resp.Request != nil && resp.Request.URL != nil {
		base = resp.Request.URL
	}
	salida := ReescribirManifiesto(base, string(cuerpo), h.prefijo)

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.WriteString(w, salida)
}

func (h *Handler) relayarBytes(w http.ResponseWriter, resp *http.Response) {
	// Solo las cabeceras que el reproductor necesita. Nada de copiar el juego
	// entero: ahí viajan Set-Cookie y el ACAO del origen, y la UI es del mismo
	// origen y no quiere ninguno de los dos.
	for _, k := range []string{"Content-Type", "Content-Range", "Accept-Ranges", "Content-Length"} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	// El contexto de la petición cancela esta copia si el navegador se va.
	_, _ = io.Copy(w, io.LimitReader(resp.Body, MaxSegmentoBytes))
}

// esManifiesto mira el tipo de contenido y, si el origen no lo declara bien
// —cosa habitual—, la extensión.
func esManifiesto(u *url.URL, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "mpegurl") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".m3u8")
}

// destinoPrivado bloquea loopback, red privada, link-local y direcciones sin
// especificar. El proxy solo escucha en loopback, así que solo lo alcanza esta
// máquina — pero eso no lo convierte en un pasadizo hacia el router de casa o
// hacia los metadatos de una nube.
func destinoPrivado(ctx context.Context, host string) bool {
	if host == "" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ipPrivada(ip)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return true
	}
	for _, a := range ips {
		if ipPrivada(a.IP) {
			return true
		}
	}
	return false
}

func ipPrivada(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `go test -race -count=1 ./internal/proxy/ -v`
Esperado: PASS en los doce (cinco del reescritor, siete del handler).

Por qué `NewHandler` lleva ese segundo parámetro: todos los tests que de verdad relayan usan un `httptest.Server`, que vive en `127.0.0.1` — justo lo que el filtro de destinos bloquea. Sin la vía de escape, o no se puede testear el relay o no se puede testear el filtro. Los tests del relay pasan `true`; `TestProxyRechazaDestinosPrivados` pasa `false`; `router.go` monta siempre con `false`.

- [ ] **Step 5: Montarlo en el router, solo si es loopback**

En `internal/api/router.go`: borrar la línea `_ = proxyActivo` y añadir, después de las rutas de `/channels`:

```go
	// El proxy HLS solo existe cuando escuchamos en loopback. No hay flag para
	// forzarlo: un proxy abierto a la red es un relay de vídeo de terceros con
	// la IP de quien lo levante, y eso no se ofrece ni por accidente. Los
	// builds del snapshot tampoco lo incluyen porque nunca son loopback.
	if proxyActivo {
		ph := proxy.NewHandler(RutaProxy, false)
		r.Get("/proxy/hls", ph.ServeHTTP)
	}
```

y arriba, junto a los imports, la constante compartida:

```go
// RutaProxy es el prefijo con el que se reescriben las URIs del manifiesto y
// la ruta que las sirve. Una sola constante para que el reescritor y el router
// no puedan divergir.
const RutaProxy = "/proxy/hls?u="
```

- [ ] **Step 6: Test de que el proxy NO existe fuera de loopback**

Añadir a `internal/api/router_test.go`:

```go
// La regla estructural: sin loopback no hay proxy. Si alguien la relaja, este
// test es el que lo dice.
func TestProxySoloExisteEnLoopback(t *testing.T) {
	for _, c := range []struct {
		activo bool
		quiero int
	}{
		{true, http.StatusBadRequest},  // montado: se queja de que falta u
		{false, http.StatusNotFound},   // no montado: la ruta no existe
	} {
		r := api.NewRouter(slog.New(slog.DiscardHandler), repoVacio{}, provVacio{}, streamsVacio{}, nil, syncerFalso{}, c.activo)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/proxy/hls", nil))
		if rec.Code != c.quiero {
			t.Errorf("proxyActivo=%v → %d, quiero %d", c.activo, rec.Code, c.quiero)
		}
	}
}
```

- [ ] **Step 7: Correr la suite y verificar el fallo de verdad**

Run: `go test -race -count=1 ./...`

Y a mano, contra un canal real sin CORS (el proxy solo sirve para eso):

```bash
go build -o open-tv ./cmd/open-tv
DB_PATH=$PWD/.devdata/iptv.db ./open-tv --no-browser &
URL=$(curl -s 'http://127.0.0.1:8080/channels?limit=200' | python3 -c "import json,sys;print([c['ID'] for c in json.load(sys.stdin) if c.get('WebOK') is False][0])")
STREAM=$(curl -s "http://127.0.0.1:8080/channels/stream?id=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1]))" "$URL")" | python3 -c "import json,sys;print(json.load(sys.stdin)['url'])")
curl -s "http://127.0.0.1:8080/proxy/hls?u=$(python3 -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1],safe=''))" "$STREAM")" | head -20
```

Esperado: un manifiesto cuyas URIs empiezan todas por `/proxy/hls?u=`. Si sale vacío o con error, el canal elegido puede estar muerto: repetir con otro `WebOK:false`.

- [ ] **Step 8: Commit**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./...
git add -A
git commit -m "feat(proxy): relay HLS de loopback para Chrome y Firefox

Existe porque el 32% de los streams vivos no manda CORS. Solo se monta
si el listener real es loopback —se pregunta al listener, no a la
configuracion—, construye una peticion nueva en vez de reenviar las
cabeceras del cliente, y bloquea destinos privados para no ser un
pasadizo hacia la LAN.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 9: El cliente web embebido y `/health` completo

**Files:**
- Create: `internal/ui/ui.go`, `internal/ui/ui_test.go`, `internal/ui/dist/.gitkeep`, `internal/ui/dist/index.html` (provisional, se sobrescribe en la Tarea 10)
- Modify: `internal/api/router.go`, `internal/api/handlers/health_handler.go`, `internal/api/handlers/health_handler_test.go`, `cmd/open-tv/main.go`

**Interfaces:**
- Produces: `ui.Handler() (http.Handler, bool)`; `api.Options{ProxyActivo bool; Version string}`; `handlers.NewHealthHandler(db, syncer, handlers.Info{Version, WebUI, ProxyEnabled})`.
- **Cambio de firma:** `api.NewRouter(logger, repo, provider, streams, sqlDB, syncer, opts api.Options)` sustituye al séptimo parámetro `proxyActivo bool` de la Tarea 3. Se actualizan las tres llamadas (`cmd/open-tv/main.go`, `internal/api/router_test.go` ×N).

- [ ] **Step 1: Crear el hueco del embed**

`go:embed` falla **en tiempo de compilación** si el directorio no existe, así que el hueco se versiona.

```bash
mkdir -p internal/ui/dist
touch internal/ui/dist/.gitkeep
printf '<!doctype html><meta charset="utf-8"><title>Korven Open TV</title><div id="app">provisional</div>\n' > internal/ui/dist/index.html
git add -f internal/ui/dist/.gitkeep
```

El `index.html` provisional **no se versiona** (lo cubre el `.gitignore` de la Tarea 1) y lo sustituye el build de Vite en la Tarea 10. Existe ahora para que los tests de esta tarea tengan algo que servir.

- [ ] **Step 2: Escribir el test del servidor de UI**

Crear `internal/ui/ui_test.go`:

```go
package ui_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gdberysan/open-tv/internal/ui"
)

func TestHandlerSirveIndexEnLaRaiz(t *testing.T) {
	h, ok := ui.Handler()
	if !ok {
		t.Skip("no hay cliente construido en internal/ui/dist")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	// index.html NUNCA se cachea: es lo que apunta a los assets con hash, y
	// una copia vieja deja al usuario en una versión anterior sin saberlo.
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("Cache-Control de index = %q, quiero no-cache", cc)
	}
}

// Fallback SPA: una ruta del cliente que el servidor no conoce devuelve el
// index, no un 404. Sin esto, recargar en /canal/x rompe la app.
func TestHandlerFallbackSPA(t *testing.T) {
	h, ok := ui.Handler()
	if !ok {
		t.Skip("no hay cliente construido en internal/ui/dist")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/canal/bbc-one", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("código %d, quiero 200 con el index", rec.Code)
	}
}

// Un fichero con extensión que no existe SÍ es 404: devolver el index para
// /favicon.ico o /assets/roto.js convierte errores de red en HTML silencioso.
func TestHandlerFicheroInexistenteEs404(t *testing.T) {
	h, ok := ui.Handler()
	if !ok {
		t.Skip("no hay cliente construido en internal/ui/dist")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/no-existe.js", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("código %d, quiero 404", rec.Code)
	}
}
```

- [ ] **Step 3: Correr el test y verificar que falla**

Run: `go test ./internal/ui/`
Esperado: FAIL — el paquete no existe.

- [ ] **Step 4: Implementar `internal/ui`**

Crear `internal/ui/ui.go`:

```go
// Package ui sirve el cliente web embebido en el binario.
//
// El cliente se construye directamente en internal/ui/dist (build.outDir de
// Vite) y no en web/dist: go:embed no admite "..", así que el destino del
// build tiene que vivir junto al paquete que lo embebe.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler devuelve el servidor del cliente y si de verdad hay cliente. Un
// binario compilado sin haber construido web/ sigue sirviendo la API: es un
// modo degradado legítimo (el LaunchAgent de desarrollo, por ejemplo), no un
// fallo fatal.
func Handler() (http.Handler, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return &servidor{archivos: sub, ficheros: http.FileServerFS(sub)}, true
}

type servidor struct {
	archivos fs.FS
	ficheros http.Handler
}

func (s *servidor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	limpia := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))

	if limpia == "/" {
		s.servirIndex(w, r)
		return
	}

	if _, err := fs.Stat(s.archivos, strings.TrimPrefix(limpia, "/")); err == nil {
		// Los assets de Vite llevan hash en el nombre: son inmutables por
		// construcción y cachearlos un año es gratis y correcto.
		if strings.HasPrefix(limpia, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		s.ficheros.ServeHTTP(w, r)
		return
	}

	// Una ruta con extensión que no existe es un 404 de verdad: devolver el
	// index para /favicon.ico o /assets/roto.js esconde errores de red detrás
	// de HTML que el navegador no sabe interpretar.
	if path.Ext(limpia) != "" {
		http.NotFound(w, r)
		return
	}

	// Ruta del cliente: fallback SPA.
	s.servirIndex(w, r)
}

func (s *servidor) servirIndex(w http.ResponseWriter, r *http.Request) {
	datos, err := fs.ReadFile(s.archivos, "index.html")
	if err != nil {
		http.Error(w, "cliente web no disponible", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// no-cache y no no-store: el navegador puede guardarlo, pero tiene que
	// revalidarlo. Así un despliegue nuevo se ve al recargar.
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(datos)
}
```

- [ ] **Step 5: Correr el test y verificar que pasa**

Run: `go test ./internal/ui/ -v`
Esperado: PASS en los tres (sin SKIP, porque el `index.html` provisional existe).

- [ ] **Step 6: Ampliar `/health`**

En `internal/api/handlers/health_handler.go`, añadir el tipo y cambiar el constructor:

```go
// Info son los datos del binario que /health publica. version_ui y
// proxy_enabled no son adorno: la página de primer arranque decide con ellos
// qué mensaje enseñar, y `open-tv` los usa para reconocer que el puerto
// ocupado es otro Open TV y no un servicio ajeno.
type Info struct {
	Version      string
	WebUI        bool
	ProxyEnabled bool
}

type HealthHandler struct {
	db     *sql.DB
	syncer SyncStatus
	info   Info
}

func NewHealthHandler(db *sql.DB, syncer SyncStatus, info Info) *HealthHandler {
	if info.Version == "" {
		info.Version = "dev"
	}
	return &HealthHandler{db: db, syncer: syncer, info: info}
}
```

Y en `Get`, sustituir la línea de `res` por:

```go
	res := map[string]any{
		"status":        "ok",
		"db":            "ok",
		"version":       h.info.Version,
		"web_ui":        h.info.WebUI,
		"proxy_enabled": h.info.ProxyEnabled,
	}
```

- [ ] **Step 7: Test de `/health`**

Añadir a `internal/api/handlers/health_handler_test.go`:

```go
func TestHealthPublicaLaInfoDelBinario(t *testing.T) {
	h := handlers.NewHealthHandler(dbDePrueba(t), syncerFalso{}, handlers.Info{
		Version: "1.2.3", WebUI: true, ProxyEnabled: true,
	})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for k, quiero := range map[string]any{"version": "1.2.3", "web_ui": true, "proxy_enabled": true} {
		if got[k] != quiero {
			t.Errorf("%s = %v, quiero %v", k, got[k], quiero)
		}
	}
}

// Sin versión inyectada por el linker (go run, go test) la respuesta dice
// "dev" y no una cadena vacía que parezca un bug.
func TestHealthVersionPorDefecto(t *testing.T) {
	h := handlers.NewHealthHandler(dbDePrueba(t), syncerFalso{}, handlers.Info{})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["version"] != "dev" {
		t.Errorf("version = %v, quiero \"dev\"", got["version"])
	}
}
```

Usar los helpers que ya tenga el fichero para la DB y el syncer falso; si no existen, escribirlos siguiendo su patrón.

- [ ] **Step 8: Cambiar la firma del router y montarlo todo**

En `internal/api/router.go`:

```go
// Options son los datos que el router necesita del proceso: qué puede montar
// y qué versión anunciar.
type Options struct {
	// ProxyActivo lo decide el listener real (loopback o no), nunca la
	// configuración: LISTEN_ADDR puede decir "localhost" y resolver a otra cosa.
	ProxyActivo bool
	Version     string
}

func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, sqlDB *sql.DB, syncer handlers.SyncStatus, opts Options) http.Handler {
```

Sustituir el montaje de `/health` y añadir el de la UI **al final**, después de todas las rutas de API:

```go
	clienteWeb, hayClienteWeb := ui.Handler()

	hh := handlers.NewHealthHandler(sqlDB, syncer, handlers.Info{
		Version:      opts.Version,
		WebUI:        hayClienteWeb,
		ProxyEnabled: opts.ProxyActivo,
	})
	r.Get("/health", hh.Get)
```

```go
	// La UI va la ÚLTIMA: NotFound solo se aplica a lo que ninguna ruta de API
	// haya reclamado, y así /channels/loquesea sigue siendo un 404 de API y no
	// devuelve el index.
	if hayClienteWeb {
		r.NotFound(clienteWeb.ServeHTTP)
	}
```

Añadir `"github.com/gdberysan/open-tv/internal/ui"` y `"github.com/gdberysan/open-tv/internal/proxy"` a los imports.

- [ ] **Step 9: Actualizar las llamadas**

En `cmd/open-tv/main.go`:

```go
		Handler: api.NewRouter(logger, channelRepoRO, provider, streamRepoRO, lecturaDB, syncer, api.Options{
			ProxyActivo: esLoopback(ln),
			Version:     version,
		}),
```

y arriba del fichero:

```go
// version la inyecta el linker en las releases (-ldflags "-X main.version=…").
// En desarrollo se queda en "dev", que es exactamente lo que es.
var version = "dev"
```

En `internal/api/router_test.go`, sustituir los `false`/`true` del séptimo argumento por `api.Options{}` y `api.Options{ProxyActivo: true}` según el caso.

- [ ] **Step 10: Correr la suite completa**

Run: `gofmt -l . && go vet ./... && go test -race -count=1 ./...`
Esperado: verde.

- [ ] **Step 11: Verificar a mano que la raíz sirve el cliente y la API sigue viva**

```bash
go build -o open-tv ./cmd/open-tv
DB_PATH=$PWD/.devdata/iptv.db ./open-tv --no-browser &
sleep 1
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' http://127.0.0.1:8080/
curl -s http://127.0.0.1:8080/health | python3 -m json.tool
curl -s -o /dev/null -w '%{http_code}\n' 'http://127.0.0.1:8080/channels?limit=1'
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/canal/loquesea
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/assets/no-existe.js
```

Esperado: `200 text/html`; `/health` con `version:"dev"`, `web_ui:true`, `proxy_enabled:true`; `/channels` 200; la ruta SPA 200; el asset inexistente 404.

- [ ] **Step 12: Commit**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./... && (cd mobile && flutter analyze && flutter test)
git add -A
git commit -m "feat(ui): cliente web embebido, fallback SPA y /health completo

go:embed sobre internal/ui/dist (no web/dist: go:embed no admite ..).
La UI se monta como NotFound del router, la ULTIMA, para que
/channels/loquesea siga siendo un 404 de API. index.html no-cache,
assets con hash inmutables un anio, y un fichero con extension que no
existe sigue siendo 404: devolver el index ahi esconde errores de red.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 10: Andamiaje de `web/` — Svelte 5, tokens de Korven, i18n tipado

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/svelte.config.js`, `web/tsconfig.json`, `web/index.html`
- Create: `web/src/main.ts`, `web/src/App.svelte`
- Create: `web/src/estilos/tokens/{base,colors,effects,fonts,spacing,typography}.css`, `web/src/estilos/global.css`
- Create: `web/src/i18n/{es.ts,en.ts,index.ts}`, `web/src/i18n/i18n.test.ts`
- Modify: `.gitignore`

**Interfaces:**
- Produces: `t(clave, params?)` y el store `idioma` desde `web/src/i18n/index.ts`; el tipo `ClaveMensaje`; los scripts `npm run {dev,build,check,test,test:e2e}`; el build que aterriza en `internal/ui/dist`.

- [ ] **Step 1: Crear el proyecto y fijar dependencias**

```bash
cd web 2>/dev/null || mkdir web && cd web
npm create vite@latest . -- --template svelte-ts
npm pkg set name=korven-open-tv-web private=true type=module
npm i --save-exact hls.js
npm i --save-exact -D vitest jsdom @testing-library/svelte @playwright/test svelte-check
npm i
```

Las versiones que resuelvan quedan **fijadas exactas** en `package.json` y el `package-lock.json` se versiona. Anotar las versiones resueltas en el mensaje de commit.

- [ ] **Step 2: Configurar Vite para que construya dentro del paquete Go**

Sustituir `web/vite.config.ts` por:

```ts
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// El cliente se construye DENTRO del paquete Go que lo embebe: go:embed no
// admite "..", así que web/dist no serviría. El binario y el cliente son un
// solo artefacto y esto es lo que lo hace literal.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/ui/dist',
    emptyOutDir: true,
    // El presupuesto es 80 KB gzip de JS propio (spec §3.1). hls.js va en su
    // propio trozo y solo se descarga al reproducir.
    chunkSizeWarningLimit: 100,
  },
  server: {
    // En desarrollo el cliente corre en 5173 y la API en 8080. Sin este proxy
    // el navegador haría peticiones cruzadas — y el CORS se retiró a
    // propósito, así que fallarían. En producción todo es el mismo origen.
    proxy: {
      '/channels': 'http://127.0.0.1:8080',
      '/health': 'http://127.0.0.1:8080',
      '/proxy': 'http://127.0.0.1:8080',
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
  },
})
```

Añadir los scripts a `web/package.json`:

```json
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "check": "svelte-check --tsconfig ./tsconfig.json",
    "test": "vitest run",
    "test:e2e": "playwright test"
  }
```

- [ ] **Step 3: Ignorar lo que no se versiona**

Añadir a `.gitignore` de la raíz:

```
# Cliente web
/web/node_modules/
/web/test-results/
/web/playwright-report/
/web/tests/fixtures/hls/
```

- [ ] **Step 4: Copiar los tokens de Korven tal cual**

```bash
cp ~/Dev/korven/design/design_handoff_korven_sitio/tokens/*.css web/src/estilos/tokens/
ls web/src/estilos/tokens/
```

Esperado: `base.css colors.css effects.css fonts.css spacing.css typography.css`.

**Se copian sin editar.** Si un token no encaja, se compone encima en `global.css`; tocar el token aquí hace que el sistema de diseño y este cliente diverjan en silencio.

Crear `web/src/estilos/global.css`:

```css
@import './tokens/base.css';
@import './tokens/colors.css';
@import './tokens/fonts.css';
@import './tokens/spacing.css';
@import './tokens/typography.css';
@import './tokens/effects.css';

/* El ámbar es SEÑAL VIVA: canal en directo, filtro activo, foco. Un botón
   neutro no lleva ámbar. Esta regla gobierna todo el tema; ver la memoria del
   proyecto y el sistema de diseño de Korven. */

html, body {
  margin: 0;
  background: var(--surface-base);
  color: var(--text-body);
  font-family: var(--font-body, system-ui, sans-serif);
}

:focus-visible {
  outline: 2px solid var(--amber-500);
  outline-offset: 2px;
}
```

- [ ] **Step 5: Escribir el test de i18n**

Crear `web/src/i18n/i18n.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { es } from './es'
import { en } from './en'
import { t, idioma } from './index'

describe('diccionario', () => {
  // El tipo Record<ClaveMensaje, string> ya impide que falte una clave en
  // inglés: esto atrapa el caso contrario, una clave de MÁS en inglés que
  // nadie usa y que se queda ahí para siempre.
  it('tiene exactamente las mismas claves en los dos idiomas', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(es).sort())
  })

  it('no deja ningún texto vacío', () => {
    for (const [clave, valor] of Object.entries({ ...es, ...en })) {
      expect(valor, `la clave ${clave} está vacía`).not.toBe('')
    }
  })

  it('interpola parámetros', () => {
    idioma.set('es')
    expect(t('catalogo.total', { n: '12639' })).toContain('12639')
  })

  it('cambia de idioma', () => {
    idioma.set('en')
    const ingles = t('accion.aleatorio')
    idioma.set('es')
    expect(t('accion.aleatorio')).not.toBe(ingles)
  })
})
```

- [ ] **Step 6: Correr el test y verificar que falla**

Run: `cd web && npm test`
Esperado: FAIL — no existen `./es`, `./en` ni `./index`.

- [ ] **Step 7: Escribir el diccionario**

Crear `web/src/i18n/es.ts`:

```ts
// El español es la fuente: sus claves definen el tipo, así que una clave que
// falte en inglés rompe el typecheck y no llega a producción.
export const es = {
  'app.titulo': 'Korven Open TV',
  'app.lema': 'Televisión abierta, sin cuentas y sin configuración',

  'catalogo.buscar': 'Buscar un canal',
  'catalogo.vacio': 'Ningún canal casa con el filtro.',
  'catalogo.cargando': 'Cargando canales…',
  'catalogo.total': '{n} canales',

  'filtro.pais': 'País',
  'filtro.categoria': 'Categoría',
  'filtro.calidad': 'Calidad',
  'filtro.todos': 'Todos',
  'filtro.limpiar': 'Limpiar filtros',
  'filtro.ocultarOffline': 'Ocultar los que no responden',

  'accion.aleatorio': 'Canal al azar',
  'accion.favoritos': 'Solo favoritos',
  'accion.rejilla': 'Ver en rejilla',
  'accion.lista': 'Ver en lista',

  'canal.favorito.anadir': 'Añadir a favoritos',
  'canal.favorito.quitar': 'Quitar de favoritos',
  'canal.soloApp': 'Este canal se ve en la app instalada o en Safari',

  'senal.viva': 'Señal viva',
  'senal.muerta': 'No responde',
  'senal.sinDatos': 'Sin comprobar',

  'reproductor.cargando': 'Conectando con el canal…',
  'reproductor.cerrar': 'Cerrar',
  'reproductor.silenciar': 'Silenciar',
  'reproductor.pantallaCompleta': 'Pantalla completa',
  'reproductor.error.noArranco': 'El canal no llegó a reproducir. Puede estar caído, geo-bloqueado o su dirección caducó.',
  'reproductor.error.corte': 'El canal dejó de emitir.',

  'estado.sincronizando': 'Sincronizando el catálogo…',
  'estado.sincronizandoDetalle': 'La primera vez tarda unos segundos: se descargan unos 13 000 canales.',
  'estado.gatewayCaido': 'No se pudo contactar con Open TV. ¿Sigue abierto?',
  'estado.sinRed': 'Sin conexión a internet.',

  'frescura.comprobado': 'Comprobado hace {horas} h',
  'frescura.envivo': 'Comprobado en vivo',

  'pie.fuente': 'La fuente es la lista pública de televisión abierta de iptv-org. Korven no retransmite nada.',
  'pie.postura': 'Sin canales premium, sin VPN, sin elusión de geobloqueo.',
  'pie.codigo': 'Ver el código',

  'idioma.es': 'Español',
  'idioma.en': 'English',
} as const

export type ClaveMensaje = keyof typeof es
```

Crear `web/src/i18n/en.ts`:

```ts
import type { ClaveMensaje } from './es'

// Record<ClaveMensaje, string>: si falta una clave, el build falla. No hay
// forma de publicar una UI a medio traducir.
export const en: Record<ClaveMensaje, string> = {
  'app.titulo': 'Korven Open TV',
  'app.lema': 'Free-to-air television, no accounts, no setup',

  'catalogo.buscar': 'Search a channel',
  'catalogo.vacio': 'No channel matches the filter.',
  'catalogo.cargando': 'Loading channels…',
  'catalogo.total': '{n} channels',

  'filtro.pais': 'Country',
  'filtro.categoria': 'Category',
  'filtro.calidad': 'Quality',
  'filtro.todos': 'All',
  'filtro.limpiar': 'Clear filters',
  'filtro.ocultarOffline': 'Hide the ones not responding',

  'accion.aleatorio': 'Random channel',
  'accion.favoritos': 'Favourites only',
  'accion.rejilla': 'Grid view',
  'accion.lista': 'List view',

  'canal.favorito.anadir': 'Add to favourites',
  'canal.favorito.quitar': 'Remove from favourites',
  'canal.soloApp': 'This channel plays in the installed app or in Safari',

  'senal.viva': 'Live signal',
  'senal.muerta': 'Not responding',
  'senal.sinDatos': 'Not checked yet',

  'reproductor.cargando': 'Connecting to the channel…',
  'reproductor.cerrar': 'Close',
  'reproductor.silenciar': 'Mute',
  'reproductor.pantallaCompleta': 'Full screen',
  'reproductor.error.noArranco': 'The channel never started playing. It may be down, geo-blocked, or its address expired.',
  'reproductor.error.corte': 'The channel stopped broadcasting.',

  'estado.sincronizando': 'Syncing the catalogue…',
  'estado.sincronizandoDetalle': 'The first run takes a few seconds: about 13,000 channels are downloaded.',
  'estado.gatewayCaido': 'Could not reach Open TV. Is it still running?',
  'estado.sinRed': 'No internet connection.',

  'frescura.comprobado': 'Checked {horas} h ago',
  'frescura.envivo': 'Checked live',

  'pie.fuente': 'The source is the public free-to-air list from iptv-org. Korven broadcasts nothing.',
  'pie.postura': 'No premium channels, no VPN, no geo-block circumvention.',
  'pie.codigo': 'View the code',

  'idioma.es': 'Español',
  'idioma.en': 'English',
}
```

Crear `web/src/i18n/index.ts`:

```ts
import { writable, get } from 'svelte/store'
import { es, type ClaveMensaje } from './es'
import { en } from './en'

export type Idioma = 'es' | 'en'

const diccionarios: Record<Idioma, Record<ClaveMensaje, string>> = { es, en }

const CLAVE_ALMACEN = 'opentv.idioma'

function idiomaInicial(): Idioma {
  try {
    const guardado = localStorage.getItem(CLAVE_ALMACEN)
    if (guardado === 'es' || guardado === 'en') return guardado
  } catch {
    // Navegación privada o almacenamiento bloqueado: el idioma del navegador
    // sigue sirviendo, no es motivo para fallar.
  }
  const nav = typeof navigator !== 'undefined' ? navigator.language : 'es'
  return nav.toLowerCase().startsWith('en') ? 'en' : 'es'
}

export const idioma = writable<Idioma>(idiomaInicial())

idioma.subscribe((v) => {
  try {
    localStorage.setItem(CLAVE_ALMACEN, v)
  } catch {
    // Ver arriba.
  }
  if (typeof document !== 'undefined') document.documentElement.lang = v
})

/** t traduce y sustituye {parametros}. */
export function t(clave: ClaveMensaje, params?: Record<string, string | number>): string {
  const texto = diccionarios[get(idioma)][clave]
  if (!params) return texto
  return texto.replace(/\{(\w+)\}/g, (crudo, nombre) =>
    nombre in params ? String(params[nombre]) : crudo,
  )
}
```

- [ ] **Step 8: Correr el test y verificar que pasa**

Run: `cd web && npm test && npm run check`
Esperado: los cuatro tests PASS y `svelte-check` sin errores.

- [ ] **Step 9: Comprobar que el build aterriza donde lo espera Go**

```bash
cd web && npm run build && cd ..
ls internal/ui/dist
go build ./... && go test ./internal/ui/ -v
```

Esperado: `index.html` y `assets/` dentro de `internal/ui/dist`; los tests de `ui` pasan sin SKIP.

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(web): andamiaje del cliente Svelte 5 con tokens de Korven e i18n tipado

El build aterriza en internal/ui/dist porque go:embed no admite '..':
binario y cliente son un solo artefacto y esto lo hace literal. El
diccionario ingles es Record<ClaveMensaje,string>, asi que una clave
sin traducir rompe el build en vez de llegar a produccion.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 11: `CatalogSource` y `HttpCatalog`

La costura que hace posible P3: el cliente no sabe si habla con un gateway vivo o con un montón de JSON estático. En P0 solo existe la implementación HTTP.

**Files:**
- Create: `web/src/datos/catalogo.ts` (tipos + interfaz), `web/src/datos/http.ts`, `web/src/datos/http.test.ts`

**Interfaces:**
- Produces: `Canal`, `Faceta`, `ConsultaCatalogo`, `PaginaCanales`, `Frescura`, `CatalogSource`, `crearHttpCatalog(base?: string): CatalogSource`.
- Consumes: las claves congeladas del gateway (`ID`, `Name`, `LogoURL`, `CategoryID`, `LanguageCode`, `CountryCode`, `Alive`, `LatencyMs`, `WebOK`, `ProviderType`) y la cabecera `X-Total-Count`.

- [ ] **Step 1: Escribir el test**

Crear `web/src/datos/http.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { crearHttpCatalog } from './http'

function respuesta(cuerpo: unknown, cabeceras: Record<string, string> = {}) {
  return new Response(JSON.stringify(cuerpo), {
    status: 200,
    headers: { 'Content-Type': 'application/json', ...cabeceras },
  })
}

afterEach(() => vi.unstubAllGlobals())

describe('HttpCatalog', () => {
  // El mapeo de nombres vive en UN solo sitio. Las claves del gateway están
  // congeladas por un test de contrato en Go y son nombres de campo de Go;
  // dentro del cliente se usan nombres propios.
  it('traduce las claves congeladas del gateway', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta(
        [{
          ID: 'opensource-BBC One', Name: 'BBC One (1080p)', LogoURL: 'http://logo',
          CategoryID: 'General;News', LanguageCode: 'en', CountryCode: 'GB',
          Alive: true, LatencyMs: 120, WebOK: false, ProviderType: 'opensource',
        }],
        { 'X-Total-Count': '12639' },
      ),
    ))

    const c = crearHttpCatalog('')
    const pagina = await c.canales({})

    expect(pagina.total).toBe(12639)
    expect(pagina.canales[0]).toMatchObject({
      id: 'opensource-BBC One',
      nombre: 'BBC One (1080p)',
      pais: 'GB',
      vivo: true,
      latenciaMs: 120,
      webOk: false,
    })
  })

  // null NO es false. Un canal sin comprobar se pinta distinto de uno que no
  // se ve, en la señal y en la marca web.
  it('conserva null en vivo y webOk', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta([{ ID: 'x', Name: 'X', Alive: null, WebOK: null, LatencyMs: 0 }]),
    ))

    const pagina = await crearHttpCatalog('').canales({})
    expect(pagina.canales[0].vivo).toBeNull()
    expect(pagina.canales[0].webOk).toBeNull()
  })

  it('construye la query con los filtros', async () => {
    const espia = vi.fn(async () => respuesta([]))
    vi.stubGlobal('fetch', espia)

    await crearHttpCatalog('').canales({
      q: 'bbc', pais: 'GB', categoria: 'News', calidad: 'hd',
      limite: 500, desplazamiento: 1000,
    })

    const url = String(espia.mock.calls[0][0])
    expect(url).toContain('q=bbc')
    expect(url).toContain('country=GB')
    expect(url).toContain('category=News')
    expect(url).toContain('quality=hd')
    expect(url).toContain('limit=500')
    expect(url).toContain('offset=1000')
  })

  // Por defecto el gateway ya oculta los muertos. "Mostrar offline" es
  // ?alive=all, que es lo que hace el toggle.
  it('mostrarOffline manda alive=all', async () => {
    const espia = vi.fn(async () => respuesta([]))
    vi.stubGlobal('fetch', espia)

    await crearHttpCatalog('').canales({ mostrarOffline: true })
    expect(String(espia.mock.calls[0][0])).toContain('alive=all')
  })

  // Con el conjunto de favoritos VACÍO hay que mandar un centinela: unos ids
  // vacíos significan "sin filtro" y devolverían los 12 000 canales, que es
  // exactamente lo contrario de lo que pidió el usuario.
  it('favoritos vacíos no devuelven el catálogo entero', async () => {
    const espia = vi.fn(async () => respuesta([]))
    vi.stubGlobal('fetch', espia)

    const pagina = await crearHttpCatalog('').canales({ ids: [] })
    expect(pagina.canales).toEqual([])
    expect(espia).not.toHaveBeenCalled()
  })

  it('un fallo de red se distingue de un gateway caído', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    await expect(crearHttpCatalog('').canales({})).rejects.toThrow(/red|gateway/i)
  })
})
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd web && npm test`
Esperado: FAIL — no existe `./http`.

- [ ] **Step 3: Escribir los tipos y la interfaz**

Crear `web/src/datos/catalogo.ts`:

```ts
/** Canal tal y como lo usa el cliente. Nombres propios, no los del cable. */
export interface Canal {
  id: string
  nombre: string
  logoUrl: string
  categoriaId: string
  idioma: string
  pais: string
  /** null = ningún stream comprobado aún. NO es lo mismo que false. */
  vivo: boolean | null
  latenciaMs: number
  /** Veredicto estricto (hls.js). null = sin comprobar. */
  webOk: boolean | null
}

export interface Faceta {
  valor: string
  total: number
}

export interface ConsultaCatalogo {
  q?: string
  pais?: string
  categoria?: string
  calidad?: string
  mostrarOffline?: boolean
  /** Filtro de favoritos. Un array VACÍO significa "ninguno", no "sin filtro". */
  ids?: string[]
  limite?: number
  desplazamiento?: number
}

export interface PaginaCanales {
  canales: Canal[]
  total: number
}

export interface Frescura {
  /** 'vivo' = gateway local; 'instantanea' = snapshot estático (P3). */
  tipo: 'vivo' | 'instantanea'
  generadoEn: Date | null
}

export interface DestinoStream {
  url: string
  airplayOk: boolean | null
}

/**
 * CatalogSource es la costura entre el gateway vivo y el snapshot estático.
 * El cliente no sabe en cuál está salvo por la línea de frescura.
 */
export interface CatalogSource {
  canales(c: ConsultaCatalogo): Promise<PaginaCanales>
  paises(): Promise<Faceta[]>
  categorias(): Promise<Faceta[]>
  aleatorio(c: ConsultaCatalogo): Promise<Canal>
  destino(id: string): Promise<DestinoStream>
  frescura(): Promise<Frescura>
  /** El proxy solo existe en el binario local. */
  proxyDisponible(): Promise<boolean>
}
```

- [ ] **Step 4: Implementar `HttpCatalog`**

Crear `web/src/datos/http.ts`:

```ts
import type {
  Canal, CatalogSource, ConsultaCatalogo, DestinoStream, Faceta, Frescura, PaginaCanales,
} from './catalogo'

/**
 * Las claves del cable son nombres de campo de Go y están CONGELADAS por un
 * test de contrato (internal/domain/channel_test.go). Este es el único sitio
 * del cliente que las conoce.
 */
interface CanalCable {
  ID: string
  Name: string
  LogoURL?: string
  CategoryID?: string
  LanguageCode?: string
  CountryCode?: string
  Alive?: boolean | null
  LatencyMs?: number
  WebOK?: boolean | null
}

function aCanal(c: CanalCable): Canal {
  return {
    id: c.ID,
    nombre: c.Name,
    logoUrl: c.LogoURL ?? '',
    categoriaId: c.CategoryID ?? '',
    idioma: c.LanguageCode ?? '',
    pais: c.CountryCode ?? '',
    // ?? null y no ?? false: "sin comprobar" es un tercer estado con su propio
    // dibujo en la tarjeta.
    vivo: c.Alive ?? null,
    latenciaMs: c.LatencyMs ?? 0,
    webOk: c.WebOK ?? null,
  }
}

function query(c: ConsultaCatalogo): URLSearchParams {
  const p = new URLSearchParams()
  if (c.q) p.set('q', c.q)
  if (c.pais) p.set('country', c.pais)
  if (c.categoria) p.set('category', c.categoria)
  if (c.calidad) p.set('quality', c.calidad)
  if (c.mostrarOffline) p.set('alive', 'all')
  if (c.ids?.length) p.set('ids', c.ids.join(','))
  if (c.limite != null) p.set('limit', String(c.limite))
  if (c.desplazamiento != null) p.set('offset', String(c.desplazamiento))
  return p
}

async function pedir(url: string): Promise<Response> {
  let resp: Response
  try {
    resp = await fetch(url)
  } catch (e) {
    // fetch solo lanza por fallo de transporte. Distinguirlo importa: el
    // mensaje "gateway caído" y el "sin red" son problemas distintos con
    // soluciones distintas, y confundirlos costó una tarde en agosto.
    throw new Error(
      typeof navigator !== 'undefined' && navigator.onLine === false
        ? 'sin red'
        : 'gateway inalcanzable',
      { cause: e },
    )
  }
  if (!resp.ok) throw new Error(`respuesta ${resp.status}`)
  return resp
}

export function crearHttpCatalog(base = ''): CatalogSource {
  return {
    async canales(c: ConsultaCatalogo): Promise<PaginaCanales> {
      // Centinela del conjunto vacío: sin esto, unos ids vacíos serían "sin
      // filtro" y el gateway devolvería los 12 000 canales.
      if (c.ids && c.ids.length === 0) return { canales: [], total: 0 }

      const resp = await pedir(`${base}/channels?${query(c)}`)
      const crudos = (await resp.json()) as CanalCable[] | null
      const canales = (crudos ?? []).map(aCanal)
      const cabecera = resp.headers.get('X-Total-Count')
      return { canales, total: cabecera ? Number(cabecera) : canales.length }
    },

    async paises(): Promise<Faceta[]> {
      const resp = await pedir(`${base}/channels/countries`)
      const crudas = (await resp.json()) as Array<{ Valor: string; Count: number }> | null
      return (crudas ?? []).map((f) => ({ valor: f.Valor, total: f.Count }))
    },

    async categorias(): Promise<Faceta[]> {
      const resp = await pedir(`${base}/channels/categories`)
      const crudas = (await resp.json()) as Array<{ Valor: string; Count: number }> | null
      return (crudas ?? []).map((f) => ({ valor: f.Valor, total: f.Count }))
    },

    async aleatorio(c: ConsultaCatalogo): Promise<Canal> {
      const resp = await pedir(`${base}/channels/random?${query(c)}`)
      return aCanal((await resp.json()) as CanalCable)
    },

    async destino(id: string): Promise<DestinoStream> {
      const resp = await pedir(`${base}/channels/stream?id=${encodeURIComponent(id)}`)
      const cuerpo = (await resp.json()) as { url: string; airplay_ok?: boolean | null }
      return { url: cuerpo.url, airplayOk: cuerpo.airplay_ok ?? null }
    },

    async frescura(): Promise<Frescura> {
      return { tipo: 'vivo', generadoEn: null }
    },

    async proxyDisponible(): Promise<boolean> {
      try {
        const resp = await pedir(`${base}/health`)
        const cuerpo = (await resp.json()) as { proxy_enabled?: boolean }
        return cuerpo.proxy_enabled === true
      } catch {
        return false
      }
    },
  }
}
```

- [ ] **Step 5: Correr los tests y verificar que pasan**

Run: `cd web && npm test && npm run check`
Esperado: los seis tests de `HttpCatalog` PASS, más los cuatro de i18n.

- [ ] **Step 6: Commit**

```bash
git add web/src/datos/
git commit -m "feat(web): CatalogSource y la implementacion HTTP

La costura que hace posible el snapshot estatico de P3: el cliente no
sabe si habla con un gateway vivo o con JSON. Un solo fichero conoce
las claves congeladas del cable. Los favoritos vacios mandan centinela:
unos ids vacios serian 'sin filtro' y devolverian los 12 000 canales.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 12: El catálogo — rejilla, filtros, favoritos

**Files:**
- Create: `web/src/lib/{debounce.ts,senal.ts,senal.test.ts,debounce.test.ts}`
- Create: `web/src/estado/{favoritos.ts,favoritos.test.ts,filtros.ts}`
- Create: `web/src/componentes/{BarrasSenal.svelte,TarjetaCanal.svelte,RejillaCanales.svelte,BarraFiltros.svelte,MarcaWeb.svelte}`
- Create: `web/src/componentes/TarjetaCanal.test.ts`
- Modify: `web/src/App.svelte`, `web/src/main.ts`

**Interfaces:**
- Produces: `nivelSenal(vivo, latenciaMs): NivelSenal`; `debounce(fn, ms)`; el store `favoritos` con `alternar(id)` y `esFavorito(id)`; el store `filtros`; los componentes de arriba.
- Consumes: `CatalogSource` y los tipos de la Tarea 11; `t()` de la Tarea 10.

- [ ] **Step 1: Escribir los tests de la lógica pura**

Crear `web/src/lib/senal.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { nivelSenal } from './senal'

// Mismos umbrales que la app de macOS (mobile/lib/domain/models/channel.dart):
// verde <200 ms, naranja 200–800, rojo >800, gris muerto, apagado sin datos.
// Divergir sería que el mismo canal se vea "bien" en un sitio y "regular" en
// el otro.
describe('nivelSenal', () => {
  it.each([
    [null, 0, 'desconocido'],
    [false, 0, 'muerta'],
    [true, 120, 'buena'],
    [true, 199, 'buena'],
    [true, 200, 'media'],
    [true, 800, 'media'],
    [true, 801, 'pobre'],
  ] as const)('vivo=%s latencia=%s → %s', (vivo, latencia, quiero) => {
    expect(nivelSenal(vivo, latencia)).toBe(quiero)
  })
})
```

Crear `web/src/lib/debounce.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { debounce } from './debounce'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('debounce', () => {
  it('solo llama una vez con el último valor', () => {
    const espia = vi.fn()
    const d = debounce(espia, 300)
    d('b'); d('bb'); d('bbc')
    vi.advanceTimersByTime(299)
    expect(espia).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(espia).toHaveBeenCalledExactlyOnceWith('bbc')
  })

  it('cancelar impide la llamada pendiente', () => {
    const espia = vi.fn()
    const d = debounce(espia, 300)
    d('x')
    d.cancelar()
    vi.advanceTimersByTime(1000)
    expect(espia).not.toHaveBeenCalled()
  })
})
```

Crear `web/src/estado/favoritos.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest'
import { get } from 'svelte/store'
import { crearFavoritos } from './favoritos'

beforeEach(() => localStorage.clear())

describe('favoritos', () => {
  it('alterna y persiste', () => {
    const f = crearFavoritos()
    f.alternar('bbc')
    expect(get(f).has('bbc')).toBe(true)

    // Una instancia nueva lee lo guardado: es lo que pasa al recargar.
    expect(get(crearFavoritos()).has('bbc')).toBe(true)

    f.alternar('bbc')
    expect(get(crearFavoritos()).has('bbc')).toBe(false)
  })

  // Un localStorage con basura no puede dejar la app en blanco.
  it('sobrevive a datos corruptos', () => {
    localStorage.setItem('opentv.favoritos', '{no es json')
    expect(get(crearFavoritos()).size).toBe(0)
  })
})
```

- [ ] **Step 2: Correr los tests y verificar que fallan**

Run: `cd web && npm test`
Esperado: FAIL — no existen `./senal`, `./debounce`, `./favoritos`.

- [ ] **Step 3: Implementar la lógica pura**

Crear `web/src/lib/senal.ts`:

```ts
export type NivelSenal = 'desconocido' | 'muerta' | 'buena' | 'media' | 'pobre'

/**
 * Mismos umbrales que la app de macOS. Si se cambian aquí hay que cambiarlos
 * allí: el mismo canal no puede verse "bien" en un cliente y "regular" en el
 * otro.
 */
export function nivelSenal(vivo: boolean | null, latenciaMs: number): NivelSenal {
  if (vivo == null) return 'desconocido'
  if (!vivo) return 'muerta'
  if (latenciaMs < 200) return 'buena'
  if (latenciaMs <= 800) return 'media'
  return 'pobre'
}
```

Crear `web/src/lib/debounce.ts`:

```ts
export interface Debounced<A extends unknown[]> {
  (...args: A): void
  cancelar(): void
}

/** debounce para la búsqueda: sin él, cada tecla es una consulta al catálogo. */
export function debounce<A extends unknown[]>(fn: (...args: A) => void, ms: number): Debounced<A> {
  let temporizador: ReturnType<typeof setTimeout> | undefined

  const envuelto = (...args: A) => {
    if (temporizador) clearTimeout(temporizador)
    temporizador = setTimeout(() => fn(...args), ms)
  }
  envuelto.cancelar = () => {
    if (temporizador) clearTimeout(temporizador)
    temporizador = undefined
  }
  return envuelto as Debounced<A>
}
```

Crear `web/src/estado/favoritos.ts`:

```ts
import { writable, type Writable } from 'svelte/store'

const CLAVE = 'opentv.favoritos'

export interface Favoritos extends Writable<Set<string>> {
  alternar(id: string): void
}

function leer(): Set<string> {
  try {
    const crudo = localStorage.getItem(CLAVE)
    if (!crudo) return new Set()
    const datos = JSON.parse(crudo)
    return Array.isArray(datos) ? new Set(datos.filter((x) => typeof x === 'string')) : new Set()
  } catch {
    // Basura en localStorage o almacenamiento bloqueado. Empezar de cero es
    // molesto; dejar la app en blanco es un fallo.
    return new Set()
  }
}

export function crearFavoritos(): Favoritos {
  const store = writable<Set<string>>(leer())

  store.subscribe((s) => {
    try {
      localStorage.setItem(CLAVE, JSON.stringify([...s]))
    } catch {
      // Ver arriba.
    }
  })

  return {
    ...store,
    alternar(id: string) {
      store.update((s) => {
        const nuevo = new Set(s)
        if (!nuevo.delete(id)) nuevo.add(id)
        return nuevo
      })
    },
  }
}

export const favoritos = crearFavoritos()
```

Crear `web/src/estado/filtros.ts`:

```ts
import { writable } from 'svelte/store'
import type { ConsultaCatalogo } from '../datos/catalogo'

export type ModoVista = 'rejilla' | 'lista'

export interface EstadoFiltros extends ConsultaCatalogo {
  soloFavoritos: boolean
  vista: ModoVista
}

export const filtros = writable<EstadoFiltros>({
  q: '',
  pais: '',
  categoria: '',
  calidad: '',
  mostrarOffline: false,
  soloFavoritos: false,
  vista: 'rejilla',
})
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

Run: `cd web && npm test`
Esperado: PASS.

- [ ] **Step 5: Escribir los componentes**

Crear `web/src/componentes/BarrasSenal.svelte`:

```svelte
<script lang="ts">
  import { nivelSenal, type NivelSenal } from '../lib/senal'
  import { t } from '../i18n'

  let { vivo, latenciaMs }: { vivo: boolean | null; latenciaMs: number } = $props()

  const nivel = $derived(nivelSenal(vivo, latenciaMs))
  const barras = $derived({ buena: 3, media: 2, pobre: 1, muerta: 0, desconocido: 0 }[nivel])
  const etiqueta = $derived(
    nivel === 'desconocido' ? t('senal.sinDatos') : nivel === 'muerta' ? t('senal.muerta') : t('senal.viva'),
  )
</script>

<!-- Un solo container con Semantics: tres barras sueltas serían tres nodos sin
     sentido para un lector de pantalla. -->
<span class="senal" role="img" aria-label={etiqueta} data-nivel={nivel}>
  {#each [1, 2, 3] as n}
    <i class:encendida={n <= barras}></i>
  {/each}
</span>

<style>
  .senal { display: inline-flex; gap: 2px; align-items: flex-end; height: 12px; }
  i { width: 3px; background: var(--graphite-500); border-radius: 1px; }
  i:nth-child(1) { height: 5px; }
  i:nth-child(2) { height: 8px; }
  i:nth-child(3) { height: 12px; }
  /* Ámbar SOLO cuando hay señal viva. Es la regla del sistema de diseño. */
  [data-nivel='buena'] i.encendida { background: var(--amber-500); }
  [data-nivel='media'] i.encendida { background: var(--amber-400); }
  [data-nivel='pobre'] i.encendida { background: var(--signal-error); }
</style>
```

Crear `web/src/componentes/MarcaWeb.svelte`:

```svelte
<script lang="ts">
  import { t } from '../i18n'
  // Solo se pinta cuando el veredicto es un NO explícito. null es "todavía no
  // lo sé" y marcar eso asustaría sin motivo: antes de la primera pasada del
  // health-worker, todo el catálogo es null.
  let { webOk }: { webOk: boolean | null } = $props()
</script>

{#if webOk === false}
  <span class="marca" title={t('canal.soloApp')} aria-label={t('canal.soloApp')}>APP</span>
{/if}

<style>
  .marca {
    font-size: 10px; letter-spacing: .05em; padding: 1px 4px; border-radius: 3px;
    background: var(--graphite-600); color: var(--text-body);
  }
</style>
```

Crear `web/src/componentes/TarjetaCanal.svelte`:

```svelte
<script lang="ts">
  import type { Canal } from '../datos/catalogo'
  import BarrasSenal from './BarrasSenal.svelte'
  import MarcaWeb from './MarcaWeb.svelte'
  import { favoritos } from '../estado/favoritos'
  import { t } from '../i18n'

  let { canal, alAbrir }: { canal: Canal; alAbrir: (c: Canal) => void } = $props()
  const esFavorito = $derived($favoritos.has(canal.id))
</script>

<article class="tarjeta">
  <button class="abrir" onclick={() => alAbrir(canal)} aria-label={canal.nombre}>
    {#if canal.logoUrl}
      <img src={canal.logoUrl} alt="" loading="lazy" />
    {:else}
      <span class="sinlogo" aria-hidden="true">{canal.nombre.slice(0, 2)}</span>
    {/if}
    <span class="nombre">{canal.nombre}</span>
  </button>

  <footer>
    <BarrasSenal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
    {#if canal.pais}<span class="pais">{canal.pais}</span>{/if}
    <MarcaWeb webOk={canal.webOk} />
    <button
      class="favorito"
      class:activo={esFavorito}
      onclick={() => favoritos.alternar(canal.id)}
      aria-pressed={esFavorito}
      aria-label={esFavorito ? t('canal.favorito.quitar') : t('canal.favorito.anadir')}
    >★</button>
  </footer>
</article>

<style>
  .tarjeta { background: var(--surface-card); border-radius: 8px; padding: 8px; display: flex; flex-direction: column; gap: 6px; }
  .abrir { all: unset; cursor: pointer; display: flex; flex-direction: column; gap: 6px; align-items: center; }
  img, .sinlogo { width: 100%; aspect-ratio: 16/9; object-fit: contain; }
  .sinlogo { display: grid; place-items: center; background: var(--surface-sunken); color: var(--text-muted, var(--graphite-300)); }
  .nombre { font-size: 13px; text-align: center; }
  footer { display: flex; align-items: center; gap: 6px; }
  .pais { font-size: 11px; color: var(--graphite-300); }
  .favorito { all: unset; cursor: pointer; margin-left: auto; color: var(--graphite-500); }
  /* Ámbar = activo. Un favorito apagado es neutro. */
  .favorito.activo { color: var(--amber-500); }
</style>
```

Crear `web/src/componentes/RejillaCanales.svelte` con la lista, el scroll infinito (un `IntersectionObserver` sobre un centinela al final que llama `alPedirMas()`), y el modo lista (misma información en una fila). Crear `web/src/componentes/BarraFiltros.svelte` con: campo de búsqueda (`debounce(…, 300)`), selectores de país y categoría alimentados por `paises()`/`categorias()` mostrando `valor (total)`, selector de calidad, casilla "ocultar los que no responden", botón de favoritos, botón de canal al azar y conmutador rejilla/lista. Ambos solo leen los stores y emiten callbacks; nada de fetch dentro de un componente.

- [ ] **Step 6: Cablear `App.svelte`**

`App.svelte` es el único que habla con `CatalogSource`: mantiene `canales`, `total`, `cargando`, `error`, la página actual, y reacciona a `filtros`. Reglas que no se pueden perder:

- Cambiar cualquier filtro **reinicia** `desplazamiento` a 0 y vacía la lista; el scroll infinito solo suma.
- `soloFavoritos` **no pagina**: manda `ids: [...$favoritos]` (con el centinela del conjunto vacío ya resuelto en `HttpCatalog`), porque un favorito puede estar en la página 20.
- La página son 500, el máximo que acepta el gateway.
- El total se lee de `X-Total-Count`, no de `canales.length`.

- [ ] **Step 7: Test de humo de la tarjeta**

Crear `web/src/componentes/TarjetaCanal.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import TarjetaCanal from './TarjetaCanal.svelte'
import type { Canal } from '../datos/catalogo'

const base: Canal = {
  id: 'x', nombre: 'BBC One', logoUrl: '', categoriaId: 'General',
  idioma: 'en', pais: 'GB', vivo: true, latenciaMs: 120, webOk: true,
}

describe('TarjetaCanal', () => {
  it('no marca APP cuando el canal se ve en la web', () => {
    render(TarjetaCanal, { canal: base, alAbrir: () => {} })
    expect(screen.queryByText('APP')).toBeNull()
  })

  it('marca APP solo con un no explícito', () => {
    render(TarjetaCanal, { canal: { ...base, webOk: false }, alAbrir: () => {} })
    expect(screen.getByText('APP')).toBeTruthy()
  })

  // null NO es false: antes de la primera pasada del health-worker todo el
  // catálogo es null y marcarlo entero sería mentir.
  it('no marca APP cuando no se ha comprobado', () => {
    render(TarjetaCanal, { canal: { ...base, webOk: null }, alAbrir: () => {} })
    expect(screen.queryByText('APP')).toBeNull()
  })
})
```

- [ ] **Step 8: Correr los gates**

Run: `cd web && npm test && npm run check && npm run build`
Esperado: verde, y el bundle propio por debajo de 80 KB gzip (`npm run build` lo imprime; anotar el tamaño).

**Tamaño del bundle (rellenar al ejecutar):** _pendiente_

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(web): catalogo — rejilla, filtros, facetas y favoritos

Los umbrales de senal son los MISMOS que los de la app de macOS: el
mismo canal no puede verse 'bien' en un cliente y 'regular' en el otro.
La marca APP solo se pinta con un no explicito; null es 'todavia no lo
se' y antes de la primera pasada del health-worker eso es TODO el
catalogo.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 13: El reproductor — `PlaybackGuard` y "directo primero, proxy si falla"

El bug que este guard arregló en la app de macOS: cualquier error de mpv —incluido un EOF transitorio de HLS en vivo— mataba el vídeo dejando el audio sonando. En el navegador el fallo equivalente es un `<video>` en negro que no dice nada. Se porta 1:1, con temporizadores falsos.

**Files:**
- Create: `web/src/reproductor/{guard.ts,guard.test.ts,plan.ts,plan.test.ts}`
- Create: `web/src/componentes/Reproductor.svelte`
- Modify: `web/src/App.svelte`

**Interfaces:**
- Produces: `class PlaybackGuard`; `planDeReproduccion(entrada): Plan`; `<Reproductor canal destino proxyDisponible alCerrar />`.
- Consumes: `CatalogSource.destino()` y `proxyDisponible()` (Tarea 11), `t()` (Tarea 10).

- [ ] **Step 1: Escribir el test del guard**

Crear `web/src/reproductor/guard.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PlaybackGuard } from './guard'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('PlaybackGuard', () => {
  it('un error antes de la primera reproducción es fatal al instante', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal })
    g.armarTimeoutDeCarga()
    g.alError('manifiesto 403')
    expect(fatal).toHaveBeenCalledOnce()
  })

  it('si nada reproduce en el timeout de carga, es fatal', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()
    vi.advanceTimersByTime(14_999)
    expect(fatal).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(fatal).toHaveBeenCalledOnce()
  })

  // La prueba de reproducción es que la POSICIÓN AVANZA. `playing` es una
  // declaración de intención: se emite antes de decodificar un fotograma, y
  // desarmar el watchdog ahí dejaba la pantalla colgada para siempre en
  // cualquier canal cuyo manifiesto carga pero cuyos segmentos nunca llegan.
  it('la primera posición solo fija la referencia', () => {
    const fatal = vi.fn()
    const confirmado = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, alConfirmar: confirmado, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()

    g.alPosicion(0)
    g.alPosicion(0) // congelado: sigue siendo la referencia
    vi.advanceTimersByTime(15_000)

    expect(confirmado).not.toHaveBeenCalled()
    expect(fatal).toHaveBeenCalledOnce()
  })

  it('una posición que avanza confirma la reproducción y desarma la carga', () => {
    const fatal = vi.fn()
    const confirmado = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, alConfirmar: confirmado, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()

    g.alPosicion(0)
    g.alPosicion(1.2)
    vi.advanceTimersByTime(60_000)

    expect(confirmado).toHaveBeenCalledOnce()
    expect(fatal).not.toHaveBeenCalled()
  })

  // El caso del bug: error transitorio DURANTE la reproducción. Si el vídeo
  // sigue avanzando, no se toca nada.
  it('un error en reproducción se ignora si la posición sigue avanzando', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000, timeoutAtasco: 8_000 })
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1)

    g.alError('bufferStalledError')
    vi.advanceTimersByTime(4_000)
    g.alPosicion(5) // sigue vivo
    vi.advanceTimersByTime(4_000)

    expect(fatal).not.toHaveBeenCalled()
  })

  it('un error en reproducción es fatal si la posición se congela', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000, timeoutAtasco: 8_000 })
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1)

    g.alError('networkError')
    vi.advanceTimersByTime(8_000)

    expect(fatal).toHaveBeenCalledExactlyOnceWith('networkError')
  })

  // Errores repetidos no pueden extender la ventana de vigilancia una y otra
  // vez: eso convertiría el guard en un observador eterno.
  it('un segundo error no re-arma la vigilancia', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000, timeoutAtasco: 8_000 })
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1)

    g.alError('e1')
    vi.advanceTimersByTime(5_000)
    g.alError('e2')
    vi.advanceTimersByTime(3_000)

    expect(fatal).toHaveBeenCalledOnce()
  })

  it('destruir cancela todo', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()
    g.destruir()
    vi.advanceTimersByTime(60_000)
    expect(fatal).not.toHaveBeenCalled()
  })
})
```

- [ ] **Step 2: Escribir el test de la política de intentos**

Crear `web/src/reproductor/plan.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { planDeReproduccion, urlProxy } from './plan'

const URL_HTTPS = 'https://cdn.example/live.m3u8'
const URL_HTTP = 'http://cdn.example/live.m3u8'

describe('planDeReproduccion', () => {
  // Safari reproduce HLS de forma nativa y NO necesita CORS. Por eso ve el
  // 85 % del catálogo y Chrome/Firefox el 67 %: negarle un canal porque
  // web_ok sea falso sería negarle algo que sí puede ver.
  it('en HLS nativo se intenta directo aunque web_ok sea falso', () => {
    const p = planDeReproduccion({ motor: 'nativo', url: URL_HTTPS, webOk: false, proxyDisponible: false })
    expect(p.intentos[0]).toBe(URL_HTTPS)
    expect(p.aviso).toBe('ninguno')
  })

  it('con hls.js y web_ok true se va directo, con el proxy de red', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: true, proxyDisponible: true })
    expect(p.intentos).toEqual([URL_HTTPS, urlProxy(URL_HTTPS)])
  })

  // Sin veredicto se prueba igualmente: "directo primero, proxy si falla".
  it('con hls.js y web_ok null se intenta directo y luego el proxy', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: null, proxyDisponible: true })
    expect(p.intentos).toEqual([URL_HTTPS, urlProxy(URL_HTTPS)])
  })

  // Con un NO explícito, el intento directo son 2-3 segundos tirados: ya
  // sabemos que el navegador lo va a cortar.
  it('con hls.js y web_ok false se va directo al proxy', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: false, proxyDisponible: true })
    expect(p.intentos).toEqual([urlProxy(URL_HTTPS)])
  })

  // El sitio hospedado NO tiene proxy: es estructural, no una opción.
  it('sin proxy y sin veredicto favorable, se avisa en vez de fingir', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: false, proxyDisponible: false })
    expect(p.intentos).toEqual([])
    expect(p.aviso).toBe('solo-app-o-safari')
  })

  it('http desde una página https solo puede ir por el proxy', () => {
    const p = planDeReproduccion({ motor: 'nativo', url: URL_HTTP, webOk: null, proxyDisponible: true })
    expect(p.intentos).toEqual([urlProxy(URL_HTTP)])
  })
})
```

- [ ] **Step 3: Correr los tests y verificar que fallan**

Run: `cd web && npm test`
Esperado: FAIL — no existen `./guard` ni `./plan`.

- [ ] **Step 4: Implementar el guard**

Crear `web/src/reproductor/guard.ts`:

```ts
export interface OpcionesGuard {
  alFallar: (mensaje: string) => void
  /** Se llama UNA vez, con prueba real de reproducción. Es la señal buena
   *  para quitar el indicador de carga. */
  alConfirmar?: () => void
  timeoutCarga?: number
  timeoutAtasco?: number
}

/**
 * Decide cuándo un error del reproductor es realmente fatal.
 *
 * Puerto 1:1 de mobile/lib/presentation/player/playback_guard.dart. El bug que
 * lo originó: cualquier error de mpv —incluido un EOF transitorio de HLS en
 * vivo— mataba el vídeo dejando el audio sonando. hls.js tiene la misma
 * naturaleza: emite bufferStalledError y fragParsingError constantemente en
 * directos que se ven perfectamente.
 *
 * Reglas:
 * - Error antes de la primera reproducción → fatal inmediato.
 * - Nada reproduce dentro de timeoutCarga → fatal.
 * - Error durante la reproducción → solo es fatal si la posición deja de
 *   avanzar durante timeoutAtasco.
 *
 * "Reproducir" significa que la POSICIÓN AVANZA, no que el elemento diga que
 * está reproduciendo.
 */
export class PlaybackGuard {
  private readonly alFallar: (m: string) => void
  private readonly alConfirmar?: () => void
  private readonly timeoutCarga: number
  private readonly timeoutAtasco: number

  private tCarga?: ReturnType<typeof setTimeout>
  private tAtasco?: ReturnType<typeof setTimeout>
  private arrancado = false
  private destruido = false
  private ultimaPosicion = 0
  private avanzoDesdeElError = false
  private posicionReferencia: number | null = null

  constructor(o: OpcionesGuard) {
    this.alFallar = o.alFallar
    this.alConfirmar = o.alConfirmar
    this.timeoutCarga = o.timeoutCarga ?? 15_000
    this.timeoutAtasco = o.timeoutAtasco ?? 8_000
  }

  /** Armar ANTES de asignar la fuente: si la carga se cuelga, el timeout tiene
   *  que saltar igualmente. El watchdog original se armaba después y por eso
   *  nunca saltaba. */
  armarTimeoutDeCarga(): void {
    if (this.tCarga) clearTimeout(this.tCarga)
    this.tCarga = setTimeout(() => {
      if (this.destruido || this.arrancado) return
      this.alFallar('timeout-de-carga')
    }, this.timeoutCarga)
  }

  alPosicion(segundos: number): void {
    if (segundos !== this.ultimaPosicion) {
      this.ultimaPosicion = segundos
      this.avanzoDesdeElError = true
    }
    if (this.arrancado || this.destruido) return

    // La primera muestra solo fija la referencia: una posición que se repite
    // es exactamente lo que se ve cuando el reproductor está atascado.
    if (this.posicionReferencia === null) {
      this.posicionReferencia = segundos
      return
    }
    if (segundos !== this.posicionReferencia) {
      this.arrancado = true
      if (this.tCarga) clearTimeout(this.tCarga)
      this.alConfirmar?.()
    }
  }

  alError(mensaje: string): void {
    if (this.destruido || mensaje === '') return

    if (!this.arrancado) {
      if (this.tCarga) clearTimeout(this.tCarga)
      this.alFallar(mensaje)
      return
    }

    // Ya en reproducción: vigilar en vez de matar. Si ya hay una vigilancia en
    // curso, no re-armar — errores repetidos no deben extender la ventana.
    if (this.tAtasco) return
    this.avanzoDesdeElError = false
    this.tAtasco = setTimeout(() => {
      this.tAtasco = undefined
      if (this.destruido) return
      if (!this.avanzoDesdeElError) this.alFallar(mensaje)
    }, this.timeoutAtasco)
  }

  destruir(): void {
    this.destruido = true
    if (this.tCarga) clearTimeout(this.tCarga)
    if (this.tAtasco) clearTimeout(this.tAtasco)
  }
}
```

- [ ] **Step 5: Implementar la política de intentos**

Crear `web/src/reproductor/plan.ts`:

```ts
export type Motor = 'nativo' | 'hlsjs'

export interface EntradaPlan {
  motor: Motor
  url: string
  webOk: boolean | null
  proxyDisponible: boolean
}

export interface Plan {
  /** URLs a probar, en orden. Vacío = no hay forma de reproducirlo aquí. */
  intentos: string[]
  aviso: 'ninguno' | 'solo-app-o-safari'
}

export const RUTA_PROXY = '/proxy/hls?u='

export function urlProxy(url: string): string {
  return RUTA_PROXY + encodeURIComponent(url)
}

/**
 * Decide cómo intentar reproducir. La regla de la spec es "directo primero,
 * proxy si falla", con dos matices que salen del censo:
 *
 * - Safari reproduce HLS de forma NATIVA y no necesita CORS, así que ve el
 *   85 % del catálogo frente al 67 % de Chrome/Firefox. web_ok es el veredicto
 *   estricto (hls.js); negarle a Safari un canal por eso sería negarle algo
 *   que sí puede ver.
 * - Con un web_ok FALSO explícito, el intento directo son dos o tres segundos
 *   tirados: ya sabemos que el navegador va a cortar el fetch.
 */
export function planDeReproduccion(e: EntradaPlan): Plan {
  const esHttps = e.url.toLowerCase().startsWith('https://')
  const paginaSegura = typeof location !== 'undefined' && location.protocol === 'https:'

  // Contenido mixto: una página https no carga medios http, ni en Safari.
  if (!esHttps && paginaSegura) {
    return e.proxyDisponible
      ? { intentos: [urlProxy(e.url)], aviso: 'ninguno' }
      : { intentos: [], aviso: 'solo-app-o-safari' }
  }

  if (e.motor === 'nativo') {
    const intentos = [e.url]
    if (e.proxyDisponible) intentos.push(urlProxy(e.url))
    return { intentos, aviso: 'ninguno' }
  }

  if (e.webOk === false) {
    return e.proxyDisponible
      ? { intentos: [urlProxy(e.url)], aviso: 'ninguno' }
      : { intentos: [], aviso: 'solo-app-o-safari' }
  }

  const intentos = [e.url]
  if (e.proxyDisponible) intentos.push(urlProxy(e.url))
  return { intentos, aviso: 'ninguno' }
}

/** Safari y iOS reproducen HLS sin librería; el resto necesita hls.js. */
export function motorDelNavegador(video: HTMLVideoElement): Motor {
  return video.canPlayType('application/vnd.apple.mpegurl') !== '' ? 'nativo' : 'hlsjs'
}
```

Nota sobre el test `http desde una página https solo puede ir por el proxy`: en jsdom `location.protocol` es `http:` por defecto, así que ese caso hay que forzarlo. En el test, envolverlo con `vi.stubGlobal('location', { protocol: 'https:' })` y restaurarlo después.

- [ ] **Step 6: Correr los tests y verificar que pasan**

Run: `cd web && npm test -- reproductor`
Esperado: PASS en los ocho del guard y los seis del plan.

- [ ] **Step 7: Escribir el componente**

Crear `web/src/componentes/Reproductor.svelte`. Puntos que no son negociables:

- **El guard se arma ANTES de asignar la fuente.** Es literalmente el bug que cerró el pendiente "watchdog se arma tarde" de la auditoría.
- La posición se alimenta desde `timeupdate` (`video.currentTime`), nunca desde `playing`.
- `hls.js` se importa con `await import('hls.js')` **dentro** del camino `hlsjs`, para que Safari no lo descargue nunca y el bundle inicial siga por debajo del presupuesto.
- Los intentos se recorren en orden: si el primero falla de forma fatal, se prueba el siguiente y solo se enseña el error cuando se agotan.
- Con `aviso === 'solo-app-o-safari'` no se intenta nada: se enseña `t('canal.soloApp')`.
- Teclado: `espacio` (pausa), `Esc` (cerrar), `f` (pantalla completa), `m` (silencio), `←`/`→` (canal anterior/siguiente).
- Botón de AirPlay **solo en Safari**, con detección de característica y sin ninguna lógica de sesión:

```ts
// AirPlay de Safari: es una llamada y un evento, sin capa nativa. Si el
// navegador no lo expone, el botón no existe. La app de macOS es la que hace
// sesiones de emisión de verdad; aquí solo se abre el selector del sistema.
const soportaAirplay = 'WebKitPlaybackTargetAvailabilityEvent' in window
function abrirSelectorAirplay() {
  // @ts-expect-error API solo de WebKit
  video.webkitShowPlaybackTargetPicker()
}
```

y el `<video>` lleva `x-webkit-airplay="allow"`.

- [ ] **Step 8: Correr los gates y commitear**

```bash
cd web && npm test && npm run check && npm run build && cd ..
go test ./internal/ui/
git add -A
git commit -m "feat(web): reproductor con PlaybackGuard portado y politica directo-o-proxy

El guard es un puerto 1:1 del de la app de macOS, con temporizadores
falsos: hls.js emite errores recuperables en directos que se ven
perfectamente, igual que mpv. La prueba de reproduccion es que la
POSICION AVANZA; 'playing' se emite antes de decodificar un fotograma.
Safari intenta directo aunque web_ok sea falso: reproduce HLS nativo y
no necesita CORS.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 14: Estados — sincronizando, gateway caído, sin red, frescura

Tres mensajes que la app de macOS ya distingue y que costaron una tarde de diagnóstico en agosto. No se pueden colapsar en "algo ha fallado".

**Files:**
- Create: `web/src/estado/{salud.ts,salud.test.ts}`
- Create: `web/src/componentes/{Sincronizando.svelte,MensajeError.svelte,Frescura.svelte}`
- Modify: `web/src/App.svelte`

**Interfaces:**
- Produces: `type EstadoSalud`, `consultarSalud(base?)`, `clasificarError(e): ClaseError`.

- [ ] **Step 1: Escribir el test**

Crear `web/src/estado/salud.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { clasificarError, consultarSalud } from './salud'

afterEach(() => vi.unstubAllGlobals())

describe('clasificarError', () => {
  // El síntoma en la UI es idéntico y el problema no lo es: uno se arregla
  // abriendo Open TV, el otro mirando el wifi. Confundirlos costó una tarde.
  it('distingue gateway caído de falta de red', () => {
    expect(clasificarError(new Error('gateway inalcanzable'))).toBe('gateway')
    expect(clasificarError(new Error('sin red'))).toBe('red')
    expect(clasificarError(new Error('respuesta 500'))).toBe('servidor')
  })
})

describe('consultarSalud', () => {
  it('lee el catálogo como listo cuando hay sync', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify({ status: 'ok', db: 'ok', last_sync: '2026-08-25T10:00:00Z', web_ui: true, proxy_enabled: true }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )))

    const s = await consultarSalud('')
    expect(s.sincronizando).toBe(false)
    expect(s.proxyDisponible).toBe(true)
  })

  // last_sync null = primer arranque: el catálogo se está descargando. Sin
  // esto, la primera pantalla es una rejilla vacía que parece rota.
  it('sin sync todavía, está sincronizando', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify({ status: 'ok', db: 'ok', last_sync: null, web_ui: true, proxy_enabled: true }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )))

    expect((await consultarSalud('')).sincronizando).toBe(true)
  })
})
```

- [ ] **Step 2: Correr el test y verificar que falla**

Run: `cd web && npm test -- salud`
Esperado: FAIL — no existe `./salud`.

- [ ] **Step 3: Implementar**

Crear `web/src/estado/salud.ts`:

```ts
export type ClaseError = 'gateway' | 'red' | 'servidor'

export interface EstadoSalud {
  sincronizando: boolean
  proxyDisponible: boolean
  ultimoSync: Date | null
  version: string
}

/**
 * clasificarError separa tres problemas que se ven igual en pantalla y se
 * arreglan de forma distinta: Open TV cerrado, sin internet, o el servidor
 * contestando mal. El 2026-08-07 el diagnóstico de un "gateway caído" costó
 * una tarde porque el síntoma parecía un bug del cliente.
 */
export function clasificarError(e: unknown): ClaseError {
  const m = e instanceof Error ? e.message : String(e)
  if (m.includes('sin red')) return 'red'
  if (m.includes('gateway')) return 'gateway'
  return 'servidor'
}

export async function consultarSalud(base = ''): Promise<EstadoSalud> {
  const resp = await fetch(`${base}/health`)
  const cuerpo = (await resp.json()) as {
    last_sync?: string | null
    proxy_enabled?: boolean
    version?: string
  }
  return {
    sincronizando: !cuerpo.last_sync,
    proxyDisponible: cuerpo.proxy_enabled === true,
    ultimoSync: cuerpo.last_sync ? new Date(cuerpo.last_sync) : null,
    version: cuerpo.version ?? 'dev',
  }
}
```

- [ ] **Step 4: Escribir los componentes**

`Sincronizando.svelte`: título `t('estado.sincronizando')`, detalle `t('estado.sincronizandoDetalle')`, y un reintento cada 2 s hasta que `consultarSalud()` diga que ya no. **Sin barra de progreso falsa**: el gateway no publica progreso y fingirlo sería exactamente lo que la spec prohíbe.

`MensajeError.svelte`: recibe `clase: ClaseError` y pinta `t('estado.gatewayCaido')`, `t('estado.sinRed')` o un mensaje de servidor. Nunca los mezcla.

`Frescura.svelte`: en el binario local, `t('frescura.envivo')`. Con un snapshot (P3), `t('frescura.comprobado', { horas })`. La distinción sale de `CatalogSource.frescura()`, así que el componente no sabe en cuál está.

Añadir el pie con `t('pie.fuente')`, `t('pie.postura')` y el conmutador de idioma en `App.svelte`.

- [ ] **Step 5: Correr los gates y commitear**

```bash
cd web && npm test && npm run check && npm run build && cd ..
git add -A
git commit -m "feat(web): estados de primer arranque, error y frescura

Gateway caido, sin red y error de servidor son tres problemas que se
ven igual y se arreglan distinto; el 2026-08-07 confundirlos costo una
tarde. La pagina de sincronizacion no tiene barra de progreso: el
gateway no publica progreso y fingirlo seria mentir.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 15: Playwright contra un `open-tv` de verdad

Nada de mocks: se levanta el binario real, sirviendo un catálogo local con un stream HLS generado con ffmpeg. Es la única forma de que el gate "los canales se reproducen" signifique algo.

**Files:**
- Create: `web/playwright.config.ts`, `web/tests/e2e/global-setup.ts`, `web/tests/e2e/servidor-fixtures.ts`
- Create: `web/tests/e2e/{catalogo.spec.ts,reproduccion.spec.ts}`
- Create: `web/tests/fixtures/catalogo.m3u` (generado por el setup)

**Interfaces:**
- Consumes: el binario `open-tv` compilado y el cliente construido en `internal/ui/dist`.

- [ ] **Step 1: Escribir el servidor de fixtures**

Crear `web/tests/e2e/servidor-fixtures.ts`:

```ts
import { createReadStream, existsSync, statSync } from 'node:fs'
import { createServer, type Server } from 'node:http'
import { extname, join, normalize } from 'node:path'

const TIPOS: Record<string, string> = {
  '.m3u8': 'application/vnd.apple.mpegurl',
  '.m3u': 'application/vnd.apple.mpegurl',
  '.ts': 'video/mp2t',
}

/**
 * Sirve las fixtures en dos rutas deliberadamente distintas:
 *
 * - /cors/…  manda Access-Control-Allow-Origin: * → web_ok = true → el cliente
 *   reproduce DIRECTO.
 * - /sincors/… no manda nada → web_ok = false → el cliente tiene que pasar por
 *   el proxy de loopback.
 *
 * Con un solo caso, la mitad del camino que P0 construye no se probaría nunca.
 */
export function servidorFixtures(raiz: string, puerto: number): Promise<Server> {
  const srv = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://localhost')
    const conCors = url.pathname.startsWith('/cors/')
    const relativa = url.pathname.replace(/^\/(cors|sincors)\//, '')
    const fichero = join(raiz, normalize(relativa).replace(/^(\.\.[/\\])+/, ''))

    if (!existsSync(fichero) || !statSync(fichero).isFile()) {
      res.writeHead(404).end('no está')
      return
    }
    if (conCors) res.setHeader('Access-Control-Allow-Origin', '*')
    res.setHeader('Content-Type', TIPOS[extname(fichero)] ?? 'application/octet-stream')
    createReadStream(fichero).pipe(res)
  })

  return new Promise((resolve) => srv.listen(puerto, '127.0.0.1', () => resolve(srv)))
}
```

- [ ] **Step 2: Escribir el global setup**

Crear `web/tests/e2e/global-setup.ts`:

```ts
import { execFileSync, spawn, type ChildProcess } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import type { Server } from 'node:http'
import { servidorFixtures } from './servidor-fixtures'

const RAIZ = resolve(__dirname, '../../..')       // raíz del repo
const FIXTURES = resolve(__dirname, '../fixtures')
const PUERTO_FIXTURES = 8099
export const PUERTO_APP = 8090

let servidor: Server | undefined
let binario: ChildProcess | undefined

/** Genera un HLS real de 10 s con ffmpeg. H.264 + AAC: lo que decodifica
 *  cualquier navegador. Se cachea entre ejecuciones. */
function generarHLS(): void {
  const salida = join(FIXTURES, 'hls')
  if (existsSync(join(salida, 'canal.m3u8'))) return
  mkdirSync(salida, { recursive: true })

  execFileSync('ffmpeg', [
    '-y',
    '-f', 'lavfi', '-i', 'testsrc=size=640x360:rate=25:duration=10',
    '-f', 'lavfi', '-i', 'sine=frequency=440:duration=10',
    '-c:v', 'libx264', '-profile:v', 'main', '-pix_fmt', 'yuv420p', '-g', '50',
    '-c:a', 'aac', '-b:a', '64k',
    '-f', 'hls', '-hls_time', '2', '-hls_list_size', '0', '-hls_playlist_type', 'vod',
    join(salida, 'canal.m3u8'),
  ], { stdio: 'inherit' })
}

function escribirM3U(): void {
  const base = `http://127.0.0.1:${PUERTO_FIXTURES}`
  writeFileSync(join(FIXTURES, 'catalogo.m3u'), [
    '#EXTM3U',
    `#EXTINF:-1 tvg-id="ConCors.xx" tvg-logo="" group-title="News",Canal Con CORS (1080p)`,
    `${base}/cors/hls/canal.m3u8`,
    `#EXTINF:-1 tvg-id="SinCors.xx" tvg-logo="" group-title="Movies",Canal Sin CORS (720p)`,
    `${base}/sincors/hls/canal.m3u8`,
    '',
  ].join('\n'))
}

async function esperarSync(url: string): Promise<void> {
  for (let i = 0; i < 120; i++) {
    try {
      const r = await fetch(`${url}/health`)
      const c = (await r.json()) as { last_sync?: string | null }
      if (c.last_sync) return
    } catch {
      // Todavía no escucha.
    }
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error('open-tv no completó el primer sync en 60 s')
}

export default async function globalSetup(): Promise<void> {
  generarHLS()
  escribirM3U()

  // El cliente y el binario se construyen aquí: el e2e prueba el ARTEFACTO,
  // no el servidor de desarrollo de Vite.
  execFileSync('npm', ['run', 'build'], { cwd: join(RAIZ, 'web'), stdio: 'inherit' })
  execFileSync('go', ['build', '-o', 'open-tv', './cmd/open-tv'], { cwd: RAIZ, stdio: 'inherit' })

  servidor = await servidorFixtures(FIXTURES, PUERTO_FIXTURES)

  const datos = mkdtempSync(join(tmpdir(), 'opentv-e2e-'))
  binario = spawn(join(RAIZ, 'open-tv'), ['serve', '--no-browser'], {
    cwd: RAIZ,
    env: {
      ...process.env,
      DB_PATH: join(datos, 'e2e.db'),
      LISTEN_ADDR: `127.0.0.1:${PUERTO_APP}`,
      IPTV_ORG_URL: `http://127.0.0.1:${PUERTO_FIXTURES}/cors/catalogo.m3u`,
      HEALTH_INTERVAL: '10s',
    },
    stdio: 'inherit',
  })

  await esperarSync(`http://127.0.0.1:${PUERTO_APP}`)
  // Un margen para que la primera pasada de salud clasifique web_ok: sin ella
  // los dos canales salen con veredicto null y el caso del proxy no se prueba.
  await new Promise((r) => setTimeout(r, 3000))

  ;(globalThis as Record<string, unknown>).__cerrarE2E = async () => {
    binario?.kill('SIGTERM')
    servidor?.close()
  }
}

export async function globalTeardown(): Promise<void> {
  const cerrar = (globalThis as Record<string, unknown>).__cerrarE2E as (() => Promise<void>) | undefined
  await cerrar?.()
}
```

- [ ] **Step 3: Configurar Playwright**

Crear `web/playwright.config.ts`:

```ts
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  globalSetup: './tests/e2e/global-setup.ts',
  globalTeardown: './tests/e2e/global-setup.ts',
  timeout: 60_000,
  use: {
    baseURL: 'http://127.0.0.1:8090',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
})
```

- [ ] **Step 4: Escribir el spec del catálogo (los tres motores)**

Crear `web/tests/e2e/catalogo.spec.ts`:

```ts
import { expect, test } from '@playwright/test'

test('la rejilla se llena y el filtro reduce', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Canal Con CORS (1080p)')).toBeVisible()
  await expect(page.getByText('Canal Sin CORS (720p)')).toBeVisible()

  await page.getByPlaceholder(/buscar|search/i).fill('Sin CORS')
  await expect(page.getByText('Canal Sin CORS (720p)')).toBeVisible()
  await expect(page.getByText('Canal Con CORS (1080p)')).toHaveCount(0)
})

// La marca APP es el veredicto web_ok llegando hasta el píxel. Si el clasificador,
// la columna, la agregación o el mapeo del cliente se rompen, se ve aquí.
test('el canal sin CORS se marca y el otro no', async ({ page }) => {
  await page.goto('/')
  const sinCors = page.locator('article', { hasText: 'Canal Sin CORS' })
  const conCors = page.locator('article', { hasText: 'Canal Con CORS' })
  await expect(sinCors.getByText('APP')).toBeVisible()
  await expect(conCors.getByText('APP')).toHaveCount(0)
})

test('el idioma cambia y se recuerda', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'English' }).click()
  await expect(page.getByPlaceholder(/search/i)).toBeVisible()
  await page.reload()
  await expect(page.getByPlaceholder(/search/i)).toBeVisible()
})

test('un favorito sobrevive a la recarga', async ({ page }) => {
  await page.goto('/')
  const tarjeta = page.locator('article', { hasText: 'Canal Con CORS' })
  await tarjeta.getByRole('button', { name: /favorit/i }).click()
  await page.reload()
  await expect(tarjeta.getByRole('button', { name: /favorit/i })).toHaveAttribute('aria-pressed', 'true')
})
```

- [ ] **Step 5: Escribir el spec de reproducción**

Crear `web/tests/e2e/reproduccion.spec.ts`:

```ts
import { expect, test } from '@playwright/test'

// El WebKit de Playwright sobre Linux NO es Safari: su soporte de H.264/HLS
// depende de plugins de GStreamer que no están garantizados en ubuntu-latest.
// Declarar verde un motor que no decodifica sería una puerta verde que no
// prueba nada. La reproducción en WebKit es un gate MANUAL en Safari de verdad
// (ver la Tarea 17 del plan).
test.skip(
  ({ browserName }) => browserName === 'webkit' && !!process.env.CI,
  'WebKit de Linux no garantiza H.264; el gate de Safari es manual',
)

async function abrir(page: import('@playwright/test').Page, nombre: string) {
  await page.goto('/')
  await page.locator('article', { hasText: nombre }).getByRole('button', { name: nombre }).click()
}

test('un canal con CORS reproduce directo y los frames avanzan', async ({ page }) => {
  await abrir(page, 'Canal Con CORS')
  const video = page.locator('video')
  await expect(video).toBeVisible()

  // Que la POSICIÓN AVANCE es la única prueba de reproducción. `playing` se
  // emite antes de decodificar un fotograma; es lo que enseñó el guard.
  await expect
    .poll(async () => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000 })
    .toBeGreaterThan(0.5)
})

test('un canal sin CORS reproduce por el proxy de loopback', async ({ page }) => {
  const porElProxy: string[] = []
  page.on('request', (r) => {
    if (r.url().includes('/proxy/hls')) porElProxy.push(r.url())
  })

  await abrir(page, 'Canal Sin CORS')
  const video = page.locator('video')
  await expect
    .poll(async () => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000 })
    .toBeGreaterThan(0.5)

  expect(porElProxy.length, 'el canal sin CORS tenía que pasar por el proxy').toBeGreaterThan(0)
})

// Verificar el FALLO, no la salud: se mata el origen y se comprueba que el
// guard declara fatal con el mensaje correcto en vez de dejar un negro mudo.
test('si el stream muere, el guard lo dice', async ({ page }) => {
  await page.route('**/hls/**', (ruta) => ruta.abort())
  await abrir(page, 'Canal Con CORS')

  await expect(page.getByText(/no llegó a reproducir|never started playing/i)).toBeVisible({ timeout: 25_000 })
})
```

- [ ] **Step 6: Correr el e2e en local, los tres motores**

```bash
cd web
npx playwright install
npm run test:e2e
```

Esperado: los cuatro del catálogo × 3 motores + los tres de reproducción × 3 motores en verde. En el Mac, WebKit **sí** corre reproducción (el skip solo aplica en CI).

Si el spec del proxy falla porque `web_ok` sale `null`, el health-check aún no ha pasado: subir el margen del setup o bajar `HEALTH_INTERVAL`.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "test(e2e): Playwright contra un open-tv real con HLS de ffmpeg

Sin mocks: se construye el cliente, se compila el binario, se genera un
HLS H.264/AAC y se sirve por dos rutas —una con CORS y otra sin— para
que el camino directo Y el del proxy se prueben los dos. La prueba de
reproduccion es que currentTime avanza. Un test mata el origen y exige
que el guard lo diga: verificar el fallo, no la salud.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 16: CI

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Añadir el job del cliente web**

Añadir al final de `.github/workflows/ci.yml`, al mismo nivel que `gateway` y `mobile`:

```yaml
  web:
    name: Cliente web
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
          cache-dependency-path: web/package-lock.json

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache-dependency-path: go.sum

      # ffmpeg genera el fixture HLS del e2e. Ya viene en la imagen de
      # ubuntu-latest, pero fijarlo evita que un cambio de imagen rompa el
      # e2e con un error que no dice nada.
      - name: ffmpeg
        run: ffmpeg -version | head -1

      - name: npm ci
        working-directory: web
        run: npm ci

      - name: svelte-check
        working-directory: web
        run: npm run check

      - name: vitest
        working-directory: web
        run: npm test

      - name: build del cliente
        working-directory: web
        run: npm run build

      # El embed es lo que convierte cliente y binario en un solo artefacto:
      # si el build del cliente no aterriza donde Go lo espera, falla aquí.
      - name: go build con el cliente embebido
        run: go build ./... && go test ./internal/ui/...

      - name: navegadores de Playwright
        working-directory: web
        run: npx playwright install --with-deps chromium firefox webkit

      - name: e2e
        working-directory: web
        run: npm run test:e2e

      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: playwright-report
          path: web/playwright-report/
          retention-days: 7
```

- [ ] **Step 2: Comprobar que el job de `gateway` sigue valiendo sin cliente construido**

El job `gateway` compila **sin** haber ejecutado `npm run build`, así que `internal/ui/dist` solo contiene el `.gitkeep`. Eso es correcto y está previsto: `go:embed all:dist` casa con el `.gitkeep`, `ui.Handler()` devuelve `false`, y los tests de `internal/ui` hacen `t.Skip`. El binario sirve la API sin cliente, que es un modo degradado legítimo.

Verificarlo antes de empujar:

```bash
rm -rf internal/ui/dist && mkdir -p internal/ui/dist && touch internal/ui/dist/.gitkeep
go build ./... && go test ./internal/ui/... -v
```

Esperado: compila, y los tres tests salen SKIP con "no hay cliente construido".

- [ ] **Step 3: Empujar la rama y ver CI en verde**

```bash
git log --format='%ae' origin/main..HEAD | sort -u   # debe decir solo gdberysan@gmail.com
git push -u origin <rama>
gh run watch
```

Esperado: los tres jobs (`gateway`, `mobile`, `web`) en verde. **`mobile` con cero diffs:** `git diff --stat origin/main..HEAD -- mobile/` vacío.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: job del cliente web con Playwright en tres motores

Incluye 'go build con el cliente embebido': si el build de Vite no
aterriza donde go:embed lo espera, el artefacto unico deja de serlo y
falla aqui y no al hacer la release.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Tarea 17: Click-through manual — el gate de P0

CI no prueba que esto se pueda usar. Este es el gate de la fase: **`go run ./cmd/open-tv` y los canales se reproducen en Chrome, Firefox y Safari en el Mac del autor.** Sobre el catálogo REAL de iptv-org, no sobre las fixtures.

- [ ] **Step 1: Arranque en frío, como un desconocido**

```bash
launchctl bootout gui/$(id -u)/dev.korven.opentv.gateway 2>/dev/null || true
rm -rf ~/Library/Application\ Support/Korven\ Open\ TV
cd web && npm run build && cd ..
go build -o open-tv ./cmd/open-tv
./open-tv
```

Comprobar, en orden: se abre el navegador solo · sale la página de sincronización · la rejilla se llena en segundos · a los pocos minutos aparecen las barras de señal y las marcas APP.

- [ ] **Step 2: Los tres navegadores, con canales reales**

En Safari, Chrome y Firefox contra `http://127.0.0.1:8080`: reproducir **tres canales distintos** en cada uno, incluyendo al menos uno marcado `APP` (que debe llegar por el proxy) y uno sin marcar. Anotar cuáles.

- [ ] **Step 3: Móvil en la misma red** — se salta en P0: el binario escucha en loopback a propósito. El gate del teléfono es P3, con `opentv.korven.dev`.

- [ ] **Step 4: Los caminos de fallo**

- Matar el binario con el navegador abierto → tiene que decir "No se pudo contactar con Open TV", **no** "sin red".
- Desconectar el wifi con el binario vivo → "Sin conexión a internet".
- Segunda instancia: con uno corriendo, `./open-tv` otra vez → abre una pestaña y **no** arranca un segundo catálogo.
- `LISTEN_ADDR=0.0.0.0:8080 ./open-tv --no-browser` y luego `curl -s -o /dev/null -w '%{http_code}\n' 'http://127.0.0.1:8080/proxy/hls?u=x'` → **404**: fuera de loopback el proxy no existe.

- [ ] **Step 5: Los dos idiomas** — recorrer la interfaz entera en español y en inglés. Ninguna cadena en el idioma equivocado, ningún `{parametro}` sin sustituir.

- [ ] **Step 6: Anotar los hallazgos aquí**

#### Hallazgos del click-through (rellenar al ejecutar)

| # | Dónde | Qué pasa | Bloquea el gate | Commit del arreglo |
|---|---|---|---|---|
| | | | | |

Los que bloqueen el gate se arreglan en esta misma fase, con su commit. Los que no, se anotan y se llevan a P2 (pulido del primer arranque).

---

## Tarea 18: Memoria y cierre

- [ ] **Step 1: Actualizar la memoria del proyecto**

En `~/.claude/projects/-Users-usuario-Dev-open-tv/memory/`:

- `web_product_launch.md`: P0 completado, con la fecha, el commit del gate y lo que quedó pendiente. Siguiente paso: escribir el plan de P1 (scrub, licencia, goreleaser, repo público) con las mismas convenciones.
- `project_iptv_status.md`: añadir el bloque de P0 — el módulo vive en la raíz (`github.com/gdberysan/open-tv`), el contrato JSON son **15** claves, existe `streams.web_ok`, el checker hace GET directo para HLS, el proxy de loopback existe y **solo** en loopback, y el CORS `*` ya no está.
- `dev_arranque_gateway.md`: el LaunchAgent apunta a `open-tv serve --no-browser` desde la raíz y `DB_PATH` va a `.devdata/`; sin `DB_PATH` el binario usa `~/Library/Application Support/Korven Open TV`.

- [ ] **Step 2: Corregir las desviaciones que este plan encontró**

Dos afirmaciones de la spec resultaron falsas al mirar el código (ver "Desviaciones"). Añadir al final de `docs/superpowers/specs/2026-08-22-open-tv-web-product-design.md` una **nota de enmienda fechada** —no reescribir el cuerpo: la spec es registro histórico— diciendo que `airplay_ok` no estaba persistido y que `web_ok` es el veredicto estricto de hls.js, no el de Safari.

- [ ] **Step 3: Cerrar la rama**

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./...
cd web && npm run check && npm test && npm run build && cd ..
cd mobile && flutter analyze && flutter test && cd ..
git diff --stat origin/main..HEAD -- mobile/    # vacío
git log --format='%ae' origin/main..HEAD | sort -u
```

Con todo en verde, mergear a `main` y anotar aquí el commit del merge.

- [ ] **Step 4: Lo que P0 deja abierto**

- Fixtures dorados `HttpCatalog` vs `StaticCatalog` → P3, cuando existan las dos.
- Notarización de Apple, `install.sh`, goreleaser, historial reescrito, repo público → P1.
- Reproducción verificada en WebKit **automáticamente** → no se hará: el gate de Safari es manual y con Safari de verdad.
- La app Flutter sigue congelada. No entra en v1.0.

---

## Self-review

**Cobertura de la spec.** §2.1 disposición del repo → Tarea 1. §3.1 stack, tokens, presupuesto de bundle → Tarea 10 (con el tamaño medido en su Step 8). §3.2 catálogo, tarjeta, paginación, búsqueda, facetas, offline, aleatorio, favoritos → Tarea 12; reproductor, guard, mensaje de `web_ok` falso, teclado, AirPlay de Safari → Tarea 13; estados y frescura → Tarea 14; i18n → Tarea 10. §3.3 `CatalogSource` → Tarea 11 (fixtures dorados diferidos a P3, desviación 4). §4.1 embed, fallback SPA, cache, fuera CORS, `--no-browser`, abrir navegador → Tareas 3, 9 y 2. §4.2 `web_ok` → Tareas 4, 5 y 6. §4.3 proxy → Tareas 7 y 8. §4.4 `snapshot` → **no es P0** (§10 lo pone en P3). §4.5 directorio de datos, puerto, URL impresa → Tarea 2. §4.6 `/health` → Tarea 9. §6.3 segunda instancia → Tarea 2 (desviación 7). §9 gates automáticos → Tareas 15 y 16; manuales → Tarea 17. §10 gate de P0 → Tarea 17 Steps 1–5.

**Escaneo de placeholders.** Quedan tres huecos, y los tres son **mediciones que solo existen al ejecutar**, no contenido sin escribir: el coste de la pasada de salud (Tarea 5 Step 6), el tamaño del bundle (Tarea 12 Step 8) y la tabla de hallazgos del click-through (Tarea 17 Step 6). Están marcados "rellenar al ejecutar". Todo el código, todos los comandos y todo el texto de UI en los dos idiomas están escritos. La única pieza descrita en prosa y no en código son `RejillaCanales.svelte`, `BarraFiltros.svelte` y los tres componentes de estado (Tareas 12 Step 5, 14 Step 4): son composición de Svelte sin lógica propia —la lógica que sí tienen (`nivelSenal`, `debounce`, `favoritos`, `clasificarError`) va con su test— y sus invariantes están enumeradas.

**Consistencia de tipos.** `domain.WebSupport`/`WebUnknown`/`WebNo`/`WebOK` y `OrigenWeb` se definen en la Tarea 4 y se usan igual en las 5, 6 y 8. `StreamResult.Web` (Tarea 5) alimenta `ports.StreamHealth.Web` (Tarea 6). `Channel.WebOK *bool` (Tarea 6) es la clave `WebOK` que lee `CanalCable` (Tarea 11) y se convierte en `Canal.webOk`, que consumen `MarcaWeb` (Tarea 12) y `planDeReproduccion` (Tarea 13). `api.Options{ProxyActivo, Version}` sustituye en la Tarea 9 al `proxyActivo bool` que la Tarea 3 introdujo, y las tres llamadas se actualizan ahí — es el único cambio de firma que ocurre dos veces, y está declarado en el bloque *Interfaces* de las dos tareas. `RutaProxy` (Go, Tarea 8) y `RUTA_PROXY` (TS, Tarea 13) son la misma cadena `/proxy/hls?u=` en los dos lados; el e2e de la Tarea 15 la comprueba de punta a punta al exigir que el canal sin CORS pase por ella. `proxy.NewHandler(prefijo, permitirDestinosPrivados)` lleva dos parámetros desde su definición y así se llama en los siete tests y en el router.
