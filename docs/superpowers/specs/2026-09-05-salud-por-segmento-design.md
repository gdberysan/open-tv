# Diseño — Salud por segmento (veredicto de códecs)

> **Fecha:** 2026-09-05 · **Estado:** pendiente de revisión del dueño
> **Origen:** AMC (720p), reportado por el dueño el 2026-09-04 y medido el
> 2026-09-05. Es el item 2 de la hoja de ruta al estado del arte
> (0 arnés → 1 bucle de verdad → **2 salud por segmento** → 3 tiempo hasta
> la imagen → 4 EPG). El dueño decidió adelantarlo al item 0.

---

## 1. Objetivo

Que el catálogo sepa **qué códecs lleva de verdad cada mirror**, para que el
reproductor no gaste 30 segundos en un origen que ningún navegador puede
decodificar, y para que el mensaje al usuario diga la verdad.

### El caso medido

AMC (720p) tiene dos mirrors. Los dos están «vivos» para el health-check
(manifiesto 200) y los dos son inservibles en un navegador:

| Mirror | Vídeo | Audio | Velocidad |
|---|---|---|---|
| `23.239.31.26:8989` (el primero por latencia, 158 ms) | **MPEG-2** 720p60 | MP2 | bien: 4,6 MB en 1,2 s |
| `41.205.93.154` | H.264 High 4.2 1080p60 | **ninguno** | un segmento de 5 s tarda 19 s |

Lo que pasa hoy, medido con ffprobe y con una traza de hls.js en un Chrome
real con la pestaña visible:

- hls.js tira el tipo de stream MPEG-TS `0x02` (vídeo MPEG-2) en silencio
  (`tsdemuxer.ts`, «unknown stream type»). Crea UN solo SourceBuffer
  `audio/mpeg`, bufferiza 16 s de MP2 y Chrome no avanza ni un tick:
  `readyState` se queda en 1, sin `timeupdate`, sin error.
- Safari nativo decodifica el MP2 pero no tiene decodificador MPEG-2 sobre
  HLS: **audio sin imagen**, tal cual lo reportó el dueño.
- El guard agota su presupuesto, el failover salta al segundo mirror, cuyo
  primer segmento se come lo que queda, y el usuario lee «El canal no llegó
  a reproducir. Puede estar caído, geo-bloqueado o su dirección caducó. Se
  probaron 2 mirrors». Es verdad, pero no dice nada: el canal está vivo, no
  está geo-bloqueado y su dirección no caducó.
- El health-check no puede verlo: solo lee el manifiesto, el ACAO y el
  atributo `CODECS`. El manifiesto del mirror 1 **no lleva `CODECS`**, y la
  verdad vive en la PMT del segmento. Medido: en los dos mirrors la PAT y la
  PMT están en los paquetes 2 y 3, o sea en los **primeros 564 bytes**.

### Decisiones del dueño (2026-09-05)

1. **Saltarse el mirror y decir por qué.** El failover no lo intenta. Si
   TODOS los mirrors son indecodificables, el mensaje honesto sale al
   instante, sin gastar los 30 s. La lista lateral conserva «Señal viva»:
   el origen está arriba.
2. **Sondear solo cuando el veredicto está caducado**, no en cada pasada.
3. **El cliente también diagnostica** cuando el servidor todavía no sabe:
   si un intento falla y hls.js solo vio pistas de audio, se clasifica como
   problema de códec. **Nunca falla rápido**: un canal legítimo de solo audio
   AAC sigue reproduciendo como hoy.
4. **La sonda es una segunda etapa de la pasada de salud** (no un worker
   aparte), con el cliente guardado del proxy extraído y reutilizado.

---

## 2. Arquitectura

```
pasada de salud (cada 60 min, pool de 50, 4 conexiones por host)
  │
  ├─ etapa 1 (hoy): GET manifiesto → is_alive, latency, web_ok
  │
  └─ etapa 2 (NUEVA, solo si vivo + HLS + veredicto caducado):
       master → media playlist (si hace falta)
       media playlist → primer segmento .ts, Range 0-16383
       PAT → PMT → tipos de stream → CodecSupport + "mpeg2video,mp2"
       ↓
     streams.codec_ok / codecs / codec_checked_at
       ↓
     GET /channels/streams → codec_ok, codecs   (aditivo)
       ↓
     cliente: filtra mirrors codecOk===false antes del failover
              si no queda ninguno → mensaje honesto al instante
              si hls.js no vio vídeo y el intento falló → clase 'codec'
```

Dos piezas puras y aisladas: el **parser de MPEG-TS** y el **clasificador**
viven en `internal/domain`, sin red ni DB, y se prueban con bytes sintéticos.
La **sonda** (los saltos HTTP) vive en el validator. El **cliente** solo
consume el veredicto.

---

## 3. Servidor

