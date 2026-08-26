# Registro de cambios

Todos los cambios notables de este proyecto se documentan aquí.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto se adhiere a [Versionado Semántico](https://semver.org/lang/es/).

## [No publicado]

Trabajo hacia la primera versión pública `1.0.0`. Open TV pasó de un stack de
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

### Seguridad

- Sin cuentas, sin telemetría, sin datos del usuario fuera de su máquina o su
  navegador (favoritos en `localStorage`).
- Retirada de `CORS *`; middleware anti-rebinding (allowlist del puerto real,
  fail-closed) con comprobación `Sec-Fetch-Site` en métodos mutantes.

### Accesibilidad

- Rejilla virtualizada con tabindex por flechas, regiones aria-live
  persistentes, modales con focus-trap e `inert`, y verificación de contraste.

[No publicado]: https://github.com/gdberysan/open-tv/commits/main
