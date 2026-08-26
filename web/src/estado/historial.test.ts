import { beforeEach, describe, expect, it, vi } from 'vitest'
import { get } from 'svelte/store'
import { crearHistorial } from './historial'
import type { Canal } from '../datos/catalogo'

beforeEach(() => localStorage.clear())

function canal(id: string, nombre = `Canal ${id}`): Canal {
  return {
    id,
    nombre,
    logoUrl: `https://ejemplo.test/${id}.png`,
    categoriaId: 'general',
    idioma: 'es',
    pais: 'ES',
    vivo: true,
    latenciaMs: 100,
    webOk: true,
  }
}

describe('historial', () => {
  it('registrar añade al frente', () => {
    const h = crearHistorial()
    h.registrar(canal('a'))
    h.registrar(canal('b'))

    const lista = get(h)
    expect(lista.map((e) => e.canalId)).toEqual(['b', 'a'])
  })

  it('reabrir un canal ya presente lo mueve al frente sin duplicar', () => {
    const h = crearHistorial()
    h.registrar(canal('a'))
    h.registrar(canal('b'))
    h.registrar(canal('c'))
    h.registrar(canal('a'))

    const lista = get(h)
    expect(lista).toHaveLength(3)
    expect(lista.map((e) => e.canalId)).toEqual(['a', 'c', 'b'])
  })

  it('tope de 24: la entrada 25 expulsa la más antigua', () => {
    const h = crearHistorial()
    for (let i = 0; i < 25; i++) {
      h.registrar(canal(`c${i}`))
    }

    const lista = get(h)
    expect(lista).toHaveLength(24)
    // La más reciente (c24) al frente.
    expect(lista[0].canalId).toBe('c24')
    // La más antigua (c0) fue expulsada.
    expect(lista.some((e) => e.canalId === 'c0')).toBe(false)
    // c1 sigue siendo la más vieja que sobrevive.
    expect(lista[lista.length - 1].canalId).toBe('c1')
  })

  it('borrar() vacía', () => {
    const h = crearHistorial()
    h.registrar(canal('a'))
    h.registrar(canal('b'))
    h.borrar()

    expect(get(h)).toEqual([])
  })

  it('robustez: setItem que lanza no rompe registrar ni el subscribe', () => {
    const h = crearHistorial()
    h.registrar(canal('a'))

    const espia = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('almacenamiento bloqueado')
    })

    expect(() => h.registrar(canal('b'))).not.toThrow()

    // El store en memoria sigue devolviendo datos sanos aunque la escritura falle.
    const lista = get(h)
    expect(lista.map((e) => e.canalId)).toEqual(['b', 'a'])

    espia.mockRestore()
  })

  it('determinismo: usa el reloj inyectado para "cuando"', () => {
    const h = crearHistorial(() => 1000)
    h.registrar(canal('a'))

    expect(get(h)[0].cuando).toBe(1000)
  })
})
