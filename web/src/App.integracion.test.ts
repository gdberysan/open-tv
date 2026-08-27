import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen, within } from '@testing-library/svelte'
import { tick } from 'svelte'
import { get } from 'svelte/store'
import App from './App.svelte'
import type { Canal, CatalogSource, ConsultaCatalogo, Fuente, PaginaCanales } from './datos/catalogo'
import { filtros } from './estado/filtros'
import { favoritos } from './estado/favoritos'
import { preferencias } from './estado/preferencias'
import { historial } from './estado/historial'
import { idioma, t } from './i18n'

// comprobarSalud() de App llama a consultarSalud(), que hace un fetch('/health')
// real. Estos tests no hablan con ningún servidor: se sustituye por una
// resolución inmediata a "ya sincronizado", igual que hace el gateway sano.
vi.mock('./estado/salud', async (importOriginal) => {
  const real = await importOriginal<typeof import('./estado/salud')>()
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

// RejillaCanales observa un centinela con IntersectionObserver para el scroll
// infinito; jsdom no lo implementa. Este doble deja disparar la intersección
// a mano desde el test, igual que haría el navegador al llegar al final.
class FalsoIntersectionObserver {
  static instancias: FalsoIntersectionObserver[] = []
  private cb: IntersectionObserverCallback
  constructor(cb: IntersectionObserverCallback) {
    this.cb = cb
    FalsoIntersectionObserver.instancias.push(this)
  }
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return []
  }
  dispararInterseccion() {
    this.cb([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalDePrueba(id: string): Canal {
  return {
    id,
    nombre: id,
    logoUrl: '',
    categoriaId: '',
    idioma: 'es',
    pais: '',
    vivo: null,
    latenciaMs: 0,
    webOk: null,
  }
}

function fuenteDePrueba(id: string): Fuente {
  return { id, label: `Fuente ${id}`, url: `https://ej.test/${id}.m3u`, kind: 'url', ultimoSync: Date.now(), canales: 1 }
}

/** CatalogSource falso en memoria: implementa TODOS los métodos de la
 * interfaz (incluido mirrors, de la Tarea 3) para que App pueda montarse
 * entero sin tocar la red.
 *
 * fuentes() por defecto devuelve UNA fuente ya sincronizada (Tarea 6, P0.7):
 * la inmensa mayoría de los tests de este fichero no le importa el onboarding
 * de fuentes, solo el comportamiento normal del catálogo — con fuentes()
 * vacío por defecto, esos tests (varios con canales:[] a propósito, para
 * ejercitar OTRO camino) habrían empezado a ver Onboarding en vez de
 * BarraAcciones/RejillaCanales sin venir a cuento. Los tests que SÍ quieren
 * ejercitar sinFuentes pasan `fuentes: vi.fn(async () => [])` explícito. */
function fuenteFalsa(overrides: Partial<CatalogSource> = {}): CatalogSource {
  const canales = vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 }))
  return {
    canales,
    paises: vi.fn(async () => []),
    categorias: vi.fn(async () => []),
    calidades: vi.fn(async () => []),
    aleatorio: vi.fn(async () => canalDePrueba('random')),
    destino: vi.fn(async () => ({ url: '', airplayOk: null })),
    mirrors: vi.fn(async () => []),
    frescura: vi.fn(async () => ({ tipo: 'vivo' as const, generadoEn: null })),
    proxyDisponible: vi.fn(async () => false),
    fuentes: vi.fn(async () => [fuenteDePrueba('f0')]),
    anadirFuente: vi.fn(async () => ({
      id: 'f1', label: 'Fuente falsa', url: '', kind: 'url' as const, ultimoSync: null, canales: 0,
    })),
    anadirFuenteFichero: vi.fn(async () => ({
      id: 'f1', label: 'Fuente falsa', url: '', kind: 'file' as const, ultimoSync: null, canales: 0,
    })),
    quitarFuente: vi.fn(async () => {}),
    resyncFuente: vi.fn(async () => {}),
    fuentesSugeridas: vi.fn(async () => []),
    epgDeCanales: vi.fn(async () => new Map()),
    epgDeCanal: vi.fn(async () => ({ ahora: null, proximos: [] })),
    ...overrides,
  }
}

beforeEach(() => {
  localStorage.clear()
  filtros.set({
    q: '',
    pais: '',
    categoria: '',
    calidad: '',
    mostrarOffline: false,
    soloFavoritos: false,
    vista: 'rejilla',
  })
  favoritos.set(new Set())
  // historial es un singleton de módulo (como favoritos): localStorage.clear()
  // no vacía el store ya vivo — sin borrar(), los canales abiertos en un test
  // reaparecen como miniaturas de «Continuar viendo» en el siguiente y
  // vuelven ambiguas las consultas por nombre de canal.
  historial.borrar()
  preferencias.set({ densidad: 'comoda', recordarVista: true, recordarFiltros: false })
  idioma.actual = 'es'
  FalsoIntersectionObserver.instancias.length = 0
})

// Reproductor-primero: helpers del escenario. El catálogo por defecto ES el
// escenario (vídeo persistente + lateral); la rejilla rica de P0.6 vive detrás
// de «Ver todo». esperarCatalogoListo() es el sentinela de "catálogo listo"
// (antes se usaba el botón «Canal al azar», que ahora solo existe en ver-todo).
async function esperarCatalogoListo() {
  await screen.findByRole('button', { name: t('escenario.verTodo') })
}

async function abrirVerTodo() {
  await esperarCatalogoListo()
  await fireEvent.click(screen.getByRole('button', { name: t('escenario.verTodo') }))
  await tick()
}

describe('App — escenario reproductor-primero', () => {
  function fuenteConCanales(n = 3) {
    const lista = Array.from({ length: n }, (_, i) => canalDePrueba(`c${i}`))
    return fuenteFalsa({
      canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales: lista, total: n })),
    })
  }

  // Consulta ACOTADA a la lista lateral: tras abrir un canal, «Continuar
  // viendo» también expone botones con el nombre del canal — sin el ámbito,
  // getByRole por nombre se vuelve ambiguo.
  async function filaLateral(nombre: string) {
    const lista = await screen.findByRole('list', { name: t('lateral.lista') })
    return within(lista).getByRole('button', { name: new RegExp(nombre) })
  }

  it('con catálogo y sin canal en curso, el panel de vídeo muestra «elige un canal» y la lateral lista los canales', async () => {
    render(App, { fuente: fuenteConCanales() })
    await vi.waitFor(() => expect(screen.getByText(t('escenario.eligeCanal'))).toBeTruthy())
    expect(await filaLateral('c1')).toBeTruthy()
    // No hay modal: nada con role=dialog, y el fondo NO está inert.
    // `.inert` (propiedad), no hasAttribute: Svelte asigna la propiedad y
    // jsdom no la refleja al atributo (misma nota que en a11y.test.ts).
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect((document.querySelector('.fondo') as HTMLElement & { inert?: boolean })?.inert).toBe(false)
  })

  it('clicar un canal en la lateral reproduce EN EL SITIO: aparece el vídeo, sin modal, y cambiar de canal NO desmonta el <video>', async () => {
    render(App, { fuente: fuenteConCanales() })
    await fireEvent.click(await filaLateral('c0'))
    await tick()
    const video = document.querySelector('video')
    expect(video).toBeTruthy()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    await fireEvent.click(await filaLateral('c1'))
    await tick()
    expect(document.querySelector('video')).toBe(video) // mismo nodo: swap in place
  })

  it('«Ver todo» abre la rejilla rica (P0.6 conservada), oculta el escenario SIN desmontar el vídeo, y el foco aterriza en «Volver»', async () => {
    render(App, { fuente: fuenteConCanales() })
    await fireEvent.click(await filaLateral('c0'))
    await tick()
    await abrirVerTodo()
    // La rejilla/acciones de siempre están de vuelta
    expect(screen.getByRole('button', { name: t('accion.favoritos') })).toBeTruthy()
    // El escenario sigue MONTADO (el video existe) pero oculto (hidden)
    expect(document.querySelector('.escenario[hidden] video')).toBeTruthy()
    await vi.waitFor(() => expect(document.activeElement?.textContent).toContain(t('escenario.volver')))
  })

  it('seleccionar un canal en ver-todo vuelve al escenario reproduciéndolo', async () => {
    render(App, { fuente: fuenteConCanales() })
    await abrirVerTodo()
    // La tarjeta c2 de la rejilla (markup de TarjetaCanal: botón con aria-label
    // del canal). findAll: la ventana virtualizada pinta sus filas tras un
    // requestAnimationFrame — la consulta tiene que esperar ese primer frame.
    await fireEvent.click((await screen.findAllByRole('button', { name: 'c2' }))[0])
    await tick()
    expect(document.querySelector('.escenario[hidden]')).toBeNull()
    expect(screen.getByRole('region', { name: 'c2' })).toBeTruthy()
  })

  it('la región polite anuncia «Reproduciendo <nombre>» al cambiar de canal (sin regiones live nuevas)', async () => {
    render(App, { fuente: fuenteConCanales() })
    await fireEvent.click(await filaLateral('c0'))
    await tick()
    const politeRegions = document.querySelectorAll('[aria-live="polite"]')
    const deApp = politeRegions[0] // la persistente de App va primera en el DOM
    expect(deApp.textContent).toContain(t('escenario.anuncioReproduciendo', { nombre: 'c0' }))
  })

  it('el surf por barra espaciadora funciona en ver-todo y NO en el escenario (ahí el espacio es del reproductor)', async () => {
    const fuente = fuenteConCanales()
    render(App, { fuente })
    await esperarCatalogoListo()
    await fireEvent.keyDown(window, { key: ' ', code: 'Space' })
    expect(fuente.aleatorio).not.toHaveBeenCalled()
    await abrirVerTodo()
    await fireEvent.keyDown(window, { key: ' ', code: 'Space' })
    await vi.waitFor(() => expect(fuente.aleatorio).toHaveBeenCalled())
  })

  it('⌘K abre la paleta también con un canal reproduciéndose (ya no hay modal que la inhiba) y el fondo queda inert', async () => {
    render(App, { fuente: fuenteConCanales() })
    await fireEvent.click(await filaLateral('c0'))
    await tick()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    await tick()
    expect(screen.queryByRole('dialog', { name: t('paleta.titulo') })).not.toBeNull()
    expect((document.querySelector('.fondo') as HTMLElement & { inert?: boolean })?.inert).toBe(true)
  })
})

