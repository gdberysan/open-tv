/**
 * Diagnóstico del fallo de reproducción: convierte la señal cruda que ya
 * emiten hls.js (`data.type`/`data.details`/`data.response?.code`) y el
 * `<video>` nativo (`video.error.code`) en una clase de fallo granular, en
 * vez del "caído, geo-bloqueado o caducó" genérico de siempre.
 *
 * Puro y sin dependencias: el Reproductor capta la info del error de cada
 * intento y la pasa aquí; este módulo no toca hls.js, el DOM ni el estado.
 */
export type ClaseFallo = 'caido' | 'geo' | 'formato' | 'inestable' | 'caducado' | 'desconocido'

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

  // 'formato' SOLO con señales de códec de verdad. Antes bastaba con
  // tipo === 'mediaError', y en hls.js TODO problema de la tubería de medios es
  // mediaError: bufferStalledError, fragParsingError, bufferAppendError,
  // bufferSeekOverHole y bufferNudgeOnStall lo son. O sea que un simple atasco
  // acababa diciéndole al usuario «tu navegador no puede reproducir este
  // formato. Prueba en Safari» — falso, y un consejo que no arregla nada
  // porque Safari recibiría exactamente los mismos bytes rotos.
  // Estas tres son las que hls.js usa cuando el códec de verdad no encaja.
  const CODEC_INCOMPATIBLE = ['manifestincompatiblecodecs', 'bufferincompatiblecodecs', 'bufferaddcodec']
  if (
    CODEC_INCOMPATIBLE.some((d) => detalles.includes(d)) ||
    mediaErrorCode === MEDIA_ERR_DECODE ||
    mediaErrorCode === MEDIA_ERR_SRC_NOT_SUPPORTED
  ) {
    return 'formato'
  }

  // Resto de la tubería de medios: el canal EMITE, pero lo que llega no se
  // puede sostener — se atasca, no se demuxa o no se puede anexar al búfer.
  // No es culpa del navegador y cambiar de navegador no lo arregla.
  if (tipo.includes('mediaerror') || detalles.includes('buffer') || detalles.includes('parsing')) {
    return 'inestable'
  }

  return 'desconocido'
}

/**
 * Devuelve la clase que se puede afirmar HONESTAMENTE sobre el canal entero.
 *
 * Si todos los intentos fallaron por lo mismo, esa es la causa y se dice. Si
 * fallaron por cosas distintas, NO se elige una: se devuelve 'desconocido', que
 * el reproductor traduce al mensaje genérico ("puede estar caído, geo-bloqueado
 * o su dirección caducó") — una triple adivinanza declarada, en vez de una
 * certeza falsa.
 *
 * El bug que lo motiva (AMC 720p, reportado por el dueño el 2026-09-04): el
 * failover probó dos mirrors. El primero, el que la salud puso delante, estaba
 * vivo pero servía segmentos de 4 s en más de 12 s → 'desconocido'. El segundo
 * daba 404 → 'caducado'. Se mostraba la del ÚLTIMO intento, así que el usuario
 * leía «La dirección del canal caducó» sobre un canal cuyo problema real era un
 * origen impracticablemente lento. Quedarse con la más ESPECÍFICA tampoco vale:
 * daría el mismo «caducó». Con evidencia contradictoria, lo honesto es no
 * afirmar ninguna.
 */
export function claseConsensuada(clases: ClaseFallo[]): ClaseFallo {
  if (clases.length === 0) return 'desconocido'
  const primera = clases[0]
  return clases.every((c) => c === primera) ? primera : 'desconocido'
}
