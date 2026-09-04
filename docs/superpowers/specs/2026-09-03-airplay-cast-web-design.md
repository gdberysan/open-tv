# Diseño — AirPlay en el cliente web (Korven Open TV)

> **Fecha:** 2026-09-03 · **Estado:** aprobado, pendiente de plan de implementación
> **Reemplaza, para el cliente web, a** `2026-08-08-airplay-cast-design.md` (Flutter/macOS,
> `mobile/` congelado — ese diseño queda como referencia histórica, no se toca).

---

## 1. Objetivo

Que el botón 📺 AirPlay ya presente en `Reproductor.svelte` emita de verdad vídeo y
audio a un Apple TV o televisor AirPlay 2, sin salir de la app: cambiar de canal
mientras se emite no debe obligar a volver a elegir dispositivo.

**Fuera de alcance, decidido explícitamente:** Chromecast queda para un spec
posterior — comparte casi nada de implementación (SDK externo de Google, CSP
relajada, app receptora) y merece su propia revisión.

---

## 2. Restricciones descubiertas — todas verificadas, ninguna supuesta

### 2.1 Causa raíz confirmada con hardware real: AirPlay no tira de fuentes MSE

El botón 📺 ya existía en el código (`Reproductor.svelte:619-621,832`, desde el
26/08) y ya estaba desplegado. Probado en Safari real contra un TV real: el
picker conecta, el TV detecta el Mac como fuente, pero no se ve ni se oye nada
— se queda "cargando" indefinidamente.

Un spike de un solo parche de consola (`HTMLMediaElement.prototype.canPlayType`
forzado a `'probably'` para `application/vnd.apple.mpegurl`, sin tocar el
repo) confirmó la causa: con el motor **nativo** (`<video src=m3u8>` directo,
sin hls.js, sin `blob:`) el mismo canal AirPlayó limpio — vídeo y audio, cero
glitch. Con hls.js/MSE, el handshake de AirPlay conecta pero no reproduce
nada.

**Conclusión:** el pipeline de AirPlay de WebKit necesita un recurso de medio
resoluble (una URL real que el propio motor nativo del `<video>` gestione), no
un `SourceBuffer` alimentado en vivo por JS. Esto no es un bug puntual — es
una incompatibilidad arquitectural entre AirPlay y MSE. `motorDelNavegador()`
(`plan.ts:81-89`) hoy siempre elige hls.js en Safari real (`canPlayType`
devuelve `'maybe'`, no `'probably'`, hallazgo del 28/08), así que ningún canal
usa hoy el motor nativo por decisión propia — hay que forzarlo explícitamente
durante una sesión de AirPlay.

### 2.2 El Mac sigue haciendo el fetch — el proxy loopback no se rompe

Para un `<video>` de página web (a diferencia de una app nativa de tvOS), el
receptor AirPlay no descarga el stream por su cuenta: el Mac sigue haciendo el
fetch y la decodificación localmente, y AirPlay solo remite la señal ya
decodificada al televisor. Por eso reintentar vía el proxy loopback
(`internal/proxy`, 127.0.0.1 únicamente) es seguro y funciona igual que en
reproducción local — el televisor nunca necesita alcanzar el proxy, solo el
propio Safari del Mac.

### 2.3 El motor nativo YA tiene una rama propia, ya probada por el mismo guard

`intentar()` (`Reproductor.svelte:307-`) ya bifurca en `motor === 'nativo'`
(línea 370) y usa el **mismo** `PlaybackGuard` que hls.js, sobre eventos
genéricos de `<video>` (`timeupdate`, `error`) — el guard es agnóstico al
motor. Forzar `motor: 'nativo'` durante un cast no es una ruta nueva de bajo
nivel: es una ruta ya existente, sencillamente poco ejercitada hoy.

### 2.4 `planDeReproduccion`/`planDeFailover` ya aceptan cualquier motor

Ambas funciones (`plan.ts`) son puras y ya reciben `motor` como parámetro,
construyendo la lista directo→proxy en ese orden. Forzar `motor: 'nativo'`
para una sesión de cast reutiliza exactamente esa lógica sin tocarla — cero
código de planificación nuevo.

### 2.5 `clasificarFallo()` ya distingue fallos de formato de los transitorios

`web/src/reproductor/diagnostico.ts` — puro, sin dependencias — ya clasifica
un fallo agotado en `'formato' | 'caido' | 'geo' | 'caducado' | 'desconocido'`
a partir de `video.error.code` (entre otras señales). La memoria de "este
canal no castea" (§6.2) se apoya en esta clasificación ya probada, en vez de
inventar una nueva.

### 2.6 Sin API de nombre de dispositivo

A diferencia del diseño de Flutter (que leía CoreAudio), una página web no
tiene forma de saber el nombre del AirPlay elegido —
`webkitCurrentPlaybackTargetIsWireless` es un booleano puro. Lo que el dueño
vio ("reproduciendo en 65 crystal uhd") era el overlay nativo **del propio
TV**, no algo que la página pueda leer. La UI dice "Emitiendo a AirPlay",
genérico.

