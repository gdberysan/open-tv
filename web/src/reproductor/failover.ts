import { planDeReproduccion, urlProxy, type Motor } from './plan'
import type { Mirror } from '../datos/catalogo'

export interface Intento {
  url: string
  viaProxy: boolean
  mirrorIndex: number
}

/**
 * planDeFailover aplana los mirrors (ya ordenados por salud) y la política
 * directo-o-proxy de cada uno en una sola secuencia de intentos. El reproductor
 * la recorre en orden: si un intento falla de forma fatal, prueba el siguiente,
 * cruzando de mirror sin ceremonia. Reutiliza planDeReproduccion para no
 * duplicar la lógica de motor/proxy/contenido-mixto.
 */
export function planDeFailover(mirrors: Mirror[], motor: Motor, proxyDisponible: boolean): Intento[] {
  const intentos: Intento[] = []
  mirrors.forEach((mirror, i) => {
    const plan = planDeReproduccion({ motor, url: mirror.url, webOk: mirror.webOk, proxyDisponible })
    for (const url of plan.intentos) {
      intentos.push({ url, viaProxy: url === urlProxy(mirror.url), mirrorIndex: i })
    }
  })
  return intentos
}

/**
 * Desenlace de un intento de reproducción: cómo terminó, con qué motor y vía,
 * y en qué mirror de la cadena de failover. Lo produce el reproductor al
 * cerrar (o no llegar a abrir) la reproducción; el reporter (Tarea 14) lo
 * serializa a snake_case para el backend.
 */
export interface DesenlaceReproduccion {
  canalId: string
  resultado: 'iniciado' | 'fallo' | 'cortado'
  motivo?: string
  motor: 'nativo' | 'hlsjs'
  /** 'ninguna': no se llegó a intentar nada (todos los mirrors con
   *  codecOk === false). El backend agrupa por cadena libre. */
  via: 'directo' | 'proxy' | 'ninguna'
  mirrorIndex: number
  msPrimerFrame?: number
  /** URL del mirror concreto que se intentó, para el reporte por-mirror
   *  (Tarea 5): el backend correla el fallo con el registro de salud de ESE
   *  mirror, no del canal entero. */
  url?: string
  /** true = la pestaña estuvo oculta (document.visibilityState) durante el
   *  intento: hls.js no pide segmentos con la pestaña oculta, así que un
   *  "fallo" así no dice nada sobre la salud real del mirror. */
  oculto?: boolean
  /** true = el motor se forzó (AirPlay/Chromecast) en vez de elegirse por la
   *  política normal: un fallo aquí no es comparable al resto. */
  motorForzado?: boolean
}
