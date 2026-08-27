# Baseline de Seguridad (CI reforzado + workflow reutilizable) — Plan de Implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dar a todos los repos de gdberysan una capa de seguridad *aplicada por CI* (secretos, SAST, dependencias vulnerables) con open-tv como primer consumidor, sin servidores locales que mantener.

**Architecture:** Un workflow reutilizable `security.yml` vive en un repo público `gdberysan/.github` y detecta solo qué ecosistemas tiene el repo que lo llama (hoy: gitleaks siempre; govulncheck y golangci-lint+gosec si hay `go.mod`). Cada repo consumidor añade un job de una línea a su CI, más `dependabot.yml`, un hook pre-commit de gitleaks y (cuando el plan de GitHub lo permita) protección de `main`. Nada de DefectDojo ni orquestación local: la fuente de verdad es el CI que bloquea el merge.

**Tech Stack:** GitHub Actions (workflows reutilizables), gitleaks, govulncheck, golangci-lint v2 con gosec, Dependabot, `gh` CLI, hooks de git vía `core.hooksPath`.

**Spec:** `~/Dev/security/security-baseline.md` (repo `security`, commit 91b349c). La sección «Decisiones de diseño» de abajo resume lo que este plan usa de la spec; ante conflicto, manda la spec. Decisión posterior a la spec (2026-08-25, sesión de ejecución): el SAST de JS/TS es **Semgrep CE en CI**, no eslint-plugin-security — cv-forge usa biome y no queremos dos linters por repo; la Task 8 corrige la spec.

## Decisiones de diseño (spec)

1. **Enforcement antes que herramientas.** Todo control corre en CI y bloquea el merge. Nada depende de acordarse de ejecutar un script local. Se descartó explícitamente el stack DefectDojo/Trivy/Semgrep/ZAP local del documento original.
2. **Una herramienta por capa, nativa del ecosistema.** Secretos: gitleaks. SAST Go: gosec (vía golangci-lint). SAST JS/TS: Semgrep CE (rulesets `p/security-audit`; independiente del linter del repo — cv-forge usa biome). Dependencias Go: govulncheck (analiza el grafo de llamadas → casi cero falsos positivos). Dependencias pub/npm/actions: Dependabot.
3. **Workflow reutilizable con autodetección.** `gdberysan/.github` es público (los workflows reutilizables públicos son llamables desde repos privados; al revés no sin plan de pago). Los jobs de Go se activan solos si existe `go.mod`; añadir Node/Rust más adelante es un job más en un solo sitio.
4. **open-tv es privado hoy, será público (MIT) en el lanzamiento.** CodeQL, secret scanning nativo y (en plan Free) las reglas de protección de rama no están disponibles en privado: quedan documentadas en `docs/seguridad.md` como checklist de publicación, no se pierden.
5. **`mobile/` está congelada con CERO diffs.** Dependabot para pub se configura con `open-pull-requests-limit: 0`: alertas sí, PRs no.
6. **El baseline local va antes que el CI.** Se escanea y limpia en local ANTES de encender los jobs, para que el CI nazca en verde y cada rojo futuro sea señal real.

## Global Constraints

- `mobile/` congelada: CERO diffs en ese directorio (solo se referencia desde `dependabot.yml`, que vive en `.github/`).
- Todo el trabajo en la rama `chore/baseline-de-seguridad` creada desde `origin/main`; NO tocar `feat/p0-cliente-web-y-binario`.
- Mensajes de commit y documentación en castellano, siguiendo el estilo del repo (`docs: …`, `ci: …`, `chore: …`).
- Firma de commits del agente: terminar cada mensaje con `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- No modificar los jobs existentes `Gateway (Go)` y `Mobile (Flutter)` de `ci.yml`; solo añadir.
- `gdberysan/.github` se crea **público** y sin secretos de ningún tipo.
- Si un escáner encuentra un hallazgo real durante el baseline, se corrige o se documenta como excepción explícita (`#nosec` con comentario, `.gitleaksignore` con motivo); nunca se silencia el escáner entero.

