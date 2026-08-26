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

  // El endpoint emite ports.Faceta de Go SIN json tags: en el cable es
  // {Valor, Count} con V mayúscula, no {valor,total}. calidades() debe
  // adaptar la forma igual que paises()/categorias().
  it('calidades traduce {Valor,Count} del cable a {valor,total}', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta([{ Valor: 'hd', Count: 123 }, { Valor: 'sd', Count: 45 }]),
    ))

    const facetas = await crearHttpCatalog('').calidades()
    expect(facetas).toEqual([
      { valor: 'hd', total: 123 },
      { valor: 'sd', total: 45 },
    ])
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

  describe('fuentes', () => {
    it('fuentes() lee GET /sources y traduce el cable', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta([
        { id: 'f1', label: 'Mi lista', url: 'https://x/lista.m3u8', kind: 'url', ultimo_sync: 1700000000, canales: 42 },
      ]))
      vi.stubGlobal('fetch', espia)

      const fuentes = await crearHttpCatalog('').fuentes()

      expect(String(espia.mock.calls[0][0])).toBe('/sources')
      expect(fuentes).toEqual([
        { id: 'f1', label: 'Mi lista', url: 'https://x/lista.m3u8', kind: 'url', ultimoSync: 1700000000, canales: 42 },
      ])
    })

    // El cable manda 0 como centinela de "nunca sincronizada": traducirlo a
    // 0 literal confundiría "nunca" con "el 1 de enero de 1970".
    it('fuentes() traduce ultimo_sync 0 a null', async () => {
      vi.stubGlobal('fetch', vi.fn(async () => respuesta([
        { id: 'f1', label: 'X', url: 'https://x', kind: 'url', ultimo_sync: 0, canales: 0 },
      ])))

      const fuentes = await crearHttpCatalog('').fuentes()
      expect(fuentes[0].ultimoSync).toBeNull()
    })

    it('anadirFuente() manda POST JSON con {url,label}', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta(
        { id: 'f2', label: 'Etiqueta', url: 'https://y/lista.m3u8', kind: 'url', ultimo_sync: 0, canales: 0 },
      ))
      vi.stubGlobal('fetch', espia)

      const fuente = await crearHttpCatalog('').anadirFuente('https://y/lista.m3u8', 'Etiqueta')

      const [url, init] = espia.mock.calls[0] as [string, RequestInit]
      expect(String(url)).toBe('/sources')
      expect(init.method).toBe('POST')
      expect(init.headers).toMatchObject({ 'Content-Type': 'application/json' })
      expect(JSON.parse(init.body as string)).toEqual({ url: 'https://y/lista.m3u8', label: 'Etiqueta' })
      expect(fuente).toEqual({ id: 'f2', label: 'Etiqueta', url: 'https://y/lista.m3u8', kind: 'url', ultimoSync: null, canales: 0 })
    })

    it('anadirFuente() sin label no manda la clave', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta(
        { id: 'f3', label: '', url: 'https://z', kind: 'url', ultimo_sync: 0, canales: 0 },
      ))
      vi.stubGlobal('fetch', espia)

      await crearHttpCatalog('').anadirFuente('https://z')

      const [, init] = espia.mock.calls[0] as [string, RequestInit]
      expect(JSON.parse(init.body as string)).toEqual({ url: 'https://z' })
    })

    it('anadirFuenteFichero() manda POST multipart con el campo fichero', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta(
        { id: 'f4', label: 'lista.m3u', url: '', kind: 'file', ultimo_sync: 0, canales: 10 },
      ))
      vi.stubGlobal('fetch', espia)

      const fichero = new File(['#EXTM3U'], 'lista.m3u', { type: 'audio/x-mpegurl' })
      const fuente = await crearHttpCatalog('').anadirFuenteFichero(fichero)

      const [url, init] = espia.mock.calls[0] as [string, RequestInit]
      expect(String(url)).toBe('/sources')
      expect(init.method).toBe('POST')
      expect(init.body).toBeInstanceOf(FormData)
      const form = init.body as FormData
      expect(form.get('fichero')).toBe(fichero)
      expect(fuente.kind).toBe('file')
    })

    it('quitarFuente() manda DELETE /sources/{id}', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => new Response(null, { status: 204 }))
      vi.stubGlobal('fetch', espia)

      await crearHttpCatalog('').quitarFuente('f1')

      const [url, init] = espia.mock.calls[0] as [string, RequestInit]
      expect(String(url)).toBe('/sources/f1')
      expect(init.method).toBe('DELETE')
    })

    it('resyncFuente() manda POST /sources/{id}/sync', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => new Response(null, { status: 204 }))
      vi.stubGlobal('fetch', espia)

      await crearHttpCatalog('').resyncFuente('f1')

      const [url, init] = espia.mock.calls[0] as [string, RequestInit]
      expect(String(url)).toBe('/sources/f1/sync')
      expect(init.method).toBe('POST')
    })

    it('fuentesSugeridas() lee GET /sources/sugeridas sin adaptar forma', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta([
        { label: 'Ejemplo', url: 'https://ejemplo/lista.m3u8' },
      ]))
      vi.stubGlobal('fetch', espia)

      const sugeridas = await crearHttpCatalog('').fuentesSugeridas()

      expect(String(espia.mock.calls[0][0])).toBe('/sources/sugeridas')
      expect(sugeridas).toEqual([{ label: 'Ejemplo', url: 'https://ejemplo/lista.m3u8' }])
    })
  })
})
