import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen, within } from '@testing-library/svelte'
import App from './App.svelte'
import type { Canal, CatalogSource, ConsultaCatalogo, Fuente, PaginaCanales } from './datos/catalogo'
import { filtros } from './estado/filtros'
import { favoritos } from './estado/favoritos'
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
  idioma.actual = 'es'
  FalsoIntersectionObserver.instancias.length = 0
})

describe('App — invariantes de integración (los que dejaron pasar los peores bugs de P0)', () => {
  it('un cambio de filtro dispara exactamente una carga nueva, no un bucle', async () => {
    const canales = vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 }))
    const fuente = fuenteFalsa({ canales })
    render(App, { fuente })

    // Carga inicial: un montaje limpio pide una página.
    await vi.waitFor(() => expect(canales).toHaveBeenCalledTimes(1))
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
    expect(container.querySelector('aside.facetas')).not.toBeNull()
    expect(container.querySelector('main')).not.toBeNull()
    const boton = getByRole('button', { name: /facetas|filtros/i })
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

    await screen.findByRole('button', { name: t('accion.aleatorio') })
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
    await screen.findByRole('button', { name: t('accion.aleatorio') })
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
    expect(main().getByRole('button', { name: t('accion.aleatorio') })).not.toBeNull()
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
    expect(main().getByRole('button', { name: t('accion.aleatorio') })).not.toBeNull()
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

    await screen.findByRole('button', { name: t('accion.aleatorio') }) // shell normal, con la fuente por defecto

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
    await screen.findByRole('button', { name: t('accion.aleatorio') })
  })
})
