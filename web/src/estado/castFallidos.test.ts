import { beforeEach, describe, expect, it } from 'vitest'
import { noCasteaPorFormato, marcarFalloFormato } from './castFallidos'

beforeEach(() => localStorage.clear())

describe('castFallidos', () => {
  it('un canal nunca marcado no está en la memoria', () => {
    expect(noCasteaPorFormato('bbc')).toBe(false)
  })

  it('marcar y persistir', () => {
    marcarFalloFormato('bbc')
    expect(noCasteaPorFormato('bbc')).toBe(true)
    // Otra "instancia" (el módulo es sin estado propio, lee localStorage
    // directamente) ve lo mismo — es lo que pasa al recargar la página.
    expect(noCasteaPorFormato('bbc')).toBe(true)
  })

  it('marcar dos veces el mismo canal no duplica ni rompe nada', () => {
    marcarFalloFormato('bbc')
    marcarFalloFormato('bbc')
    expect(noCasteaPorFormato('bbc')).toBe(true)
  })

  it('canales distintos no se pisan', () => {
    marcarFalloFormato('bbc')
    expect(noCasteaPorFormato('itv')).toBe(false)
  })

  // Un localStorage con basura no puede tumbar el intento de castear.
  it('sobrevive a datos corruptos', () => {
    localStorage.setItem('opentv.cast.sinFormato', '{no es json')
    expect(noCasteaPorFormato('bbc')).toBe(false)
    expect(() => marcarFalloFormato('bbc')).not.toThrow()
  })
})
