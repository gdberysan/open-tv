# P2 — Guía de programación (EPG) por fuente, con degradación honesta

**Estado:** diseño aprobado en brainstorming (2026-08-26). Autoridad para el plan de implementación.

## 1. Contexto y objetivo

Hoy Open TV dice *qué canales existen* (nombre, logo, calidad, señal viva) pero
no *qué están dando*. Este trabajo añade **ahora/después** por canal: en la
tarjeta del catálogo y en el overlay del reproductor se ve el programa en
emisión y el siguiente — como una televisión de verdad — **sin configurar
nada** y **sin mentir cuando no hay datos**.

### 1.1 Por qué falló el intento anterior (y qué cambia)

El EPG se construyó y se retiró en el proyecto pre-web (commit `517f065`). No
fue un fallo técnico: el parser tragaba 38 MB sin despeinarse y el ADR-001
quedó marcado «REVERTIDO… la decisión técnica era correcta, el problema eran
los datos». El muro fue la **estrategia de datos**: se apuntaba a **una** URL
XMLTV global, cuyos ids casaban con solo **478 de 12 639 canales (465 de
India)**. La parrilla salía vacía para casi cualquier canal real, y la única
forma de cubrir más era montar y hospedar el grabber de iptv-org/epg. 1 875
líneas y 32 tests para ~5% de cobertura → se borró.

El diseño se **invierte**:

1. **EPG declarado por la propia fuente.** Las playlist M3U anuncian su guía en
   la cabecera: `#EXTM3U url-tvg="…"`. El parser actual **descarta** esa
   cabecera. Al leerla, la guía de cada fuente trae ids del **mismo proveedor**
   que los canales → el join está **garantizado**, y la cobertura es alta *para
   esa fuente*. Esto ataca el fallo exacto (ids que no casan) en la raíz.
2. **Cobertura parcial = el estado normal DISEÑADO, no un defecto.** ahora/
   después donde hay datos; «sin guía» donde no — nunca un programa falso o
   caducado. Misma honestidad que el indicador de señal. El listón de éxito NO
   es «cobertura universal» (esa fue la trampa que lo mató).
3. **Pequeño y perezoso.** `tvg_id` sobrevive en el esquema y está **indexado**;
   el join ya medio existe. Nada de reconstruir el subsistema de 1 875 líneas.

### 1.2 Decisiones cerradas (brainstorming 2026-08-26)

- **Superficie:** ahora/después primero, **preparado para rejilla**. Insignia en
  la tarjeta + línea now/next en el overlay del reproductor. Se almacena una
  ventana rodante con los programas completos, de modo que una rejilla-línea de
  tiempo futura sea una consulta más ancha sobre la misma tabla, **sin retrabajo**.
- **Captura de `url-tvg`:** **solo automática desde la cabecera** `#EXTM3U`
  (`url-tvg` / `x-tvg-url`). Sin campo manual en la UI. Una fuente sin `url-tvg`
  declarada simplemente no tiene guía (degradación honesta).
- **Ventana:** aprox. `now−2h … now+48h`. **Cadencia de refresco:** refetch si la
  guía tiene más de ~6h. **Tarjetas sin guía:** se omite la insignia (sin ruido
  «sin guía» en cada tarjeta); la honestidad explícita vive en el overlay.

## 2. Principios de diseño

Por fuente y declarado por el proveedor · cobertura parcial honesta por diseño ·
pequeño, perezoso, **aditivo** · se guardan los programas completos de una
ventana para no rehacer nada al añadir la rejilla · **`mobile/` cero diffs**,
contratos JSON existentes intactos · sin dependencias nuevas (solo stdlib).

## 3. Flujo de datos, de punta a punta

```
#EXTM3U url-tvg="…"  ──parse de cabecera──▶  providers.tvg_url
        (por fuente, en cada sync)
                │  refresco EPG (dentro del ciclo por-fuente del Syncer)
                ▼
   fetch XMLTV (gzip, con cap de tamaño)  ──parse en streaming──▶  epg_programmes
                                                        (ventana rodante,
                                                         por provider_id)
                │  join: channel(provider_id, tvg_id) = programme(provider_id, channel_id)
                ▼
   GET /channels/epg?ids=…      +      GET /channels/{id}/epg
                │
                ▼
   insignia de tarjeta «● Ahora: …»   ·   now/next del overlay del reproductor
```