---

### Task 1: Baseline local de secretos y dependencias en open-tv

**Files:**
- Create: (ninguno salvo hallazgos) — posible `.gitleaksignore` en la raíz si hay falsos positivos
- Modify: (solo si hay hallazgos reales que corregir)

**Interfaces:**
- Consumes: nada (primera tarea)
- Produces: repo con `gitleaks git`, `govulncheck ./...` y `semgrep scan` (rulesets JS/TS) saliendo con código 0, precondición de las Tasks 4 y 6

- [ ] **Step 1: Crear la rama de trabajo desde main**

```bash
cd ~/Dev/open-tv
git fetch origin
git checkout -b chore/baseline-de-seguridad origin/main
```

- [ ] **Step 2: Instalar las herramientas locales**

```bash
brew install gitleaks golangci-lint semgrep
go install golang.org/x/vuln/cmd/govulncheck@latest
gitleaks version && golangci-lint version && semgrep --version && "$(go env GOPATH)/bin/govulncheck" -version
```

Esperado: las cuatro imprimen versión (gitleaks ≥ 8.19, golangci-lint ≥ 2.0). Si `govulncheck` no está en el PATH, usar la ruta completa `$(go env GOPATH)/bin/govulncheck` en los pasos siguientes.

- [ ] **Step 3: Escanear TODO el historial de git con gitleaks (el test que debe pasar)**

```bash
gitleaks git --redact --verbose
```

Esperado: `no leaks found`, exit 0.

- [ ] **Step 4: Si hay hallazgos, triar uno a uno**

Para cada hallazgo: (a) si es un secreto real → rotarlo en el servicio de origen ANTES de nada, y avisar al usuario de que está en el historial (limpiar historial de un repo privado con un solo colaborador puede esperar a antes de publicarlo; anotarlo en el checklist de la Task 7); (b) si es un falso positivo (p. ej. un token de ejemplo en docs) → añadir su huella a `.gitleaksignore`:

```
# Falso positivo: <motivo>, revisado el 2026-08-25
<fingerprint que imprime gitleaks, formato commit:ruta:regla:línea>
```

Re-ejecutar el Step 3 hasta exit 0.

- [ ] **Step 5: Escanear dependencias Go con govulncheck**

```bash
govulncheck ./...
```

Esperado: `No vulnerabilities found`, exit 0. Si reporta CVEs *alcanzables*: subir la dependencia afectada con `go get <módulo>@<versión-parcheada> && go mod tidy`, correr `go test -race ./...` y confirmar verde.

- [ ] **Step 6: Escanear el cliente web (JS/TS) con Semgrep**

Solo los rulesets de JS/TS — el SAST de Go ya es gosec y correr dos motores sobre el mismo código genera ruido (spec §8):

```bash
semgrep scan --config p/javascript --config p/typescript --metrics=off --error
```

Esperado: exit 0 (semgrep respeta `.gitignore`, así que `node_modules/` y `dist/` quedan fuera). Si hay hallazgos: corregir los reales (mismo criterio que gosec); un falso positivo claro se excluye en línea con `// nosemgrep: <rule-id> -- motivo`, nunca desactivando la regla globalmente. Re-ejecutar hasta exit 0.

- [ ] **Step 7: Commit (solo si hubo cambios)**

