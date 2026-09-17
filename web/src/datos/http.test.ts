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

  // Modo red: si la sesión caduca con la app abierta, la API responde 401 y
  // lo correcto es volver a la página de acceso, no pintar "gateway caído".
  it('un 401 lleva a la página de acceso', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{"error":"acceso requerido"}', { status: 401 })))
    const assign = vi.fn()
    vi.stubGlobal('location', { assign, protocol: 'http:' })

    const c = crearHttpCatalog('')
    await expect(c.fuentes()).rejects.toThrow('sesión caducada')
    expect(assign).toHaveBeenCalledWith('/acceso')
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

  it('mirrors traduce las claves del cable, codec_ok y audio/imagen incluidos', async () => {
    vi.stubGlobal('fetch', vi.fn(async () =>
      respuesta([
        {
          url: 'https://a/x.m3u8', is_alive: true, latency_ms: 100, web_ok: true, codec_ok: false, codecs: 'mpeg2video,mp2',
          audio_ok: false, imagen_ms: 2100, sin_imagen: true, ultimo_fallo_hace_s: 3600,
        },
        { url: 'https://b/x.m3u8', is_alive: true, latency_ms: 300, web_ok: false, codec_ok: true, codecs: 'h264,aac' },
        { url: 'https://c/x.m3u8', is_alive: false, latency_ms: 0, web_ok: null, codec_ok: null, codecs: '' },
        { url: 'https://d/x.m3u8', is_alive: true, latency_ms: 50 }, // servidor viejo: sin claves
      ]),
    ))
    const mirrors = await crearHttpCatalog('').mirrors('c1')
    expect(mirrors).toHaveLength(4)
    expect(mirrors[0]).toEqual({
      url: 'https://a/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, codecOk: false, codecs: 'mpeg2video,mp2',
      audioOk: false, imagenMs: 2100, sinImagen: true, ultimoFalloHaceS: 3600,
    })
    expect(mirrors[1].codecOk).toBe(true)
    expect(mirrors[2].vivo).toBe(false)
    expect(mirrors[2].webOk).toBeNull()
    expect(mirrors[2].codecOk).toBeNull()
    // "sin sondear" es un tercer estado: nunca false.
    expect(mirrors[3].codecOk).toBeNull()
    expect(mirrors[3].codecs).toBe('')
    // Servidor viejo sin las claves nuevas: valores por defecto, no undefined.
    expect(mirrors[3].audioOk).toBeNull()
    expect(mirrors[3].imagenMs).toBe(0)
    expect(mirrors[3].sinImagen).toBe(false)
    expect(mirrors[3].ultimoFalloHaceS).toBe(0)
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

  describe('imagen', () => {
    it('imagen() lee GET /channels/imagen y devuelve el mapa', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta({
        c1: { imagen_ms: 1200, sin_imagen: false },
      }))
      vi.stubGlobal('fetch', espia)

      const mapa = await crearHttpCatalog('').imagen!()

      expect(String(espia.mock.calls[0][0])).toBe('/channels/imagen')
      expect(mapa).toEqual({ c1: { imagenMs: 1200, sinImagen: false } })
    })
  })

  describe('epg', () => {
    it('epgDeCanales() lee GET /channels/epg?ids=… y traduce inicio/fin a inicioSeg/finSeg', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta({
        a: { ahora: { titulo: 'Noticias', inicio: 1787770800, fin: 1787774400 }, siguiente: null },
      }))
      vi.stubGlobal('fetch', espia)

      const mapa = await crearHttpCatalog('').epgDeCanales(['a', 'b'])

      expect(String(espia.mock.calls[0][0])).toBe('/channels/epg?ids=a,b')
      expect(mapa.get('a')).toEqual({
        ahora: { titulo: 'Noticias', inicioSeg: 1787770800, finSeg: 1787774400 },
        siguiente: null,
      })
    })

    // Ausente = sin guía posible: el handler serializa literalmente las
    // claves que le da el repo, así que un id sin tvg_id no aparece como
    // clave del objeto — y por tanto tampoco debe aparecer en el Map.
    it('epgDeCanales() no añade una clave para un id ausente en la respuesta', async () => {
      vi.stubGlobal('fetch', vi.fn(async () => respuesta({
        a: { ahora: null, siguiente: null },
      })))

      const mapa = await crearHttpCatalog('').epgDeCanales(['a', 'b'])

      expect(mapa.has('a')).toBe(true)
      expect(mapa.has('b')).toBe(false)
    })

    it('epgDeCanales() con ids vacío no pide nada y devuelve un Map vacío', async () => {
      const espia = vi.fn(async () => respuesta({}))
      vi.stubGlobal('fetch', espia)

      const mapa = await crearHttpCatalog('').epgDeCanales([])

      expect(mapa.size).toBe(0)
      expect(espia).not.toHaveBeenCalled()
    })

    it('epgDeCanal() lee GET /channels/{id}/epg?limit=… y traduce el cable', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta({
        ahora: { titulo: 'Ahora', inicio: 1000, fin: 2000 },
        proximos: [{ titulo: 'Luego', inicio: 2000, fin: 3000 }],
      }))
      vi.stubGlobal('fetch', espia)

      const resultado = await crearHttpCatalog('').epgDeCanal('c1', 3)

      expect(String(espia.mock.calls[0][0])).toBe('/channels/c1/epg?limit=3')
      expect(resultado.ahora).toEqual({ titulo: 'Ahora', inicioSeg: 1000, finSeg: 2000 })
      expect(resultado.proximos).toEqual([{ titulo: 'Luego', inicioSeg: 2000, finSeg: 3000 }])
    })

    it('epgDeCanal() sin limite explícito no manda el parámetro', async () => {
      const espia = vi.fn(async (..._args: unknown[]) => respuesta({ ahora: null, proximos: [] }))
      vi.stubGlobal('fetch', espia)

      const resultado = await crearHttpCatalog('').epgDeCanal('c1')

      expect(String(espia.mock.calls[0][0])).toBe('/channels/c1/epg')
      expect(resultado.ahora).toBeNull()
      expect(resultado.proximos).toEqual([])
    })
  })
})
