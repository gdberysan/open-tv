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

function persistir(v: Idioma): void {
  try {
    localStorage.setItem(CLAVE_ALMACEN, v)
  } catch {
    // Ver arriba.
  }
  if (typeof document !== 'undefined') document.documentElement.lang = v
}

// $state, no un `writable` de svelte/store: un componente que solo LLAMA a
// t(clave) —sin leer `$idioma` él mismo, que es casi todos salvo App— nunca
// se enteraba de que el idioma había cambiado. `get(store)` dentro de t()
// devuelve una foto suelta, no una suscripción, así que ese componente no se
// re-renderizaba hasta el próximo cambio que SÍ le tocara un $state propio
// (p.ej. recargar la página). Con la runa, cualquier lectura de `actual`
// durante un render queda enganchada al grafo reactivo de Svelte 5 pase lo
// que pase por medio —incluida una función importada de otro módulo—, así
// que t() vuelve a ser reactivo en todas partes sin tocar cada componente que
// lo llama. Bug real que solo un render de verdad (el e2e de la Tarea 15)
// podía enseñar: un test que llama a t() a pelo, sin montar un componente,
// nunca pasa por el camino que fallaba.
const inicial = idiomaInicial()
persistir(inicial) // primer arranque: guarda el idioma detectado del navegador
let actual = $state<Idioma>(inicial)

export const idioma = {
  get actual(): Idioma {
    return actual
  },
  set actual(v: Idioma) {
    actual = v
    persistir(v)
  },
}

/** t traduce y sustituye {parametros}. */
export function t(clave: ClaveMensaje, params?: Record<string, string | number>): string {
  const texto = diccionarios[actual][clave]
  if (!params) return texto
  return texto.replace(/\{(\w+)\}/g, (crudo, nombre) =>
    nombre in params ? String(params[nombre]) : crudo,
  )
}
