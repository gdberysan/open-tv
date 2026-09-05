# Diseño — Mirrors y metadatos desde la API de iptv-org

> **Fecha:** 2026-09-04 · **Estado:** pendiente de revisión del dueño
> **Origen:** la sesión del 2026-09-04, midiendo por qué fallan los canales.
> Números y evidencia en la memoria del proyecto (`fallos_de_reproduccion_medidos.md`).

---

## 1. Objetivo

Que un canal cuya única URL está muerta deje de ser un canal muerto.

Hoy el **96 % de los canales tiene un solo mirror**, así que la cadena de
failover —construida, probada y recién arreglada— casi nunca tiene a dónde ir.
La API de iptv-org ya publica los mirrors que el `index.m3u` descarta.
Aprovecharlos es un cambio de **fuente de datos**: sin infraestructura nueva, sin
credenciales, sin coste mensual y sin dependencias Go nuevas.

De paso, y porque sale de la misma API y del mismo sync, se arreglan dos cosas
más: las **categorías** de 2.606 canales que hoy dicen solo «General», y las
cabeceras (`referrer`, `user_agent`) que algunos orígenes exigen para servir el
stream.

**Fuera de alcance, decidido explícitamente:**

- **Validación a nivel de segmento** (que `is_alive` signifique «reproduce» y no
  «el manifiesto contestó»). Es el otro lever grande y merece su propio spec:
  duplica el coste del health-check y toca otro adaptador.
- **Salidas geográficas / proxies.** Medido: techo del ~3 % del catálogo y exige
  infraestructura nueva. Parcado a propósito.

---

## 2. Lo que se midió — todo verificado, nada supuesto

Contra el catálogo real (`.devdata/iptv.db`) y la API de iptv-org, el 2026-09-04.

**Estado de partida:**

| Hecho | Valor |
|---|---|
| Canales en el catálogo | 12.097 |
| Canales con **un solo** mirror | 11.843 (**96 %**) |
| Canales con 2 mirrors | 240 |
| Canales con 3 mirrors | 14 |
| Streams que responden 200 (sondeo directo de 9.375 URLs) | ~74 % |
| Streams inalcanzables / 404 / 403 | ~26 % |

**Lo que la API ofrece y hoy se tira** (`api/streams.json`, 17.207 entradas):

| Hecho | Valor |
|---|---|
| Canales nuestros con `tvg_id` | 10.414 |
| Sin correspondencia exacta en la API | **2** |
| **Canales que ganan al menos un mirror** | **2.112** |
| **Filas de stream extra a insertar** | **4.180** |
| Streams con `referrer` o `user_agent` propios | 1.046 |
| Canales con categoría «General» que `channels.json` resuelve | 2.606 |

**Resultado esperado: de 254 a 2.112 canales con alternativa real (8,3×).**

### 2.1 El emparejamiento es exacto: `channel` + `feed`

El sufijo de nuestro `tvg_id` **es** el `feed` de la API:

```
tvg_id "AndTV.in@HD"  →  channel "AndTV.in" + feed "HD"
```

`streams.json` trae `feed` en 15.259 de sus 17.207 entradas. Emparejando por el
par exacto, **solo 2 de nuestros 10.414 canales se quedan sin correspondencia**.
Cero fuzzy matching, cero adivinanza por nombre. Es esperable: nuestro catálogo
ES `iptv-org/index.m3u`, así que los identificadores salen de la misma fuente.

**Emparejar solo por `channel`, ignorando el `feed`, está MAL** y hay que decirlo
porque es la trampa obvia: da 3.061 canales y 14.734 filas, pero le asigna al
feed SD los streams del HD y viceversa. Los números buenos son los de arriba.

### 2.2 El país NO se puede arreglar por esta vía — medido y descartado

La idea original era rellenar los **1.683 canales sin `country_code`**. **No se
puede, y conviene dejarlo escrito para que nadie lo reintente:** los 1.683 son
**exactamente los mismos que no tienen `tvg_id`**. Sin `tvg_id` no hay clave con
la que unir contra `channels.json`, así que la API resuelve **0 de 1.683**.

Es coherente con el resto: son las entradas que upstream nunca ligó a un canal
conocido — las mismas que aparecen en `streams.json` sin campo `channel` (1.948).

Probado también el emparejamiento **por nombre** (normalizando y quitando el
sufijo `(1080p)`), contra `name` y `alt_names`: de 1.683 solo **81** casan de
forma única, **109** son ambiguos (varios países) y **1.493** no casan.
**Descartado:** 81 canales no justifican un emparejador difuso que además puede
asignar el país equivocado en los 109 ambiguos.

