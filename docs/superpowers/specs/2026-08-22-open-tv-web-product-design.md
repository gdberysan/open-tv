# Korven Open TV — versión web, distribución y presencia en korven.dev

**Estado:** aprobado 2026-08-22 · **Alcance:** gateway Go + cliente web nuevo + release + `opentv.korven.dev` + páginas en korven.dev. La app Flutter de macOS queda **congelada e intacta**.

## 1. Contexto y objetivos

Korven Open TV funciona —catálogo correcto, salud de streams con histéresis, AirPlay, 130 tests en la app y suite Go en verde— pero solo existe para quien sabe arrancar un gateway Go y compilar una app Flutter. El objetivo es convertirlo en lo que CVForge ya es: algo que un desconocido obtiene y arranca, algo que un visitante prueba en dos minutos, y una pieza de portafolio en korven.dev que sirve a dos lectores. Con una diferencia decidida de antemano: **Open TV es gratis y de código abierto. No se vende nada.**

**Objetivos, por prioridad:**

1. Un desconocido obtiene y arranca Korven Open TV en menos de un minuto en macOS, Linux o Windows, sin herramientas de desarrollo.
2. `opentv.korven.dev` es la **versión web**, de primera clase: el mismo cliente, nada que instalar, reproduce directamente del emisor el 67–85 % de los canales vivos.
3. korven.dev lo enseña a dos lectores: el usuario ("pruébalo / descárgalo") y el reclutador o cliente potencial de Korven ("así está hecho, lee el código").
4. Todo es honesto: qué se reproduce dónde, qué antigüedad tiene "vivo", qué se decidió no construir.

**Decisiones ya tomadas (no se reabren aquí):**

- Gratis y open source: **MIT + cláusula de marca** (el nombre Korven, el wordmark, el emblema y el icono no se licencian; los forks deben usar su propio nombre y emblema).
- **Web-first**: un cliente web nuevo (Svelte 5 + TypeScript) embebido en el binario Go. La app Flutter de macOS no se toca, no entra en la release v1.0, y se mejora en una fase posterior.
- Proxy de streams **solo en loopback**, nunca en el sitio público. Veredicto `web_ok` hermano de `airplay_ok`.
- Versión hospedada = **snapshot estático** regenerado cada 6 h desde GitHub Actions, hospedaje estático, coste 0 €. Un gateway vivo en un VPS se descartó (≈5 $/mes y atención permanente a cambio de "vivo hace 1 h" en vez de "hace ≤6 h").
- UI en **español e inglés** desde el día uno (diccionario tipado; falta una clave en un idioma → falla el build).
- Canales de obtención: GitHub Release (goreleaser) + Homebrew tap + `go install` + `curl | sh`. Notarización de Apple: **más adelante** (Apple Developer Program, 99 €/año, también lo necesitará la app Flutter). Docker: **diferido** (implica modo LAN sin auth; necesita diseño).
- Soporte por GitHub Issues; WhatsApp solo para la puerta comercial de Korven; sin GitHub Sponsors en el lanzamiento.
- `docs/superpowers/` (specs y planes) **se publica** depurado: para un cliente de Korven el proceso visible spec → plan → gate es parte del producto. `docs/prompts/` no se publica nunca.
- Una sola identidad git, **gdberysan@gmail.com**: el historial se reescribe antes de hacer público el repo.
- El repo se hace público al final de P1, tras pasar el scrub.
- Subdominio `opentv.korven.dev`; rutas del sitio `/open-tv` y `/open-tv/como-esta-hecho`.
- Disciplina de gasto: cualquier despliegue o servicio con coste se anuncia antes de ejecutarse; nada se ejecuta "a ver qué pasa".

### 1.1 Para quién es

- **El espectador**: quiere televisión abierta que funcione, en cualquier sistema operativo o en una pestaña del navegador, sin cuentas ni configuración.
- **El lector**: reclutador o cliente potencial que juzga si Korven sabe construir; lee la página de arquitectura, el repo y los tests, y quiere verlo funcionar en dos minutos desde el teléfono.

### 1.2 No-objetivos (v1.0)

