# Diseño — Tiempo hasta la imagen (bucle de verdad por mirror)

> **Fecha:** 2026-09-06 · **Estado:** pendiente de revisión del dueño
> **Origen:** item 3 de la hoja de ruta al estado del arte
> (0 arnés → 1 bucle de verdad → 2 salud por segmento → **3 tiempo hasta la
> imagen** → 4 EPG). Sucesor directo de
> `2026-09-05-salud-por-segmento-design.md`: el veredicto de códecs deja
> pasar al mirror H.264 de AMC (720p), que sigue sin verse porque no da la
> señal a tiempo.

---

## 1. Objetivo

Que el catálogo sepa **qué mirrors dan imagen de verdad desde esta máquina,
y en cuánto tiempo**, para que el reproductor no gaste su presupuesto en un
origen que ya falló aquí, para que el orden de los mirrors sea el de la
imagen real y no el de la latencia del manifiesto, y para que la tarjeta
diga «Imagen en 2,1 s» o «Sin imagen desde aquí» en vez de «Señal viva
158 ms».

### El caso medido

AMC (720p), tras salud por segmento: su mirror MPEG-2 ya se salta; el
mirror `41.205.93.154` es H.264 **sin pista de audio** y sirve 4,9 MB por
cada 6 s de vídeo (≈6,5 Mbit/s necesarios) a **2,1 Mbit/s sostenidos**
(medido: 1,3 MB en 5,0 s tras el arranque TCP). El reproductor lo intenta,
agota los 20 s del guard y muestra «Se probaron 1 mirrors». Verdad, pero
inútil: el usuario no sabe que ese origen NUNCA va a dar imagen desde aquí.

### Por qué NO se mide la velocidad desde el servidor (medido, 2026-09-06)

Se sondearon 150 mirrors HLS vivos con H.264 desde esta máquina:

- Con **256 KB** por Range, la tasa medida sale entre 3 y 5 veces por
  debajo de la real: el arranque lento de TCP domina (con 0,55 s de ida y
  vuelta, 256 KB no salen del slow-start). AMC daba 0,56 Mbit/s con 256 KB
  y 2,1 Mbit/s con 1,3 MB más. Un origen lejano y rápido se confundiría con
  uno lento: **falsos «no se ve»**.
- Una muestra fiable necesita ≥1,5 MB por mirror. Solo para el primer
  mirror de cada canal son ~9.000 × 1,5 MB ≈ **13 GB al día** en la máquina
  del usuario. Inaceptable para una app local.
- El tiempo total de los 16 KB que ya lee la sonda de códecs (p50 0,65 s,
  p90 1,5 s, AMC 0,82 s) **no separa** a AMC del resto.
- La verdad es **por red del usuario**: un origen lento desde México puede
  ser perfecto desde Berlín. El único medidor honesto y gratis es el propio
  reproductor, que ya sabe cuánto tarda cada mirror en dar imagen y cuándo
  no la da.

**Decisión del dueño (2026-09-06):** bucle de verdad del reproductor,
persistido en el servidor, por mirror. Sin sonda de velocidad. Mirrors sin
audio: se **relegan** y se etiquetan, no se saltan. La tarjeta muestra el
tiempo real hasta la imagen cuando se conoce.

---

## 2. Arquitectura

```
Reproductor ── intento sobre mirror U ──► iniciado (ms hasta la imagen) | fallo (clase)
      │
      └─ POST /streams/desenlace {url, resultado, motivo, ms_primer_frame}   (mismo origen)
                    │
            streams.imagen_ms / fallos_reales / ultimo_desenlace_at / ultimo_motivo
                    │
     GET /channels/streams ─► audio_ok, imagen_ms, sin_imagen, ultimo_fallo_hace_s  (aditivo)
     GET /channels/imagen  ─► {canal: {imagen_ms, sin_imagen}} solo canales con datos
                    │
     cliente: failover salta sin_imagen (y codec_ok=false), respeta el orden del
              servidor (sin audio al final), mensaje honesto + «Probar de todos modos»;
              tarjeta: «Imagen en 2,1 s» / «Sin imagen desde aquí»
```

Todo es local: el desenlace viaja del navegador al gateway de la misma
máquina, protegido por `MismoOrigen`. Nada sale a la red.

---

## 3. Servidor

### 3.1 Qué es un desenlace real