```bash
git add -A -- ':!mobile'
git status   # verificar: cero cambios bajo mobile/
git commit -m "fix: baseline de seguridad — hallazgos de gitleaks/govulncheck

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

Si no hubo hallazgos, no hay commit: la tarea termina con los dos escáneres en exit 0.

---

### Task 2: gosec vía golangci-lint en open-tv

**Files:**
- Create: `.golangci.yml` (raíz del repo)
- Modify: código Go solo si gosec encuentra algo real

**Interfaces:**
- Consumes: golangci-lint instalado (Task 1, Step 2)
- Produces: `.golangci.yml` en formato v2 que el job `golangci-lint` del workflow reutilizable (Task 3) leerá tal cual; `golangci-lint run ./...` en exit 0

- [ ] **Step 1: Escribir la configuración**

Crear `.golangci.yml`:

```yaml
# Formato v2 de golangci-lint. El set "standard" (errcheck, govet,
# staticcheck, ineffassign, unused) más gosec como capa SAST.
version: "2"
linters:
  default: standard
  enable:
    - gosec
```

- [ ] **Step 2: Ejecutar y verificar el fallo o el verde**

```bash
golangci-lint run ./...
```

Esperado la primera vez: o exit 0, o una lista de hallazgos `gosec`/`staticcheck` con fichero:línea.

- [ ] **Step 3: Corregir los hallazgos reales**

Regla de triaje: los `G1xx`–`G5xx` de gosec sobre rutas de red, ficheros o exec se corrigen; un falso positivo claro se marca en línea con excepción justificada, nunca desactivando la regla globalmente:

```go
resp, err := client.Get(url) // #nosec G107 -- url viene de la config local del usuario, no de red
```

Re-ejecutar el Step 2 hasta exit 0.

- [ ] **Step 4: Confirmar que los tests siguen verdes**

```bash
go test -race -count=1 ./...
```

Esperado: PASS.

- [ ] **Step 5: Commit**

```bash
git add .golangci.yml $(git diff --name-only -- '*.go')
git commit -m "ci: configuración de golangci-lint v2 con gosec

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: Repo gdberysan/.github con el workflow reutilizable security.yml

**Files:**
- Create: `~/Dev/dot-github/` (clon del repo nuevo `gdberysan/.github`)
- Create: `.github/workflows/security.yml` (el workflow reutilizable)
- Create: `.github/workflows/ci.yml` (auto-test: el repo se llama a sí mismo)
- Create: `README.md`

**Interfaces:**
- Consumes: nada de open-tv (repo independiente)
- Produces: workflow llamable como `gdberysan/.github/.github/workflows/security.yml@main` con `on: workflow_call` sin inputs; jobs con nombre `gitleaks`, `govulncheck`, `golangci-lint` y `semgrep` (la Task 7 usa estos nombres exactos como status checks)

- [ ] **Step 1: Crear el repo público y clonarlo**

```bash
gh repo create gdberysan/.github --public --description "Workflows reutilizables y ficheros comunitarios compartidos" --clone
mv .github ~/Dev/dot-github   # si gh clonó en el cwd; ajustar si ya clonó en ~/Dev
cd ~/Dev/dot-github
```

- [ ] **Step 2: Escribir el workflow reutilizable**

Crear `.github/workflows/security.yml`:

