import { beforeEach, describe, expect, it } from 'vitest'
import { get } from 'svelte/store'
import { crearFavoritos } from './favoritos'

beforeEach(() => localStorage.clear())

describe('favoritos', () => {
  it('alterna y persiste', () => {
    const f = crearFavoritos()
    f.alternar('bbc')
    expect(get(f).has('bbc')).toBe(true)

    // Una instancia nueva lee lo guardado: es lo que pasa al recargar.
    expect(get(crearFavoritos()).has('bbc')).toBe(true)

    f.alternar('bbc')
    expect(get(crearFavoritos()).has('bbc')).toBe(false)
  })

  // Un localStorage con basura no puede dejar la app en blanco.
  it('sobrevive a datos corruptos', () => {
    localStorage.setItem('opentv.favoritos', '{no es json')
    expect(get(crearFavoritos()).size).toBe(0)
  })
})
