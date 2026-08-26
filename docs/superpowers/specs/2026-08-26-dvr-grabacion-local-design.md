# DVR — grabación local (record-now)

**Estado:** diseño aprobado en brainstorming (2026-08-26). Autoridad para el plan.
**Alcance v1:** grabar-ahora (● Grabar / Parar) + vista de Grabaciones (reproducir/borrar). Timeshift (pausar el directo) y grabación programada por EPG quedan como follow-ons sobre la MISMA base.

## 1. Contexto y objetivo

Poder pulsar ● Grabar en un canal en directo, capturarlo a disco LOCAL, y luego
reproducir la grabación en el mismo reproductor. Sin transcodificar, sin
dependencias nuevas, sin nube: encaja en la espina del producto (el proxy ya
descarga/relaya el HLS) y en su postura (todo en tu máquina, un solo binario).

**Nota legal (para el flip público):** esto añade una capacidad de *grabar*. El
time-shifting personal de televisión abierta (FTA) a tu propio disco es la línea
bien asentada del PVR/Betamax, y graba solo a la máquina del usuario — pero como
el marco legal se acaba de revisar con abogado, **dale un aviso de una línea
(«la herramienta ya puede grabar a disco local») antes de hacer público el
repo**. El plan/implementación se construyen igual; el flip sigue bloqueado.

## 2. Arquitectura — grabar por tee de segmentos HLS (sin ffmpeg)

El proxy ya relaya manifiestos (`relayarManifiesto`) y segmentos
(`relayarBytes`). La grabación reutiliza ese mismo camino: **la grabación NO
transcodifica; es HLS VOD sobre disco.**

```
● Grabar (canal X)
      │  POST /recordings {channelID}
      ▼
  Recorder (goroutine en el gateway, backend-owned)
      │  GetStreamURL(X) → manifiesto HLS en vivo
      │  poll del manifiesto (ventana deslizante) →
      │  por cada segmento NUEVO: descarga (cliente HTTP GUARDADO del proxy)
      │  → grabaciones/<id>/seg-NNNNN.ts + append a grabaciones/<id>/playlist.m3u8
      ▼  Parar (o fin del stream): finaliza el playlist con #EXT-X-ENDLIST
  Grabación = HLS VOD local
      │  GET /grabaciones/<id>/playlist.m3u8 (solo loopback)
      ▼
  hls.js la reproduce en el MISMO reproductor (cero transcode)
```

- **Backend-owned:** grabar DEBE sobrevivir a cerrar la pestaña o cambiar de
  canal, así que corre en el gateway (una goroutine por grabación activa,
  rastreada — a diferencia de los sync fire-and-forget, estas son de larga vida
  y las controla el usuario; se cancelan con context al Parar o al apagar).
- **Reproducción in-app:** como la grabación ES un `.m3u8` VOD, se sirve desde el
  gateway y la reproduce el reproductor existente sin cambios de motor.

## 3. Componentes

### 3.1 Recorder (`internal/services/recorder.go`)
- `Recorder.Iniciar(ctx, channelID) (idGrabacion string, error)` — resuelve la URL
  vía `ProviderPort.GetStreamURL` (o el mirror más sano de `FindMirrorsByChannelID`),
  arranca la goroutine de captura, registra metadatos «grabando».
- Bucle de captura: pollea el manifiesto en vivo cada ~target-duration; para cada
  URI de segmento no vista, la descarga con el **cliente HTTP guardado** (mismo
  `controlConexion`/timeout/límite de tamaño que el proxy — NO abrir una vía de
  red sin guarda SSRF) y la escribe en `grabaciones/<id>/seg-NNNNN.ts`; añade la
  entrada `#EXTINF` al `playlist.m3u8` local (rutas relativas a los .ts locales).
- `Recorder.Parar(idGrabacion)` — cancela el context; finaliza el playlist con
  `#EXT-X-ENDLIST`; sella metadatos «completa» (duración, bytes).
