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

## Distribución (P1, cerrado 2026-08-28)

- **Cask, no fórmula.** `.goreleaser.yml` usa `homebrew_casks`; `brews` está
  deprecado y hacía fallar `goreleaser check`. Consecuencia real: los casks son
  **solo macOS**, así que `brew install` en Linux ya NO está cubierto — para
  Linux está `install.sh`, que detecta sistema/arquitectura y verifica checksum.
  goreleaser genera bloques `on_linux` en el cask, pero Homebrew no soporta
  `--cask` ahí: no te fíes de ellos.
- **La credencial del tap es una DEPLOY KEY, no un PAT.** GitHub no tiene API
  para emitir PATs, pero sí para deploy keys, y una deploy key es menos
  privilegio (una sola repo, sin acceso a la cuenta, sin caducidad). Vive como
  secret `HOMEBREW_TAP_SSH_KEY`; el workflow la materializa en `$RUNNER_TEMP`
  con permisos 600 porque **goreleaser espera la RUTA, no el contenido**.
- **`{{ .KeyPath }}` NO existe** en el esquema ni en la documentación de
  goreleaser, aunque ande por ejemplos sueltos. Se usa
  `{{ .Env.HOMEBREW_TAP_KEY_PATH }}`. Importa: esa línea solo corre DESPUÉS de
  empujar el tag, y un fallo ahí deja una release a medias contra una etiqueta
  ya publicada.
- Rotar la clave: `gh repo deploy-key add … --allow-write` sobre
  `gdberysan/homebrew-tap` + `gh secret set`. `gh` con scope `repo` basta.

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
- **Las pruebas NO ven maquetación.** `astro check`/`svelte-check` y toda la
  suite pasan con un CSS roto: en el sitio hermano (korven) se desplegó un
  televisor cortado por la mitad con 743 pruebas en verde. Cualquier cambio de
  CSS se comprueba midiendo geometría en un navegador de verdad (anchos,
  desbordes, opacidad efectiva), no solo con los gates. Y **nunca editar CSS
  con expresiones regulares**: una borró una declaración y su llave de cierre,
  fundiendo dos reglas sin que nada se quejara.
- **TRAMPA de gate visual en Chrome:** una pestaña abierta cachea el bundle
  viejo (SPA en memoria). Verifica el `index-<hash>.js` que sirve el gateway
  (`curl -s :8080/ | grep index-`) y recarga con un query cache-buster
  (`?fresh=xyz`). «Connection refused / Could not reach Open TV» = gateway
  caído, no un bug.

## Estado y hoja de ruta

Ver `MEMORY.md` (personal, se carga por sesión). **Al 2026-08-28: los cinco
bloqueadores de lanzamiento están CERRADOS y pusheados** (cask válido,
`--version`, `SECURITY.md`, tap creado, credencial puesta), y el repo tiene ya
`CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` y plantillas de issue/PR. Quedan solo
los dos pasos irreversibles —tag `v1.0.0` y el flip— que son del usuario.
Resumen previo: **P0–P2 mergeadas y
pusheadas al remoto PRIVADO** (cliente web embebido, fiabilidad/failover, sala
de control UX, fuentes bring-your-own, estado del arte, guía EPG por fuente).
**Reproductor-primero (layout 1b) HECHO, mergeado y PUSHEADO:** escenario con
vídeo persistente + catálogo lateral, cambio de canal en el sitio, modo «ver
todo» conservando la rejilla P0.6, entrada muted con CTA de sonido. El flip
público está **desbloqueado por el abogado**; el tap y su credencial ya están
puestos, así que solo faltan los dos pasos irreversibles (tag y flip).

**Pasada de Safari REAL hecha** (por `safaridriver`/WebDriver, no el WebKit de
Playwright): reproducción, ⌘K con trap/`inert`/Esc y landmarks, todo limpio.
De ahí salió un hallazgo que sigue vivo: **Safari 26.6 devuelve `'maybe'`, no
`'probably'`**, a `canPlayType('application/vnd.apple.mpegurl')`, así que la
rama nativa de `web/src/reproductor/plan.ts` no se toma en un Safari de hoy
(va por hls.js, `src` de tipo blob). NO se borró: los Safari anteriores sí
dicen `'probably'` y ahí el nativo es mejor. Al probar en Safari por WebDriver
hay que usar **clic real de WebDriver**: un `click()` inyectado por JS no es
gesto de usuario y Safari bloquea el autoplay.

**AirPlay en el cliente web: HECHO y verificado contra un Apple TV real
(2026-09-04, `11f096b`, mergeado en main local, NO pusheado).** El botón 📺
del reproductor ya emite vídeo y audio de verdad. Causa raíz: AirPlay no
reproduce fuentes MSE/`blob:` (lo que da hls.js) — el motor se fuerza a
nativo (`<video src>`) SOLO durante una sesión de cast, en
`web/src/componentes/Reproductor.svelte`, reutilizando toda la máquina de
reproducción existente. Dos bugs reales de hardware aparecieron y se
cerraron con evidencia (trace real vía `window.__airplayDebug` +
`osascript … do JavaScript`, no con hipótesis sin probar — ver
`verificar_antes_de_arreglar.md` en memoria): un bucle de reconexión sin fin
(arreglado preparando el motor nativo AL PULSAR el botón, no al reaccionar
al evento de ruta) y el selector de AirPlay cancelado dejando la app
convencida de estar emitiendo (arreglado con un timeout de 45 s). Detalle
técnico completo y qué queda por probar (Task 5 del plan, mayormente
pendiente aún) en la memoria del proyecto. Spec:
`docs/superpowers/specs/2026-09-03-airplay-cast-web-design.md`. Plan:
`docs/superpowers/plans/2026-09-03-airplay-cast-web.md`.

Specs en cola (gate de revisión): **DVR record-now** (su ● Grabar
vive en el panel persistente ya creado) → **Chromecast** (spec propio, no
comparte código con AirPlay) → **framecapture**.
