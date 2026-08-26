import { writable, type Writable } from 'svelte/store'

const CLAVE = 'opentv.preferencias'

export interface Preferencias {
  /** 'comoda' (por defecto, más aire por tarjeta) o 'compacta' (más columnas
   * en la rejilla, tarjetas más pequeñas). Tarea 5 (P0.8): solo el store y el
   * cableado a RejillaVirtual — la UI para cambiarla es la Tarea 6 (#ajustes). */
  densidad: 'comoda' | 'compacta'
  /** Reservado para la Tarea 6: recordar la última vista (rejilla/lista) entre
   * sesiones. No se lee ni se escribe todavía. */
  recordarVista: boolean
  /** Reservado para la Tarea 6: recordar los filtros activos entre sesiones.
   * No se lee ni se escribe todavía. */
  recordarFiltros: boolean
}

const PREFERENCIAS_DEFECTO: Preferencias = {
  densidad: 'comoda',
  recordarVista: true,
  recordarFiltros: false,
}

export interface AlmacenPreferencias extends Writable<Preferencias> {
  actualizar(cambios: Partial<Preferencias>): void
}

function esDensidadValida(x: unknown): x is Preferencias['densidad'] {
  return x === 'comoda' || x === 'compacta'
}

function leer(): Preferencias {
  try {
    const crudo = localStorage.getItem(CLAVE)
    if (!crudo) return { ...PREFERENCIAS_DEFECTO }
    const datos = JSON.parse(crudo)
    if (!datos || typeof datos !== 'object') return { ...PREFERENCIAS_DEFECTO }
    // Se valida campo a campo (no un JSON.parse "a ciegas"): una clave
    // corrupta o de una versión futura no puede tumbar las demás — cada una
    // cae a su propio valor por defecto si no tiene la forma esperada.
    return {
      densidad: esDensidadValida((datos as Record<string, unknown>).densidad)
        ? (datos as Record<string, unknown>).densidad as Preferencias['densidad']
        : PREFERENCIAS_DEFECTO.densidad,
      recordarVista: typeof (datos as Record<string, unknown>).recordarVista === 'boolean'
        ? (datos as Record<string, unknown>).recordarVista as boolean
        : PREFERENCIAS_DEFECTO.recordarVista,
      recordarFiltros: typeof (datos as Record<string, unknown>).recordarFiltros === 'boolean'
        ? (datos as Record<string, unknown>).recordarFiltros as boolean
        : PREFERENCIAS_DEFECTO.recordarFiltros,
    }
  } catch {
    // Basura en localStorage o almacenamiento bloqueado. Empezar de cero es
    // molesto; dejar la app en blanco es un fallo. Mismo criterio que
    // favoritos.ts/historial.ts.
    return { ...PREFERENCIAS_DEFECTO }
  }
}

export function crearPreferencias(): AlmacenPreferencias {
  const store = writable<Preferencias>(leer())

  store.subscribe((p) => {
    try {
      localStorage.setItem(CLAVE, JSON.stringify(p))
    } catch {
      // Ver arriba.
    }
  })

  return {
    ...store,
    actualizar(cambios: Partial<Preferencias>) {
      store.update((p) => ({ ...p, ...cambios }))
    },
  }
}

export const preferencias = crearPreferencias()
