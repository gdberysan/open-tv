import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { get } from 'svelte/store'
import { crearEpg } from './epg'
import type { AhoraDespues, CatalogSource } from '../datos/catalogo'

// Doble mínimo: el store solo llama a epgDeCanales, así que es lo único que
// hace falta implementar aquí (a diferencia del doble completo de
// App.integracion.test.ts).
function fuenteFalsa(epgDeCanales: CatalogSource['epgDeCanales']): CatalogSource {
  return {
    canales: vi.fn(),
    paises: vi.fn(),
    categorias: vi.fn(),
    calidades: vi.fn(),
    aleatorio: vi.fn(),
    destino: vi.fn(),
    mirrors: vi.fn(),
    frescura: vi.fn(),
    proxyDisponible: vi.fn(),
    fuentes: vi.fn(),
    anadirFuente: vi.fn(),
    anadirFuenteFichero: vi.fn(),
    quitarFuente: vi.fn(),
    resyncFuente: vi.fn(),
    fuentesSugeridas: vi.fn(),
    epgDeCanales,
    epgDeCanal: vi.fn(async () => ({ ahora: null, proximos: [] })),
  } as CatalogSource
}

const PROGRAMA_A: AhoraDespues = {
  ahora: { titulo: 'Noticias A', inicioSeg: 1000, finSeg: 2000 },
  siguiente: null,
}
const PROGRAMA_B: AhoraDespues = {
  ahora: { titulo: 'Noticias B', inicioSeg: 1000, finSeg: 2000 },
  siguiente: { titulo: 'Después B', inicioSeg: 2000, finSeg: 3000 },
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('crearEpg — coalescencia y TTL', () => {
  it('asegurar([a,b]) hace UNA petición en lote, no una por id', async () => {
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A], ['b', PROGRAMA_B]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales))

    await epg.asegurar(['a', 'b'])

    expect(epgDeCanales).toHaveBeenCalledOnce()
    expect(epgDeCanales).toHaveBeenCalledWith(['a', 'b'])
    expect(epg.ahoraDespuesDe('a')).toEqual(PROGRAMA_A)
    expect(epg.ahoraDespuesDe('b')).toEqual(PROGRAMA_B)

    epg.detener()
  })

  it('una segunda llamada dentro del TTL no re-pide', async () => {
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales))

    await epg.asegurar(['a'])
    await epg.asegurar(['a'])
    await epg.asegurar(['a'])

    expect(epgDeCanales).toHaveBeenCalledOnce()
    epg.detener()
  })

  it('pasado el TTL, re-pide', async () => {
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales), { ttlMs: 5 * 60_000, intervaloMs: 10 * 60_000 })

    await epg.asegurar(['a'])
    expect(epgDeCanales).toHaveBeenCalledOnce()

    vi.advanceTimersByTime(5 * 60_000 + 1)
    await epg.asegurar(['a'])

    expect(epgDeCanales).toHaveBeenCalledTimes(2)
    epg.detener()
  })

  it('solo re-pide los ids que faltan o cuyo TTL venció, no el lote entero', async () => {
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A], ['b', PROGRAMA_B]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales))

    await epg.asegurar(['a', 'b'])
    await epg.asegurar(['a', 'b', 'c'])

    expect(epgDeCanales).toHaveBeenCalledTimes(2)
    expect(epgDeCanales).toHaveBeenNthCalledWith(2, ['c'])
    epg.detener()
  })

  it('un id ausente en la respuesta deja ahoraDespuesDe(id) en undefined, sin romper', async () => {
    // "a" viene en la respuesta, "b" no (sin guía posible).
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales))

    await expect(epg.asegurar(['a', 'b'])).resolves.not.toThrow()

    expect(epg.ahoraDespuesDe('a')).toEqual(PROGRAMA_A)
    expect(epg.ahoraDespuesDe('b')).toBeUndefined()
    epg.detener()
  })

  it('un fallo de la fuente no lanza; los ids quedan sin dato', async () => {
    const epgDeCanales = vi.fn(async () => { throw new Error('gateway inalcanzable') })
    const epg = crearEpg(fuenteFalsa(epgDeCanales))

    await expect(epg.asegurar(['a'])).resolves.not.toThrow()
    expect(epg.ahoraDespuesDe('a')).toBeUndefined()
    epg.detener()
  })

  it('el store (Readable) refleja el mismo contenido que ahoraDespuesDe', async () => {
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales))

    await epg.asegurar(['a'])

    const snapshot = get(epg)
    expect(snapshot.get('a')).toEqual(PROGRAMA_A)
    expect(snapshot.has('b')).toBe(false)
    epg.detener()
  })

  it('el intervalo refresca por sí solo el set visible sin llamada explícita', async () => {
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales), { ttlMs: 1000, intervaloMs: 2000 })

    await epg.asegurar(['a'])
    expect(epgDeCanales).toHaveBeenCalledOnce()

    // Tras el TTL (1s) y el tick del intervalo (2s), debe haber re-pedido
    // "a" sin que nadie vuelva a llamar a asegurar() a mano.
    await vi.advanceTimersByTimeAsync(2000)

    expect(epgDeCanales).toHaveBeenCalledTimes(2)
    epg.detener()
  })

  it('detener() para el intervalo: ya no vuelve a pedir por su cuenta', async () => {
    const epgDeCanales = vi.fn(async () => new Map([['a', PROGRAMA_A]]))
    const epg = crearEpg(fuenteFalsa(epgDeCanales), { ttlMs: 1000, intervaloMs: 2000 })

    await epg.asegurar(['a'])
    epg.detener()

    await vi.advanceTimersByTimeAsync(10_000)

    expect(epgDeCanales).toHaveBeenCalledOnce()
  })
})