- Paridad con la app de macOS (AirPlay completo, amplitud de códecs de libmpv).
- Cuentas, telemetría, hospedaje de datos del usuario, DRM.
- Canales premium, VPN, elusión de geobloqueo, credenciales de terceros.
- Modo LAN / multiusuario, Docker.
- iOS, Android, Apple TV, Android TV.
- Guía de programación (retirada el 2026-08-08; las razones siguen en pie).

## 2. Arquitectura del entregable

| Artefacto | Qué es | Lo produce |
|---|---|---|
| Binario `open-tv` | Gateway Go + cliente web embebido. `open-tv` sirve `http://127.0.0.1:8080` y abre el navegador. Subcomandos: `serve` (por defecto), `snapshot`. | goreleaser desde un tag: 5 targets + `checksums.txt` |
| `opentv.korven.dev` | El mismo cliente + snapshot estático del catálogo en `/data/*.json` | GitHub Action cada 6 h → proyecto estático en Vercel |
| `install.sh` | Instalador por `curl`, servido en `opentv.korven.dev/install.sh` | El mismo despliegue |
| Fórmula Homebrew | `gdberysan/homebrew-tap` | goreleaser |
| Páginas en korven.dev | `/open-tv`, `/open-tv/como-esta-hecho`, tile del portafolio, imagen OG | Sitio Astro, push a main |

### 2.1 Nueva disposición del repo (la única reestructuración)

El módulo Go pasa a la raíz como `github.com/gdberysan/open-tv`:

```
open-tv/
├── cmd/open-tv/main.go          # entrypoint: serve | snapshot
├── internal/                    # lo que hoy es gateway/internal, movido con git mv
│   ├── domain/  ports/  services/
│   ├── adapters/{db,providers,validator}
│   ├── api/{handlers,middleware,router.go}
│   ├── ui/                      # go:embed de web/dist + SPA fallback
│   ├── proxy/                   # proxy HLS de loopback
│   └── snapshot/                # exportador del catálogo a JSON
├── web/                         # cliente Svelte 5 (package.json, src/, dist/ ignorado)
├── mobile/                      # app Flutter de macOS — INTACTA
├── assets/brand/                # emblema, wordmark, icono + LICENSE de marca
├── tools/                       # scrubcheck, render_icono.swift, plist
├── docs/
├── .goreleaser.yaml
├── go.mod                       # module github.com/gdberysan/open-tv
└── README.md
```

Los imports se reescriben mecánicamente en un único commit; CI es el juez. Esto es lo que hace posible `go install github.com/gdberysan/open-tv/cmd/open-tv@latest` y la historia de "un binario". `PROMPT_MAESTRO.md` §3 se actualiza para reflejarlo.

**El contrato JSON que consume la app Flutter sigue congelado.** Los campos nuevos son solo aditivos (`WebOK`), y los tests de contrato existentes deben seguir pasando sin tocarlos. El job de Flutter de CI se mantiene y debe seguir en verde con cero diffs bajo `mobile/`.

## 3. El cliente web (`web/`)

### 3.1 Stack

Svelte 5 + TypeScript + Vite. hls.js para Chrome/Firefox; HLS nativo del `<video>` en Safari. Vitest (unitarios) + Playwright (Chromium, Firefox, WebKit). Tokens de Korven copiados **tal cual** desde `design_handoff_korven_sitio/tokens/*.css` (la misma regla que la app: el ámbar `#FF8A2B` es señal viva y nada más). Salida `web/dist`, embebida. Objetivo: ≤ 80 KB gzip de JS propio; hls.js se carga perezosamente solo al reproducir.

### 3.2 Pantallas y funciones (v1.0)

- **Catálogo** — alternancia rejilla/lista; tarjeta de canal (logo, nombre, bandera, calidad, barras de señal, estrella de favorito, marca `web_ok`); paginación de 500 con scroll infinito; búsqueda server-side con debounce; filtros país/categoría/calidad desde los mismos endpoints de facetas; "ocultar offline"; canal aleatorio; filtro de favoritos (`?ids=` con el centinela de conjunto vacío, exactamente como la app).
- **Reproductor** — panel con `<video>`; `PlaybackGuard` portado 1:1 (fatal solo si nunca reprodujo, si expira el timeout de carga, o si la posición se congela 8 s tras un error), con tests de temporizadores falsos; al fallar: mensaje claro y, si `web_ok` es falso, "este canal se ve en la app instalada / en Safari". Teclado: espacio, Esc, F, M, ←/→ cambian de canal.
- **Estado** — página de primer arranque "sincronizando el catálogo…" guiada por `/health` (solo local); los mensajes "gateway caído" y "sin red" siguen siendo distintos; línea de frescura "comprobado hace N h" en la versión hospedada.
- **i18n** — diccionario tipado, `es` y `en` completos o falla el build; idioma del navegador por defecto; conmutador en el pie; persistido.
- **Favoritos** en `localStorage`.
- **Botón AirPlay de Safari** (`webkitShowPlaybackTargetPicker`) solo si en el plan resulta trivial; sin lógica de sesión de emisión. Si no, diferido.

