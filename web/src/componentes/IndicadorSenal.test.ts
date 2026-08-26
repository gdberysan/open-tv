import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import IndicadorSenal from './IndicadorSenal.svelte'
import { idioma } from '../i18n'

// Falsable (brief Tarea 5, Paso 1): un componente que solo cambiara el color
// del punto pasaría un assert que solo mirara data-estado/clase, pero
// fallaría este, porque exige un TEXTO accesible distinto por estado — no
// basta con el color para comunicar la diferencia.
describe('IndicadorSenal', () => {
  // El idioma inicial de idioma.svelte.ts sale de navigator.language (jsdom
  // suele reportar en-US); se fija a 'es' para que las cadenas esperadas no
  // dependan del entorno de test (mismo patrón que App.integracion.test.ts).
  beforeEach(() => {
    idioma.actual = 'es'
  })

  it('vivo: punto con clase de señal ok y texto "Comprobado en vivo"', () => {
    const { container } = render(IndicadorSenal, { estado: 'vivo' })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('vivo')).toBe(true)
    expect(screen.getByText('Comprobado en vivo')).toBeTruthy()
  })

  it('sincronizando: punto con clase ámbar y texto distinto de "vivo"', () => {
    const { container } = render(IndicadorSenal, { estado: 'sincronizando' })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('sincronizando')).toBe(true)
    expect(screen.getByText('Sincronizando…')).toBeTruthy()
    expect(screen.queryByText('Comprobado en vivo')).toBeNull()
  })

  it('sin-gateway: punto apagado y texto distinto de los otros dos', () => {
    const { container } = render(IndicadorSenal, { estado: 'sin-gateway' })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('sin-gateway')).toBe(true)
    expect(screen.getByText('Sin conexión con Open TV')).toBeTruthy()
    expect(screen.queryByText('Comprobado en vivo')).toBeNull()
    expect(screen.queryByText('Sincronizando…')).toBeNull()
  })

  it('cada estado produce un texto único (el color por sí solo no basta)', () => {
    const textos = (['vivo', 'sincronizando', 'sin-gateway'] as const).map((estado) => {
      const { container, unmount } = render(IndicadorSenal, { estado })
      const texto = container.querySelector('.texto')?.textContent
      unmount()
      return texto
    })
    expect(new Set(textos).size).toBe(3)
  })
})
