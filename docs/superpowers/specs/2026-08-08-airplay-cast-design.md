# Diseño — Emisión por AirPlay a Apple TV y televisores AirPlay 2

> **Fecha:** 2026-08-08 · **Estado:** aprobado, pendiente de plan de implementación
> **Fase:** adelanto parcial de la Fase 9 (targets externos), sobre macOS

---

## 1. Objetivo

Enviar cualquier canal del catálogo a un Apple TV o a un televisor con AirPlay 2
integrado (Samsung, LG, Sony, Vizio de 2019 en adelante), manteniendo la app
como mando a distancia: la rejilla sigue navegable mientras el televisor
reproduce, y cambiar de canal no obliga a volver a elegir el dispositivo.

**Fuera de alcance, decidido explícitamente:** Google Cast, DLNA/UPnP y Roku.
Solo AirPlay.

---

## 2. Restricciones descubiertas en el código y en los datos

Estas cinco restricciones son las que dan forma al diseño. Ninguna es una
suposición: todas salen de leer el código o de medir el catálogo.

### 2.1 libmpv no tiene ruta AirPlay

`media_kit`/libmpv decodifica en la app y pinta en una textura de Flutter. No
existe ningún punto donde entregar ese vídeo a AirPlay. La única vía sancionada
por Apple es `AVPlayer` con `allowsExternalPlayback = true`, seleccionando ruta
con `AVRoutePickerView`. Implica una segunda ruta de reproducción, nativa.

### 2.2 El catálogo es HLS, y eso juega a favor

Medido sobre `gateway/iptv.db`, 12 655 streams:

| Contenedor | Streams | % |
|---|---|---|
| HLS (`.m3u8`) | 12 229 | 96,6 % |
| DASH (`.mpd`) | 139 | 1,1 % |
| Otros | 282 | 2,2 % |
| TS directo | 5 | 0,04 % |

| Esquema | Streams | % |
|---|---|---|
| `https` | 10 242 | 80,9 % |
| `http` | 2 408 | 19,0 % |
| `rtmp` / `mmsh` | 5 | 0,04 % |

HLS es el formato nativo de AirPlay. Por eso este diseño no necesita relay ni
proxy en el gateway: el receptor descarga el stream directamente del origen, y
al no ser un navegador no impone CORS ni bloquea contenido mixto.

El 19 % de URLs `http` sí es un problema, pero de ATS, y se resuelve con una
clave en el `Info.plist` (§6.3), no con infraestructura.

### 2.3 AVFoundation es mucho más estricto que mpv

mpv reproduce manifiestos malformados y códecs que AVFoundation rechaza de
plano. Habrá canales que se ven en el Mac y fallan al enviarse al televisor.
Esto no se puede eliminar; se gestiona (§5, §7).

### 2.4 El validador no lee el cuerpo del manifiesto

`checker.go:76` hace HEAD primero y solo cae a GET cuando el HEAD falla. En ese
fallback ya lee 64 KB y los descarta (`checker.go:122`). Sondear códecs en todos
los streams exigiría un GET por stream, contra un checker que limita
`maxConnsPerHost = 4` deliberadamente «para evitar ahogar servidores». El sondeo
se diseña alrededor de esa restricción (§4.2).

### 2.5 CI compila en Ubuntu

`.github/workflows/ci.yml` corre los dos jobs en `ubuntu-latest`. El código
Swift **nunca** pasa por CI. Todos los tests Dart deben correr en Linux sin
canal de plataforma. Esta restricción es la que justifica la costura de §3.2.

---

## 3. Arquitectura

### 3.1 Capa nativa (Swift) — `mobile/macos/Runner/AirPlay/`

| Fichero | Responsabilidad |
|---|---|
| `AirPlayPlugin.swift` | Registro de canales y de la factoría de vistas. Solo cableado. |
| `AirPlaySession.swift` | Posee el `AVPlayer`. KVO sobre `isExternalPlaybackActive`, `timeControlStatus` y `currentItem.status`. |
| `RoutePickerFactory.swift` | `NSViewFactory` que devuelve un `AVRoutePickerView` teñido con la paleta Korven. |
| `RouteName.swift` | Nombre del dispositivo, best-effort, vía el dispositivo de salida por defecto de CoreAudio. |

El contrato nativo completo son dos canales:

- **MethodChannel** `dev.korven.opentv/airplay`
  - `start(url: String, title: String)`
  - `stop()`
- **EventChannel** `dev.korven.opentv/airplay/events`
  - `{"type": "route", "active": bool, "name": String?}`
  - `{"type": "status", "state": "loading"|"playing"|"failed", "error": String?, "codigo": String?}`

