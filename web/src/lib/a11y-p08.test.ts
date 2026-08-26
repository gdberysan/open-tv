import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/svelte'
import { tick, type ComponentProps } from 'svelte'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import Reproductor from '../componentes/Reproductor.svelte'
import Paleta from '../componentes/Paleta.svelte'
import type { Canal, Faceta, Mirror } from '../datos/catalogo'
import { favoritos } from '../estado/favoritos'
import { idioma, t } from '../i18n'

// Pase de a11y de P0.8 (Tarea 8): audita los componentes NUEVOS de este ciclo
// (overlay del Reproductor — Tareas 1/2 —, la Paleta ⌘K — Tareas 3/4 —) con
// aserciones FALSABLES — cada una se comprobó fallando contra una versión sin
// el guard/atributo correspondiente antes de escribirse así. Ajustes.svelte
// (Tarea 6) y las micro-interacciones (Tarea 7) ya tienen cobertura de este
// tipo en Ajustes.test.ts y estilos/movimiento.test.ts respectivamente — no
// se duplica aquí, solo se complementa donde de verdad faltaba algo (ningún
// hueco encontrado en Ajustes: los checkboxes ya usan <label> nativo, los
// botones de densidad ya llevan aria-pressed).

const canal = { id: 'c1', nombre: 'X', webOk: true } as Canal

function fuenteSinMirrors() {
  return {
    mirrors: vi.fn(async () => [] as Mirror[]),
    destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
    proxyDisponible: vi.fn(async () => false),
  }
}

beforeEach(() => {
  favoritos.set(new Set())
  idioma.actual = 'es'
})

describe('a11y P0.8 — overlay del Reproductor: focus-trap completo', () => {
  // Gap real (sin cobertura previa): NINGÚN test existente ejercitaba el
  // ciclo de Tab del Reproductor con los controles NUEVOS del overlay
  // (silenciar/favorito/pantalla-completa/PiP) de por medio — solo se
  // documenta en un comentario de Reproductor.svelte. Si alTeclado/
  // elementosFocables dejaran fuera alguno de estos botones (p. ej. por vivir
  // dentro de .overlay en vez de .controles), Tab desde Cerrar NO envolvería
  // al primero de ellos y este test fallaría.
  it('Tab desde "Cerrar" envuelve al primer control del overlay (Silenciar) — PiP incluido cuando está soportado', async () => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { value: true, configurable: true })
    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
      const dialogo = container.querySelector('[role="dialog"]') as HTMLElement
      const silenciar = dialogo.querySelector('.overlay-controles .silenciar') as HTMLButtonElement
      const pip = dialogo.querySelector('.overlay-controles .pip') as HTMLButtonElement
      const cerrar = dialogo.querySelector('.controles .cerrar') as HTMLButtonElement
      expect(pip).toBeTruthy() // si esto falla, el resto del test no dice nada del trap

      cerrar.focus()
      expect(document.activeElement).toBe(cerrar)
      // El handler real vive en <svelte:window onkeydown> (App.svelte usa el
      // mismo patrón) — se dispara sobre window, como el resto de los tests
      // de teclado de Reproductor.test.ts (Escape, etc.), no sobre un
      // elemento interno.
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))

      expect(document.activeElement).toBe(silenciar)
    } finally {
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.pictureInPictureEnabled
    }
  })

  it('Shift+Tab desde "Silenciar" (el primero) envuelve al último control ("Cerrar")', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
    const dialogo = container.querySelector('[role="dialog"]') as HTMLElement
    const silenciar = dialogo.querySelector('.overlay-controles .silenciar') as HTMLButtonElement
    const cerrar = dialogo.querySelector('.controles .cerrar') as HTMLButtonElement

    silenciar.focus()
    expect(document.activeElement).toBe(silenciar)
    await fireEvent.keyDown(dialogo, { key: 'Tab', shiftKey: true })

    expect(document.activeElement).toBe(cerrar)
  })

  // Todos los controles de la barra fija ("Cerrar") también viven DENTRO de
  // contenedorDialogo — si "Cerrar" se hubiera puesto fuera del diálogo (un
  // error de refactor plausible: es el único control que NO está dentro de
  // .overlay), elementosFocables() no lo encontraría y este test fallaría.
  it('"Cerrar" está DENTRO del contenedor role=dialog, no fuera de él', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
    const dialogo = container.querySelector('[role="dialog"]') as HTMLElement
    const cerrar = screen.getByRole('button', { name: t('reproductor.cerrar') })
    expect(dialogo.contains(cerrar)).toBe(true)
  })
})