### 3.3 Capa de datos

Interfaz `CatalogSource` con dos implementaciones: `HttpCatalog` (API del gateway, local) y `StaticCatalog` (JSON del snapshot, hospedada; filtros y búsqueda en memoria sobre ~12 k canales). Las dos se demuestran equivalentes con **fixtures dorados generados por la suite Go** (consulta → resultado esperado) que Vitest consume. El cliente no sabe en cuál está salvo por la línea de frescura y la página de sincronización.

### 3.4 No se construye

Sesiones de emisión, barra de emisión persistente, memoria de canales rechazados por AirPlay, EPG, modo LAN.

## 4. Cambios en el gateway (`internal/`)

1. **Embed y enrutado.** `web/dist` se sirve en `/`, con fallback SPA a `index.html`; la API sigue bajo `/channels` y `/health`; `Cache-Control` inmutable para assets con hash. El middleware CORS **se elimina**: la UI es del mismo origen y la app Flutter es nativa (nunca lo necesitó). Hoy cualquier web que visites puede leer `localhost:8080`; deja de poder. Flag `--no-browser`; por defecto se abre el navegador en cuanto el servidor escucha.
2. **Veredicto `web_ok`** en el health-checker, hermano de `airplay_ok`: esquema final HTTPS, CORS que admite cualquier origen o el nuestro, `CODECS` del master playlist ⊆ {avc1/avc3, mp4a.40.x} o ausente. Se decide sobre el manifiesto que el checker ya descarga (sin peticiones extra), se guarda en el stream y se expone por canal como campo JSON aditivo (`WebOK`, nullable = sin comprobar, la misma semántica que `Alive`).
3. **Proxy de loopback** `GET /proxy/hls?u=<url>`: reescribe URIs del manifiesto (variantes, segmentos, `EXT-X-KEY`, `EXT-X-MAP`, byte-ranges) para que pasen por el proxy; transmite segmentos con cancelación por contexto; reutiliza el límite de conexiones por host; cabeceras `Referer`/`Origin` eliminadas; ≤ 50 MB por segmento, 30 s por petición. **Solo está activo si la dirección de escucha es loopback, y los builds de snapshot no incluyen la ruta.** El cliente lo usa solo cuando `web_ok` es falso o el intento directo falla ("directo primero, proxy si falla"). Esto revierte, de forma deliberada y acotada, la regla "el gateway nunca proxya vídeo": es el precio de Chrome y Firefox.
4. **`open-tv snapshot --out DIR [--budget 20m]`**: sync único → pasada de salud acotada → exportación de `catalog.json`, `facets.json`, `meta.json` (`generated_at`, recuentos, URL de origen). Es lo que ejecuta la GitHub Action.
5. **Directorio de datos** por sistema como `DB_PATH` por defecto (`~/Library/Application Support/Korven Open TV`, `$XDG_DATA_HOME/korven-open-tv`, `%APPDATA%\Korven Open TV`); todas las variables de entorno existentes siguen valiendo; puerto con fallback 8080 → 8081 → … y la URL elegida se imprime y se abre.
6. **`/health`** añade `version`, `web_ui`, `proxy_enabled`.

## 5. La versión web hospedada (`opentv.korven.dev`)

