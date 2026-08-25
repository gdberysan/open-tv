export type Motor = 'nativo' | 'hlsjs'

export interface EntradaPlan {
  motor: Motor
  url: string
  webOk: boolean | null
  proxyDisponible: boolean
}

export interface Plan {
  /** URLs a probar, en orden. Vacío = no hay forma de reproducirlo aquí. */
  intentos: string[]
  aviso: 'ninguno' | 'solo-app-o-safari'
}

export const RUTA_PROXY = '/proxy/hls?u='

export function urlProxy(url: string): string {
  return RUTA_PROXY + encodeURIComponent(url)
}

/**
 * Decide cómo intentar reproducir. La regla de la spec es "directo primero,
 * proxy si falla", con dos matices que salen del censo:
 *
 * - Safari reproduce HLS de forma NATIVA y no necesita CORS, así que ve el
 *   85 % del catálogo frente al 67 % de Chrome/Firefox. web_ok es el veredicto
 *   estricto (hls.js); negarle a Safari un canal por eso sería negarle algo
 *   que sí puede ver.
 * - Con un web_ok FALSO explícito, el intento directo son dos o tres segundos
 *   tirados: ya sabemos que el navegador va a cortar el fetch.
 */
export function planDeReproduccion(e: EntradaPlan): Plan {
  const esHttps = e.url.toLowerCase().startsWith('https://')
  const paginaSegura = typeof location !== 'undefined' && location.protocol === 'https:'

  // Contenido mixto: una página https no carga medios http, ni en Safari.
  if (!esHttps && paginaSegura) {
    return e.proxyDisponible
      ? { intentos: [urlProxy(e.url)], aviso: 'ninguno' }
      : { intentos: [], aviso: 'solo-app-o-safari' }
  }

  if (e.motor === 'nativo') {
    const intentos = [e.url]
    if (e.proxyDisponible) intentos.push(urlProxy(e.url))
    return { intentos, aviso: 'ninguno' }
  }

  if (e.webOk === false) {
    return e.proxyDisponible
      ? { intentos: [urlProxy(e.url)], aviso: 'ninguno' }
      : { intentos: [], aviso: 'solo-app-o-safari' }
  }

  const intentos = [e.url]
  if (e.proxyDisponible) intentos.push(urlProxy(e.url))
  return { intentos, aviso: 'ninguno' }
}

/** Safari y iOS reproducen HLS sin librería; el resto necesita hls.js. */
export function motorDelNavegador(video: HTMLVideoElement): Motor {
  return video.canPlayType('application/vnd.apple.mpegurl') !== '' ? 'nativo' : 'hlsjs'
}
