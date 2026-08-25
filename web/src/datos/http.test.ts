import { afterEach, describe, expect, it, vi } from 'vitest'
import { crearHttpCatalog } from './http'

function respuesta(cuerpo: unknown, cabeceras: Record<string, string> = {}) {
  return new Response(JSON.stringify(cuerpo), {
    status: 200,
    headers: { 'Content-Type': 'application/json', ...cabeceras },
  })
}

afterEach(() => vi.unstubAllGlobals())

describe('HttpCatalog', () => {
  // El mapeo de nombres vive en UN solo sitio. Las claves del gateway están
  // congeladas por un test de contrato en Go y son nombres de campo de Go;
  // dentro del cliente se usan nombres propios.
  it('traduce las claves congeladas del gateway', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta(
        [{
          ID: 'opensource-BBC One', Name: 'BBC One (1080p)', LogoURL: 'http://logo',
          CategoryID: 'General;News', LanguageCode: 'en', CountryCode: 'GB',
          Alive: true, LatencyMs: 120, WebOK: false, ProviderType: 'opensource',
        }],
        { 'X-Total-Count': '12639' },
      ),
    ))

    const c = crearHttpCatalog('')
    const pagina = await c.canales({})

    expect(pagina.total).toBe(12639)
    expect(pagina.canales[0]).toMatchObject({
      id: 'opensource-BBC One',
      nombre: 'BBC One (1080p)',
      pais: 'GB',
      vivo: true,
      latenciaMs: 120,
      webOk: false,
    })
  })

  // null NO es false. Un canal sin comprobar se pinta distinto de uno que no
  // se ve, en la señal y en la marca web.
  it('conserva null en vivo y webOk', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta([{ ID: 'x', Name: 'X', Alive: null, WebOK: null, LatencyMs: 0 }]),
    ))

    const pagina = await crearHttpCatalog('').canales({})
    expect(pagina.canales[0].vivo).toBeNull()
    expect(pagina.canales[0].webOk).toBeNull()
  })

  it('construye la query con los filtros', async () => {
    // Firma variádica: sin esto, TS infiere una tupla de longitud 0 para
    // los argumentos de la llamada y `.mock.calls[0][0]` no compila.
    const espia = vi.fn(async (..._args: unknown[]) => respuesta([]))
    vi.stubGlobal('fetch', espia)

    await crearHttpCatalog('').canales({
      q: 'bbc', pais: 'GB', categoria: 'News', calidad: 'hd',
      limite: 500, desplazamiento: 1000,
    })

    const url = String(espia.mock.calls[0][0])
    expect(url).toContain('q=bbc')
    expect(url).toContain('country=GB')
    expect(url).toContain('category=News')
    expect(url).toContain('quality=hd')
    expect(url).toContain('limit=500')
    expect(url).toContain('offset=1000')
  })

  // Por defecto el gateway ya oculta los muertos. "Mostrar offline" es
  // ?alive=all, que es lo que hace el toggle.
  it('mostrarOffline manda alive=all', async () => {
    const espia = vi.fn(async (..._args: unknown[]) => respuesta([]))
    vi.stubGlobal('fetch', espia)

    await crearHttpCatalog('').canales({ mostrarOffline: true })
    expect(String(espia.mock.calls[0][0])).toContain('alive=all')
  })

  // Con el conjunto de favoritos VACÍO hay que mandar un centinela: unos ids
  // vacíos significan "sin filtro" y devolverían los 12 000 canales, que es
  // exactamente lo contrario de lo que pidió el usuario.
  it('favoritos vacíos no devuelven el catálogo entero', async () => {
    const espia = vi.fn(async () => respuesta([]))
    vi.stubGlobal('fetch', espia)

    const pagina = await crearHttpCatalog('').canales({ ids: [] })
    expect(pagina.canales).toEqual([])
    expect(espia).not.toHaveBeenCalled()
  })

  it('la consulta de favoritos manda un limit alto para no truncar', async () => {
    const espia = vi.fn(async (..._args: unknown[]) => respuesta([]))
    vi.stubGlobal('fetch', espia)
    await crearHttpCatalog('').canales({ ids: ['a', 'b', 'c'] })
    const url = String(espia.mock.calls[0][0])
    expect(url).toContain('limit=')
  })

  it('un fallo de red se distingue de un gateway caído', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    await expect(crearHttpCatalog('').canales({})).rejects.toThrow(/red|gateway/i)
  })

  it('mirrors traduce las claves del cable', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta([
        { url: 'https://a/x.m3u8', is_alive: true, latency_ms: 100, web_ok: true },
        { url: 'https://b/x.m3u8', is_alive: true, latency_ms: 300, web_ok: false },
        { url: 'https://c/x.m3u8', is_alive: false, latency_ms: 0, web_ok: null },
      ]),
    ))
    const mirrors = await crearHttpCatalog('').mirrors('c1')
    expect(mirrors).toHaveLength(3)
    expect(mirrors[0]).toEqual({ url: 'https://a/x.m3u8', vivo: true, latenciaMs: 100, webOk: true })
    expect(mirrors[2].vivo).toBe(false)
    expect(mirrors[2].webOk).toBeNull()
  })
})
