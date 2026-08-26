// Matcher difuso para el command palette (Tarea 3, P0.8): decide si `termino`
// "casa" con `texto` y, si casa, devuelve un puntaje para ordenar resultados
// (mayor = mejor). No es búsqueda exacta ni por substring: es por
// subsecuencia — cada carácter de `termino`, en orden, debe aparecer en
// `texto`, pero no hace falta que sean contiguos. Así "mx" casa con "México"
// y "cnn" casa con "CNN en Español".

// Normaliza para comparar sin distinguir mayúsculas ni acentos: 'mex' debe
// casar con 'México'. NFD separa cada letra de sus diacríticos combinantes
// (U+0300–U+036F), que el replace descarta. Mismo patrón que
// BarraLateralFacetas.svelte (normalizarTexto) — se duplica aquí porque esta
// lib es pura y no importa de un componente Svelte.
function normalizar(valor: string): string {
  return valor.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
}

const LIMITES_PALABRA = new Set([' ', '·', '-'])

// Puntaje neutro para término vacío: la paleta de comandos, con la caja
// vacía, muestra todo sin ordenar por relevancia — cualquier texto "casa"
// con el mismo puntaje (0).
const PUNTAJE_TERMINO_VACIO = 0

const PUNTOS_POR_CARACTER = 1
const BONUS_CONSECUTIVO = 15
const BONUS_INICIO = 10
const BONUS_LIMITE_PALABRA = 8
// Penaliza según el largo de `texto`: entre dos matches equivalentes, gana
// el de la etiqueta más corta (un match en "CNN" > el mismo match perdido
// en un párrafo largo).
const PENALIZACION_POR_CARACTER_TEXTO = 0.1

/**
 * Matcher difuso por subsecuencia, caso/acento-insensible, con puntaje.
 *
 * Devuelve `null` si `termino` no es subsecuencia de `texto`. Si casa,
 * devuelve un número (mayor = mejor) que premia: matches consecutivos,
 * matches al inicio del texto o justo tras un límite de palabra (espacio,
 * '·' o '-'), y textos más cortos.
 *
 * Nota de implementación: usa la primera ocurrencia disponible de cada
 * carácter (greedy, de izquierda a derecha) en vez de explorar todas las
 * subsecuencias posibles — más simple y suficientemente bueno para
 * etiquetas cortas de UI; no garantiza el puntaje óptimo global en textos
 * largos con múltiples ocurrencias del mismo carácter.
 *
 * Término vacío (tras trim) casa con cualquier texto con puntaje neutro
 * (constante `PUNTAJE_TERMINO_VACIO` = 0): la paleta de comandos, con la
 * caja de búsqueda vacía, muestra todo.
 */
export function coincideDifuso(termino: string, texto: string): number | null {
  const q = normalizar(termino.trim())
  if (!q) return PUNTAJE_TERMINO_VACIO

  const t = normalizar(texto)

  let puntaje = 0
  let posicionAnterior = -1

  for (const caracter of q) {
    const posicion = t.indexOf(caracter, posicionAnterior + 1)
    if (posicion === -1) return null

    puntaje += PUNTOS_POR_CARACTER
    if (posicion === 0) {
      puntaje += BONUS_INICIO
    } else if (LIMITES_PALABRA.has(t[posicion - 1])) {
      puntaje += BONUS_LIMITE_PALABRA
    }
    if (posicionAnterior !== -1 && posicion === posicionAnterior + 1) {
      puntaje += BONUS_CONSECUTIVO
    }

    posicionAnterior = posicion
  }

  puntaje -= t.length * PENALIZACION_POR_CARACTER_TEXTO
  return puntaje
}
