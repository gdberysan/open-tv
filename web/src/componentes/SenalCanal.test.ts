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
})
