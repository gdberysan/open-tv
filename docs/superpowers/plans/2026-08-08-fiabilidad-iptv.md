# Plan de Fiabilidad — Ecosistema IPTV

> **ESTADO: COMPLETADO** (2026-08-08). Las 19 tareas ejecutadas en la rama
> `fiabilidad`, 13 commits, CI en verde. Ver el resumen de desviaciones al
> final del documento.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convertir un MVP que funciona cuando todo va bien en un sistema que no miente sobre su catálogo, no se cuelga nunca, y avisa cuando está roto.

**Architecture:** Cinco fases independientes y ordenadas por riesgo eliminado por hora invertida. La Fase 0 pone la red de seguridad (remoto git, CI, README) para que todo lo demás sea reversible y verificable. La Fase 1 arregla la correctitud del catálogo en el gateway Go (histéresis de salud, poda de fantasmas, contrato de wire). La Fase 2 elimina los caminos por los que la app Flutter se cuelga para siempre. La Fase 3 hace el sistema observable. La Fase 4 limpia el código muerto para que la señal de tests sea honesta. La Fase 5 quita los cuellos de botella de SQLite ya medidos.

Cada fase deja software funcionando y testeado por sí sola. Se pueden ejecutar en sesiones separadas y en este orden; no hay dependencias hacia atrás salvo las anotadas.

**Tech Stack:** Go 1.25 (chi v5, modernc.org/sqlite, log/slog, stdlib testing) · Flutter 3.x (Riverpod, media_kit, http, shared_preferences, flutter_test) · GitHub Actions.

## Global Constraints

Copiadas de `PROMPT_MAESTRO.md` §10. Se aplican a **todas** las tareas de este plan:

- `go build ./...` pasa sin warnings al terminar cada tarea.
- `go vet ./...` cero errores.
- `gofmt -l .` no lista ningún archivo.
- `flutter analyze` cero errores.
- Tests en la misma tarea que el código que testean (TDD: test primero, verlo fallar, implementar, verlo pasar).
- Errores envueltos con contexto: `fmt.Errorf("paquete.Operacion: %w", err)`.
- Cero `_` descartando errores en producción.
- Goroutines con context propagado y timeout explícito; `defer cancel()` inmediato.
- `slog.Error/Info/Debug` — nunca `fmt.Println`.
- `domain/` sin imports externos (solo `time` y primitivos).
- Un commit por tarea, mensaje con scope `gateway:` o `mobile:` o `ci:`.

**Idioma:** comentarios y mensajes de commit en español, como el resto del repo.

**Directorio de trabajo del gateway:** todos los comandos `go` se ejecutan desde `~/Dev/ip-tv/gateway`.
**Directorio de trabajo de la app:** todos los comandos `flutter` se ejecutan desde `~/Dev/ip-tv/mobile`.

---

## File Structure

**Se crean:**

| Archivo | Responsabilidad |
|---|---|
| `.github/workflows/ci.yml` | Gate automático: build, vet, gofmt, test (Go) + analyze, test (Flutter) |
| `README.md` | Cómo arrancar el stack, tabla de variables de entorno, expectativa de primer sync |
| `gateway/internal/api/handlers/health_handler.go` | `/health` real: ping a DB + edad del último sync |
| `gateway/internal/api/handlers/health_handler_test.go` | Tests del handler de salud |
| `mobile/lib/data/api_config.dart` | Configuración compartida: base URL y timeout HTTP, hoy duplicados en dos repos |
| `mobile/test/data/api_timeout_test.dart` | Tests de timeout de la capa de datos |
| `mobile/test/presentation/player_screen_test.dart` | Tests de widget de `PlayerScreen` (hoy sin ningún test) |

**Se modifican:**

| Archivo | Cambio |
|---|---|
| `gateway/internal/adapters/db/db.go` | PRAGMAs vía DSN; `alterMigrations` añade `fail_count` y `last_seen_at` |
| `gateway/internal/adapters/db/schema.sql` | Columnas `fail_count`, `last_seen_at`; borrar tablas muertas |
| `gateway/internal/adapters/db/stream_repository.go` | Histéresis en `MarkDead`; reset en `MarkAlive`; `MarkBatch` transaccional |
| `gateway/internal/adapters/db/channel_repository.go` | `AliveOnly` excluye canales sin streams; `ORDER BY` con desempate; `last_seen_at`; `DeleteStale` |
| `gateway/internal/ports/stream_repository.go` | Añadir `MarkBatch` |
| `gateway/internal/ports/channel_repository.go` | Añadir `DeleteStale` |
| `gateway/internal/services/syncer.go` | Suelo de cordura; poda de obsoletos; exponer `LastSuccess()` |
| `gateway/internal/adapters/validator/checker.go` | `DisableKeepAlives`; User-Agent |
| `gateway/internal/adapters/validator/worker.go` | Escrituras en lote vía `MarkBatch` |
| `gateway/internal/api/handlers/channel_handler.go` | Logger inyectado; loguear la causa de cada 5xx |
| `gateway/internal/api/handlers/epg_handler.go` | Logger inyectado; loguear la causa de cada 5xx |
| `gateway/internal/api/router.go` | Cablear logger a handlers; montar `/health` real |
| `gateway/internal/domain/channel.go` | Tags JSON explícitos que congelan el contrato actual |
| `gateway/internal/domain/stream.go` | Tags JSON explícitos |
| `gateway/cmd/server/main.go` | Extraer a `run()` testeable; `WaitGroup` en el apagado; puente `log` → `slog` |
| `mobile/lib/data/repositories/channel_repository.dart` | Timeout por petición; usar `ApiConfig` |
| `mobile/lib/data/repositories/epg_repository.dart` | Timeout por petición; usar `ApiConfig` |
| `mobile/lib/presentation/screens/player_screen.dart` | Armar watchdog antes del fetch; token de generación en el retry |
| `mobile/lib/presentation/providers/channel_provider.dart` | Token de generación en `loadMore`; `==`/`hashCode` en el estado |
| `mobile/lib/domain/models/channel_filter.dart` | `==`/`hashCode` |
| `mobile/lib/presentation/screens/guide_screen.dart` | Rama `AsyncError` explícita |

**Se borran (Fase 4):** `gateway/internal/adapters/failover/`, `gateway/internal/adapters/healthcheck/`, `gateway/internal/adapters/cache/`, `gateway/internal/domain/mirror.go`, `gateway/internal/domain/sport_event.go`, `gateway/internal/domain/provider.go`, `gateway/internal/domain/category.go`, `gateway/internal/ports/cache_port.go`, `scripts/scaffold.sh`.

---

# FASE 0 — Red de seguridad

*Sin esto, todo lo demás se hace sin marcha atrás. Es la fase más barata y la de mayor retorno.*

---

### Task 1: Remoto git y push inicial

**Files:** ninguno (operación sobre el repositorio).

**Interfaces:**
- Consumes: nada.
- Produces: un remoto `origin` alcanzable, para que las tareas siguientes puedan hacer push.

**Contexto:** el repositorio tiene 11 commits, una sola rama `main`, y **cero remotos**. Existe únicamente en este Mac. Esta tarea no tiene test automatizado; su verificación es que `git ls-remote origin` responda.

- [x] **Step 1: Limpiar el archivo suelto en la raíz**

Hay un screenshot sin trackear en la raíz del repositorio. Moverlo fuera o borrarlo:

```bash
cd ~/Dev/ip-tv
rm -f Screenshot*.png
git status --short
```

Esperado: salida vacía (árbol limpio).

- [x] **Step 2: Crear el repositorio remoto**

Requiere que el usuario tenga `gh` autenticado. Si `gh auth status` falla, el usuario debe ejecutar `gh auth login` él mismo (es interactivo).

```bash
gh auth status
gh repo create ip-tv --private --source=. --remote=origin
```

Si prefiere otro proveedor, basta con `git remote add origin <url>`.

- [x] **Step 3: Push y verificación**

```bash
git push -u origin main
git ls-remote origin
```

Esperado: `git ls-remote` lista el `refs/heads/main` con el mismo SHA que `git rev-parse HEAD`.

- [x] **Step 4: Confirmar que el árbol sigue limpio**

```bash
git status --short
```

Esperado: salida vacía. No hay commit en esta tarea; el entregable es el remoto.

---

### Task 2: Gate de CI en GitHub Actions

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: el remoto de la Task 1.
- Produces: un workflow llamado `CI` con dos jobs, `gateway` y `mobile`, que corren en cada push y cada pull request contra `main`.

**Contexto:** hoy no existe ningún gate automático. La suite Go corre en ~10 s y la de Flutter en ~5 s, así que el workflow completo cabe holgadamente en un minuto. `gofmt -l` lista actualmente 5 archivos, por lo que el primer run fallará a propósito — se arregla en el Step 4.

- [x] **Step 1: Escribir el workflow**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  gateway:
    name: Gateway (Go)
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: gateway
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: gateway/go.mod
          cache-dependency-path: gateway/go.sum

      - name: gofmt
        run: |
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            echo "Archivos sin formatear:"
            echo "$unformatted"
            exit 1
          fi

      - name: go vet
        run: go vet ./...

      - name: go build
        run: go build ./...

      - name: go test
        run: go test -race -count=1 ./...

  mobile:
    name: Mobile (Flutter)
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: mobile
    steps:
      - uses: actions/checkout@v4

      - uses: subosito/flutter-action@v2
        with:
          channel: stable
          cache: true

      - name: flutter pub get
        run: flutter pub get

      - name: flutter analyze
        run: flutter analyze

      - name: flutter test
        run: flutter test
```

- [x] **Step 2: Verificar localmente que los mismos comandos pasan (menos gofmt)**

```bash
cd ~/Dev/ip-tv/gateway && go vet ./... && go build ./... && go test -race -count=1 ./...
```

Esperado: PASS en todos los paquetes. `-race` es nuevo respecto a lo que se corría a mano; si aparece una carrera, **pararse y arreglarla antes de seguir** — es un hallazgo real.

```bash
cd ~/Dev/ip-tv/mobile && flutter analyze && flutter test
```

Esperado: "No issues found!" y 57 tests en verde.

- [x] **Step 3: Verificar que gofmt falla ahora mismo**

```bash
cd ~/Dev/ip-tv/gateway && gofmt -l .
```

Esperado (falla intencionada, confirma que el gate sirve para algo):

```
internal/adapters/epg/parser_test.go
internal/api/middleware/logger.go
internal/api/middleware/recover.go
internal/domain/mirror.go
internal/domain/sport_event.go
```

- [x] **Step 4: Formatear y verificar que ya pasa**

```bash
cd ~/Dev/ip-tv/gateway && gofmt -w . && gofmt -l .
```

Esperado: salida vacía.

- [x] **Step 5: Commit y push**

```bash
cd ~/Dev/ip-tv
git add .github/workflows/ci.yml gateway/
git commit -m "ci: gate de build, vet, gofmt y tests para gateway y app"
git push
```

- [x] **Step 6: Confirmar que el workflow pasa en remoto**

```bash
gh run watch
```

Esperado: ambos jobs en verde. Si `mobile` falla por versión de Flutter, fijar `flutter-version` en el `with:` a la versión que devuelve `flutter --version` en local.

---

### Task 3: README con el arranque real y las variables de entorno

**Files:**
- Create: `README.md`
- Delete: `scripts/scaffold.sh`

**Interfaces:**
- Consumes: nada.
- Produces: documentación; ninguna interfaz de código.

**Contexto:** no hay README en la raíz. El de `mobile/` es la plantilla intacta de Flutter. Siete variables de entorno controlan el gateway y ninguna está documentada fuera de comentarios en el código; `EPG_URL` en particular es opt-in y sin ella toda la guía de programación queda vacía en silencio. `scripts/scaffold.sh` está obsoleto: crea `pipeline/` y `providers/xtreamcodes/`, ambos eliminados.

- [x] **Step 1: Escribir el README**

```markdown
# Ecosistema IPTV

Agregador y reproductor de televisión abierta (FTA) desde fuentes públicas
tipo IPTV-org. Gateway en Go + app Flutter para macOS.

Sin canales premium, sin VPN, sin geo-bypass, sin credenciales de terceros.

## Arrancar el stack

Hacen falta dos procesos. **El gateway no se arranca solo**: no hay launchd,
ni docker-compose, ni supervisor. Si no está corriendo, la app falla con
`Connection refused` en `127.0.0.1:8080`.

### 1. Gateway

```bash
cd gateway
go run ./cmd/server
```

Escucha en `127.0.0.1:8080`. En el primer arranque descarga ~13 000 canales
de IPTV-org antes de que la app muestre nada; tarda unos segundos y lo
registra como `Sync completado`.

### 2. App

```bash
cd mobile
flutter run -d macos
```

Apunta a `http://127.0.0.1:8080` por defecto.

### Comprobar que el gateway está vivo

```bash
curl -s localhost:8080/health
lsof -iTCP:8080 -sTCP:LISTEN -n -P
```

## Variables de entorno del gateway

| Variable | Default | Para qué sirve |
|---|---|---|
| `LISTEN_ADDR` | `127.0.0.1:8080` | Dirección de escucha. **No exponer a la red**: la API no tiene autenticación. |
| `DB_PATH` | `iptv.db` | Ruta del SQLite. Es **relativa al directorio de trabajo**. |
| `IPTV_ORG_URL` | `https://iptv-org.github.io/iptv/index.m3u` | Origen del catálogo M3U. |
| `SYNC_INTERVAL` | `12h` | Cadencia del re-sync del catálogo. |
| `HEALTH_INTERVAL` | `60m` | Cadencia del health-check de streams. |
| `EPG_URL` | *(ninguno)* | **Opt-in.** Fuente XMLTV (`.xml` o `.xml.gz`). **Sin ella el worker de EPG no arranca y la guía sale vacía.** |
| `EPG_INTERVAL` | `12h` | Cadencia de refresco del EPG. |

### Sobre `EPG_URL`

