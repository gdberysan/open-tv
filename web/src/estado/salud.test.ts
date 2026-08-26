import { afterEach, describe, expect, it, vi } from 'vitest'
import { clasificarError, consultarSalud } from './salud'

afterEach(() => vi.unstubAllGlobals())

describe('clasificarError', () => {
  // El síntoma en la UI es idéntico y el problema no lo es: uno se arregla
  // abriendo Open TV, el otro mirando el wifi. Confundirlos costó una tarde.
  it('distingue gateway caído de falta de red', () => {
    expect(clasificarError(new Error('gateway inalcanzable'))).toBe('gateway')
    expect(clasificarError(new Error('sin red'))).toBe('red')
    expect(clasificarError(new Error('respuesta 500'))).toBe('servidor')
  })
})

describe('consultarSalud', () => {
  it('lee el catálogo como listo cuando hay sync', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify({ status: 'ok', db: 'ok', last_sync: '2026-08-25T10:00:00Z', web_ui: true, proxy_enabled: true }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )))

    const s = await consultarSalud('')
    expect(s.sincronizando).toBe(false)
    expect(s.proxyDisponible).toBe(true)
  })

  // last_sync null = primer arranque: el catálogo se está descargando. Sin
  // esto, la primera pantalla es una rejilla vacía que parece rota.
  it('sin sync todavía, está sincronizando', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(
      JSON.stringify({ status: 'ok', db: 'ok', last_sync: null, web_ui: true, proxy_enabled: true }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    )))

    expect((await consultarSalud('')).sincronizando).toBe(true)
  })
})