```yaml
# Workflow reutilizable de seguridad para todos los repos de gdberysan.
# Cada job de ecosistema se activa solo si el repo llamante tiene sus
# ficheros de manifiesto; añadir un ecosistema nuevo = un job nuevo aquí.
name: Seguridad

on:
  workflow_call: {}

jobs:
  detectar:
    runs-on: ubuntu-latest
    outputs:
      go: ${{ steps.stack.outputs.go }}
      js: ${{ steps.stack.outputs.js }}
    steps:
      - uses: actions/checkout@v4
      - id: stack
        run: |
          if [ -f go.mod ]; then
            echo "go=true" >> "$GITHUB_OUTPUT"
          else
            echo "go=false" >> "$GITHUB_OUTPUT"
          fi
          # package.json en la raíz o a un nivel (web/, site/…);
          # */package.json NO matchea node_modules (los paquetes están
          # dos niveles abajo: node_modules/<pkg>/package.json).
          if [ -f package.json ] || compgen -G "*/package.json" > /dev/null; then
            echo "js=true" >> "$GITHUB_OUTPUT"
          else
            echo "js=false" >> "$GITHUB_OUTPUT"
          fi

  gitleaks:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          # gitleaks escanea todo el historial, no solo el último commit
          fetch-depth: 0
      - uses: gitleaks/gitleaks-action@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  govulncheck:
    needs: detectar
    if: needs.detectar.outputs.go == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache-dependency-path: go.sum
      - run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - run: govulncheck ./...

  golangci-lint:
    needs: detectar
    if: needs.detectar.outputs.go == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache-dependency-path: go.sum
      - uses: golangci/golangci-lint-action@v8

  semgrep:
    needs: detectar
    if: needs.detectar.outputs.js == 'true'
    runs-on: ubuntu-latest
    container:
      image: semgrep/semgrep
    steps:
      - uses: actions/checkout@v4
      # Solo rulesets JS/TS: el SAST de Go ya es gosec; dos motores
      # sobre el mismo código = ruido (spec §8). --error hace fallar
      # el job con hallazgos; --metrics=off evita telemetría.
      - run: semgrep scan --config p/javascript --config p/typescript --metrics=off --error
```

Nota para el implementador: `gitleaks-action@v2` es gratuito para cuentas personales (gdberysan lo es); solo las organizaciones necesitan `GITLEAKS_LICENSE`. Si la action fallara pidiendo licencia, sustituir ese paso por la instalación directa del binario (`curl` de la release + `gitleaks git --redact`).

- [ ] **Step 3: Escribir el auto-test (el repo consume su propio workflow)**

Crear `.github/workflows/ci.yml`:

```yaml
# Auto-test: este repo llama a su propio workflow reutilizable en cada
# push. Sin go.mod aquí, solo debe ejecutarse detectar + gitleaks.
name: CI

on:
  push:

jobs:
  seguridad:
    uses: ./.github/workflows/security.yml
```

- [ ] **Step 4: README mínimo**

Crear `README.md`:

```markdown
# .github

Workflows reutilizables compartidos por los repos de gdberysan.

## security.yml

Capa de seguridad estándar: gitleaks (secretos, historial completo);
si el repo tiene `go.mod`, govulncheck (dependencias vulnerables
alcanzables) y golangci-lint con gosec (SAST); si tiene `package.json`
(raíz o un nivel abajo), Semgrep CE con rulesets JS/TS. Uso desde
cualquier repo:

    jobs:
      seguridad:
        name: Seguridad
        uses: gdberysan/.github/.github/workflows/security.yml@main
        permissions:
          contents: read

El repo consumidor con Go debe tener su propio `.golangci.yml`
(formato v2) en la raíz.
```

- [ ] **Step 5: Commit, push y verificar el run en verde**

```bash
git add -A
git commit -m "ci: workflow reutilizable de seguridad (gitleaks + capa Go autodetectada)

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
git push -u origin main
gh run watch --exit-status
```

Esperado: run `CI` en verde; en el detalle, `gitleaks` ejecutado y los jobs de Go y de Semgrep saltados (este repo no tiene `go.mod` ni `package.json`). Si `gh run watch` no encuentra el run aún, `sleep 10` y reintentar.

---

### Task 4: open-tv consume el workflow reutilizable

**Files:**
- Modify: `.github/workflows/ci.yml` de open-tv (solo añadir un job al final)

**Interfaces:**
- Consumes: `gdberysan/.github/.github/workflows/security.yml@main` (Task 3), `.golangci.yml` (Task 2), baseline limpio (Task 1)
- Produces: checks `Seguridad / gitleaks`, `Seguridad / govulncheck`, `Seguridad / golangci-lint`, `Seguridad / semgrep` en cada push (open-tv tiene `web/package.json`, así que el job de Semgrep se activa); la Task 7 los referencia por esos nombres

- [ ] **Step 1: Añadir el job al ci.yml existente**

