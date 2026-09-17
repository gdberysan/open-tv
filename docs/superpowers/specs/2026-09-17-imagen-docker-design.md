# Docker y acceso desde la red — diseño (pendiente de aprobación)

**Estado:** propuesta, NO aprobada. Objetivo fijado por el dueño el
2026-09-17: **máxima adopción y robustez.**

## Por qué

El público natural de Open TV (r/selfhosted, r/homelab) espera
`docker compose up` y, sobre todo, **ver la tele desde otros dispositivos**:
el televisor, el móvil, la tablet. Hoy ninguna de las dos cosas es posible.

## Lo que hay hoy, medido en el código

- `LISTEN_ADDR` por defecto `127.0.0.1:8080`. Si se cambia a `0.0.0.0`:
  - `esLoopback(ln)` da `false` y **el proxy HLS se apaga** (`main.go`). Los
    canales que dependen de él dejan de verse: en Chrome/Firefox es el ~33 %
    del catálogo según el censo de `plan.ts`.
  - `hostsPermitidos(ln)` solo acepta `127.0.0.1`, `localhost` y `[::1]` como
    `Host`, así que **otro dispositivo recibe 403** al entrar por la IP de la
    LAN. Exponerlo hoy no funciona, ni siquiera a medias.
- La API no tiene autenticación. Expuesta, cualquiera en la red podría:
  añadir y borrar fuentes (`POST/DELETE /fuentes`), hacer que el servidor
  descargue URLs http/https arbitrarias al sincronizar, **incluida la red
  privada** (el syncer usa un cliente sin guarda SSRF a propósito: una lista en
  la LAN es un caso legítimo en local; `validarURLFuente` sí bloquea `file://`), y usar `/proxy/hls?u=` como
  **relé abierto hacia Internet** (la guarda SSRF bloquea destinos privados,
  no públicos).
- La lista blanca de `Host` NO es control de acceso: un atacante en la LAN
  pone el `Host` que quiera con `curl`. Solo protege del DNS-rebinding desde
  un navegador.

## Opciones consideradas

- **A. Imagen solo con `--network host`.** Cero cambios, seguridad intacta.
  Pero solo Linux, y sigue sin verse desde otros dispositivos, que es lo que
  pide este público. Adopción baja: parece Docker sin serlo del todo.
- **B. Encender el proxy en contenedor con una variable.** Barato y
  peligroso: con el `-p 8080:8080` habitual deja la API y un relé abiertos a
  la red. La documentación no evita el error por defecto. **Descartada.**
- **C. Modo red con clave de acceso.** Es la que cumple el objetivo.
  Detallada abajo.

## Recomendación: C, en v1.1.0

Principio: **sin autenticación solo en loopback; cualquier listener que no sea
loopback exige clave.** Así la instalación local sigue siendo cero
configuración, y exponerlo es seguro por construcción, no por documentación.

### 1. Clave de acceso

- Al arrancar con un listener no-loopback, se lee `OPEN_TV_ACCESS_KEY`; si no
  existe, se genera una aleatoria (32 bytes, base64url), se guarda en
  `<datadir>/access-key` con permisos 600 y se imprime en el log una sola vez
  con la URL: `Abre http://<host>:8080 y usa la clave …`. Mismo patrón que
  Jupyter o Portainer.
- Pantalla de acceso en el cliente (i18n es/en, a11y): pegar la clave →
  `POST /acceso` → cookie de sesión `HttpOnly; SameSite=Strict`, firmada con
  HMAC, con caducidad larga (30 días) y rotación al cambiar la clave.
- Todo el API, el proxy y los assets de la app salvo la propia pantalla de
  acceso exigen sesión. Comparación de clave en tiempo constante, y límite de
  intentos por IP.
- En loopback no cambia nada: ni clave ni pantalla.

### 2. El proxy deja de ser un relé abierto (en los dos modos)

Hoy `/proxy/hls?u=<cualquier URL pública>` relaya lo que se le pida. Se
cambia a URLs firmadas:

- El servidor solo entrega URLs de proxy firmadas con HMAC
  (`/proxy/hls?u=…&f=<firma>`) para streams que existen en el catálogo.
- Al reescribir un manifiesto, firma cada URL hija (variantes, segmentos,
  claves). Una URL sin firma válida recibe 403.
- Mejora también el modo local: una web maliciosa ya no puede usar el proxy
  del usuario aunque encontrara la forma de llegar a él.

### 3. Orígenes y CSRF en modo red

- `hostsPermitidos` pasa a aceptar además los `Host` de `OPEN_TV_HOSTS` (lista
  separada por comas). Si está vacía en modo red, se acepta cualquier `Host`,
  porque la sesión ya es el control de acceso y la cookie `SameSite=Strict`
  cierra el DNS-rebinding (el dominio atacante no tiene la cookie).
- Se mantiene la comprobación de `Sec-Fetch-Site` en métodos mutantes y se
  añade la de `Origin`.

### 4. HTTPS detrás de un reverse proxy

Quien expone a la red suele poner Caddy/Traefik/nginx con TLS. Implicaciones:

- `plan.ts` ya manda por el proxy los streams `http` cuando la página es
  `https` (contenido mixto). Con TLS, más tráfico pasa por el proxy: se
  documenta y se mide.
- Cookie con `Secure` cuando llega `X-Forwarded-Proto: https` desde un proxy
  de confianza (`OPEN_TV_TRUSTED_PROXIES`).

### 5. La imagen

- `Dockerfile` multi-etapa: Node construye `web/` → Go con `CGO_ENABLED=0` →
  `gcr.io/distroless/static-debian12:nonroot`. Sin shell, usuario sin
  privilegios, ~20 MB.
- `open-tv healthcheck` (subcomando nuevo) para el `HEALTHCHECK` de Docker, que
  en distroless no puede usar `curl`.
- Volumen `/data` (`DB_PATH=/data/open-tv.db`, clave y fuentes subidas ahí).
  `LISTEN_ADDR=0.0.0.0:8080` y `--no-browser` por defecto en la imagen.
- Publicación multi-arquitectura (amd64, arm64 — Raspberry Pi incluida) en
  `ghcr.io/gdberysan/open-tv` con goreleaser `dockers_v2`, etiquetas `1.1.0`,
  `1.1`, `1` y `latest`. Docker Hub como espejo opcional (más visible en
  búsquedas).
- `compose.yaml` de ejemplo en el repo y sección Docker en los dos README.

### 6. Robustez

- Pruebas: sesión requerida en modo red y no en loopback; firma del proxy
  (válida, manipulada, caducada, URL hija); límite de intentos; `Host` en los
  dos modos; e2e de Playwright contra el binario escuchando en `0.0.0.0`.
- Revisión de seguridad dedicada antes del merge (es un cambio de invariante).
- CLAUDE.md: el invariante pasa de «proxy solo loopback» a «sin autenticación
  solo en loopback; proxy solo con URLs firmadas».
- Documentación honesta: ni la clave sustituye a TLS si se expone a Internet,
  ni se recomienda abrirlo fuera de casa sin un reverse proxy con HTTPS.

### Fuera de alcance

Usuarios múltiples, perfiles, roles, OAuth. Una sola clave por instalación.

## Coste y orden

Es una feature mediana: middleware y sesión, firma del proxy, pantalla de
acceso, imagen y publicación, pruebas y revisión. Se hace por el flujo
habitual: esta spec aprobada → plan → SDD → review final → v1.1.0.

Mientras tanto, el README dice la verdad: sin imagen de Docker todavía, con
el motivo, e issue #7 para agrupar el interés.