describe('a11y P0.8 — overlay del Reproductor: el guard focus-within impide auto-ocultarse con el foco dentro', () => {
  // Gap real: el guard de ocultarSiProcede (overlayEl.contains(activeElement))
  // solo se ejercitaba leyendo el código fuente, nunca disparando el
  // temporizador de verdad. vi.advanceTimersByTimeAsync dispara el
  // setTimeout(…, 3000) real del componente; sin el guard, este test fallaría
  // (el overlay se ocultaría igual con el foco dentro).
  //
  // El auto-ocultar solo se arma tras CONFIRMAR reproducción (cargando pasa a
  // false — ver el $effect de Reproductor.svelte): el guard interno
  // (PlaybackGuard.alPosicion) exige DOS timeupdate con una posición distinta
  // para llamar a alConfirmar(); jsdom nunca decodifica de verdad, así que se
  // fuerza el motor nativo (canPlayType) y se disparan esos dos timeupdate a
  // mano — el mismo patrón que el test "el camino nativo llama a load() y
  // play()" de Reproductor.test.ts ya usa para lo mismo.
  it('con el foco en un control del overlay, pasar de sobra los 3s programados NO lo oculta — y SIN foco, sí', async () => {
    const canPlayTypeSpy = vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockReturnValue('maybe')
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.useFakeTimers()
    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
      const video = container.querySelector('video') as HTMLVideoElement
      const overlay = container.querySelector('.overlay') as HTMLElement

      // Deja resolver destino()/proxyDisponible() (microtasks reales, no
      // temporizadores) y confirma reproducción con dos posiciones distintas.
      await vi.advanceTimersByTimeAsync(0)
      video.currentTime = 0
      video.dispatchEvent(new Event('timeupdate'))
      video.currentTime = 1
      video.dispatchEvent(new Event('timeupdate'))
      await vi.advanceTimersByTimeAsync(0)

      expect(overlay.classList.contains('oculto')).toBe(false) // confirmado: mostrar() ya se llamó, arrancando el ciclo de auto-ocultar

      const boton = container.querySelector('.overlay-controles .silenciar') as HTMLButtonElement
      boton.focus() // dispara focusin → mostrar() → reprograma el temporizador (ya bajo el reloj falso)

      await vi.advanceTimersByTimeAsync(5000)
      expect(overlay.classList.contains('oculto')).toBe(false) // el guard reprogramó en vez de ocultar

      boton.blur()
      await vi.advanceTimersByTimeAsync(5000)
      expect(overlay.classList.contains('oculto')).toBe(true) // sin foco dentro, sí se oculta
    } finally {
      vi.useRealTimers()
      canPlayTypeSpy.mockRestore()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })
})

describe('a11y P0.8 — overlay del Reproductor: estado por texto/aria, nunca solo color', () => {
  // "insignia-vivo" (el punto rojo) va SIEMPRE acompañada del texto
  // t('reproductor.envivo') — si algún día se quitara el texto y se dejara
  // solo el punto de color, este test lo cazaría.
  it('la insignia "en vivo" lleva el punto de color Y el texto, no solo el punto', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
    const insignia = container.querySelector('.insignia-vivo') as HTMLElement
    const punto = insignia.querySelector('.punto-vivo')
    expect(punto).toBeTruthy()
    expect(punto?.getAttribute('aria-hidden')).toBe('true') // decorativo: el texto es lo que se anuncia
    expect(insignia.textContent?.trim().length).toBeGreaterThan(0)
  })

  // Barrido estructural sobre el FUENTE de Reproductor.svelte (mismo método
  // que estilos/movimiento.test.ts): todo botón con `class:activo={x}` en el
  // marcado del overlay debe llevar TAMBIÉN `aria-pressed={x}` — si un botón
  // nuevo se añadiera con solo la clase visual (color) y sin el atributo
  // aria, este test fallaría al no encontrar aria-pressed en el mismo bloque.
  it('todo botón del overlay con class:activo lleva también aria-pressed (nunca solo color)', () => {
    const aqui = dirname(fileURLToPath(import.meta.url))
    const fuente = readFileSync(resolve(aqui, '../componentes/Reproductor.svelte'), 'utf-8')
    const bloqueOverlay = fuente.slice(fuente.indexOf('<div class="overlay-controles">'), fuente.indexOf('</div>\n    </div>\n  </div>\n\n  <div class="controles">'))
    const botones = [...bloqueOverlay.matchAll(/<button[\s\S]*?<\/button>/g)].map((m) => m[0])
    expect(botones.length).toBeGreaterThan(0)
    for (const boton of botones) {
      if (boton.includes('class:activo')) {
        expect(boton, `botón sin aria-pressed junto a class:activo: ${boton.slice(0, 80)}…`).toMatch(/aria-pressed=/)
      }
    }
  })
})