El reproductor reporta cada intento sobre un mirror que **de verdad
intentó**:

- `iniciado` con `ms_primer_frame` (lo que ya mide `PlaybackGuard.alConfirmar`).
- `fallo` con su `motivo` (la `ClaseFallo` de `diagnostico.ts`).

**No se reporta** un intento cuando: el motor está forzado a nativo
(cast AirPlay en curso), la pestaña estuvo oculta durante el intento (el
guard congela el presupuesto, pero un intento pausado no dice nada del
origen), o `navigator.onLine === false`.

**Clases que cuentan como fallo REAL del origen:** `desconocido` (timeout
sin imagen), `inestable`, `caido`, `caducado`. **No cuentan** (se guardan
como `ultimo_motivo` para stats, pero no tocan `fallos_reales`): `geo`
(no es el origen, es dónde estamos), `formato` y `codec` (ya tienen su
propio veredicto y su propio mensaje).

### 3.2 Persistencia (aditiva, en `streams`)

```sql
audio_ok             INTEGER CHECK (audio_ok IN (0,1)),   -- NULL = sin sondear (sale de la PMT)
imagen_ms            INTEGER NOT NULL DEFAULT 0,          -- ÚLTIMO tiempo real hasta la imagen; 0 = nunca
fallos_reales        INTEGER NOT NULL DEFAULT 0,          -- fallos reales CONSECUTIVOS
ultimo_desenlace_at  INTEGER NOT NULL DEFAULT 0,          -- epoch s del último desenlace (éxito o fallo)
ultimo_motivo        TEXT    NOT NULL DEFAULT ''
```

En `schema.sql` Y en `alterMigrations`, como siempre.

- `audio_ok` lo escribe la sonda de códecs ya existente, en el mismo
  `MarkBatch`: `CodecOK/No` → además `AudioOK` si la PMT trae algún stream
  de audio (`0x03, 0x04, 0x0f, 0x11, 0x81, 0x87`), `AudioNo` si no lo trae,
  `AudioUnknown` si la sonda no decidió. `Unknown` nunca pisa.
- **Éxito:** `imagen_ms = ms_primer_frame`, `fallos_reales = 0`,
  `ultimo_desenlace_at = now`, `ultimo_motivo = ''`.
- **Fallo real:** `fallos_reales += 1`, `ultimo_desenlace_at = now`,
  `ultimo_motivo = motivo`. `imagen_ms` NO se toca: sigue siendo la última
  vez que sí se vio.
- **Fallo no real** (`geo`/`formato`/`codec`): **solo `ultimo_motivo`**
  (para stats). `ultimo_desenlace_at` **NO se toca** — corregido tras la
  revisión de rama completa (defecto de spec, no del código): un geo 403
  en un mirror cuyos fallos reales ya caducaron (pasadas las 24 h de
  §3.3) no debe re-armar la ventana y volver a saltarlo. Para "es geo" ya
  existe el motivo/mensaje de clase (`reproductor.error.geo`); la ventana
  de «sin imagen» es solo para fallos reales.

### 3.3 La regla de «sin imagen desde aquí»

Misma filosofía que la histéresis del health-check (`DeadFailThreshold =
3` chequeos para declarar muerto): **un mirror se salta cuando
`fallos_reales >= 2` y `ultimo_desenlace_at` está dentro de las últimas
24 h.** Pasadas 24 h vuelve a tener una oportunidad. Constantes con test:
`UmbralFallosReales = 2`, `VentanaFallosReales = 24h`. Se evalúa en el
servidor (una función pura en `domain`, `SinImagen(fallos, ultimoAt, ahora)`)
y viaja ya decidida en el cable: el cliente **nunca** rederiva la regla.
Importante: **solo un fallo real actualiza `ultimo_desenlace_at`** (§3.2);
un fallo no real (geo/formato/codec) nunca re-arma la ventana, así que una
racha de fallos reales ya caducada no revive por culpa de un motivo ajeno
al origen.

### 3.4 Orden de los mirrors

`FindMirrorsByChannelID` pasa a ordenar:

1. vivos antes que muertos (como hoy);
2. **con audio antes que sin audio** (`audio_ok = 0` al final del tramo);
3. dentro de cada tramo, **`imagen_ms` conocido ascendente** primero
   (0 = desconocido va después de cualquier conocido);
4. luego latencia del manifiesto ascendente (como hoy).

