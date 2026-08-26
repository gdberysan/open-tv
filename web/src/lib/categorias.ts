// Iconos identificativos por categoría. Las categorías del catálogo son el
// set fijo de iptv-org (News, Sports, Movies…), texto crudo en inglés que
// llega en group-title. Un emoji por categoría conocida evita añadir un set
// de SVG (dependencia + peso de bundle) y es CSP-safe; lo desconocido cae a un
// icono por defecto para que ninguna fila quede sin marca.

// Clave: nombre de categoría en minúsculas. Cubre las 30 categorías de
// iptv-org; cualquier otra (o "Undefined") usa ICONO_DEFECTO.
const ICONOS: Record<string, string> = {
  news: '📰',
  sports: '⚽',
  movies: '🎬',
  music: '🎵',
  kids: '🧸',
  religious: '⛪',
  documentary: '🎥',
  entertainment: '🎭',
  comedy: '😂',
  series: '🎞️',
  animation: '🎨',
  education: '🎓',
  culture: '🏛️',
  legislative: '⚖️',
  lifestyle: '🛋️',
  cooking: '🍳',
  business: '💼',
  outdoor: '🏕️',
  travel: '✈️',
  family: '👪',
  auto: '🚗',
  science: '🔬',
  weather: '🌤️',
  relax: '🧘',
  shop: '🛒',
  classic: '📽️',
  public: '📡',
  interactive: '🕹️',
  general: '📺',
}

// Marca neutra para "Undefined" y cualquier categoría fuera del set conocido.
export const ICONO_DEFECTO = '🏷️'

/**
 * Devuelve el emoji identificativo de una categoría. Case-insensitive. Para
 * categorías compuestas ("News;General", "Movies,Series") gana el PRIMER
 * segmento conocido; si ninguno se reconoce (o la cadena está vacía), devuelve
 * ICONO_DEFECTO — nunca cadena vacía, para que toda fila lleve icono.
 */
export function iconoDeCategoria(categoria: string): string {
  if (!categoria) return ICONO_DEFECTO
  const segmentos = categoria.split(/[;,|]/)
  for (const seg of segmentos) {
    const icono = ICONOS[seg.trim().toLowerCase()]
    if (icono) return icono
  }
  return ICONO_DEFECTO
}