describe('App — invariantes de integración (los que dejaron pasar los peores bugs de P0)', () => {
  it('un cambio de filtro dispara exactamente una carga nueva, no un bucle', async () => {
    const canales = vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 }))
    const fuente = fuenteFalsa({ canales })
    render(App, { fuente })

    // Carga inicial: un montaje limpio pide una página.
    await vi.waitFor(() => expect(canales).toHaveBeenCalledTimes(1))
    // Las facetas anchas (con «Mostrar los que no responden») viven en el
    // modo ver-todo desde el rework reproductor-primero. Abrirlo no dispara
    // ninguna carga: es un modo de pintar, no un filtro.
    await abrirVerTodo()
    canales.mockClear()

    const offline = await screen.findByLabelText('Mostrar los que no responden')
    await fireEvent.click(offline)

    // Exactamente una carga nueva por el cambio de filtro...
    await vi.waitFor(() => expect(canales).toHaveBeenCalledTimes(1))
    // ...y ninguna más: si el efecto se retroalimentara (el bug real del
    // `desplazamiento` sin `untrack`), aquí ya habría una segunda o tercera
    // llamada encadenada.
    await new Promise((r) => setTimeout(r, 50))
    expect(canales).toHaveBeenCalledTimes(1)
  })

  it('cambiar de filtro reinicia el desplazamiento y no acumula la página anterior', async () => {
    const pagina1: PaginaCanales = { canales: [canalDePrueba('a'), canalDePrueba('b')], total: 4 }
    const pagina2: PaginaCanales = { canales: [canalDePrueba('c'), canalDePrueba('d')], total: 4 }
    const canales = vi.fn(async (c: ConsultaCatalogo): Promise<PaginaCanales> =>
      (c.desplazamiento ?? 0) === 0 ? pagina1 : pagina2,
    )
    const fuente = fuenteFalsa({ canales })
    render(App, { fuente })
    await abrirVerTodo()

    await screen.findByLabelText('a')
    await screen.findByLabelText('b')

    // Simular scroll infinito: entra el centinela, se pide la segunda página.
    FalsoIntersectionObserver.instancias.at(-1)!.dispararInterseccion()
    await screen.findByLabelText('c')
    await screen.findByLabelText('d')
    expect(screen.queryByLabelText('a')).not.toBeNull() // las dos páginas conviven tras paginar

    // Cambiar un filtro: la lista se vacía y vuelve a empezar por la página 1,
    // sin arrastrar 'c'/'d' de la página anterior.
    canales.mockClear()
    const offline = await screen.findByLabelText('Mostrar los que no responden')
    await fireEvent.click(offline)

    await vi.waitFor(() => expect(canales).toHaveBeenCalledWith(expect.objectContaining({ desplazamiento: 0 })))
    await screen.findByLabelText('a')
    expect(screen.queryByLabelText('c')).toBeNull()
    expect(screen.queryByLabelText('d')).toBeNull()
  })

  it('soloFavoritos manda ids y nunca paginación (limite/desplazamiento)', async () => {
    const canales = vi.fn(async (_c: ConsultaCatalogo): Promise<PaginaCanales> => ({ canales: [], total: 0 }))
    const fuente = fuenteFalsa({ canales })
    render(App, { fuente })

    await vi.waitFor(() => expect(canales).toHaveBeenCalledTimes(1))
    await abrirVerTodo()
    canales.mockClear()

    const botonFavoritos = await screen.findByRole('button', { name: 'Solo favoritos' })
    await fireEvent.click(botonFavoritos)

    await vi.waitFor(() => expect(canales).toHaveBeenCalledTimes(1))
    const args = canales.mock.calls[0][0] as ConsultaCatalogo
    expect(args).toHaveProperty('ids')
    expect(args.limite).toBeUndefined()
    expect(args.desplazamiento).toBeUndefined()
  })

  it('el contador de canales sale de total (X-Total-Count), no de canales.length', async () => {
    const canales = vi.fn(async (): Promise<PaginaCanales> => ({
      canales: [canalDePrueba('a')],
      total: 12345,
    }))
    const fuente = fuenteFalsa({ canales })
    render(App, { fuente })

    await screen.findByText('12345 canales')
  })

  it('cambiar de idioma re-renderiza los textos visibles', async () => {
    const fuente = fuenteFalsa({
      canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 7 })),
    })
    render(App, { fuente })

    await screen.findByText('7 canales')

    const botonIdioma = await screen.findByRole('button', { name: 'English' })
    await fireEvent.click(botonIdioma)

    // El bug real (get(store) en vez de la runa) dejaba estos textos
    // congelados en español hasta un remount. Aquí se comprueban DOS strings
    // reactivos independientes.
    await screen.findByText('7 channels')
    await screen.findByRole('button', { name: 'Español' })
  })

  it('el shell tiene aside de facetas y area principal, y el cajon alterna', async () => {
    const { container, getByRole } = render(App, { fuente: fuenteFalsa() })
    await abrirVerTodo()
    expect(container.querySelector('aside.facetas')).not.toBeNull()
    expect(container.querySelector('main')).not.toBeNull()
    // Nombre EXACTO del cajón: con ver-todo montado, un regex laxo
    // (/facetas|filtros/i) también casa «Limpiar filtros» de BarraAcciones.
    const boton = getByRole('button', { name: t('shell.facetas') })
    const abiertoAntes = container.querySelector('aside.facetas')?.getAttribute('data-abierto')
    await fireEvent.click(boton)
    expect(container.querySelector('aside.facetas')?.getAttribute('data-abierto')).not.toBe(abiertoAntes)
  })
})

