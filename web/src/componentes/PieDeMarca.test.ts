import { beforeEach, describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import PieDeMarca from './PieDeMarca.svelte'
import { idioma } from '../i18n'

// Mismo patrón que IndicadorSenal.test.ts / ContinuarViendo.test.ts: se fija
// 'es' porque idioma.actual arranca de navigator.language (jsdom -> en-US),
// y las cadenas esperadas aquí son las españolas.
describe('PieDeMarca', () => {
  beforeEach(() => {
    idioma.actual = 'es'
  })

  it('pinta el emblema de Korven con alt no vacío', () => {
    render(PieDeMarca)
    const img = screen.getByRole('img', { name: /korven/i })
    expect(img.getAttribute('src')).toBe('/marca/korven-emblema.svg')
    expect(img.getAttribute('alt')).toBeTruthy()
  })

  it('pinta el texto de crédito "Desarrollado por … Korven … Claude Code"', () => {
    const { container } = render(PieDeMarca)
    const texto = container.textContent ?? ''
    expect(texto).toMatch(/Desarrollado por/)
    expect(texto).toMatch(/Korven/)
    expect(texto).toMatch(/Claude Code/)
  })

  it('enlaza Korven a korven.dev, en pestaña nueva y sin abrir opener', () => {
    render(PieDeMarca)
    const enlace = screen.getByRole('link', { name: 'Korven' })
    expect(enlace.getAttribute('href')).toBe('https://korven.dev')
    expect(enlace.getAttribute('target')).toBe('_blank')
    expect(enlace.getAttribute('rel')).toBe('noopener')
  })

  it('enlaza Claude Code a claude.com/claude-code, en pestaña nueva y sin abrir opener', () => {
    render(PieDeMarca)
    const enlace = screen.getByRole('link', { name: 'Claude Code' })
    expect(enlace.getAttribute('href')).toBe('https://claude.com/claude-code')
    expect(enlace.getAttribute('target')).toBe('_blank')
    expect(enlace.getAttribute('rel')).toBe('noopener')
  })

  it('enlaza discretamente al repositorio de GitHub, en pestaña nueva y sin abrir opener', () => {
    render(PieDeMarca)
    const enlaces = screen.getAllByRole('link')
    const repo = enlaces.find((a) => a.getAttribute('href') === 'https://github.com/gdberysan/open-tv')
    expect(repo).toBeTruthy()
    expect(repo?.getAttribute('target')).toBe('_blank')
    expect(repo?.getAttribute('rel')).toBe('noopener')
  })

  it('funciona igual en inglés (solo cambia el texto de "Desarrollado por")', () => {
    idioma.actual = 'en'
    const { container } = render(PieDeMarca)
    const texto = container.textContent ?? ''
    expect(texto).not.toMatch(/Desarrollado por/)
    expect(texto).toMatch(/Korven/)
    expect(texto).toMatch(/Claude Code/)
    expect(screen.getByRole('link', { name: 'Korven' })).toBeTruthy()
  })
})
