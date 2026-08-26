import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import ContinuarViendo from './ContinuarViendo.svelte'
import { historial } from '../estado/historial'
import type { Canal } from '../datos/catalogo'
import { idioma } from '../i18n'

// Consume el store SINGLETON `historial` (no crearHistorial() inyectado):
// es lo mismo que hace App, y lo que este componente importa de verdad.
// Se limpia en cada test para que ninguno herede el historial del anterior.
beforeEach(() => {
  historial.borrar()
  idioma.actual = 'es'
})

function canal(id: string, nombre = `Canal ${id}`): Canal {
  return {
    id,
    nombre,
    logoUrl: '',
    categoriaId: '',
    idioma: 'es',
    pais: '',
    vivo: null,
    latenciaMs: 0,
    webOk: null,
  }
}

describe('ContinuarViendo', () => {
  it('sin historial no renderiza nada (ni contenedor)', () => {
    const { container } = render(ContinuarViendo, { alAbrir: () => {} })
    // Svelte 5 deja un comentario ancla para el bloque {#if} vacío — no es
    // contenido: ningún elemento real, ningún botón, ningún texto.
    expect(container.querySelector('*')).toBeNull()
  })

  it('con historial: muestra el más reciente en grande + CTA "Seguir viendo", y miniaturas de los demás', () => {
    historial.registrar(canal('a', 'Canal A'))
    historial.registrar(canal('b', 'Canal B'))
    historial.registrar(canal('c', 'Canal C'))

    render(ContinuarViendo, { alAbrir: () => {} })

    // El más reciente (el último registrado) se ve en grande.
    expect(screen.getByText('Canal C')).toBeTruthy()
    expect(screen.getByText(/Seguir viendo/)).toBeTruthy()

    // Los otros dos aparecen como miniaturas clicables con nombre accesible.
    expect(screen.getByRole('button', { name: 'Canal A' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Canal B' })).toBeTruthy()
  })

  it('clic en una miniatura llama a alAbrir con esa entrada del historial', async () => {
    historial.registrar(canal('a', 'Canal A'))
    historial.registrar(canal('b', 'Canal B'))
    const alAbrir = vi.fn()
    render(ContinuarViendo, { alAbrir })

    await fireEvent.click(screen.getByRole('button', { name: 'Canal A' }))

    expect(alAbrir).toHaveBeenCalledTimes(1)
    expect(alAbrir).toHaveBeenCalledWith(expect.objectContaining({ canalId: 'a', nombre: 'Canal A' }))
  })

  it('clic en la entrada principal llama a alAbrir con la más reciente', async () => {
    historial.registrar(canal('a', 'Canal A'))
    historial.registrar(canal('b', 'Canal B'))
    const alAbrir = vi.fn()
    render(ContinuarViendo, { alAbrir })

    await fireEvent.click(screen.getByText(/Seguir viendo/))

    expect(alAbrir).toHaveBeenCalledWith(expect.objectContaining({ canalId: 'b', nombre: 'Canal B' }))
  })

  it('"Borrar historial" vacía el store y el héroe deja de renderizarse', async () => {
    historial.registrar(canal('a'))
    const { container } = render(ContinuarViendo, { alAbrir: () => {} })

    await fireEvent.click(screen.getByRole('button', { name: 'Borrar historial' }))

    expect(get(historial)).toEqual([])
    expect(container.querySelector('*')).toBeNull()
  })
})
