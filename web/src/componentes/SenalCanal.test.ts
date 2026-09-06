import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import SenalCanal from './SenalCanal.svelte'
import { idioma } from '../i18n'

// El idioma inicial sale de navigator.language (jsdom suele reportar en-US);
// se fija a 'es' para que las etiquetas esperadas no dependan del entorno
// de test (mismo patrón que IndicadorSenal.test.ts).
beforeEach(() => {
  idioma.actual = 'es'
})

describe('SenalCanal', () => {
  it('vivo===true: punto con clase "vivo" y etiqueta "Señal viva"', () => {
    const { container } = render(SenalCanal, { vivo: true, latenciaMs: 120 })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('vivo')).toBe(true)
    expect(punto?.getAttribute('aria-label')).toBe('Señal viva')
  })

  it('vivo===false: punto con clase "muerta" y etiqueta "Sin respuesta"', () => {
    const { container } = render(SenalCanal, { vivo: false, latenciaMs: 0 })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('muerta')).toBe(true)
    expect(punto?.getAttribute('aria-label')).toBe('Sin respuesta')
  })

  it('vivo===null: punto con clase "desconocido" y etiqueta "Sin comprobar"', () => {
    const { container } = render(SenalCanal, { vivo: null, latenciaMs: 0 })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('desconocido')).toBe(true)
    expect(punto?.getAttribute('aria-label')).toBe('Sin comprobar')
  })

  it('muestra el ms cuando hay latencia', () => {
    render(SenalCanal, { vivo: true, latenciaMs: 120 })
    expect(screen.getByText('120 ms')).toBeTruthy()
  })

  it('omite el ms cuando no hay latencia', () => {
    render(SenalCanal, { vivo: true, latenciaMs: 0 })
    expect(screen.queryByText('0 ms')).toBeNull()
  })

  // Tarea 7 (tiempo-hasta-la-imagen): imagenMs/sinImagen son opcionales —
  // sin ellos la tarjeta sigue como hoy (tests de arriba).
  // M6 (revisión de rama completa): el texto visible ya dice el estado
  // entero en esta rama, así que el punto pasa a aria-hidden sin aria-label
  // — un lector de pantalla no debe oírlo dos veces.
  it('con imagenMs pinta «Imagen en 2,1 s» y el punto queda aria-hidden (el texto ya lo dice)', () => {
    const { container } = render(SenalCanal, { vivo: true, latenciaMs: 120, imagenMs: 2140 })
    expect(container.textContent).toContain('Imagen en 2,1 s')
    expect(container.querySelector('.punto')?.getAttribute('aria-hidden')).toBe('true')
    expect(container.querySelector('.punto')?.hasAttribute('aria-label')).toBe(false)
    expect(container.textContent).not.toContain('120 ms')
  })

  it('con sinImagen pinta el punto de error, aria-hidden y «Sin imagen desde aquí» (el texto ya lo dice)', () => {
    const { container } = render(SenalCanal, { vivo: true, latenciaMs: 120, sinImagen: true })
    expect(container.querySelector('.punto')?.classList.contains('sin-imagen')).toBe(true)
    expect(container.querySelector('.punto')?.getAttribute('aria-hidden')).toBe('true')
    expect(container.querySelector('.punto')?.hasAttribute('aria-label')).toBe(false)
    expect(container.textContent).toContain('Sin imagen desde aquí')
  })

  it('sin datos de imagen sigue como hoy', () => {
    const { container } = render(SenalCanal, { vivo: true, latenciaMs: 120 })
    expect(container.textContent).toContain('120 ms')
  })
})
