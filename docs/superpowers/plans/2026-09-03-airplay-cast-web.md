# AirPlay en el cliente web — Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que el botón 📺 AirPlay ya presente en `Reproductor.svelte` emita de
verdad vídeo y audio a un Apple TV/AirPlay 2 real, forzando el motor nativo
(`<video src>`, sin hls.js/MSE) durante una sesión de cast, reutilizando la
máquina de reproducción existente sin duplicarla.

**Architecture:** Un solo `<video>` persistente (sin segundo elemento) cuyo
motor se fuerza a `'nativo'` mientras dura el cast, reusando literalmente
`reproducir()`/`intentar()`/`planDeReproduccion`/`planDeFailover`/
`PlaybackGuard`/`clasificarFallo` tal cual existen hoy — el único código
genuinamente nuevo es el estado de la sesión de cast, el listener del evento
de WebKit, la UI del panel de cast, y un módulo de memoria de fallos de
formato.

**Tech Stack:** Svelte 5 (runas), TypeScript, Vitest + Testing Library
(`@testing-library/svelte`), sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-09-03-airplay-cast-web-design.md`

## Global Constraints

- **`mobile/` CERO diffs** — este plan no toca nada bajo `mobile/`.
- **Ninguna dependencia nueva.** Todo lo de este plan usa APIs del navegador
  y código ya existente en el repo.
- **`gofmt -l .` / `go vet` / `go build` / `go test -race` / `golangci-lint
  run ./...` / `go run ./tools/scrubcheck`** siguen en verde — este plan no
  toca Go, pero los gates se corren igual al final de cada tarea (invariante
  del proyecto).
- **`cd web && npm run check && npm test && npm run build`** en verde al
  final de cada tarea.
- **Español fuente de verdad en `web/src/i18n/es.ts`**, inglés en paridad
  exacta (`en.ts` tipado como `Record<ClaveMensaje, string>`, y
  `i18n.test.ts` verifica claves idénticas en ambos).
- **NUNCA `git add -A`** — añadir por ruta explícita.
- **Autor de commits:** `Gerard <gdberysan@gmail.com>` (ya verificado en esta
  sesión). Trailer `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`.
- **Sin animación nueva** que `prefers-reduced-motion` deba anular — el panel
  de cast no lleva transición de entrada, igual que el resto del overlay 1b.
- **Ámbar = señal-viva/activo/foco**, nada hardcodeado — el botón AirPlay
  activo usa la clase `.activo` ya existente
  (`.overlay-controles button.activo { color: var(--amber-500); }`,
  `Reproductor.svelte:994`), no un color nuevo.
- **Cero regiones `aria-live` nuevas.** Las dos ya existentes en
  `Reproductor.svelte:738-739` (`cargando` y `mensajeError`) son las ÚNICAS
  anunciadoras — el estado de cast reusa su `textContent`, no añade una
  tercera región.

---

### Task 1: `castFallidos` — memoria de canales que no casteán por formato

**Files:**
- Create: `web/src/estado/castFallidos.ts`
- Test: `web/src/estado/castFallidos.test.ts`

**Interfaces:**
- Consumes: nada (módulo puro, sin dependencias, mismo patrón que
  `web/src/estado/favoritos.ts`).
- Produces: `noCasteaPorFormato(canalId: string): boolean`,
  `marcarFalloFormato(canalId: string): void` — los consume la Task 3.

- [ ] **Step 1: Escribir el test que falla**

```typescript
// web/src/estado/castFallidos.test.ts
import { beforeEach, describe, expect, it } from 'vitest'
import { noCasteaPorFormato, marcarFalloFormato } from './castFallidos'

beforeEach(() => localStorage.clear())