Los mirrors saltables (`sin_imagen`, `codec_ok = 0`) se devuelven igual,
con sus flags: el cliente decide, exactamente como con `codec_ok`.

### 3.5 API (aditiva)

- **`POST /streams/desenlace`** (mismo origen; el middleware `MismoOrigen`
  es global). Cuerpo acotado (mismo `maxCuerpoPlayback`):
  `{ "url": string, "resultado": "iniciado"|"fallo", "motivo": string, "ms_primer_frame": number }`.
  Aplica §3.2 sobre TODAS las filas con esa URL (dos canales pueden
  compartir origen). URL desconocida → **204 sin cambios**, nunca error
  (un mirror podado en un sync no rompe la reproducción). Body inválido →
  400. Escribe con el pool de ESCRITURA (como `/sources`).
- **`GET /channels/streams`** gana: `audio_ok` (bool nullable), `imagen_ms`
  (int, 0 = desconocido), `sin_imagen` (bool, §3.3 ya evaluada),
  `ultimo_fallo_hace_s` (int, segundos desde el último fallo real; 0 =
  ninguno).
- **`GET /channels/imagen`** → `{ "<channel_id>": { "imagen_ms": int, "sin_imagen": bool } }`
  solo para canales con algún desenlace registrado (tamaño: los canales
  que el usuario ha sintonizado). Por canal: `imagen_ms` = el menor
  `imagen_ms > 0` entre sus mirrors vivos; `sin_imagen` = TODOS sus mirrors
  vivos están saltados (por códec o por §3.3).
- **`GET /stats`** → `catalogo` gana `sin_imagen` (mirrors) e
  `imagen_p50_ms` (mediana de `imagen_ms > 0`).
- **`/channels` y `/sources` NO cambian.** 15 claves intactas.

---

## 4. Cliente

### 4.1 Datos

`Mirror` gana `audioOk?: boolean | null`, `imagenMs?: number`,
`sinImagen?: boolean`, `ultimoFalloHaceS?: number` (opcionales, misma regla
`?? null`/`?? 0`/`?? false` que `codecOk`). `DesenlaceReproduccion` gana
`url: string` (el `Intento` ya la tiene).

Nuevo `estado/desenlaces.ts`: `reportarDesenlaceMirror(d)` aplica las
exclusiones de §3.1 (motor forzado, pestaña oculta durante el intento,
offline) y hace `POST /streams/desenlace` best-effort, como
`estadisticas.ts`. El reporte a `/stats/playback` sigue igual.

Nuevo `estado/imagen.ts`: store con el mapa de `GET /channels/imagen`, se
carga al arrancar y se refresca tras cada desenlace reportado (con un
pequeño debounce: un cambio de canal rápido no dispara diez fetches).

### 4.2 Failover

Antes de `planDeFailover`, un único filtro en un solo sitio:

```
reproducibles = mirrors sin codecOk === false y sin sinImagen === true
```

El orden es el del servidor (ya relega los sin audio). Si había mirrors y
no queda ninguno:

- todos descartados por códec → mensaje de códec de hoy, sin botón (igual);
- alguno descartado por `sinImagen` → **mensaje nuevo**
  `reproductor.error.sinImagen`: «Ningún origen de este canal llega a dar
  imagen desde aquí (último intento hace {hace}).» con el botón
  **«Probar de todos modos»** (`reproductor.error.probarIgual`). Ese botón
  llama a `reproducir({ ignorarSinImagen: true })`: recorre la cadena
  ignorando `sinImagen` (los saltos por códec se mantienen). Si un origen
  se recuperó, su éxito pone `fallos_reales = 0` y limpia su propio
  registro. `{hace}` se formatea con la utilidad de tiempo relativo que ya
  usa «continuar viendo» (o una mínima si no existe: «hace 3 h», «hace 20 min»).

Desenlace reportado en ese caso a `/stats/playback`: `motivo: 'sinImagen'`,
`via: 'ninguna'`.

### 4.3 Sin audio

Mientras se reproduce un mirror con `audioOk === false`, el overlay del
escenario muestra una nota persistente y discreta
`reproductor.sinAudio`: «Sin audio en este origen». Vive dentro de la
región persistente del reproductor, sin región `aria-live` nueva; se
anuncia por la región sr-only de App que ya anuncia «Reproduciendo X»
(añadiendo «, sin audio»).

### 4.4 Tarjeta

