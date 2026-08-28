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

/**
 * Un veredicto DEFINITIVO ('probably') se queda con el <video> nativo, que
 * además es más eficiente y permite AirPlay. Chrome y
 * Firefox devuelven 'maybe' (Chrome) o '' (Firefox) — un veredicto ambiguo
 * que en la práctica esconde un HLS nativo poco fiable: el gate manual vio
 * un <video src=proxiedM3U8>+.load() quedarse en readyState 0 en Chrome con
 * un stream que el mismo proxy servía bien. Por eso, si hay MSE disponible
 * (Media Source Extensions, lo que usa hls.js), se prefiere hls.js sobre
 * cualquier soporte nativo que no sea definitivo. Sin MSE (p. ej. iOS, que
 * bloquea MSE fuera de Safari) solo queda el nativo si lo soporta.
 *
 * OJO con 'probably': era la respuesta de Safari, pero **Safari 26.6 devuelve
 * 'maybe'** (comprobado en Safari real por WebDriver el 2026-08-28; también
 * responde 'maybe' a 'video/mp4', así que parece que redujo la huella que
 * dejaba `canPlayType`). O sea: en un Safari de hoy esta primera rama NO se
 * toma y se reproduce por hls.js, con `src` de tipo blob. La rama se queda
 * porque las versiones anteriores sí dicen 'probably' y ahí el nativo es
 * mejor. Lo que hay que vigilar es AirPlay, que se lleva mal con fuentes
 * MSE; confirmarlo necesita un Apple TV de verdad.
 */
export function motorDelNavegador(video: HTMLVideoElement): Motor {
  const nativo = video.canPlayType('application/vnd.apple.mpegurl')
  const mseDisponible =
    typeof MediaSource !== 'undefined' || typeof (globalThis as any).ManagedMediaSource !== 'undefined'

  if (nativo === 'probably') return 'nativo'
  if (mseDisponible) return 'hlsjs'
  return nativo !== '' ? 'nativo' : 'hlsjs'
}
