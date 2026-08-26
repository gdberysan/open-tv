import { describe, expect, it } from 'vitest'
import { banderaDePais, nombreDePais } from './paises'

describe('nombreDePais', () => {
  it('traduce un código ISO conocido al nombre en el locale pedido', () => {
    expect(nombreDePais('MX', 'es')).toBe('México')
    expect(nombreDePais('MX', 'en')).toBe('Mexico')
  })

  it('devuelve el código tal cual si no es una región reconocida', () => {
    // Minúsculas: forma bien formada mecánicamente, pero sin traducción en
    // el CLDR — el motor la devuelve sin cambios, no lanza.
    expect(nombreDePais('mx', 'es')).toBe('mx')
  })

  it('devuelve el código tal cual si Intl.DisplayNames#of lanza (código malformado)', () => {
    // Menos de dos letras: Intl.DisplayNames#of lanza RangeError.
    expect(nombreDePais('M', 'es')).toBe('M')
  })

  it('devuelve el código vacío tal cual, sin lanzar', () => {
    expect(nombreDePais('', 'es')).toBe('')
  })

  it('no lanza al pedir el mismo locale muchas veces (instancia cacheada)', () => {
    expect(() => {
      for (let i = 0; i < 50; i++) nombreDePais('US', 'es')
    }).not.toThrow()
    expect(nombreDePais('US', 'es')).toBe('Estados Unidos')
  })
})

describe('banderaDePais', () => {
  it('convierte un código ISO de dos letras a su bandera emoji', () => {
    expect(banderaDePais('US')).toBe('🇺🇸')
    expect(banderaDePais('MX')).toBe('🇲🇽')
  })

  it('acepta minúsculas y las normaliza', () => {
    expect(banderaDePais('es')).toBe('🇪🇸')
  })

  it('devuelve "" para cualquier cosa que no sean dos letras ASCII', () => {
    expect(banderaDePais('')).toBe('')
    expect(banderaDePais('U')).toBe('')
    expect(banderaDePais('USA')).toBe('')
    expect(banderaDePais('12')).toBe('')
  })
})