describe('App — onboarding cuando no hay fuentes (Tarea 6, P0.7)', () => {
  it('(a) sin fuentes y catálogo vacío: se ve el Onboarding y no la rejilla/acciones', async () => {
    const fuente = fuenteFalsa({ fuentes: vi.fn(async () => []) })
    render(App, { fuente })

    await screen.findByText(t('onboarding.titulo'))
    expect(screen.queryByRole('button', { name: t('accion.aleatorio') })).toBeNull()
    expect(screen.queryByRole('list', { name: t('rejilla.etiquetaLista') })).toBeNull()
  })

  it('(e) con al menos una fuente, el Onboarding no aparece', async () => {
    const fuente = fuenteFalsa() // fuenteFalsa ya trae una fuente por defecto (Tarea 6)
    render(App, { fuente })

    await esperarCatalogoListo()
    expect(screen.queryByText(t('onboarding.titulo'))).toBeNull()
  })

  it('añadir una fuente cuyo primer sondeo ya ve canales hace que el Onboarding desaparezca y el resto del shell aparezca', async () => {
    const nueva: Fuente = {
      id: 'nueva', label: 'Nueva', url: 'https://ej.test/nueva.m3u', kind: 'url', ultimoSync: null, canales: 0,
    }
    const anadirFuente = vi.fn(async () => nueva)
    // Primera llamada (carga inicial, catálogo aún vacío) → total 0, la que
    // dispara sinFuentes; TODAS las siguientes (el sondeo tras añadir) → ya
    // hay canales — el sondeo inmediato (sin esperar ningún temporizador,
    // ver iniciarSondeoFuente) basta para salir sin tocar timers falsos.
    const canales = vi.fn()
    canales
      .mockResolvedValueOnce({ canales: [], total: 0 })
      .mockResolvedValue({ canales: [canalDePrueba('a')], total: 1 })
    const fuente = fuenteFalsa({ fuentes: vi.fn(async () => []), anadirFuente, canales })
    render(App, { fuente })

    const campo = await screen.findByLabelText(t('onboarding.url.etiqueta'))
    await fireEvent.input(campo, { target: { value: 'https://ej.test/nueva.m3u' } })
    const boton = screen.getByRole('button', { name: t('onboarding.anadir') })
    await fireEvent.click(boton)

    await vi.waitFor(() => expect(anadirFuente).toHaveBeenCalledWith('https://ej.test/nueva.m3u'))
    await vi.waitFor(() => expect(screen.queryByText(t('onboarding.titulo'))).toBeNull())
    await esperarCatalogoListo()
  })
})

