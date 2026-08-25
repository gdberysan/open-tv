import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { tick } from 'svelte'
import App from '../App.svelte'
import Sincronizando from '../componentes/Sincronizando.svelte'
import MensajeError from '../componentes/MensajeError.svelte'
import RejillaVirtual from '../componentes/RejillaVirtual.svelte'
import RejillaCanales from '../componentes/RejillaCanales.svelte'
import Reproductor from '../componentes/Reproductor.svelte'
import { filtros } from '../estado/filtros'
import { favoritos } from '../estado/favoritos'
import { t } from '../i18n'
import type { Canal, CatalogSource, PaginaCanales } from '../datos/catalogo'

// Tarea 18 — auditoría de accesibilidad. Estas aserciones son las que
// jsdom + testing-library pueden verificar de verdad (rol, nombre accesible,
// aria-live, tabindex, foco); VoiceOver de verdad es el gate manual del
// autor (ver task-18-brief.md, paso 5 — no se ejecuta aquí).

// App.svelte llama a consultarSalud() (fetch a /health) en onMount; se
// sustituye por una resolución inmediata a "ya sincronizado", igual que
// App.integracion.test.ts — sin esto la App se queda en la fase
// 'comprobando' para siempre y nunca llega a montar la rejilla.
vi.mock('../estado/salud', async (importOriginal) => {
  const real = await importOriginal<typeof import('../estado/salud')>()
  return {
    ...real,
    consultarSalud: vi.fn(async () => ({
      sincronizando: false,
      proxyDisponible: false,
      ultimoSync: new Date('2026-01-01T00:00:00Z'),
      version: 'test',
    })),
  }
})

// Mismo doble que RejillaVirtual.test.ts / App.integracion.test.ts: jsdom no
// implementa IntersectionObserver, y tanto RejillaVirtual como RejillaCanales
// lo usan para el scroll infinito.
class FalsoIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return []
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalFalso(i: number): Canal {
  return {
    id: String(i),
    nombre: `Canal ${i}`,
    logoUrl: '',
    categoriaId: 'General',
    idioma: 'en',
    pais: 'GB',
    vivo: true,
    latenciaMs: 100,
    webOk: true,
  }
}

// Igual que RejillaVirtual.test.ts: el cálculo de la ventana virtualizada se
// agenda con requestAnimationFrame (coalescencia por rAF); hay que esperarlo
// explícitamente antes de esperar también un tick de Svelte para que el
// estado se propague al DOM.
async function asentar() {
  await new Promise<number>((resolve) => requestAnimationFrame(resolve))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function fuenteFalsa(overrides: Partial<CatalogSource> = {}): CatalogSource {
  return {
    canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 })),
    paises: vi.fn(async () => []),
    categorias: vi.fn(async () => []),
    aleatorio: vi.fn(async () => canalFalso(0)),
    destino: vi.fn(async () => ({ url: '', airplayOk: null })),
    mirrors: vi.fn(async () => []),
    frescura: vi.fn(async () => ({ tipo: 'vivo' as const, generadoEn: null })),
    proxyDisponible: vi.fn(async () => false),
    ...overrides,
  }
}

beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
  favoritos.set(new Set())
})

describe('a11y — estados con aria-live', () => {
  it('Sincronizando anuncia con aria-live="polite" (role=status no basta para la aserción)', () => {
    const { container } = render(Sincronizando, { alListo: () => {} })
    const region = container.querySelector('[role="status"]')
    expect(region).not.toBeNull()
    // Antes del arreglo, role="status" estaba solo (sin el atributo
    // aria-live explícito): esta aserción fallaba porque getAttribute
    // devolvía null, no 'polite'.
    expect(region?.getAttribute('aria-live')).toBe('polite')
  })

  it('MensajeError anuncia con aria-live="assertive"', () => {
    const { container } = render(MensajeError, { clase: 'gateway' })
    const region = container.querySelector('[role="alert"]')
    expect(region).not.toBeNull()
    expect(region?.getAttribute('aria-live')).toBe('assertive')
  })
})

