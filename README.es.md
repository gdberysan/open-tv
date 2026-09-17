<!-- markdownlint-disable MD033 MD041 -->
<div align="center">

<img src="assets/readme/banner-es.svg" alt="Korven Open TV — televisión abierta, sin cuentas ni nube, en tu máquina" width="100%">

<p>
  <a href="https://github.com/gdberysan/open-tv/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/gdberysan/open-tv?style=flat-square&color=FF8A2B&labelColor=171E29"></a>
  <a href="https://github.com/gdberysan/open-tv/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/gdberysan/open-tv/ci.yml?branch=main&style=flat-square&label=CI&labelColor=171E29"></a>
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/github/license/gdberysan/open-tv?style=flat-square&color=97A3B2&labelColor=171E29"></a>
  <img alt="macOS · Linux · Windows" src="https://img.shields.io/badge/plataformas-macOS%20%C2%B7%20Linux%20%C2%B7%20Windows-EFF3F8?style=flat-square&labelColor=171E29">
</p>

**Televisión abierta desde tus propias listas M3U, en el navegador, con un solo binario.**<br>
Sin cuentas. Sin telemetría. Sin nube. Y una señal honesta en cada canal: te
dice qué se va a ver de verdad antes de que hagas clic.

[Instalar](#instalar) · [Funciones](#funciones) · [Cómo funciona](#cómo-funciona) · [Privacidad](#privacidad) · [Limitaciones](#limitaciones-conocidas) · [FAQ](#preguntas-frecuentes) · **[English](README.md)**

<img src="assets/readme/demo.gif" alt="Open TV: elegir un canal, cambiar de canal en el sitio y buscar con ⌘K" width="100%">

</div>

---

## Instalar

```bash
# macOS (Homebrew)
brew install --cask gdberysan/tap/open-tv

# macOS y Linux (verifica el checksum antes de instalar)
curl -fsSL https://raw.githubusercontent.com/gdberysan/open-tv/main/install.sh | sh

# En cualquier sitio con Go
go install github.com/gdberysan/open-tv/cmd/open-tv@latest
```

Después, ejecuta `open-tv` y se abre el navegador con la app.

En Windows, o si prefieres una descarga directa: baja el archivo de tu sistema
desde [Releases](https://github.com/gdberysan/open-tv/releases/latest) y
compruébalo con `checksums.txt`.

> **¿macOS y lo bajaste con el navegador?** El binario aún no está notarizado
> por Apple, así que Gatekeeper lo bloquea al primer arranque. Ejecuta una vez
> `xattr -d com.apple.quarantine ./open-tv`, o clic derecho → **Abrir** →
> **Abrir de todas formas**. Homebrew y el instalador por curl ya lo resuelven.

## Docker: verlo desde cualquier dispositivo de tu red

```bash
docker run -d --name open-tv -p 8080:8080 -v open-tv-data:/data \
  --restart unless-stopped ghcr.io/gdberysan/open-tv:latest
docker logs open-tv   # la clave de acceso sale en el primer arranque
```

Abre `http://<ip-del-servidor>:8080` en la tele, el móvil o el portátil y pega
la clave. Fija la tuya con `-e OPEN_TV_ACCESS_KEY=…`, o vuelve a verla con
`docker exec open-tv /open-tv access-key`. En el repo hay un
[`compose.yaml`](compose.yaml) listo. Las imágenes son para `linux/amd64` y
`linux/arm64` (Raspberry Pi 4/5).

Fuera de `127.0.0.1`, Open TV siempre pide la clave de acceso. Para llegar a
él desde fuera de casa, ponlo detrás de un reverse proxy con HTTPS (Caddy,
Traefik, nginx) en vez de abrir el puerto a Internet.

Un reverse proxy debe reenviar la cabecera `Host` original (nginx:
`proxy_set_header Host $host;` — Caddy y Traefik ya lo hacen por defecto) y
debería mandar `X-Forwarded-Proto` para que la cookie de sesión salga marcada
`Secure` detrás de HTTPS.

## Funciones

- **Un binario, cero configuración.** Un servidor Go con el cliente web
  embebido. Sin base de datos que instalar, Docker opcional, sin media center.
  Descargas, ejecutas y ves.
- **Señal honesta.** Cada stream se comprueba en segundo plano: vivo o caído,
  latencia, resolución y si tu navegador puede decodificarlo. Los canales que
  no pueden dar imagen lo dicen, en vez de quedarse cargando para siempre.
- **Failover entre mirrors.** Un canal con varias fuentes las prueba por orden
  de salud hasta que una se ve.
- **El reproductor primero.** El vídeo sigue en pantalla mientras recorres el
  catálogo lateral; cambiar de canal lo sustituye en el sitio.
- **Paleta de comandos ⌘K** con búsqueda difusa, favoritos, «Continuar
  viendo» y atajos de teclado para reproducir, silenciar, pantalla completa y
  zapear.
- **AirPlay** a un Apple TV, **Picture-in-Picture** y pantalla completa.
- **Guía EPG** por fuente, cuando la lista la trae.
- **Tus propias listas.** Pega una URL M3U, sube un fichero o añade con un
  clic una de las listas públicas sugeridas de
  [iptv-org](https://github.com/iptv-org/iptv).
- **Interfaz en español e inglés.** Pensada para teclado y lectores de
  pantalla.

## Cómo funciona

<img src="assets/readme/diagrama-es.svg" alt="Cómo funciona Open TV: el navegador reproduce el vídeo directo de las emisoras; open-tv, en 127.0.0.1 por defecto (o modo red con clave de acceso), sincroniza las listas y guías que añades, comprueba los streams, lo guarda todo en un SQLite local y solo retransmite un stream cuando el navegador no puede pedirlo." width="100%">

Una instalación limpia arranca **vacía**: Open TV no trae canales. Las listas
las pones tú; la app las ordena y te dice la verdad sobre cada stream.

El vídeo va **directo de la emisora a tu navegador**; no pasa por open-tv. El
proxy local solo entra cuando el navegador no puede pedir un stream por sí
mismo (si faltan cabeceras CORS, o es un stream `http` en una página `https`),
y solo retransmite streams de tu catálogo. El servidor tiene su propio tráfico
en segundo plano: sincroniza tus listas y guías, y comprueba cada stream
leyendo su playlist y el principio de su primer segmento de vídeo.

## Privacidad

- Sin cuentas, sin registro, sin nube.
- Sin telemetría: no se envía nada a Korven ni a nadie. El único tráfico va a
  las listas, guías y emisoras que añades (ver el diagrama de arriba).
- Tus fuentes, el catálogo y el historial de salud de cada stream viven en un
  SQLite local. Los favoritos y «Continuar viendo» viven en tu navegador.
- Por defecto el servidor escucha en `127.0.0.1` y no pide acceso. Si escucha
  en cualquier otra dirección (como en la imagen de Docker), cada petición
  exige la clave de acceso.

## Lo que no hace

Open TV es, a propósito, solo un reproductor de televisión abierta:

- Sin canales premium ni de pago, sin DRM y sin saltarse DRM.
- Sin geo-bypass ni VPN: si tu región bloquea un stream, sigue bloqueado.
- No aloja ni retransmite nada: solo reproduce las URLs que tú añades. Esas
  fuentes son responsabilidad tuya.

## Limitaciones conocidas

- **Algunos códecs nunca se ven en un navegador.** Unos pocos canales emiten
  vídeo MPEG-2, que ningún navegador decodifica. Open TV lo detecta y lo dice;
  para esos, usa VLC.
- **Los canales van y vienen.** Las listas públicas cambian a diario. Que un
  canal concreto no se vea casi siempre es cosa del stream, no de la app.
- **El geobloqueo** lo aplican las emisoras, y Open TV no lo esquiva.
- **Binarios sin firmar.** Ni el de macOS ni el de Windows están firmados
  todavía (ver la nota de macOS en [Instalar](#instalar)).
- **Sin Chromecast todavía.** AirPlay funciona; Chromecast está planeado.

## Preguntas frecuentes

**¿Trae canales?** No. Arranca vacía; añades tu propia lista M3U o una de las
sugeridas de iptv-org.

**¿Es legal?** Open TV no aloja ni distribuye nada. Es un reproductor de
listas que tú decides añadir, y la legalidad de esas listas depende de ti y
de tu jurisdicción. Las sugeridas son la colección comunitaria de iptv-org de
streams de televisión abierta disponibles públicamente.

**¿Recoge algún dato?** No. Ver [Privacidad](#privacidad).

**¿Cómo lo desinstalo?** Borra el binario (o `brew uninstall --cask
open-tv`). Para borrar también el catálogo, elimina el directorio de datos:
`~/Library/Application Support/Korven Open TV` en macOS,
`$XDG_DATA_HOME/korven-open-tv` (o `~/.local/share/korven-open-tv`) en Linux,
`%APPDATA%\Korven Open TV` en Windows.

## Comandos y variables

```bash
open-tv                # arranca el servidor y abre el navegador
open-tv --no-browser   # arranca sin abrir el navegador
open-tv --version      # muestra la versión
open-tv access-key     # muestra la clave de acceso del modo red
open-tv healthcheck    # sale con 0 si el servidor local está sano (lo usa Docker)
```

Si ya hay un `open-tv` corriendo, lanzarlo otra vez solo abre el navegador en
el que ya está vivo. Ctrl-C lo detiene.

| Variable | Default | Para qué sirve |
|---|---|---|
| `LISTEN_ADDR` | `127.0.0.1:8080` | Dirección de escucha. Cualquier cosa que no sea loopback activa el modo red, que exige la clave de acceso. Si el puerto está ocupado, usa el siguiente libre. |
| `OPEN_TV_ACCESS_KEY` | generada | Clave de acceso del modo red. Si no se fija, se genera en el primer arranque, se guarda junto a la base de datos y sale una vez en el log. |
| `DB_PATH` | directorio de datos del sistema | Ruta del SQLite del catálogo. |
| `SYNC_INTERVAL` | `12h` | Cada cuánto se re-sincronizan las fuentes. |
| `HEALTH_INTERVAL` | `60m` | Cada cuánto se comprueban los streams. |

## Contribuir y soporte

- Fallos e ideas: [GitHub Issues](https://github.com/gdberysan/open-tv/issues).
- ¿Vas a mandar código? Lee antes [`CONTRIBUTING.md`](CONTRIBUTING.md): dice
  qué encaja en el proyecto y qué no.
- ¿Un fallo de seguridad? No abras un issue público; ver
  [`SECURITY.md`](SECURITY.md).
- [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) explica cómo se trata la gente
  por aquí.

### Compilar desde el código fuente

```bash
cd web && npm ci && npm run build   # compila el cliente en internal/ui/dist
cd .. && go build -o open-tv ./cmd/open-tv
```

Pruebas: `go test -race ./...` y `cd web && npm run check && npm test`. CI
corre el conjunto completo, más análisis de seguridad, en cada push.

La app nativa de macOS de `mobile/` (Flutter) está congelada: sigue
compilando, pero el producto es el cliente web. `IPTV_ORG_URL` es un atajo de
desarrollo que da de alta una fuente al arrancar, y en `tools/` hay una
plantilla de LaunchAgent de macOS para tener el servidor corriendo mientras
desarrollas.

## Licencia y marca

Código bajo licencia **MIT** — ver [`LICENSE`](LICENSE) y
[`NOTICE`](NOTICE). Los nombres «Korven» y «Korven Open TV», el wordmark, el
emblema y el icono de la app **no** están cubiertos por ella
([`assets/brand/LICENSE`](assets/brand/LICENSE)): si publicas un fork, usa tu
propio nombre y emblema.

Una obra de **[Korven](https://korven.dev)** — *del núcleo a la obra*.