- **Pipeline:** `.github/workflows/snapshot.yml`, cron cada 6 h + disparo manual: compila `open-tv`, ejecuta `snapshot --budget 20m` (sync desde iptv-org, pasada de salud con la cortesía por host existente, clasificación `web_ok`, exportación), compila `web/` en modo hospedado, ensambla `dist/` = cliente + `/data/*.json` + `install.sh`, despliega con `vercel deploy --prebuilt --prod` (token en secrets del repo). Minutos de Actions en repo público: gratis. Hospedaje: proyecto Vercel nuevo `open-tv-web`, estático, en la misma cuenta que `cvforge.korven.dev`; CNAME del subdominio en el DNS de korven.dev. **Coste: 0 €, anunciado antes del primer despliegue. El único spike es dónde vive el DNS de korven.dev (Vercel o Cloudflare).**
- **Lo que recibe el visitante:** el cliente completo, catálogo de ≤ 6 h con frescura visible, favoritos en su navegador, reproducción **directa desde el emisor** (sin proxy en el sitio público, nunca: `proxy_enabled=false` es estructural), y `web_ok` decidiendo si el botón de reproducir está vivo o dice "este canal se ve en la app instalada / en Safari".
- **Honestidad y postura:** línea de pie/acerca de en ambos idiomas — la fuente es la lista pública FTA de iptv-org; Korven no retransmite nada; sin premium, sin VPN, sin elusión de geobloqueo; línea de contacto/retirada (GitHub Issues + contacto de korven.dev). Las retiradas de iptv-org se propagan en el siguiente snapshot.
- **Superficie de abuso:** solo ficheros estáticos. Nada que llamar, nada que escribir, nada que proxyar.

### 5.1 Censo de reproducibilidad (2026-08-22, base del diseño)

Sondeo de los 8 975 streams vivos de la base local (GET del manifiesto, 6 s, máx. 2 en vuelo por host): 98,2 % respondieron 200; 85 % HTTPS; 99 % HLS; 93,6 % envían `Access-Control-Allow-Origin` (pero 1 359 —Pluto TV— solo admiten `pluto.tv`); 80 % son master playlists, de las que el 0,9 % declaran códecs que ningún navegador decodifica (HEVC 161, AC-3 4); cifrado: 2 streams.

| Dónde está el espectador | Canales que se reproducen sin descargar nada y sin proxy |
|---|---|
| `opentv.korven.dev` en Safari (iPhone, iPad, Mac) | 7 565 de 8 922 — **85 %** |
| `opentv.korven.dev` en Chrome / Firefox | 5 957 — **67 %** |
| Binario instalado, navegador, sin proxy | 77 % |
| Binario instalado **con proxy de loopback** | 8 699 — **97,5 %** |
| App nativa de macOS (libmpv) | ≈ 100 % + AirPlay |

Estos números van a la página de arquitectura y justifican los tres escalones: web, binario, app nativa.

## 6. Corte de release, scrub y onboarding

### 6.1 Scrub (antes del flip, P1)

- Reescritura del historial a una sola identidad (`git filter-repo --mailmap`, nombre y correo normalizados a `Gerard <gdberysan@gmail.com>`), force-push de `main`; las dos ramas de PR fusionadas incluidas. Es seguro ahora (repo privado, sin otros contribuidores) e imposible de hacer limpio una vez público.
- `tools/scrubcheck` en Go, ejecutado sobre **el árbol versionado** de un ref (`git ls-files`, no `git archive`: en un repo público lo que la gente lee es el árbol, y `export-ignore` solo afecta al zip), con una **lista negra privada fuera del repo** (`~/.config/korven/open-tv-denylist.txt`: correos personales, el nombre de usuario del Mac, teléfonos, rutas privadas, la ruta de la carpeta de diseño). Códigos de salida 0 limpio / 1 hallazgos / 2 sin lista, como CVForge. Falla también ante `.DS_Store`, `*.db` versionados y ante cualquier fichero versionado bajo `docs/prompts/` o `.claude/settings.local.json`.
- `docs/prompts/` **se añade a `.gitignore` y no se versiona nunca** (hoy está sin seguimiento; así se queda). `.gitattributes export-ignore` solo para higiene del tarball: `docs/prompts/`, `.claude/`, `.gitattributes`.
- Hallazgos ya conocidos a corregir (verificado con `git grep` el 2026-08-22): `.claude/settings.json` lleva rutas absolutas con el usuario del Mac en los permisos Read/Write (se reescriben como rutas relativas o se retira del seguimiento); la ruta absoluta de la carpeta de diseño en `docs/superpowers/specs/2026-08-08-korven-open-tv-design.md` y en los planes `2026-08-08-korven-open-tv.md` y `2026-08-08-fiabilidad-iptv.md` (se sustituye por una referencia genérica al design system de Korven); `mobile/README.md` aún es la plantilla de Flutter; `docs/risks.md` pre-v3 (se reescribe a la realidad actual o se borra). No existe `scripts/scaffold.sh`; la memoria estaba desactualizada.

