<!-- markdownlint-disable MD033 MD041 -->
<div align="center">

```
   ▄▄▄▄    ▄▄▄▄   ▄▄▄▄▄   ▄▄   ▄▄        ▄▄▄▄▄▄  ▄▄   ▄▄
  ██  ██  ██  ██  ██  ██  ███ ███         ██    ██   ██
  ██  ██  ██████  █████   ██ █ ██         ██    ██   ██
  ██  ██  ██  ██  ██      ██   ██         ██    ██   ██
   ▀▀▀▀   ▀▀  ▀▀  ▀▀      ▀▀   ▀▀         ▀▀     ▀▀▀▀▀
             K  O  R  V  E  N   ·   O P E N   T V
```

### La televisión abierta, sin cuentas ni nube — corriendo en tu máquina

Un solo binario Go sirve la API **y** un cliente web embebido (Svelte). No trae
canales de fábrica: **tú** añades tus fuentes M3U. Sin cuentas, sin telemetría,
sin relay — todo se sirve desde tu propio equipo.

`● señal honesta`  ·  `⌘K búsqueda difusa`  ·  `guía EPG por fuente`  ·  `MIT + marca Korven`  ·  `macOS · Linux · Windows`

Una obra de **[Korven](https://korven.dev)** — *del núcleo a la obra*

**Español**  ·  **[English ↓](#korven-open-tv-english)**

[Qué es](#qué-es) · [Instalar](#instalar) · [Primer arranque](#primer-arranque) · [Privacidad](#privacidad) · [Lo que no hace](#lo-que-no-hace) · [FAQ](#preguntas-frecuentes) · [Para desarrolladores](#para-desarrolladores)

</div>

---

## Qué es

Open TV agrega y reproduce canales de televisión abierta a partir de listas
M3U que tú mismo das de alta: la tuya propia, un fichero que subas, o
cualquiera de las seis fuentes públicas sugeridas de
[iptv-org](https://github.com/iptv-org/iptv) (global y por país/categoría),
con un clic. El gateway sincroniza el catálogo, comprueba la salud de cada
stream (vivo/caído, latencia, si se ve en el navegador) y el cliente web te
deja buscar, filtrar y reproducir — todo sirviéndose desde tu propia máquina.

No es un servicio: no hay cuentas, no hay nube, no hay canales premium. Es
software libre que reproduce enlaces que ya tienes.

## Dónde verlo

- **Instalado en tu máquina** (macOS, Linux, Windows): la forma recomendada.
  Ver [Instalar](#instalar) abajo.
- **Versión web hospedada** en `opentv.korven.dev`: próximamente. Será el
  mismo cliente sirviendo un catálogo público de solo lectura, sin nada que
  instalar.
- **Código fuente**:
  [github.com/gdberysan/open-tv](https://github.com/gdberysan/open-tv).

## Instalar

**Homebrew** (macOS y Linux):

```bash
brew install gdberysan/tap/open-tv
```

**Instalador por curl** (macOS y Linux, sin marca de cuarentena):

```bash
curl -fsSL https://raw.githubusercontent.com/gdberysan/open-tv/main/install.sh | sh
```

**Con Go instalado**:

```bash
go install github.com/gdberysan/open-tv/cmd/open-tv@latest
```

**Binario suelto**: en la pestaña
[Releases](https://github.com/gdberysan/open-tv/releases) del repo hay un
asset para cada combinación de sistema y arquitectura (macOS arm64/amd64,
Linux amd64/arm64, Windows amd64) con su `checksums.txt`. Descarga el que
corresponda al tuyo. *(Todavía no existe una release publicada — este README
documenta el flujo tal como quedará en cuanto se etiquete la primera.)*

### macOS: binario descargado con el navegador

Si lo bajas directamente desde Releases con Safari o Chrome, macOS lo marca
en cuarentena y Gatekeeper se queja al primer arranque. No hay notarización
de Apple todavía, así que hace falta uno de estos dos pasos:

```bash
xattr -d com.apple.quarantine /ruta/a/open-tv
```

o bien clic derecho → **Abrir** → **Abrir de todas formas** en el diálogo de
Gatekeeper. Homebrew y el instalador por curl no pasan por cuarentena: son el
camino limpio.

## Primer arranque

```bash
open-tv
```

Crea el directorio de datos si no existe, escucha en el primer puerto libre
(`127.0.0.1:8080` y, si está ocupado, el siguiente) y abre el navegador. Como
la instalación no trae canales, la primera pantalla pide una fuente: tu
propia URL M3U, un fichero, o una de las seis sugeridas de iptv-org con un
clic. En cuanto añades una, la sincronización de esa fuente tarda unos
segundos y la rejilla se llena; los veredictos de salud de cada stream
(vivo/caído, calidad) van llegando en los minutos siguientes.

Ctrl-C detiene el proceso. Si ya hay un `open-tv` corriendo, lanzarlo otra vez
no arranca una segunda instancia: solo abre el navegador en la que ya está
viva.

## Privacidad

- Sin cuentas, sin registro, sin nube.
- Sin telemetría: nada sale de tu máquina hacia Korven ni hacia nadie.
- Sin relay: el reproductor apunta directo al emisor de cada canal; el
  gateway local solo actúa de proxy cuando el navegador no puede reproducir
  el stream él solo, y ese proxy nunca escucha fuera de `localhost`.
- Los favoritos viven en el `localStorage` de tu navegador. Las estadísticas
  de reproducción viven solo en memoria del proceso y se pierden al
  reiniciar — no se escriben a disco ni se envían a ningún sitio.
- Las fuentes que añadas (URLs, ficheros M3U) se guardan solo en tu SQLite
  local.

## Lo que no hace

Open TV es, deliberadamente, solo un reproductor de televisión abierta:

- No trae canales premium ni de pago.
- No incluye DRM ni lo rompe.
- No hace geo-bypass ni VPN: si un stream lo bloquea tu región, seguirá
  bloqueado aquí.
- No pide ni gestiona credenciales de terceros.
- No aloja ni retransmite contenido: solo reproduce las URLs que tú das de
  alta. La responsabilidad de esas fuentes es tuya.

## Preguntas frecuentes

**¿Trae canales de fábrica?** No. Una instalación limpia arranca vacía;
añades tu propia lista M3U o eliges alguna de las seis sugeridas de iptv-org.

**¿Es legal?** Open TV no aloja ni distribuye nada: es un reproductor para
listas que tú decides añadir. La legalidad de esas listas depende de ti y de
tu jurisdicción.

**¿Recoge algún dato mío?** No. Ver [Privacidad](#privacidad).

**¿Por qué algunos canales no se reproducen en el navegador?** Algunos
streams usan códecs o cabeceras CORS que el navegador rechaza aunque el
canal esté vivo. El binario instalado detecta el caso y usa un proxy local
(solo loopback) como segundo intento; si aun así falla, el mensaje lo dice
con claridad en vez de fingir que funciona.

**¿Cómo lo desinstalo?** Borra el binario (o `brew uninstall open-tv`) y,
si quieres borrar también el catálogo, el directorio de datos:
`~/Library/Application Support/Korven Open TV` en macOS,
`$XDG_DATA_HOME/korven-open-tv` (o `~/.local/share/korven-open-tv`) en Linux,
`%APPDATA%\Korven Open TV` en Windows.

**macOS dice que el binario "no se puede abrir". ¿Qué hago?** Ver la nota de
cuarentena en [Instalar](#instalar).

## Comandos y variables

```bash
open-tv               # arranca el gateway (subcomando `serve` implícito)
open-tv serve          # lo mismo, explícito
open-tv --no-browser   # no abre el navegador al arrancar
```

| Variable | Default | Para qué sirve |
|---|---|---|
| `LISTEN_ADDR` | `127.0.0.1:8080` | Dirección de escucha. **No exponer a la red**: la API no tiene autenticación. |
| `DB_PATH` | directorio de datos del sistema | Ruta del SQLite del catálogo. Si no se fija, usa el directorio estándar de la plataforma (ver desinstalar arriba). |
| `SYNC_INTERVAL` | `12h` | Cadencia del re-sync automático de las fuentes activas. |
| `HEALTH_INTERVAL` | `60m` | Cadencia del health-check de streams. |

## Licencia y marca

Código bajo licencia **MIT** — ver [`LICENSE`](LICENSE).

El nombre «Korven» y «Korven Open TV», el wordmark, el emblema y el icono de
la aplicación **no** están cubiertos por esa licencia — ver
[`assets/brand/LICENSE`](assets/brand/LICENSE). El código es libre; la
identidad, no: si publicas un fork, usa tu propio nombre y emblema.

## Soporte

[GitHub Issues](https://github.com/gdberysan/open-tv/issues) — errores, un
canal que no se ve, o una idea. No hay soporte por correo personal.

## Para desarrolladores

La app de macOS nativa (Flutter, libmpv, AirPlay) vive en `mobile/`. Está
**congelada**: sigue compilando y no forma parte de la v1.0, pero no recibe
desarrollo activo. Para compilarla y correrla:

```bash
cd mobile
flutter run -d macos
```

Apunta a `http://127.0.0.1:8080` por defecto; para otro gateway:

```bash
flutter run -d macos --dart-define=GATEWAY_URL=http://192.168.1.50:8080
```

### Compilar el gateway + cliente web desde el código fuente

```bash
cd web && npm install && npm run build   # genera web/dist, embebido con go:embed
cd .. && go build -o open-tv ./cmd/open-tv
```

### Tests y gates

```bash
go test -race ./... && go vet ./... && gofmt -l .
cd web && npm run check && npm test
cd mobile && flutter test && flutter analyze
```

CI corre lo mismo en cada push (`.github/workflows/ci.yml`).

### Notas para quien desarrolla el gateway

- `IPTV_ORG_URL` es un atajo de desarrollo, opt-in: si se fija, da de alta esa
  URL como fuente al arrancar (para no pasar por el onboarding a mano). No es
  para uso normal — una instalación real siempre parte de cero fuentes.
- Para no arrancar el gateway a mano en cada sesión de desarrollo hay una
  plantilla de LaunchAgent de macOS en `tools/dev.korven.opentv.gateway.plist`
  (instrucciones en [`tools/README.md`](tools/README.md)).

---

<a id="english"></a>

# Korven Open TV (English)

A free-to-air (FTA) television player for your own M3U lists. A single Go
binary serves the API and an embedded web client (Svelte); it ships with no
channels — you add your own sources.

A work by [Korven](https://korven.dev) — *from the core to the work*.

[Español](#korven-open-tv) · **English**

## What it is

Open TV aggregates and plays free-to-air channels from M3U lists that you add
yourself: your own list, a file you upload, or any of the six suggested
public sources from [iptv-org](https://github.com/iptv-org/iptv) (global and
by country/category), added with one click. The gateway syncs the catalog,
checks the health of each stream (alive/down, latency, whether it plays in a
browser), and the web client lets you search, filter and play — all served
from your own machine.

It is not a service: no accounts, no cloud, no premium channels. It's free
software that plays links you already have.

## Where to watch it

- **Installed on your machine** (macOS, Linux, Windows): the recommended way.
  See [Install](#install) below.
- **Hosted web version** at `opentv.korven.dev`: coming soon. It will be the
  same client serving a public, read-only catalog, with nothing to install.
- **Source code**:
  [github.com/gdberysan/open-tv](https://github.com/gdberysan/open-tv).

## Install

**Homebrew** (macOS and Linux):

```bash
brew install gdberysan/tap/open-tv
```

**curl installer** (macOS and Linux, no quarantine flag):

```bash
curl -fsSL https://raw.githubusercontent.com/gdberysan/open-tv/main/install.sh | sh
```

**With Go installed**:

```bash
go install github.com/gdberysan/open-tv/cmd/open-tv@latest
```

**Raw binary**: the
[Releases](https://github.com/gdberysan/open-tv/releases) tab has an asset
for every OS/architecture combination (macOS arm64/amd64, Linux amd64/arm64,
Windows amd64) with a `checksums.txt`. Download the one that matches your
system. *(No release has been published yet — this README documents the
flow as it will work once the first tag ships.)*

### macOS: binary downloaded via a browser

If you download it straight from Releases with Safari or Chrome, macOS marks
it quarantined and Gatekeeper complains on first launch. There is no Apple
notarization yet, so you need one of these two steps:

```bash
xattr -d com.apple.quarantine /path/to/open-tv
```

or right-click → **Open** → **Open Anyway** in the Gatekeeper dialog.
Homebrew and the curl installer skip quarantine entirely — they're the clean
path.

## First run

```bash
open-tv
```

Creates the data directory if it doesn't exist, listens on the first free
port (`127.0.0.1:8080`, or the next one if taken) and opens your browser.
Since the install ships with no channels, the first screen asks for a
source: your own M3U URL, a file, or one of the six suggested iptv-org
sources with one click. Once you add one, syncing that source takes a few
seconds and the grid fills in; health verdicts for each stream (alive/down,
quality) follow over the next few minutes.

Ctrl-C stops the process. If an `open-tv` is already running, launching it
again doesn't start a second instance — it just opens your browser to the
one that's already alive.

## Privacy

- No accounts, no sign-up, no cloud.
- No telemetry: nothing leaves your machine toward Korven or anyone else.
- No relay: the player points straight at each channel's own origin; the
  local gateway only proxies when the browser can't play the stream on its
  own, and that proxy never listens outside `localhost`.
- Favorites live in your browser's `localStorage`. Playback stats live only
  in the process's memory and are lost on restart — never written to disk
  or sent anywhere.
- Sources you add (URLs, M3U files) are stored only in your local SQLite.

## What it doesn't do

Open TV is, deliberately, only a free-to-air player:

- No premium or paid channels.
- No DRM, and no breaking it either.
- No geo-bypass and no VPN: if your region blocks a stream, it stays
  blocked here.
- No third-party credentials requested or managed.
- No hosting or rebroadcasting: it only plays URLs you add yourself.
  Responsibility for those sources is yours.

## FAQ

**Does it ship with channels?** No. A clean install starts empty; you add
your own M3U list or pick one of the six suggested iptv-org sources.

**Is it legal?** Open TV hosts and distributes nothing: it's a player for
lists you choose to add. The legality of those lists depends on you and your
jurisdiction.

**Does it collect any of my data?** No. See [Privacy](#privacy).

**Why don't some channels play in the browser?** Some streams use codecs or
CORS headers the browser rejects even though the channel is alive. The
installed binary detects this and falls back to a local (loopback-only)
proxy as a second attempt; if that still fails, the message says so plainly
instead of pretending it works.

**How do I uninstall it?** Remove the binary (or `brew uninstall open-tv`)
and, if you also want to delete the catalog, the data directory:
`~/Library/Application Support/Korven Open TV` on macOS,
`$XDG_DATA_HOME/korven-open-tv` (or `~/.local/share/korven-open-tv`) on
Linux, `%APPDATA%\Korven Open TV` on Windows.

**macOS says the binary "can't be opened". What do I do?** See the
quarantine note in [Install](#install).

## Commands and variables

```bash
open-tv               # starts the gateway (`serve` subcommand is implicit)
open-tv serve          # same, explicit
open-tv --no-browser   # don't open the browser on startup
```

| Variable | Default | What it's for |
|---|---|---|
| `LISTEN_ADDR` | `127.0.0.1:8080` | Listen address. **Don't expose it to the network**: the API has no authentication. |
| `DB_PATH` | system data directory | Path to the catalog's SQLite file. If unset, uses the platform's standard directory (see uninstall above). |
| `SYNC_INTERVAL` | `12h` | Cadence of the automatic re-sync of active sources. |
| `HEALTH_INTERVAL` | `60m` | Cadence of the stream health-check. |

## License and brand

Code under the **MIT** license — see [`LICENSE`](LICENSE).

The "Korven" and "Korven Open TV" names, the wordmark, the emblem and the
application icon are **not** covered by that license — see
[`assets/brand/LICENSE`](assets/brand/LICENSE). The code is free; the
identity is not: if you publish a fork, use your own name and emblem.

## Support

[GitHub Issues](https://github.com/gdberysan/open-tv/issues) — bugs, a
channel that doesn't play, or an idea. No support over personal email.

## For developers

The native macOS app (Flutter, libmpv, AirPlay) lives in `mobile/`. It is
**frozen**: it still builds and isn't going away, but it isn't part of v1.0
and receives no active development. To build and run it:

```bash
cd mobile
flutter run -d macos
```

Points at `http://127.0.0.1:8080` by default; for a different gateway:

```bash
flutter run -d macos --dart-define=GATEWAY_URL=http://192.168.1.50:8080
```

### Building the gateway + web client from source

```bash
cd web && npm install && npm run build   # generates web/dist, embedded via go:embed
cd .. && go build -o open-tv ./cmd/open-tv
```

### Tests and gates

```bash
go test -race ./... && go vet ./... && gofmt -l .
cd web && npm run check && npm test
cd mobile && flutter test && flutter analyze
```

CI runs the same on every push (`.github/workflows/ci.yml`).

### Notes for gateway developers

- `IPTV_ORG_URL` is an opt-in development shortcut: if set, it registers that
  URL as a source on startup, skipping the onboarding form by hand. It's not
  for normal use — a real install always starts from zero sources.
- To avoid starting the gateway by hand on every dev session, there's a
  macOS LaunchAgent template at `tools/dev.korven.opentv.gateway.plist`
  (instructions in [`tools/README.md`](tools/README.md)).
