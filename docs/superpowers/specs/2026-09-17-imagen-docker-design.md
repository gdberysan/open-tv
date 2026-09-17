# Imagen de Docker — diseño (pendiente de decisión del dueño)

**Estado:** propuesta, NO aprobada. No se implementa nada hasta que el dueño
elija opción.

## Por qué

r/selfhosted y r/homelab son el público más natural de Open TV, y ese público
espera `docker compose up`. Sin imagen, buena parte descarta el proyecto sin
probarlo.

## El choque con un invariante duro

CLAUDE.md fija: **proxy HLS solo loopback**. Hoy se cumple así
(`cmd/open-tv/main.go`):

- `esLoopback(ln)` decide `ProxyActivo` mirando la IP real del listener.
- `hostsPermitidos(ln)` solo admite `127.0.0.1:<puerto>`, `localhost:<puerto>`
  y `[::1]:<puerto>` como `Host` (anti DNS-rebinding).

Dentro de un contenedor con la red por defecto (bridge), el proceso tiene que
escuchar en `0.0.0.0` para que `-p 8080:8080` le llegue. Consecuencia:
`esLoopback` da `false` y **el proxy se apaga**. La app arranca, pero todos los
canales que dependen del proxy (sin CORS, con cabeceras obligatorias) fallan.
Una imagen así funcionaría a medias y generaría issues del tipo «en Docker no
se ve nada».

## Opciones

### A. Imagen solo para `--network host` (Linux) — sin tocar el invariante

El contenedor comparte la red del anfitrión, así que el proceso sigue
escuchando en `127.0.0.1` del anfitrión y todo funciona igual que el binario.

- **Cambio de código:** ninguno.
- **Seguridad:** idéntica a la del binario.
- **Limitación:** `--network host` es de Linux. En Docker Desktop (macOS,
  Windows) existe solo en versiones recientes y detrás de un ajuste. Además
  sigue sin poder usarse desde otro dispositivo de la red, que es justo lo que
  suele querer quien monta un homelab.
- **Mensaje honesto:** «imagen para correrlo en un servidor Linux y verlo
  desde ese mismo equipo o por túnel SSH».

### B. Modo contenedor explícito — relaja el invariante con opt-in

Variable `OPEN_TV_MODO_CONTENEDOR=1` que activa el proxy aunque el listener no
sea loopback, más `OPEN_TV_HOSTS` para ampliar la lista blanca de `Host`. La
documentación obliga a publicar solo en loopback: `-p 127.0.0.1:8080:8080`.

- **Cambio de código:** pequeño, en `main.go`, con pruebas.
- **Riesgo:** si alguien publica `-p 8080:8080` (lo habitual), la API sin
  autenticación y un proxy HTTP quedan expuestos a la red. La lista blanca de
  `Host` NO lo impide: un atacante en la LAN fija el `Host` que quiera. La
  guarda SSRF evita que el proxy alcance destinos privados, pero sí serviría de
  relé hacia Internet.
- **Requiere:** cambiar el invariante en CLAUDE.md, y revisión de seguridad.

### C. B + autenticación mínima para exponer a la red

Token de acceso (o usuario/contraseña) obligatorio cuando el listener no es
loopback. Es lo que haría falta para el caso homelab real (ver la tele desde
el móvil o el televisor de casa).

- **Cambio de código:** mediano (middleware, pantalla de acceso en el cliente,
  i18n, a11y, pruebas).
- **Es una feature en sí**, con su propio brainstorming → spec → plan.

## Recomendación

**A ahora, C como feature aparte.** A se publica sin tocar la seguridad y
cubre al usuario Linux técnico. B es la opción peligrosa: abre el caso de uso
de red sin la protección que ese caso necesita, y el README no basta para
evitar el `-p 8080:8080` por defecto. Si el público pide acceso desde otros
dispositivos, lo correcto es C.

## Alcance de A (si se aprueba)

- `Dockerfile` multi-etapa: Node construye `web/` → Go compila con
  `CGO_ENABLED=0` → imagen final `gcr.io/distroless/static-debian12:nonroot`.
- Volumen `/data` con `DB_PATH=/data/open-tv.db`; arranque con `--no-browser`.
- Publicación en `ghcr.io/gdberysan/open-tv` desde goreleaser (`dockers_v2`),
  multi-arquitectura amd64/arm64, en el siguiente release.
- `compose.yaml` de ejemplo con `network_mode: host`.
- README: sección Docker con la limitación dicha de frente.
