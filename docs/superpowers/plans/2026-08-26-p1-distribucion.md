# P1 — Distribución (scrub, licencia, release, historia) — Plan de implementación

> **Para trabajadores agénticos:** SUB-SKILL REQUERIDA: usa superpowers:subagent-driven-development para ejecutar este plan tarea a tara. Los pasos usan checkbox (`- [ ]`).

**Goal:** Dejar Open TV listo para el flip público — identidad única en la historia, sin PII en el árbol, con un `scrubcheck` que lo garantice, LICENSE+marca+CHANGELOG+README de producto, y automatización de release (goreleaser+tap+install.sh) — SIN hacer aún el `tag v1.0.0` ni el flip público (ambos GATED, decisión del usuario tras lectura de abogado).

**Architecture:** Repo Go a la raíz (`github.com/gdberysan/open-tv`, entrypoint `cmd/open-tv`) que sirve el cliente Svelte embebido. La distribución se añade como capa nueva: un tool Go `tools/scrubcheck`, ficheros de release en la raíz, y un workflow `release.yml`. La reescritura de identidad usa `git filter-repo --mailmap` sobre el repo privado.

**Tech Stack:** Go 1.26, git-filter-repo, goreleaser, GitHub Actions, bash (install.sh).

**Spec:** `docs/superpowers/specs/2026-08-22-open-tv-web-product-design.md` (§6.1 scrub, §6.2 release, §6.3 onboarding, §10 secuencia). La spec es la autoridad.

## Global Constraints

