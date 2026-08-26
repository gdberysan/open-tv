import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen, within } from '@testing-library/svelte'
import { get } from 'svelte/store'
import BarraLateralFacetas from './BarraLateralFacetas.svelte'
import { filtros } from '../estado/filtros'
import { t } from '../i18n'
import type { Faceta } from '../datos/catalogo'

// Patrón establecido en P0.5: el reset vive a nivel de fichero, no anidado en
// cada describe — así ningún test hereda el store mutado por el anterior.
beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
})

const paises: Faceta[] = [{ valor: 'ES', total: 12 }, { valor: 'MX', total: 7 }]
const categorias: Faceta[] = [{ valor: 'Deportes', total: 3 }]
const calidades: Faceta[] = [{ valor: 'hd', total: 20 }, { valor: 'fhd', total: 9 }, { valor: '4k', total: 2 }]

// getByRole con `name` de cadena hace coincidencia EXACTA por defecto (no
// substring), y la etiqueta traducida de "hd" lleva paréntesis que no son
// literales seguros para una RegExp — de ahí este filtro manual en vez de un
// matcher de nombre.
function filaCalidadHd(contenedor: HTMLElement): HTMLElement {
  const etiqueta = t('filtro.calidad.hd')
  const boton = within(contenedor)
    .getAllByRole('button')
    .find((b) => b.textContent?.includes(etiqueta))
  if (!boton) throw new Error(`No se encontró la fila de calidad "${etiqueta}"`)
  return boton
}

describe('BarraLateralFacetas — grupos con conteos', () => {
  it('pinta País/Categoría/Calidad, cada fila con su valor y su conteo', () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    expect(within(grupoPais).getByRole('button', { name: /ES/ }).textContent).toContain('12')
    expect(within(grupoPais).getByRole('button', { name: /MX/ }).textContent).toContain('7')

    const grupoCategoria = screen.getByRole('group', { name: t('filtro.categoria') })
    expect(within(grupoCategoria).getByRole('button', { name: /Deportes/ }).textContent).toContain('3')

    const grupoCalidad = screen.getByRole('group', { name: t('filtro.calidad') })
    expect(filaCalidadHd(grupoCalidad).textContent).toContain('20')
  })
})

describe('BarraLateralFacetas — seleccionar/deseleccionar una faceta', () => {
  it('pulsar un país escribe filtros.pais; pulsarlo otra vez lo limpia', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    const filaEs = within(grupoPais).getByRole('button', { name: /ES/ })

    expect(filaEs.getAttribute('aria-pressed')).toBe('false')

    await fireEvent.click(filaEs)
    expect(get(filtros).pais).toBe('ES')
    expect(filaEs.getAttribute('aria-pressed')).toBe('true')

    await fireEvent.click(filaEs)
    expect(get(filtros).pais).toBe('')
    expect(filaEs.getAttribute('aria-pressed')).toBe('false')
  })

  it('pulsar una categoría escribe filtros.categoria', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoCategoria = screen.getByRole('group', { name: t('filtro.categoria') })
    await fireEvent.click(within(grupoCategoria).getByRole('button', { name: /Deportes/ }))
    expect(get(filtros).categoria).toBe('Deportes')
  })

  it('pulsar una calidad escribe filtros.calidad', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoCalidad = screen.getByRole('group', { name: t('filtro.calidad') })
    await fireEvent.click(filaCalidadHd(grupoCalidad))
    expect(get(filtros).calidad).toBe('hd')
  })
})

describe('BarraLateralFacetas — bloque Señal', () => {
  it('«Mostrar los que no responden» escribe mostrarOffline=true, como promete su etiqueta', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const casilla = screen.getByLabelText(t('filtro.mostrarOffline')) as HTMLInputElement
    expect(casilla.checked).toBe(false)
    expect(get(filtros).mostrarOffline).toBe(false)

    await fireEvent.click(casilla)

    expect(casilla.checked).toBe(true)
    expect(get(filtros).mostrarOffline).toBe(true)
  })

  it('«Solo señal viva» vuelve a poner mostrarOffline=false', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    filtros.update((f) => ({ ...f, mostrarOffline: true }))

    const soloViva = screen.getByRole('button', { name: t('senal.soloViva') })
    await fireEvent.click(soloViva)

    expect(get(filtros).mostrarOffline).toBe(false)
  })
})

describe('BarraLateralFacetas — búsqueda con debounce', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('no escribe en el store hasta 300ms después de la última tecla, y solo con el último valor', async () => {
    vi.useFakeTimers()
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const campo = screen.getByLabelText(t('catalogo.buscar')) as HTMLInputElement

    await fireEvent.input(campo, { target: { value: 'b' } })
    vi.advanceTimersByTime(100)
    await fireEvent.input(campo, { target: { value: 'bb' } })
    vi.advanceTimersByTime(100)
    await fireEvent.input(campo, { target: { value: 'bbc' } })

    vi.advanceTimersByTime(299)
    expect(get(filtros).q).toBe('')

    vi.advanceTimersByTime(1)
    expect(get(filtros).q).toBe('bbc')
  })
})

describe('BarraLateralFacetas — país largo', () => {
  it('colapsa la lista de país y «Ver los N países» la expande', async () => {
    const paisesLargos: Faceta[] = Array.from({ length: 15 }, (_, i) => ({ valor: `P${i}`, total: i + 1 }))
    render(BarraLateralFacetas, { paises: paisesLargos, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    expect(within(grupoPais).queryByRole('button', { name: /P14/ })).toBeNull()

    const verMas = within(grupoPais).getByRole('button', { name: t('filtro.verPaises', { n: 15 }) })
    await fireEvent.click(verMas)

    expect(within(grupoPais).queryByRole('button', { name: /P14/ })).not.toBeNull()
  })
})