describe('castFallidos', () => {
  it('un canal nunca marcado no está en la memoria', () => {
    expect(noCasteaPorFormato('bbc')).toBe(false)
  })

  it('marcar y persistir', () => {
    marcarFalloFormato('bbc')
    expect(noCasteaPorFormato('bbc')).toBe(true)
    // Otra "instancia" (el módulo es sin estado propio, lee localStorage
    // directamente) ve lo mismo — es lo que pasa al recargar la página.
    expect(noCasteaPorFormato('bbc')).toBe(true)
  })

  it('marcar dos veces el mismo canal no duplica ni rompe nada', () => {
    marcarFalloFormato('bbc')
    marcarFalloFormato('bbc')
    expect(noCasteaPorFormato('bbc')).toBe(true)
  })

  it('canales distintos no se pisan', () => {
    marcarFalloFormato('bbc')
    expect(noCasteaPorFormato('itv')).toBe(false)
  })

  // Un localStorage con basura no puede tumbar el intento de castear.
  it('sobrevive a datos corruptos', () => {
    localStorage.setItem('opentv.cast.sinFormato', '{no es json')
    expect(noCasteaPorFormato('bbc')).toBe(false)
    expect(() => marcarFalloFormato('bbc')).not.toThrow()
  })
})
```

- [ ] **Step 2: Verificar que el test falla**

Run: `cd web && npx vitest run src/estado/castFallidos.test.ts`
Expected: FAIL — `Cannot find module './castFallidos'`

- [ ] **Step 3: Implementación mínima**

```typescript
// web/src/estado/castFallidos.ts
// Memoria de qué canales NO se pueden castear por AirPlay porque el motor
// nativo (forzado durante una sesión de cast, ver Reproductor.svelte) los
// rechaza por formato/códec — nunca por un fallo de red o timeout, que son
// transitorios (ver spec §6.2). Sin store de Svelte a propósito: se lee solo
// al INICIAR un intento de cast, no hace falta reactividad — mismo motivo
// por el que no hay insignia en la rejilla (fuera de alcance, spec §9).
//
// Mismo patrón defensivo que favoritos.ts: localStorage puede estar
// bloqueado (modo privado) o contener basura; en ningún caso debe romper el
// intento de castear.
const CLAVE = 'opentv.cast.sinFormato'

function leer(): Set<string> {
  try {
    const crudo = localStorage.getItem(CLAVE)
    if (!crudo) return new Set()
    const datos = JSON.parse(crudo)
    return Array.isArray(datos) ? new Set(datos.filter((x) => typeof x === 'string')) : new Set()
  } catch {
    return new Set()
  }
}

export function noCasteaPorFormato(canalId: string): boolean {
  return leer().has(canalId)
}

export function marcarFalloFormato(canalId: string): void {
  try {
    const s = leer()
    s.add(canalId)
    localStorage.setItem(CLAVE, JSON.stringify([...s]))
  } catch {
    // Ver arriba: localStorage bloqueado no es fatal, solo se pierde la
    // optimización de saltar el reintento la próxima vez.
  }
}
```

- [ ] **Step 4: Verificar que el test pasa**

Run: `cd web && npx vitest run src/estado/castFallidos.test.ts`
Expected: PASS, 5 tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/estado/castFallidos.ts web/src/estado/castFallidos.test.ts
git commit -m "feat(web): memoria de canales que no casteán por formato

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 2: Claves de idioma para el cast

**Files:**
- Modify: `web/src/i18n/es.ts:131` (insertar tras el bloque `reproductor.error.*`)
- Modify: `web/src/i18n/en.ts:78` (mismo punto, en paridad)

**Interfaces:**
- Consumes: nada.
- Produces: las claves `reproductor.cast.emitiendo`, `reproductor.cast.parar`,
  `reproductor.cast.fallo`, `reproductor.cast.noDisponible`,
  `reproductor.cast.terminada`, `reproductor.airplay` — las consume la Task 4
  (y `reproductor.airplay` sustituye el `aria-label="AirPlay"` hardcodeado
  que ya existe en `Reproductor.svelte:832`, por consistencia con el resto
  de botones del overlay, que ya usan `t(...)` para su `aria-label`).

- [ ] **Step 1: Añadir las claves en español**

En `web/src/i18n/es.ts`, tras la línea 131
(`'reproductor.error.probarSiguienteMirror': 'Probar el siguiente mirror',`)
y antes de la línea en blanco que sigue, insertar:

```typescript
  // AirPlay (spec 2026-09-03): sin nombre de dispositivo posible — WebKit no
  // expone uno a la página (ver spec §2.6) — así que el texto es genérico.
  'reproductor.airplay': 'AirPlay',
  'reproductor.cast.emitiendo': 'Emitiendo a AirPlay — {canal}',
  'reproductor.cast.parar': 'Dejar de emitir',
  'reproductor.cast.fallo': 'Este canal no se puede emitir por AirPlay. Sigue reproduciéndose aquí.',
  'reproductor.cast.noDisponible': 'Este canal no se pudo emitir por AirPlay antes. Sigue reproduciéndose aquí.',
  'reproductor.cast.terminada': 'Emisión por AirPlay terminada.',