Lo que `channels.json` SÍ arregla es la **categoría**: 2.606 canales que hoy
caen en «General» tienen `tvg_id` y categorías reales upstream.

### 2.3 `streams.json` NO es un superconjunto

9.910 canales frente a nuestros 12.097, y **1.948 de sus streams no traen
`channel`**. Por eso el diseño es **aditivo**, no un reemplazo: el `index.m3u`
sigue mandando en la amplitud del catálogo y la API solo **enriquece**.

### 2.4 Cabeceras: 1.046 streams las necesitan y hoy no las tienen

`streams.json` trae `referrer` (331 entradas) y `user_agent` (877). Son
exactamente los orígenes con protección de hotlink o filtro de agente — parte de
los 403 que el sondeo del catálogo encontró y que **no** son geo.

Hoy el proxy manda un `User-Agent` fijo (`VLC/3.0.20 LibVLC/3.0.20`) y ningún
`Referer`. Y el navegador **no puede** poner esas cabeceras en una petición de
medio: `Referer` y `User-Agent` son cabeceras prohibidas para `fetch`/XHR. **El
único sitio del sistema que puede mandarlas es el proxy de loopback**, que ya
existe y ya construye una petición nueva desde cero.

---

## 3. Hallazgos del código actual

### 3.1 El proveedor guarda UNA url por canal, y la última gana

`internal/adapters/providers/opensource/provider.go`:

```go
streamURLs map[domain.ChannelID]string
```

Un `map` de `ChannelID` a **string**. Si el M3U trae varias entradas para el
mismo canal, la última pisa a las anteriores. `ProviderPort.GetStreamURL`
devuelve también un solo string. Ahí muere el 96 %.

### 3.2 Los streams NUNCA se podan — y eso ya es deuda

`DeleteStale` existe en `ChannelRepository`, **no en `StreamRepository`**. El
syncer poda canales, jamás streams. Cuando upstream cambia la URL de un canal, la
vieja se queda en la DB para siempre.

**Los 254 canales que hoy tienen 2+ mirrors son eso: restos.** Inspeccionados,
sus URLs son de orígenes distintos y sin relación — leftovers de sincronizaciones
antiguas, no mirrors curados. Hoy hacen poco daño porque el failover casi nunca
llega a usarlos; en cuanto los mirrors sean intencionados, cada resto muerto le
cuesta al usuario un intento entero (hasta 7 s) antes de cruzar al siguiente.

**Por eso la poda de streams entra en ESTE spec.** Sin ella la tabla crece sin
límite y el failover se llena de historia muerta.

### 3.3 Lo que NO hay que tocar

- **`/channels` está congelado en 15 claves** (`domain/channel_test.go` lo
  asevera para Flutter). Este diseño **no añade ni quita claves**: solo rellena
  un campo que ya existe (`CategoryID`).
- **`/channels/streams` ya devuelve mirrors** (`FindMirrorsByChannelID`,
  ordenados vivos-primero-por-latencia) y el cliente web ya los recorre con
  `planDeFailover`. **No hace falta endpoint nuevo ni cambio de contrato.**
- **`mobile/` cero diffs.**
- **Ninguna dependencia Go nueva:** son dos JSON; `encoding/json` basta.

---

## 4. Arquitectura

### 4.1 Alternativas consideradas

**A. Meter la API dentro del proveedor opensource.** Sencillo, pero acopla el
parser genérico de M3U a un proveedor concreto — y ese mismo parser sirve los M3U
que sube el usuario, que no tienen API.

**B. Un proveedor `iptvorg` aparte.** Limpio, pero duplica el parseo de M3U o
exige componerlo, y obliga a decidir el tipo de fuente en el syncer.

**C. Enriquecedor inyectado (recomendada).** El proveedor opensource sigue siendo
el parser genérico de M3U y recibe **opcionalmente** un colaborador:

```go
// Enriquecedor aporta lo que el M3U no trae. nil = comportamiento de siempre,
// que es justo lo que necesita un M3U subido por el usuario.
type Enriquecedor interface {
    // Streams devuelve los streams del par (channel, feed), con sus cabeceras.
    Streams(ctx context.Context, canal, feed string) []StreamExtra
    Metadatos(ctx context.Context, canal string) (Metadatos, bool)
}

type StreamExtra struct {
    URL       string
    Referrer  string // "" = ninguno
    UserAgent string // "" = el de siempre
}
```