- **Idioma español** en artefactos del repo (código, commits, docs); README y CHANGELOG **español primero, inglés después** con cabeceras simétricas (spec §6.2).
- **`mobile/` CERO diffs.** Contrato Flutter intocable. Ninguna dependencia Go/JS nueva en el runtime (las herramientas de release —goreleaser, git-filter-repo— son externas, no dependencias del módulo).
- **Gates verdes al final de CADA tarea de código:** `gofmt -l .`·`go vet ./...`·`go build ./...`·`go test -race -count=1 ./...`; con `web/`: `cd web && npm run check && npm test && npm run build`; con `mobile/`: `flutter analyze && flutter test`.
- **Identidad `gdberysan@gmail.com`** en cada commit NUEVO (verificar `git config user.email` antes de commitear); trailer `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- **No `git add -A`** (untracked ajenos: `docs/prompts/`, `open-tv-interface-design/`, `docs/superpowers/plans/2026-08-25-baseline-de-seguridad.md`, el binario `open-tv`).
- **Rama de trabajo `feat/p1-distribucion` desde `main` (`fd5fb85`).** El merge a main es decisión del controlador; el force-push y el flip son GATES.
- **GATES DUROS (parar y NO ejecutar sin autorización explícita en esta sesión):** el `tag v1.0.0` y el **flip público** están FUERA de este plan (van tras la lectura de abogado). La Tarea 6 (scrub de identidad + force-push al remoto PRIVADO) SÍ está autorizada por el usuario en esta sesión, pero se ejecuta con copia de seguridad y confirmación del force-push.
- **Alcance del scrub de contenido = el ÁRBOL versionado** (spec §6.1: "lo que la gente lee es el árbol"). Los blobs históricos de contenido NO se reescriben; solo la IDENTIDAD (autor/committer) se normaliza en toda la historia vía mailmap.

---

### Task 1: Higiene del repo + correcciones de scrub conocidas (en el árbol)

**Files:**
- Modify: `.gitignore` (añadir `docs/prompts/`)
- Create: `.gitattributes` (export-ignore de higiene)
- Modify: `.claude/settings.json` (rutas absolutas con el usuario del Mac → relativas, o retirar del seguimiento)
- Modify: `docs/superpowers/specs/2026-08-08-korven-open-tv-design.md`, `docs/superpowers/plans/2026-08-08-korven-open-tv.md`, `docs/superpowers/plans/2026-08-08-fiabilidad-iptv.md`, `docs/superpowers/plans/2026-08-25-p0-cliente-web-y-binario.md`, `docs/superpowers/plans/2026-08-25-p0.5-fiabilidad-y-robustez.md` (rutas absolutas `/Users/usuario/...` → referencia genérica)
- Modify or Delete: `docs/risks.md` (pre-v3 desactualizado → reescribir a la realidad o borrar), `mobile/README.md` (plantilla de Flutter → **NO tocar: `mobile/` cero diffs; en su lugar añadir `mobile/README.md` a `.gitattributes export-ignore`** o dejarlo, es plantilla estándar de Flutter y no lleva PII)

**Interfaces:**
- Produces: un árbol sin rutas absolutas con el usuario del Mac ni PII conocida; consumido por Task 2 (scrubcheck debe pasar sobre este árbol).

- [ ] **Step 1:** `git grep -l "/Users/usuario"` para enumerar los hits actuales. Para cada fichero de `docs/`, sustituir la ruta absoluta por una referencia genérica (p. ej. `~/Dev/<carpeta-de-diseño>` o "el design system de Korven"). Para `.claude/settings.json`: si lleva rutas absolutas en permisos, reescribirlas como relativas al proyecto; si eso no es viable, `git rm --cached .claude/settings.json` y añadir `.claude/` a `.gitignore` (la config local no debe viajar en un repo público).
- [ ] **Step 2:** Añadir `docs/prompts/` a `.gitignore` (ya está `.claude/settings.local.json`; añadir `docs/prompts/` y, si se decidió, `.claude/`).
- [ ] **Step 3:** Crear `.gitattributes` con `export-ignore` para `docs/prompts/`, `.claude/`, `.gitattributes` (higiene del tarball; no afecta a lo que se lee en el árbol).
- [ ] **Step 4:** `docs/risks.md`: leerlo; si está desactualizado (pre-v3), reescribirlo a la realidad actual del proyecto o borrarlo con `git rm`.
- [ ] **Step 5:** Gates verdes (Go+web+mobile como arriba; `mobile/` cero diffs — verificar `git diff --stat -- mobile/` vacío). Commit `chore(scrub): rutas relativas y gitignore/gitattributes de higiene`.

### Task 2: `tools/scrubcheck` (Go) + denylist privada + wiring en CI

**Files:**
- Create: `tools/scrubcheck/main.go`, `tools/scrubcheck/scrub.go`, `tools/scrubcheck/scrub_test.go`
- Modify: `.github/workflows/ci.yml` (job/step que corre scrubcheck)
- Create (FUERA del repo, no versionado): `~/.config/korven/open-tv-denylist.txt`

**Interfaces:**
- Consumes: el árbol limpio de Task 1.
- Produces: binario `scrubcheck` con contrato de salida `0` limpio / `1` hallazgos / `2` sin lista (como CVForge `lib/release/scrub.ts`).

- [ ] **Step 1 (test primero):** `scrub_test.go` con casos: (a) `findDenied` (matcher) — término ≥3 chars, case-insensitive, substring; ignora términos <3; encuentra hit en contenido; (b) el escáner falla ante fichero versionado `.DS_Store`, `*.db`, cualquier ruta bajo `docs/prompts/`, `.claude/settings.local.json`; (c) exit 2 si no hay denylist. Correr → falla (no existe).
- [ ] **Step 2:** Implementar `scrub.go`: `findDenied(files []Fichero, terms []string) []Hit` — normaliza términos (`strings.TrimSpace`+`ToLower`, filtra `len<3`), match por `strings.Contains(strings.ToLower(content), term)`. Y `escanearArbol(ref string) ([]Fichero, error)` vía `git ls-files` (no `git archive`).
- [ ] **Step 3:** `main.go`: lee `~/.config/korven/open-tv-denylist.txt` (path por env `KORVEN_DENYLIST` con default a `~/.config/korven/open-tv-denylist.txt`); si no existe → exit 2 con mensaje. Escanea el árbol de `HEAD`; añade las reglas duras (DS_Store/*.db/docs/prompts/settings.local.json versionados). Imprime hallazgos (fichero + regla/término, SIN imprimir el término de la denylist si es PII — imprime solo el fichero y "término de la lista privada"). exit 1 si hay hallazgos, 0 si limpio.
- [ ] **Step 4:** Crear la denylist privada `~/.config/korven/open-tv-denylist.txt` con: `correo-privado@example.com`, `usuario` (usuario del Mac), y cualquier teléfono/ruta privada conocida. UNA línea por término. (Este fichero NO se versiona.)
- [ ] **Step 5:** Correr `go run ./tools/scrubcheck` sobre el árbol tras Task 1 → debe salir 0 (limpio). Si sale 1, corregir el hit en el árbol y re-correr.
- [ ] **Step 6:** Wire en `ci.yml`: un step que instala nada (Go ya está) y corre `scrubcheck`, pero con una denylist de CI mínima (la privada no está en CI) — o un modo `--reglas-duras-solo` que corre solo las reglas de ficheros prohibidos (DS_Store/db/prompts) sin la denylist privada, para que CI no exija el fichero privado. Decidir: CI corre `scrubcheck --solo-reglas-duras` (exit 0/1, nunca 2); el escaneo con denylist es un gate LOCAL del autor pre-release.
- [ ] **Step 7:** Gates verdes. Commit `feat(release): tool scrubcheck (árbol + reglas duras + denylist privada)`.

### Task 3: LICENSE (MIT) + marca + CHANGELOG

**Files:**
- Create: `LICENSE` (MIT, © Gerard Bernal / Korven), `assets/brand/LICENSE` (aviso de marca), `CHANGELOG.md` (Keep a Changelog, español)

- [ ] **Step 1:** `LICENSE`: texto MIT estándar, titular `Gerard Bernal (Korven)`, año 2026.
- [ ] **Step 2:** `assets/brand/LICENSE`: aviso de que el nombre "Korven", el wordmark, el emblema y el icono son © Korven, todos los derechos reservados, NO cubiertos por MIT; los forks usan su propio nombre y emblema.
- [ ] **Step 3:** `CHANGELOG.md` formato Keep a Changelog en español, con una entrada `[No publicado]` que resuma P0–P0.8 (cliente web embebido, fiabilidad/failover, sala de control UX, fuentes bring-your-own, estado del arte) hacia un futuro `[1.0.0]`.
- [ ] **Step 4:** Commit `docs(release): LICENSE MIT + aviso de marca + CHANGELOG`.

### Task 4: README como documentación de producto (ES + EN)

**Files:** Modify `README.md` (reescritura completa según spec §6.2)

**Interfaces:** Consumes la línea de instalación que Task 5 define (`install.sh` URL, `brew install`, `go install`); coordinar los comandos exactos.

- [ ] **Step 1:** Reescribir `README.md` español primero, inglés después, cabeceras simétricas: qué es · dónde verlo (web) · instalar (brew / línea de instalación / binario / `go install github.com/gdberysan/open-tv/cmd/open-tv@latest`) · primer arranque · privacidad (sin cuentas/telemetría/relay) · lo que no hace (la postura, FTA) · preguntas frecuentes · comandos y variables (incluye `DB_PATH`, `--no-browser`, `serve`, y las var de entorno reales) · licencia y marca · soporte (Issues) · "para desarrolladores" (incluida la app de macOS congelada en `mobile/` y cómo compilarla). Para macOS descargado por navegador: documentar con honestidad `xattr -d com.apple.quarantine` y "Abrir de todas formas"; señalar brew y el instalador como caminos limpios.
- [ ] **Step 2:** Verificar que los comandos/variables que documenta EXISTEN (leer `cmd/open-tv` y los flags reales; no inventar).
- [ ] **Step 3:** Commit `docs(release): README como documentación de producto (ES/EN)`.

### Task 5: goreleaser + install.sh + release.yml

**Files:**
- Create: `.goreleaser.yml`, `install.sh`, `.github/workflows/release.yml`

**Interfaces:** Consumes el módulo/entrypoint (`cmd/open-tv`) y el embed (el web dist debe construirse ANTES del build Go en el workflow).

- [ ] **Step 1:** `.goreleaser.yml`: versión desde el tag; builds `darwin/arm64+amd64`, `linux/amd64+arm64`, `windows/amd64`; `main: ./cmd/open-tv`; `checksums.txt`; release notes en español desde el changelog; brew formula empujada a `gdberysan/homebrew-tap` (bajo una clave `brews:` — NOTA: el push real al tap ocurre solo al hacer release con tag = GATED; la config puede existir sin disparar nada). Validar con `goreleaser check`.
- [ ] **Step 2:** `install.sh`: detecta SO/arquitectura, descarga el asset de la última release con curl, verifica el checksum de `checksums.txt`, instala en `~/.local/bin` (o `/usr/local/bin` si escribible), lo arranca una vez. Documentar Windows (descarga+doble clic, SmartScreen). `shellcheck` limpio.
- [ ] **Step 3:** `release.yml`: se dispara al empujar un tag `v*`; jobs: build web (`cd web && npm ci && npm run build`) → build Go (embed ya presente) → `goreleaser release`. Token del tap en secrets (`HOMEBREW_TAP_TOKEN`). **El workflow NO se dispara en este plan** (no se empuja tag); solo se versiona el fichero.
- [ ] **Step 4:** `goreleaser check` (si goreleaser está instalado; si no, instalarlo con `brew install goreleaser` como parte de esta tarea) y `shellcheck install.sh` verdes. Commit `feat(release): goreleaser + install.sh + workflow de release`.

### Task 6: [AUTORIZADA] Scrub de identidad (`git filter-repo --mailmap`) + force-push al remoto PRIVADO

**Files:** Create (temporal, no versionado): `mailmap` (fichero de mapeo)

**PRE-REQUISITO:** Tasks 1–5 mergeadas a `main`. Este paso reescribe TODA la historia (SHAs cambian). Es DESTRUCTIVO pero autorizado por el usuario en esta sesión (repo privado, sin otros contribuidores). **Copia de seguridad obligatoria antes.**

- [ ] **Step 1:** Instalar la herramienta: `brew install git-filter-repo` (verificar `git filter-repo --version`).
- [ ] **Step 2:** **Copia de seguridad:** `git bundle create ../open-tv-backup-pre-scrub.bundle --all` (recuperación total si algo sale mal) y anotar el HEAD actual.
- [ ] **Step 3:** Crear el fichero `mailmap` con:
  ```
  Gerard <gdberysan@gmail.com> <correo-privado@example.com>
  Gerard <gdberysan@gmail.com> Gerard Bernal <gdberysan@gmail.com>
  ```
  (dependabot[bot] NO se toca.)
- [ ] **Step 4:** Correr `git filter-repo --mailmap mailmap --force` sobre el repo. Verificar con `git shortlog -sne --all` que solo quedan `Gerard <gdberysan@gmail.com>` y `dependabot[bot]` (0 `correo-privado`, 0 `Gerard Bernal`).
- [ ] **Step 5:** filter-repo retira el remoto por seguridad; re-añadir: `git remote add origin https://github.com/gdberysan/open-tv.git`.
- [ ] **Step 6 (GATE de confirmación):** Antes del force-push, verificar que es el remoto PRIVADO correcto y presentar al usuario el resumen (SHAs nuevos, identidades normalizadas). **Force-push:** `git push --force-with-lease origin main` (o `--force` si lease no aplica por historia divergente). Confirmar que `origin/main` refleja la historia reescrita.
- [ ] **Step 7:** Verificar `scrubcheck` (con denylist privada) sale 0 sobre el árbol final. Actualizar el ledger y la memoria.