```

- [ ] **Step 2: Añadir las claves en inglés, misma posición relativa**

En `web/src/i18n/en.ts`, tras la línea 78
(`'reproductor.error.probarSiguienteMirror': 'Try the next mirror',`):

```typescript
  'reproductor.airplay': 'AirPlay',
  'reproductor.cast.emitiendo': 'Casting to AirPlay — {canal}',
  'reproductor.cast.parar': 'Stop casting',
  'reproductor.cast.fallo': "This channel can't cast over AirPlay. Still playing here.",
  'reproductor.cast.noDisponible': "This channel couldn't cast over AirPlay before. Still playing here.",
  'reproductor.cast.terminada': 'AirPlay casting ended.',
```

- [ ] **Step 3: Verificar paridad y build**

Run: `cd web && npx vitest run src/i18n/i18n.test.ts && npm run check`
Expected: PASS — mismas claves en ambos idiomas, ningún texto vacío,
`svelte-check`/`tsc` en verde.

- [ ] **Step 4: Commit**

```bash
git add web/src/i18n/es.ts web/src/i18n/en.ts
git commit -m "feat(web): textos de la sesión de cast AirPlay (es/en)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 3: Forzar el motor nativo durante una sesión de cast

Esta es la tarea central: engancha el evento real de WebKit a la máquina de
reproducción ya existente, sin duplicar su lógica de reintento.

**Files:**
- Modify: `web/src/componentes/Reproductor.svelte`
- Test: `web/src/componentes/Reproductor.test.ts`

**Interfaces:**
- Consumes: `noCasteaPorFormato`/`marcarFalloFormato` (Task 1),
  `t('reproductor.cast.*')` (Task 2), y TODO lo ya existente en el fichero:
  `reproducir()`, `limpiarIntento()`, `motorDelNavegador`, `clasificarFallo`,
  `PlaybackGuard`.
- Produces: los `$state` `motorForzado: Motor | null` y
  `estadoCast: 'idle' | 'conectando' | 'emitiendo'` y la función
  `pararCast(): void` — los consume la Task 4 para la UI.

- [ ] **Step 1: Importar el nuevo módulo**

En `web/src/componentes/Reproductor.svelte`, junto a los demás imports de
`../estado/` (línea 13, `import { favoritos } from '../estado/favoritos'`):

```typescript
  import { noCasteaPorFormato, marcarFalloFormato } from '../estado/castFallidos'
```

- [ ] **Step 2: Declarar el estado del cast**

Junto a `let guardActual: PlaybackGuard | undefined` (línea 226), añadir:

```typescript
  // Sesión de AirPlay (spec 2026-09-03): motorForzado fuerza 'nativo' en
  // reproducir() en vez de dejar que motorDelNavegador() decida — hoy en
  // Safari real SIEMPRE elige hls.js (canPlayType devuelve 'maybe', no
  // 'probably'; ver plan.ts), y AirPlay no reproduce fuentes MSE/blob
  // (confirmado con hardware real, spec §2.1). null = sin cast en curso.
  // El tipo NO incluye 'fallido' (spec §4): ese estado es una transición
  // instantánea de un solo tick (Step 4 más abajo hace el aviso + vuelve a
  // 'idle' + reproducir() en la misma pasada), nunca algo que la UI necesite
  // pintar de forma sostenida — por eso no hay una cuarta rama visual en la
  // Task 4.
  let motorForzado: Motor | null = $state(null)
  let estadoCast = $state<'idle' | 'conectando' | 'emitiendo'>('idle')
  // Mensaje transitorio de cast (fallo o fin de sesión) — reusa las mismas
  // dos regiones aria-live de siempre (líneas 738-739), NUNCA una tercera.
  let avisoCast = $state<string | null>(null)
```

- [ ] **Step 3: Forzar el motor en `reproducir()`**

En la línea 429, cambiar:

```typescript
    const motor = motorDelNavegador(video)
```

por:

```typescript
    const motor = motorForzado ?? motorDelNavegador(video)
```

- [ ] **Step 4: Manejar el agotamiento del cast por separado del error normal**

En el bloque de agotamiento del bucle de intentos (líneas 521-531), donde
hoy dice:

```typescript
    if (destruido || miId !== intentoId) return
    limpiarIntento()
    cargando = false
    mensajeError = t(claveDeClase(ultimaClase))
    numMirrorsDisponibles = totalMirrors
```

insertar el caso de cast ANTES de tocar `mensajeError` (que es la tarjeta de
error a pantalla completa — un cast fallido no debe mostrarla, porque hls.js
probablemente sí reproduce este canal con normalidad):

```typescript
    if (destruido || miId !== intentoId) return
    limpiarIntento()
    if (motorForzado === 'nativo') {
      // El motor nativo falló — pero hls.js (motorDelNavegador de verdad)
      // probablemente SÍ reproduce este canal, así que no se muestra la
      // tarjeta de error a pantalla completa: solo un aviso transitorio,
      // y se reanuda la reproducción local normal.
      if (ultimaClase === 'formato') marcarFalloFormato(canal.id)
      avisoCast = t('reproductor.cast.fallo')
      motorForzado = null
      estadoCast = 'idle'
      reproducir()
      return
    }
    cargando = false
    mensajeError = t(claveDeClase(ultimaClase))
    numMirrorsDisponibles = totalMirrors
```

- [ ] **Step 5: Derivar `estadoCast = 'emitiendo'` cuando el intento nativo confirma**

Junto al `$effect` que ya detecta reproducción confirmada para el
auto-ocultar del overlay (línea 197-199,
`$effect(() => { if (!cargando && !mensajeError) mostrar() })`), añadir un
efecto hermano, mismo idioma reactivo que el resto del fichero:

```typescript
  // Misma idea que el $effect de arriba (mostrar()): cargando=false Y
  // mensajeError=null significa "este intento confirmó reproducción",
  // cualquiera sea el motor. Con motorForzado='nativo' eso es "la sesión de
  // AirPlay está reproduciendo de verdad" — PlaybackGuard.alConfirmar() ya
  // hizo cargando=false, aquí solo se traduce a estadoCast.
  $effect(() => {
    if (motorForzado === 'nativo' && estadoCast === 'conectando' && !cargando && !mensajeError) {
      estadoCast = 'emitiendo'
    }
  })
```

- [ ] **Step 6: `iniciarCast()` y `pararCast()`**

Junto a `abrirSelectorAirplay()` (línea 619-622), añadir:

```typescript
  function iniciarCast() {
    if (noCasteaPorFormato(canal.id)) {
      avisoCast = t('reproductor.cast.noDisponible')
      return
    }
    avisoCast = null
    motorForzado = 'nativo'
    estadoCast = 'conectando'
    limpiarIntento()
    reproducir()
  }

  function pararCast() {
    motorForzado = null
    estadoCast = 'idle'
    avisoCast = t('reproductor.cast.terminada')
    limpiarIntento()
    reproducir()
  }

  // Traduce el evento REAL de WebKit (se dispara cuando el usuario elige o
  // suelta una ruta en el picker nativo que abre abrirSelectorAirplay(), y
  // también si el TV se apaga a mitad de emisión) al arranque/parada del
  // cast. video.webkitCurrentPlaybackTargetIsWireless no está en el tipo
  // HTMLVideoElement — API solo de WebKit, igual que
  // webkitShowPlaybackTargetPicker más arriba.
  function alCambioRutaAirplay() {
    // @ts-expect-error API solo de WebKit
    const activa = Boolean(video?.webkitCurrentPlaybackTargetIsWireless)
    if (activa) {
      iniciarCast()
    } else if (estadoCast !== 'idle') {
      pararCast()
    }
  }
```

- [ ] **Step 7: Enganchar el listener cuando el `<video>` está montado**

Junto al `$effect` de EPG (línea 148-156) o cualquier otro `$effect` de
efecto-secundario-con-cleanup ya existente, añadir uno nuevo — se dispara una
sola vez porque `video` es el mismo elemento persistente durante toda la vida
del componente (no cambia al cambiar de canal):

```typescript
  // El listener se engancha una sola vez: video es el MISMO elemento
  // persistente durante toda la vida del panel (spec §3) — no hay que
  // re-enganchar al cambiar de canal.
  $effect(() => {
    if (!video || !soportaAirplay) return
    const el = video
    el.addEventListener('webkitcurrentplaybacktargetiswirelesschanged', alCambioRutaAirplay)
    return () => el.removeEventListener('webkitcurrentplaybacktargetiswirelesschanged', alCambioRutaAirplay)
  })
```