`AVRoutePickerView` exige macOS 10.15, que es exactamente el
`MACOSX_DEPLOYMENT_TARGET` actual. No hay que subirlo.

**Excluido a propósito:** cabeceras HTTP personalizadas en el asset.
`AVURLAssetHTTPHeaderFieldsKey` no está documentada, la ruta de reproducción
actual no fija cabeceras, y un receptor AirPlay no las arrastraría de todos
modos.

### 3.2 Capa Dart

| Fichero | Responsabilidad |
|---|---|
| `data/airplay/airplay_platform.dart` | Interfaz + implementación real. **El único** fichero que toca `MethodChannel`. |
| `domain/models/cast_session.dart` | Modelo puro + `enum CastState { idle, armed, connecting, casting, failed }`. Sin imports. |
| `presentation/providers/cast_provider.dart` | `CastNotifier extends StateNotifier<CastSession>`, a nivel de app. |
| `presentation/player/airplay_guard.dart` | Timeout de carga y clasificación de fatalidad para la ruta `AVPlayer`. |
| `presentation/widgets/cast_bar.dart` | Barra persistente de sesión. |
| `presentation/widgets/airplay_button.dart` | `AppKitView` con el selector de ruta. |

`airplay_platform.dart` es la costura que hace testeable toda la máquina de
estados en CI, en Linux, sin ningún Apple TV delante (§2.5). Por el mismo
motivo, `codecs.go` en el gateway será una función pura sobre una cadena, sin
E/S.

`AppKitView` está disponible en Flutter 3.44 (verificado en
`packages/flutter/lib/src/widgets/platform_view.dart:349`).

### 3.3 Capa gateway (Go)

| Fichero | Responsabilidad |
|---|---|
| `internal/adapters/validator/codecs.go` | Clasificador puro de manifiestos. Sin E/S. |
| `internal/adapters/validator/checker.go` | Aprovecha el GET de fallback para clasificar gratis. |
| `internal/adapters/db/` | Columna `airplay_ok`, migración, lectura y escritura del veredicto. |
| `internal/api/handlers/channel_handler.go` | Expone el veredicto. |

---

## 4. El sondeo de compatibilidad

### 4.1 Clasificador

```go
func ClassifyManifest(url, body string) AirplaySupport // Unknown | No | OK
```

Lista de permitidos, no de prohibidos. Una variante es reproducible si **todos**
sus códecs casan con un prefijo soportado: `avc1.`, `avc3.`, `hvc1.`, `hev1.`,
`dvh1.`, `dvhe.`, `mp4a.40.`, `ac-3`, `ec-3`, `alac`.

| Entrada | Veredicto |
|---|---|
| URL `.mpd`, `rtmp://`, `mmsh://` | `No` |
| Master con `CODECS`, alguna variante soportada | `OK` |
| Master con `CODECS`, ninguna variante soportada | `No` |
| `#EXT-X-KEY:METHOD=SAMPLE-AES` con keyformat no Apple | `No` |
| Master sin `CODECS` | `Unknown` |
| Playlist de medios directa | `Unknown` |
| Cuerpo vacío o ilegible | `Unknown` |

### 4.2 Cuándo se ejecuta

Medido sobre una muestra de 40 streams vivos, para saber si el sondeo sirve de
algo antes de construirlo:

| Resultado | Streams | % |
|---|---|---|
| Master con `CODECS` → clasificable | 23 | 57,5 % |
| Master sin `CODECS` | 6 | 15 % |
| Playlist de medios directa | 6 | 15 % |
| No descargable en 6 s | 5 | 12,5 % |

Los códecs observados fueron todos `avc1.4d4028,mp4a.40.2` — H.264 Main@4.0 con
AAC-LC, justo lo que AVFoundation quiere. Conclusión: el sondeo merece la pena,
pero **~42 % del catálogo quedará en `Unknown`**, y la UI no puede tratar
`Unknown` como sospechoso.

Dos fuentes, ninguna de ellas de fondo:

1. **Gratis.** El fallback GET de `checker.go:122` ya lee 64 KB y los tira.
   Se clasifican en vez de descartarse. Cero peticiones añadidas.
2. **Bajo demanda.** En `GET /channels/stream?id=X`, si ese stream sigue en
   `NULL`, el gateway hace un GET del manifiesto con presupuesto de ~1 s, lo
   clasifica, lo persiste y devuelve el veredicto en la misma respuesta. Se paga
   una vez por canal, y solo por canales que de verdad se emiten.