### 3.1 Veredicto y parser (`internal/domain`)

- `CodecSupport` tri-estado: `CodecUnknown` (cero valor), `CodecNo`,
  `CodecOK`. Hermano de `WebSupport` con OTRO significado: `WebNo` dice «no
  directo, ve por el proxy»; `CodecNo` dice «ningún navegador lo decodifica,
  ni lo intentes».
- `mpegts.go`: parser de un PREFIJO de MPEG-TS. Recorre paquetes de 188
  bytes (sync `0x47`), localiza la PAT (PID 0), toma el PID de la PMT del
  primer programa distinto de 0, lee la PMT y devuelve la lista de
  `(stream_type, PID)`. Tolera campo de adaptación, pointer field y
  descriptores. Si la PMT no cabe en el prefijo o el sync falla, devuelve
  error (→ `CodecUnknown`, nunca `CodecNo` por no haber podido leer).
- `ClassifyCodecs(tipos)`: regla **centrada en el vídeo**.
  - `CodecOK` si algún stream de vídeo es H.264 (`0x1b`).
  - `CodecNo` si hay vídeo y ninguno es H.264: MPEG-2 (`0x02`), MPEG-4
    parte 2 (`0x10`), HEVC (`0x24`, coherente con `codecsWeb`, que ya lo
    excluye), VC-1 (`0xea`).
  - Sin stream de vídeo → `CodecUnknown`: aquí no se juzga el solo-audio.
- `NombreCodecs(tipos)`: cadena corta para stats y mensaje, p. ej.
  `mpeg2video,mp2`, `h264`, `h264,aac`. Tipos desconocidos salen como
  `0x80` en hexadecimal.

### 3.2 Sonda (`internal/adapters/validator`)

- **Disparo:** en `CheckConCabeceras`, DESPUÉS del GET del manifiesto,
  solo si `IsAlive`, protocolo HLS y la tarea dice `CodecCaducado`. El worker
  calcula la caducidad a partir de la fila y la mete en `TareaCheck`:
  caducado = `codec_checked_at == 0`, o más de **24 h**, o el mirror estaba
  muerto en la pasada anterior (`is_alive == 0` al leer la fila).
- **Saltos:** si el manifiesto es master (`#EXT-X-STREAM-INF`), se sigue la
  PRIMERA variante a su media playlist (URI resuelta contra la URL final del
  master). De la media playlist se toma la primera URI de segmento. Se salta
  la sonda (→ `CodecUnknown`, con `codec_checked_at` sellado igualmente) si:
  la playlist trae `#EXT-X-MAP` (fMP4), la URI no acaba en `.ts` y la
  respuesta no es `video/MP2T`, o no hay segmentos.
- **Segmento:** `GET` con `Range: bytes=0-16383`. Si el origen contesta 206,
  se lee ese cuerpo. Si contesta 200 ignorando el Range, se lee por
  `io.LimitReader` a 16 KB y se cierra la conexión (el transporte va sin
  keep-alive, así que cerrar aborta la transferencia). 16 KB = 87 paquetes,
  contra los 3 medidos.
- **Cabeceras y tiempos:** los dos saltos mandan el mismo User-Agent y
  Referer de la tarea, y comparten el `context.WithTimeout` del chequeo (8 s
  por defecto): la sonda nunca alarga el chequeo más allá del timeout que
  ya tenía.
- **Cliente guardado (SSRF):** la URL del segmento la dicta un manifiesto de
  terceros; es la misma superficie que el proxy ya guarda. Se extrae
  `proxy.NuevoClienteGuardado(permitirPrivados bool) *http.Client` del
  código que hoy construye `NewHandler` (control de conexión sobre la IP
  resuelta, `checkRedirect`, sin keep-alive, 4 conexiones por host).
  `NewHandler` pasa a usarlo, con comportamiento idéntico. El validator lo
  usa **solo para los saltos derivados del manifiesto**; el GET del
  manifiesto sigue con el cliente de siempre (la URL viene del catálogo, no
  de un tercero, y cambiar eso no es de este spec). En tests,
  `permitirPrivados=true` como ya hace el proxy con httptest.
- **Resultado:** `StreamResult` gana `Codec domain.CodecSupport`, `Codecs
  string` y `CodecSondeado bool` (la sonda corrió, sea cual sea el veredicto).

### 3.3 Persistencia

Tres columnas aditivas en `streams`, en `schema.sql` Y en `alterMigrations`:

```sql
codec_ok         INTEGER CHECK (codec_ok IN (0,1)),   -- NULL = sin sondear
codecs           TEXT    NOT NULL DEFAULT '',
codec_checked_at INTEGER NOT NULL DEFAULT 0
```

