import { afterEach, describe, expect, it, vi } from 'vitest'
import { motorDelNavegador, planDeReproduccion, RUTA_PROXY, urlProxy } from './plan'

const URL_HTTPS = 'https://cdn.example/live.m3u8'
const URL_HTTP = 'http://cdn.example/live.m3u8'

// RUTA_PROXY vive por partida doble: en Go (router.RutaProxy) y aquí. El
// gateway la publica en /health como proxy_ruta (ver
// TestHealthPublicaLaRutaDelProxy en internal/api/handlers); este test fija
// el mismo literal en el lado TS para que ambos no puedan divergir en
// silencio. Ideal sería leerla de /health al arrancar; por ahora basta con
// que coincidan.
describe('RUTA_PROXY', () => {
  it('coincide con la ruta que /health publica en proxy_ruta', () => {
    expect(RUTA_PROXY).toBe('/proxy/hls?u=')
  })
})

describe('planDeReproduccion', () => {
  // Safari reproduce HLS de forma nativa y NO necesita CORS. Por eso ve el
  // 85 % del catálogo y Chrome/Firefox el 67 %: negarle un canal porque
  // web_ok sea falso sería negarle algo que sí puede ver.
  it('en HLS nativo se intenta directo aunque web_ok sea falso', () => {
    const p = planDeReproduccion({ motor: 'nativo', url: URL_HTTPS, webOk: false, proxyDisponible: false })
    expect(p.intentos[0]).toBe(URL_HTTPS)
    expect(p.aviso).toBe('ninguno')
  })

  it('con hls.js y web_ok true se va directo, con el proxy de red', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: true, proxyDisponible: true })
    expect(p.intentos).toEqual([URL_HTTPS, urlProxy(URL_HTTPS)])
  })

  // Sin veredicto se prueba igualmente: "directo primero, proxy si falla".
  it('con hls.js y web_ok null se intenta directo y luego el proxy', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: null, proxyDisponible: true })
    expect(p.intentos).toEqual([URL_HTTPS, urlProxy(URL_HTTPS)])
  })

  // Con un NO explícito, el intento directo son 2-3 segundos tirados: ya
  // sabemos que el navegador lo va a cortar.
  it('con hls.js y web_ok false se va directo al proxy', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: false, proxyDisponible: true })
    expect(p.intentos).toEqual([urlProxy(URL_HTTPS)])
  })

  // El sitio hospedado NO tiene proxy: es estructural, no una opción.
  it('sin proxy y sin veredicto favorable, se avisa en vez de fingir', () => {
    const p = planDeReproduccion({ motor: 'hlsjs', url: URL_HTTPS, webOk: false, proxyDisponible: false })
    expect(p.intentos).toEqual([])
    expect(p.aviso).toBe('solo-app-o-safari')
  })

  // jsdom expone location.protocol = 'http:' por defecto: hay que forzar
  // 'https:' para que este caso (contenido mixto) sea real.
  it('http desde una página https solo puede ir por el proxy', () => {
    vi.stubGlobal('location', { protocol: 'https:' })
    try {
      const p = planDeReproduccion({ motor: 'nativo', url: URL_HTTP, webOk: null, proxyDisponible: true })
      expect(p.intentos).toEqual([urlProxy(URL_HTTP)])
    } finally {
      vi.unstubAllGlobals()
    }
  })
})

// canPlayType('application/vnd.apple.mpegurl') es 'maybe' en Chrome incluso
// cuando su <video> nativo no decodifica el HLS de forma fiable (visto en el
// gate manual: readyState se queda en 0 con un proxy que sí funciona). Solo
// 'probably' (Safari) es un veredicto DEFINITIVO; con MSE disponible se
// prefiere hls.js sobre cualquier 'maybe'/''  ambiguo.
function video(canPlayType: string): HTMLVideoElement {
  return { canPlayType: () => canPlayType } as unknown as HTMLVideoElement
}

describe('motorDelNavegador', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('Chrome y Safari 26.6+: maybe + MediaSource → hlsjs (el nativo es poco fiable)', () => {
    vi.stubGlobal('MediaSource', class {})
    expect(motorDelNavegador(video('maybe'))).toBe('hlsjs')
  })

  it('probably → nativo aunque haya MediaSource (Safari anteriores; soporte definitivo)', () => {
    vi.stubGlobal('MediaSource', class {})
    expect(motorDelNavegador(video('probably'))).toBe('nativo')
  })

  it('Firefox: sin soporte nativo pero con MediaSource → hlsjs', () => {
    vi.stubGlobal('MediaSource', class {})
    expect(motorDelNavegador(video(''))).toBe('hlsjs')
  })

  it('sin MSE (tipo iOS) pero con soporte nativo maybe → nativo', () => {
    vi.stubGlobal('MediaSource', undefined)
    expect(motorDelNavegador(video('maybe'))).toBe('nativo')
  })

  it('sin MSE y sin soporte nativo → hlsjs como último recurso', () => {
    vi.stubGlobal('MediaSource', undefined)
    expect(motorDelNavegador(video(''))).toBe('hlsjs')
  })
})