No existe una URL XMLTV pública canónica que cubra el catálogo de IPTV-org,
por eso es opt-in. Para que la guía muestre algo, los `channel id` del XMLTV
deben coincidir con los `tvg-id` del M3U — por ejemplo los que genera el
proyecto [iptv-org/epg](https://github.com/iptv-org/epg).

```bash
EPG_URL=https://ejemplo/guia.xml.gz go run ./cmd/server
```

## Configuración de la app

La URL del gateway se fija en tiempo de compilación:

```bash
flutter run -d macos --dart-define=GATEWAY_URL=http://192.168.1.50:8080
```

## Desarrollo

```bash
cd gateway && go test ./... && go vet ./... && gofmt -l .
cd mobile  && flutter test && flutter analyze
```

CI corre exactamente eso en cada push (`.github/workflows/ci.yml`).

## Documentación

- `PROMPT_MAESTRO.md` — contrato y roadmap del proyecto
- `docs/adr/` — decisiones de arquitectura
- `docs/superpowers/plans/` — planes de implementación
```

- [x] **Step 2: Verificar que las instrucciones funcionan de verdad**

Desde un directorio limpio, seguir literalmente el README:

```bash
cd ~/Dev/ip-tv/gateway && (go run ./cmd/server &) && sleep 8 && curl -s localhost:8080/health && echo
```

Esperado: `{"status":"ok"}`. Parar el proceso después con `pkill -f "cmd/server"`.

- [x] **Step 3: Borrar el script obsoleto**

`scripts/scaffold.sh` crea `pipeline/` y `providers/xtreamcodes/`; el primero se eliminó en el commit `0cfd541` y el segundo nunca existió en el código actual. Seguirlo produciría un árbol que no compila.

```bash
cd ~/Dev/ip-tv && git rm scripts/scaffold.sh
```

- [x] **Step 4: Reconciliar `PROMPT_MAESTRO.md` con la realidad**

Tres secciones describen un estado que ya no es cierto y desorientan a quien lea el contrato:

- **§8 "Estado actual"** (líneas 210-218) sigue listando como pendientes (`✗`) el *EPG Worker wired en main.go*, el *`StreamRepository` SQLite* y el *Stream health validator*. Los tres están hechos (commits `4fbb821`, `1ec19f0`). Moverlos a la tabla "Completado ✓" y dejar en pendiente solo *iOS / Android targets*.
- **§9 "Quick wins disponibles HOY"** (líneas 305-310) afirma que cablear el worker EPG son "10 líneas". Costó una capa de persistencia entera (~700 líneas). Borrar ese punto; los otros dos (`/channels/categories`, `/channels/countries`) siguen siendo válidos.
- **§3 "Estructura de directorios"** (líneas 58-89) omite `internal/services/`, `internal/adapters/validator/` y `internal/adapters/healthcheck/`, e incluye `failover/` y `cache/`. Actualizar el árbol al real; si la Task 16 ya corrió, reflejar también las bajas.

Añadir además una línea en §8 anotando que la Fase 6 (EPG) está completa **pero apagada por defecto** porque `EPG_URL` es opt-in.

- [x] **Step 5: Commit**

```bash
git add README.md PROMPT_MAESTRO.md
git commit -m "docs: README con arranque del stack, variables de entorno y roadmap reconciliado"
git push
```

---

# FASE 1 — Correctitud del catálogo

*El problema que más se nota usando la app: se ocultan canales que funcionan y se listan canales que no.*

---

### Task 4: PRAGMAs vía DSN, no vía el pool

**Files:**
- Modify: `gateway/internal/adapters/db/db.go:18-59`
- Test: `gateway/internal/adapters/db/db_test.go` (crear)

**Interfaces:**
- Consumes: nada.
- Produces: `db.Open(path string) (*sql.DB, error)` — misma firma, pero ahora los PRAGMAs valen para **cualquier** conexión del pool. Las Tasks 7 y 19 dependen de esto: la primera necesita que el `ON DELETE CASCADE` se aplique de verdad, la segunda va a subir `MaxOpenConns` por encima de 1.

**Contexto:** `applyPragmas` hace `db.Exec("PRAGMA ...")`, que configura **solo la conexión que el pool entregue en ese momento**. Funciona hoy por accidente: `SetMaxOpenConns(1)` mantiene una única conexión viva para siempre. Si esa conexión se cae y se re-marca, `foreign_keys` vuelve a su default, que es **OFF**, y nada lo avisa. Falta además `busy_timeout`.

- [x] **Step 1: Escribir el test que falla**

Crear `gateway/internal/adapters/db/db_test.go`:

```go
package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
)