describe('a11y — rejilla y lista exponen role=list/listitem', () => {
  it('RejillaVirtual: el contenedor es role=list y cada tarjeta es role=listitem', async () => {
    const canales = Array.from({ length: 4 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const lista = container.querySelector('.rejilla-virtual')
    expect(lista?.getAttribute('role')).toBe('list')
    const items = [...container.querySelectorAll('article')]
    expect(items.length).toBeGreaterThan(0)
    for (const item of items) expect(item.getAttribute('role')).toBe('listitem')
  })

  it('RejillaCanales en modo lista: el contenedor es role=list y cada fila es role=listitem', () => {
    const canales = [canalFalso(0), canalFalso(1)]
    const { container } = render(RejillaCanales, {
      canales, vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {},
    })
    const lista = container.querySelector('.canales.lista')
    expect(lista?.getAttribute('role')).toBe('list')
    const filas = [...container.querySelectorAll('article.fila')]
    expect(filas.length).toBe(2)
    for (const fila of filas) expect(fila.getAttribute('role')).toBe('listitem')
  })
})

describe('a11y — roving tabindex en la rejilla virtualizada', () => {
  it('solo la tarjeta activa tiene tabindex=0; ArrowRight mueve el foco y el tabindex a la siguiente', async () => {
    const canales = Array.from({ length: 5 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const botones = () => [...container.querySelectorAll<HTMLElement>('article .abrir')]
    // Antes del arreglo ningún botón tenía atributo tabindex: getAttribute
    // devolvía null en las dos comprobaciones siguientes, no '0'/'-1'.
    expect(botones()[0].getAttribute('tabindex')).toBe('0')
    expect(botones()[1].getAttribute('tabindex')).toBe('-1')

    botones()[0].focus()
    await fireEvent.keyDown(botones()[0], { key: 'ArrowRight' })
    await asentar()

    expect(document.activeElement).toBe(botones()[1])
    expect(botones()[0].getAttribute('tabindex')).toBe('-1')
    expect(botones()[1].getAttribute('tabindex')).toBe('0')
  })

  it('End mueve el foco a un canal que NO estaba montado (sobrevive a la ventana virtualizada)', async () => {
    // 300 canales fuerza una ventana real (con 5 no se llega a virtualizar
    // de verdad: el buffer de filas los cubre todos desde el principio,
    // como ya prueba RejillaVirtual.test.ts). El índice 299 no existe en el
    // DOM al montar — es justo el caso que puede perder el foco si el nodo
    // se desmonta/no existe todavía.
    const canales = Array.from({ length: 300 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    expect(container.querySelector('[data-indice="299"]')).toBeNull() // todavía no montado

    const primerBoton = container.querySelectorAll<HTMLElement>('article .abrir')[0]
    primerBoton.focus()
    await fireEvent.keyDown(primerBoton, { key: 'End' })
    await asentar()

    const ultimo = container.querySelector<HTMLElement>('[data-indice="299"] .abrir')
    expect(ultimo).not.toBeNull() // enfocarIndice lo montó a propósito
    expect(document.activeElement).toBe(ultimo)
    expect(ultimo?.getAttribute('tabindex')).toBe('0')
    // La tarjeta 0 ya no está en el DOM (la ventana se movió, no se
    // "ensanchó" hasta cubrir las 300 filas de por medio).
    expect(container.querySelector('[data-indice="0"]')).toBeNull()
  })

  it('Enter/clic activa la tarjeta enfocada mediante la semántica nativa del <button> (no reimplementada)', async () => {
    const alAbrir = vi.fn()
    const canales = Array.from({ length: 3 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir, alPedirMas: () => {} })
    await asentar()

    const boton = container.querySelector<HTMLElement>('article .abrir')!
    await fireEvent.click(boton)
    expect(alAbrir).toHaveBeenCalledOnce()
  })
})

describe('a11y — nombres accesibles', () => {
  it('el botón que abre el canal tiene el nombre del canal como nombre accesible', async () => {
    const canales = [canalFalso(7)]
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    expect(screen.getByRole('button', { name: 'Canal 7' })).toBe(container.querySelector('.abrir'))
  })

  it('el botón de favorito anuncia añadir/quitar y expone su estado con aria-pressed', async () => {
    const canales = [canalFalso(0)]
    render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const favorito = screen.getByRole('button', { name: t('canal.favorito.anadir') })
    expect(favorito.getAttribute('aria-pressed')).toBe('false')
  })
})

describe('a11y — reproductor: controles etiquetados, aria-live y foco', () => {
  const canal: Canal = { ...canalFalso(0), nombre: 'Canal de prueba' }

  it('todos los botones de control tienen nombre accesible', () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    const botones = screen.getAllByRole('button')
    expect(botones.length).toBeGreaterThan(0)
    for (const boton of botones) {
      const nombre = boton.getAttribute('aria-label') || boton.textContent?.trim()
      expect(nombre).toBeTruthy()
    }
  })

  it('el diálogo es role=dialog, aria-modal y su nombre accesible es el del canal', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    const dialogo = container.querySelector('[role="dialog"]')
    expect(dialogo?.getAttribute('aria-modal')).toBe('true')
    expect(dialogo?.getAttribute('aria-label')).toBe('Canal de prueba')
  })

  it('el estado "cargando" es aria-live="polite"', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    const estado = container.querySelector('.estado')
    expect(estado?.textContent).toBe(t('reproductor.cargando'))
    // Antes del arreglo, <p class="estado"> no llevaba aria-live: un fallo
    // de carga silencioso para quien usa lector de pantalla.
    expect(estado?.getAttribute('aria-live')).toBe('polite')
  })

  it('al montar, el foco entra en el reproductor en vez de quedarse fuera', async () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    await tick()
    await tick() // el enfoque real ocurre en un .then() encadenado tras tick(), dentro de onMount

    // Antes del arreglo, document.activeElement se quedaba en <body> (o en
    // lo que tuviera el foco antes de abrir el reproductor): esta
    // comprobación fallaba porque closest('.reproductor') daba null.
    expect(document.activeElement?.closest('.reproductor')).not.toBeNull()
  })

  it('Tab en el último control cicla de vuelta al primero (foco atrapado dentro del diálogo)', async () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    await tick()

    const controles = [...document.querySelectorAll<HTMLElement>('.controles button')]
    expect(controles.length).toBeGreaterThan(1)
    controles.at(-1)!.focus()
    await fireEvent.keyDown(controles.at(-1)!, { key: 'Tab' })

    expect(document.activeElement).toBe(controles[0])
  })

  it('al cerrarse (desmontar), el foco vuelve a quien lo abrió', async () => {
    document.body.innerHTML = '<button id="disparador">abrir</button>'
    const disparador = document.getElementById('disparador') as HTMLButtonElement
    disparador.focus()
    expect(document.activeElement).toBe(disparador)

    const { unmount } = render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    await tick()
    await tick()
    expect(document.activeElement).not.toBe(disparador) // el foco entró al reproductor al abrir

    unmount()
    expect(document.activeElement).toBe(disparador)
  })
})

describe('a11y — App: el fondo queda inert mientras el reproductor está abierto', () => {
  // jsdom (a fecha de esta tarea) no implementa la propiedad IDL `inert` del
  // estándar HTML — 'inert' in document.createElement('div') da false — así
  // que Svelte no puede pasar por el mismo camino de reflejo attr↔propiedad
  // que un navegador real. Lo que SÍ hace de forma fiable (comprobado a
  // mano) es asignar la propiedad JS `element.inert = valor`, que jsdom
  // conserva como una propiedad normal aunque no la reflecte a un atributo
  // del DOM. Por eso la aserción lee `.inert` (la propiedad), no
  // `hasAttribute('inert')` (que en jsdom se queda en null pasara lo que
  // pasara, y habría dado un falso verde/rojo sin relación con el fix). En
  // un navegador real ambos caminos —propiedad y atributo— están enlazados.
  it('<main>/<footer> no son inert sin reproductor, y sí en cuanto se abre uno', async () => {
    const canales = [canalFalso(0)]
    const fuente = fuenteFalsa({ canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales, total: 1 })) })
    const { container } = render(App, { fuente })

    const abrirCanal = await screen.findByLabelText('Canal 0')
    const main = container.querySelector('main') as (HTMLElement & { inert?: boolean }) | null
    const pie = container.querySelector('footer.pie') as (HTMLElement & { inert?: boolean }) | null
    // Antes del arreglo, <main>/<footer> no tenían el atributo inert={...}
    // en absoluto: esta propiedad era simplemente undefined, no false.
    expect(main?.inert).toBe(false)
    expect(pie?.inert).toBe(false)

    await fireEvent.click(abrirCanal)
    await tick()

    expect(main?.inert).toBe(true)
    expect(pie?.inert).toBe(true)
  })
})