Se inyecta como las opciones que ya existen (`WithAllowedFileDir`) y **solo** se
construye cuando la fuente es iptv-org. Un M3U del usuario pasa `nil` y se
comporta exactamente igual que hoy. Testeable con un doble, sin acoplar, y la
superficie nueva son dos métodos.

### 4.2 Cómo se decide que una fuente es iptv-org

Por **host de la URL ya parseada** (`iptv-org.github.io`), no por substring. Si
no casa, no hay enriquecimiento: comportamiento de hoy.

### 4.3 El puerto crece sin romper a nadie

```go
// GetStreamURL sigue igual: la url PRINCIPAL. Lo usa /channels/stream.
GetStreamURL(ctx, channelID) (string, error)
// GetStreamsDeCanal devuelve la principal PRIMERO y luego los mirrors,
// deduplicados, cada uno con sus cabeceras.
GetStreamsDeCanal(ctx, channelID) ([]StreamExtra, error)
```

---

## 5. Flujo de sync nuevo

```
1. provider.GetLiveChannels()               // M3U, igual que hoy
2. (si hay enriquecedor) cargar streams.json + channels.json UNA vez por sync
3. comprobación de catálogo sospechoso      // igual que hoy, sobre nº de canales
4. fusionar metadatos: rellenar SOLO campos vacíos del M3U
5. channels.SaveBatch()
6. por canal: GetStreamsDeCanal() -> N streams (principal + mirrors, dedup)
7. streams.SaveBatch()
8. streams.DeleteStale(fuente, inicio)      // NUEVO
9. channels.DeleteStale(fuente, inicio)     // igual que hoy
```

**Orden de los mirrors:** la URL del `index.m3u` va **primera** — es la curada y
hoy es la que funciona. Los extras de la API van detrás. A partir de ahí manda la
salud: `FindMirrorsByChannelID` ya ordena vivos primero y por latencia, así que el
orden de inserción solo decide el arranque en frío.

**Deduplicación** por URL normalizada dentro del canal: la API repite a menudo la
URL que ya venía en el M3U.

### 5.1 Poda de streams (nueva)

```go
// DeleteStale borra los streams de la fuente cuyo last_seen_at sea anterior a
// `before`. Mismo scoping por fuente que la poda de canales: nunca cruza fuentes.
DeleteStale(ctx, providerID string, before time.Time) (int64, error)
```

Exige una columna `last_seen_at` en `streams` (migración aditiva) que `SaveBatch`
actualiza. La poda de streams corre **antes** que la de canales, para que un canal
que se va no deje streams huérfanos.

**Primera pasada tras el despliegue: se limpian los ~254 restos históricos.** Es
el efecto deseado, pero conviene saberlo — el conteo de streams bajará antes de
subir.

---

## 6. Metadatos y cabeceras

### 6.1 Categorías: rellenar lo débil, nunca pisar lo bueno

`channels.json` (31.127 entradas) trae `country` y `categories`. **No trae
`languages`** — comprobado sobre el JSON real; los campos son `alt_names`,
`categories`, `closed`, `country`, `id`, `is_nsfw`, `launched`, `name`,
`network`, `owners`, `replaced_by`, `website`.

`country` **no se usa**, por lo medido en §2.2. Solo se toca la categoría:

> Se rellena la categoría **solo** cuando la del M3U está vacía o es «General»
> (el cajón de sastre de iptv-org). Cualquier otra categoría del M3U gana.

Objetivo medible: **2.606 canales pasan de «General» a categorías reales.** Los
3.147 «Undefined» no tienen `tvg_id` y quedan como están.

### 6.2 Cabeceras por stream

Dos columnas nuevas en `streams` (aditivas, por defecto vacías): `referrer` y
`user_agent`. Se persisten en el sync y las consumen **dos** sitios:

1. **El proxy de loopback**, al construir la petición al origen: si el stream trae
   `referrer`/`user_agent`, se usan; si no, el `User-Agent` fijo de siempre. El
   proxy ya construye una petición NUEVA (no reenvía las cabeceras del cliente),
   así que es un cambio contenido — y sigue sin propagar `Origin`, `Cookie` ni
   nada del navegador.
2. **El health-checker**, para no marcar muerto lo que solo necesitaba una
   cabecera.

**Lo que NO cambia:** la guarda SSRF (`controlConexion`, `checkRedirect`, tope de
tamaño, solo loopback). Poner una cabecera no relaja ninguna de esas defensas.

**Cómo llega el stream elegido al proxy:** el proxy recibe la URL en `?u=`. Para
saber qué cabeceras tocan, las busca en la DB por URL. Si no la encuentra —una URL
que no está en el catálogo—, usa las de siempre. **No** se aceptan cabeceras por
query string: sería dejar que la página dicte con qué identidad sale el proxy.