## 4. Componentes

### 4.1 Captura de `url-tvg` (la pieza clave)

- **`internal/adapters/providers/opensource/provider.go`** — `parseM3UStream`
  hoy solo mira líneas `#EXTINF:`. Se amplía para leer la **primera** línea
  `#EXTM3U` y extraer los atributos `url-tvg` y su alias `x-tvg-url` (pueden ser
  una lista separada por comas → se conserva la lista). Se devuelve junto a los
  canales: `parseM3UStream` pasa a devolver también `tvgURLs []string` (o un
  struct `ResultadoM3U{Canales, StreamURLs, TvgURLs}` para no crecer la lista de
  retornos). `GetLiveChannels` expone la(s) URL(s) al llamador. **Ambas vías
  (HTTP y file://) comparten el parser, así que un M3U subido a mano que declare
  `url-tvg` también trae guía.**
- **Puerto:** `ports.ProviderPort` gana una vía para recuperar la(s) `tvg_url`
  tras `GetLiveChannels` (p. ej. `TvgURLs() []string`, poblado en el último
  parse), o se devuelve en un resultado enriquecido. La decisión exacta de firma
  es del plan; el invariante es: **la URL de guía sale del mismo parse que los
  canales.**

### 4.2 Persistencia

- **`providers` gana `tvg_url TEXT`** (ALTER idempotente, mismo patrón que las
  ALTER de label/kind de P0.7). Se sella en cada sync desde la cabecera; vacío =
  la fuente no declara guía.
- **Nueva tabla `epg_programmes`:**
  ```sql
  CREATE TABLE IF NOT EXISTS epg_programmes (
      provider_id  TEXT    NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
      channel_id   TEXT    NOT NULL,   -- id del <channel> XMLTV (= tvg-id del M3U)
      start_utc    INTEGER NOT NULL,   -- epoch segundos, UTC
      stop_utc     INTEGER NOT NULL,   -- epoch segundos, UTC
      title        TEXT    NOT NULL,
      sub_title    TEXT,
      description  TEXT,
      PRIMARY KEY (provider_id, channel_id, start_utc)
  );
  CREATE INDEX IF NOT EXISTS idx_epg_lookup
      ON epg_programmes (provider_id, channel_id, start_utc);
  ```
  `ON DELETE CASCADE` desde `providers`: borrar una fuente se lleva su guía. Se
  guardan los **programas completos** de la ventana (no solo now/next) → la
  rejilla futura es una consulta más ancha, sin cambio de esquema ni de ingesta.
- **Ventana rodante:** al refrescar, se reemplaza la guía de esa fuente por los
  programas con `stop_utc >= now−2h` y `start_utc <= now+48h`; el resto se poda.
- **Aditivo y seguro:** tabla nueva; `/channels`, `/sources` y el contrato JSON
  congelado de Flutter (15 claves) **no cambian**. `epg_entries` de la vieja
  época no se reusa (nombre y forma nuevos).

### 4.3 Join y aislamiento por fuente

Los programas se claven por **(provider_id, channel_id XMLTV)** y un canal une
por **(provider_id, tvg_id)**. Aislar por `provider_id` evita que un mismo
`tvg_id` presente en dos fuentes distintas se contamine: cada canal ve **solo la
guía de su propia fuente**. `tvg_id` vacío → sin guía, limpio.

### 4.4 Refresco y frescura (honesta por construcción)

- **`internal/services/syncer.go`** ya itera fuentes con `PerSourceTimeout` y
  éxito parcial. El refresco de EPG se **pliega** a ese ciclo: tras sincronizar
  los canales de una fuente, si tiene `tvg_url` y su guía está ausente o tiene
  más de ~6h, se hace fetch+parse+reemplazo de su ventana. **Cadencia propia** e
  **independiente del éxito del sync de canales** (mismas reglas de éxito parcial
  que P0.7: el fallo de la guía de una fuente no tumba el ciclo).
- **Poda** de programas caducados en cada refresco.
- **Frescura honesta:** now/next se computa de los programas guardados en tiempo
  de consulta. Si **ningún programa cubre `now`** (feed rancio, sin `tvg_url`, o
  fuente sin guía) → «sin guía». **Nunca** un programa pasado o inventado.

### 4.5 Cómputo de ahora/después + zonas horarias

- Las horas XMLTV traen offset (`20260826200000 +0000`). Se parsean a **UTC
  (epoch)** en la ingesta; el cliente pinta la hora local.
- `ahora` = programa con `start_utc <= now < stop_utc` para (provider_id,
  channel_id). `siguiente` = el inmediato posterior por `start_utc`. Casos
  límite explícitos: exactamente en `start`/`stop`, hueco entre programas, tabla
  vacía, feed enteramente en el pasado → todos caen a «sin guía» sin lanzar.

### 4.6 API (nueva, aditiva, segura para Flutter)

- **`GET /channels/epg?ids=<id,…>`** — para cada channelID pedido, su `ahora` y
  `siguiente` (título, inicio, fin). En lote, para la ventana visible de la
  rejilla. Reutiliza el patrón `ids=` que ya existe en `/channels`. El cliente le
  pasa los ids que el virtualizador tiene en pantalla y refresca por intervalo.
- **`GET /channels/{id}/epg`** — `ahora` + unos pocos `próximos` de ESE canal,
  para el overlay del reproductor.
- **`/channels` NO cambia.** now/next varía con el tiempo y depende del conjunto
  visible, así que **no** se hornea en los objetos de canal (rompería el modelo y
  acoplaría datos temporales al catálogo). Endpoints aparte, montados bajo el
  `Route("/channels", …)` existente en `internal/api/router.go`.
- Forma de respuesta (snake_case, como el resto): `{ "<channelID>": { "ahora": {
  "titulo","inicio","fin" } | null, "siguiente": { … } | null } }`. `null`
  explícito = sin guía (el cliente distingue «sin datos» de «hueco»).

### 4.7 Superficie de cliente

- **Store `epg`** (`web/src/estado/epg.ts`, patrón de los otros stores): cacheado
  por channelID con TTL corto; dado el set visible del virtualizador, pide
  `/channels/epg?ids=…`, cachea y refresca por intervalo (unos minutos) y al
  hacer scroll. Sin peticiones por canal individual en la rejilla.
- **`TarjetaCanal.svelte`:** cuando hay `ahora`, muestra `● Ahora: <título>` y
  `Sig HH:MM · <título>`; **sin guía → la tarjeta queda limpia** (nada de «sin
  guía» en cada tarjeta). Texto plano → lo lee el lector de pantalla; **sin
  regiones aria-live nuevas** (invariante P0.6/P0.8).
- **`Reproductor.svelte` (overlay):** línea now/next honesta del canal abierto
  vía `/channels/{id}/epg`; «sin guía para esta fuente» explícito aquí (no en la
  rejilla). Respeta `prefers-reduced-motion` (sin animación nueva salvo la ya
  existente del overlay).
- i18n ES/EN en paridad para las cadenas nuevas («Ahora», «Sig», «sin guía»).

### 4.8 Preparado para rejilla (sin retrabajo)

Como se guardan los **programas completos** de la ventana, una futura rejilla
canal×tiempo es (a) una consulta de rango más ancha sobre `epg_programmes` y (b)
una vista nueva. **Cero** cambios de esquema, de ingesta o de la captura de
`url-tvg`. Fuera de alcance en P2; el diseño solo garantiza que no habrá que
rehacer nada.

## 5. Restricciones (Global Constraints del plan)

- **`mobile/` CERO diffs.** Contrato JSON congelado (15 claves) intacto — EPG va
  en endpoints y tabla NUEVOS que Flutter no toca.
- **Sin dependencias Go/JS nuevas:** XMLTV con `encoding/xml` en streaming
  (`Decoder.Token`), gzip con `compress/gzip`, ambos stdlib. Cliente: solo fetch
  + render.
- **Cap de tamaño** del XMLTV descargado (mismo criterio que el cap de 50 MB del
  M3U, con `io.LimitReader`); gzip en streaming, sin materializar el XML entero.
- **SSRF / confianza:** la `tvg_url` la declara la playlist que el usuario ya
  eligió — mismo modelo de confianza y mismo cliente HTTP guardado que el fetch
  del M3U (gateway loopback de un solo usuario). No abre una superficie nueva.
- **Bundle propio ≤ 80 KB gzip.** hls.js sigue perezoso.
- **Identidad `gdberysan@gmail.com`** en cada commit; trailer Claude.
- **a11y P0.6/P0.8 intacta:** sin regiones live nuevas; roving/inert/focus-trap
  sin tocar; reduced-motion respetado.

## 6. Métrica de éxito (declarada por adelantado)

El éxito es que **ahora/después se enciende para las fuentes que traen guía**.
La cobertura parcial es lo **esperado**, no un defecto — no hay listón de
cobertura universal. Una fuente con `url-tvg` compatible muestra guía real; una
sin ella queda honestamente en blanco. Ese es el criterio que evita repetir la
retirada del intento anterior.

## 7. Pruebas y gates

- **Go:** parser XMLTV con fixtures (offsets de zona, `.xml` y `.xml.gz`, XML
  malformado, tope de tamaño), cómputo ahora/después (límites exactos en
  start/stop, hueco, vacío, feed rancio), aislamiento por fuente (colisión de
  `tvg_id` entre dos providers), extracción de cabecera (`url-tvg` único / lista
  / `x-tvg-url` / ausente), poda de la ventana. Gates de siempre (`gofmt`,
  `vet`, `build`, `test -race`) y `golangci-lint` (incluir el tool nuevo en el
  lint — lección de P1: los gates de una tarea Go DEBEN incluir golangci-lint).
- **Web:** la insignia pinta ahora/después / queda limpia sin datos; TTL y
  refresco del store `epg`; overlay con «sin guía»; paridad i18n.
- **Playwright:** una fuente con un XMLTV de fixture enciende now/next; una
  fuente sin guía se queda honestamente vacía; ambas sobre un `open-tv` real.
- **`mobile/` cero diffs** verificado en cada tarea.

## 8. No-objetivos (fuera de P2)

- **Rejilla canal×tiempo** (timeline). El esquema la deja lista; la vista es
  trabajo posterior.
- **Correr/hospedar el grabber de iptv-org/epg.** Es la única vía a cobertura
  universal y NO cabe en un binario local de un solo usuario (infraestructura +
  scraping). Si algún día la versión web hospedada (P3) quiere guía más rica para
  las *fuentes sugeridas*, podría hornearla en servidor — opcional y posterior.
- **URL de EPG manual por fuente.** Descartado por decisión: solo captura
  automática desde la cabecera.
- **Descripciones largas / imágenes de programa / búsqueda en la guía.** Se
  guarda `description` por si se usa luego, pero la UI de P2 es ahora/después.

## 9. Riesgos

| Riesgo | Mitigación |
|---|---|
| Una fuente declara `url-tvg` pero sus ids no casan con sus `tvg_id` | Degradación honesta: sin match → sin guía. No rompe nada; es el caso «sin datos». |
| XMLTV enorme de un proveedor hostil/roto | Cap de tamaño + gzip en streaming, igual que el M3U. |
| Feed con horas sin offset o mal formadas | Parser tolerante: entrada no parseable se descarta (esa entrada), no tumba el fetch; se registra. |
| El refresco por-fuente alarga el ciclo del Syncer | `PerSourceTimeout` ya acota cada fuente; el EPG hereda ese aislamiento y su propia cadencia (~6h) evita refetch en cada ciclo. |
| Percepción de «cobertura baja» como en el intento anterior | La métrica de éxito (§6) y la degradación honesta se documentan como el estado esperado, no como fallo. |