**Se descartó una tercera fuente**, un relleno en segundo plano sobre
`airplay_ok IS NULL`. Habría rellenado el catálogo entero en unos días, pero era
la única pieza que añadía tráfico de fondo contra orígenes que el código ya
protege a conciencia (`maxConnsPerHost = 4`, §2.4), y solo compraba que la
insignia estuviese visible **antes** de emitir un canal por primera vez. Se
puede añadir después sin tocar nada de lo demás: el clasificador es puro y la
columna ya existe.

**Consecuencia para la interfaz.** Sin relleno, `airplay_ok` llega en `null`
para casi todo el catálogo en `GET /channels`. La insignia de la rejilla, por
tanto, se alimenta en la práctica de la memoria local de fallos (§7) y de los
canales ya emitidos alguna vez, no del sondeo. El sondeo sigue siendo lo que
protege el momento de emitir —que es donde importa— pero **la rejilla no debe
construirse esperando datos del gateway que en su mayoría no llegarán.**

### 4.3 Dónde vive el veredicto — en memoria, no en SQLite

Los handlers reciben un pool **de solo lectura** (`db.OpenReadOnly`,
`main.go:62`, cableado en `main.go:129`), separado a propósito del pool de
escritura. El sondeo bajo demanda vive en un handler, así que no puede persistir
sin romper esa separación.

Con el relleno de fondo descartado, persistir tampoco valdría gran cosa: la
columna solo la escribiría la fuente 1, que cubre únicamente los streams cuyo
HEAD falla. Por eso **no hay columna `airplay_ok`, ni campo en `domain.Channel`,
ni migración**. El veredicto vive en:

- Una **caché en memoria del handler**: `map[domain.ChannelID]AirplaySupport`
  bajo `sync.RWMutex`, con TTL de 12 h y tope de entradas. Se pierde al
  reiniciar el gateway, lo que cuesta un GET de ~1 s la primera vez que se
  vuelve a emitir ese canal. Aceptable.
- La **memoria local de la app** para los fallos de formato en tiempo real
  (§7), que es la que de verdad alimenta la insignia de la rejilla.

Contrato afectado, mínimo y aditivo:

- `GET /channels/stream?id=` pasa a devolver
  `{"url": ..., "airplay_ok": true|false|null}`.
- `GET /channels` **no cambia**. `domain/channel_test.go` sigue en 14 claves y
  no se toca. `mobile/lib/domain/models/channel.dart` tampoco.

Si algún día se quiere el catálogo entero clasificado, se añade la columna y el
relleno de fondo sin tocar nada de esto: el clasificador es puro y el contrato
de `/channels/stream` ya transporta el veredicto.

---

## 5. Máquina de estados y traspaso

Invariante que gobierna todo: **un solo reproductor tiene el stream en cada
momento.** `media_kit` y `AVPlayer` nunca están abiertos a la vez.

| Desde | Evento | Hasta | Acción |
|---|---|---|---|
| `idle` | Ruta elegida, sin reproducción local en curso | `armed` | Aparece la barra con el dispositivo, sin medio |
| `idle` | Ruta elegida estando en `PlayerScreen` | `connecting` | `media_kit.stop()` → `start()` |
| `armed` | Toque en un canal | `connecting` | Resolver URL → `start()`. **No** abre `PlayerScreen` |
| `connecting` | `status: playing` | `casting` | La barra muestra el nombre del canal |
| `connecting` | 15 s sin reproducir, o `item.status == .failed` | `failed` | → traspaso a local |
| `casting` | Toque en otro canal | `connecting` | `start(nuevaUrl)`. **Sin** volver a elegir dispositivo |
| `casting` | ⏹ en la barra | `idle` | `stop()`, liberar `AVPlayer` |
| `casting` | `isExternalPlaybackActive → false` | `idle` | Sesión caída (televisor apagado) |
| `failed` | Traspaso consumido por la pantalla | `idle` | `stop()`; la reproducción local arranca en `PlayerScreen` |

`armed` y `casting` son los dos estados que dibujan la `CastBar` y los que
alteran el `onTap` de la rejilla (§6.3).

**Quién navega.** `CastNotifier` no navega nunca: publica `failed` con el motivo
y la pantalla activa es la que reacciona empujando o devolviendo el foco a
`PlayerScreen`. Mantener la navegación fuera del notifier es lo que permite
testear el traspaso en CI sin `WidgetTester` ni árbol de widgets.

El traspaso reutiliza el presupuesto de 15 s ya existente (`_kPlayTimeout`) y se
comunica con el idioma de consola establecido en `player_screen.dart:207`:

```
$ korven cast --fallback local
// este canal no viaja a AirPlay
```