- [ ] **Step 8: Test — forzar el motor nativo arranca por la rama `nativo` de `intentar()`**

Añadir a `web/src/componentes/Reproductor.test.ts`, en el mismo `describe`
que ya cubre el motor nativo (buscar el test existente que fija
`canPlayType` a `'probably'`, o si no existe ninguno, seguir el patrón del
bloque `soportaAirplay` de la línea ~789 para fijar
`WebKitPlaybackTargetAvailabilityEvent` en el `window` de prueba):

```typescript
  it('el evento webkitcurrentplaybacktargetiswirelesschanged fuerza el motor nativo, sin hls.js', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    try {
      const destino = vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null }))
      const { container } = render(Reproductor, {
        props: {
          canal: canalDePrueba(),
          fuente: fuenteDePrueba({ destino }),
        },
      })
      await tick()

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()

      // Motor nativo: video.src se asigna directo (rama motor==='nativo' de
      // intentar(), línea 370), NUNCA pasa por el import('hls.js') mockeado
      // de este fichero — si hls.js se hubiera instanciado, hlsState
      // tendría una entrada.
      expect(video.src).toContain('x.m3u8')
      expect(hlsState.instancias.length).toBe(0)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
    }
  })
```

> Nota para quien implemente: revisar los helpers que el fichero ya usa
> (`canalDePrueba`/`fuenteDePrueba` o los literales inline que ya usan otros
> `it(...)` de este fichero — seguir EXACTAMENTE el patrón que ya exista ahí,
> no inventar uno nuevo) y ajustar los nombres si difieren. El punto no
> negociable del test es la aserción final: motor nativo, cero instancias de
> hls.js.

- [ ] **Step 9: Verificar que el test pasa junto al resto**

Run: `cd web && npx vitest run src/componentes/Reproductor.test.ts`
Expected: PASS, incluyendo todos los tests preexistentes (no hay
regresión).

- [ ] **Step 10: Test — el agotamiento en modo cast no muestra la tarjeta de error y reanuda local**

```typescript
  it('agotar los intentos en modo cast NO muestra mensajeError y reanuda hls.js', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    try {
      const destino = vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null }))
      const { container } = render(Reproductor, {
        props: {
          canal: canalDePrueba(),
          fuente: fuenteDePrueba({ destino }),
        },
      })
      await tick()

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()

      // Motor nativo: el fallo se señala con el evento 'error' nativo del
      // <video>, código 4 = MEDIA_ERR_SRC_NOT_SUPPORTED (jsdom no reproduce
      // vídeo de verdad, así que se dispara a mano).
      Object.defineProperty(video, 'error', { value: { code: 4 }, configurable: true })
      video.dispatchEvent(new Event('error'))
      await tick()

      // Sin más mirrors tras el fallo nativo: reproducir() cae al bloque de
      // cast (Step 4) y llama reproducir() de nuevo — esta vez SIN
      // motorForzado, así que motorDelNavegador(video) decide, y en jsdom
      // (sin canPlayType real) eso es 'hlsjs'. hls.js mockeado registra una
      // instancia.
      await tick()
      expect(hlsState.instancias.length).toBeGreaterThan(0)
      expect(screen.queryByText(t('reproductor.cast.fallo'))).toBeNull() // no hay tarjeta de error visible
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
    }
  })
```

> Nota: `screen.queryByText(t('reproductor.cast.fallo'))` verifica que el
> texto NO está en una tarjeta visible — el aviso real vive en la región
> `sr-only` (Task 4), que sí contiene el texto pero no está pensada como
> "tarjeta". Si el matcher de arriba resulta ambiguo una vez la Task 4 esté
> hecha, cambiarlo por comprobar que `.estado.error` (la clase de la tarjeta
> de `mensajeError`) no está presente:
> `expect(container.querySelector('.estado.error')).toBeNull()`.

- [ ] **Step 11: Verificar el test**

Run: `cd web && npx vitest run src/componentes/Reproductor.test.ts`
Expected: PASS.

- [ ] **Step 12: Gates completos**

Run: `cd web && npm run check && npm test && npm run build`
Expected: todo en verde.

- [ ] **Step 13: Commit**

