import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import BarraFiltros from './BarraFiltros.svelte'
import { filtros } from '../estado/filtros'
import { t } from '../i18n'
import type { Faceta } from '../datos/catalogo'

// Ronda 1 de revisión: la casilla decía "Ocultar los que no responden" pero
// bind:checked iba directo sobre mostrarOffline, cuyo `true` en http.ts
// manda ?alive=all — es decir, REVELA los muertos, no los oculta. La
// etiqueta y el store decían lo contrario. Este test fija la promesa: lo que
// la etiqueta dice es lo que el store hace.
beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
})

describe('BarraFiltros — casilla de canales que no responden', () => {
  it('marcarla escribe mostrarOffline=true, como promete "Mostrar"', async () => {
    render(BarraFiltros, { paises: [], categorias: [], alAleatorio: () => {} })

    const casilla = screen.getByLabelText(t('filtro.mostrarOffline')) as HTMLInputElement
    expect(casilla.checked).toBe(false)
    expect(get(filtros).mostrarOffline).toBe(false)

    await fireEvent.click(casilla)

    expect(casilla.checked).toBe(true)
    expect(get(filtros).mostrarOffline).toBe(true)
  })
})

describe('BarraFiltros — búsqueda con debounce', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('no escribe en el store hasta 300ms después de la última tecla, y solo con el último valor', async () => {
    vi.useFakeTimers()
    render(BarraFiltros, { paises: [], categorias: [], alAleatorio: () => {} })

    const campo = screen.getByLabelText(t('catalogo.buscar')) as HTMLInputElement

    await fireEvent.input(campo, { target: { value: 'b' } })
    vi.advanceTimersByTime(100)
    await fireEvent.input(campo, { target: { value: 'bb' } })
    vi.advanceTimersByTime(100)
    await fireEvent.input(campo, { target: { value: 'bbc' } })

    // Cada tecla reinicia el temporizador: a los 299ms de la última, nada.
    vi.advanceTimersByTime(299)
    expect(get(filtros).q).toBe('')

    // Al milisegundo 300, una sola escritura, con el último valor tecleado.
    vi.advanceTimersByTime(1)
    expect(get(filtros).q).toBe('bbc')
  })
})

describe('BarraFiltros — facetas', () => {
  it('puebla país y categoría desde las facetas recibidas, como "valor (total)"', () => {
    const paises: Faceta[] = [{ valor: 'ES', total: 12 }, { valor: 'MX', total: 7 }]
    const categorias: Faceta[] = [{ valor: 'Deportes', total: 3 }]

    render(BarraFiltros, { paises, categorias, alAleatorio: () => {} })

    const selectorPais = screen.getByLabelText(t('filtro.pais')) as HTMLSelectElement
    const opcionesPais = Array.from(selectorPais.options).map((o) => o.textContent)
    expect(opcionesPais).toContain('ES (12)')
    expect(opcionesPais).toContain('MX (7)')

    const selectorCategoria = screen.getByLabelText(t('filtro.categoria')) as HTMLSelectElement
    const opcionesCategoria = Array.from(selectorCategoria.options).map((o) => o.textContent)
    expect(opcionesCategoria).toContain('Deportes (3)')
  })
})

describe('BarraFiltros — botones', () => {
  it('"Canal al azar" invoca el callback recibido por props', async () => {
    const alAleatorio = vi.fn()
    render(BarraFiltros, { paises: [], categorias: [], alAleatorio })

    await fireEvent.click(screen.getByText(t('accion.aleatorio')))

    expect(alAleatorio).toHaveBeenCalledOnce()
  })

  it('el conmutador rejilla/lista escribe filtros.vista', async () => {
    render(BarraFiltros, { paises: [], categorias: [], alAleatorio: () => {} })

    expect(get(filtros).vista).toBe('rejilla')

    await fireEvent.click(screen.getByText(t('accion.lista')))
    expect(get(filtros).vista).toBe('lista')

    await fireEvent.click(screen.getByText(t('accion.rejilla')))
    expect(get(filtros).vista).toBe('rejilla')
  })
})