`AirplayGuard` es hermano de `PlaybackGuard`, no una refactorización suya. Los
dos clasifican de forma distinta y fundirlos corrompería comportamiento ganado a
pulso: los errores transitorios de HLS en mpv se ignoran deliberadamente
(`playback_guard.dart:101`), mientras que `AVPlayerItem.status == .failed` es
terminal a la primera. `PlaybackGuard` no se toca.

**Peculiaridad honesta:** una app no puede deseleccionar por código una ruta
AirPlay del sistema; esa UI es de Apple. El ⏹ destruye el `AVPlayer` y termina
la sesión, pero la ruta sigue seleccionada a nivel de sistema y macOS puede
seguir enviando el *audio del sistema* al televisor. La barra debe decir «sesión
terminada», no «desconectado», o se leerá como un fallo.

---

## 6. Interfaz

### 6.1 `AirplayButton`

`AppKitView` envolviendo el `AVRoutePickerView` real, encajado en 28×28 dentro de
`actions` del `AppBar` de `HomeScreen` y de `PlayerScreen`, teñido con
`setRoutePickerButtonColor(_:for:)` en el acento Korven. En targets que no son
macOS colapsa a `SizedBox.shrink()`, de modo que la Fase 9 compila sin tocarlo.

### 6.2 `CastBar`

En el hueco `bottomNavigationBar` de ambos scaffolds, ~44 px, presente siempre
que `state != idle`:

```
⧉ Salón Apple TV · BBC News                              ⏹
```

Tipografía mono (`JetBrainsMono`) y `KorvenColors`, siguiendo el idioma de
`ConsoleLine` ya establecido en el reproductor.

### 6.3 Filas y tarjetas de canal

Marca apagada `sin airplay` bajo exactamente dos condiciones: **el canal consta
como incompatible en la memoria local** (§7) **y** hay sesión armada o activa.
Nunca por ausencia de dato. Esa contención es la razón entera de que el
clasificador tenga tres estados: el 42 % del catálogo caerá en `Unknown` (§4.2)
y marcarlo convertiría la rejilla en ruido.

La memoria local se puebla desde dos sitios: el veredicto `airplay_ok == false`
que devuelve `GET /channels/stream?id=` al emitir, y los fallos de formato en
tiempo real de AVFoundation. El modelo `Channel` **no** gana ningún campo: la
rejilla no recibe compatibilidad del gateway (§4.3).

El toque en un canal cambia de comportamiento según el estado: con ruta armada o
emitiendo, casta y **se queda en la rejilla**, sin empujar `PlayerScreen`. Es la
experiencia de segunda pantalla elegida, e implica que `channel_row.dart` y
`channel_card.dart` consulten `castProvider` en `onTap`. Un canal marcado `sin
airplay` se emite igual si se toca; no hay diálogo de confirmación. La insignia
informa, el traspaso recupera.

### 6.4 `Info.plist`

`NSAppTransportSecurity` → `NSAllowsArbitraryLoadsInMedia = true`, para el 19 %
de URLs `http` (§2.2). Es la clave estrecha, limitada a cargas de medios; no se
usa `NSAllowsArbitraryLoads`.

---

## 7. Manejo de errores

| Fallo | Detección | Respuesta |
|---|---|---|
| Apple TV dormido o inalcanzable | Timeout de 15 s, o `item.status == .failed` | → `failed` → traspaso a local |
| Códec rechazado por AVFoundation | `.failed` con `AVErrorDecodeFailed`, `FileFormatNotRecognized` o `FailedToLoadMediaData` | Traspaso **y** recordar el canal como incompatible |
| URL de stream caducada (404) | `.failed`, dominio de error de red | Traspaso, sin escribir memoria |
| Ruta perdida a mitad de emisión | KVO `isExternalPlaybackActive → false` | Sesión → `idle`, la barra se retira con línea de consola |
| Gateway caído durante la emisión | `getStreamUrl` lanza | Sesión → `failed`, reutilizando `ApiError.desde(e).mensaje` |
| Cierre de la app emitiendo | `applicationWillTerminate` | `stop()`; si no, el `AVPlayer` retiene la ruta tras salir |

**La memoria de incompatibilidad vive en el cliente.** Cuando AVFoundation falla
con un error de *formato* —no de red— eso es información de campo que el sondeo
de manifiesto no podía obtener. Pero el gateway sostiene su base con handles de
solo lectura (`channelRepoRO`, `streamRepoRO`, `main.go:129`), y añadir un
endpoint de escritura a una API sin autenticación sería un mal cambio. En su
lugar, la app guarda los IDs de canal fallidos en `SharedPreferences`,
reutilizando el patrón que ya establece `favorites_provider.dart`. Sin endpoint
nuevo, sin invariante rota, y la insignia sigue aprendiendo.

