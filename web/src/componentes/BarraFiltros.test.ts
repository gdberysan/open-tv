import { beforeEach, describe, expect, it } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import BarraFiltros from './BarraFiltros.svelte'
import { filtros } from '../estado/filtros'
import { t } from '../i18n'

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
