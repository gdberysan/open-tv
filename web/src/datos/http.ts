import type {
  Canal, CatalogSource, ConsultaCatalogo, DestinoStream, Faceta, Frescura, Mirror, PaginaCanales,
} from './catalogo'

/**
 * Las claves del cable son nombres de campo de Go y están CONGELADAS por un
 * test de contrato (internal/domain/channel_test.go). Este es el único sitio
 * del cliente que las conoce.
 */
interface CanalCable {
  ID: string
  Name: string
  LogoURL?: string
  CategoryID?: string
  LanguageCode?: string
  CountryCode?: string
  Alive?: boolean | null
  LatencyMs?: number
  WebOK?: boolean | null
}

function aCanal(c: CanalCable): Canal {
  return {
    id: c.ID,
    nombre: c.Name,
    logoUrl: c.LogoURL ?? '',
    categoriaId: c.CategoryID ?? '',
    idioma: c.LanguageCode ?? '',
    pais: c.CountryCode ?? '',
    // ?? null y no ?? false: "sin comprobar" es un tercer estado con su propio
    // dibujo en la tarjeta.
    vivo: c.Alive ?? null,
    latenciaMs: c.LatencyMs ?? 0,
    webOk: c.WebOK ?? null,
  }
}

interface MirrorCable {
  url: string
  is_alive?: boolean
  latency_ms?: number
  web_ok?: boolean | null
}

function query(c: ConsultaCatalogo): URLSearchParams {
  const p = new URLSearchParams()
  if (c.q) p.set('q', c.q)
  if (c.pais) p.set('country', c.pais)
  if (c.categoria) p.set('category', c.categoria)
  if (c.calidad) p.set('quality', c.calidad)
  if (c.mostrarOffline) p.set('alive', 'all')
  if (c.ids?.length) p.set('ids', c.ids.join(','))
  // Sin limit explícito, el gateway aplica el suyo por defecto y puede
  // truncar la lista de favoritos por debajo del nº de ids pedidos: la
  // lista se recorta en silencio. 1000 favoritos ya no caben en ningún uso
  // real.
  if (c.ids?.length && c.limite == null) p.set('limit', '1000')
  if (c.limite != null) p.set('limit', String(c.limite))
  if (c.desplazamiento != null) p.set('offset', String(c.desplazamiento))
  return p
}

async function pedir(url: string): Promise<Response> {
  let resp: Response
  try {
    resp = await fetch(url)
  } catch (e) {
    // fetch solo lanza por fallo de transporte. Distinguirlo importa: el
    // mensaje "gateway caído" y el "sin red" son problemas distintos con
    // soluciones distintas, y confundirlos costó una tarde en agosto.
    throw new Error(
      typeof navigator !== 'undefined' && navigator.onLine === false
        ? 'sin red'
        : 'gateway inalcanzable',
      { cause: e },
    )
  }
  if (!resp.ok) throw new Error(`respuesta ${resp.status}`)
  return resp
}

export function crearHttpCatalog(base = ''): CatalogSource {
  return {
    async canales(c: ConsultaCatalogo): Promise<PaginaCanales> {
      // Centinela del conjunto vacío: sin esto, unos ids vacíos serían "sin
      // filtro" y el gateway devolvería los 12 000 canales.
      if (c.ids && c.ids.length === 0) return { canales: [], total: 0 }

      const resp = await pedir(`${base}/channels?${query(c)}`)
      const crudos = (await resp.json()) as CanalCable[] | null
      const canales = (crudos ?? []).map(aCanal)
      const cabecera = resp.headers.get('X-Total-Count')
      return { canales, total: cabecera ? Number(cabecera) : canales.length }
    },

    async paises(): Promise<Faceta[]> {
      const resp = await pedir(`${base}/channels/countries`)
      const crudas = (await resp.json()) as Array<{ Valor: string; Count: number }> | null
      return (crudas ?? []).map((f) => ({ valor: f.Valor, total: f.Count }))
    },

    async categorias(): Promise<Faceta[]> {
      const resp = await pedir(`${base}/channels/categories`)
      const crudas = (await resp.json()) as Array<{ Valor: string; Count: number }> | null
      return (crudas ?? []).map((f) => ({ valor: f.Valor, total: f.Count }))
    },

    async aleatorio(c: ConsultaCatalogo): Promise<Canal> {
      const resp = await pedir(`${base}/channels/random?${query(c)}`)
      return aCanal((await resp.json()) as CanalCable)
    },

    async destino(id: string): Promise<DestinoStream> {
      const resp = await pedir(`${base}/channels/stream?id=${encodeURIComponent(id)}`)
      const cuerpo = (await resp.json()) as { url: string; airplay_ok?: boolean | null }
      return { url: cuerpo.url, airplayOk: cuerpo.airplay_ok ?? null }
    },

    async mirrors(id: string): Promise<Mirror[]> {
      const resp = await pedir(`${base}/channels/streams?id=${encodeURIComponent(id)}`)
      const crudos = (await resp.json()) as MirrorCable[] | null
      return (crudos ?? []).map((m) => ({
        url: m.url,
        // ?? null y no ?? false: "sin comprobar" es un tercer estado, igual
        // que en aCanal.
        vivo: m.is_alive ?? null,
        latenciaMs: m.latency_ms ?? 0,
        webOk: m.web_ok ?? null,
      }))
    },

    async frescura(): Promise<Frescura> {
      return { tipo: 'vivo', generadoEn: null }
    },

    async proxyDisponible(): Promise<boolean> {
      try {
        const resp = await pedir(`${base}/health`)
        const cuerpo = (await resp.json()) as { proxy_enabled?: boolean }
        return cuerpo.proxy_enabled === true
      } catch {
        return false
      }
    },
  }
}
