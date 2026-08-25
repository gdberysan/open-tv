/**
 * Diagnóstico del fallo de reproducción: convierte la señal cruda que ya
 * emiten hls.js (`data.type`/`data.details`/`data.response?.code`) y el
 * `<video>` nativo (`video.error.code`) en una clase de fallo granular, en
 * vez del "caído, geo-bloqueado o caducó" genérico de siempre.
 *
 * Puro y sin dependencias: el Reproductor capta la info del error de cada
 * intento y la pasa aquí; este módulo no toca hls.js, el DOM ni el estado.
 */
export type ClaseFallo = 'caido' | 'geo' | 'formato' | 'caducado' | 'desconocido'

export interface InfoFallo {
  /** `data.type` de hls.js (p.ej. "networkError", "mediaError"). */
  tipoHls?: string
  /** `data.details` de hls.js (p.ej. "manifestLoadError", "bufferAppendError"). */
  detallesHls?: string
  /** `data.response?.code` de hls.js: el status HTTP del recurso que falló. */
  httpStatus?: number
  /** `video.error.code` nativo: 1 ABORTED, 2 NETWORK, 3 DECODE, 4 SRC_NOT_SUPPORTED. */
  mediaErrorCode?: number
}

// MediaError.* del <video> nativo (spec HTML, no exportadas como constantes).
const MEDIA_ERR_NETWORK = 2
const MEDIA_ERR_DECODE = 3
const MEDIA_ERR_SRC_NOT_SUPPORTED = 4

/**
 * Reglas, en orden de prioridad (la primera que casa gana):
 * 1. httpStatus 403 → 'geo' (geo-bloqueo o token caducado; ambiguo a
 *    propósito, el mensaje no afirma cuál de los dos es).
 * 2. httpStatus 404/410 → 'caducado' (el mirror ya no existe).
 * 3. Error de red sin status útil (tipoHls tipo NETWORK_ERROR, o
 *    mediaErrorCode MEDIA_ERR_NETWORK) → 'caido'.
 * 4. Error de media/decodificación (tipoHls MEDIA_ERROR, detallesHls con
 *    BUFFER_APPEND/DECODE, o mediaErrorCode DECODE/SRC_NOT_SUPPORTED) →
 *    'formato'.
 * 5. Cualquier otra cosa (o nada) → 'desconocido'.
 *
 * Coincidencia por substring en minúsculas sobre tipoHls/detallesHls: hls.js
 * no exporta sus enums como valores estables en todas las versiones, así que
 * emparejar el texto es más robusto que comparar por igualdad exacta.
 */
export function clasificarFallo(info: InfoFallo): ClaseFallo {
  const { tipoHls, detallesHls, httpStatus, mediaErrorCode } = info

  if (httpStatus === 403) return 'geo'
  if (httpStatus === 404 || httpStatus === 410) return 'caducado'

  const tipo = (tipoHls ?? '').toLowerCase()
  const detalles = (detallesHls ?? '').toLowerCase()

  if (tipo.includes('networkerror') || mediaErrorCode === MEDIA_ERR_NETWORK) return 'caido'

  if (
    tipo.includes('mediaerror') ||
    detalles.includes('bufferappend') ||
    detalles.includes('decode') ||
    mediaErrorCode === MEDIA_ERR_DECODE ||
    mediaErrorCode === MEDIA_ERR_SRC_NOT_SUPPORTED
  ) {
    return 'formato'
  }

  return 'desconocido'
}