describe('App — fix round 1 (Tarea 6, P0.7): sondeo del catálogo tras añadir una fuente', () => {
  // Gate real en Chrome del controlador: el backend sincroniza la fuente
  // nueva de forma ASÍNCRONA — un solo refetch inmediato tras el 201 casi
  // siempre ve total=0 todavía, y (código anterior a este fix) nada volvía a
  // mirar. Estos dos tests son falsables contra ESE código: con un solo
  // refetch, ninguno de los dos vería jamás
  // onboarding.sondeo.titulo/agotado, porque esas cadenas ni siquiera
  // existían.
  const INTERVALO_TEST_MS = 10 // rápido a propósito — sondeoFuenteIntervaloMs es inyectable solo para tests

  function nuevaFuenteDePrueba(): Fuente {
    return { id: 'nueva', label: 'Nueva', url: 'https://ej.test/nueva.m3u', kind: 'url', ultimoSync: null, canales: 0 }
  }

  async function anadirDesdeOnboarding() {
    const campo = await screen.findByLabelText(t('onboarding.url.etiqueta'))
    await fireEvent.input(campo, { target: { value: 'https://ej.test/nueva.m3u' } })
    const boton = screen.getByRole('button', { name: t('onboarding.anadir') })
    await fireEvent.click(boton)
  }

  afterEach(() => {
    vi.useRealTimers()
  })

  it('total=0 en los primeros intentos y luego >0: se ve "sincronizando la fuente", después desaparece y aparece la rejilla', async () => {
    vi.useFakeTimers()
    // Llamada 1: carga inicial (sinFuentes). Llamadas 2 y 3 (sondeo,
    // intentos 1 y 2): total sigue en 0 — el bug real que este fix corrige.
    // Llamada 4 (sondeo, intento 3): el backend ya terminó de sincronizar.
    const canales = vi.fn()
    canales
      .mockResolvedValueOnce({ canales: [], total: 0 })
      .mockResolvedValueOnce({ canales: [], total: 0 })
      .mockResolvedValueOnce({ canales: [], total: 0 })
      .mockResolvedValue({ canales: [canalDePrueba('a')], total: 160 })
    const anadirFuente = vi.fn(async () => nuevaFuenteDePrueba())
    const fuente = fuenteFalsa({ fuentes: vi.fn(async () => []), anadirFuente, canales })

    // Consultas ámbito <main> (dentro de `within`): las regiones aria-live
    // PERSISTENTES de App también reflejan `onboarding.sondeo.titulo`/
    // `agotado` como texto plano (fuera de <main>) — sin acotar, getByText
    // encuentra DOS nodos con el mismo texto (el visible y el anunciado) y
    // lanza "multiple elements found". Acotar a <main> comprueba la copia
    // VISIBLE sin desactivar esa comprobación de accesibilidad (que hacen
    // los otros tests de a11y de este mismo repo).
    const { container } = render(App, { fuente, sondeoFuenteIntervaloMs: INTERVALO_TEST_MS, sondeoFuenteIntentosMax: 5 })
    const main = () => within(container.querySelector('main') as HTMLElement)
    await vi.advanceTimersByTimeAsync(0) // resuelve onMount (salud/facetas/fuentes) + la carga inicial

    expect(main().getByText(t('onboarding.titulo'))).not.toBeNull()

    await anadirDesdeOnboarding()
    await vi.advanceTimersByTimeAsync(0) // anadirFuente resuelve → iniciarSondeoFuente → intento 1 (inmediato)

    // El Onboarding ya se fue (fuentes dejó de estar vacío) pero la rejilla
    // TODAVÍA no debe verse — total sigue en 0 tras el intento 1. Esto es
    // justo lo que el código pre-fix hacía mal: mostraba "0 canales" aquí.
    expect(main().queryByText(t('onboarding.titulo'))).toBeNull()
    expect(main().getByText(t('onboarding.sondeo.titulo'))).not.toBeNull()
    expect(main().queryByRole('button', { name: t('accion.aleatorio') })).toBeNull()
    expect(canales).toHaveBeenCalledTimes(2)

    await vi.advanceTimersByTimeAsync(INTERVALO_TEST_MS) // intento 2: total sigue en 0
    expect(canales).toHaveBeenCalledTimes(3)
    expect(main().getByText(t('onboarding.sondeo.titulo'))).not.toBeNull()

    await vi.advanceTimersByTimeAsync(INTERVALO_TEST_MS) // intento 3: total=160 → éxito
    expect(canales).toHaveBeenCalledTimes(4)

    expect(main().queryByText(t('onboarding.sondeo.titulo'))).toBeNull()
    expect(main().queryByText(t('onboarding.titulo'))).toBeNull()
    expect(main().getByRole('button', { name: t('escenario.verTodo') })).not.toBeNull()
  })

  it('total=0 en todos los intentos: al agotar el tope aparece el aviso honesto con «Reintentar»', async () => {
    vi.useFakeTimers()
    const canales = vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 })) // nunca hay canales
    const anadirFuente = vi.fn(async () => nuevaFuenteDePrueba())
    const fuente = fuenteFalsa({ fuentes: vi.fn(async () => []), anadirFuente, canales })

    const { container } = render(App, { fuente, sondeoFuenteIntervaloMs: INTERVALO_TEST_MS, sondeoFuenteIntentosMax: 3 })
    const main = () => within(container.querySelector('main') as HTMLElement)
    await vi.advanceTimersByTimeAsync(0) // carga inicial → Onboarding

    await anadirDesdeOnboarding()
    await vi.advanceTimersByTimeAsync(0) // intento 1/3
    expect(main().getByText(t('onboarding.sondeo.titulo'))).not.toBeNull()
    expect(main().queryByText(t('onboarding.sondeo.agotado'))).toBeNull()

    await vi.advanceTimersByTimeAsync(INTERVALO_TEST_MS) // intento 2/3
    expect(main().queryByText(t('onboarding.sondeo.agotado'))).toBeNull()

    await vi.advanceTimersByTimeAsync(INTERVALO_TEST_MS) // intento 3/3: se agota el tope
    expect(main().getByText(t('onboarding.sondeo.agotado'))).not.toBeNull()

    const reintentar = main().getByRole('button', { name: t('onboarding.sondeo.reintentar') })
    expect(reintentar).not.toBeNull()
    // NO silencios (brief): ni la rejilla ni el Onboarding vuelven a
    // aparecer solos — hace falta la acción explícita de "Reintentar".
    expect(main().queryByRole('button', { name: t('accion.aleatorio') })).toBeNull()
    expect(main().queryByText(t('onboarding.titulo'))).toBeNull()

    // "Reintentar" vuelve a sondear desde cero: mismo camino, nueva
    // oportunidad de éxito si el backend ya terminó.
    canales.mockImplementation(async () => ({ canales: [canalDePrueba('a')], total: 42 }))
    await fireEvent.click(reintentar)
    await vi.advanceTimersByTimeAsync(0)

    expect(main().queryByText(t('onboarding.sondeo.agotado'))).toBeNull()
    expect(main().getByRole('button', { name: t('escenario.verTodo') })).not.toBeNull()
  })

  it('F2: en el estado agotado, «Gestionar fuentes» limpia el aviso y lleva a la vista de gestión, con la fuente rota lista para borrar', async () => {
    // Antes de este fix: sondeoAgotado se quedaba en true para siempre, y esa
    // rama del <main> (la primera en el orden de App.svelte) ganaba pase lo
    // que pasara — ni «Reintentar» (mismo camino, mismo timeout) ni el botón
    // «Fuentes» de la cabecera (ponía vistaFuentes=true, pero esa rama nunca
    // se llegaba a evaluar) sacaban de aquí a quien había añadido una fuente
    // con un typo. Este test es falsable contra ESE código: sin
    // alGestionarFuentes limpiando sondeoAgotado, «Gestionar fuentes» ni
    // siquiera existiría en el DOM.
    vi.useFakeTimers()
    let fuentesBackend: Fuente[] = []
    const fuentes = vi.fn(async () => fuentesBackend)
    const anadirFuente = vi.fn(async () => {
      const f = nuevaFuenteDePrueba()
      fuentesBackend = [...fuentesBackend, f] // el backend SÍ crea la fuente — solo nunca trae canales (typo en la URL)
      return f
    })
    const canales = vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 })) // nunca hay canales
    const fuente = fuenteFalsa({ fuentes, anadirFuente, canales })

    const { container } = render(App, { fuente, sondeoFuenteIntervaloMs: INTERVALO_TEST_MS, sondeoFuenteIntentosMax: 3 })
    const main = () => within(container.querySelector('main') as HTMLElement)
    await vi.advanceTimersByTimeAsync(0) // carga inicial → Onboarding

    await anadirDesdeOnboarding()
    await vi.advanceTimersByTimeAsync(0) // intento 1/3
    await vi.advanceTimersByTimeAsync(INTERVALO_TEST_MS) // intento 2/3
    await vi.advanceTimersByTimeAsync(INTERVALO_TEST_MS) // intento 3/3: se agota el tope
    expect(main().getByText(t('onboarding.sondeo.agotado'))).not.toBeNull()

    const gestionar = main().getByRole('button', { name: t('onboarding.sondeo.gestionar') })
    await fireEvent.click(gestionar)
    await vi.advanceTimersByTimeAsync(0) // Fuentes.svelte monta y refetchea fuentes()

    // El aviso desaparece Y aparece la vista de gestión de VERDAD — no un
    // hash cambiado sin ningún efecto visible.
    expect(screen.queryByText(t('onboarding.sondeo.agotado'))).toBeNull()
    expect(screen.getByRole('heading', { name: t('fuentes.titulo') })).not.toBeNull()
    expect(window.location.hash).toBe('#fuentes')

    // La fuente rota SÍ está en la lista: quien llega hasta aquí puede
    // borrarla, no solo "salir" del aviso de agotado.
    expect(screen.getByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Nueva' }) })).not.toBeNull()
  })

  it('desmontar App durante el sondeo no deja timers sueltos (no llama a canales() otra vez tras desmontar)', async () => {
    vi.useFakeTimers()
    const canales = vi.fn(async () => ({ canales: [], total: 0 })) // nunca hay canales
    const anadirFuente = vi.fn(async () => nuevaFuenteDePrueba())
    const fuente = fuenteFalsa({ fuentes: vi.fn(async () => []), anadirFuente, canales })

    const { unmount } = render(App, { fuente, sondeoFuenteIntervaloMs: INTERVALO_TEST_MS, sondeoFuenteIntentosMax: 60 })
    await vi.advanceTimersByTimeAsync(0)
    await anadirDesdeOnboarding()
    await vi.advanceTimersByTimeAsync(0) // intento 1, programa el intento 2

    const llamadasAntesDeDesmontar = canales.mock.calls.length
    unmount()

    await vi.advanceTimersByTimeAsync(INTERVALO_TEST_MS * 5)
    expect(canales.mock.calls.length).toBe(llamadasAntesDeDesmontar)
  })
})