### 2.7 Streams en vivo: no hay posición que preservar

Son canales FTA en directo, no VOD. Al pasar de hls.js a nativo (o viceversa)
no hace falta preservar `currentTime`: reconectar simplemente reengancha al
directo, igual que cambiar de canal hoy. Sin esto, todo el diseño sería
bastante más complejo.

---

## 3. Arquitectura: intercambiar el motor EN el mismo `<video>`

Se descartó una segunda etiqueta `<video>` oculta dedicada solo a AirPlay.
Motivo, no solo "menos código": este reproductor tiene exactamente **un**
elemento `<video>` persistente, del que cuelgan fullscreen, PiP, los atajos
de teclado y toda la arquitectura de foco/roving de P0.6-P0.8 (invariante
duro de CLAUDE.md). Un segundo elemento significa que cada feature futura que
toque el reproductor —ya hay dos en cola, DVR record-now y framecapture—
tiene que acordarse para siempre de cuál de los dos es "el de verdad". Es un
impuesto que se paga en cada cambio futuro, no solo al construir esto.

El riesgo real de reusar el mismo elemento —higiene del `hls.destroy()` antes
de asignar un `src` nativo, y una reinicialización limpia al volver— es
acotado y testeable una sola vez, no un impuesto recurrente.

**Mecánica:** al entrar en `conectando` (§4), se destruye la instancia de
hls.js activa, se fuerza `motor: 'nativo'` (sin pasar por
`motorDelNavegador()`), se reconstruye la lista de intentos con
`planDeReproduccion`/`planDeFailover` igual que en reproducción local, y se
llama a `intentar(intento, 'nativo')` — la misma función, la misma rama que
ya existe. Al salir de un cast (parada, fallo agotado, o ruta perdida), se
revierte: se limpia el `src` nativo y se reinicia el flujo normal de
`reproducir()`, que vuelve a elegir hls.js.

---

## 4. Máquina de estados

| Desde | Evento | Hasta | Acción |
|---|---|---|---|
| `idle` | `webkitcurrentplaybacktargetiswirelesschanged` → `true` | `conectando` | Destruir hls.js, forzar motor nativo, reconstruir intentos, cubrir el vídeo con el panel "Emitiendo…" |
| `conectando` | Intento directo falla/agota timeout | `conectando` | Reintento vía proxy (mismo motor nativo) — mismo patrón directo→proxy que ya existe |
| `conectando` | Proxy también falla/agota, sin más mirrors | `fallido` → `idle` | Revertir a hls.js, **reanudar reproducción local del mismo canal**, mensaje "este canal no puede emitirse"; si la clase de fallo es `'formato'` (§2.5), persistir el veredicto (§6.2) |
| `conectando` | `PlaybackGuard.alConfirmar()` | `emitiendo` | El panel muestra el control de parar |
| `emitiendo` | Cambio de canal (rejilla/flechas) | `conectando` | Repite la mecánica de arriba para el canal nuevo, **sin volver a abrir el selector** — misma ruta AirPlay |
| `emitiendo` | Botón "Parar" | `idle` | Revertir a hls.js, reanudar local |
| `emitiendo` | `webkitcurrentplaybacktargetiswirelesschanged` → `false` (TV se apaga) | `idle` | Igual que "Parar", más una línea de estado breve "emisión terminada" |

---

## 5. Interfaz

- **Panel de cast:** reutiliza el `{#if cargando} … {:else if mensajeError} …`
  ya existente en `Reproductor.svelte`, añadiendo una rama
  `{:else if estadoCast !== 'idle'}` con "Emitiendo a AirPlay — `<canal.nombre>`"
  y un control para parar. Sin nombre de dispositivo (§2.6).
- **Botón 📺:** ámbar mientras `estadoCast !== 'idle'`, siguiendo la regla de
  marca ya establecida (ámbar = señal-viva/activo).
- **Atajo 'm' (silenciar):** deshabilitado mientras se emite. El mismo
  `<video>` alimenta al TV (§3), así que silenciar localmente muy
  probablemente silencia también la salida real del televisor — un accidente
  de teclado no debe cortar el sonido del TV. No-op durante el cast.
- **Atajo 'f' (pantalla completa):** no-op durante el cast — no hay vídeo
  local visible que expandir, está cubierto por el panel.
- **Flechas/cambio de canal:** siguen activas y reutilizan la sesión AirPlay
  existente (tabla §4).

---

## 6. Manejo de errores

### 6.1 Agotamiento del presupuesto de reintento

Igual que hoy: cada intento (`PlaybackGuard.armarTimeoutDeCarga()`) tiene su
presupuesto de carga. Agotados el intento directo y el de proxy, sin más
mirrors, se cae a `fallido` → `idle`: nunca se deja el TV con una pantalla
negra colgada — se revierte a reproducción local con un mensaje claro.

### 6.2 Memoria de canales que no casteán

