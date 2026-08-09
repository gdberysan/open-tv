# Korven Open TV

Agregador y reproductor de televisión abierta (FTA) desde fuentes públicas
tipo IPTV-org. Gateway en Go + app Flutter para macOS.

Una obra de [Korven](https://korven.dev) — *del núcleo a la obra*.

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

## Configuración de la app

La URL del gateway se fija en tiempo de compilación:

```bash
flutter run -d macos --dart-define=GATEWAY_URL=http://192.168.1.50:8080
```

## Desarrollo

```bash
cd gateway && go test -race ./... && go vet ./... && gofmt -l .
cd mobile  && flutter test && flutter analyze
```

CI corre exactamente eso en cada push (`.github/workflows/ci.yml`).

## Documentación

- `PROMPT_MAESTRO.md` — contrato y roadmap del proyecto
- `docs/adr/` — decisiones de arquitectura
- `docs/superpowers/plans/` — planes de implementación
