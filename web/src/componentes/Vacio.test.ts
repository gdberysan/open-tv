import { beforeEach, describe, expect, it } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import Vacio from './Vacio.svelte'
import { filtros } from '../estado/filtros'
import { idioma } from '../i18n'

// Mismo patrón de reset que ChipsFiltro.test.ts: el store `filtros` es
// compartido por módulo, así que cada test arranca desde cero.
beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
  idioma.actual = 'es'
})

describe('Vacio — sugerencia derivada del filtro más restrictivo', () => {
  // Test que falla primero (Tarea 13, Step 1): contra un vacío estático sin
  // sugerencia ("Ningún canal casa con el filtro." a secas, sin más
  // botones que quizá "Limpiar filtros"), este test busca un botón "Quitar
  // 4K" que solo existe si el componente deriva la sugerencia de
  // $filtros.calidad. Clicarlo debe limpiar SOLO esa dimensión.
  it('con calidad=4k activo, ofrece "Quitar 4K" y una CTA aparte "Limpiar filtros"', async () => {
    filtros.update((f) => ({ ...f, calidad: '4k', pais: 'México' }))
    render(Vacio)

    const botonQuitar4k = screen.getByRole('button', { name: 'Quitar 4K' })
    expect(screen.getByRole('button', { name: 'Limpiar filtros' })).toBeTruthy()

    await fireEvent.click(botonQuitar4k)

    expect(get(filtros).calidad).toBe('')
    expect(get(filtros).pais).toBe('México') // la dimensión menos restrictiva no se toca
  })

  it('heurística calidad > categoría > país > q: con las cuatro activas, prioriza calidad', () => {
    filtros.update((f) => ({ ...f, calidad: 'hd', categoria: 'Noticias', pais: 'México', q: 'rock' }))
    render(Vacio)

    expect(screen.getByRole('button', { name: 'Quitar HD (720p o más)' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Quitar Noticias' })).toBeNull()
  })

  it('sin calidad, prioriza categoría sobre país y q', () => {
    filtros.update((f) => ({ ...f, categoria: 'Noticias', pais: 'México', q: 'rock' }))
    render(Vacio)

    expect(screen.getByRole('button', { name: 'Quitar Noticias' })).toBeTruthy()
  })

  it('solo país activo: ofrece quitar ese país', () => {
    filtros.update((f) => ({ ...f, pais: 'México' }))
    render(Vacio)

    expect(screen.getByRole('button', { name: 'Quitar México' })).toBeTruthy()
  })

  it('solo búsqueda de texto activa: CTA distinta, entre comillas', async () => {
    filtros.update((f) => ({ ...f, q: 'rock' }))
    render(Vacio)

    const boton = screen.getByRole('button', { name: 'Quitar la búsqueda «rock»' })
    await fireEvent.click(boton)
    expect(get(filtros).q).toBe('')
  })

  it('sin ningún filtro de faceta activo (caso límite: offline oculta todo), ofrece Mostrar los que no responden', async () => {
    render(Vacio)

    const boton = screen.getByRole('button', { name: 'Mostrar los que no responden' })
    await fireEvent.click(boton)
    expect(get(filtros).mostrarOffline).toBe(true)
  })

  it('"Limpiar filtros" resetea las cuatro dimensiones más soloFavoritos', async () => {
    filtros.update((f) => ({ ...f, calidad: '4k', pais: 'México', categoria: 'Noticias', q: 'rock', soloFavoritos: true }))
    render(Vacio)

    await fireEvent.click(screen.getByRole('button', { name: 'Limpiar filtros' }))

    const estado = get(filtros)
    expect(estado.calidad).toBe('')
    expect(estado.pais).toBe('')
    expect(estado.categoria).toBe('')
    expect(estado.q).toBe('')
    expect(estado.soloFavoritos).toBe(false)
  })
})
