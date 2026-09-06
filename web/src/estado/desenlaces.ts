import type { DesenlaceReproduccion } from '../reproductor/failover'

/**
 * reportarDesenlaceMirror manda el desenlace de UN mirror concreto a
 * /streams/desenlace, para que el backend correle el fallo con la salud de
 * ESE mirror (a diferencia de reportarDesenlace en estadisticas.ts, que
 * reporta por canal). Mismo patrón best-effort: try/catch +
 * `void fetch(...).catch(() => {})`, nunca lanza.
 *
 * No reporta (y no llama a fetch) cuando el desenlace no dice nada sobre la
 * salud real del mirror:
 * - via === 'ninguna': no se llegó a intentar nada.
 * - motorForzado: el motor se forzó (AirPlay/Chromecast), no es comparable.
 * - oculto: la pestaña estuvo oculta durante el intento (hls.js no pide
 *   segmentos con la pestaña oculta — ver TRAMPA en CLAUDE.md).
 * - resultado === 'cortado': un corte tras verse no es un intento fallido.
 * - !url: no hay mirror que reportar.
 * - sin conexión (navigator.onLine === false): el fetch fallaría igual.
 */
export function reportarDesenlaceMirror(o: DesenlaceReproduccion, base = ''): boolean {
  if (
    o.via === 'ninguna'
    || o.motorForzado === true
    || o.oculto === true
    || o.resultado === 'cortado'
    || !o.url
    || (typeof navigator !== 'undefined' && navigator.onLine === false)
  ) {
    return false
  }

  try {
    const body = JSON.stringify({
      url: o.url,
      resultado: o.resultado,
      motivo: o.motivo ?? '',
      // Redondeado: performance.now() trae decimales (mismo motivo que
      // estadisticas.ts).
      ms_primer_frame: Math.round(o.msPrimerFrame ?? 0),
    })
    void fetch(`${base}/streams/desenlace`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
    }).catch(() => {})
  } catch {
    // Ni un fallo de serialización ni de transporte rompe la reproducción.
  }
  return true
}
