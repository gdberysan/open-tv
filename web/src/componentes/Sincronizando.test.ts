import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Sincronizando from './Sincronizando.svelte'
import { idioma, t } from '../i18n'

// Tarea 12 (P0.6): la barra de progreso es INDETERMINADA — el gateway no
// publica ningún porcentaje de avance, así que fingir un aria-valuenow sería
// mentir sobre lo que se sabe (misma razón que el comentario original del
// componente daba para no tener barra en absoluto). Este test es falsable:
// un componente que solo pintara una barra con aria-valuenow="50" fijo
// pasaría un assert ingenuo de "existe role=progressbar" pero fallaría el
// assert explícito de "sin aria-valuenow".
describe('Sincronizando — barra de progreso indeterminada', () => {
  beforeEach(() => {
    idioma.actual = 'es'
  })

  it('renderiza título, copy y una barra de progreso', () => {
    render(Sincronizando, { alListo: () => {} })
    expect(screen.getByText(t('estado.sincronizando'))).toBeTruthy()
    expect(screen.getByText(t('estado.sincronizandoDetalle'))).toBeTruthy()
    expect(screen.getByRole('progressbar')).toBeTruthy()
  })

  it('la barra es indeterminada: sin aria-valuenow (no hay porcentaje real que reportar)', () => {
    render(Sincronizando, { alListo: () => {} })
    const barra = screen.getByRole('progressbar')
    expect(barra.hasAttribute('aria-valuenow')).toBe(false)
  })

  // Invariante de P0.5 (ver también lib/a11y.test.ts, describe "un único
  // anunciador, sin doble anuncio"): App.svelte ya tiene su propia región
  // aria-live PERSISTENTE para este mismo texto. Si este componente volviera
  // a llevar aria-live/role="status"/role="alert" propios, un lector de
  // pantalla anunciaría "Sincronizando el catálogo…" DOS VECES seguidas.
  it('no lleva su propia semántica live (App.svelte es la única anunciadora)', () => {
    const { container } = render(Sincronizando, { alListo: () => {} })
    expect(container.querySelector('[aria-live]')).toBeNull()
    expect(container.querySelector('[role="status"]')).toBeNull()
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })
})
