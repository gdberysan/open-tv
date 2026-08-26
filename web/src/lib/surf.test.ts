import { describe, expect, it } from 'vitest'
import { debeHacerSurf } from './surf'

// Construye un KeyboardEvent con un target concreto: jsdom no deja asignar
// `target` en el constructor, así que se dispara sobre el propio elemento
// (el listener real de App también lee event.target, nunca currentTarget).
function eventoEn(elemento: EventTarget, init: KeyboardEventInit = { key: ' ' }): KeyboardEvent {
  const e = new KeyboardEvent('keydown', { ...init, bubbles: true, cancelable: true })
  Object.defineProperty(e, 'target', { value: elemento })
  return e
}

describe('debeHacerSurf', () => {
  it('true con Space sobre un div', () => {
    expect(debeHacerSurf(eventoEn(document.createElement('div')))).toBe(true)
  })

  it('true con Space sobre body', () => {
    expect(debeHacerSurf(eventoEn(document.body))).toBe(true)
  })

  it('true con code Space aunque key difiera', () => {
    const e = eventoEn(document.createElement('article'), { key: 'Unidentified', code: 'Space' })
    expect(debeHacerSurf(e)).toBe(true)
  })

  it('false con Space sobre un input', () => {
    expect(debeHacerSurf(eventoEn(document.createElement('input')))).toBe(false)
  })

  it('false con Space sobre un textarea', () => {
    expect(debeHacerSurf(eventoEn(document.createElement('textarea')))).toBe(false)
  })

  it('false con Space sobre un select', () => {
    expect(debeHacerSurf(eventoEn(document.createElement('select')))).toBe(false)
  })

  it('false con Space sobre un button', () => {
    expect(debeHacerSurf(eventoEn(document.createElement('button')))).toBe(false)
  })

  it('false con Space sobre un elemento contenteditable', () => {
    const div = document.createElement('div')
    div.setAttribute('contenteditable', 'true')
    // jsdom no calcula isContentEditable a partir del atributo: se fuerza
    // para probar la rama que sí lo comprueba en tiempo real de navegador.
    Object.defineProperty(div, 'isContentEditable', { value: true })
    expect(debeHacerSurf(eventoEn(div))).toBe(false)
  })

  it('false con otra tecla sobre un div', () => {
    expect(debeHacerSurf(eventoEn(document.createElement('div'), { key: 'a' }))).toBe(false)
  })
})
