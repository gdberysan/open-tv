import { beforeEach, describe, expect, it } from 'vitest'
import { get } from 'svelte/store'
import { crearPreferencias } from './preferencias'

beforeEach(() => localStorage.clear())

describe('preferencias', () => {
  it('persiste la densidad entre instancias (lo que pasa al recargar)', () => {
    const p = crearPreferencias()
    expect(get(p).densidad).toBe('comoda') // por defecto

    p.actualizar({ densidad: 'compacta' })
    expect(get(p).densidad).toBe('compacta')

    // Una instancia nueva lee lo guardado: es lo que pasa al recargar.
    expect(get(crearPreferencias()).densidad).toBe('compacta')
  })

  // Un localStorage con basura no puede dejar la app en blanco.
  it('sobrevive a datos corruptos: cae al valor por defecto completo', () => {
    localStorage.setItem('opentv.preferencias', '{no es json')
    expect(get(crearPreferencias())).toEqual({
      densidad: 'comoda',
      recordarVista: true,
      recordarFiltros: false,
    })
  })

  // Una clave con forma inesperada (tipo equivocado, valor fuera del enum) no
  // puede tumbar las DEMÁS claves — cada una se valida por su cuenta.
  it('una clave con forma inválida cae a su propio valor por defecto, sin tocar las demás', () => {
    localStorage.setItem(
      'opentv.preferencias',
      JSON.stringify({ densidad: 'gigante', recordarVista: false, recordarFiltros: 'sí' }),
    )
    expect(get(crearPreferencias())).toEqual({
      densidad: 'comoda',
      recordarVista: false,
      recordarFiltros: false,
    })
  })

  it('es reactivo: un subscribe ve el cambio sin releer localStorage a mano', () => {
    const p = crearPreferencias()
    let vista: Array<'comoda' | 'compacta'> = []
    const detener = p.subscribe((v) => vista.push(v.densidad))

    p.actualizar({ densidad: 'compacta' })

    expect(vista).toEqual(['comoda', 'compacta'])
    detener()
  })
})