// Los PRAGMAs son por conexión. Si se aplican con db.Exec sobre el pool solo
// valen para la conexión que el pool entregue en ese momento; cualquier
// conexión nueva arranca con foreign_keys = OFF y las cascadas dejan de
// aplicarse en silencio.
func TestOpenAplicaPragmasEnTodasLasConexiones(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	sqlDB.SetMaxOpenConns(4)

	ctx := context.Background()
	conns := make([]*sql.Conn, 0, 4)
	for i := 0; i < 4; i++ {
		c, err := sqlDB.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn(%d): %v", i, err)
		}
		conns = append(conns, c)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	for i, c := range conns {
		var fk int
		if err := c.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatalf("conexión %d: PRAGMA foreign_keys: %v", i, err)
		}
		if fk != 1 {
			t.Errorf("conexión %d: foreign_keys = %d, quiero 1", i, fk)
		}

		var busy int
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
			t.Fatalf("conexión %d: PRAGMA busy_timeout: %v", i, err)
		}
		if busy < 5000 {
			t.Errorf("conexión %d: busy_timeout = %d, quiero >= 5000", i, busy)
		}
	}
}
```

- [x] **Step 2: Correr el test y verlo fallar**

Run: `go test ./internal/adapters/db/ -run TestOpenAplicaPragmasEnTodasLasConexiones -v`

Esperado: FAIL. Las conexiones 2, 3 y 4 reportan `foreign_keys = 0`, y todas reportan `busy_timeout = 0`.

- [x] **Step 3: Mover los PRAGMAs al DSN**

En `gateway/internal/adapters/db/db.go`, sustituir el cuerpo de `Open` y eliminar `applyPragmas`:

```go
// Open abre o crea la base de datos SQLite, migra el esquema y siembra los
// providers por defecto. Listo para usar tras retornar.
//
// Los PRAGMAs van en el DSN, no en un Exec posterior: son estado POR CONEXIÓN
// y el driver los reaplica en cada conexión que abre el pool. Con db.Exec solo
// se configuraría la conexión que el pool entregue en ese momento, y cualquier
// conexión nueva arrancaría con foreign_keys en su default (OFF).
func Open(path string) (*sql.DB, error) {
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=cache_size(-32000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}

	// WAL permite lecturas concurrentes, pero SQLite serializa escrituras
	// internamente. Una sola conexión abierta evita errores SQLITE_BUSY en MVP.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := seedProviders(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
```

Borrar la función `applyPragmas` completa (líneas 47-59 del original) y la llamada a `seedUserAgents` junto con su función: la tabla `user_agents` no la lee nadie y se re-siembra en cada arranque. Si `strings` deja de usarse en el archivo, quitar el import.

- [x] **Step 4: Correr el test y verlo pasar**

Run: `go test ./internal/adapters/db/ -run TestOpenAplicaPragmasEnTodasLasConexiones -v`
Esperado: PASS.

- [x] **Step 5: Correr toda la suite**

Run: `go test -race -count=1 ./...`
Esperado: PASS en todos los paquetes. Los tests que usaban `seedUserAgents` implícitamente no existen, pero si alguno consulta `user_agents`, borrar esa aserción.

- [x] **Step 6: Commit**

```bash
git add gateway/internal/adapters/db/
git commit -m "gateway: PRAGMAs en el DSN para que valgan en toda conexión del pool"
```

---

### Task 5: Histéresis en el health-check — no matar un stream por un solo fallo

**Files:**
- Modify: `gateway/internal/adapters/db/schema.sql:62-72`
- Modify: `gateway/internal/adapters/db/db.go` (función `alterMigrations`)
- Modify: `gateway/internal/adapters/db/stream_repository.go:148-168`
- Test: `gateway/internal/adapters/db/stream_repository_test.go`

**Interfaces:**
- Consumes: `db.Open` de la Task 4.
- Produces: `db.DeadFailThreshold` (constante `int64 = 3`). `MarkDead(ctx, streamID)` mantiene su firma pero ahora incrementa un contador y solo marca muerto al alcanzar el umbral. `MarkAlive(ctx, streamID, latencyMs)` mantiene su firma y resetea el contador a 0.

**Contexto:** hoy un único HEAD fallido pone `is_alive = 0` de forma permanente. Como el filtro `AliveOnly` está activo por defecto, el canal desaparece de la lista durante una hora entera hasta la siguiente pasada. Medido sobre la DB real: **4 148 de 13 859 canales (30%) están ocultos ahora mismo**, y al menos uno de ellos (`https://dgwfm675921lp.cloudfront.net/playlist.m3u8`) responde HTTP 200 al probarlo a mano.

- [x] **Step 1: Escribir los tests que fallan**

Añadir al final de `gateway/internal/adapters/db/stream_repository_test.go`:

```go
// Un fallo aislado (blip de red, 503 momentáneo) no debe ocultar un canal
// durante una hora entera. Solo N fallos consecutivos lo dan por muerto.
func TestMarkDeadRequiereFallosConsecutivos(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")

	if err := stRepo.Save(ctx, makeStream("st-1", "ch-1", "http://a/1.m3u8")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := stRepo.MarkAlive(ctx, "st-1", 120); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}

	isAlive := func() bool {
		t.Helper()
		streams, err := stRepo.FindByChannelID(ctx, "ch-1")
		if err != nil {
			t.Fatalf("FindByChannelID: %v", err)
		}
		if len(streams) != 1 {
			t.Fatalf("quiero 1 stream, tengo %d", len(streams))
		}
		return streams[0].IsAlive
	}

	for i := int64(1); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
			t.Fatalf("MarkDead #%d: %v", i, err)
		}
		if !isAlive() {
			t.Fatalf("tras %d fallo(s) el stream ya está muerto; el umbral es %d", i, db.DeadFailThreshold)
		}
	}

	if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
		t.Fatalf("MarkDead final: %v", err)
	}
	if isAlive() {
		t.Errorf("tras %d fallos consecutivos el stream debería estar muerto", db.DeadFailThreshold)
	}
}

// Un chequeo exitoso borra el historial de fallos: dos fallos hoy y uno
// mañana no deben sumar tres.
func TestMarkAliveReseteaElContadorDeFallos(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")

	if err := stRepo.Save(ctx, makeStream("st-1", "ch-1", "http://a/1.m3u8")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	for i := int64(1); i < db.DeadFailThreshold; i++ {
		if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
			t.Fatalf("MarkDead #%d: %v", i, err)
		}
	}
	if err := stRepo.MarkAlive(ctx, "st-1", 90); err != nil {
		t.Fatalf("MarkAlive: %v", err)
	}
	if err := stRepo.MarkDead(ctx, "st-1"); err != nil {
		t.Fatalf("MarkDead tras reset: %v", err)
	}

	streams, err := stRepo.FindByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	if !streams[0].IsAlive {
		t.Errorf("MarkAlive debe resetear el contador; un solo fallo posterior no puede matarlo")
	}
}
```

- [x] **Step 2: Correr los tests y verlos fallar**

Run: `go test ./internal/adapters/db/ -run "TestMarkDead|TestMarkAliveResetea" -v`
Esperado: FAIL con `undefined: db.DeadFailThreshold`.

- [x] **Step 3: Añadir la columna al esquema**

En `gateway/internal/adapters/db/schema.sql`, en la tabla `streams`, añadir la columna tras `is_alive`:

```sql
CREATE TABLE IF NOT EXISTS streams (
    id           TEXT    PRIMARY KEY,
    channel_id   TEXT    NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    url          TEXT    NOT NULL,
    protocol     TEXT    NOT NULL CHECK (protocol IN ('HLS','DASH','RTMP')),
    latency_ms   INTEGER,
    is_alive     INTEGER NOT NULL DEFAULT 0 CHECK (is_alive IN (0,1)),
    fail_count   INTEGER NOT NULL DEFAULT 0,
    last_checked INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);
```

En `gateway/internal/adapters/db/db.go`, añadir el ALTER idempotente para las DBs ya existentes:

```go
func alterMigrations(db *sql.DB) error {
	alters := []string{
		"ALTER TABLE channels ADD COLUMN tvg_id TEXT",
		"ALTER TABLE streams ADD COLUMN fail_count INTEGER NOT NULL DEFAULT 0",
	}
	// ... resto igual
}
```

- [x] **Step 4: Implementar la histéresis**

En `gateway/internal/adapters/db/stream_repository.go`, añadir la constante junto a `streamColumns` y reescribir los dos métodos:

```go
// DeadFailThreshold es el número de chequeos fallidos consecutivos necesarios
// para dar un stream por muerto. Un solo HEAD fallido no basta: un blip de red
// ocultaría el canal hasta la siguiente pasada del worker, una hora después.
const DeadFailThreshold int64 = 3

func (r *SQLiteStreamRepository) MarkAlive(ctx context.Context, streamID string, latencyMs int64) error {
	now := time.Now().Unix()
	if _, err := r.db.ExecContext(ctx,
		`UPDATE streams
		 SET is_alive = 1, fail_count = 0, latency_ms = ?, last_checked = ?, updated_at = ?
		 WHERE id = ?`,
		latencyMs, now, now, streamID,
	); err != nil {
		return fmt.Errorf("db.Stream.MarkAlive (id=%s): %w", streamID, err)
	}
	return nil
}

// MarkDead cuenta el fallo y solo apaga is_alive al alcanzar DeadFailThreshold.
// El incremento y la comparación van en la misma sentencia para que no haya
// lectura-modificación-escritura ni carrera entre pasadas.
func (r *SQLiteStreamRepository) MarkDead(ctx context.Context, streamID string) error {
	now := time.Now().Unix()
	if _, err := r.db.ExecContext(ctx,
		`UPDATE streams
		 SET fail_count   = fail_count + 1,
		     is_alive     = CASE WHEN fail_count + 1 >= ? THEN 0 ELSE is_alive END,
		     last_checked = ?,
		     updated_at   = ?
		 WHERE id = ?`,
		DeadFailThreshold, now, now, streamID,
	); err != nil {
		return fmt.Errorf("db.Stream.MarkDead (id=%s): %w", streamID, err)
	}
	return nil
}
```

- [x] **Step 5: Correr los tests y verlos pasar**

Run: `go test ./internal/adapters/db/ -run "TestMarkDead|TestMarkAliveResetea" -v`
Esperado: PASS en ambos.

- [x] **Step 6: Correr toda la suite**

Run: `go test -race -count=1 ./...`
Esperado: PASS.

- [x] **Step 7: Resetear el estado envenenado de la DB de desarrollo**

La DB local tiene 4 148 streams marcados muertos por la regla vieja. Darles una oportunidad limpia:

```bash
cd ~/Dev/ip-tv/gateway
sqlite3 iptv.db "UPDATE streams SET is_alive = 0, fail_count = 0, last_checked = NULL;"
sqlite3 iptv.db "SELECT COUNT(*) FROM streams WHERE last_checked IS NOT NULL;"
```

Esperado: `0`. Con `last_checked` en NULL los canales vuelven a ser visibles hasta que el worker los evalúe de verdad tres veces.

- [x] **Step 8: Commit**

```bash
git add gateway/internal/adapters/db/
git commit -m "gateway: histéresis de 3 fallos antes de dar un stream por muerto"
```

---

### Task 6: `AliveOnly` deja de mostrar canales sin ningún stream

**Files:**
- Modify: `gateway/internal/adapters/db/channel_repository.go:178-187`
- Test: `gateway/internal/adapters/db/channel_repository_test.go`

**Interfaces:**
- Consumes: nada de tareas anteriores.
- Produces: cambio de comportamiento en `FindFiltered` con `AliveOnly = true`. Ningún cambio de firma.

**Contexto:** 1 157 canales tienen cero filas en `streams`. La cláusula actual los deja pasar explícitamente (`NOT EXISTS (SELECT 1 FROM streams ...)`), así que aparecen en la lista y devuelven 404 al pulsarlos. La intención original de esa rama era no vaciar la app antes de la primera pasada del health-worker, pero eso ya lo cubre la rama `last_checked IS NULL`: un canal **con** streams sin chequear sigue visible. Un canal **sin** streams es injugable por definición.

- [x] **Step 1: Escribir el test que falla**

Añadir a `gateway/internal/adapters/db/channel_repository_test.go`:

```go
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
```

- [x] **Step 2: Correr los tests y verlos fallar**

Run: `go test ./internal/adapters/db/ -run TestFindFilteredAliveOnly -v`
Esperado: `TestFindFilteredAliveOnlyOcultaCanalesSinStreams` FAIL (lista los 2 canales); el segundo ya pasa y debe seguir pasando.

- [x] **Step 3: Ajustar la cláusula**

En `gateway/internal/adapters/db/channel_repository.go`, dentro de `FindFiltered`, sustituir el bloque `if f.AliveOnly`:

```go
	if f.AliveOnly {
		// Visible si tiene al menos un stream vivo o aún sin chequear. Los
		// canales SIN NINGÚN stream quedan fuera: son injugables (el handler
		// /channels/stream devuelve 404) y aparecen cuando el proveedor
		// renombra un canal y deja huérfana la fila anterior.
		where = append(where, `EXISTS (
			SELECT 1 FROM streams s
			WHERE s.channel_id = channels.id
			  AND (s.is_alive = 1 OR s.last_checked IS NULL)
		)`)
	}
```

- [x] **Step 4: Correr los tests y verlos pasar**

Run: `go test ./internal/adapters/db/ -run TestFindFilteredAliveOnly -v`
Esperado: PASS en ambos.

- [x] **Step 5: Correr toda la suite**

Run: `go test -race -count=1 ./...`
Esperado: PASS. Si algún test de handlers asumía que un canal sin streams se listaba, actualizarlo: el comportamiento nuevo es el correcto.

- [x] **Step 6: Commit**

```bash
git add gateway/internal/adapters/db/
git commit -m "gateway: AliveOnly oculta canales sin ningún stream (injugables)"
```

---

### Task 7: Suelo de cordura en el sync y poda de canales obsoletos

**Files:**
- Modify: `gateway/internal/adapters/db/schema.sql` (tabla `channels`)
- Modify: `gateway/internal/adapters/db/db.go` (`alterMigrations`)
- Modify: `gateway/internal/adapters/db/channel_repository.go` (upsert de `SaveBatch` y `Save`, nuevo `DeleteStale`)
- Modify: `gateway/internal/ports/channel_repository.go`
- Modify: `gateway/internal/services/syncer.go:118-151`
- Test: `gateway/internal/adapters/db/channel_repository_test.go`, `gateway/internal/services/syncer_test.go`

**Interfaces:**
- Consumes: `foreign_keys(ON)` fiable de la Task 4 — la poda se apoya en `ON DELETE CASCADE` de `streams` hacia `channels`.
- Produces:
  - `ChannelRepository.DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error)`, que borra los canales de ese proveedor cuyo `last_seen_at` sea anterior a `before` y devuelve cuántos borró. `SaveBatch` y `Save` ahora sellan `last_seen_at = now` en cada upsert.
  - `services.ErrCatalogoSospechoso` (variable de error) y la constante `services.minRatioCatalogo = 0.5`. `SyncOnce` devuelve ese error, sin escribir nada, cuando el proveedor entrega menos de la mitad de los canales del último sync exitoso.

**Contexto:** el ID de canal es `provider + "-" + Name`. Cualquier renombrado aguas arriba crea una fila nueva y deja la vieja para siempre, porque `SaveBatch` solo hace upsert y nada borra nunca. El catálogo crece de forma monótona y el health-worker sigue pagando por chequear cadáveres.

**⚠️ Por qué el suelo de cordura va en esta misma tarea y no después:** hoy, si iptv-org devuelve un 200 con una página HTML de error, un portal cautivo, o un cuerpo truncado en un EOF limpio, el parser M3U produce cero canales, `SaveBatch(nil)` retorna `nil` antes de tiempo, y `SyncOnce` reporta **éxito** — el log dice alegremente `Sync completado canales=0` y el backoff se resetea de 30 s a 12 h. Añadir la poda sin un suelo de cordura convierte ese fallo silencioso en **el borrado del catálogo entero**. Las dos mitades tienen que entrar juntas.

- [x] **Step 1: Escribir el test de repositorio que falla**

Añadir a `gateway/internal/adapters/db/channel_repository_test.go`:

```go
// El ID de canal se deriva del nombre, así que un renombrado aguas arriba deja
// huérfana la fila anterior. DeleteStale la barre tras un sync exitoso.
func TestDeleteStaleBorraCanalesNoVistosEnElUltimoSync(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	seedChannel(t, chRepo, "viejo")
	if err := stRepo.Save(ctx, makeStream("st-viejo", "viejo", "http://a/v.m3u8")); err != nil {
		t.Fatalf("Save stream: %v", err)
	}

	// Frontera del sync: todo lo sellado antes de este instante es obsoleto.
	time.Sleep(1100 * time.Millisecond) // last_seen_at tiene resolución de segundos
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
```

- [x] **Step 2: Correr el test y verlo fallar**

Run: `go test ./internal/adapters/db/ -run TestDeleteStale -v`
Esperado: FAIL con `chRepo.DeleteStale undefined`.

- [x] **Step 3: Añadir la columna**

En `schema.sql`, tabla `channels`, tras `updated_at`:

```sql
    updated_at    INTEGER NOT NULL,
    last_seen_at  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_channels_last_seen ON channels(provider_id, last_seen_at);
```

En `db.go`, `alterMigrations`:

```go
	alters := []string{
		"ALTER TABLE channels ADD COLUMN tvg_id TEXT",
		"ALTER TABLE streams ADD COLUMN fail_count INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE channels ADD COLUMN last_seen_at INTEGER NOT NULL DEFAULT 0",
	}
```

- [x] **Step 4: Sellar `last_seen_at` en el upsert e implementar `DeleteStale`**

En `channel_repository.go`, localizar la constante del SQL de upsert (la usan `Save` y `SaveBatch`) y añadir la columna tanto en el INSERT como en el DO UPDATE SET. El valor es el mismo `now` que ya se pasa para `updated_at`, así que se añade un placeholder más al final de los argumentos de cada `Exec`. Después añadir:

```go
// DeleteStale borra los canales del proveedor que no aparecieron en el último
// sync exitoso. El ID se deriva del nombre, así que un renombrado aguas arriba
// deja una fila huérfana que nunca se jugaría: /channels/stream da 404 porque
// el proveedor ya no la conoce. Los streams caen por ON DELETE CASCADE.
func (r *SQLiteChannelRepository) DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		"DELETE FROM channels WHERE provider_id = ? AND last_seen_at < ?",
		providerID, before.Unix())
	if err != nil {
		return 0, fmt.Errorf("db.DeleteStale (provider=%s): %w", providerID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("db.DeleteStale (RowsAffected): %w", err)
	}
	return n, nil
}
```

En `gateway/internal/ports/channel_repository.go`, añadir a la interfaz:

```go
	// DeleteStale borra los canales del proveedor cuyo last_seen_at sea
	// anterior a before. Devuelve cuántos borró.
	DeleteStale(ctx context.Context, providerID string, before time.Time) (int64, error)
```

Añadir el import de `time` si falta. Actualizar cualquier fake de `ChannelRepository` en los tests para que implemente el método nuevo.

- [x] **Step 5: Correr el test y verlo pasar**

Run: `go test ./internal/adapters/db/ -run TestDeleteStale -v`
Esperado: PASS.

- [x] **Step 6: Escribir el test del syncer que falla**

Primero, extender el fake que ya existe en `gateway/internal/services/syncer_test.go:54-57` para que registre las llamadas:

```go
type fakeChannelRepo struct {
	mu           sync.Mutex
	batches      [][]domain.Channel
	deleteStales []deleteStaleCall
}

type deleteStaleCall struct {
	providerID string
	before     time.Time
}

func (f *fakeChannelRepo) DeleteStale(_ context.Context, providerID string, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteStales = append(f.deleteStales, deleteStaleCall{providerID, before})
	return 0, nil
}

func (f *fakeChannelRepo) staleCalls() []deleteStaleCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]deleteStaleCall(nil), f.deleteStales...)
}
```

Y añadir los tests:

```go
// El ID de canal se deriva del nombre: un renombrado aguas arriba deja una
// fila huérfana que ya no se puede reproducir. Cada sync exitoso la barre.
func TestSyncOncePodaCanalesObsoletos(t *testing.T) {
	prov := &fakeProvider{}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := services.NewSyncer(nil, prov, chRepo, stRepo, services.Config{})

	antes := time.Now()
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	llamadas := chRepo.staleCalls()
	if len(llamadas) != 1 {
		t.Fatalf("quiero 1 llamada a DeleteStale, tengo %d", len(llamadas))
	}
	if llamadas[0].providerID != prov.ID() {
		t.Errorf("providerID = %q, quiero %q", llamadas[0].providerID, prov.ID())
	}
	if llamadas[0].before.Before(antes) {
		t.Errorf("el corte de poda (%v) debe ser posterior al arranque del sync (%v)",
			llamadas[0].before, antes)
	}
}

// Si el proveedor falla no hay catálogo con el que comparar: podar borraría
// canales buenos por un fallo de red.
func TestSyncOnceNoPodaSiElProveedorFalla(t *testing.T) {
	prov := &fakeProvider{err: errors.New("proveedor caído")}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}

	s := services.NewSyncer(nil, prov, chRepo, stRepo, services.Config{})

	if err := s.SyncOnce(context.Background()); err == nil {
		t.Fatal("SyncOnce debería fallar si el proveedor falla")
	}
	if n := len(chRepo.staleCalls()); n != 0 {
		t.Errorf("no se debe podar tras un sync fallido; hubo %d llamadas", n)
	}
}
```

Y el test del suelo de cordura, que es lo que hace segura la poda:

```go
// Un 200 con una página HTML de error, un portal cautivo o un cuerpo truncado
// produce cero canales. Sin suelo de cordura eso cuenta como sync exitoso y,
// con la poda activa, borraría el catálogo entero.
func TestSyncOnceRechazaUnCatalogoAnomalamentePequeno(t *testing.T) {
	prov := &fakeProvider{}
	chRepo := &fakeChannelRepo{}
	stRepo := &fakeStreamRepo{}
	s := services.NewSyncer(nil, prov, chRepo, stRepo, services.Config{})

	// Primer sync: catálogo normal. fakeProvider devuelve N canales.
	if err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("primer SyncOnce: %v", err)
	}
	llamadasTrasPrimero := len(chRepo.staleCalls())

	// Segundo sync: el proveedor solo devuelve uno.
	prov.setChannels([]domain.Channel{
		{ID: "solo-uno", Name: "Solo Uno", ProviderType: domain.ProviderOpenSource},
	})

	err := s.SyncOnce(context.Background())
	if !errors.Is(err, services.ErrCatalogoSospechoso) {
		t.Fatalf("SyncOnce = %v, quiero ErrCatalogoSospechoso", err)
	}
	if n := len(chRepo.staleCalls()); n != llamadasTrasPrimero {
		t.Errorf("un catálogo sospechoso no debe podar nada; llamadas nuevas: %d",
			n-llamadasTrasPrimero)
	}
}

// El primer sync de la vida del proceso no tiene con qué comparar.
func TestSyncOncePrimerSyncSiempreSeAcepta(t *testing.T) {
	prov := &fakeProvider{}
	prov.setChannels([]domain.Channel{
		{ID: "uno", Name: "Uno", ProviderType: domain.ProviderOpenSource},
	})
	s := services.NewSyncer(nil, prov, &fakeChannelRepo{}, &fakeStreamRepo{}, services.Config{})

	if err := s.SyncOnce(context.Background()); err != nil {
		t.Errorf("el primer sync no tiene referencia previa; debe aceptarse: %v", err)
	}
}
```

Si `fakeProvider` no tiene ya un campo `err` que `GetLiveChannels` devuelva ni un `setChannels`, añadírselos (con el mutex que ya usa). Añadir los imports de `errors` y `time` si faltan.

- [x] **Step 7: Suelo de cordura y poda en el syncer**

En `gateway/internal/services/syncer.go`, añadir junto a los tipos:

```go
// ErrCatalogoSospechoso se devuelve cuando el proveedor entrega muchos menos
// canales que en el último sync exitoso. Un 200 con una página de error o un
// cuerpo truncado produce un M3U válido pero casi vacío; sin este suelo eso
// contaría como éxito, resetearía el backoff a 12h y —con la poda activa—
// borraría el catálogo entero.
var ErrCatalogoSospechoso = errors.New("catálogo anómalamente pequeño")

// minRatioCatalogo es la fracción del catálogo anterior por debajo de la cual
// un sync se rechaza. Los catálogos FTA fluctúan un poco entre syncs; perder
// más de la mitad de golpe es un fallo de la fuente, no una actualización.
const minRatioCatalogo = 0.5
```

Añadir el campo de referencia al struct `Syncer` (junto al `mu` que introduce la Task 14, o creándolo aquí si esta tarea va primero):

```go
	// ultimoConteo es el número de canales del último sync aceptado. Cero
	// significa que aún no hay referencia con la que comparar.
	ultimoConteo int
```

Y reescribir `SyncOnce`:

```go
// SyncOnce descarga los canales del proveedor y persiste canales y streams.
func (s *Syncer) SyncOnce(ctx context.Context) error {
	// Frontera de la poda: todo canal cuyo last_seen_at quede por debajo de
	// este instante es que no apareció en este sync.
	inicio := time.Now()

	channels, err := s.provider.GetLiveChannels(ctx)
	if err != nil {
		return fmt.Errorf("services.SyncOnce (GetLiveChannels): %w", err)
	}

	// Rechazar ANTES de escribir nada: si el catálogo es sospechoso no se hace
	// upsert ni poda, y el backoff sigue tratándolo como fallo.
	if s.ultimoConteo > 0 &&
		float64(len(channels)) < float64(s.ultimoConteo)*minRatioCatalogo {
		return fmt.Errorf("services.SyncOnce: %w (recibidos %d, anterior %d)",
			ErrCatalogoSospechoso, len(channels), s.ultimoConteo)
	}

	if err := s.channels.SaveBatch(ctx, channels); err != nil {
		return fmt.Errorf("services.SyncOnce (canales): %w", err)
	}

	// ... el bloque de streams se queda igual ...

	if err := s.streams.SaveBatch(ctx, streams); err != nil {
		return fmt.Errorf("services.SyncOnce (streams): %w", err)
	}

	podados, err := s.channels.DeleteStale(ctx, s.provider.ID(), inicio)
	if err != nil {
		return fmt.Errorf("services.SyncOnce (poda): %w", err)
	}

	s.ultimoConteo = len(channels)

	s.logger.Info("Sync completado",
		slog.Int("canales", len(channels)),
		slog.Int("streams", len(streams)),
		slog.Int64("podados", podados))
	return nil
}
```

Añadir el import de `errors`.

**Nota de concurrencia:** `ultimoConteo` solo lo toca `SyncOnce`, que corre en serie dentro del bucle de `Run`. No necesita mutex mientras nadie más lo lea; si en el futuro se expone por `/health`, protegerlo con el mismo `mu` que la Task 14 añade para `lastSuccess`.

- [x] **Step 8: Correr los tests y verlos pasar**

Run: `go test ./internal/services/ -v`
Esperado: PASS.

- [x] **Step 9: Correr toda la suite y verificar contra la DB real**

Run: `go test -race -count=1 ./...` → PASS.

```bash
cd ~/Dev/ip-tv/gateway
sqlite3 iptv.db "SELECT COUNT(*) FROM channels c WHERE NOT EXISTS (SELECT 1 FROM streams s WHERE s.channel_id = c.id);"
go build -o server ./cmd/server && ./server &
sleep 25
sqlite3 iptv.db "SELECT COUNT(*) FROM channels c WHERE NOT EXISTS (SELECT 1 FROM streams s WHERE s.channel_id = c.id);"
pkill -f "./server"
```

Esperado: el primer conteo ronda 1157; el segundo baja drásticamente. Los que queden son canales que el proveedor sí lista pero para los que `GetStreamURL` falla — quedan ocultos por la Task 6, que es el comportamiento correcto.

- [x] **Step 10: Commit**

```bash
git add gateway/
git commit -m "gateway: podar canales que el proveedor dejó de listar tras cada sync"
```

---

### Task 8: Congelar el contrato de wire y desempatar la paginación

**Files:**
- Modify: `gateway/internal/domain/channel.go:15-34`
- Modify: `gateway/internal/domain/stream.go`
- Modify: `gateway/internal/adapters/db/channel_repository.go:201`
- Test: `gateway/internal/domain/channel_test.go` (crear)
- Test: `gateway/internal/adapters/db/channel_repository_test.go`

**Interfaces:**
- Consumes: nada.
- Produces: los structs de dominio serializan **exactamente los mismos nombres que hoy** (`ID`, `TvgID`, `Name`, `LogoURL`, `CategoryID`, `LanguageCode`, `CountryCode`, `ProviderID`, `Alive`, `LatencyMs`, `ProviderType`, `IsAdult`, `CreatedAt`, `UpdatedAt`), pero ahora por tag explícito y no por accidente del nombre del campo.

**Contexto:** la app Flutter parsea `json['ID']`, `json['LogoURL']`, `json['Alive']`, `json['LatencyMs']` — es decir, nombres de campo de Go. Renombrar un campo del dominio compila limpio y rompe la app en silencio. Añadir tags que reproduzcan los nombres actuales es un no-op en el cable y convierte el contrato en algo explícito. Aparte, `ORDER BY name COLLATE NOCASE` sin desempate sobre 13 859 filas con nombres repetidos puede duplicar u omitir filas en los bordes de página.

- [x] **Step 1: Escribir el test de contrato que falla**

Crear `gateway/internal/domain/channel_test.go`:

```go
package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/domain"
)

// La app Flutter lee estas claves literalmente (channel.dart). Este test es el
// contrato: si alguien renombra un campo del dominio, falla aquí y no en
// producción con una lista vacía.
func TestChannelJSONCongelaElContratoConLaApp(t *testing.T) {
	alive := true
	ch := domain.Channel{
		ID:           "opensource-BBC One",
		TvgID:        "BBCOne.uk",
		Name:         "BBC One (1080p)",
		LogoURL:      "http://logo",
		CategoryID:   "General",
		LanguageCode: "en",
		CountryCode:  "GB",
		ProviderID:   "opensource",
		Alive:        &alive,
		LatencyMs:    120,
		ProviderType: domain.ProviderOpenSource,
		IsAdult:      false,
		CreatedAt:    time.Unix(0, 0),
		UpdatedAt:    time.Unix(0, 0),
	}

	raw, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, k := range []string{
		"ID", "TvgID", "Name", "LogoURL", "CategoryID", "LanguageCode",
		"CountryCode", "ProviderID", "Alive", "LatencyMs", "ProviderType",
		"IsAdult", "CreatedAt", "UpdatedAt",
	} {
		if _, ok := got[k]; !ok {
			t.Errorf("falta la clave %q en el JSON; la app Flutter la lee", k)
		}
	}
	if len(got) != 14 {
		t.Errorf("el JSON tiene %d claves, quiero 14: añadir un campo al dominio lo filtra al cable", len(got))
	}
}
```

- [x] **Step 2: Correr el test y verlo pasar (es una red, no un cambio)**

Run: `go test ./internal/domain/ -run TestChannelJSON -v`
Esperado: PASS. Confirma cuál es el contrato **actual** antes de tocarlo.

- [x] **Step 3: Añadir los tags explícitos**

En `gateway/internal/domain/channel.go`:

```go
type Channel struct {
	ID ChannelID `json:"ID"`
	// TvgID es el identificador XMLTV (tvg-id del M3U); une el canal con su EPG.
	TvgID        string `json:"TvgID"`
	Name         string `json:"Name"`
	LogoURL      string `json:"LogoURL"`
	CategoryID   string `json:"CategoryID"`
	LanguageCode string `json:"LanguageCode"` // ISO 639-1
	CountryCode  string `json:"CountryCode"`  // ISO 3166-1 alpha-2
	ProviderID   string `json:"ProviderID"`
	// Salud agregada de los streams del canal, rellenada por FindFiltered
	// para que la lista pinte el indicador sin N+1 a /channels/{id}/health.
	// Alive nil = ningún stream chequeado aún.
	Alive        *bool        `json:"Alive"`
	LatencyMs    int64        `json:"LatencyMs"` // mejor latencia entre streams vivos; 0 si no aplica
	ProviderType ProviderType `json:"ProviderType"`
	IsAdult      bool         `json:"IsAdult"`
	CreatedAt    time.Time    `json:"CreatedAt"`
	UpdatedAt    time.Time    `json:"UpdatedAt"`
}
```

Hacer lo mismo en `gateway/internal/domain/stream.go` con los nombres de campo actuales.

- [x] **Step 4: Verificar que el cable no cambió**

Run: `go test ./internal/domain/ -run TestChannelJSON -v`
Esperado: PASS, sin cambios. Los tags reproducen los nombres previos.

- [x] **Step 5: Escribir el test de paginación que falla**

Añadir a `gateway/internal/adapters/db/channel_repository_test.go`:

```go
// Sin desempate, dos canales con el mismo nombre pueden salir en distinto
// orden entre dos queries independientes: una fila se repite en una página y
// desaparece de la otra.
func TestFindFilteredPaginacionEstableConNombresRepetidos(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepo(t)

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
	únicos := map[string]bool{}
	for _, id := range vistos {
		if únicos[id] {
			t.Errorf("el canal %q apareció en dos páginas distintas", id)
		}
		únicos[id] = true
	}
	if len(únicos) != 6 {
		t.Errorf("la paginación omitió canales: %d únicos de 6", len(únicos))
	}
}
```

Si en ese archivo el helper para abrir un repo se llama distinto de `openTestRepo`, usar el que exista.

- [x] **Step 6: Correr el test**

Run: `go test ./internal/adapters/db/ -run TestFindFilteredPaginacion -v`

Esperado: puede pasar o fallar según cómo ordene SQLite este dataset concreto — el orden entre claves iguales no está garantizado, que es precisamente el problema. El arreglo lo vuelve determinista pase lo que pase ahora.

- [x] **Step 7: Añadir el desempate**

En `channel_repository.go`, `FindFiltered`:

```go
		FROM channels WHERE ` + whereSQL +
		// Desempate por id: sin él, SQLite no garantiza un orden estable entre
		// nombres iguales y una fila puede repetirse entre páginas u omitirse.
		" ORDER BY name COLLATE NOCASE, id LIMIT ? OFFSET ?"
```

- [x] **Step 8: Correr los tests y verlos pasar**

Run: `go test ./internal/adapters/db/ -v`
Esperado: PASS.

- [x] **Step 9: Suite completa y commit**

```bash
go test -race -count=1 ./... && gofmt -l . && go vet ./...
git add gateway/
git commit -m "gateway: tags JSON explícitos y desempate por id en la paginación"
```

---

# FASE 2 — La app no se cuelga nunca

*Dos caminos hacen que la app se quede esperando para siempre. Ambos se arreglan en una tarde.*

---

### Task 9: Timeout en toda la capa de datos

**Files:**
- Create: `mobile/lib/data/api_config.dart`
- Modify: `mobile/lib/data/repositories/channel_repository.dart:16-73`
- Modify: `mobile/lib/data/repositories/epg_repository.dart:12-35`
- Test: `mobile/test/data/api_timeout_test.dart` (crear)

**Interfaces:**
- Consumes: nada.
- Produces: `ApiConfig` con `static const String baseUrl` y `static const Duration timeout`. Ambos repositorios lo usan; sus constructores mantienen los parámetros `baseUrl` y `client` para los tests.

**Contexto:** `http.Client()` no tiene deadline por petición. No hay `.timeout()`, ni `connectionTimeout`, ni reintentos en ninguna parte de la app. Cualquier pantalla puede quedarse colgada indefinidamente si el gateway acepta la conexión TCP y no responde. Además la base URL está declarada por duplicado en los dos repositorios.

- [x] **Step 1: Escribir el test que falla**

Crear `mobile/test/data/api_timeout_test.dart`:

```dart
import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'package:iptv_ecosystem/data/api_config.dart';
import 'package:iptv_ecosystem/data/repositories/channel_repository.dart';
import 'package:iptv_ecosystem/data/repositories/epg_repository.dart';

/// Cliente que acepta la petición y no responde jamás: reproduce un gateway
/// que acepta el TCP y se queda colgado, que es el caso que la app no cubre.
class ClienteQueNuncaResponde extends http.BaseClient {
  final _nunca = Completer<http.StreamedResponse>();

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) => _nunca.future;
}

