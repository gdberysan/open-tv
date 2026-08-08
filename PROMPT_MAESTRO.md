# PROMPT MAESTRO — Ecosistema IPTV Open-Source
> **Versión:** 3.0 | **Herramienta objetivo:** Claude Code CLI | **Modo:** Iterativo por fases

---

## 0. CÓMO USAR ESTE DOCUMENTO

Este archivo es el contrato permanente del proyecto. Se lee una vez al inicio de cada sesión de Claude Code con:

```bash
claude < PROMPT_MAESTRO.md
# o bien, dentro de una sesión interactiva:
# /read PROMPT_MAESTRO.md
```

Al inicio de **cada sesión nueva**, antepón el siguiente bloque de contexto antes de tu instrucción:

```
FASE ACTUAL    : [número y nombre, ej. "Fase 6 — EPG"]
ARCHIVOS TOCADOS EN SESIÓN ANTERIOR : [lista o "ninguno — primera sesión"]
BLOQUEANTES CONOCIDOS : [lista o "ninguno"]
TAREA DE ESTA SESIÓN  : [una sola oración concreta y verificable]
```

**Regla de oro:** Una sesión = una tarea = un output verificable con `go build ./...` o `flutter analyze`.

---

## 1. VISIÓN Y ALCANCE

Sistema open-source para agregación, indexación y reproducción de **televisión abierta (free-to-air)** desde fuentes públicas como IPTV-org. Alcance deliberadamente limitado: sin canales premium, sin VPN, sin geo-bypass, sin credenciales de terceros. Solo TV pública que cualquiera puede ver.

### Pilares no negociables

| Pilar | Definición operativa |
|---|---|
| **Clean Architecture** | `domain` sin imports externos. Dependencias apuntan hacia adentro. |
| **Concurrencia segura** | Pools acotados. Canales para comunicación, mutexes solo para estado compartido. |
| **Observabilidad desde el día 1** | `slog` estructurado en todos los componentes. Sin `fmt.Println` en producción. |
| **Error handling explícito** | Cero `_` descartando errores. Errores envueltos con `fmt.Errorf("op: %w", err)`. |

---

## 2. DECISIONES DE STACK (CERRADAS — NO REABRIR)

| Componente | Tecnología | Versión mínima | Razón |
|---|---|---|---|
| API Gateway | Go + `chi` router | 1.22 / chi v5 | Sin magic, middleware composable |
| Base de datos | SQLite (`modernc.org/sqlite`) | latest | Zero dependencias C, embebido, portable |
| EPG Parser | `xml.Decoder` streaming | stdlib | Archivos >500MB; nunca carga total en memoria |
| Frontend | Flutter | 3.x stable | Un codebase → iOS + Android + macOS |
| Logging | `log/slog` | stdlib Go 1.21+ | Estructurado, sin dependencias externas |

---

## 3. ESTRUCTURA DE DIRECTORIOS (CANÓNICA)

```
iptv-ecosystem/
│
├── gateway/
│   ├── cmd/server/main.go              # Entrypoint; solo wiring
│   ├── internal/
│   │   ├── domain/                     # Entidades puras — ZERO imports externos
│   │   ├── ports/                      # Interfaces (entrada y salida)
│   │   ├── services/                   # Casos de uso (Syncer)
│   │   ├── adapters/
│   │   │   ├── db/                     # SQLite repositories + schema.sql
│   │   │   ├── epg/                    # XMLTV streaming parser + worker
│   │   │   ├── validator/              # Health-check de streams (pool HEAD→GET)
│   │   │   └── providers/
│   │   │       └── opensource/         # IPTV-org y fuentes públicas M3U
│   │   └── api/
│   │       ├── handlers/               # HTTP handlers, uno por recurso
│   │       └── middleware/             # Logging, rate-limit, CORS, recover
│   ├── go.mod
│   └── go.sum
│
├── mobile/                             # Flutter app (solo target macOS activo)
│   ├── lib/
│   │   ├── domain/models/
│   │   ├── data/repositories/
│   │   └── presentation/
│   │       ├── player/                 # PlaybackGuard
│   │       ├── providers/
│   │       ├── screens/
│   │       └── widgets/
│   └── test/
│
├── .github/workflows/ci.yml            # Gate: build, vet, gofmt, test, analyze
├── docs/adr/                           # Architecture Decision Records
├── docs/superpowers/plans/             # Planes de implementación
├── README.md                           # Arranque del stack y variables de entorno
└── PROMPT_MAESTRO.md
```