describe('App — vista Fuentes (gestión, Tarea 7 de P0.7)', () => {
  it('(f) el botón «Fuentes» de la cabecera abre la vista y fija el hash #fuentes', async () => {
    render(App, { fuente: fuenteFalsa() })

    const botonFuentes = await screen.findByRole('button', { name: t('fuentes.abrir') })
    await fireEvent.click(botonFuentes)

    await screen.findByRole('heading', { name: t('fuentes.titulo') })
    expect(window.location.hash).toBe('#fuentes')
  })

  it('montar App con el hash #fuentes ya puesto abre la vista directamente', async () => {
    window.location.hash = 'fuentes'
    try {
      render(App, { fuente: fuenteFalsa() })
      await screen.findByRole('heading', { name: t('fuentes.titulo') })
    } finally {
      window.location.hash = ''
    }
  })

  it('(e) quitar la ÚLTIMA fuente desde la vista de gestión hace que el Onboarding vuelva a aparecer', async () => {
    const fuentes = vi.fn()
    fuentes
      .mockResolvedValueOnce([fuenteDePrueba('f0')]) // carga inicial de App (onMount)
      .mockResolvedValueOnce([fuenteDePrueba('f0')]) // carga inicial de Fuentes.svelte al montar la vista
      .mockResolvedValueOnce([]) // refetch tras quitar: cero fuentes
    const quitarFuente = vi.fn(async () => {})
    const fuente = fuenteFalsa({ fuentes, quitarFuente })
    render(App, { fuente })

    await esperarCatalogoListo() // shell normal, con la fuente por defecto

    const botonFuentes = await screen.findByRole('button', { name: t('fuentes.abrir') })
    await fireEvent.click(botonFuentes)

    const etiqueta = `Fuente f0`
    const botonQuitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: etiqueta }) })
    await fireEvent.click(botonQuitar)
    const confirmar = await screen.findByRole('button', { name: t('fuentes.quitar.confirmar.etiqueta', { label: etiqueta }) })
    await fireEvent.click(confirmar)

    await vi.waitFor(() => expect(quitarFuente).toHaveBeenCalledWith('f0'))
    // El Onboarding vuelve solo: App recibió la lista fresca (vacía) vía
    // alFuentesCambiaron, sinFuentes pasó a true, y esa rama gana sobre
    // vistaFuentes en el <main> de App.svelte.
    await screen.findByText(t('onboarding.titulo'))
    expect(screen.queryByRole('heading', { name: t('fuentes.titulo') })).toBeNull()
  })

  it('añadir una fuente desde la vista Fuentes cierra la vista y dispara el mismo sondeo que el onboarding', async () => {
    const nueva: Fuente = {
      id: 'nueva', label: 'Nueva', url: 'https://ej.test/nueva.m3u', kind: 'url', ultimoSync: null, canales: 0,
    }
    const anadirFuente = vi.fn(async () => nueva)
    const canales = vi.fn()
    canales
      .mockResolvedValueOnce({ canales: [], total: 0 }) // carga inicial de App
      .mockResolvedValue({ canales: [canalDePrueba('a')], total: 1 }) // sondeo inmediato: ya hay canales
    const fuente = fuenteFalsa({ anadirFuente, canales })
    render(App, { fuente })

    const botonFuentes = await screen.findByRole('button', { name: t('fuentes.abrir') })
    await fireEvent.click(botonFuentes)
    await screen.findByRole('heading', { name: t('fuentes.titulo') })

    const campo = await screen.findByLabelText(t('onboarding.url.etiqueta'))
    await fireEvent.input(campo, { target: { value: 'https://ej.test/nueva.m3u' } })
    const botonAnadir = screen.getByRole('button', { name: t('onboarding.anadir') })
    await fireEvent.click(botonAnadir)

    await vi.waitFor(() => expect(anadirFuente).toHaveBeenCalledWith('https://ej.test/nueva.m3u'))
    // La vista Fuentes se cierra (mismo hash que #stats: pushState limpia el
    // hash) y el sondeo de siempre (SincronizandoFuente / rejilla) toma el
    // relevo — nunca se queda "atascado" mostrando la vista de gestión.
    await vi.waitFor(() => expect(screen.queryByRole('heading', { name: t('fuentes.titulo') })).toBeNull())
    await esperarCatalogoListo()
  })

  it('F3: sin fuentes, «Fuentes» de la cabecera SÍ abre la vista de gestión (no un clic mudo) y añadir la primera fuente ahí aterriza en el catálogo, no en gestión', async () => {
    // Antes de este fix: sinFuentes (fuentes.length === 0) ganaba siempre a
    // vistaFuentes en el <main> de App.svelte, así que pulsar «Fuentes»
    // durante el onboarding dejaba vistaFuentes=true SIN NINGÚN efecto
    // visible — y luego, al sincronizar la primera fuente (añadida desde el
    // Onboarding, que seguía siendo lo que se veía), sinFuentes pasaba a
    // false y esa vistaFuentes nunca reseteada ganaba: quien acababa de
    // añadir su primera fuente aterrizaba en la vista de GESTIÓN en vez de
    // en su catálogo recién sincronizado.
    const nueva: Fuente = {
      id: 'nueva', label: 'Nueva', url: 'https://ej.test/nueva.m3u', kind: 'url', ultimoSync: null, canales: 0,
    }
    const anadirFuente = vi.fn(async () => nueva)
    const canales = vi.fn()
    canales
      .mockResolvedValueOnce({ canales: [], total: 0 }) // carga inicial: sin fuentes, catálogo vacío
      .mockResolvedValue({ canales: [canalDePrueba('a')], total: 1 }) // sondeo inmediato tras añadir: ya hay canales
    const fuente = fuenteFalsa({ fuentes: vi.fn(async () => []), anadirFuente, canales })
    render(App, { fuente })

    await screen.findByText(t('onboarding.titulo')) // arranque en limpio: onboarding

    const botonFuentes = await screen.findByRole('button', { name: t('fuentes.abrir') })
    await fireEvent.click(botonFuentes)

    // El clic tiene que verse: la vista de gestión real aparece, aunque la
    // lista de fuentes esté vacía (con su propio bloque «Añadir otra
    // fuente» — la MISMA AnadirFuente que el onboarding).
    await screen.findByRole('heading', { name: t('fuentes.titulo') })
    expect(window.location.hash).toBe('#fuentes')

    const campo = await screen.findByLabelText(t('onboarding.url.etiqueta'))
    await fireEvent.input(campo, { target: { value: 'https://ej.test/nueva.m3u' } })
    const botonAnadir = screen.getByRole('button', { name: t('onboarding.anadir') })
    await fireEvent.click(botonAnadir)

    await vi.waitFor(() => expect(anadirFuente).toHaveBeenCalledWith('https://ej.test/nueva.m3u'))
    // Aterriza en el catálogo (rejilla/BarraAcciones) — NUNCA de vuelta en
    // la vista de gestión, que es justo el bug que este fix cierra.
    await esperarCatalogoListo()
    expect(screen.queryByRole('heading', { name: t('fuentes.titulo') })).toBeNull()
  })
})