### 6.2 Mecánica de release

- `CHANGELOG.md` (Keep a Changelog, en español). `LICENSE` (MIT) + `assets/brand/LICENSE` (aviso de marca: © Korven, todos los derechos reservados; no cubierto por MIT) + línea en el README: los forks usan su propio nombre y emblema.
- goreleaser: versión desde el tag; `darwin/arm64+amd64`, `linux/amd64+arm64`, `windows/amd64`; `checksums.txt`; notas de release en español desde el changelog; fórmula Homebrew empujada a `gdberysan/homebrew-tap`. Corre en CI al empujar un tag (`release.yml`: build web → build Go → goreleaser).
- README como documentación de producto, español primero e inglés después, con cabeceras simétricas: qué es, dónde verlo (web), instalar (brew / línea de instalación / binario / go install), primer arranque, privacidad, lo que no hace, preguntas frecuentes, comandos y variables, licencia y marca, soporte, "para desarrolladores" (incluida la app de macOS congelada y cómo compilarla).
- El repo **se hace público al final de P1**: descripción, topics, social preview y plantillas de Issues (error / canal que no se ve / idea) puestos.

### 6.3 Onboarding

`open-tv` → crea el directorio de datos → escucha en el primer puerto libre → abre el navegador → página de sincronización con progreso → la rejilla se llena al completar el primer sync (segundos) → los veredictos de salud aparecen en los minutos siguientes ("comprobando señal…"). Ctrl-C detiene; un segundo `open-tv` con uno ya corriendo solo abre el navegador a la instancia existente.

`install.sh`: detecta SO/arquitectura, descarga el asset de la release con curl (sin marca de cuarentena), verifica el checksum, instala en `~/.local/bin` (o `/usr/local/bin` si es escribible), lo arranca una vez. Windows: descarga + doble clic (SmartScreen "Ejecutar de todas formas" documentado). Binario de macOS descargado con el navegador: el README documenta `xattr -d com.apple.quarantine` y "Abrir de todas formas" con honestidad, y señala brew y el instalador como caminos limpios.

## 7. Integración en korven.dev y capturas

- `src/data.ts`: objeto `OPENTV` (`name`, `tagline`, `webUrl`, `pageUrl`, `howUrl`, `repoUrl`, `downloadUrl` = última release, `installLine`, `requirements`, `stance`, `waKorven` con un mensaje que nombra Open TV) y segunda entrada de `PORTAFOLIO` (`n: '02'`, estado "v1 · web + descarga"), con copy escrito como contrapartida deliberada de CVForge: una herramienta de pago que corre en tu máquina; una gratis y abierta.
- `Portafolio.astro`: segundo tile del mismo bucle; `shots` gana la captura de Open TV; las etiquetas de los botones pasan a ser datos (`Abrir la web` / `Ver el producto`) para que las de CVForge no cambien.
- `/open-tv` (producto): hero de resultado, tres capturas, "¿Dónde verlo?" con tres CTAs **Abrir la web · Descargar · Ver el código**, qué hace, privacidad (sin cuentas, sin telemetría, sin relay), requisitos, lo que no hace (la postura), preguntas frecuentes, soporte (Issues), y la única puerta comercial: "¿Quieres un sistema así para tu negocio?" → WhatsApp.
- `/open-tv/como-esta-hecho`: la promesa —*lo que ves está vivo, y la herramienta dice que no*—; cómo se verifica (health-checker con histéresis, cortesía por host tras saturar jmp2.uk, clasificadores `web_ok`/`airplay_ok`, los números del censo, gateway caído ≠ sin red); decisiones que muestran criterio (EPG retirado por datos, categorías compuestas, snapshot estático en vez de VPS, proxy solo loopback); lo que no se construyó; stack; CTAs **Ver el código** (repo público) y la puerta de Korven. Sin "pedir acceso".
- OG `public/og-open-tv.png` (1200×630). Capturas: versión web con Playwright a 2× (escritorio y teléfono), script versionado esta vez; app de macOS: una captura de ventana real para la página de arquitectura y un MP4 de ≤ 60 s reproduciendo y emitiendo por AirPlay — **tarea del autor** (permiso de grabación de pantalla o ⌘⇧5). Si el MP4 va en la página, el CSP de `vercel.json` gana `media-src 'self'`; si no, no se toca.
- Herramienta de gate nueva y pequeña: comprobación de enlaces en el repo del sitio (todo href/src de las dos páginas responde 200; los externos son exactamente una lista permitida: repo, release, web, WhatsApp).