- `ports.StreamHealth` gana `Codec`, `Codecs`, `CodecSondeado`. `MarkBatch`
  aplica: si `CodecSondeado`, sella `codec_checked_at = now` y, si el
  veredicto NO es `CodecUnknown`, escribe `codec_ok` y `codecs`. Un
  `CodecUnknown` nunca pisa un veredicto anterior (misma regla que
  `WebUnknown`).
- `ports.MirrorHealth` gana `Codec` y `Codecs`. `FindMirrorsByChannelID`
  conserva su orden (vivos por latencia): el cliente decide qué salta.
- `FindAll` devuelve también `codec_checked_at` e `is_alive` (ya lo hace)
  para que el worker calcule la caducidad.

### 3.4 API (aditiva)

- `GET /channels/streams?id=` gana `codec_ok` (bool nullable, `null` = sin
  sondear) y `codecs` (string, `''` si no se sabe).
- `GET /stats` → `catalogo` gana `codec_no` (streams con `codec_ok = 0`).
- **`/channels` y `/sources` NO cambian.** El contrato de 15 claves de
  Flutter sigue aseverado por `domain/channel_test.go`.

### 3.5 Coste

Primera pasada tras actualizar: todos los mirrors HLS vivos (~11,5k) están
caducados, así que se sondean una vez: como mucho dos peticiones pequeñas
cada uno, dentro del pool y del tope por host que ya existen. Después, solo
los caducados: del orden de 23k peticiones pequeñas **al día**, no por hora.

---

## 4. Cliente

### 4.1 Datos

`Mirror` gana `codecOk: boolean | null` y `codecs: string`, mapeados en
`datos/http.ts` con la misma regla `?? null` que `webOk` («sin sondear» es
un tercer estado).

### 4.2 Saltar y decir por qué

- Antes de `planDeFailover`, el reproductor separa los mirrors con
  `codecOk === false`. Los demás siguen la cadena de siempre.
- Si había mirrors y NO queda ninguno: `cargando = false`, `mensajeError =
  t('reproductor.error.codec', { codecs })`, y se reporta a stats un
  desenlace `fallo` con `motivo: 'codec'` y `mirrorIndex: 0`. Cero segundos
  gastados. **Sin botón «Reintentar»** en este estado: reintentar no cambia
  los códecs, y un botón que no puede arreglar nada es una promesa falsa
  (la misma lección que el contador de mirrors).
- Texto (es, con paridad en en):
  «El vídeo de este canal viene en {codecs}, un formato que ningún navegador
  decodifica. Solo lo puede ver un reproductor de escritorio (VLC o la app
  instalada).» Sin consejo de Safari: Safari tampoco lo ve.
- La lista lateral sigue diciendo «Señal viva»: el origen está arriba.
  Etiquetar el códec en la tarjeta queda fuera (§7).

### 4.3 Diagnóstico en el cliente

- En el camino hls.js, el intento escucha `Hls.Events.BUFFER_CODECS` y
  anota `pistas = { video: !!(data.video ?? data.audiovideo), audio:
  !!data.audio }`.
- Al fallar el intento (por la vía que sea: timeout del guard o error
  fatal), `infoUltimoError` lleva además `sinVideo: pistas.audio &&
  !pistas.video`.
- `clasificarFallo` gana una regla, ANTES de la de `formato`: `sinVideo` →
  `'codec'`. Nueva `ClaseFallo` `'codec'`. Aquí no hay nombre de códec, así
  que se usa una SEGUNDA clave, `reproductor.error.codecGenerico`: «El vídeo
  de este canal viene en un formato que ningún navegador decodifica. Solo lo
  puede ver un reproductor de escritorio (VLC o la app instalada).» La de
  §4.2, `reproductor.error.codec`, es la que lleva `{codecs}`.
- `claseConsensuada` no cambia. `marcarFalloFormato` (cast) no cambia: la
  clase `'formato'` sigue significando «error de decodificación del
  navegador», y su mensaje queda como está.
- **No falla rápido**: un canal de solo audio AAC llega a `timeupdate`, el
  guard lo confirma y nunca entra aquí.

---

## 5. Pruebas

**Go** (gates: `gofmt`, `go vet`, `go build`, `go test -race`,
`golangci-lint run`, `scrubcheck`):

- `domain/mpegts_test.go`: paquetes PAT/PMT SINTÉTICOS construidos en el
  test (no se copian bytes de orígenes de terceros). Casos: MPEG-2 + MP2 →
  `CodecNo`, `mpeg2video,mp2`; H.264 + AAC → `CodecOK`; solo audio →
  `CodecUnknown`; H.264 + AC-3 → `CodecOK` (regla centrada en vídeo); PMT
  fuera del prefijo → error; sync roto → error; paquete con campo de
  adaptación y pointer field distinto de 0.