// Tarea 4 (P0.8): el atajo ⌘K/Ctrl+K en sí (abrir/inhibir) es responsabilidad
// de App (alTeclaVentana) — el resto del comportamiento de la paleta
// (filtrado, navegación, Enter/Esc) está cubierto de forma aislada en
// Paleta.test.ts, montando el componente directamente sin pasar por App.
function paletaDialogo() {
  return screen.queryByRole('dialog', { name: t('paleta.titulo') })
}

describe('App — paleta de comandos ⌘K', () => {
  // La acción «Abrir Fuentes» (más abajo) navega a #fuentes sin pasar por
  // volverDeFuentes() — a diferencia de otros tests de este fichero que sí
  // cierran esa vista (y con ella limpian el hash vía history.pushState),
  // este deja window.location.hash='#fuentes' sucio para el SIGUIENTE test
  // del archivo, que montaría App ya "dentro" de la vista Fuentes (bug real
  // cazado al escribir este mismo bloque). Se resetea aquí explícitamente.
  afterEach(() => {
    window.location.hash = ''
  })

  it('(a) ⌘ (metaKey) + K abre la paleta', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    await esperarCatalogoListo()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    await tick()

    expect(paletaDialogo()).not.toBeNull()
  })

  it('Ctrl+K también la abre (no solo ⌘ de mac)', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    await esperarCatalogoListo()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await tick()

    expect(paletaDialogo()).not.toBeNull()
  })

  it('(a) NO se abre con el foco en un input ajeno — no le roba el atajo al buscador de facetas', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    await esperarCatalogoListo()

    const buscador = screen.getByLabelText(t('catalogo.buscar')) as HTMLInputElement
    buscador.focus()
    expect(document.activeElement).toBe(buscador)
    await fireEvent.keyDown(buscador, { key: 'k', metaKey: true })
    await tick()

    expect(paletaDialogo()).toBeNull()
  })

  // M1 del pase de a11y (Tarea 8, P0.8): el guard de arriba (INPUT/TEXTAREA)
  // dejaba pasar ⌘K con el foco en un <button> real de la app — p. ej. el
  // segmentado de densidad de Ajustes.svelte — o en un <select>/contenido
  // contenteditable cualquiera. esObjetivoInteractivo (lib/surf.ts) es AHORA
  // el único criterio, compartido con debeHacerSurf; estas tres pruebas
  // habrían fallado con el guard viejo (ver surf.test.ts para el mismo
  // criterio probado en aislamiento).
  it('(M1) NO se abre con el foco en un <button> real de la app (p. ej. «Ver todo»)', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    const boton = await screen.findByRole('button', { name: t('escenario.verTodo') })

    boton.focus()
    expect(document.activeElement).toBe(boton)
    await fireEvent.keyDown(boton, { key: 'k', metaKey: true })
    await tick()

    expect(paletaDialogo()).toBeNull()
  })

  it('(M1) NO se abre con el foco en un <select> ajeno', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    await esperarCatalogoListo()

    const select = document.createElement('select')
    document.body.appendChild(select)
    select.focus()
    expect(document.activeElement).toBe(select)
    await fireEvent.keyDown(select, { key: 'k', metaKey: true })
    await tick()

    expect(paletaDialogo()).toBeNull()
    select.remove()
  })

  it('(M1) NO se abre con el foco en contenido contenteditable', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    await esperarCatalogoListo()

    const editable = document.createElement('div')
    editable.setAttribute('contenteditable', 'true')
    editable.tabIndex = 0
    // jsdom no calcula isContentEditable a partir del atributo (mismo motivo
    // que surf.test.ts): se fuerza para ejercitar la rama que sí lo comprueba
    // en tiempo real de navegador.
    Object.defineProperty(editable, 'isContentEditable', { value: true })
    document.body.appendChild(editable)
    editable.focus()
    expect(document.activeElement).toBe(editable)
    await fireEvent.keyDown(editable, { key: 'k', metaKey: true })
    await tick()

    expect(paletaDialogo()).toBeNull()
    editable.remove()
  })

  // Fix (revisión final, P0.8): al invariante "los dos modales nunca
  // coexisten" le faltaba esta pata. La paleta tiene tabindex="-1" en su
  // contenedor — un clic en su chrome NO interactivo (la pista, un título de
  // grupo) saca el foco del <input> y lo deja en ese div. Sin `paletaAbierta`
  // en la guarda del surf, la barra espaciadora pasaba debeHacerSurf (el div
  // no es un objetivo interactivo) y abría un canal ENCIMA de la paleta ya
  // abierta. Este test reproduce justo ese camino: foco fuera del input,
  // paleta abierta, espacio sobre window.
  it('el surf (barra espaciadora) NO abre un canal con la paleta abierta, aunque el foco esté fuera del input', async () => {
    const canales = [canalDePrueba('a')]
    const fuente = fuenteFalsa({
      canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales, total: 1 })),
      // debe coincidir con el canal real de la rejilla — si no, el falso
      // "aleatorio" por defecto (id 'random') abriría un diálogo con OTRO
      // nombre y la aserción de más abajo no detectaría el fallo real.
      aleatorio: vi.fn(async () => canalDePrueba('a')),
    })
    render(App, { fuente })
    // El surf vive en el modo ver-todo (reproductor-primero): se abre ANTES
    // de la paleta para ejercitar justo la guarda paletaAbierta del surf.
    await abrirVerTodo()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    await tick()
    await screen.findByRole('dialog', { name: t('paleta.titulo') })

    // Simula el foco cayendo en el propio contenedor de la paleta (o
    // cualquier div no interactivo), como tras un clic en su chrome.
    const divNoInteractivo = document.createElement('div')
    divNoInteractivo.tabIndex = -1
    document.body.appendChild(divNoInteractivo)
    divNoInteractivo.focus()
    expect(document.activeElement).toBe(divNoInteractivo)

    await fireEvent.keyDown(divNoInteractivo, { key: ' ' })
    await tick()

    expect(paletaDialogo()).not.toBeNull() // la paleta sigue abierta
    expect(fuente.aleatorio).not.toHaveBeenCalled() // el surf no saltó por debajo
    expect(screen.queryByRole('region', { name: 'a' })).toBeNull() // ningún canal se abrió
    divNoInteractivo.remove()
  })

  it('Esc cierra la paleta (App la desmonta al recibir alCerrar)', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    await esperarCatalogoListo()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    await tick()
    await screen.findByRole('dialog', { name: t('paleta.titulo') })

    const campo = screen.getByRole('combobox')
    await fireEvent.keyDown(campo, { key: 'Escape' })
    await tick()

    expect(paletaDialogo()).toBeNull()
  })

  it('elegir «Abrir Fuentes» desde la paleta navega a la vista de gestión de fuentes real de App', async () => {
    const fuente = fuenteFalsa()
    render(App, { fuente })
    await esperarCatalogoListo()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    await tick()

    await fireEvent.click(screen.getByRole('option', { name: t('paleta.accion.abrirFuentes') }))

    await screen.findByRole('heading', { name: t('fuentes.titulo') })
    expect(paletaDialogo()).toBeNull()
  })

  it('elegir un canal desde la paleta lo reproduce en el MISMO panel persistente del escenario', async () => {
    const canales = [canalDePrueba('a')]
    const fuente = fuenteFalsa({ canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales, total: 1 })) })
    render(App, { fuente })
    await esperarCatalogoListo()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    await tick()

    await fireEvent.click(screen.getByRole('option', { name: 'a' }))

    await screen.findByRole('region', { name: 'a' }) // el panel del Reproductor, no un modal
    expect(paletaDialogo()).toBeNull()
  })

})

