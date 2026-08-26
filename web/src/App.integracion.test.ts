import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import App from './App.svelte'
import type { Canal, CatalogSource, ConsultaCatalogo, PaginaCanales } from './datos/catalogo'
import { filtros } from './estado/filtros'
import { favoritos } from './estado/favoritos'
import { idioma } from './i18n'

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

/** CatalogSource falso en memoria: implementa TODOS los métodos de la
 * interfaz (incluido mirrors, de la Tarea 3) para que App pueda montarse
 * entero sin tocar la red. */
function fuenteFalsa(overrides: Partial<CatalogSource> = {}): CatalogSource {
  const canales = vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 }))
  return {
    canales,
    paises: vi.fn(async () => []),
    categorias: vi.fn(async () => []),
    aleatorio: vi.fn(async () => canalDePrueba('random')),
    destino: vi.fn(async () => ({ url: '', airplayOk: null })),
    mirrors: vi.fn(async () => []),
    frescura: vi.fn(async () => ({ tipo: 'vivo' as const, generadoEn: null })),
    proxyDisponible: vi.fn(async () => false),
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
})