void main() {
  test('getChannels aborta con TimeoutException si el gateway no responde',
      () async {
    final repo = ChannelRepository(
      baseUrl: 'http://127.0.0.1:9',
      client: ClienteQueNuncaResponde(),
    );

    expect(
      () => repo.getChannels(),
      throwsA(isA<TimeoutException>()),
    );
  }, timeout: Timeout(ApiConfig.timeout * 3));

  test('getStreamUrl aborta con TimeoutException si el gateway no responde',
      () async {
    final repo = ChannelRepository(
      baseUrl: 'http://127.0.0.1:9',
      client: ClienteQueNuncaResponde(),
    );

    expect(
      () => repo.getStreamUrl('ch-1'),
      throwsA(isA<TimeoutException>()),
    );
  }, timeout: Timeout(ApiConfig.timeout * 3));

  test('el EPG aborta con TimeoutException si el gateway no responde',
      () async {
    final repo = EPGRepository(
      baseUrl: 'http://127.0.0.1:9',
      client: ClienteQueNuncaResponde(),
    );

    expect(
      () => repo.getForChannel('ch-1', DateTime(2026), DateTime(2026, 1, 2)),
      throwsA(isA<TimeoutException>()),
    );
  }, timeout: Timeout(ApiConfig.timeout * 3));
}
```

Ajustar el nombre del método del EPG (`getForChannel` y su firma) al que exista realmente en `epg_repository.dart`.

- [x] **Step 2: Correr el test y verlo fallar**

Run: `flutter test test/data/api_timeout_test.dart`
Esperado: FAIL al no encontrar `package:iptv_ecosystem/data/api_config.dart`.

- [x] **Step 3: Crear la configuración compartida**

Crear `mobile/lib/data/api_config.dart`:

```dart
/// Configuración de acceso al gateway, compartida por todos los repositorios.
///
/// Vivía duplicada en channel_repository.dart y epg_repository.dart.
class ApiConfig {
  const ApiConfig._();

  /// URL del gateway. Sobreescribible en build con
  /// --dart-define=GATEWAY_URL=http://host:puerto
  static const String baseUrl = String.fromEnvironment(
    'GATEWAY_URL',
    defaultValue: 'http://127.0.0.1:8080',
  );

  /// Deadline por petición. Sin esto, un gateway que acepta la conexión TCP y
  /// no responde deja cualquier pantalla cargando para siempre: http.Client no
  /// impone ningún límite por su cuenta.
  static const Duration timeout = Duration(seconds: 10);
}
```

- [x] **Step 4: Aplicar el timeout en `ChannelRepository`**

En `mobile/lib/data/repositories/channel_repository.dart`, añadir el import de `api_config.dart`, borrar la constante `_defaultBaseUrl` y sustituir las dos llamadas:

```dart
  ChannelRepository({String? baseUrl, http.Client? client})
      : _base = Uri.parse(baseUrl ?? ApiConfig.baseUrl),
        client = client ?? http.Client();
```

```dart
    final response = await client
        .get(_endpoint('/channels', params))
        .timeout(ApiConfig.timeout);
```

```dart
    final response = await client
        .get(_endpoint('/channels/stream', {'id': channelId}))
        .timeout(ApiConfig.timeout);
```

Añadir `import 'dart:async';` si el analizador lo pide para `TimeoutException`.

- [x] **Step 5: Aplicar el mismo cambio en `EPGRepository`**

En `mobile/lib/data/repositories/epg_repository.dart`, borrar su `String.fromEnvironment` duplicado, importar `api_config.dart`, usar `ApiConfig.baseUrl` en el constructor y encadenar `.timeout(ApiConfig.timeout)` a cada `client.get(...)`.

- [x] **Step 6: Correr los tests y verlos pasar**

Run: `flutter test test/data/api_timeout_test.dart`
Esperado: PASS en los 3 tests.

- [x] **Step 7: Suite completa**

Run: `flutter test && flutter analyze`
Esperado: 60 tests en verde, "No issues found!".

- [x] **Step 8: Commit**

```bash
git add mobile/lib/data/ mobile/test/data/
git commit -m "mobile: timeout de 10s en toda la capa de datos y ApiConfig compartido"
```

---

### Task 10: Armar el watchdog del player antes del fetch de la URL

**Files:**
- Modify: `mobile/lib/presentation/screens/player_screen.dart:57-105`
- Test: `mobile/test/presentation/player_screen_test.dart` (crear)

**Interfaces:**
- Consumes: `ApiConfig.timeout` de la Task 9.
- Produces: ningún cambio de firma pública.

**Contexto:** el overlay de carga muestra literalmente `Timeout en 15s`, pero `guard.armLoadTimeout()` está en la línea 94 y `await repo.getStreamUrl(...)` en la 74. El watchdog solo cubre `_player.open()`. Si el gateway acepta la conexión y no responde, el usuario ve "Cargando… / Timeout en 15s" para siempre, y el botón de reintentar está oculto porque depende de `!_isLoading`. La Task 9 acota ese fetch a 10 s, pero el watchdog debe cubrirlo igualmente: es la garantía que la UI promete.

Además `_loadAndPlay` es reentrante sin token de generación: un doble clic en Reintentar hace que la primera invocación añada sus suscripciones a un `_subs` ya vaciado por la segunda, y que abra su URL obsoleta.

- [x] **Step 1: Escribir el test que falla**

Crear `mobile/test/presentation/player_screen_test.dart`:

```dart
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:iptv_ecosystem/data/repositories/channel_repository.dart';
import 'package:iptv_ecosystem/domain/models/channel.dart';
import 'package:iptv_ecosystem/domain/models/channel_filter.dart';
import 'package:iptv_ecosystem/presentation/providers/channel_provider.dart';
import 'package:iptv_ecosystem/presentation/screens/player_screen.dart';

/// Repo cuyo getStreamUrl nunca resuelve: el gateway acepta la conexión y se
/// queda callado. Es el caso que hoy deja la pantalla cargando para siempre.
class RepoColgado implements IChannelRepository {
  @override
  Future<List<Channel>> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit = 500,
    int offset = 0,
  }) async =>
      const [];

  @override
  Future<String> getStreamUrl(String channelId) => Completer<String>().future;
}

void main() {
  testWidgets(
      'si el gateway no responde, el watchdog muestra el error y el botón de reintentar',
      (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          channelRepositoryProvider.overrideWithValue(RepoColgado()),
        ],
        child: const MaterialApp(
          home: PlayerScreen(channelId: 'ch-1', channelName: 'Canal Uno'),
        ),
      ),
    );

    await tester.pump();
    expect(find.text('Canal no disponible'), findsNothing,
        reason: 'al principio debe estar cargando');

    // Pasado el timeout de carga el guard tiene que disparar.
    await tester.pump(const Duration(seconds: 16));
    await tester.pump();

    expect(find.text('Canal no disponible'), findsOneWidget,
        reason: 'el watchdog debe cubrir también el fetch de la URL');
    expect(find.text('Reintentar'), findsWidgets);
  });
}
```

- [x] **Step 2: Correr el test y verlo fallar**

Run: `flutter test test/presentation/player_screen_test.dart`
Esperado: FAIL — la pantalla sigue en estado de carga a los 16 s porque el guard nunca se armó.

- [x] **Step 3: Reordenar y añadir el token de generación**

En `mobile/lib/presentation/screens/player_screen.dart`, añadir el campo de generación junto a `_guard`:

```dart
  final List<StreamSubscription<dynamic>> _subs = [];
  PlaybackGuard? _guard;

  /// Generación de carga. Cada _loadAndPlay incrementa este contador; las
  /// invocaciones anteriores que sigan en vuelo se descartan al volver del
  /// await en vez de pisar el estado de la más reciente.
  int _generacion = 0;
