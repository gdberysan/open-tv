import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import App from '../App.svelte'
import BarraLateralFacetas from '../componentes/BarraLateralFacetas.svelte'
import BarraAcciones from '../componentes/BarraAcciones.svelte'
import ChipsFiltro from '../componentes/ChipsFiltro.svelte'
import ContinuarViendo from '../componentes/ContinuarViendo.svelte'
import IndicadorSenal from '../componentes/IndicadorSenal.svelte'
import { filtros } from '../estado/filtros'
import { favoritos } from '../estado/favoritos'
import { historial } from '../estado/historial'
import { consultarSalud } from '../estado/salud'
import { idioma, t } from '../i18n'
import type { Canal, CatalogSource, PaginaCanales } from '../datos/catalogo'

// Tarea 15 (P0.6) — pase de accesibilidad del shell "Sala de control". Estas
// aserciones son las que jsdom + testing-library pueden verificar de verdad
// (nombre accesible, aria-expanded/aria-label, texto de estado, la región
// aria-live que anuncia); VoiceOver de verdad es el gate manual del autor
// (task-15-brief.md, paso 5 — no se ejecuta aquí). Mismo patrón que
// lib/a11y.test.ts, que audita el shell anterior a P0.6.

// App.svelte llama a consultarSalud() (fetch a /health) en onMount; se
// sustituye por una resolución inmediata a "ya sincronizado", igual que
// lib/a11y.test.ts — sin esto la App se queda en la fase 'comprobando' para
// siempre.
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

// jsdom no implementa IntersectionObserver; RejillaCanales/RejillaVirtual lo
// usan para el scroll infinito (mismo doble que lib/a11y.test.ts).
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

// fuentes() por defecto devuelve UNA fuente ya sincronizada (Tarea 6, P0.7):
// estos tests ejercitan el catálogo vacío por FILTRO/sin resultados (mismo
// camino de siempre), no el onboarding "sin fuentes" (App.integracion.test.ts
// cubre ese) — con fuentes() vacío por defecto, App mostraría Onboarding en
// vez del shell normal y estos tests dejarían de comprobar lo que dicen
// comprobar.
function fuenteFalsa(overrides: Partial<CatalogSource> = {}): CatalogSource {
  return {
    canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 })),
    paises: vi.fn(async () => []),
    categorias: vi.fn(async () => []),
    calidades: vi.fn(async () => []),
    aleatorio: vi.fn(async () => canalFalso(0)),
    destino: vi.fn(async () => ({ url: '', airplayOk: null })),
    mirrors: vi.fn(async () => []),
    frescura: vi.fn(async () => ({ tipo: 'vivo' as const, generadoEn: null })),
    proxyDisponible: vi.fn(async () => false),
    fuentes: vi.fn(async () => [
      { id: 'f0', label: 'Fuente de prueba', url: 'https://ej.test/f0.m3u', kind: 'url' as const, ultimoSync: Date.now(), canales: 1 },
    ]),
    anadirFuente: vi.fn(async () => ({
      id: 'f1', label: 'Fuente falsa', url: '', kind: 'url' as const, ultimoSync: null, canales: 0,
    })),
    anadirFuenteFichero: vi.fn(async () => ({
      id: 'f1', label: 'Fuente falsa', url: '', kind: 'file' as const, ultimoSync: null, canales: 0,
    })),
    quitarFuente: vi.fn(async () => {}),
    resyncFuente: vi.fn(async () => {}),
    fuentesSugeridas: vi.fn(async () => []),
    ...overrides,
  }
}

// Fix round 1 (Tarea 15): App.svelte lee innerWidth vía <svelte:window
// bind:innerWidth> (mismo patrón que RejillaVirtual.svelte) — jsdom expone
// `window.innerWidth` como una propiedad normal (no de solo lectura como en
// un navegador real), así que asignarla y disparar 'resize' a mano basta
// para que el binding de Svelte la recoja, sin necesidad de mockear
// matchMedia.
function fijarAnchoVentana(px: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: px })
  window.dispatchEvent(new Event('resize'))
}

const ANCHO_ESCRITORIO_JSDOM = 1024 // valor por defecto de jsdom; > 900 ⇒ no estrecho

beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
  favoritos.set(new Set())
  historial.borrar()
  idioma.actual = 'es'
  fijarAnchoVentana(ANCHO_ESCRITORIO_JSDOM) // cada test arranca en ancho, salvo que diga lo contrario
  vi.mocked(consultarSalud).mockReset()
  vi.mocked(consultarSalud).mockImplementation(async () => ({
    sincronizando: false,
    proxyDisponible: false,
    ultimoSync: new Date('2026-01-01T00:00:00Z'),
    version: 'test',
  }))
})

