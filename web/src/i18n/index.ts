import { writable, get } from 'svelte/store'
import { es, type ClaveMensaje } from './es'
import { en } from './en'

export type Idioma = 'es' | 'en'

const diccionarios: Record<Idioma, Record<ClaveMensaje, string>> = { es, en }

const CLAVE_ALMACEN = 'opentv.idioma'

function idiomaInicial(): Idioma {
  try {
    const guardado = localStorage.getItem(CLAVE_ALMACEN)
    if (guardado === 'es' || guardado === 'en') return guardado
  } catch {
    // Navegación privada o almacenamiento bloqueado: el idioma del navegador
    // sigue sirviendo, no es motivo para fallar.
  }
  const nav = typeof navigator !== 'undefined' ? navigator.language : 'es'
  return nav.toLowerCase().startsWith('en') ? 'en' : 'es'
}

export const idioma = writable<Idioma>(idiomaInicial())

idioma.subscribe((v) => {
  try {
    localStorage.setItem(CLAVE_ALMACEN, v)
  } catch {
    // Ver arriba.
  }
  if (typeof document !== 'undefined') document.documentElement.lang = v
})

/** t traduce y sustituye {parametros}. */
export function t(clave: ClaveMensaje, params?: Record<string, string | number>): string {
  const texto = diccionarios[get(idioma)][clave]
  if (!params) return texto
  return texto.replace(/\{(\w+)\}/g, (crudo, nombre) =>
    nombre in params ? String(params[nombre]) : crudo,
  )
}