```

Y reescribir `_loadAndPlay`:

```dart
  Future<void> _loadAndPlay() async {
    final generacion = ++_generacion;

    _guard?.dispose();
    for (final s in _subs) {
      s.cancel();
    }
    _subs.clear();
    await _player.stop();

    setState(() {
      _isLoading = true;
      _error = null;
    });

    final guard = PlaybackGuard(onFatal: _onFatal, loadTimeout: _kPlayTimeout);
    _guard = guard;

    // Armar ANTES de resolver la URL: el fetch al gateway puede colgarse igual
    // que open(), y el overlay promete un timeout de _kPlayTimeout para todo
    // el proceso, no solo para la apertura del stream.
    guard.armLoadTimeout();

    try {
      final repo = ref.read(channelRepositoryProvider);
      final streamUrl = await repo.getStreamUrl(widget.channelId);
      if (generacion != _generacion) return;

      if (_player.platform is NativePlayer) {
        final mpv = _player.platform as NativePlayer;
        await mpv.setProperty('network-timeout', '10');
        await mpv.setProperty('demuxer-max-bytes', '4MiB');
      }
      if (generacion != _generacion) return;

      _subs.add(_player.stream.playing.listen((playing) {
        guard.onPlaying(playing);
        if (playing && _isLoading && mounted) {
          setState(() => _isLoading = false);
        }
      }));
      _subs.add(_player.stream.position.listen(guard.onPosition));
      // Los errores de mpv pasan por el guard: los transitorios de HLS en
      // vivo (EOF de segmento, reconexiones) NO matan la reproducción.
      _subs.add(_player.stream.error.listen(guard.onError));

      await _player.open(Media(streamUrl));
    } catch (e) {
      if (generacion != _generacion) return;
      guard.dispose();
      if (mounted) {
        setState(() {
          _isLoading = false;
          _error = e.toString();
        });
      }
    }
  }
```

- [x] **Step 4: Correr el test y verlo pasar**

Run: `flutter test test/presentation/player_screen_test.dart`
Esperado: PASS.

- [x] **Step 5: Suite completa**

Run: `flutter test && flutter analyze`
Esperado: verde. Los 7 tests de `playback_guard_test.dart` deben seguir pasando sin tocarlos.

- [x] **Step 6: Commit**

```bash
git add mobile/lib/presentation/screens/player_screen.dart mobile/test/presentation/player_screen_test.dart
git commit -m "mobile: el watchdog cubre el fetch de la URL y el retry deja de ser reentrante"
```

---

### Task 11: `loadMore` deja de pisar los resultados de un filtro nuevo

**Files:**
- Modify: `mobile/lib/presentation/providers/channel_provider.dart:40-110`
- Modify: `mobile/lib/domain/models/channel_filter.dart`
- Test: `mobile/test/presentation/channel_list_notifier_test.dart`

**Interfaces:**
- Consumes: nada.
- Produces: `ChannelListState` y `ChannelFilter` con `==`/`hashCode` por valor. `loadMore()` mantiene su firma.

**Contexto:** `loadMore` captura `current`, hace `await`, y luego escribe el estado sin comprobar si el filtro cambió mientras tanto. Riverpod no recrea el notifier al re-ejecutar `build()`, así que una búsqueda escrita durante el scroll produce el estado correcto y acto seguido la continuación obsoleta lo sobrescribe con la lista vieja más una página del filtro nuevo. Aparte, `ChannelFilter` no define `==`, así que `StateProvider` compara por identidad y `copyWith` siempre asigna un objeto nuevo: volver a tocar el chip ya activo resetea la lista y el scroll.

- [x] **Step 1: Escribir el test que falla**

Añadir a `mobile/test/presentation/channel_list_notifier_test.dart`:

```dart
  test('loadMore no pisa el resultado de un filtro cambiado a media carga',
      () async {
    final repo = FakeRepoConControl();
    final container = ProviderContainer(overrides: [
      channelRepositoryProvider.overrideWithValue(repo),
      sharedPreferencesProvider.overrideWithValue(await _prefs()),
    ]);
    addTearDown(container.dispose);

    await container.read(channelListProvider.future);

    // Arranca una carga de página que se quedará en vuelo.
    final enVuelo = container.read(channelListProvider.notifier).loadMore();

    // El usuario escribe en el buscador mientras tanto.
    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(query: 'noticias');
    await container.read(channelListProvider.future);

    // Ahora resuelve la página vieja.
    repo.completarPaginaPendiente();
    await enVuelo;

    final estado = container.read(channelListProvider).value!;
    expect(
      estado.channels.every((c) => c.name.toLowerCase().contains('noticias')),
      isTrue,
      reason: 'la continuación obsoleta no puede reintroducir canales del filtro anterior',
    );
  });

  test('ChannelFilter compara por valor', () {
    expect(
      const ChannelFilter(query: 'a', country: 'ES'),
      const ChannelFilter(query: 'a', country: 'ES'),
    );
    expect(
      const ChannelFilter(query: 'a').hashCode,
      const ChannelFilter(query: 'a').hashCode,
    );
  });
```

Añadir en el mismo archivo un `FakeRepoConControl` derivado del `FakeRepo` existente que exponga `completarPaginaPendiente()` para resolver a mano la petición en vuelo, y un helper `_prefs()` que devuelva `SharedPreferences.getInstance()` tras `SharedPreferences.setMockInitialValues({})`.

- [x] **Step 2: Correr los tests y verlos fallar**

Run: `flutter test test/presentation/channel_list_notifier_test.dart`
Esperado: FAIL en ambos — la lista contiene canales del filtro anterior, y `ChannelFilter` compara por identidad.

- [x] **Step 3: Igualdad por valor en `ChannelFilter`**

En `mobile/lib/domain/models/channel_filter.dart`, dentro de la clase:

```dart
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ChannelFilter &&
          other.query == query &&
          other.country == country &&
          other.category == category &&
          other.quality == quality &&
          other.showOffline == showOffline;

  @override
  int get hashCode =>
      Object.hash(query, country, category, quality, showOffline);
```

Ajustar la lista de campos a los que realmente declara la clase.

- [x] **Step 4: Token de generación en el notifier**

En `mobile/lib/presentation/providers/channel_provider.dart`:

```dart
class ChannelListNotifier extends AsyncNotifier<ChannelListState> {
  static const pageSize = 500;

  /// Generación del filtro vigente. build() la incrementa; una continuación de
  /// loadMore que vuelva del await con una generación vieja se descarta en vez
  /// de sobrescribir la lista del filtro nuevo. Riverpod no recrea el notifier
  /// al re-ejecutar build(), así que el estado del objeto sobrevive al cambio.
  int _generacion = 0;

  ChannelFilter _effectiveFilter() => ref
      .read(channelFilterProvider)
      .copyWith(showOffline: ref.read(showOfflineProvider));

  @override
  Future<ChannelListState> build() async {
    ref.watch(channelFilterProvider);
    ref.watch(showOfflineProvider);
    final repo = ref.watch(channelRepositoryProvider);

    final generacion = ++_generacion;
    final page =
        await repo.getChannels(filter: _effectiveFilter(), limit: pageSize);
    if (generacion != _generacion) {
      // Otro build arrancó mientras este esperaba: su resultado manda.
      return state.valueOrNull ??
          const ChannelListState(channels: [], hasMore: false);
    }
    return ChannelListState(
      channels: page,
      hasMore: page.length == pageSize,
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || !current.hasMore || current.isLoadingMore) return;

    final generacion = _generacion;
    state = AsyncData(current.copyWith(isLoadingMore: true));
    try {
      final repo = ref.read(channelRepositoryProvider);
      final page = await repo.getChannels(
        filter: _effectiveFilter(),
        limit: pageSize,
        offset: current.channels.length,
      );
      if (generacion != _generacion) return;
      state = AsyncData(ChannelListState(
        channels: [...current.channels, ...page],
        hasMore: page.length == pageSize,
      ));
    } catch (_) {
      if (generacion != _generacion) return;
      // Conservar lo ya cargado; el usuario puede reintentar con más scroll
      state = AsyncData(current.copyWith(isLoadingMore: false));
    }
  }
}
```

- [x] **Step 5: Correr los tests y verlos pasar**

Run: `flutter test test/presentation/channel_list_notifier_test.dart`
Esperado: PASS.

- [x] **Step 6: Suite completa y commit**

```bash
flutter test && flutter analyze
git add mobile/lib/ mobile/test/
git commit -m "mobile: token de generación en loadMore e igualdad por valor en ChannelFilter"
```

---

### Task 12: Los errores dejan de ser texto crudo de excepción

**Files:**
- Create: `mobile/lib/data/api_error.dart`
- Modify: `mobile/lib/data/repositories/channel_repository.dart`
- Modify: `mobile/lib/data/repositories/epg_repository.dart`
- Modify: `mobile/lib/presentation/screens/home_screen.dart:145-146`
- Modify: `mobile/lib/presentation/screens/guide_screen.dart:154-180`
- Modify: `mobile/lib/presentation/screens/player_screen.dart` (usar el mensaje mapeado)
- Test: `mobile/test/data/api_error_test.dart` (crear)

**Interfaces:**
- Consumes: `ApiConfig` de la Task 9.
- Produces: `ApiError implements Exception` con `final String mensaje` y `factory ApiError.desde(Object error)`. `mensaje` es siempre apto para mostrar a un usuario.

**Contexto:** hoy el usuario ve `ClientException with SocketException: Connection refused (OS Error: Connection refused, errno = 61), address = 127.0.0.1, port = 8080` — exactamente la captura que abrió esta investigación. `home_screen.dart` y `guide_screen.dart` renderizan `e.toString()` directamente; VoiceOver lo lee literal. Además el gateway devuelve `{"error": "..."}` y los repositorios lo descartan. Y en la guía, un `AsyncError` cae en la rama `_` del switch y se pinta como un spinner infinito.

- [x] **Step 1: Escribir el test que falla**

Crear `mobile/test/data/api_error_test.dart`:

```dart
import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'package:iptv_ecosystem/data/api_error.dart';

void main() {
  test('el gateway caído se explica, no se vuelca', () {
    final err = ApiError.desde(
      http.ClientException('Connection refused', Uri.parse('http://127.0.0.1:8080/channels')),
    );
    expect(err.mensaje, contains('gateway'));
    expect(err.mensaje, isNot(contains('errno')));
    expect(err.mensaje, isNot(contains('SocketException')));
  });

  test('el timeout se explica', () {
    final err = ApiError.desde(TimeoutException('agotado'));
    expect(err.mensaje.toLowerCase(), contains('tardó'));
    expect(err.mensaje, isNot(contains('TimeoutException')));
  });

  test('sin red se distingue del gateway caído', () {
    final err = ApiError.desde(const SocketException('Network is unreachable'));
    expect(err.mensaje.toLowerCase(), contains('conexión'));
  });

  test('un ApiError existente no se re-envuelve', () {
    final original = ApiError('mensaje propio');
    expect(identical(ApiError.desde(original), original), isTrue);
  });
}
```

- [x] **Step 2: Correr el test y verlo fallar**

Run: `flutter test test/data/api_error_test.dart`
Esperado: FAIL — no existe `api_error.dart`.

- [x] **Step 3: Implementar el mapeo**

Crear `mobile/lib/data/api_error.dart`:

```dart
import 'dart:async';
import 'dart:io';

import 'package:http/http.dart' as http;

/// Error de la capa de datos con un mensaje apto para enseñar al usuario.
///
/// Sin esto las pantallas pintan e.toString() y el usuario acaba leyendo
/// "ClientException with SocketException: Connection refused (OS Error:
/// Connection refused, errno = 61), address = 127.0.0.1, port = 8080".
class ApiError implements Exception {
  const ApiError(this.mensaje);

  final String mensaje;

  factory ApiError.desde(Object error) {
    if (error is ApiError) return error;

    if (error is TimeoutException) {
      return const ApiError('El gateway tardó demasiado en responder.');
    }
    if (error is SocketException) {
      return const ApiError('Sin conexión de red.');
    }
    if (error is http.ClientException) {
      return const ApiError(
        'No se pudo contactar con el gateway. ¿Está arrancado en el puerto 8080?',
      );
    }
    return const ApiError('Ha ocurrido un error inesperado.');
  }

  /// Construye el error de una respuesta HTTP no exitosa, aprovechando el
  /// cuerpo {"error": "..."} que devuelve el gateway.
  factory ApiError.deRespuesta(int statusCode, String? detalle) {
    if (statusCode == 404) {
      return const ApiError('No disponible.');
    }
    if (statusCode >= 500) {
      return const ApiError('El gateway ha fallado. Inténtalo de nuevo.');
    }
    return ApiError(detalle?.isNotEmpty == true
        ? detalle!
        : 'La petición no se pudo completar ($statusCode).');
  }

  @override
  String toString() => mensaje;
}
```

- [x] **Step 4: Usarlo en los repositorios**

En `channel_repository.dart`, envolver cada llamada y aprovechar el cuerpo de error del gateway:

```dart
  Future<http.Response> _get(Uri uri) async {
    try {
      return await client.get(uri).timeout(ApiConfig.timeout);
    } catch (e) {
      throw ApiError.desde(e);
    }
  }

  String? _detalleDeError(http.Response r) {
    try {
      final body = json.decode(r.body);
      if (body is Map<String, dynamic>) return body['error'] as String?;
    } catch (_) {
      // cuerpo no-JSON: no hay detalle que extraer
    }
    return null;
  }
```

y sustituir en `getChannels` y `getStreamUrl` el `client.get(...)` por `_get(...)` y el `throw Exception(...)` por `throw ApiError.deRespuesta(response.statusCode, _detalleDeError(response))`. Aplicar lo mismo en `epg_repository.dart`.

- [x] **Step 5: Pintar el mensaje, no la excepción**

En `home_screen.dart:145-146`, sustituir `message: e.toString()` por `message: ApiError.desde(e).mensaje`.

En `player_screen.dart`, en el `catch (e)`, sustituir `_error = e.toString()` por `_error = ApiError.desde(e).mensaje`.

En `guide_screen.dart`, añadir la rama de error explícita al switch (líneas 154-180), antes del `_`:

```dart
      AsyncError(:final error) => Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Text(
              ApiError.desde(error).mensaje,
              style: const TextStyle(color: Colors.grey, fontSize: 11),
              textAlign: TextAlign.center,
            ),
          ),
        ),
```

y sustituir el `Text('Error: $e')` de la línea 88 por `Text(ApiError.desde(e).mensaje)`.

- [x] **Step 6: Reconciliar los tests de timeout de la Task 9**

Envolver las excepciones en `ApiError` cambia lo que ve quien llama, así que `mobile/test/data/api_timeout_test.dart` (creado en la Task 9) deja de pasar: esperaba `TimeoutException` en crudo. Ese contrato ya no es el correcto — lo que importa es que la petición aborte y que el mensaje sea legible. Actualizar los tres `expect`:

```dart
    expect(
      () => repo.getChannels(),
      throwsA(
        isA<ApiError>().having(
          (e) => e.mensaje.toLowerCase(),
          'mensaje',
          contains('tardó'),
        ),
      ),
    );
