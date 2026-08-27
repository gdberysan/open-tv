import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import CatalogoLateral from './CatalogoLateral.svelte'
import { filtros } from '../estado/filtros'
import type { Canal } from '../datos/catalogo'
import { t } from '../i18n'

class FalsoIntersectionObserver {
  constructor(private cb: IntersectionObserverCallback) {}
  observe() {}
  disconnect() {}
  unobserve() {}
  takeRecords() {
    return []
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalDePrueba(id: string): Canal {
  return {
    id,
    nombre: `Canal ${id}`,
    logoUrl: '',
    categoriaId: '',
    idioma: 'es',
    pais: '',
    vivo: true,
    latenciaMs: 10,
    webOk: true,
  }
}

const props = {
  canales: [canalDePrueba('a')],
  total: 1234,
  cargando: false,
  paises: [{ valor: 'MX', total: 9 }],
  categorias: [{ valor: 'news', total: 5 }],
  alAbrir: vi.fn(),
  alPedirMas: vi.fn(),
}

beforeEach(() => {
  vi.useRealTimers()
  filtros.set({
    q: '',
    pais: '',
    categoria: '',
    calidad: '',
    mostrarOffline: false,
    soloFavoritos: false,
    vista: 'rejilla',
  })
})

describe('CatalogoLateral', () => {
  it('monta buscador, chips de facetas, conteo total y la lista de canales', () => {
    render(CatalogoLateral, props)
    expect(screen.getByLabelText(t('catalogo.buscar'))).toBeTruthy()
    expect(screen.getByRole('button', { name: /MX/ })).toBeTruthy()
    expect(screen.getByText(t('catalogo.total', { n: 1234 }))).toBeTruthy()
    expect(screen.getByRole('button', { name: /Canal a/ })).toBeTruthy()
  })

  it('teclear en el buscador escribe filtros.q tras el debounce', async () => {
    vi.useFakeTimers()
    render(CatalogoLateral, props)
    const input = screen.getByLabelText(t('catalogo.buscar'))
    await fireEvent.input(input, { target: { value: 'tele' } })
    expect(get(filtros).q).toBe('')
    vi.advanceTimersByTime(350)
    expect(get(filtros).q).toBe('tele')
  })
})