Solo se persiste un veredicto de "no castea" cuando `clasificarFallo()`
(§2.5) devuelve `'formato'` — nunca ante `'caido'`, `'geo'`, `'caducado'` ni
`'desconocido'`: esos son fallos potencialmente transitorios (red floja, mirror
caído un momento) y blacklistear el canal para siempre por uno de ellos sería
el mismo error que el spec de Flutter ya identificó y evitó (§7 de ese
documento). El siguiente intento de castear ese canal se salta directo a
`fallido` sin gastar el presupuesto de reintento (~30 s directo+proxy).

**Nota para quien implemente:** un agotamiento de `PlaybackGuard` por puro
timeout (sin `MediaError` ni evento de hls.js de por medio, ninguno de los dos
intentos llegó a fallar con una señal concreta) no trae `mediaErrorCode` ni
`tipoHls` — `clasificarFallo()` cae a `'desconocido'` por construcción, así
que NO se persiste. Es el comportamiento correcto (un timeout es la definición
de "transitorio"), pero es fácil asumir por error que "se agotó el
presupuesto" y "es un fallo de formato" son lo mismo — no lo son.

Vive en `localStorage`, siguiendo el patrón ya establecido por
`web/src/estado/favoritos.ts`/`historial.ts` (mismo tipo de módulo, mismo
estilo de test). **Sin insignia en la rejilla** — la memoria es solo una
optimización de latencia al intentar castear, no una superficie de UI nueva;
una insignia predictiva queda fuera de alcance (§7).

---

## 7. Estrategia de test

- **Puro:** un reductor `estadoCast` (idle/conectando/emitiendo/fallido),
  testeado en aislamiento igual que `PlaybackGuard`/`clasificarFallo`. El
  nuevo módulo de memoria de fallos (§6.2), testeado igual que
  `favoritos.test.ts`.
- **Componente (`Reproductor.test.ts`):** mockear
  `webkitcurrentplaybacktargetiswirelesschanged` (mismo patrón que ya mockea
  `WebKitPlaybackTargetAvailabilityEvent`) para cubrir: destrucción de hls.js
  al entrar en cast, reintento directo→proxy, reversión a hls.js al fallar o
  parar, cambio de canal sin reabrir el selector, atajo 'm' deshabilitado,
  panel de cast reemplazando la vista de vídeo.
- **Verificación manual, obligatoria, no automatizable** (mismo motivo que la
  pasada de Safari real por `safaridriver` ya documentada):
  1. Castear un canal bueno: vídeo y audio limpios en el TV (ya confirmado
     por el spike).
  2. Castear un canal que necesita el proxy: confirmar que el reintento
     funciona de punta a punta.
  3. Castear un canal con códec no soportado nativamente: cae a local con
     mensaje, sin dejar el TV colgado.
  4. Cambiar de canal en plena emisión: mismo dispositivo, sin picker de
     nuevo.
  5. Apagar el TV en plena emisión: la sesión cae a `idle` limpia.
  6. Probar 'm' durante la emisión: no hace nada.
  7. Volver a castear un canal ya marcado `'formato'`: falla rápido, sin
     gastar el presupuesto de reintento.

---

## 8. Riesgos

| # | Riesgo | Severidad | Mitigación |
|---|---|---|---|
| 1 | Higiene de `hls.destroy()`/reinicialización incompleta deja estado colgado | Media | Acotado a un solo punto de intercambio de motor (§3); cubierto por tests de componente que verifican limpieza en ambas direcciones |
| 2 | `webkitcurrentplaybacktargetiswirelesschanged` no dispara de forma fiable en todas las versiones de Safari | Media | Solo verificable con hardware real (§7); si no dispara, el peor caso es quedarse en `conectando` — el timeout de `PlaybackGuard` ya cubre ese caso |
| 3 | Silenciar localmente no silencia de verdad la salida AirPlay (o viceversa) | Baja | Mitigado a propósito deshabilitando 'm' entero durante el cast (§5), en vez de intentar acertar el comportamiento exacto de WebKit |
| 4 | Canal que en realidad SÍ castearía bien se marca `'formato'` por un fallo puntual mal clasificado | Baja | `clasificarFallo()` ya es la misma lógica que clasifica errores de reproducción local hoy, ya probada; no es lógica nueva sin pulir |

---

## 9. Lo que este diseño deliberadamente no hace

- **No incluye Chromecast.** SDK externo de Google, CSP distinta, spec propio
  (§1).
- **No añade insignia predictiva en la rejilla.** La memoria de fallos (§6.2)
  es interna, no UI nueva — YAGNI hasta que se demuestre falta.
- **No define interacción con DVR record-now** (spec en cola,
  `2026-08-26-dvr-grabacion-local-design.md`). Esa feature decide, en su
  propio spec, cómo conviven grabación y una sesión de cast activa.
- **No toca `PlaybackGuard` ni `plan.ts`.** Se reutilizan sin modificar — el
  cast es un nuevo llamador de código ya probado, no una reescritura.
- **No preserva posición de reproducción entre motores** (§2.7) — innecesario
  para directo en vivo.
- **No expone nombre de dispositivo** (§2.6) — no hay API del navegador que lo
  dé.