```

Aplicar la misma forma a los tres tests, añadiendo `import 'package:iptv_ecosystem/data/api_error.dart';` y quitando el import de `dart:async` si deja de usarse.

- [x] **Step 7: Correr los tests y verlos pasar**

Run: `flutter test test/data/`
Esperado: PASS en los 4 de `api_error_test.dart` y en los 3 de `api_timeout_test.dart`.

- [x] **Step 8: Verificar a mano el caso original**

Con el gateway **parado**:

```bash
pkill -f "cmd/server" ; pkill -f "gateway/server"
cd ~/Dev/ip-tv/mobile && flutter run -d macos
```

Esperado: la pantalla de error dice "No se pudo contactar con el gateway. ¿Está arrancado en el puerto 8080?" en lugar del volcado de `SocketException`.

- [x] **Step 9: Suite completa y commit**

```bash
flutter test && flutter analyze
git add mobile/
git commit -m "mobile: mapear errores de red a mensajes legibles y rama AsyncError en la guía"
```

---

# FASE 3 — Observabilidad

*Que el próximo incidente se diagnostique solo.*

---

### Task 13: Loguear la causa de cada 5xx

**Files:**
- Modify: `gateway/internal/api/handlers/channel_handler.go:14-22,54-57,93-97`
- Modify: `gateway/internal/api/handlers/epg_handler.go`
- Modify: `gateway/internal/api/router.go:25-26`
- Test: `gateway/internal/api/handlers/channel_handler_test.go`

**Interfaces:**
- Consumes: nada.
- Produces: `handlers.NewChannelHandler(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository) *ChannelHandler` y `handlers.NewEPGHandler(logger *slog.Logger, epg ports.EPGRepository) *EPGHandler` — el logger pasa a ser el **primer** parámetro en ambos.

**Contexto:** `if err != nil { h.writeError(w, 500, "Error obteniendo canales"); return }` descarta `err` sin loguearlo. Un error de DB, un WAL corrupto y un context deadline salen idénticos: un 500 opaco y silencio en los logs.

- [x] **Step 1: Escribir el test que falla**

Añadir a `gateway/internal/api/handlers/channel_handler_test.go`:

```go
// Un 500 sin rastro en los logs hace indistinguible un fallo de DB de un
// deadline o un WAL corrupto.
func TestGetChannelsLogueaLaCausaDelError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	repo := &fakeChannelRepo{err: errors.New("disco en llamas")}
	h := handlers.NewChannelHandler(logger, repo, &fakeProvider{}, &fakeStreamRepo{})

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rec := httptest.NewRecorder()
	h.GetChannels(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, quiero 500", rec.Code)
	}
	if !strings.Contains(buf.String(), "disco en llamas") {
		t.Errorf("la causa del 500 debe aparecer en los logs; log:\n%s", buf.String())
	}
}
```

Ajustar los nombres de los fakes a los que ya existan en ese archivo, y darle al fake de canales un campo `err` que `FindFiltered` devuelva.

- [x] **Step 2: Correr el test y verlo fallar**

Run: `go test ./internal/api/handlers/ -run TestGetChannelsLoguea -v`
Esperado: FAIL — `NewChannelHandler` no acepta un logger.

- [x] **Step 3: Inyectar el logger y usarlo**

En `channel_handler.go`:

```go
type ChannelHandler struct {
	logger   *slog.Logger
	repo     ports.ChannelRepository
	provider ports.ProviderPort
	streams  ports.StreamRepository
}

func NewChannelHandler(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository) *ChannelHandler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &ChannelHandler{logger: logger, repo: repo, provider: provider, streams: streams}
}
```

Y en cada punto que hoy devuelve un 500, loguear antes. En `GetChannels`:

```go
	if err != nil {
		h.logger.Error("GetChannels: fallo consultando el catálogo", slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo canales")
		return
	}
```

En `GetHealth`:

```go
	if err != nil {
		h.logger.Error("GetHealth: fallo consultando streams",
			slog.String("channel", id), slog.Any("error", err))
		h.writeError(w, http.StatusInternalServerError, "Error obteniendo salud del canal")
		return
	}
```

Hacer lo mismo en `epg_handler.go` en sus tres puntos de 500 (líneas 68, 87 y 97 del original).

- [x] **Step 4: Cablear en el router**

En `gateway/internal/api/router.go`:

```go
	ch := handlers.NewChannelHandler(logger, repo, provider, streams)
	eh := handlers.NewEPGHandler(logger, epg)
```

- [x] **Step 5: Correr los tests y verlos pasar**

Run: `go test ./internal/api/... -v`
Esperado: PASS. Actualizar las llamadas a `NewChannelHandler`/`NewEPGHandler` en los tests existentes para que pasen un logger (`slog.New(slog.DiscardHandler)` sirve).

- [x] **Step 6: Suite completa y commit**

```bash
go test -race -count=1 ./... && gofmt -l . && go vet ./...
git add gateway/internal/api/
git commit -m "gateway: loguear la causa de cada 5xx en vez de descartarla"
```

---

### Task 14: `/health` que dice la verdad

**Files:**
- Create: `gateway/internal/api/handlers/health_handler.go`
- Create: `gateway/internal/api/handlers/health_handler_test.go`
- Modify: `gateway/internal/services/syncer.go`
- Modify: `gateway/internal/api/router.go:15,28-31`
- Modify: `gateway/cmd/server/main.go` (pasar el syncer y la DB al router)

**Interfaces:**
- Consumes: el `Syncer` de la Task 7.
- Produces:
  - `Syncer.LastSuccess() time.Time` — instante del último sync exitoso; cero si aún no hubo ninguno. Protegido con `sync.RWMutex`.
  - `handlers.NewHealthHandler(db *sql.DB, syncer SyncStatus) *HealthHandler` con `type SyncStatus interface { LastSuccess() time.Time }`.
  - `NewRouter(logger, repo, provider, streams, epg, sqlDB, syncer)` — dos parámetros nuevos al final.
  - Respuesta: `{"status":"ok|degraded","db":"ok|error","last_sync":"RFC3339|null","sync_age_seconds":N|null}`. `status` es `degraded` si la DB no responde o si el último sync exitoso tiene más de 26 horas (dos intervalos de 12 h más margen).

**Contexto:** `/health` devuelve el literal `{"status":"ok"}` sin tocar la DB ni mirar la edad del sync. Un gateway cuyo syncer lleva tres días fallando sigue reportando salud perfecta.

- [x] **Step 1: Escribir el test que falla**

Crear `gateway/internal/api/handlers/health_handler_test.go`:

```go
package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/adapters/db"
	"github.com/tu-org/iptv-ecosystem/gateway/internal/api/handlers"
)

type fakeSyncStatus struct{ last time.Time }

func (f fakeSyncStatus) LastSuccess() time.Time { return f.last }

func TestHealthReportaEdadDelSync(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	h := handlers.NewHealthHandler(sqlDB, fakeSyncStatus{last: time.Now().Add(-2 * time.Hour)})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quiero 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if got["status"] != "ok" {
		t.Errorf("status = %v, quiero ok", got["status"])
	}
	if got["db"] != "ok" {
		t.Errorf("db = %v, quiero ok", got["db"])
	}
	edad, ok := got["sync_age_seconds"].(float64)
	if !ok || edad < 7000 || edad > 7400 {
		t.Errorf("sync_age_seconds = %v, quiero ~7200", got["sync_age_seconds"])
	}
}

// Un syncer que lleva días fallando no puede reportar salud perfecta.
func TestHealthDegradadoSiElSyncEsMuyViejo(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	h := handlers.NewHealthHandler(sqlDB, fakeSyncStatus{last: time.Now().Add(-72 * time.Hour)})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("respuesta no es JSON: %v", err)
	}
	if got["status"] != "degraded" {
		t.Errorf("status = %v, quiero degraded con un sync de 72h", got["status"])
	}
}

// Una DB cerrada debe reflejarse, no ocultarse.
func TestHealthDetectaDBCaida(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	sqlDB.Close()

	h := handlers.NewHealthHandler(sqlDB, fakeSyncStatus{last: time.Now()})
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status HTTP = %d, quiero 503 con la DB caída", rec.Code)
	}
}
```

- [x] **Step 2: Correr los tests y verlos fallar**

Run: `go test ./internal/api/handlers/ -run TestHealth -v`
Esperado: FAIL — `handlers.NewHealthHandler` no existe.

- [x] **Step 3: Exponer `LastSuccess` en el syncer**

En `gateway/internal/services/syncer.go`, añadir al struct y actualizarlo en `Run`:

```go
type Syncer struct {
	// ... campos existentes ...

	mu          sync.RWMutex
	lastSuccess time.Time
}

// LastSuccess devuelve el instante del último sync exitoso. Cero si aún no
// ha habido ninguno. Lo consume /health para reportar la edad del catálogo.
func (s *Syncer) LastSuccess() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSuccess
}
```

y en el `else` de `Run` (rama de éxito), antes de `s.firstOnce.Do(...)`:

```go
		} else {
			s.mu.Lock()
			s.lastSuccess = time.Now()
			s.mu.Unlock()

			s.firstOnce.Do(func() { close(s.firstDone) })
			wait = s.cfg.Interval
			backoff = s.cfg.RetryBase
		}
```

- [x] **Step 4: Implementar el handler**

Crear `gateway/internal/api/handlers/health_handler.go`:

```go
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// maxSyncAge es la edad a partir de la cual el catálogo se considera rancio.
// Dos intervalos de sync (12h) más margen: por debajo de eso un fallo aislado
// con reintentos todavía es operación normal.
const maxSyncAge = 26 * time.Hour

// SyncStatus expone lo que /health necesita saber del syncer sin acoplarse
// al tipo concreto.
type SyncStatus interface {
	LastSuccess() time.Time
}

// HealthHandler responde /health con el estado real: si la DB contesta y si el
// catálogo se ha sincronizado hace poco. El literal {"status":"ok"} anterior
// no distinguía un gateway sano de uno cuyo syncer llevaba días fallando.
type HealthHandler struct {
	db     *sql.DB
	syncer SyncStatus
}

func NewHealthHandler(db *sql.DB, syncer SyncStatus) *HealthHandler {
	return &HealthHandler{db: db, syncer: syncer}
}