En `~/Dev/open-tv/.github/workflows/ci.yml`, añadir al final (sin tocar `gateway` ni `mobile`):

```yaml
  seguridad:
    name: Seguridad
    uses: gdberysan/.github/.github/workflows/security.yml@main
    permissions:
      contents: read
```

- [ ] **Step 2: Commit y push (esto ES el test: el run debe salir verde)**

```bash
cd ~/Dev/open-tv
git add .github/workflows/ci.yml
git commit -m "ci: capa de seguridad vía workflow reutilizable de gdberysan/.github

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
git push -u origin chore/baseline-de-seguridad
gh run watch --exit-status
```

Esperado: los 7 jobs en verde — `Gateway (Go)`, `Mobile (Flutter)`, `Cliente web`, `Seguridad / gitleaks`, `Seguridad / govulncheck`, `Seguridad / golangci-lint`, `Seguridad / semgrep`. Como las Tasks 1–2 dejaron el repo limpio en local, un rojo aquí es un problema de fontanería del workflow (ruta del `uses:`, permisos), no un hallazgo nuevo: depurar el workflow, no silenciar el escáner.

- [ ] **Step 3: Anotar los nombres exactos de los checks**

```bash
gh api "repos/gdberysan/open-tv/commits/$(git rev-parse HEAD)/check-runs" --jq '.check_runs[].name'
```

Guardar la salida literal: son los `context` que necesita la Task 7.

---

### Task 5: Dependabot en open-tv

**Files:**
- Create: `.github/dependabot.yml`

**Interfaces:**
- Consumes: nada
- Produces: alertas y PRs automáticos de dependencias (gomod, actions, npm en `/web`) y solo alertas para pub

- [ ] **Step 1: Escribir la configuración**

Crear `.github/dependabot.yml`:

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly

  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly

  - package-ecosystem: npm
    directory: /web
    schedule:
      interval: weekly

  # mobile/ está congelada con CERO diffs: límite 0 = Dependabot vigila
  # y alerta, pero no abre PRs que tocarían el directorio congelado.
  - package-ecosystem: pub
    directory: /mobile
    schedule:
      interval: weekly
    open-pull-requests-limit: 0
```

- [ ] **Step 2: Habilitar las alertas de vulnerabilidades del repo**

```bash
gh api -X PUT repos/gdberysan/open-tv/vulnerability-alerts
gh api repos/gdberysan/open-tv/vulnerability-alerts >/dev/null 2>&1 && echo "alertas: ON"
```

Esperado: `alertas: ON` (el GET devuelve 204 cuando está activo).

- [ ] **Step 3: Commit y push**

```bash
git add .github/dependabot.yml
git commit -m "ci: dependabot para gomod, actions, npm (web) y pub (pub solo alertas, mobile congelada)

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
git push
```

- [ ] **Step 4: Verificar que Dependabot acepta el fichero**

```bash
sleep 30
gh api repos/gdberysan/open-tv/dependabot/alerts --jq 'length' 2>&1
```

Esperado: un número (normalmente `0`) — no un error 403/404. Los errores de sintaxis del yml aparecerían en la pestaña Insights → Dependency graph → Dependabot; si el comando falla, revisar ahí.

---

### Task 6: Hook pre-commit de gitleaks (versionado, sin frameworks)

**Files:**
- Create: `scripts/git-hooks/pre-commit`

**Interfaces:**
- Consumes: gitleaks instalado (Task 1)
- Produces: bloqueo local de commits con secretos en cualquier clon que active `core.hooksPath`

- [ ] **Step 1: Escribir el hook**

Crear `scripts/git-hooks/pre-commit`:

```bash
#!/usr/bin/env bash
# Bloquea commits que introducen secretos. Se activa por clon con:
#   git config core.hooksPath scripts/git-hooks
set -euo pipefail

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "aviso: gitleaks no está instalado (brew install gitleaks); escaneo de secretos omitido" >&2
  exit 0