```bash
git add web/src/componentes/Reproductor.svelte web/src/componentes/Reproductor.test.ts
git commit -m "feat(web): forzar motor nativo durante una sesión de AirPlay

Reusa reproducir()/intentar()/planDeReproduccion/planDeFailover/
PlaybackGuard/clasificarFallo sin duplicarlos: motorForzado hace que
reproducir() use 'nativo' en vez de motorDelNavegador(). Un fallo en
modo cast reanuda hls.js en vez de mostrar la tarjeta de error — el
canal probablemente sí reproduce en local aunque no caste.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 4: Interfaz — panel de cast, botón activo, teclado, anuncios

**Files:**
- Modify: `web/src/componentes/Reproductor.svelte`
- Test: `web/src/componentes/Reproductor.test.ts`

**Interfaces:**
- Consumes: `estadoCast`, `motorForzado`, `avisoCast`, `pararCast()`,
  `iniciarCast()`/`alCambioRutaAirplay` (Task 3), `t('reproductor.cast.*')`
  (Task 2).
- Produces: nada que otra tarea consuma — es la superficie visible final.

- [ ] **Step 1: Deshabilitar 'm' y 'f' durante el cast**

En `alTeclado` (líneas 638-671), cambiar:

```typescript
      case 'm':
        alternarSilencio()
        break
```

por:

```typescript
      case 'm':
        // Con estadoCast !== 'idle' el MISMO <video> alimenta al TV (spec
        // §3/§5): silenciar aquí muy probablemente silencia el TV también.
        // No-op a propósito mientras se está emitiendo o conectando.
        if (estadoCast === 'idle') alternarSilencio()
        break
```

y:

```typescript
      case 'f':
        alternarPantallaCompleta()
        break
```

por:

```typescript
      case 'f':
        // No hay vídeo local visible que expandir durante el cast — está
        // cubierto por el panel "Emitiendo…" (Step 3 de esta tarea).
        if (estadoCast === 'idle') alternarPantallaCompleta()
        break