describe('a11y P0.6 — nombre accesible en todo control interactivo del shell nuevo', () => {
  it('BarraLateralFacetas: filas de facetas, "solo señal viva" y el checkbox de offline tienen todos nombre accesible', () => {
    const paises = [{ valor: 'ES', total: 12 }]
    const categorias = [{ valor: 'Deportes', total: 3 }]
    const calidades = [{ valor: 'hd', total: 20 }]
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const botones = screen.getAllByRole('button')
    expect(botones.length).toBeGreaterThan(0)
    for (const boton of botones) {
      const nombre = boton.getAttribute('aria-label') || boton.textContent?.trim()
      expect(nombre).toBeTruthy()
    }
    // El buscador es un <input>, no un <button>: su nombre accesible sale
    // de aria-label, comprobado aparte.
    expect(screen.getByLabelText(t('catalogo.buscar'))).toBeTruthy()
    expect(screen.getByLabelText(t('filtro.mostrarOffline'))).toBeTruthy()
  })

  it('BarraAcciones: favoritos/aleatorio/rejilla/lista tienen todos nombre accesible', () => {
    render(BarraAcciones, { total: 42, frescura: null, alAleatorio: () => {} })
    const botones = screen.getAllByRole('button')
    expect(botones.length).toBeGreaterThan(0)
    for (const boton of botones) {
      const nombre = boton.getAttribute('aria-label') || boton.textContent?.trim()
      expect(nombre).toBeTruthy()
    }
  })

  it('ContinuarViendo: la fila principal, las miniaturas y "borrar" tienen todos nombre accesible', () => {
    historial.registrar(canalFalso(0))
    historial.registrar(canalFalso(1))
    historial.registrar(canalFalso(2))
    render(ContinuarViendo, { alAbrir: () => {} })

    const botones = screen.getAllByRole('button')
    expect(botones.length).toBeGreaterThan(0)
    for (const boton of botones) {
      const nombre = boton.getAttribute('aria-label') || boton.textContent?.trim()
      expect(nombre).toBeTruthy()
    }
  })
})

describe('a11y P0.6 — chips de filtro llevan aria-label (no solo el texto visible del valor)', () => {
  it('cada chip expone aria-label describiendo qué filtro quita, no solo la etiqueta del valor', () => {
    filtros.update((f) => ({ ...f, pais: 'México', calidad: '4k' }))
    render(ChipsFiltro)

    const chips = screen.getAllByRole('button')
    expect(chips.length).toBe(2)
    for (const chip of chips) {
      const etiqueta = chip.getAttribute('aria-label')
      expect(etiqueta).toBeTruthy()
      // Falsable: un chip que solo llevara el valor como texto visible sin
      // aria-label seguiría teniendo NOMBRE accesible (el textContent), pero
      // esta aserción exige el ATRIBUTO aria-label en concreto — el criterio
      // explícito del brief ("los chips tienen aria-label").
      expect(etiqueta!.length).toBeGreaterThan(0)
    }
  })
})

describe('a11y P0.6 — el botón del cajón responsive expone aria-controls/aria-expanded', () => {
  it('aria-controls apunta al panel real; aria-expanded refleja el estado y cambia al alternarlo', async () => {
    const { container } = render(App, { fuente: fuenteFalsa() })

    const boton = container.querySelector('button.boton-cajon') as HTMLElement
    expect(boton).not.toBeNull()
    expect(boton.getAttribute('aria-controls')).toBe('panel-facetas')
    expect(container.querySelector('#panel-facetas')).not.toBeNull()

    // Estado inicial: abierto (lateralAbierto arranca en true).
    expect(boton.getAttribute('aria-expanded')).toBe('true')

    await fireEvent.click(boton)
    expect(boton.getAttribute('aria-expanded')).toBe('false')

    await fireEvent.click(boton)
    expect(boton.getAttribute('aria-expanded')).toBe('true')
  })
})

describe('a11y P0.6 — IndicadorSenal comunica el estado por texto, no solo por color', () => {
  it('los tres estados producen tres textos distintos', () => {
    const textos = (['vivo', 'sincronizando', 'sin-gateway'] as const).map((estado) => {
      const { container, unmount } = render(IndicadorSenal, { estado })
      const texto = container.querySelector('.texto')?.textContent
      unmount()
      return texto
    })
    expect(textos.every((texto) => !!texto)).toBe(true)
    expect(new Set(textos).size).toBe(3)
  })
})