`SenalCanal` gana `imagenMs?: number` y `sinImagen?: boolean`:

- `sinImagen` → punto en color de error y texto «Sin imagen desde aquí»
  (`senal.sinImagen`), también en el `aria-label`;
- `imagenMs > 0` → «Imagen en 2,1 s» (`senal.imagenEn`, un decimal);
- si no, lo de hoy (`latenciaMs` ms).

Los tres llamadores (`ListaCanalesLateral`, `RejillaCanales`,
`TarjetaCanal`) leen el store de `estado/imagen.ts` por `canal.id`. **El
orden del catálogo no cambia**: el dato es informativo.

---

## 5. Pruebas

**Go** (gates: `gofmt`, `go vet`, `go build`, `go test -race`,
`golangci-lint run`, `scrubcheck`):

- `domain`: `SinImagen` (umbral, ventana, frontera exacta de 24 h);
  `ClassifyAudio` desde la PMT (con audio, sin audio, vacío).
- `db`: `MarkBatch` escribe `audio_ok` (y `Unknown` no pisa);
  `RegistrarDesenlace(url, …)` sobre filas compartidas por URL; éxito
  resetea; fallo real incrementa; fallo no real no incrementa; URL
  desconocida no falla; el ORDER BY de `FindMirrorsByChannelID` (audio,
  imagen, latencia) contra SQLite real.
- `handlers`: `POST /streams/desenlace` (204, 400 por cuerpo inválido y
  por tamaño, 204 en URL desconocida); `/channels/streams` con los cuatro
  campos; `/channels/imagen` (solo canales con datos; `sin_imagen` por
  canal); `/stats` con `sin_imagen` e `imagen_p50_ms`; el test de 15
  claves intacto. `MismoOrigen` ya tiene tests; se añade uno que
  demuestre que `POST /streams/desenlace` con `Sec-Fetch-Site: cross-site`
  es 403.

**Web** (gates: `npm run check && npm test && npm run build`, ≤ 80 KB gzip):

- `desenlaces.test.ts`: qué se reporta y qué no (motor forzado, pestaña
  oculta, offline; clases).
- `Reproductor.test.ts`: filtro `sinImagen`; mensaje nuevo con «Probar de
  todos modos» y su reintento ignorando `sinImagen` pero no `codecOk`; el
  caso «todo por códec» sigue sin botón; nota «Sin audio» con
  `audioOk=false`; desenlace lleva `url`.
- `SenalCanal.test.ts`: los tres estados y sus `aria-label`.
- `imagen.test.ts`: carga, refresco con debounce.
- i18n: paridad.

**Comprobación real (obligatoria, con evidencia):** en Chrome VISIBLE,
sintonizar AMC (720p) dos veces; tras la segunda, `sqlite3` muestra
`fallos_reales = 2` en `41.205.93.154`; la tercera vez el mensaje «Ningún
origen… hace N min» sale al instante con «Probar de todos modos», y la
tarjeta de AMC dice «Sin imagen desde aquí». Sintonizar un canal bueno y
ver «Imagen en N s» en su tarjeta y `/channels/imagen` con su entrada.

---

## 6. Riesgos

- **Un fallo de red local cuenta como fallo del origen.** Mitigado con
  `navigator.onLine` y con la ventana de 24 h; y «Probar de todos modos»
  siempre existe. Si el usuario tiene una tarde de wifi mala, mañana todo
  vuelve a intentarse solo.
- **Dos fallos consecutivos con dos causas distintas** (un timeout, un
  corte): cuentan igual. Es a propósito: ambos son «no dio imagen desde
  aquí».
- **Refrescar `/channels/imagen` en cada desenlace:** payload minúsculo
  (solo canales sintonizados) y con debounce.
- **`imagen_ms` es la ÚLTIMA medida, no una media:** puede oscilar. Es lo
  honesto («la última vez tardó 2,1 s»); una media queda para cuando haya
  historial (fuera de alcance).

---

## 7. Fuera de alcance (a propósito)

- **Sonda de velocidad en el servidor.** Descartada con datos (§1).
- **Historial/medias de tiempos**; solo la última medida.
- **Ordenar el catálogo por tiempo hasta la imagen.**
- **Cambiar el orden de la lista lateral.**
- **`mobile/`:** cero diffs. Contrato de 15 claves intacto.
- **Compartir desenlaces entre máquinas.** Todo es local.
