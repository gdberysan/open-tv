# Registro de cambios

Todos los cambios notables de este proyecto se documentan aquí.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto se adhiere a [Versionado Semántico](https://semver.org/lang/es/).

## [1.1.0] - 2026-09-17

Open TV se puede tener en un servidor de casa y ver desde cualquier
dispositivo de la red.

### Añadido

- **Modo red con clave de acceso**: fuera de `127.0.0.1`, Open TV exige una
  clave (generada en el primer arranque o fijada con `OPEN_TV_ACCESS_KEY`) y
  una sesión firmada que dura 30 días. Página de acceso sin JavaScript, en
  español e inglés. Tras 5 intentos fallidos por minuto desde una IP, espera;
  los accesos correctos no gastan intentos. En `127.0.0.1` no cambia nada.
- **Imagen de Docker** multi-arquitectura (`linux/amd64`, `linux/arm64`) en
  `ghcr.io/gdberysan/open-tv`: distroless, sin root, con `HEALTHCHECK` y
  `compose.yaml` de ejemplo. Las prereleases no mueven `latest`.
- Subcomandos `open-tv access-key` y `open-tv healthcheck`.
- Si la sesión caduca con la app abierta, el cliente vuelve a la página de
  acceso.

### Seguridad

- El proxy HLS deja de relayar URLs arbitrarias, también en `127.0.0.1`: solo
  las del catálogo o las que firma al reescribir un manifiesto. La clave de
  firma se guarda en el directorio de datos, así que la reproducción sigue
  tras reiniciar el servidor.
- `MismoOrigen` usa `Sec-Fetch-Site` y, cuando el navegador no lo manda,
  compara `Origin` con `Host` en los métodos mutantes.

### Documentación

- El README deja de recomendar `go install`: sin compilar antes el cliente
  web, el binario no sirve ninguna interfaz.

## [1.0.0] - 2026-09-16

Primera versión pública. Open TV pasó de un stack de
dos procesos (gateway Go + app Flutter) a un **binario único** que sirve un
cliente web embebido; la app de macOS queda congelada y compatible.

### Añadido

- **Cliente web embebido**: un solo `open-tv` sirve la API y un cliente
  Svelte 5 (`go:embed`), abre el navegador y escucha en el primer puerto
  libre. UI en español e inglés.
- **Reproducción robusta**: proxy HLS solo en loopback con guarda SSRF
  (anti-redirección y anti-DNS-rebinding), detección de motor del navegador y
  `PlaybackGuard` que declara fallos con un mensaje claro.
- **Fiabilidad y failover**: los mirrors de un canal se ordenan por salud y se
  recorren hasta que uno reproduce; diagnóstico de fallo por clase
  (caído/geo/formato/caducado); estadísticas de reproducción **solo locales**
  (nada sale de la máquina).
- **Sala de control (UX)**: barra lateral de facetas con conteos, chips de
  filtro removibles, tarjetas con señal honesta (latencia + resolución),
  «Continuar viendo», salto a un canal al azar con la barra espaciadora.
- **Fuentes (bring-your-own)**: la instalación limpia arranca vacía y ofrece
  fuentes sugeridas de la comunidad (iptv-org) con un toque; añadir listas
  M3U propias es directo. El producto público NO viaja con un catálogo
  preconfigurado.
- **Estado del arte**: overlay del reproductor, pantalla completa y
  Picture-in-Picture, **paleta de comandos ⌘K** con búsqueda difusa, densidad
  de rejilla configurable, vista de ajustes con «Acerca de» y aviso legal,
  micro-interacciones que respetan `prefers-reduced-motion`.
- **Reproductor primero**: el vídeo vive en un panel persistente junto al
  catálogo lateral; cambiar de canal no lo desmonta, y «Ver todo» conserva la
  rejilla completa. Arranca en silencio con un botón para activar el sonido.
- **AirPlay**: el botón de emitir manda vídeo y audio a un Apple TV (el motor
  pasa a reproducción nativa solo mientras dura la emisión).
- **Mirrors de iptv-org**: cada canal suma las alternativas que publica la API
  de iptv-org, con las cabeceras (referrer y user-agent) que exige cada origen.
- **Salud por segmento**: el chequeo lee el primer segmento y detecta los
  códecs que ningún navegador decodifica; el reproductor los salta y lo dice.
- **Tiempo hasta la imagen**: cada intento real anota si llegó a dar imagen y
  cuánto tardó; los mirrors sin imagen se saltan, con «Probar de todos modos».
- **Distribución**: `brew install --cask`, instalador `install.sh` con
  verificación de checksum para macOS y Linux, y `open-tv --version`.

### Seguridad

- Sin cuentas, sin telemetría, sin datos del usuario fuera de su máquina o su
  navegador (favoritos en `localStorage`).
- Retirada de `CORS *`; middleware anti-rebinding (allowlist del puerto real,
  fail-closed) con comprobación `Sec-Fetch-Site` en métodos mutantes.

### Accesibilidad

- Rejilla virtualizada con tabindex por flechas, regiones aria-live
  persistentes, paleta ⌘K con focus-trap e `inert`, y verificación de contraste.

[1.1.0]: https://github.com/gdberysan/open-tv/releases/tag/v1.1.0
[1.0.0]: https://github.com/gdberysan/open-tv/releases/tag/v1.0.0
