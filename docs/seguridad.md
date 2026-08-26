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
- [ ] Ruleset `main-requiere-ci`: pendiente por plan Free (el POST a la API de rulesets devuelve 403 "Upgrade to GitHub Pro or make this repository public" en repos privados); repetirlo al hacer el repo público, ahí es gratis
- [ ] Revisar historial de git por secretos una última vez ANTES de publicar: `gitleaks git --redact`

## Triaje de hallazgos

Un hallazgo se corrige o se excluye con justificación en línea
(`#nosec G### -- motivo`, entrada comentada en `.gitleaksignore`).
Nunca se desactiva un escáner ni una regla globalmente.