---

## 4. MODELO DE DOMINIO

### 4.1 Entidades (`gateway/internal/domain/`)

**Restricciones absolutas:** Zero imports externos (solo `time` y tipos primitivos Go).

```go
// channel.go
type ChannelID string
type ProviderType string

const (
    ProviderOpenSource ProviderType = "opensource"
)

type Channel struct {
    ID           ChannelID
    Name         string
    LogoURL      string
    CategoryID   string
    LanguageCode string    // ISO 639-1
    CountryCode  string    // ISO 3166-1 alpha-2
    ProviderID   string
    ProviderType ProviderType
    IsAdult      bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

func (c Channel) Validate() error

// epg.go
type EPGEntry struct {
    ChannelID   ChannelID
    Title       string
    Description string
    StartAt     time.Time
    EndAt       time.Time
}
```

---

## 5. INTERFACES DE PUERTO (`gateway/internal/ports/`)

### `channel_repository.go`
```go
type ChannelFilter struct {
    Query      string // búsqueda LIKE
    Country    string // ISO 3166-1 alpha-2
    Category   string // ID exacto
    MinQuality string // "4k" | "fhd" | "hd" | "" (todos)
    Limit      int
    Offset     int
}

type ChannelRepository interface {
    Save(ctx context.Context, ch domain.Channel) error
    SaveBatch(ctx context.Context, channels []domain.Channel) error
    FindByID(ctx context.Context, id domain.ChannelID) (domain.Channel, error)
    FindFiltered(ctx context.Context, f ChannelFilter) ([]domain.Channel, error)
    Search(ctx context.Context, query string, limit int) ([]domain.Channel, error)
    Delete(ctx context.Context, id domain.ChannelID) error
}
```

### `provider_port.go`
```go
type ProviderPort interface {
    ID()   string
    Type() domain.ProviderType
    GetLiveChannels(ctx context.Context) ([]domain.Channel, error)
    GetStreamURL(ctx context.Context, channelID domain.ChannelID) (string, error)
    GetEPGData(ctx context.Context, channelID domain.ChannelID) ([]domain.EPGEntry, error)
    HealthCheck(ctx context.Context) error
}
```

---

## 6. RIESGOS Y MITIGACIONES

| # | Riesgo | Severidad | Mitigación |
|---|---|---|---|
| 1 | **Race condition en validador** | Alta | Canal `results chan StreamResult` con un único consumidor. Sin mutex en escritura. |
| 2 | **OOM en parser EPG** — XMLs >500MB | Alta | `xml.Decoder.Token()` streaming. Pool de buffers con `sync.Pool`. |
| 3 | **Rate limiting de IPTV-org** | Media | Pool acotado (50 goroutines max). Jitter 100–500ms. Backoff exponencial. |
| 4 | **URLs rotatorias** — tokens en URL | Media | `GetStreamURL` nunca cachea la URL final. TTL corto (5min) en caché L1. |
| 5 | **Goroutine leak en validador** | Alta | `context.WithTimeout` en cada request. `defer cancel()` inmediato. |
| 6 | **M3U gigantes** — 50k+ entradas | Media | Parseo línea a línea con `bufio.Scanner`. Pipeline separado: parsear → validar → persistir. |

---

## 7. ADR-001: ESTRATEGIA DE PARSING EPG

**Decisión:** `xml.Decoder.Token()` en streaming, con callback `onEntry` para batch-insert cada 500 entradas.

**Razón:** Archivos XMLTV pueden superar 500MB. Carga total en memoria causaría OOM.

**Trade-off:** Mayor complejidad vs. RAM constante ~2MB independiente del tamaño del archivo.

---

## 8. ESTADO ACTUAL DEL PROYECTO (agosto 2026)

### Completado ✓

