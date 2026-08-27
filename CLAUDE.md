# CLAUDE.md — guía del proyecto para sesiones de Claude Code

> Este fichero se carga automáticamente cada sesión. Es la orientación del
> PROYECTO (arquitectura, convenciones, gates, invariantes). El estado y la
> hoja de ruta viven en la memoria personal (`MEMORY.md` y sus ficheros de
> detalle), que también se carga por sesión.

## Qué es

**Korven Open TV** — reproductor de televisión abierta (FTA) desde listas M3U
públicas tipo iptv-org. **Un solo binario Go** que sirve un **cliente Svelte 5
embebido** (`go:embed`); la app **Flutter de macOS (`mobile/`) está CONGELADA**
y debe quedar **con CERO diffs**. Sin cuentas, sin telemetría, sin nube: todo
corre en la máquina del usuario. Instalación limpia arranca VACÍA
(bring-your-own): el usuario añade sus fuentes.

## Arquitectura

- Módulo Go: `github.com/gdberysan/open-tv` en la raíz. Entrypoint
  `cmd/open-tv`. Clean Architecture: `internal/domain` · `internal/ports`
  (interfaces) · `internal/adapters` (db SQLite modernc, providers, epg,
  validator) · `internal/services` (syncer, recorder-futuro) · `internal/api`
  (chi v5, handlers, middleware) · `internal/proxy` (proxy HLS **solo loopback**
  con guarda SSRF) · `internal/ui` (`go:embed all:dist`).
- Cliente: `web/` — Svelte 5 (runas `$state/$derived/$props/$effect`) + Vite +
  TS + `hls.js` (chunk perezoso). Se construye a `internal/ui/dist`. Stores en
  `web/src/estado/`, datos en `web/src/datos/` (interfaz `CatalogSource`),
  componentes en `web/src/componentes/`, i18n en `web/src/i18n/` (es fuente de
  verdad, en paridad).
- La DB va en el directorio de datos del sistema (`datadir`), salvo `DB_PATH`.
  Fuentes subidas en `<datadir>/fuentes/`.

## Convenciones

- **Idioma:** artefactos del repo (código, commits, UI, docs) en **español**;
  UI también en inglés con paridad. El chat con el usuario, en inglés.
- **Identidad de commits:** autor `Gerard <gdberysan@gmail.com>` (verificar
  `git config user.email` antes de commitear). Trailer obligatorio:
  `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>` (o el modelo en uso).
- **NUNCA `git add -A`.** Añade por ruta explícita. Untracked ajenos que NO se
  commitean: `docs/prompts/`, `open-tv-interface-design/`, el binario `open-tv`,
  `/dist/` (salida goreleaser), `.superpowers/` (scratch de SDD, gitignored).
- **NADA a `main` ni push sin autorización explícita del usuario.** El merge y
  el flip público son decisiones suyas.

## Gates (verdes al final de CADA tarea)

- **Go:** `gofmt -l .` · `go vet ./...` · `go build ./...` ·
  `go test -race -count=1 ./...` · **`golangci-lint run ./...`** (LECCIÓN de P1:
  incluir SIEMPRE golangci-lint en los gates Go — sin él coló deuda gosec/errcheck)
  · `go run ./tools/scrubcheck` (exit 0; sin PII en el árbol).
- **Web:** `cd web && npm run check && npm test && npm run build`.
- **Mobile:** `cd mobile && flutter analyze && flutter test` (solo para confirmar
  que sigue verde y con **CERO diffs**; no tocar ficheros de `mobile/`).

## Invariantes DUROS (no romper)

- **`mobile/` CERO diffs.** Contrato JSON congelado de Flutter: **15 claves** en
  `/channels` (`domain/channel_test.go` lo asevera). Cambios de backend = tablas
  y endpoints NUEVOS, aditivos; nunca alterar `/channels` ni `/sources`.
- **Ninguna dependencia Go/JS nueva** sin muy buena razón. **Bundle propio del
  cliente ≤ 80 KB gzip** (hls.js va en chunk perezoso aparte).