- Robustez: un segmento que falla se reintenta un par de veces y si no, se
  registra y la grabación SIGUE (hueco), no se cae. Si el stream termina
  (ENDLIST del origen) o el manifiesto deja de dar segmentos por N ciclos, se
  finaliza sola como «completa».

### 3.2 Almacenamiento
- Ficheros en `<datadir>/grabaciones/<id>/` (junto a `fuentes/`, permisos 0700).
- Tabla `grabaciones`:
  ```sql
  CREATE TABLE IF NOT EXISTS grabaciones (
      id           TEXT PRIMARY KEY,
      channel_id   TEXT NOT NULL,
      channel_name TEXT NOT NULL,   -- congelado al iniciar (el canal puede desaparecer del catálogo)
      started_at   INTEGER NOT NULL,
      ended_at     INTEGER,          -- NULL mientras graba
      duration_s   INTEGER NOT NULL DEFAULT 0,
      bytes        INTEGER NOT NULL DEFAULT 0,
      estado       TEXT NOT NULL CHECK (estado IN ('grabando','completa','error')),
      ruta         TEXT NOT NULL     -- ruta relativa al playlist dentro de grabaciones/
  );
  ```
  Aditiva; no toca `channels`/`providers`/contrato Flutter.
- **Guarda de disco (obligatoria):** un tope por grabación (`GRABACION_MAX_MB`,
  default p. ej. 4096) y/o de duración; al alcanzarlo, se para sola y se marca
  «completa» con un aviso. Impide que una grabación olvidada llene el disco.

### 3.3 Endpoints (nuevos, aditivos, seguros para Flutter)
- `POST /recordings` `{ "channel_id": "…" }` → `{ "id": "…", "estado": "grabando" }`.
- `POST /recordings/{id}/stop` → `{ "estado": "completa", "duration_s":…, "bytes":… }`.
- `GET /recordings` → lista (snake_case: id, channel_id, channel_name, started_at, ended_at, duration_s, bytes, estado).
- `DELETE /recordings/{id}` → borra fila + carpeta `grabaciones/<id>/` (con guarda de contención Abs+Clean+Rel bajo el dir de grabaciones, mismo patrón que la subida de M3U de P0.7).
- `GET /grabaciones/{id}/playlist.m3u8` y `GET /grabaciones/{id}/{segmento}.ts` — sirve la grabación como VOD, **solo loopback** (mismo middleware `MismoOrigen`/mismo modelo que el proxy: una web de terceros NO puede leer tus grabaciones), Content-Type correcto, guarda de path-traversal.
- Todas las de mutación (`POST`/`DELETE`) bajo la guarda CSRF `MismoOrigen` (Host + Sec-Fetch-Site), igual que `/sources`.

### 3.4 Cliente (Svelte)
- **Control ● Grabar** en el overlay del reproductor: inicia/para; refleja el
  estado (grabando = ● rojo pulsante bajo reduced-motion respetado; aria-pressed).
  Al cambiar de canal, la grabación del canal anterior SIGUE (backend-owned); el
  control refleja si el canal ACTUAL se está grabando.
- **Vista «Grabaciones»** (`#grabaciones`, mismo patrón hash-view que
  `#ajustes`/`#fuentes`/`#stats`): lista con nombre de canal, fecha, duración,
  tamaño, estado; reproducir (abre el reproductor sobre la VOD local) y borrar
  (con confirmación). Estado vacío honesto («aún no has grabado nada»).
- Punto de acceso discreto en la cabecera (junto a «Fuentes»/«Ajustes»).
- Store `grabaciones` (patrón de los otros stores) que sondea el estado mientras
  hay una grabación activa (como el onboarding de P0.7 sondea el sync).
- i18n ES/EN paridad; a11y (sin regiones aria-live nuevas; reproducir/borrar con
  nombres accesibles; reduced-motion anula el pulso del ●).

## 4. Límites honestos (declarados en la UI/spec)
- **Streams cifrados con clave rotatoria/expirante (AES-128 con token):** se
  guardan los segmentos y, si la clave es estática y accesible, la clave; si la
  clave rota o expira, la reproducción posterior puede fallar. Se documenta;
  para FTA la mayoría son claros. No se intenta descifrar ni burlar DRM (postura
  del producto: sin elusión).