**La distinción de dominio de error es crítica.** Un Apple TV dormido y un stream
MPEG-2 afloran los dos como `.failed`. Persistir incompatibilidad ante un error
de red etiquetaría mal, y para siempre, canales perfectamente buenos. Solo los
códigos de formato de `AVFoundationErrorDomain` escriben en la memoria; todo lo
demás hace traspaso en silencio.

---

## 8. Estrategia de test

### 8.1 Go — puro o `httptest`, sin red

- `codecs_test.go` — tabla sobre ~10 fixtures de manifiesto: master soportado,
  MPEG-2 (`mp4v.20`), sin `CODECS`, playlist de medios, `SAMPLE-AES`, URL
  `.mpd`, `rtmp://`, cuerpo vacío, multivariante mixto.
- `checker_test.go` — el aprovechamiento del GET de fallback produce el
  veredicto.
- `channel_handler_test.go` — `airplay_ok` en `/channels/stream`; la caché
  responde sin repetir la petición; el sondeo que agota su presupuesto devuelve
  `null` sin romper la respuesta ni retrasar la URL.
- `channel_test.go` — **no se toca**: `/channels` sigue en 14 claves.

### 8.2 Dart — todo contra `FakeAirplayPlatform`

- `cast_provider_test.dart` — armado→emisión, cambio de canal sin volver a
  elegir, timeout→traspaso, ruta perdida→`idle`, stop→`idle`, error de
  gateway→`failed`.
- `airplay_guard_test.dart` — el presupuesto de 15 s con `fake_async`, ya
  presente en `dev_dependencies`.
- `cast_bar_test.dart` — se dibuja cuando no está `idle`, ausente cuando lo está.

Ningún test toca un `MethodChannel` real, así que CI sigue verde en Ubuntu
(§2.5).

### 8.3 Verificación manual — obligatoria, no automatizable

1. El selector lista el Apple TV.
2. Emitir un canal bueno: vídeo en el televisor, barra con el nombre del
   dispositivo.
3. Cambiar de canal desde la rejilla sin volver a elegir dispositivo.
4. Emitir un canal `.mpd`: traspaso a local en menos de 15 s con la línea de
   consola.
5. Apagar el televisor a mitad de emisión: la sesión cae a `idle`.
6. Cerrar la app emitiendo: la ruta queda liberada.
7. Emitir un canal `http://`: la excepción ATS funciona.

---

## 9. Riesgos

| # | Riesgo | Severidad | Mitigación |
|---|---|---|---|
| 1 | El código Swift no tiene cobertura de CI (§2.5) | Alta | Superficie nativa mínima (dos canales, sin lógica de negocio). Toda la decisión vive en Dart y en Go, ambos testeados. Checklist manual §8.3. |
| 2 | AVFoundation rechaza canales que mpv reproduce | Alta | Aceptado y gestionado: sondeo (§4) + traspaso a local (§5) + memoria de fallo (§7). |
| 3 | El nombre del dispositivo por CoreAudio es best-effort | Baja | Si no se resuelve, la barra muestra «AirPlay». No bloquea nada. |
| 4 | Permiso de red local en macOS 15+ | Media | AVFoundation enruta vía demonio del sistema, así que probablemente no haya diálogo. **Verificar en el punto 1 de §8.3 antes de dar por buena la fase.** |
| 5 | El sondeo bajo demanda añade latencia al primer casteo de cada canal | Baja | Presupuesto de ~1 s; si expira, devuelve `null` y la URL sale igual. Nunca bloquea la emisión. |
| 6 | La ruta del sistema sigue activa tras ⏹ | Baja | Es comportamiento de Apple, no un bug. Se nombra en la UI (§5). |

---

## 10. Lo que este diseño deliberadamente no hace

- **No añade relay ni proxy de vídeo al gateway.** Con AirPlay el receptor
  descarga del origen; no hay CORS ni contenido mixto que resolver. El gateway
  sigue sin proxear vídeo y sigue escuchando solo en loopback.
- **No toca `PlaybackGuard` ni la ruta de reproducción local.** Es código
  afinado contra fallos reales de campo.
- **No transcodifica ni remuxea.** Un canal que AVFoundation no acepta se
  reproduce en local, y ya.
- **No añade tráfico de fondo ni toca el esquema de SQLite.** Sin relleno, sin
  columna, sin migración y sin romper el contrato JSON de `/channels` (§4.3).
- **No implementa el protocolo AirPlay a mano.** tvOS moderno exige el
  handshake de emparejamiento HAP; reimplementarlo sería frágil y sin soporte.