describe('a11y P0.6 — el vacío llega a la región polite persistente de App (hallazgo del ledger)', () => {
  // Falsable: contra el código PRE-fix (mensajeSincronizandoAccesible solo
  // cubría fase.tipo === 'sincronizando'), la región polite se quedaba
  // vacía ('') al llegar a cero canales — este test habría fallado ahí. El
  // fix (Tarea 15) extiende la MISMA cadena derivada para que también cubra
  // catalogoVacio, sin montar una segunda región en Vacio.svelte (eso
  // reabriría el bug del doble anuncio que el fix round 2 ya cerró).
  it('con el catálogo cargado y cero canales, la región aria-live="polite" persistente anuncia el mensaje de vacío', async () => {
    const fuente = fuenteFalsa({
      canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 })),
    })
    const { container } = render(App, { fuente })

    // Ya existe desde el montaje (persistente, fix round 1) — nunca se crea
    // un nodo nuevo para el vacío.
    const region = container.querySelector('[aria-live="polite"].sr-only')
    expect(region).not.toBeNull()

    await vi.waitFor(() => expect(region?.textContent).toBe(t('catalogo.vacio')))

    // El componente visual Vacio.svelte sigue sin su propia semántica live:
    // solo la región persistente de App anuncia. Si Vacio montara la SUYA,
    // habría dos nodos [aria-live] con el mismo texto.
    const conElTexto = [...container.querySelectorAll('[aria-live]')].filter(
      (el) => el.textContent === t('catalogo.vacio'),
    )
    expect(conElTexto.length).toBe(1)
  })

  it('con canales cargados, la región polite NO anuncia el mensaje de vacío', async () => {
    const canales = [canalFalso(0)]
    const fuente = fuenteFalsa({
      canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales, total: 1 })),
    })
    const { container } = render(App, { fuente })

    await screen.findByLabelText('Canal 0')

    const region = container.querySelector('[aria-live="polite"].sr-only')
    expect(region?.textContent).toBe('')
  })
})

describe('a11y P0.6 fix round 1 — el cajón colapsado en viewport estrecho queda inert (no alcanzable por Tab)', () => {
  // Hallazgo del informe original de la Tarea 15 (no arreglado entonces):
  // bajo el breakpoint de 900px, el cajón cerrado se saca de pantalla solo
  // con transform (translateX) — sin inert, sus controles seguían siendo
  // tab-ables estando invisibles. Falsable: contra el código de antes de
  // este fix (aside.facetas sin atributo inert en absoluto), las tres
  // aserciones de abajo que esperan la PRESENCIA de inert habrían fallado
  // (aside?.inert habría sido `undefined`, nunca `true`).
  function aside(container: HTMLElement) {
    return container.querySelector('aside.facetas') as (HTMLElement & { inert?: boolean }) | null
  }

  it('viewport estrecho + cajón cerrado: el aside queda inert', async () => {
    fijarAnchoVentana(800)
    const { container } = render(App, { fuente: fuenteFalsa() })
    // lateralAbierto arranca en true (abierto) — se cierra con el mismo
    // botón que usaría cualquier persona en viewport estrecho.
    const boton = container.querySelector('button.boton-cajon') as HTMLElement
    await fireEvent.click(boton)
    await vi.waitFor(() => expect(aside(container)?.getAttribute('data-abierto')).toBe('false'))

    expect(aside(container)?.inert).toBe(true)
  })

  it('viewport estrecho + cajón ABIERTO: el aside NO es inert (se puede usar mientras está desplegado)', async () => {
    fijarAnchoVentana(800)
    const { container } = render(App, { fuente: fuenteFalsa() })
    // lateralAbierto arranca en true (ver App.svelte) — el efecto de
    // fijarAnchoVentana(800) ya corrió en beforeEach de este describe vía
    // el propio test, así que basta esperar a que esEstrecho se asiente.
    await vi.waitFor(() => expect(aside(container)?.getAttribute('data-abierto')).toBe('true'))
    expect(aside(container)?.inert).toBe(false)

    const boton = container.querySelector('button.boton-cajon') as HTMLElement
    await fireEvent.click(boton) // cierra
    await vi.waitFor(() => expect(aside(container)?.inert).toBe(true))

    await fireEvent.click(boton) // reabre — el botón vive FUERA del aside, nunca quedó inert
    await vi.waitFor(() => expect(aside(container)?.inert).toBe(false))
  })

  it('viewport ANCHO: el aside nunca es inert, ni siquiera con el cajón "cerrado" (ahí no es un cajón, es la columna fija)', async () => {
    const { container } = render(App, { fuente: fuenteFalsa() })
    const boton = container.querySelector('button.boton-cajon') as HTMLElement

    await fireEvent.click(boton) // lateralAbierto pasa a false
    await vi.waitFor(() => expect(aside(container)?.getAttribute('data-abierto')).toBe('false'))

    // Sigue en ancho (jsdom por defecto, 1024px > 900): esEstrecho es false,
    // así que el inert NUNCA se activa pase lo que pase lateralAbierto.
    expect(aside(container)?.inert).toBe(false)
  })
})