fi

gitleaks git --pre-commit --staged --redact --verbose
```

```bash
chmod +x scripts/git-hooks/pre-commit
git config core.hooksPath scripts/git-hooks
```

Nota: la sintaxis `gitleaks git --pre-commit --staged` es la de gitleaks ≥ 8.19 (la instalada por brew). Verificar con `gitleaks git --help` que los flags existen; en versiones antiguas el equivalente era `gitleaks protect --staged`.

- [ ] **Step 2: Test — el hook debe BLOQUEAR un secreto de ejemplo**

```bash
cat > /tmp/secreto-de-prueba.txt <<'EOF'
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRCYEXAMPLEKEY
EOF
cp /tmp/secreto-de-prueba.txt .
git add secreto-de-prueba.txt
git commit -m "test: no debe llegar a crearse" && echo "FALLO: el hook no bloqueó" || echo "OK: hook bloqueó el commit"
```

Esperado: `OK: hook bloqueó el commit` (gitleaks sale con código ≠ 0 y git aborta).

- [ ] **Step 3: Limpiar el secreto de prueba**

```bash
git reset HEAD secreto-de-prueba.txt
rm secreto-de-prueba.txt /tmp/secreto-de-prueba.txt
git status   # verificar árbol limpio salvo scripts/git-hooks/
```

- [ ] **Step 4: Test — el hook deja pasar un commit limpio (el commit del propio hook)**

```bash
git add scripts/git-hooks/pre-commit
git commit -m "chore: hook pre-commit de gitleaks vía core.hooksPath

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

Esperado: el commit se crea y en la salida se ve a gitleaks ejecutarse con `no leaks found`.

---

### Task 7: Protección de main, documentación y PR

**Files:**
- Create: `docs/seguridad.md`
- Modify: (nada más)

**Interfaces:**
- Consumes: nombres de checks anotados en Task 4 Step 3
- Produces: ruleset sobre `main` (o su documentación como pendiente), doc de referencia, PR listo para merge

- [ ] **Step 1: Intentar crear el ruleset de main**

Sustituir los `context` por los nombres EXACTOS anotados en Task 4 Step 3:

```bash
gh api -X POST repos/gdberysan/open-tv/rulesets --input - <<'EOF'
{
  "name": "main-requiere-ci",
  "target": "branch",
  "enforcement": "active",
  "conditions": { "ref_name": { "include": ["~DEFAULT_BRANCH"], "exclude": [] } },
  "rules": [
    { "type": "deletion" },
    { "type": "non_fast_forward" },
    { "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": false,
        "required_status_checks": [
          { "context": "Gateway (Go)" },
          { "context": "Mobile (Flutter)" },
          { "context": "Cliente web" },
          { "context": "Seguridad / gitleaks" },
          { "context": "Seguridad / govulncheck" },
          { "context": "Seguridad / golangci-lint" },
          { "context": "Seguridad / semgrep" }
        ]
      }
    }
  ]
}
EOF
```

Esperado: JSON del ruleset creado, **o** un 403 mencionando upgrade — en plan Free los rulesets con enforcement activo no están disponibles en repos privados. Si es 403: NO es un bloqueo del plan; dejarlo anotado en el paso siguiente y seguir.

- [ ] **Step 2: Escribir la documentación**

Crear `docs/seguridad.md`:

```markdown
# Seguridad del repo

## Capas activas

| Capa | Herramienta | Dónde corre |
|---|---|---|
| Secretos (historial completo) | gitleaks | CI, job `Seguridad / gitleaks` |
| Secretos (antes del commit) | gitleaks | hook local `scripts/git-hooks/pre-commit` |
| SAST Go | gosec vía golangci-lint (`.golangci.yml`) | CI y local (`golangci-lint run ./...`) |
| SAST JS/TS (cliente web) | Semgrep CE, rulesets `p/javascript` + `p/typescript` | CI, job `Seguridad / semgrep`; local: `semgrep scan --config p/javascript --config p/typescript --metrics=off --error` |
| Dependencias Go alcanzables | govulncheck | CI y local (`govulncheck ./...`) |
| Dependencias gomod/actions/pub | Dependabot (`.github/dependabot.yml`) | GitHub (pub: solo alertas, mobile/ congelada) |

El CI de seguridad es el workflow reutilizable
`gdberysan/.github/.github/workflows/security.yml`; los cambios de
herramientas se hacen ALLÍ, una vez, para todos los repos.

## Activación por clon

    git config core.hooksPath scripts/git-hooks

## Checklist al hacer el repo público (lanzamiento MIT)

- [ ] Activar CodeQL (Settings → Security → Code scanning → default setup)
- [ ] Activar secret scanning + push protection nativos de GitHub
- [ ] Ruleset `main-requiere-ci` (si el POST devolvió 403 en privado, repetirlo: en público es gratis)
- [ ] Revisar historial de git por secretos una última vez ANTES de publicar: `gitleaks git --redact`

## Triaje de hallazgos

Un hallazgo se corrige o se excluye con justificación en línea
(`#nosec G### -- motivo`, entrada comentada en `.gitleaksignore`).
Nunca se desactiva un escáner ni una regla globalmente.
```

Ajuste al escribirlo: si el Step 1 devolvió 403, marcar la línea del ruleset del checklist como pendiente por plan Free; si funcionó, quitarla del checklist.

- [ ] **Step 3: Commit final y PR**

```bash
git add docs/seguridad.md
git commit -m "docs: capas de seguridad del repo y checklist de publicación

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
git push
gh pr create --base main --title "Baseline de seguridad: CI reforzado + workflow reutilizable" --body "Añade la capa de seguridad aplicada por CI: gitleaks (historial + pre-commit), gosec vía golangci-lint, govulncheck y Dependabot, consumiendo el workflow reutilizable de gdberysan/.github. mobile/ sin diffs (pub solo alertas). Detalle en docs/seguridad.md.

🤖 Generated with [Claude Code](https://claude.com/claude-code)"
```

- [ ] **Step 4: Verificar el PR en verde antes de dar por cerrada la tarea**

```bash
gh pr checks --watch
```

Esperado: todos los checks en verde. El merge lo decide el usuario.

---

### Task 8: Corregir la spec (SAST JS/TS → Semgrep CE)

**Files:**
- Modify: `~/Dev/security/security-baseline.md` (repo `security`, NO open-tv)

**Interfaces:**
- Consumes: decisión de diseño de la sesión 2026-08-25 (Semgrep CE para JS/TS)
- Produces: spec alineada con lo implementado; ningún código depende de esto

- [ ] **Step 1: Actualizar la fila JS/TS de la tabla per-stack (§3)**

En `~/Dev/security/security-baseline.md`, sustituir la fila:

```
| JS/TS | eslint + eslint-plugin-security | Dependabot / osv-scanner |
```

por:

```
| JS/TS | Semgrep CE (`p/javascript` + `p/typescript`; independiente del linter — cv-forge usa biome) | Dependabot / osv-scanner |
```

Y en §8 (capas diferidas), eliminar la fila «Second SAST opinion wanted, or Dart SAST | Semgrep CE job» o reescribirla como «Dart SAST wanted | Semgrep CE rulesets de Dart» — Semgrep ya no es capa diferida para JS/TS.

- [ ] **Step 2: Verificar consistencia**

```bash
grep -n "eslint" ~/Dev/security/security-baseline.md
```

Esperado: ninguna referencia restante a eslint-plugin-security como SAST del baseline.

- [ ] **Step 3: Commit en el repo security**

```bash
cd ~/Dev/security
git add security-baseline.md
git commit -m "docs: SAST de JS/TS pasa a Semgrep CE (cv-forge usa biome, no eslint)

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```
