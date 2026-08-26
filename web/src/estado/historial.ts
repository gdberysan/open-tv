import { writable, type Readable } from 'svelte/store'
import type { Canal } from '../datos/catalogo'

const CLAVE = 'opentv.historial'
const TOPE = 24

export interface EntradaHistorial {
  canalId: string
  nombre: string
  sigla?: string
  logoUrl?: string
  /** epoch ms */
  cuando: number
  /** Reservado: los streams en vivo no tienen posición. No se usa en v1. */
  posicionS?: number
}

export interface Historial extends Readable<EntradaHistorial[]> {
  registrar(c: Canal): void
  borrar(): void
}

function leer(): EntradaHistorial[] {
  try {
    const crudo = localStorage.getItem(CLAVE)
    if (!crudo) return []
    const datos = JSON.parse(crudo)
    return Array.isArray(datos)
      ? datos.filter((x) => x && typeof x.canalId === 'string' && typeof x.cuando === 'number')
      : []
  } catch {
    // Basura en localStorage o almacenamiento bloqueado. Empezar de cero es
    // molesto; dejar la app en blanco es un fallo.
    return []
  }
}

export function crearHistorial(reloj: () => number = () => Date.now()): Historial {
  const store = writable<EntradaHistorial[]>(leer())

  store.subscribe((lista) => {
    try {
      localStorage.setItem(CLAVE, JSON.stringify(lista))
    } catch {
      // Ver arriba.
    }
  })

  return {
    ...store,
    registrar(c: Canal) {
      store.update((lista) => {
        const entrada: EntradaHistorial = {
          canalId: c.id,
          nombre: c.nombre,
          sigla: c.nombre.slice(0, 2),
          logoUrl: c.logoUrl,
          cuando: reloj(),
        }
        const resto = lista.filter((e) => e.canalId !== c.id)
        return [entrada, ...resto].slice(0, TOPE)
      })
    },
    borrar() {
      store.set([])
    },
  }
}

export const historial = crearHistorial()