---

## 7. Coste del health-check

El validador corre con **50 workers y timeout de 8 s**
(`validator.DefaultConfig()`). El catálogo pasa de ~12.400 a ~16.600 streams:
**+34 % de trabajo por pasada.**

**Decisión: validar todo, sin trato especial.** Razones:

- Un mirror sin validar es inútil: `FindMirrorsByChannelID` ordena por salud, y un
  stream con `is_alive = 0` se va al final de la cola igualmente.
- Un esquema en dos fases (principales primero, extras después) no se paga con
  +34 %.
- El worker ya corre por ticker en segundo plano y `MarkBatch` existe justo para
  que las escrituras no congelen la API.

**A vigilar tras el despliegue:** la duración real de una pasada. Si se dispara, la
palanca barata es subir `MaxWorkers`, no partir el diseño.

---

## 8. Manejo de fallos

- **La API no responde o viene corrupta → el sync NO falla.** Se registra un aviso
  y se sincroniza solo con el M3U, exactamente como hoy. El enriquecimiento es una
  mejora, jamás un requisito: un fallo de `iptv-org.github.io` no puede dejar al
  usuario sin catálogo.
- **La comprobación de catálogo sospechoso** (`minRatioCatalogo`) sigue mirando el
  **número de canales**, que este cambio no altera. No se toca.
- **Tamaño:** `streams.json` son ~3,5 MB y `channels.json` ~7,8 MB (31.127
  entradas). Se descargan **una vez por sync**, no una vez por canal, y se
  indexan antes del bucle: `streams.json` por `(channel, feed)` y de
  `channels.json` se guarda **solo la categoría** de cada id, descartando el
  resto del objeto para no cargar 7,8 MB de campos que no se usan.
- **Memoria:** un mapa de ~17 k entradas y otro de ~31 k strings cortos.
  Irrelevante frente al catálogo que ya se maneja en memoria durante el sync.

---

## 9. Pruebas

TDD, como el resto del repo:

1. **Emparejamiento `(channel, feed)`**: `X@HD` casa con `channel X, feed HD` y
   **no** con `feed SD`. Un `tvg_id` sin `@` y uno desconocido no revientan nada.
2. **Orden**: la URL del M3U sale primera; los mirrors detrás.
3. **Deduplicación**: una URL presente en el M3U y en la API produce UN stream.
4. **Enriquecedor `nil`** (el M3U del usuario): comportamiento idéntico al de hoy.
   Es la prueba que protege a las fuentes bring-your-own.
5. **La API falla**: el sync termina bien y guarda el catálogo del M3U.
6. **Categorías**: se rellena cuando el M3U trae vacío o «General»; NO se pisa
   una categoría real del M3U.
7. **Poda de streams**: se borra el que no apareció en este sync; la poda no cruza
   fuentes.
8. **Cabeceras**: el proxy manda `referrer`/`user_agent` del stream cuando existen
   y el fijo cuando no; una URL desconocida no hereda cabeceras de otra.
9. **Contrato congelado**: las 15 claves de `/channels` siguen verdes y `mobile/`
   sigue con cero diffs.

Gates de siempre al final: `gofmt`, `go vet`, `go build`, `go test -race`,
`golangci-lint`, `scrubcheck`, y los del cliente web.

---

## 10. Riesgos abiertos

- **No sabemos cuántos de los 4.180 mirrors extra funcionan de verdad.** La primera
  pasada del health-check lo dirá. Aunque solo la mitad estén vivos, el beneficio
  se mantiene: hoy esos 2.112 canales tienen exactamente cero alternativas.
- **`is_alive` sigue significando «el manifiesto contestó», no «reproduce».** El
  ~30 % de streams que responden 200 pero no reproducen en el navegador seguirá
  ahí. Es justo lo que ataca el spec de validación por segmento, que va después.
- **Buscar cabeceras por URL en la DB** añade una consulta por petición al proxy.
  Con `MaxOpenConns(1)` conviene medirlo; si molesta, un mapa en memoria
  refrescado por el sync lo resuelve sin tocar el diseño.
- **La API y el M3U pueden ir desfasados.** Si la API va por detrás, algún mirror
  llegará ya muerto; la poda y el health-check lo corrigen en la pasada siguiente.

---

## 11. Qué NO cambia

`/channels` (15 claves), `/channels/streams` (ya devuelve mirrors), el cliente web
(`planDeFailover` ya recorre lo que le den), `mobile/`, las guardas del proxy, y el
comportamiento de cualquier fuente que no sea iptv-org.
