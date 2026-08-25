import { writable, type Writable } from 'svelte/store'

const CLAVE = 'opentv.favoritos'

export interface Favoritos extends Writable<Set<string>> {
  alternar(id: string): void
}

function leer(): Set<string> {
  try {
    const crudo = localStorage.getItem(CLAVE)
    if (!crudo) return new Set()
    const datos = JSON.parse(crudo)
    return Array.isArray(datos) ? new Set(datos.filter((x) => typeof x === 'string')) : new Set()
  } catch {
    // Basura en localStorage o almacenamiento bloqueado. Empezar de cero es
    // molesto; dejar la app en blanco es un fallo.
    return new Set()
  }
}

export function crearFavoritos(): Favoritos {
  const store = writable<Set<string>>(leer())

  store.subscribe((s) => {
    try {
      localStorage.setItem(CLAVE, JSON.stringify([...s]))
    } catch {
      // Ver arriba.
    }
  })

  return {
    ...store,
    alternar(id: string) {
      store.update((s) => {
        const nuevo = new Set(s)
        if (!nuevo.delete(id)) nuevo.add(id)
        return nuevo
      })
    },
  }
}

export const favoritos = crearFavoritos()