describe('App — vista Ajustes (Tarea 6, P0.8)', () => {
  afterEach(() => {
    window.location.hash = ''
  })

  it('(a) el botón «Ajustes» de la cabecera abre la vista y fija el hash #ajustes', async () => {
    render(App, { fuente: fuenteFalsa() })

    const botonAjustes = await screen.findByRole('button', { name: t('ajustes.abrir') })
    await fireEvent.click(botonAjustes)

    await screen.findByRole('heading', { name: t('ajustes.titulo') })
    expect(window.location.hash).toBe('#ajustes')
  })

  it('(a) montar App con el hash #ajustes ya puesto abre la vista directamente', async () => {
    window.location.hash = 'ajustes'
    render(App, { fuente: fuenteFalsa() })
    await screen.findByRole('heading', { name: t('ajustes.titulo') })
  })

  it('«Ajustes» es alcanzable aunque el catálogo siga sincronizando (vista top-level, no anidada en fase listo)', async () => {
    const fuente = fuenteFalsa()
    const { consultarSalud } = await import('./estado/salud')
    vi.mocked(consultarSalud).mockResolvedValueOnce({
      sincronizando: true,
      proxyDisponible: false,
      ultimoSync: null,
      version: 'test',
    })
    render(App, { fuente })

    const botonAjustes = await screen.findByRole('button', { name: t('ajustes.abrir') })
    await fireEvent.click(botonAjustes)

    await screen.findByRole('heading', { name: t('ajustes.titulo') })
  })

  it('«Volver» cierra la vista y limpia el hash; Ajustes y Fuentes son mutuamente excluyentes', async () => {
    render(App, { fuente: fuenteFalsa() })

    await fireEvent.click(await screen.findByRole('button', { name: t('ajustes.abrir') }))
    await screen.findByRole('heading', { name: t('ajustes.titulo') })

    await fireEvent.click(await screen.findByRole('button', { name: t('ajustes.volver') }))
    expect(screen.queryByRole('heading', { name: t('ajustes.titulo') })).toBeNull()
    expect(window.location.hash).toBe('')

    // Abrir Fuentes y luego Ajustes deja solo Ajustes visible — nunca las dos
    // vistas montadas a la vez.
    await fireEvent.click(await screen.findByRole('button', { name: t('fuentes.abrir') }))
    await screen.findByRole('heading', { name: t('fuentes.titulo') })
    await fireEvent.click(screen.getByRole('button', { name: t('ajustes.abrir') }))
    await screen.findByRole('heading', { name: t('ajustes.titulo') })
    expect(screen.queryByRole('heading', { name: t('fuentes.titulo') })).toBeNull()
  })

  it('la densidad elegida en Ajustes se refleja en la rejilla (Tarea 5 ya cableada, aquí solo el extremo de UI)', async () => {
    render(App, { fuente: fuenteFalsa() })
    await esperarCatalogoListo()

    await fireEvent.click(await screen.findByRole('button', { name: t('ajustes.abrir') }))
    await screen.findByRole('heading', { name: t('ajustes.titulo') })

    await fireEvent.click(screen.getByRole('button', { name: t('ajustes.densidad.compacta') }))
    expect(get(preferencias).densidad).toBe('compacta')
  })
})