func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	res := map[string]any{"status": "ok", "db": "ok"}
	httpStatus := http.StatusOK

	if err := h.db.PingContext(ctx); err != nil {
		res["db"] = "error"
		res["status"] = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	last := h.syncer.LastSuccess()
	if last.IsZero() {
		res["last_sync"] = nil
		res["sync_age_seconds"] = nil
	} else {
		edad := time.Since(last)
		res["last_sync"] = last.Format(time.RFC3339)
		res["sync_age_seconds"] = int64(edad.Seconds())
		if edad > maxSyncAge {
			res["status"] = "degraded"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(res)
}
```

- [x] **Step 5: Cablear en el router y en main**

En `gateway/internal/api/router.go`:

```go
func NewRouter(logger *slog.Logger, repo ports.ChannelRepository, provider ports.ProviderPort, streams ports.StreamRepository, epg ports.EPGRepository, sqlDB *sql.DB, syncer handlers.SyncStatus) http.Handler {
	// ...
	hh := handlers.NewHealthHandler(sqlDB, syncer)
	r.Get("/health", hh.Get)
```

Añadir el import de `database/sql` y borrar el handler inline. En `cmd/server/main.go`, la llamada pasa a:

```go
	handler := api.NewRouter(logger, channelRepo, provider, streamRepo, epgRepo, sqlDB, syncer)
```

- [x] **Step 6: Correr los tests y verlos pasar**

Run: `go test ./internal/... -v`
Esperado: PASS.

- [x] **Step 7: Verificar contra el gateway real**

```bash
cd ~/Dev/ip-tv/gateway && go build -o server ./cmd/server && ./server &
sleep 25 && curl -s localhost:8080/health | python3 -m json.tool ; pkill -f "./server"
```

Esperado: `status: ok`, `db: ok`, `sync_age_seconds` de pocos segundos.

- [x] **Step 8: Commit**

```bash
git add gateway/
git commit -m "gateway: /health reporta estado de DB y edad del último sync"
```

---

### Task 15: Silenciar el ruido del checker y capturar lo que escape a slog

**Files:**
- Modify: `gateway/internal/adapters/validator/checker.go:29-42,58`
- Modify: `gateway/cmd/server/main.go:20-22`
- Test: `gateway/internal/adapters/validator/checker_test.go`

**Interfaces:**
- Consumes: nada.
- Produces: ningún cambio de firma.

**Contexto:** el `Unsolicited response received on idle HTTP channel` del log tiene causa confirmada. Se manda `HEAD` con keep-alive activo; orígenes IPTV rotos (MistServer sobre todo) responden a un HEAD escribiendo cuerpo igualmente, o escriben una segunda respuesta HTTP completa en el mismo socket. El transporte de Go devuelve la conexión al pool de inactivas con esos bytes sin leer, lo detecta al siguiente peek, y lo registra con el paquete **global `log`** — que no pasa por el handler JSON de `slog`. Son ~8-9 líneas por pasada horaria (0,07% de 12 728 URLs).

- [x] **Step 1: Escribir el test que falla**

Añadir a `gateway/internal/adapters/validator/checker_test.go`:

```go
// Los orígenes IPTV rotos responden a un HEAD escribiendo cuerpo igualmente.
// Con keep-alive, esa conexión vuelve al pool con bytes sin leer y el
// transporte de Go escupe "Unsolicited response received on idle HTTP channel"
// por el paquete log global, saltándose el handler JSON de slog.
func TestCheckerNoReutilizaConexionesInactivas(t *testing.T) {
	c := validator.NewChecker(nil, time.Second)
	tr := c.Transport()
	if tr == nil {
		t.Fatal("el checker debe exponer su transporte para poder verificarlo")
	}
	if !tr.DisableKeepAlives {
		t.Error("DisableKeepAlives debe estar activo: sin pool de inactivas no hay peek fallido")
	}
}

func TestCheckerMandaUserAgentDeReproductor(t *testing.T) {
	var recibido string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recibido = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := validator.NewChecker(nil, 2*time.Second)
	res := c.Check(context.Background(), srv.URL+"/stream.m3u8")
	if !res.IsAlive {
		t.Fatalf("el stream de prueba debería dar vivo: %v", res.Error)
	}
	if recibido == "" || strings.HasPrefix(recibido, "Go-http-client") {
		t.Errorf("User-Agent = %q; algunos orígenes filtran el default de Go", recibido)
	}
}
```

- [x] **Step 2: Correr los tests y verlos fallar**

Run: `go test ./internal/adapters/validator/ -run "TestCheckerNoReutiliza|TestCheckerManda" -v`
Esperado: FAIL — no existe `Transport()` y el User-Agent es el de Go.

- [x] **Step 3: Ajustar el checker**

En `gateway/internal/adapters/validator/checker.go`:

```go
// userAgent identifica al checker como un reproductor. Algunos orígenes
// filtran el default de Go (Go-http-client/2.0) con un 403.
const userAgent = "VLC/3.0.20 LibVLC/3.0.20"

type Checker struct {
	client    HTTPChecker
	transport *http.Transport
	timeout   time.Duration
}

func NewChecker(client HTTPChecker, timeout time.Duration) *Checker {
	var tr *http.Transport
	if client == nil {
		// DisableKeepAlives: los orígenes IPTV rotos responden a un HEAD
		// escribiendo cuerpo igualmente, o escriben una segunda respuesta
		// completa en el mismo socket. Si la conexión vuelve al pool de
		// inactivas con esos bytes pendientes, el transporte lo detecta al
		// siguiente peek y lo registra por el paquete log global, fuera de
		// slog. Sin pool de inactivas, ese camino no existe.
		tr = &http.Transport{
			MaxConnsPerHost:   maxConnsPerHost,
			DisableKeepAlives: true,
		}
		client = &http.Client{Timeout: timeout, Transport: tr}
	}
	return &Checker{client: client, transport: tr, timeout: timeout}
}

// Transport expone el transporte propio del checker, o nil si se le inyectó un
// cliente desde fuera. Solo para tests.
func (c *Checker) Transport() *http.Transport { return c.transport }
```

Y añadir la cabecera a las dos peticiones, tras crear cada request:

```go
	req.Header.Set("User-Agent", userAgent)
```

```go
	reqGet.Header.Set("User-Agent", userAgent)
```

Además, drenar el cuerpo del GET de fallback antes de cerrarlo para no dejar la conexión inutilizable:

```go
	defer respGet.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(respGet.Body, 64<<10))
```

Añadir el import de `io`.

- [x] **Step 4: Puente del log global a slog en main**

En `gateway/cmd/server/main.go`, tras crear el logger:

```go
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// El paquete log global de la stdlib lo usan dependencias como
	// net/http.Transport para avisos que no pasan por slog. Redirigirlo evita
	// líneas sin estructurar mezcladas con el JSON.
	slog.SetDefault(logger)
	log.SetFlags(0)
	log.SetOutput(slogWriter{logger})
```

y añadir al final del archivo:

```go
// slogWriter reencamina lo que escriba el paquete log global hacia slog.
type slogWriter struct{ l *slog.Logger }

func (w slogWriter) Write(p []byte) (int, error) {
	w.l.Warn("stdlib log", slog.String("msg", strings.TrimSpace(string(p))))
	return len(p), nil
}
```

Añadir los imports de `log` y `strings`.

- [x] **Step 5: Correr los tests y verlos pasar**

Run: `go test ./internal/adapters/validator/ -v`
Esperado: PASS.

- [x] **Step 6: Verificar sobre tráfico real**

```bash
cd ~/Dev/ip-tv/gateway
go build -o server ./cmd/server
HEALTH_INTERVAL=2m ./server > /tmp/gw.log 2>&1 &
sleep 240
grep -c "Unsolicited response" /tmp/gw.log || echo "0 líneas de ruido"
grep -c "Health-check completado" /tmp/gw.log
pkill -f "./server"
```

Esperado: cero apariciones de `Unsolicited response`, y al menos una pasada de health-check completada.

- [x] **Step 7: Commit**

```bash
go test -race -count=1 ./... && gofmt -l . && go vet ./...
git add gateway/
git commit -m "gateway: sin keep-alive ni ruido en el checker; log global redirigido a slog"
```

---

# FASE 4 — Que la señal de tests sea honesta

---

### Task 16: Borrar el código muerto

**Files:**
- Delete: `gateway/internal/adapters/failover/` (paquete completo, incluidos tests)
- Delete: `gateway/internal/adapters/healthcheck/` (paquete completo, incluidos tests)
- Delete: `gateway/internal/adapters/cache/` (directorio vacío)
- Delete: `gateway/internal/domain/mirror.go`, `gateway/internal/domain/sport_event.go`, `gateway/internal/domain/provider.go`, `gateway/internal/domain/category.go`
- Delete: `gateway/internal/ports/cache_port.go`
- Modify: `gateway/internal/adapters/db/schema.sql` (borrar tablas `sync_log` y `user_agents`)
- Modify: `gateway/internal/ports/provider_port.go` (quitar `HealthCheck` si sigue sin llamarse)

**Interfaces:**
- Consumes: nada.
- Produces: nada. Es una resta pura.

**Contexto:** `failover` (68% de cobertura) y `healthcheck` (80%) no los importa nadie fuera de sus propios tests. Dos de los ocho paquetes Go en verde testean código que no puede ejecutarse, lo que hace que "todos los tests pasan" suene más fuerte de lo que es.

**Decisión previa obligatoria:** `healthcheck/hls_checker.go` **sí descarga el primer segmento HLS**, que es lo que `PROMPT_MAESTRO.md:250` especifica para la Fase 7.1, mientras que el `validator/checker.go` que está cableado solo hace HEAD→GET sobre la playlist. Es decir, el checker que cumple el spec es el muerto. Antes de borrar, decidir:

- **(a) Borrar `healthcheck`** y actualizar `PROMPT_MAESTRO.md:250` para que refleje que la validación es a nivel de playlist. Más simple, menos preciso.
- **(b) Conservar `healthcheck`** y cablearlo como segunda pasada profunda sobre los streams que la pasada superficial da por vivos. Cumple el spec, pero es una tarea aparte con su propio diseño.

Este plan asume **(a)**. Si se prefiere (b), sacar `healthcheck/` de esta tarea y planificarlo por separado.

- [x] **Step 1: Confirmar que de verdad no los usa nadie**

```bash
cd ~/Dev/ip-tv/gateway
for p in failover healthcheck; do
  echo "== $p =="
  grep -rn "adapters/$p" --include="*.go" . | grep -v "/$p/" || echo "  sin importadores"
done
grep -rn "CachePort\|domain.Category\|domain.Mirror\|domain.SportEvent\|XtreamCredentials" --include="*.go" . | grep -v "_test.go" || echo "sin referencias en producción"
```

Esperado: "sin importadores" y "sin referencias en producción". **Si algo aparece, parar** y revisar antes de borrar.

- [x] **Step 2: Borrar**

```bash
cd ~/Dev/ip-tv
git rm -r gateway/internal/adapters/failover gateway/internal/adapters/healthcheck
git rm gateway/internal/domain/mirror.go gateway/internal/domain/sport_event.go \
       gateway/internal/domain/provider.go gateway/internal/domain/category.go \
       gateway/internal/ports/cache_port.go
rmdir gateway/internal/adapters/cache 2>/dev/null || true
```

- [x] **Step 3: Limpiar el esquema**

En `gateway/internal/adapters/db/schema.sql`, borrar los bloques completos de `sync_log` (con su índice) y `user_agents`. La tabla `sync_log` no se escribe ni se lee nunca; `user_agents` no se lee (y su función de siembra ya se eliminó en la Task 4).

Las tablas seguirán existiendo en las DBs ya creadas; no se borran para no arriesgar datos. Dejarlo anotado con un comentario en el archivo:

```sql
-- Nota: sync_log y user_agents existieron en esquemas anteriores y pueden
-- seguir presentes en DBs antiguas. No se usan; no se borran para no tocar
-- datos existentes sin necesidad.
```

- [x] **Step 4: Compilar y correr toda la suite**

```bash
cd ~/Dev/ip-tv/gateway
go build ./... && go vet ./... && go test -race -count=1 ./... && gofmt -l .
```

Esperado: PASS. Si algún import queda huérfano, quitarlo. Confirmar que los paquetes listados bajan de 8 a 6.

- [x] **Step 5: Verificar la nueva foto de cobertura**

```bash
go test -cover ./... 2>&1 | grep -v "no test files"
```

Esperado: ya no aparecen `failover` ni `healthcheck`. Los porcentajes restantes describen código que se ejecuta de verdad.

- [x] **Step 6: Commit**

```bash
cd ~/Dev/ip-tv
git add gateway/
git commit -m "gateway: borrar failover, healthcheck y tipos de dominio sin uso"
```

---

### Task 17: Tests del middleware y `main` testeable

**Files:**
- Create: `gateway/internal/api/middleware/ratelimit_test.go`
- Create: `gateway/internal/api/middleware/recover_test.go`
- Modify: `gateway/cmd/server/main.go` (extraer `run()`)
- Create: `gateway/cmd/server/main_test.go`

**Interfaces:**
- Consumes: `NewRouter` de la Task 14.
- Produces: `func run(ctx context.Context, logger *slog.Logger) error` en `package main`. `main()` queda como `os.Exit` sobre su resultado.

**Contexto:** `internal/api/middleware/` no tiene ningún test: el semáforo del rate limiter, el recover, el CORS y el logger están sin cubrir. `main.go` tiene ~90 líneas de parseo de entorno y secuenciación de workers (incluido el gating por `FirstSyncDone()`) que hoy son intestables por estar todas dentro de `main`. Además el apagado no espera a los workers: los defers se desenrollan LIFO y `sqlDB.Close()` corre mientras las goroutines pueden seguir dentro de un `ExecContext`.

- [x] **Step 1: Escribir los tests de middleware**

Crear `gateway/internal/api/middleware/ratelimit_test.go`:

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/api/middleware"
)

// El limitador es un semáforo global: al llenarse debe responder 429 en vez de
// encolar. Si esto regresa en silencio, la API se cuelga bajo carga.
func TestRateLimiterDevuelve429AlLlenarse(t *testing.T) {
	bloquear := make(chan struct{})
	dentro := make(chan struct{})

	h := middleware.RateLimiter(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dentro <- struct{}{}
		<-bloquear
	}))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()

	<-dentro // el único slot está ocupado

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, quiero 429 con el semáforo lleno", rec.Code)
	}

	close(bloquear)
	wg.Wait()
}

func TestRateLimiterLiberaElSlotAlTerminar(t *testing.T) {
	h := middleware.RateLimiter(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("petición %d: status = %d, quiero 200", i, rec.Code)
		}
	}
}
```

Crear `gateway/internal/api/middleware/recover_test.go`:

```go
package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tu-org/iptv-ecosystem/gateway/internal/api/middleware"
)

func TestRecoverConvierteElPanicEn500YLoRegistra(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := middleware.Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, quiero 500", rec.Code)
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("el panic debe quedar registrado; log:\n%s", buf.String())
	}
}
```

- [x] **Step 2: Correr los tests**

Run: `go test ./internal/api/middleware/ -v`

Esperado: PASS si el middleware ya se comporta así; FAIL si no. Si `RateLimiter` encola en vez de rechazar, o `Recover` no loguea, **eso es un hallazgo real**: arreglar el middleware, no el test.

- [x] **Step 3: Extraer `run()` en main**

Reestructurar `gateway/cmd/server/main.go`:

```go
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	log.SetFlags(0)
	log.SetOutput(slogWriter{logger})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("Fallo fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

// run monta el stack completo y bloquea hasta que ctx se cancele. Separado de
// main para que sea testeable: main solo traduce el error a un exit code.
func run(ctx context.Context, logger *slog.Logger) error {
	logger.Info("Iniciando IPTV Ecosystem API Gateway")

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "iptv.db"
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("abriendo la base de datos: %w", err)
	}
	defer sqlDB.Close()
	logger.Info("SQLite abierta", slog.String("path", dbPath))

	channelRepo := db.NewChannelRepository(sqlDB)
	streamRepo := db.NewStreamRepository(sqlDB)
	epgRepo := db.NewEPGRepository(sqlDB)

	iptvOrgURL := os.Getenv("IPTV_ORG_URL")
	if iptvOrgURL == "" {
		iptvOrgURL = "https://iptv-org.github.io/iptv/index.m3u"
	}
	provider := opensource.NewProvider("opensource", iptvOrgURL, nil)

	syncInterval := durationEnv(logger, "SYNC_INTERVAL", 12*time.Hour)
	healthInterval := durationEnv(logger, "HEALTH_INTERVAL", 60*time.Minute)

	syncer := services.NewSyncer(logger, provider, channelRepo, streamRepo, services.Config{
		Interval: syncInterval,
	})

	syncCtx, stopSync := context.WithCancel(context.Background())
	defer stopSync()

	// Un WaitGroup por cada worker: el apagado tiene que esperarlos antes de
	// cerrar la DB, o sqlDB.Close() corre mientras alguno sigue en ExecContext.
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		syncer.Run(syncCtx)
	}()

	healthWorker := validator.NewWorker(streamRepo, validator.DefaultConfig(), healthInterval, logger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-syncCtx.Done():
		case <-syncer.FirstSyncDone():
			healthWorker.Start(syncCtx)
		}
	}()

	if epgURL := os.Getenv("EPG_URL"); epgURL != "" {
		epgInterval := durationEnv(logger, "EPG_INTERVAL", 12*time.Hour)
		epgWorker := epg.NewWorker(epg.NewParser(), epgRepo, epgURL, epgInterval, logger)
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-syncCtx.Done():
			case <-syncer.FirstSyncDone():
				epgWorker.Start(syncCtx)
			}
		}()
	} else {
		logger.Info("EPG_URL no configurada; worker EPG desactivado")
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:8080"
	}
	handler := api.NewRouter(logger, channelRepo, provider, streamRepo, epgRepo, sqlDB, syncer)
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// El fallo del servidor viaja por un canal en vez de os.Exit(1): así el
	// apagado ordenado se ejecuta igual y los defers no se saltan.
	srvErr := make(chan error, 1)
	go func() {
		logger.Info("Servidor escuchando", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	select {
	case err := <-srvErr:
		return fmt.Errorf("servidor HTTP: %w", err)
	case <-ctx.Done():
	}

	logger.Info("Apagando servidor...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown del servidor HTTP", slog.Any("error", err))
	}

	// Orden importante: parar los workers y esperarlos ANTES de que el defer
	// de sqlDB.Close() se desenrolle.
	stopSync()
	wg.Wait()

	logger.Info("Apagado limpio")
	return nil
}

// durationEnv lee una duración de entorno con fallback y aviso si es inválida.
// Extraída porque el mismo patrón estaba repetido tres veces en main.
func durationEnv(logger *slog.Logger, key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		logger.Warn("Duración inválida, usando default",
			slog.String("var", key), slog.String("valor", v), slog.Duration("default", def))
		return def
	}
	return d
}
```

Añadir el import de `sync`. El `defer sqlDB.Close()` se queda donde está: al ir después de `wg.Wait()` en el orden de desenrollado, la DB se cierra la última.

- [x] **Step 4: Escribir el test de arranque y apagado**

Crear `gateway/cmd/server/main_test.go`:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// run debe montar el stack, servir, y apagar limpiamente al cancelar el
// contexto — sin dejar la DB cerrada bajo los workers.
func TestRunArrancaYApagaLimpio(t *testing.T) {
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "test.db"))
	t.Setenv("LISTEN_ADDR", "127.0.0.1:18080")
	// Provider inalcanzable: el sync fallará y reintentará, que es justo lo que
	// queremos comprobar que no impide un apagado limpio.
	t.Setenv("IPTV_ORG_URL", "http://127.0.0.1:1/index.m3u")

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- run(ctx, slog.New(slog.DiscardHandler)) }()

	// Esperar a que escuche.
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		resp, err = http.Get("http://127.0.0.1:18080/health")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("el servidor no llegó a escuchar: %v", err)
	}
	resp.Body.Close()

	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("run devolvió error en un apagado limpio: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run no terminó en 15s tras cancelar el contexto")
	}
}
```

- [x] **Step 5: Correr el test y verlo pasar**

Run: `go test ./cmd/server/ -race -v`
Esperado: PASS. Si cuelga, es que un worker no respeta la cancelación del contexto — arreglarlo, es un bug real.

- [x] **Step 6: Suite completa y commit**

```bash
go test -race -count=1 ./... && gofmt -l . && go vet ./...
git add gateway/
git commit -m "gateway: tests de middleware y run() testeable con apagado ordenado"
```

---

# FASE 5 — Rendimiento medido

*Los números de esta fase salen de mediciones sobre una copia de la DB real, no de estimaciones.*

---

### Task 18: Escrituras del health-check en lote

**Files:**
- Modify: `gateway/internal/ports/stream_repository.go`
- Modify: `gateway/internal/adapters/db/stream_repository.go`
- Modify: `gateway/internal/adapters/validator/worker.go:82-99`
- Test: `gateway/internal/adapters/db/stream_repository_test.go`

**Interfaces:**
- Consumes: la histéresis de la Task 5.
- Produces: `StreamRepository.MarkBatch(ctx context.Context, resultados []ports.StreamHealth) error`, con `type StreamHealth struct { StreamID string; IsAlive bool; LatencyMs int64 }`. Aplica todos los resultados en **una** transacción, respetando la misma histéresis que `MarkDead`.

**Contexto:** medido sobre una copia de la DB real con el mismo driver: 2 000 UPDATEs sueltos tardan **90 ms**; los mismos 2 000 dentro de una transacción tardan **8,6 ms**. Extrapolado a los 12 728 streams del catálogo, la pasada horaria pasa de ~573 ms de conexión ocupada en exclusiva a ~55 ms. Como `MaxOpenConns` es 1, ese tiempo es API congelada.

- [x] **Step 1: Escribir el test que falla**

Añadir a `gateway/internal/adapters/db/stream_repository_test.go`:

```go
// MarkBatch debe aplicar todos los resultados en una transacción y respetar
// la misma histéresis que MarkDead.
func TestMarkBatchAplicaVivosYMuertosConHisteresis(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	seedChannel(t, chRepo, "ch-1")

	for _, id := range []string{"st-vivo", "st-muerto"} {
		if err := stRepo.Save(ctx, makeStream(id, "ch-1", "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save(%s): %v", id, err)
		}
	}

	// Tantas pasadas fallidas como marque el umbral.
	for i := int64(0); i < db.DeadFailThreshold; i++ {
		err := stRepo.MarkBatch(ctx, []ports.StreamHealth{
			{StreamID: "st-vivo", IsAlive: true, LatencyMs: 42},
			{StreamID: "st-muerto", IsAlive: false},
		})
		if err != nil {
			t.Fatalf("MarkBatch #%d: %v", i, err)
		}
	}

	streams, err := stRepo.FindByChannelID(ctx, "ch-1")
	if err != nil {
		t.Fatalf("FindByChannelID: %v", err)
	}
	porID := map[string]domain.Stream{}
	for _, s := range streams {
		porID[s.ID] = s
	}

	if !porID["st-vivo"].IsAlive {
		t.Error("st-vivo debe seguir vivo")
	}
	if porID["st-vivo"].LatencyMs != 42 {
		t.Errorf("latencia = %d, quiero 42", porID["st-vivo"].LatencyMs)
	}
	if porID["st-muerto"].IsAlive {
		t.Errorf("st-muerto debe estar muerto tras %d fallos", db.DeadFailThreshold)
	}
}