| Componente | Estado | Notas |
|---|---|---|
| Go gateway — Clean Architecture | ✓ | chi v5, modernc/sqlite, WAL mode |
| IPTV-org provider (~13 500 canales FTA) | ✓ | Sync periódico con backoff, upsert bulk |
| `FindFiltered` con calidad dinámica | ✓ | Patterns `(1080p)`, `(4K)` entre paréntesis — sin falsos positivos |
| Flutter macOS — media_kit + libmpv | ✓ | HLS nativo |
| Player timeout 15 s + `PlaybackGuard` | ✓ | Los errores transitorios de mpv ya no matan el vídeo |
| Filter bar (calidad + país + categoría) | ✓ | `channelFilterProvider`, `FilterChip`, picker dialog |
| **EPG Worker wired en `main.go`** | ✓ | Fase 6.1 (`4fbb821`). Capa de persistencia completa, no las "10 líneas" previstas |
| **`StreamRepository` SQLite** | ✓ | Fase 7.1 (`1ec19f0`) |
| **Stream health validator + worker** | ✓ | Fase 7.1. Pool HEAD→GET, `HEALTH_INTERVAL` default 60m |
| **Guía EPG en Flutter** | ✓ | Fase 6.2 (`0ba8467`). Parrilla canales × slots de 30 min |
| **Indicador de señal + toggle offline** | ✓ | Fase 7.2 (`cf98db2`) |
| CI (GitHub Actions) | ✓ | build, vet, gofmt, test `-race`, analyze |
| `README.md` | ✓ | Arranque del stack y tabla de variables de entorno |

> ⚠️ **La Fase 6 está completa pero apagada por defecto.** `EPG_URL` es opt-in
> y no tiene default: sin configurarla, el worker de EPG no arranca y la guía
> sale vacía. No existe una URL XMLTV pública canónica para el catálogo de
> IPTV-org. Ver `README.md`.

### Pendiente prioritario

| Componente | Estado |
|---|---|
| iOS / Android targets | ✗ Solo macOS activo (`mobile/` no tiene `ios/` ni `android/`) |
| Empaquetado del gateway | ✗ Se arranca a mano; sin launchd, Docker ni supervisor |
| `/metrics` y `/health` enriquecido | ✗ Fase 10 |
| Favoritos e historial | ✗ Fase 8; `sqflite` no está en `pubspec.yaml` |

### Plan de fiabilidad en curso

`docs/superpowers/plans/2026-08-08-fiabilidad-iptv.md` — 19 tareas derivadas de
la auditoría del 2026-08-08, ordenadas por riesgo eliminado. Cubre correctitud
del catálogo (histéresis de salud, poda de canales fantasma), robustez de la
app (timeouts, watchdog), observabilidad y limpieza de código muerto.

---

## 9. ROADMAP — PRÓXIMAS FASES

---

### FASE 6 — EPG: Guía de programación en vivo
> **Output verificable:** `GET /epg?channel=X&from=T1&to=T2` retorna JSON; Flutter muestra timeline.

#### 6.1 Backend
- [ ] Wire `epg.NewWorker` en `cmd/server/main.go` con URL de EPG de IPTV-org
- [ ] `internal/adapters/db/epg_repository.go`: `SaveBatch([]domain.EPGEntry)` con upsert, batch de 500
- [ ] `internal/ports/epg_repository.go`: interfaz con `SaveBatch`, `FindByChannelAndWindow`, `FindCurrentlyAiring`
- [ ] `GET /epg?channel=X&from=RFC3339&to=RFC3339` — ventana máxima 24 h
- [ ] `GET /epg/now` — todos los canales con su programa actual

#### 6.2 Flutter — Timeline
- [ ] `lib/domain/models/epg_entry.dart`
- [ ] `lib/data/repositories/epg_repository.dart`
- [ ] `lib/presentation/providers/epg_provider.dart`: `FutureProvider` que recarga cada 5 min
- [ ] `lib/presentation/screens/guide_screen.dart`: grid horizontal, canales × slots de 30 min
- [ ] `HomeScreen`: nombre del programa actual debajo del nombre de canal

---

### FASE 7 — Channel Health Scoring
> **Output verificable:** Indicador de señal en lista; canales offline ocultados por defecto.

#### 7.1 Backend
- [ ] `internal/adapters/db/stream_repository.go`: implementar `StreamRepository`
- [ ] `internal/adapters/validator/worker.go`: pool de 50 goroutines; HEAD → GET fallback; verifica primer segment HLS
- [ ] Worker cada 60 min en background
- [ ] `GET /channels/{id}/health` → `{ alive, latency_ms, checked_at }`
- [ ] `ChannelFilter.AliveOnly bool` en `FindFiltered`; default `true`