- **La grabación es tan buena como el stream:** si el origen cae a mitad, la
  grabación queda con lo capturado hasta ahí (VOD válido hasta el corte).
- **Sin transcode:** el contenedor es HLS/TS; se reproduce en el propio Open TV
  y en cualquier reproductor que lea HLS (VLC). No se produce un MP4 único (eso
  sería ffmpeg, follow-on opcional).

## 5. Restricciones (Global Constraints del plan)
- **Un solo binario, SIN ffmpeg, sin dependencias Go/JS nuevas** (net/http, os,
  bufio, encoding — stdlib; reutiliza el cliente guardado del proxy).
- **`mobile/` CERO diffs;** contrato JSON `/channels` (15 claves) + Flutter
  intacto (tabla y endpoints NUEVOS).
- **SSRF/seguridad:** el recorder descarga por el MISMO cliente guardado que el
  proxy (`controlConexion`, timeout, tope de tamaño por segmento). Las
  grabaciones se sirven SOLO en loopback (MismoOrigen). Mutaciones con la guarda
  CSRF. Borrado con guarda de contención de path.
- **a11y P0.6/P0.8:** sin regiones aria-live nuevas; roving/inert/focus-trap
  intactos; `prefers-reduced-motion` anula el pulso del ● Grabar.
- **i18n ES (fuente) + EN paridad.** Identidad `gdberysan@gmail.com` + trailer.
- **Gates:** `gofmt`/`vet`/`build`/`test -race`/**`golangci-lint`**/`scrubcheck`;
  web `check`/`test`/`build` (bundle ≤ 80 KB gzip); Flutter `analyze`/`test`.

## 6. Pruebas y gates
- **Go (recorder):** un servidor de fixtures que sirve un manifiesto HLS EN VIVO
  que crece (ventana deslizante de segmentos) → el recorder tee-a los segmentos
  a disco, construye un `playlist.m3u8` VÁLIDO, y al Parar lo finaliza con
  `#EXT-X-ENDLIST`; el tope de disco/duración lo para solo; un segmento 404 no
  tumba la grabación (hueco + sigue); el borrado limpia fichero+carpeta con la
  guarda de contención. Aislamiento SSRF: una URL no permitida se rechaza.
- **Endpoints:** start/stop/list/delete; el playlist servido es solo-loopback
  (una petición cross-site → 403); path-traversal en el segmento → rechazado.
- **Web:** el control ● Grabar refleja estado; la vista Grabaciones lista/
  reproduce/borra; estado vacío honesto; paridad i18n.
- **e2e Playwright:** grabar un canal de fixture unos segundos → aparece una
  grabación → se reproduce (VOD local) → se borra. Contra el binario real.
- **`mobile/` cero diffs** verificado en cada tarea.

## 7. No-objetivos (follow-ons sobre esta misma base)
- **Timeshift / pausar el directo** (buffer rodante + seek en vivo).
- **Grabación programada por EPG** («grabar este programa» desde la guía) — se
  apoya en el EPG de P2 y en un scheduler.
- **Transcodificar a MP4 único** (requeriría ffmpeg).
- **Compartir/subir grabaciones a la nube** (rompe la postura local/privada).

## 8. Riesgos
| Riesgo | Mitigación |
|---|---|
| Grabación olvidada llena el disco | Tope por grabación (MB/duración) que la para y avisa; guarda de disco obligatoria. |
| Clave HLS rota/expira → grabación irreproducible | Documentado como límite honesto; sin elusión de DRM (postura del producto). |
| El recorder abre una vía de red sin guarda | Reutiliza el cliente guardado del proxy (controlConexion + timeout + tope). No hay fetch nuevo sin guarda. |
| Una web de terceros lee tus grabaciones | Servidas solo en loopback (MismoOrigen), como el proxy; mutaciones con CSRF. |
| Goroutine de grabación colgada al apagar | context cancelado en el shutdown; la grabación se finaliza (completa/error) en vez de perderse. |
