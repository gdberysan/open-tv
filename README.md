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
deben coincidir con los `tvg-id` del M3U (formato `Nombre.cc@Feed`, p.ej.
`10TV.in@SD`).

**Fuente que funciona hoy** — la única de la lista comunitaria de
[iptv-org/epg](https://github.com/iptv-org/epg/blob/master/GUIDES.md) con ids
compatibles:

```bash
EPG_URL="https://raw.githubusercontent.com/StrangeDrVN/epg/public/output/guide.xml" go run ./cmd/server
```

> ⚠️ **Cobertura limitada.** Son 478 canales, de los cuales 477 casan con el
> catálogo, pero **465 son de India**. El resto: 7 de Canadá, 3 de Países
> Bajos, 1 de EE.UU. y 1 del Reino Unido. La guía se llena de verdad para esos
> canales y queda vacía para todos los demás.
>
> Ojo: el enlace que aparece en `GUIDES.md` (`.../public/guide.xml`) da 404;
> la ruta buena lleva `/output/`.

**Para cubrir otros países** hay que generar la guía uno mismo con el grabber
de iptv-org/epg (`npm run grab --- --sites=...`) y servir el XML resultante por
HTTP. El grabber tiene sites por país; el `channels.xml` del proyecto dice qué
ids produce cada uno.

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
