// Fix round 1 (Tarea 9, P0.7): las facetas de país que sirve el catálogo son
// códigos ISO 3166-1 alpha-2 crudos ("MX", "US"…). Mostrarlos tal cual es
// mucho menos legible que un nombre, y además rompe el buscador de tipeo: la
// entrada natural del usuario es "méxico"/"mex", no "mx". Intl.DisplayNames
// es una API nativa del motor (sin dependencia nueva) que traduce un código
// de región al nombre en el locale pedido.

// Una instancia de Intl.DisplayNames por locale: crearla es más caro que una
// búsqueda de Map, y aquí se llama una vez por fila renderizada.
const instanciasPorLocale = new Map<string, Intl.DisplayNames>()

function instanciaPara(locale: string): Intl.DisplayNames | undefined {
  let instancia = instanciasPorLocale.get(locale)
  if (instancia) return instancia
  try {
    instancia = new Intl.DisplayNames([locale], { type: 'region' })
    instanciasPorLocale.set(locale, instancia)
    return instancia
  } catch {
    // Intl.DisplayNames ausente en el entorno (motor viejo, algún runtime de
    // test): cae al código en vez de romper el render.
    return undefined
  }
}

/**
 * Traduce un código de país (p. ej. "MX") a su nombre en el locale dado (p.
 * ej. "México" en 'es', "Mexico" en 'en'). Si el código no se reconoce (raro,
 * tipo "XK", o vacío) o Intl.DisplayNames no está disponible, devuelve el
 * código tal cual — nunca deja una fila sin texto.
 */
export function nombreDePais(codigo: string, locale: string): string {
  if (!codigo) return codigo
  const instancia = instanciaPara(locale)
  if (!instancia) return codigo
  try {
    return instancia.of(codigo) ?? codigo
  } catch {
    return codigo
  }
}
