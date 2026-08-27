import { beforeEach, describe, expect, it } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import FacetasCompactas from './FacetasCompactas.svelte'
import { filtros } from '../estado/filtros'
import type { Faceta } from '../datos/catalogo'

const paises: Faceta[] = [
  { valor: 'MX', total: 900 },
  { valor: 'ES', total: 800 },
  { valor: 'AR', total: 700 },
  { valor: 'US', total: 600 },
  { valor: 'FR', total: 500 },
  { valor: 'DE', total: 400 },
  { valor: 'IT', total: 300 },
  { valor: 'JP', total: 200 },
]
const categorias: Faceta[] = [
  { valor: 'news', total: 50 },
  { valor: 'sports', total: 40 },
]

beforeEach(() => {
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

describe('FacetasCompactas', () => {
  it('muestra como mucho el TOP 6 por conteo de cada grupo, no las 8 facetas', () => {
    render(FacetasCompactas, { paises, categorias })
    expect(screen.queryByRole('button', { name: /MX/ })).toBeTruthy()
    // IT (300) y JP (200) quedan fuera del top 6
    expect(screen.queryByRole('button', { name: /IT/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /JP/ })).toBeNull()
  })

  it('cada chip lleva su conteo y alternar escribe/limpia el filtro (aria-pressed)', async () => {
    render(FacetasCompactas, { paises, categorias })
    const chip = screen.getByRole('button', { name: /MX/ })
    expect(chip.textContent).toContain('900')
    await fireEvent.click(chip)
    expect(get(filtros).pais).toBe('MX')
    expect(chip.getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(chip)
    expect(get(filtros).pais).toBe('')
  })

  it('una faceta ACTIVA fuera del top 6 aparece igualmente (si no, sería imposible verla/quitarla)', () => {
    filtros.update((f) => ({ ...f, pais: 'JP' }))
    render(FacetasCompactas, { paises, categorias })
    const chip = screen.getByRole('button', { name: /JP/ })
    expect(chip.getAttribute('aria-pressed')).toBe('true')
  })
})