---

## Self-review

**Cobertura de la spec §6/§10 (P1):** reescritura de historia→T6; scrubcheck+denylist→T2; correcciones de scrub conocidas→T1; LICENSE+marca+CHANGELOG→T3; README de producto→T4; goreleaser+tap+install.sh+release.yml→T5. **FUERA de alcance (GATED, no en este plan):** `tag v1.0.0` y **flip público** (tras lectura de abogado); creación/push real del tap y disparo de release (ocurren con el tag). Notarización Apple: diferida por la spec (no es P1).

**Consistencia de tipos/interfaces:** `findDenied`/exit-codes de T2 espejan CVForge; la línea de instalación de T5 (`install.sh` URL, `go install .../cmd/open-tv@latest`, `brew install gdberysan/tap/open-tv`) debe coincidir letra por letra con lo que T4 documenta en el README — coordinar. El mailmap de T6 cubre las 2 identidades no canónicas verificadas hoy (`correo-privado@example.com` ×96, `Gerard Bernal` ×9).

**Placeholders:** ninguno — los valores exactos (módulo, entrypoint, mapeos de mailmap, exit codes, targets de build) van verbatim.

**Orden:** T1→T2 (scrubcheck valida el árbol de T1) →T3→T4↔T5 (README y comandos coordinados) → merge a main → T6 (scrub sobre la historia final + force-push). T6 al final porque reescribe SHAs de todo lo anterior.