- **Seguridad:** proxy HLS **solo loopback** con guarda SSRF (`controlConexion`
  + `checkRedirect` + tope de tamaño); middleware `MismoOrigen` (Host +
  `Sec-Fetch-Site` en métodos mutantes) contra CSRF/DNS-rebinding; CSP estricta
  (`script-src 'self'`). Reutiliza el cliente HTTP guardado del proxy para
  cualquier fetch server-side de streams.
- **a11y (P0.6/P0.8, rework reproductor-primero):** 2 regiones sr-only
  PERSISTENTES de App como ÚNICOS anunciadores nuevos (NO añadir regiones
  aria-live); roving tabindex en las rejillas y en la lista lateral (con
  sincronía por `onfocus` — el foco puede llegar por clic); **el Reproductor
  es un PANEL persistente (`role=region`), NO un modal** — el único overlay
  con focus-trap + focus-restore + fondo `inert` es la paleta ⌘K; el modo
  ver-todo y las vistas-hash CUBREN el escenario con `hidden`, nunca lo
  desmontan (el `<video>` persiste). **`prefers-reduced-motion` anula TODA
  animación nueva.** WCAG 2.5.3 en controles con aria-label: el nombre
  accesible EMPIEZA por el texto visible.
- **Tokens de marca:** ámbar = señal-viva/activo/foco. Nada hardcodeado.

## Flujo de trabajo

- **subagent-driven-development (SDD):** un implementador fresco por tarea →
  review por tarea (spec + calidad) → fix-loops → review final de rama en el
  modelo más capaz (opus). Ledger en `.superpowers/sdd/<plan>/progress.md`.
  Specs en `docs/superpowers/specs/`, planes en `docs/superpowers/plans/`.
- **Brainstorming antes de construir** features nuevas (arquitectura): spec →
  gate de revisión del usuario → plan → build. No implementar sin spec aprobada.
- **Hook pre-commit de gitleaks** activo; workflow de seguridad en CI (gitleaks,
  golangci-lint, semgrep, govulncheck) en cada push. `scrubcheck` (tool propio)
  garantiza que no viaja PII al árbol público.

## Entorno de desarrollo

- El gateway corre bajo **launchd** (`dev.korven.opentv.gateway`) sirviendo
  `open-tv serve --no-browser` en `:8080` desde la raíz del repo.
- Tras cambiar el cliente: `cd web && npm run build`, luego
  `go build -o open-tv ./cmd/open-tv` desde la RAÍZ (ojo: `cd web` persiste
  entre comandos del shell), restaura `internal/ui/dist/.gitkeep` si el build lo
  borró, mata el proceso `open-tv serve` (launchd lo respawnea con el binario
  nuevo) y espera a que sirva el bundle nuevo.
- **TRAMPA de gate visual en Chrome:** una pestaña abierta cachea el bundle
  viejo (SPA en memoria). Verifica el `index-<hash>.js` que sirve el gateway
  (`curl -s :8080/ | grep index-`) y recarga con un query cache-buster
  (`?fresh=xyz`). «Connection refused / Could not reach Open TV» = gateway
  caído, no un bug.

## Estado y hoja de ruta

Ver `MEMORY.md` (personal, se carga por sesión). Resumen: **P0–P2 mergeadas y
pusheadas al remoto PRIVADO** (cliente web embebido, fiabilidad/failover, sala
de control UX, fuentes bring-your-own, estado del arte, guía EPG por fuente).
**Reproductor-primero (layout 1b) HECHO y mergeado en main LOCAL (sin push):**
escenario con vídeo persistente + catálogo lateral, cambio de canal en el
sitio, modo «ver todo» conservando la rejilla P0.6, entrada muted con CTA de
sonido. El flip público está **desbloqueado por el abogado** pero pendiente
de: tag `v1.0.0` (+ repo tap Homebrew + secret) y el flip, ambos decisión del
usuario. Specs en cola (gate de revisión): **DVR record-now** (su ● Grabar
vive en el panel persistente ya creado) → **casting AirPlay+Chromecast** →
**framecapture**.