- `validator/checker_test.go` con `httptest`: master → media → segmento con
  206; origen que ignora el Range y responde 200 con 4 MB (se lee solo 16
  KB y termina en tiempo); media playlist directa sin master; `EXT-X-MAP`
  → `CodecUnknown` sondeado; segmento apuntando a IP privada → bloqueado
  por el cliente guardado y `CodecUnknown`; cabeceras Referer/User-Agent
  presentes en los dos saltos; tarea no caducada → cero peticiones extra.
- `validator/worker_test.go`: caducidad (sin sondear / >24 h / vuelto de
  muerto → sondea; reciente y vivo → no).
- `proxy/handler_test.go`: el proxy sigue verde tras la extracción (sin
  cambios de comportamiento).
- `db/stream_repository_test.go`: las tres columnas, `MarkBatch` con
  `CodecUnknown` no pisa, `codec_checked_at` se sella igual.
- `handlers/channel_handler_test.go` y `stats_handler_test.go`: campos
  aditivos. `domain/channel_test.go`: 15 claves, sin tocar.

**Web** (gates: `npm run check && npm test && npm run build`, bundle propio
≤ 80 KB gzip):

- `datos/http.test.ts`: mapeo `codec_ok`/`codecs` con el tercer estado.
- `Reproductor.test.ts`: (a) mirrors `[codecOk=false, codecOk=null]` →
  solo se intenta el segundo; (b) todos `codecOk=false` → mensaje al
  instante, sin instancia de hls.js, desenlace `motivo:'codec'`, sin botón
  Reintentar; (c) el doble de hls.js emite `BUFFER_CODECS` solo con audio y
  luego el guard falla → clase `codec`; (d) `BUFFER_CODECS` con vídeo y
  fallo → NO es `codec`.
- `diagnostico.test.ts`: `sinVideo` → `'codec'`, y su prioridad frente a
  `formato`/`inestable`.
- `i18n`: paridad es/en (la prueba de paridad existente).

**Comprobación real (no automatizable aquí):** construir el binario,
dejar que launchd lo relance, esperar la pasada de salud, comprobar en
SQLite que el mirror `23.239.31.26:8989/amc/index.m3u8` queda con
`codec_ok = 0` y `codecs = 'mpeg2video,mp2'`, y sintonizar AMC (720p) en un
Chrome real con la pestaña VISIBLE: el mensaje honesto tiene que salir al
instante. Se documenta con la evidencia (SQL + captura), no con «debería».

---

## 6. Riesgos

- **Orígenes que ignoran `Range`** y empiezan a mandar el segmento entero:
  se lee 16 KB y se cierra; sin keep-alive el cierre aborta la descarga.
  Coste real: 16 KB por mirror, no 4 MB.
- **PAT/PMT más allá de 16 KB:** raro (lo normal es paquete 1-3), y el
  fallo es benigno: `CodecUnknown`, el cliente sigue como hoy.
- **Programas múltiples en una PMT** (multiplex): se toma el primer
  programa; es lo que hls.js hace también.
- **Un veredicto viejo en un mirror que cambió de códec:** 24 h de
  caducidad acotan el daño; y el diagnóstico del cliente (§4.3) cubre el
  hueco en la dirección peligrosa (servidor dice OK, cliente no ve vídeo).
  En la otra dirección (servidor dice No, el origen ya arregló el códec) se
  espera a la siguiente pasada: es el precio de no sondear cada hora.
- **Timeout compartido:** la sonda vive dentro del timeout del chequeo
  (8 s). Un origen lento puede dejarla sin tiempo → `CodecUnknown` con
  `codec_checked_at` sellado; se reintenta a las 24 h. Aceptable: la
  alternativa (alargar el chequeo) alarga toda la pasada.

---

## 7. Fuera de alcance (a propósito)

- **fMP4 / `EXT-X-MAP`:** parsear `moov`/`stsd` del init segment. El
  `CODECS` del master ya cubre casi todo ese mundo.
- **Juzgar el solo-audio.** Ni en el servidor ni en el cliente.
- **Audio no decodificable con vídeo bueno** (AC-3, MP2 con H.264): el
  manifiesto ya lo cubre cuando declara `CODECS`; por PMT queda para
  después, con medición previa de cuántos casos hay.
- **Mirror lento** (AMC mirror 2, 5 s de vídeo en 19 s): es el item 3,
  «tiempo hasta la imagen».
- **Etiqueta de códec en la tarjeta/lista lateral.**
- **`mobile/`:** cero diffs. Contrato de 15 claves intacto.
- **Fixture `/hostil/mpeg2` en el e2e:** pertenece al plan del arnés
  (item 0, `2026-09-05-verificacion-honesta-de-reproduccion.md`), que se
  ejecuta después; ese plan gana una nota para añadirlo.
- **Cambiar el cliente HTTP del GET del manifiesto** al guardado: deuda
  conocida y separada.