```

- [ ] **Step 2: Botón AirPlay refleja el estado activo**

En la línea 829-833, cambiar:

```svelte
          {#if soportaAirplay}
            <!-- La barra inferior del modal se fue con el modal: AirPlay vive
                 aquí, junto a PiP (decisión 4 del plan reproductor-primero). -->
            <button type="button" class="airplay" onclick={abrirSelectorAirplay} aria-label="AirPlay">📺</button>
          {/if}
```

por:

```svelte
          {#if soportaAirplay}
            <!-- La barra inferior del modal se fue con el modal: AirPlay vive
                 aquí, junto a PiP (decisión 4 del plan reproductor-primero).
                 class:activo reusa la regla de marca ya existente (ámbar =
                 señal-viva/activo, Reproductor.svelte:994) — sin CSS nuevo. -->
            <button
              type="button"
              class="airplay"
              class:activo={estadoCast !== 'idle'}
              onclick={abrirSelectorAirplay}
              aria-pressed={estadoCast !== 'idle'}
              aria-label={t('reproductor.airplay')}
            >📺</button>
          {/if}
```

- [ ] **Step 3: Panel de cast reemplaza la vista de vídeo mientras dura**

Localizar el bloque (líneas 717-729):

```svelte
    {#if cargando}
      <p class="estado">{t('reproductor.cargando')}</p>
    {:else if mensajeError}
      <div class="estado error">
        <p class="mensaje">{mensajeError}</p>
        {#if numMirrorsDisponibles > 0}
          <p class="mirrors">{t('reproductor.error.mirrorsDisponibles', { n: numMirrorsDisponibles })}</p>
          <button type="button" class="probar-mirror" onclick={probarSiguienteMirror}>
            {t('reproductor.error.probarSiguienteMirror')}
          </button>
        {/if}
      </div>
    {/if}
```

y añadir una rama de cast ANTES del cierre — solo se pinta con
`estadoCast === 'emitiendo'`: mientras `estadoCast === 'conectando'` ya se
ve `cargando` (que sigue en `true` hasta que `PlaybackGuard.alConfirmar()`
dispara), así que no hace falta un estado visual propio para "conectando",
evita un tercer texto de carga redundante:

```svelte
    {#if cargando}
      <p class="estado">{t('reproductor.cargando')}</p>
    {:else if mensajeError}
      <div class="estado error">
        <p class="mensaje">{mensajeError}</p>
        {#if numMirrorsDisponibles > 0}
          <p class="mirrors">{t('reproductor.error.mirrorsDisponibles', { n: numMirrorsDisponibles })}</p>
          <button type="button" class="probar-mirror" onclick={probarSiguienteMirror}>
            {t('reproductor.error.probarSiguienteMirror')}
          </button>
        {/if}
      </div>
    {:else if estadoCast === 'emitiendo'}
      <div class="estado cast">
        <p class="mensaje">{t('reproductor.cast.emitiendo', { canal: canal.nombre })}</p>
        <button type="button" class="parar-cast" onclick={pararCast}>
          {t('reproductor.cast.parar')}
        </button>
      </div>
    {/if}
```

- [ ] **Step 4: Reusar las regiones `sr-only` YA EXISTENTES para el aviso de cast**

Cambiar las líneas 738-739, de:

```svelte
    <p class="sr-only" aria-live="polite" aria-atomic="true">{cargando ? t('reproductor.cargando') : ''}</p>
    <p class="sr-only" role="alert" aria-live="assertive" aria-atomic="true">{mensajeError ?? ''}</p>
```

a:

```svelte
    <!-- Cast (spec 2026-09-03): reusa estas DOS regiones ya existentes, no
         añade una tercera — invariante duro del proyecto (CLAUDE.md). -->
    <p class="sr-only" aria-live="polite" aria-atomic="true">
      {cargando ? t('reproductor.cargando') : estadoCast === 'emitiendo' ? t('reproductor.cast.emitiendo', { canal: canal.nombre }) : ''}
    </p>
    <p class="sr-only" role="alert" aria-live="assertive" aria-atomic="true">{mensajeError ?? avisoCast ?? ''}</p>
```

- [ ] **Step 5: Estilo mínimo del panel de cast**

En el bloque `<style>`, justo después de las líneas 901-902
(`.estado.error .mensaje { color: var(--signal-error); margin: 0; }` /
`.estado.error .mirrors { color: var(--text-muted); margin: 0; }`), añadir:

```css
  /* Sin color de error (var(--signal-error)) a propósito: fallar el cast no
     es un error de reproducción — hls.js sigue funcionando en local. Mismo
     margin:0 que .estado.error .mensaje, por la misma razón (el <p> suelto
     traería el margen por defecto del user-agent y desalinearía el gap del
     flex de .estado). */
  .estado.cast .mensaje { color: var(--text-body); margin: 0; }
```

Y justo después de la línea 915 (`.probar-mirror:hover { background:
var(--tint-amber-line); }`), añadir un bloque hermano — mismo tratamiento
visual que `.probar-mirror`, clase separada porque es una acción distinta
(parar un cast, no reintentar un mirror), siguiendo el mismo criterio que ya
usa el fichero de una clase por acción en vez de compartir selector:

```css
  .parar-cast {
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-4);
    color: var(--amber-500);
    cursor: pointer;
    font: inherit;
  }
  .parar-cast:hover { background: var(--tint-amber-line); }
```

- [ ] **Step 6: Test — cambiar de canal en pleno cast NO reabre el selector**

```typescript
  it('cambiar de canal durante un cast activo sigue emitiendo, sin volver a elegir dispositivo', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    try {
      const destino = vi.fn(async (id: string) => ({ url: `https://unico/${id}.m3u8`, airplayOk: null }))
      const canalA = canalDePrueba({ id: 'a' })
      const canalB = canalDePrueba({ id: 'b' })
      const { container, rerender } = render(Reproductor, {
        props: { canal: canalA, fuente: fuenteDePrueba({ destino }) },
      })
      await tick()

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()
      expect(video.src).toContain('a.m3u8')

      await rerender({ canal: canalB, fuente: fuenteDePrueba({ destino }) })
      await tick()

      // El $effect existente de cambio de canal (canal.id) llama
      // limpiarIntento()+reproducir() igual que siempre — motorForzado NO se
      // tocó, así que el canal nuevo también arranca en motor nativo, sin
      // que el test dispare el evento de WebKit de nuevo.
      expect(video.src).toContain('b.m3u8')
      expect(hlsState.instancias.length).toBe(0)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
    }
  })
```

- [ ] **Step 7: Test — 'm' no hace nada mientras se emite**

```typescript
  it("'m' no silencia mientras hay una sesión de AirPlay activa", async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    try {
      const destino = vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null }))
      const { container } = render(Reproductor, {
        props: { canal: canalDePrueba(), fuente: fuenteDePrueba({ destino }) },
      })
      await tick()

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()

      const mutedAntes = video.muted
      await window.dispatchEvent(new KeyboardEvent('keydown', { key: 'm' }))
      await tick()
      expect(video.muted).toBe(mutedAntes)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
    }
  })