func TestMarkBatchVacioNoFalla(t *testing.T) {
	_, stRepo := openStreamTestRepos(t)
	if err := stRepo.MarkBatch(context.Background(), nil); err != nil {
		t.Errorf("MarkBatch(nil) = %v, quiero nil", err)
	}
}
```

- [x] **Step 2: Correr los tests y verlos fallar**

Run: `go test ./internal/adapters/db/ -run TestMarkBatch -v`
Esperado: FAIL — `MarkBatch` no existe.

- [x] **Step 3: Definir el tipo en ports**

En `gateway/internal/ports/stream_repository.go`:

```go
// StreamHealth es el resultado de un chequeo de salud listo para persistir.
type StreamHealth struct {
	StreamID  string
	IsAlive   bool
	LatencyMs int64
}

type StreamRepository interface {
	// ... métodos existentes ...

	// MarkBatch aplica todos los resultados en una sola transacción. El worker
	// chequea ~12k streams por pasada: hacerlo con un UPDATE suelto por stream
	// son ~12k transacciones implícitas y otros tantos fsync.
	MarkBatch(ctx context.Context, resultados []StreamHealth) error
}
```

- [x] **Step 4: Implementar `MarkBatch`**

En `gateway/internal/adapters/db/stream_repository.go`:

```go
func (r *SQLiteStreamRepository) MarkBatch(ctx context.Context, resultados []ports.StreamHealth) error {
	if len(resultados) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db.Stream.MarkBatch (BeginTx): %w", err)
	}
	defer tx.Rollback() //nolint:errcheck — Rollback es no-op si Commit tuvo éxito

	stmtVivo, err := tx.PrepareContext(ctx,
		`UPDATE streams
		 SET is_alive = 1, fail_count = 0, latency_ms = ?, last_checked = ?, updated_at = ?
		 WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("db.Stream.MarkBatch (Prepare vivo): %w", err)
	}
	defer stmtVivo.Close()

	stmtMuerto, err := tx.PrepareContext(ctx,
		`UPDATE streams
		 SET fail_count   = fail_count + 1,
		     is_alive     = CASE WHEN fail_count + 1 >= ? THEN 0 ELSE is_alive END,
		     last_checked = ?,
		     updated_at   = ?
		 WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("db.Stream.MarkBatch (Prepare muerto): %w", err)
	}
	defer stmtMuerto.Close()

	now := time.Now().Unix()
	for _, res := range resultados {
		if res.IsAlive {
			_, err = stmtVivo.ExecContext(ctx, res.LatencyMs, now, now, res.StreamID)
		} else {
			_, err = stmtMuerto.ExecContext(ctx, DeadFailThreshold, now, now, res.StreamID)
		}
		if err != nil {
			return fmt.Errorf("db.Stream.MarkBatch (Exec id=%s): %w", res.StreamID, err)
		}
	}

	return tx.Commit()
}
```

- [x] **Step 5: Usarlo desde el worker**

En `gateway/internal/adapters/validator/worker.go`, sustituir el bucle de `checkOnce` que llama a `MarkAlive`/`MarkDead` uno a uno:

```go
	var alive, dead int
	resultados := make([]ports.StreamHealth, 0, len(streams))
	for res := range w.validator.Start(ctx, urls) {
		for _, id := range byURL[res.URL] {
			resultados = append(resultados, ports.StreamHealth{
				StreamID:  id,
				IsAlive:   res.IsAlive,
				LatencyMs: res.LatencyMs,
			})
			if res.IsAlive {
				alive++
			} else {
				dead++
			}
		}
	}

	// Una transacción para toda la pasada: con MaxOpenConns(1), cada UPDATE
	// suelto es tiempo en el que la API no puede leer.
	if err := w.repo.MarkBatch(ctx, resultados); err != nil {
		w.logger.Error("Health-check: fallo persistiendo resultados", slog.Any("error", err))
		return
	}
```

Añadir el import de `ports` si falta.

- [x] **Step 6: Correr los tests y verlos pasar**

Run: `go test ./internal/adapters/db/ ./internal/adapters/validator/ -race -v`
Esperado: PASS. Actualizar los fakes de `StreamRepository` en tests para que implementen `MarkBatch`.

- [x] **Step 7: Medir la mejora**

```bash
cd ~/Dev/ip-tv/gateway
go build -o server ./cmd/server
HEALTH_INTERVAL=3m ./server > /tmp/gw.log 2>&1 &
sleep 260
grep "Health-check completado" /tmp/gw.log
pkill -f "./server"
```

Esperado: la pasada completa. Comparar con el tiempo previo si se anotó; el objetivo es que la fase de escritura deje de ser perceptible.

- [x] **Step 8: Suite completa y commit**

```bash
go test -race -count=1 ./... && gofmt -l . && go vet ./...
git add gateway/
git commit -m "gateway: escrituras del health-check en una sola transacción"
```

---

### Task 19: Pool de lectura separado para los handlers

**Files:**
- Modify: `gateway/internal/adapters/db/db.go`
- Modify: `gateway/cmd/server/main.go`
- Modify: `gateway/internal/api/router.go`
- Test: `gateway/internal/adapters/db/db_test.go`

**Interfaces:**
- Consumes: los PRAGMAs por DSN de la Task 4 — sin ellos, una segunda conexión arrancaría sin `foreign_keys`.
- Produces: `db.OpenReadOnly(path string) (*sql.DB, error)`, con `MaxOpenConns(4)` y `_txlock=deferred`. `Open` sigue siendo el pool de escritura con `MaxOpenConns(1)`.

**Contexto:** medido: con una transacción de escritura abierta 800 ms, un `QueryRowContext` concurrente se bloqueó **760 ms** (`sql.DBStats.WaitDuration: 750ms`). `SetMaxOpenConns(1)` descarta el motivo entero de WAL, que es precisamente permitir lectores concurrentes durante una escritura. Un `SaveBatch` de 13 859 canales ocupa la conexión **167 ms** en exclusiva.

**Nota de alcance:** esta tarea es la de mayor riesgo del plan porque cambia la topología de conexiones. Hacerla al final y con la suite en verde. Si aparece cualquier `SQLITE_BUSY`, `busy_timeout(5000)` de la Task 4 debería absorberlo; si no, revertir y replantear.

- [x] **Step 1: Escribir el test que falla**

Añadir a `gateway/internal/adapters/db/db_test.go`:

```go
// WAL existe para que los lectores no esperen al escritor. Con un único pool
// de una conexión, cualquier lectura se encola detrás de la escritura en curso.
func TestLecturaNoSeBloqueaDetrasDeUnaEscritura(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	escritura, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer escritura.Close()

	lectura, err := db.OpenReadOnly(path)
	if err != nil {
		t.Fatalf("db.OpenReadOnly: %v", err)
	}
	defer lectura.Close()

	ctx := context.Background()
	tx, err := escritura.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO channels (id, name, provider_id, provider_type, is_adult, created_at, updated_at, last_seen_at)
		 VALUES ('x', 'X', 'opensource', 'opensource', 0, 0, 0, 0)`); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	inicio := time.Now()
	var n int
	if err := lectura.QueryRowContext(ctx, "SELECT COUNT(*) FROM channels").Scan(&n); err != nil {
		t.Fatalf("lectura concurrente: %v", err)
	}
	transcurrido := time.Since(inicio)

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if transcurrido > 200*time.Millisecond {
		t.Errorf("la lectura tardó %v con una escritura abierta; WAL debería permitirla sin esperar", transcurrido)
	}
}
```

- [x] **Step 2: Correr el test y verlo fallar**

Run: `go test ./internal/adapters/db/ -run TestLecturaNoSeBloquea -v`
Esperado: FAIL con `undefined: db.OpenReadOnly`.

- [x] **Step 3: Implementar `OpenReadOnly`**

En `gateway/internal/adapters/db/db.go`:

```go
// OpenReadOnly abre un pool de solo lectura sobre la misma DB. En WAL los
// lectores no bloquean al escritor ni viceversa, pero el pool de escritura
// está limitado a una conexión para evitar SQLITE_BUSY; si los handlers
// compartieran ese pool, cada lectura se encolaría detrás de la escritura en
// curso (medido: 760 ms de espera tras una transacción de 800 ms).
//
// Asume que Open ya creó y migró la DB.
func OpenReadOnly(path string) (*sql.DB, error) {
	dsn := "file:" + path +
		"?mode=ro" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=cache_size(-32000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db.OpenReadOnly: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db.OpenReadOnly (Ping): %w", err)
	}
	return db, nil
}
```

- [x] **Step 4: Correr el test y verlo pasar**

Run: `go test ./internal/adapters/db/ -run TestLecturaNoSeBloquea -v`
Esperado: PASS, con la lectura resolviéndose en pocos milisegundos.

- [x] **Step 5: Cablear en main**

En `gateway/cmd/server/main.go`, dentro de `run`, tras abrir la DB de escritura:

```go
	lecturaDB, err := db.OpenReadOnly(dbPath)
	if err != nil {
		return fmt.Errorf("abriendo el pool de lectura: %w", err)
	}
	defer lecturaDB.Close()
```

Los repositorios que solo leen desde los handlers pasan a usar `lecturaDB`; los que escriben (los que consumen el syncer y el health-worker) siguen con `sqlDB`. En concreto, construir **dos** juegos de repositorios:

```go
	// Escritura: los usan syncer y health-worker.
	channelRepo := db.NewChannelRepository(sqlDB)
	streamRepo := db.NewStreamRepository(sqlDB)
	epgRepo := db.NewEPGRepository(sqlDB)

	// Lectura: los usan los handlers HTTP.
	channelRepoRO := db.NewChannelRepository(lecturaDB)
	streamRepoRO := db.NewStreamRepository(lecturaDB)
	epgRepoRO := db.NewEPGRepository(lecturaDB)

	handler := api.NewRouter(logger, channelRepoRO, provider, streamRepoRO, epgRepoRO, lecturaDB, syncer)
```

- [x] **Step 6: Verificar de punta a punta**

```bash
cd ~/Dev/ip-tv/gateway
go build -o server ./cmd/server && ./server > /tmp/gw.log 2>&1 &
sleep 3
# Martillear /channels mientras corre el sync inicial
for i in $(seq 1 40); do
  curl -s -o /dev/null -w "%{http_code} %{time_total}\n" "http://127.0.0.1:8080/channels?limit=100"
done
grep -i "busy\|locked" /tmp/gw.log || echo "sin SQLITE_BUSY"
pkill -f "./server"
```

Esperado: todas las respuestas 200, tiempos por debajo de ~50 ms incluso durante el sync, y ningún `SQLITE_BUSY` en el log.

- [x] **Step 7: Suite completa y commit**

```bash
go test -race -count=1 ./... && gofmt -l . && go vet ./...
git add gateway/
git commit -m "gateway: pool de solo lectura para los handlers, separado del de escritura"
```

---

## Fuera de alcance de este plan

Tres cosas del informe de auditoría **no** están aquí porque necesitan una decisión de diseño previa, no una implementación mecánica. Cada una merece su propio plan:

1. **Empaquetado y distribución.** Decidir entre compilar el gateway dentro del proceso Flutter como c-archive (viable porque `modernc.org/sqlite` es Go puro, sin cgo) o empaquetarlo como login-item con `SMAppService`. Conviene resolverlo **antes** de la Fase 9 del roadmap, porque esa fase multiplica la decisión por tres plataformas más. Incluye mover `DB_PATH` a `~/Library/Application Support/`.

2. **Parser de EPG y `EPG_URL`.** El parser descarta en silencio las entradas con fechas que no encajan en sus dos layouts, y trata las fechas sin offset como UTC cuando XMLTV las define como hora local del emisor. Arreglarlo solo tiene sentido junto con la decisión de qué fuente XMLTV usar de verdad — hoy la guía está apagada por defecto.

3. **Autenticación y CORS.** `Access-Control-Allow-Origin: *` sin auth significa que cualquier web que el usuario visite mientras el gateway corre puede leer la API entera desde su navegador. Mientras el bind sea loopback el riesgo es acotado, pero `LISTEN_ADDR=0.0.0.0:8080` lo convierte en un servicio de LAN sin autenticación con un cambio de una variable. La decisión (origen concreto en vez de comodín, o token obligatorio para binds no-loopback) va ligada a la de empaquetado.

Del resto del informe, quedan sin abordar por ser de bajo impacto: memoria de la lista acumulada y logos sin `cacheWidth`, refetch del EPG al hacer scroll, ventana de la guía que se queda rancia, migración no transaccional, `sqflite` para favoritos (Fase 8 del roadmap), y accesibilidad más allá de `SignalBars`.


---

## Desviaciones respecto al plan (ejecución 2026-08-08)

Lo que cambió al ejecutarlo, y por qué:

1. **Carrera de datos encontrada al añadir `-race` al CI** (Task 2). El contador
   del handler `httptest` en `TestHLSChecker_CheckBatch_ConcurrentAndBounded`
   se incrementaba sin sincronizar desde una goroutine por conexión. Arreglado
   con `atomic.Int64`. Al asertarlo salió que el checker hace 2 peticiones por
   URL (playlist + primer segmento), dato que luego sirvió para decidir la
   Task 16.

2. **La histéresis no surtía efecto** (descubierto midiendo tras la Task 6).
   `is_alive` arranca en 0, así que un stream que nunca había estado vivo
   seguía ocultándose a su primer fallo. El filtro `AliveOnly` pasó a preguntar
   si el canal está *probado muerto* (`is_alive = 1 OR fail_count < 3`).
   Commit extra `744fa81`. Impacto: 11389 → 12633 canales visibles.

3. **Suelo de cordura movido a la Task 7**, junto con la poda, en vez de ser
   una mejora independiente: sin él, un sync degenerado habría hecho que la
   poda borrase el catálogo entero.

4. **`migrate()` se rompió con un comentario propio** (Task 16). Trocea
   `schema.sql` por `;` y el `);` dentro de la prosa de un comentario partía la
   sentencia. Añadido `stripSQLComments` con test de regresión — el problema de
   migración frágil que la auditoría marcaba como "bajo impacto" resultó ser
   real.

5. **`PlayerScreen` sigue sin test de widget** (Task 10). `media_kit` exige el
   framework nativo de mpv, que no existe headless ni en los runners Linux de
   CI. El arreglo (armar el watchdog antes del fetch + token de generación) se
   aplicó igualmente; el contrato del guard sí está cubierto en
   `playback_guard_test.dart`.

6. **Tests de timeout con deadline inyectable** (Task 9). La primera versión
   esperaba los 10 s reales tres veces, 30 s añadidos a cada run de CI. Los
   repositorios aceptan ahora un `timeout` opcional.

7. **El test de la carrera de `loadMore` se verificó por mutación** (Task 11).
   La primera versión pasaba con y sin el arreglo, porque el query `'Par'`
   casaba también con `'Impar'` y no discriminaba nada. Cambiado a `'Impar'` y
   comprobado que falla al quitar el guard.

8. **`healthcheck` borrado** (Task 16, opción (a) del plan). Su `HLSChecker` es
   el que cumple el spec de la Fase 7.1, pero son 2 peticiones × 12.6k streams
   = 25k peticiones por pasada horaria contra servidores públicos gratuitos.
   Con la histéresis ya en su sitio, el chequeo a nivel de playlist da señal
   suficiente. Queda en el historial de git.