// Paleta ⌘K (Tareas 3/4): combobox — dialog/combobox/listbox/option, nombre
// accesible por opción. aria-activedescendant siguiendo la opción activa y el
// foco atrapado/restaurado ya tienen cobertura completa en Paleta.test.ts
// (describe "apertura y estructura", "(d) Esc cierra y restaura el foco") —
// aquí se complementa con lo que faltaba: el nombre accesible de CADA opción
// (no solo que el grupo se pinte) y que los tres roles del patrón combobox
// coexistan en el MISMO montaje (no solo por separado en tests distintos).
function canalDePrueba(id: string, nombre: string): Canal {
  return { id, nombre, logoUrl: '', categoriaId: '', idioma: 'es', pais: '', vivo: null, latenciaMs: 0, webOk: null }
}
function faceta(valor: string, total = 1): Faceta {
  return { valor, total }
}
function propsPaleta(overrides: Partial<ComponentProps<typeof Paleta>> = {}): ComponentProps<typeof Paleta> {
  return {
    canales: [] as Canal[],
    paises: [] as Faceta[],
    categorias: [] as Faceta[],
    calidades: [] as Faceta[],
    alCerrar: vi.fn(),
    alAbrirCanal: vi.fn(),
    alAleatorio: vi.fn(),
    alAbrirFuentes: vi.fn(),
    alAbrirStats: vi.fn(),
    ...overrides,
  }
}

describe('a11y P0.8 — Paleta: patrón combobox completo, con nombre accesible por opción', () => {
  it('dialog + combobox + listbox coexisten en el mismo montaje, y CADA opción tiene un nombre accesible no vacío', async () => {
    render(Paleta, propsPaleta({
      canales: [canalDePrueba('1', 'CNN en Español'), canalDePrueba('2', 'BBC News')],
      paises: [faceta('MX')],
    }))
    await tick()

    expect(screen.getByRole('dialog', { name: t('paleta.titulo') })).toBeTruthy()
    expect(screen.getByRole('combobox')).toBeTruthy()
    expect(screen.getByRole('listbox')).toBeTruthy()

    const opciones = screen.getAllByRole('option')
    expect(opciones.length).toBeGreaterThan(0)
    for (const opcion of opciones) {
      // getAllByRole ya exige que cada nodo con role=option tenga un nombre
      // accesible calculable — pero se reafirma aquí que ese nombre no es
      // una cadena vacía/solo-espacios (un <div role="option"></div> sin
      // contenido pasaría el role pero no diría nada a un lector de
      // pantalla).
      const nombre = opcion.textContent?.trim() ?? ''
      expect(nombre.length, `opción sin nombre accesible: ${opcion.outerHTML.slice(0, 80)}`).toBeGreaterThan(0)
    }
  })

  it('la opción activa (aria-activedescendant) también lleva aria-selected=true — nunca solo el resaltado de color', async () => {
    render(Paleta, propsPaleta({ canales: [canalDePrueba('1', 'Canal Uno'), canalDePrueba('2', 'Canal Dos')] }))
    await tick()

    const entrada = screen.getByRole('combobox') as HTMLInputElement
    const idActivo = entrada.getAttribute('aria-activedescendant')
    expect(idActivo).toBeTruthy()
    const opcionActiva = document.getElementById(idActivo!)
    expect(opcionActiva?.getAttribute('aria-selected')).toBe('true')

    // Y la NO activa no debe decir lo mismo.
    const otras = screen.getAllByRole('option').filter((o) => o.id !== idActivo)
    expect(otras.length).toBeGreaterThan(0)
    for (const otra of otras) {
      expect(otra.getAttribute('aria-selected')).toBe('false')
    }
  })
})
