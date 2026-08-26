import { beforeEach, describe, expect, it } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import ChipsFiltro from './ChipsFiltro.svelte'
import { filtros } from '../estado/filtros'
import { idioma } from '../i18n'

// Patrón establecido en P0.5/P0.6: el reset vive a nivel de fichero, no
// anidado en cada describe — así ningún test hereda el store mutado por el
// anterior. idioma.actual se fija a 'es' (como App.integracion.test.ts):
// las aserciones comparan texto en español y el idioma real detectado del
// entorno de jsdom no puede decidir eso por su cuenta.
beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
  idioma.actual = 'es'
})

describe('ChipsFiltro — un chip removible por filtro activo', () => {
  it('pinta un chip por cada dimensión activa; quitar uno solo limpia esa dimensión', async () => {
    filtros.update((f) => ({ ...f, pais: 'México', categoria: 'Noticias' }))
    render(ChipsFiltro)

    expect(screen.getAllByRole('button')).toHaveLength(2)

    const chipPais = screen.getByRole('button', { name: 'Quitar filtro México' })
    expect(screen.getByRole('button', { name: 'Quitar filtro Noticias' })).toBeTruthy()

    await fireEvent.click(chipPais)

    expect(get(filtros).pais).toBe('')
    expect(get(filtros).categoria).toBe('Noticias') // la otra dimensión no se toca
    expect(screen.getAllByRole('button')).toHaveLength(1)
  })

  it('no pinta nada cuando no hay ningún filtro activo', () => {
    render(ChipsFiltro)
    expect(screen.queryAllByRole('button')).toHaveLength(0)
  })

  it('soloFavoritos y la búsqueda también generan su propio chip removible', async () => {
    filtros.update((f) => ({ ...f, q: 'rock', soloFavoritos: true }))
    render(ChipsFiltro)

    expect(screen.getAllByRole('button')).toHaveLength(2)
    const chipFavoritos = screen.getByRole('button', { name: 'Quitar filtro Solo favoritos' })
    await fireEvent.click(chipFavoritos)
    expect(get(filtros).soloFavoritos).toBe(false)

    const chipBusqueda = screen.getByRole('button', { name: 'Quitar búsqueda «rock»' })
    await fireEvent.click(chipBusqueda)
    expect(get(filtros).q).toBe('')
  })
})
