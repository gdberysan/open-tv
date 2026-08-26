import { beforeEach, describe, expect, it } from 'vitest'
import {
  borrarFiltrosGuardados,
  borrarVistaGuardada,
  escribirFiltrosGuardados,
  escribirVistaGuardada,
  leerFiltrosGuardados,
  leerVistaGuardada,
} from './persistenciaSesion'

beforeEach(() => localStorage.clear())

describe('persistenciaSesion — vista', () => {
  it('escribe y relee la vista guardada', () => {
    expect(leerVistaGuardada()).toBeNull()
    escribirVistaGuardada('lista')
    expect(leerVistaGuardada()).toBe('lista')
  })

  it('borrar deja sin vista guardada', () => {
    escribirVistaGuardada('lista')
    borrarVistaGuardada()
    expect(leerVistaGuardada()).toBeNull()
  })

  it('un valor corrupto o fuera del enum cae a null, no tumba la lectura', () => {
    localStorage.setItem('opentv.vista', '"gigante"')
    expect(leerVistaGuardada()).toBeNull()
    localStorage.setItem('opentv.vista', '{no es json')
    expect(leerVistaGuardada()).toBeNull()
  })
})

describe('persistenciaSesion — filtros', () => {
  const snapshot = { q: 'bbc', pais: 'GB', categoria: 'news', calidad: 'hd', soloFavoritos: true }

  it('escribe y relee los filtros guardados', () => {
    expect(leerFiltrosGuardados()).toBeNull()
    escribirFiltrosGuardados(snapshot)
    expect(leerFiltrosGuardados()).toEqual(snapshot)
  })

  it('borrar deja sin filtros guardados', () => {
    escribirFiltrosGuardados(snapshot)
    borrarFiltrosGuardados()
    expect(leerFiltrosGuardados()).toBeNull()
  })

  it('una clave con forma inválida cae a su propio valor por defecto, sin tumbar las demás', () => {
    localStorage.setItem(
      'opentv.filtros',
      JSON.stringify({ q: 'bbc', pais: 42, categoria: 'news', calidad: 'hd', soloFavoritos: 'sí' }),
    )
    expect(leerFiltrosGuardados()).toEqual({ q: 'bbc', pais: '', categoria: 'news', calidad: 'hd', soloFavoritos: false })
  })

  it('basura en localStorage cae a null, no tumba la lectura', () => {
    localStorage.setItem('opentv.filtros', '{no es json')
    expect(leerFiltrosGuardados()).toBeNull()
  })
})