## 8. Seguridad y privacidad (delta)

- Sin cuentas, sin telemetría, sin datos del usuario fuera de su máquina o su navegador (favoritos en `localStorage`).
- El gateway local deja de ser legible por webs de terceros al eliminar CORS `*`.
- El proxy no existe fuera de loopback ni en builds de snapshot; cuando existe, solo relaya al navegador de la misma máquina.
- El sitio público no ejecuta nada: ficheros estáticos. Ningún secreto en el cliente. El token de Vercel vive en secrets del repo y solo lo usa la Action.
- El instalador verifica el checksum publicado antes de ejecutar nada.

## 9. Pruebas y gates de calidad

**Automáticas (CI, cada push):**

- Go: gofmt, vet, build, `test -race`, con tests nuevos para el enrutado del embed, el clasificador `web_ok`, la reescritura de manifiestos del proxy (corpus de fixtures), la exportación del snapshot, el directorio de datos y el fallback de puerto, y `scrubcheck`. Los tests de contrato congelado no se tocan.
- Web: Vitest (guard con temporizadores falsos, filtros, completitud de i18n, las dos `CatalogSource` contra los fixtures dorados de Go, diccionario tipado).
- **Playwright en Chromium, Firefox y WebKit** contra un `open-tv` real que sirve un M3U local con un fixture HLS H.264/AAC generado con ffmpeg: abrir → página de sync → rejilla → filtrar → reproducir (los frames avanzan) → matar el stream → el guard declara fatal con el mensaje correcto.
- El job de Flutter sigue y debe seguir en verde con cero diffs bajo `mobile/`.
- El workflow de snapshot tiene un modo de ensayo que CI ejecuta.

**Manuales, por fase:** click-through en tu Mac (Safari y Chrome) y en el teléfono, registrado dentro del plan; instalación en máquina limpia (vale una cuenta de usuario nueva) por brew y por instalador; un amigo; usarlo como tu televisión varias noches antes del lanzamiento.

## 10. Secuencia de construcción

Cada fase termina en un gate observable, no en una sensación.

| Fase | Contenido | Gate |
|---|---|---|
| **P0** | Reestructuración del módulo; embed; `web_ok`; proxy; directorio de datos / puerto / abrir navegador; cliente Svelte MVP (ES/EN); guard; Playwright ×3 en CI | `go run ./cmd/open-tv` → los canales se reproducen en Chrome, Firefox y Safari en tu Mac; CI verde en tres motores; `mobile/` intacto y verde |
| **P1** | Reescritura del historial; scrubcheck + lista negra; LICENSE + marca; CHANGELOG; README como docs de producto; goreleaser + tap + `install.sh`; tag `v1.0.0`; **repo público** | Un desconocido obtiene un binario funcionando desde el repo público por brew, instalador o descarga |
| **P2** | Pulido del primer arranque, mensajes, casos límite de puerto y directorio, comprobaciones en Windows/Linux, fricción de docs | Un amigo instala sin ayuda y ve canales reproduciéndose |
| **P3** | Comando `snapshot`; `StaticCatalog`; workflow de 6 h; proyecto Vercel; subdominio; copy de honestidad; frescura | `opentv.korven.dev` reproduce canales en tu teléfono; 0 €; nada que abusar |
| **P4** | `OPENTV` en data.ts; tile; `/open-tv`; `/open-tv/como-esta-hecho`; OG; capturas; comprobación de enlaces; despliegue | Enlaces en verde; leído en un teléfono; un desconocido llega a la release o a la web en dos clics desde el tile |
| **P5** | Checklist de lanzamiento: capturas finales, copy leído en ambos idiomas, plantillas de Issues, topics y social preview, notas de release, vídeo de la app de macOS | El post de lanzamiento del autor |

