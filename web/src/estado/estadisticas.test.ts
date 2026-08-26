import { afterEach, describe, expect, it, vi } from 'vitest'
import { reportarDesenlace } from './estadisticas'

afterEach(() => vi.unstubAllGlobals())

describe('reportarDesenlace', () => {
  it('hace POST del desenlace a /stats/playback', async () => {
    // Firma variádica: sin esto, TS infiere una tupla de longitud 0 para los
    // argumentos de la llamada y `.mock.calls[0]` no compila (mismo patrón
    // que datos/http.test.ts).
    const espia = vi.fn(async (..._args: unknown[]) => new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', espia)
    reportarDesenlace({ canalId: 'c1', resultado: 'iniciado', motor: 'hlsjs', via: 'proxy', mirrorIndex: 0 }, '')
    // best-effort: no await; comprobamos que se llamó a fetch con el body correcto.
    expect(espia).toHaveBeenCalledOnce()
    const [url, opts] = espia.mock.calls[0]
    expect(String(url)).toContain('/stats/playback')
    expect(JSON.parse((opts as RequestInit).body as string).canal_id).toBe('c1')
  })

  it('un fallo de red no lanza (best-effort)', () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('down') }))
    expect(() => reportarDesenlace({ canalId: 'c1', resultado: 'fallo', motor: 'hlsjs', via: 'directo', mirrorIndex: 0 }, '')).not.toThrow()
  })
})
