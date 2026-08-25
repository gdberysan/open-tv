import { afterEach, describe, expect, it, vi } from 'vitest'
import { planDeReproduccion, urlProxy } from './plan'

const URL_HTTPS = 'https://cdn.example/live.m3u8'
const URL_HTTP = 'http://cdn.example/live.m3u8'

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