describe('App — recordar vista (preferencias.recordarVista, Tarea 6 de P0.8)', () => {
  it('(c) ON por defecto: cambiar a «lista» y simular un remonte restaura la vista guardada', async () => {
    const { unmount } = render(App, { fuente: fuenteFalsa() })
    await abrirVerTodo()

    await fireEvent.click(screen.getByRole('button', { name: t('accion.lista') }))
    await vi.waitFor(() => expect(get(filtros).vista).toBe('lista'))

    unmount()
    // Simula lo que pasa en un reload real: el store de filtros vuelve a su
    // valor por defecto (rejilla) — solo localStorage sobrevive.
    filtros.update((f) => ({ ...f, vista: 'rejilla' }))

    render(App, { fuente: fuenteFalsa() })
    await abrirVerTodo()
    expect(screen.getByRole('button', { name: t('accion.lista') }).getAttribute('aria-pressed')).toBe('true')
  })

  it('OFF: apagar «recordar vista» hace que un remonte NO restaure la vista', async () => {
    preferencias.set({ densidad: 'comoda', recordarVista: false, recordarFiltros: false })
    const { unmount } = render(App, { fuente: fuenteFalsa() })
    await abrirVerTodo()

    await fireEvent.click(screen.getByRole('button', { name: t('accion.lista') }))
    await vi.waitFor(() => expect(get(filtros).vista).toBe('lista'))

    unmount()
    filtros.update((f) => ({ ...f, vista: 'rejilla' }))

    render(App, { fuente: fuenteFalsa() })
    await abrirVerTodo()
    expect(screen.getByRole('button', { name: t('accion.rejilla') }).getAttribute('aria-pressed')).toBe('true')
  })
})

describe('App — recordar filtros (preferencias.recordarFiltros, default OFF, Tarea 6 de P0.8)', () => {
  afterEach(() => {
    window.location.hash = ''
  })


  it('(d) por defecto (OFF): cambiar un filtro y simular un remonte NO lo restaura', async () => {
    const { unmount } = render(App, { fuente: fuenteFalsa() })
    await esperarCatalogoListo()

    filtros.update((f) => ({ ...f, pais: 'MX' }))
    await tick()

    unmount()
    filtros.update((f) => ({ ...f, pais: '' }))

    render(App, { fuente: fuenteFalsa() })
    await esperarCatalogoListo()
    expect(get(filtros).pais).toBe('')
  })

  it('(d) activado desde Ajustes: cambiar un filtro y simular un remonte SÍ lo restaura', async () => {
    const { unmount } = render(App, { fuente: fuenteFalsa() })
    await esperarCatalogoListo()

    await fireEvent.click(await screen.findByRole('button', { name: t('ajustes.abrir') }))
    const casilla = await screen.findByRole('checkbox', { name: t('ajustes.recordarFiltros.titulo') })
    await fireEvent.click(casilla)
    expect(get(preferencias).recordarFiltros).toBe(true)
    await fireEvent.click(screen.getByRole('button', { name: t('ajustes.volver') }))
    await esperarCatalogoListo()

    filtros.update((f) => ({ ...f, pais: 'MX' }))
    await tick()

    unmount()
    filtros.update((f) => ({ ...f, pais: '' }))

    render(App, { fuente: fuenteFalsa() })
    await esperarCatalogoListo()
    expect(get(filtros).pais).toBe('MX')
  })
})