```

- [ ] **Step 8: Verificar todos los tests del fichero**

Run: `cd web && npx vitest run src/componentes/Reproductor.test.ts`
Expected: PASS, todos — preexistentes y nuevos.

- [ ] **Step 9: Gates completos**

Run: `cd web && npm run check && npm test && npm run build`
Expected: todo en verde.

- [ ] **Step 10: Verificación visual mínima en Chrome (no automatizable)**

Seguir el flujo de desarrollo de CLAUDE.md: `cd web && npm run build`, luego
`go build -o open-tv ./cmd/open-tv` desde la raíz, restaurar
`internal/ui/dist/.gitkeep` si el build lo borró, matar el proceso
`open-tv serve` (launchd lo respawnea), y comprobar en Chrome con
cache-buster (`?fresh=...`) que:
- El panel de cast reemplaza la vista de vídeo cuando `estadoCast` se fuerza
  a mano desde la consola del navegador (sin AirPlay real disponible en
  Chrome, esto solo confirma layout/CSS, no la sesión real — eso es la
  Task 5).
- El botón 📺 no aparece en absoluto en Chrome (correcto:
  `soportaAirplay` es `false` ahí).

- [ ] **Step 11: Commit**

```bash
git add web/src/componentes/Reproductor.svelte web/src/componentes/Reproductor.test.ts
git commit -m "feat(web): interfaz de la sesión de cast AirPlay

Panel que reemplaza la vista de vídeo mientras se emite, botón AirPlay
ámbar cuando está activo (reusa la regla de marca existente, sin CSS
nuevo), 'm'/'f' deshabilitados durante el cast, avisos reusando las dos
regiones aria-live ya existentes — cero regiones nuevas.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 5: Verificación manual contra hardware real (obligatoria, no automatizable)

**Files:** ninguno — checklist puro, mismo motivo que la pasada de Safari
real ya documentada en memoria de proyecto (no automatizable en CI, jsdom no
decodifica HLS ni simula un Apple TV).

- [ ] **Step 1:** En Safari real, abrir Open TV, elegir un canal que ya se
  sabe bueno (el mismo del spike de la conversación de diseño). Pulsar 📺,
  elegir el TV. Confirmar: vídeo y audio limpios en el TV, sin glitch —
  mismo resultado que el spike de consola ya confirmó, ahora sin parche
  manual.

- [ ] **Step 2:** Repetir con un canal que se sepa que necesita el proxy
  (mixed-content o `webOk === false` en el catálogo). Confirmar que el
  reintento directo→proxy funciona de punta a punta (puede tardar unos
  segundos más que el directo).

- [ ] **Step 3:** Repetir con un canal de códec no soportado nativamente
  (buscar uno que ya falle con `motor === 'nativo'` en Safari antiguo, o
  forzar uno con el mismo parche de consola del spike). Confirmar: cae a
  reproducción local hls.js sin dejar el TV con pantalla negra colgada, y el
  aviso "este canal no se puede emitir por AirPlay" es legible (inspeccionar
  con VoiceOver o el DOM, dado que la región es `sr-only`).

- [ ] **Step 4:** Cambiar de canal (flechas) en plena emisión. Confirmar:
  mismo TV, sin volver a abrir el selector nativo.

- [ ] **Step 5:** Apagar el TV a mitad de emisión. Confirmar: la sesión cae a
  `idle`, vuelve la vista de vídeo local.

- [ ] **Step 6:** Pulsar 'm' durante la emisión. Confirmar: no pasa nada
  (ni local ni en el TV).

- [ ] **Step 7:** Volver a castear el canal marcado como fallo de formato en
  el Step 3. Confirmar: falla rápido (sin esperar el presupuesto de
  reintento completo) — comprobable con el Network/timing del propio
  navegador, o simplemente notando que el aviso aparece antes que la
  primera vez.

- [ ] **Step 8:** Reportar los resultados. Si algún paso falla, NO se
  considera esta feature terminada — volver a Task 3/4 según corresponda,
  no parchear a mano sobre el plan.
