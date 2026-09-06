import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reportarDesenlaceMirror } from './desenlaces'
import type { DesenlaceReproduccion } from '../reproductor/failover'

const base: DesenlaceReproduccion = {
  canalId: 'c1', resultado: 'fallo', motivo: 'desconocido', motor: 'hlsjs', via: 'proxy', mirrorIndex: 0, url: 'http://o/x.m3u8',
}
let espia: ReturnType<typeof vi.fn>
beforeEach(() => {
  espia = vi.fn(async (..._args: unknown[]) => new Response(null, { status: 204 }))
  vi.stubGlobal('fetch', espia)
  Object.defineProperty(navigator, 'onLine', { value: true, configurable: true })
})
afterEach(() => vi.unstubAllGlobals())

describe('reportarDesenlaceMirror', () => {
  it('reporta un fallo con su url y motivo', () => {
    expect(reportarDesenlaceMirror(base, '')).toBe(true)
    const [url, opts] = espia.mock.calls[0]
    expect(String(url)).toContain('/streams/desenlace')
    expect(JSON.parse((opts as RequestInit).body as string)).toEqual({ url: 'http://o/x.m3u8', resultado: 'fallo', motivo: 'desconocido', ms_primer_frame: 0 })
  })
  it('reporta un éxito con ms_primer_frame entero', () => {
    expect(reportarDesenlaceMirror({ ...base, resultado: 'iniciado', motivo: undefined, msPrimerFrame: 2100.6 }, '')).toBe(true)
    expect(JSON.parse((espia.mock.calls[0][1] as RequestInit).body as string).ms_primer_frame).toBe(2101)
  })
  it.each([
    ['sin intento (via ninguna)', { ...base, via: 'ninguna' as const }],
    ['motor forzado (cast)', { ...base, motorForzado: true }],
    ['pestaña oculta durante el intento', { ...base, oculto: true }],
    ['corte tras verse', { ...base, resultado: 'cortado' as const }],
    ['sin url', { ...base, url: '' }],
  ])('NO reporta: %s', (_n, d) => {
    expect(reportarDesenlaceMirror(d, '')).toBe(false)
    expect(espia).not.toHaveBeenCalled()
  })
  it('NO reporta sin conexión', () => {
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true })
    expect(reportarDesenlaceMirror(base, '')).toBe(false)
  })
  it('nunca lanza aunque fetch reviente', () => {
    vi.stubGlobal('fetch', vi.fn(() => { throw new Error('boom') }))
    expect(() => reportarDesenlaceMirror(base, '')).not.toThrow()
  })
})