P0 es la fase grande (1–2 semanas). P1 puede solapar su cola. P3 depende de P0; P4 de P3.

## 11. Riesgos

| Riesgo | Mitigación |
|---|---|
| Manifiestos exóticos rompen el proxy | Corpus de fixtures sacado del censo; "directo primero, proxy si falla"; el fallo degrada a mensaje, nunca a cuelgue |
| Churn de Svelte 5 / Vite | Versiones fijadas; el cliente es pequeño |
| El job de snapshot excede el presupuesto | Pasada de salud acotada; un snapshot parcial es válido y lo dice `meta.json` |
| Quejas de emisores sobre korven.dev | Procedencia iptv-org explícita, sin relay, línea de contacto, retirada a petición |
| Fricción de Gatekeeper para usuarios no técnicos de macOS | brew e instalador primero; notarización más adelante; README honesto |
| La reestructuración del módulo rompe CI o rutas | Un único commit mecánico; CI como juez; `mobile/` no participa |
| Deriva entre `HttpCatalog` y `StaticCatalog` | Fixtures dorados generados por Go y consumidos por Vitest |

## 12. Diferido, con razones

- **Lanzador `.app` notarizado y release de la app Flutter** — requiere Apple Developer Program (99 €/año). Cuando se contrate, una sola cuenta notariza ambos artefactos y el cableado de CI se hace una vez.
- **Docker / modo LAN** — implica escuchar en la red sin autenticación; necesita un diseño propio, no una línea en goreleaser.
- **AirPlay más allá del selector de Safari** — necesita capa nativa; la app de macOS ya lo hace.
- **Gateway vivo hospedado** — solo si el tráfico o una promesa de "vivo ahora" lo justifican (≈5 $/mes + atención); la costura `CatalogSource` lo convierte en un despliegue, no en una reescritura.
- **iOS / Android / TV** — fase 9 del roadmap.
- **Registro de la marca Korven (IMPI)** — la marca pasa a ser pública; fuera de este proyecto.
- **Páginas de korven.dev en inglés** — seguimiento del sitio, no de este proyecto.
- **GitHub Sponsors** — enturbia "gratis, no se vende nada"; se puede añadir después.

## Nota de enmienda (2026-08-25)

Al ejecutar el plan de P0 (`docs/superpowers/plans/2026-08-25-p0-cliente-web-y-binario.md`), leer el código real reveló que dos afirmaciones de esta spec eran falsas. Se documentan aquí sin reescribir el cuerpo de arriba, que queda como registro histórico de lo que se diseñó antes de implementar.

1. **§4.2 dice que `web_ok` es "hermano de `airplay_ok`" y da a entender que ambos se guardan en el stream. `airplay_ok` NO está persistido.** Se sondea bajo demanda en `/channels/stream` (`handlers.AirplayProber`, con caché en memoria y TTL de 12 h), y el health-worker calcula `res.Airplay` pero lo descarta — solo persiste `IsAlive`/`LatencyMs`. `web_ok` sí se persiste, porque la rejilla necesita pintarlo para 500 canales a la vez y un sondeo por canal no sirve para eso. Así que `web_ok` y `airplay_ok` son hermanos en **semántica** (tri-estado: `null` = sin comprobar, `true`/`false` según lista de permitidos) pero NO en almacenamiento: uno se guarda, el otro no.

2. **§3.2 da a entender que `web_ok` cubre la reproducción en el navegador en general. `web_ok` es el veredicto ESTRICTO de hls.js (Chrome/Firefox), no el de Safari.** Safari reproduce HLS de forma nativa sin necesitar CORS, así que reproduce más de lo que `web_ok` admite — el censo lo confirma: 85 % en Safari frente a 67 % en Chrome/Firefox (que sí necesitan CORS además de HTTPS). Por eso el cliente decide la política de intento según el motor (`planDeReproduccion`, Tarea 13): en HLS nativo se intenta directo con solo comprobar HTTPS, aunque `web_ok` sea falso. `web_ok` no debe usarse para bloquear la reproducción en Safari.

Ambos puntos ya estaban recogidos como desviaciones decididas en `docs/superpowers/sdd/2026-08-25-p0-cliente-web-y-binario/global-constraints.md` (desviaciones 1 y 3); esta nota los deja también en la spec original, que es donde alguien los buscaría primero.