#### 7.2 Flutter
- [ ] Indicador de 3 barras en `ListTile`: verde (<200ms), naranja (200–800ms), rojo (>800ms), gris (muerto)
- [ ] Toggle `Ocultar offline` en AppBar, persistido en `SharedPreferences`

---

### FASE 8 — Favoritos, Historial y UX Premium
> **Output verificable:** Favoritos persisten entre reinicios.

- [ ] SQLite local con `sqflite`: tablas `favorites`, `watch_history`
- [ ] `favorites_provider.dart`: `StateNotifier<Set<String>>`
- [ ] Icono ♥ en cada `ListTile`; favoritos aparecen primero
- [ ] Tab `Recientes` con últimos 10 canales vistos
- [ ] `PlayerScreen`: guardar posición al cerrar, ofrecer "Continuar desde..." al reabrir
- [ ] macOS: Picture-in-Picture, atajos de teclado (espacio, ←/→, ↑/↓, F, M)

---

### FASE 9 — iOS, Android y Apple TV
> **Output verificable:** App instalable en simulador iOS y emulador Android.

- [ ] iOS: `DebugProfile.entitlements` → `network.client = true`; TestFlight
- [ ] Android: `INTERNET` + `FOREGROUND_SERVICE`; background playback con `audio_service`
- [ ] Apple TV: layout 10-foot UI; `FocusTraversalGroup`; Siri Remote
- [ ] Android TV: layout de tarjetas grandes; Leanback

---

### FASE 10 — Observabilidad y Operaciones
> **Output verificable:** `GET /metrics` retorna Prometheus; `docker compose up` levanta el stack completo.

- [ ] `metrics_handler.go`: `iptv_channels_total`, `iptv_channels_alive`, `iptv_stream_errors_total`
- [ ] `Dockerfile` multi-stage: build `golang:alpine` → runtime `scratch`, imagen < 20 MB
- [ ] `docker-compose.yml`: gateway + Prometheus + Grafana + volumen SQLite
- [ ] CI/CD: GitHub Actions → `go test` + `flutter analyze` + Docker build en cada merge a `main`
- [ ] `GET /health` enriquecido: `{ status, uptime, channels_alive, last_sync, db_size_mb }`

---

### Tabla resumen de fases

| Fase | Nombre | Prioridad | Esfuerzo estimado |
|---|---|---|---|
| **6** | EPG + Guía interactiva | 🔴 Alta | 2–3 sesiones |
| **7** | Channel health scoring | 🔴 Alta | 1–2 sesiones |
| **8** | Favoritos + UX premium | 🟠 Media | 1–2 sesiones |
| **9** | iOS + Android + TV | 🟡 Media | 3–4 sesiones |
| **10** | Observabilidad prod-ready | 🟡 Media | 1–2 sesiones |

---

### Quick wins disponibles HOY (sin nueva fase)

1. **`GET /channels/categories`**: listar categorías únicas para mejorar el picker de categorías en Flutter
2. **`GET /channels/countries`**: listar países disponibles para reemplazar la lista hardcodeada en Flutter

---

## 10. REGLAS PERMANENTES DE CÓDIGO

```
✓ go build ./...     debe pasar sin warnings al terminar cada sesión
✓ go vet ./...       cero errores
✓ flutter analyze    cero errores
✓ Tests en la misma sesión que el código que testean
✓ Errores con contexto: fmt.Errorf("validator.Check: %w", err)
✓ Goroutines con context propagado y timeout explícito
✓ slog.Error/Info/Debug — nunca fmt.Println
✓ defer cancel() inmediatamente después de context.WithTimeout/WithCancel

✗ Cero _ descartando errores en producción
✗ Cero goroutines sin límite de concurrencia
✗ Cero carga de archivos completos >10MB en memoria
✗ Cero comentarios que repiten lo que el código ya dice
✗ Cero globals mutables sin protección
```

---

## 11. CHECKLIST DE SESIÓN COMPLETADA

```
✓ [archivo] — compila / pasa tests
...
SESIÓN COMPLETADA
Próxima fase sugerida: [nombre]
Bloqueantes: [lista o "ninguno"]
```
