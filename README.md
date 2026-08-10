# Korven Open TV

Agregador y reproductor de televisión abierta (FTA) desde fuentes públicas
tipo IPTV-org. Gateway en Go + app Flutter para macOS.

Una obra de [Korven](https://korven.dev) — *del núcleo a la obra*.

Sin canales premium, sin VPN, sin geo-bypass, sin credenciales de terceros.

## Arrancar el stack

Hacen falta dos procesos. **El gateway no se arranca solo** salvo que se
instale el LaunchAgent de abajo: no hay docker-compose ni supervisor. Si no
está corriendo, la app no enseña nada y avisa de que no ha podido contactar
con el gateway en el puerto 8080.

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

### Que el gateway se arranque solo (opcional, macOS)

Para no repetir el paso 1 cada día hay una plantilla de LaunchAgent en
`tools/dev.korven.opentv.gateway.plist`: arranca al iniciar sesión y se revive
si se cae. Instalación, actualización y desinstalación en
[`tools/README.md`](tools/README.md).

Sirve un binario compilado, así que **tras tocar código Go hay que recompilar y
reiniciarlo** o seguirá sirviendo la versión anterior.

### Comprobar que el gateway está vivo

```bash
curl -s localhost:8080/health
lsof -iTCP:8080 -sTCP:LISTEN -n -P
launchctl print gui/$(id -u)/dev.korven.opentv.gateway   # si está bajo launchd
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

## Emisión a un televisor

Con un Apple TV o un televisor con AirPlay 2 en la misma red, el botón de
AirPlay de la barra superior abre el selector del sistema. Al elegir destino, la
barra inferior queda fija y los toques en la rejilla emiten al televisor en vez
de abrir el reproductor.

Solo AirPlay: no hay Chromecast, DLNA ni Roku.

Algunos canales se ven en el Mac y no viajan a AirPlay. AVFoundation es mucho
más estricto que libmpv y rechaza manifiestos y códecs que mpv reproduce sin
quejarse. Cuando pasa, la app se da cuenta en 15 s, reproduce el canal en local
y lo recuerda para marcarlo en la lista.

También se puede ceder un canal que ya se está viendo: abrir el canal, pulsar
AirPlay y elegir destino. El reproductor local para y el televisor toma el
relevo.

El ⏹ de la barra termina la sesión, pero **no** deselecciona la ruta del
sistema: esa UI es de Apple y una app no puede tocarla. macOS puede seguir
enviando el audio del sistema al televisor hasta que se cambie a mano.

### Depurar la emisión

La capa Swift no pasa por CI, así que va instrumentada. Los mensajes salen por
**stderr**, es decir en la salida de `flutter run` — no con `log stream`:

```bash
cd mobile && flutter run -d macos 2>&1 | grep airplay
```

La línea que importa es `a los 3s: externalPlaybackActive=…`: `true` significa
que macOS está descargando el vídeo en el televisor. En `readyToPlay` todavía
sale `false`, tarda un par de segundos en cambiar.

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
