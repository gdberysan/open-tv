import type { ModoVista } from './filtros'

// Tarea 6 (P0.8): «recordar vista» y «recordar filtros» (Ajustes.svelte) son
// dos preferencias INDEPENDIENTES entre sí y de `preferencias.ts` — cada una
// gobierna su propia clave de localStorage, escrita/leída/borrada por App
// (ver App.svelte: restaurarSesionGuardada + los dos $effect de persistencia).
// Este módulo solo sabe leer/escribir/borrar esas dos claves con el mismo
// blindaje try/catch que preferencias.ts/favoritos.ts — nunca decide POR SÍ
// SOLO si debe persistir: eso lo decide App mirando `preferencias`.
const CLAVE_VISTA = 'opentv.vista'
const CLAVE_FILTROS = 'opentv.filtros'

export interface FiltrosGuardados {
  q: string
  pais: string
  categoria: string
  calidad: string
  soloFavoritos: boolean
}

function esVistaValida(x: unknown): x is ModoVista {
  return x === 'rejilla' || x === 'lista'
}

export function leerVistaGuardada(): ModoVista | null {
  try {
    const crudo = localStorage.getItem(CLAVE_VISTA)
    if (!crudo) return null
    const datos = JSON.parse(crudo)
    return esVistaValida(datos) ? datos : null
  } catch {
    // Basura en localStorage o almacenamiento bloqueado: sin vista guardada
    // que restaurar, App se queda con el valor por defecto del store — nunca
    // en blanco.
    return null
  }
}

export function escribirVistaGuardada(vista: ModoVista): void {
  try {
    localStorage.setItem(CLAVE_VISTA, JSON.stringify(vista))
  } catch {
    // Ver arriba.
  }
}

export function borrarVistaGuardada(): void {
  try {
    localStorage.removeItem(CLAVE_VISTA)
  } catch {
    // Ver arriba.
  }
}

// Se valida campo a campo (mismo criterio que preferencias.ts): una clave
// corrupta no puede tumbar las demás, y un objeto sin forma reconocible cae a
// "nada que restaurar" en vez de a un EstadoFiltros a medias.
export function leerFiltrosGuardados(): FiltrosGuardados | null {
  try {
    const crudo = localStorage.getItem(CLAVE_FILTROS)
    if (!crudo) return null
    const datos = JSON.parse(crudo)
    if (!datos || typeof datos !== 'object') return null
    const d = datos as Record<string, unknown>
    return {
      q: typeof d.q === 'string' ? d.q : '',
      pais: typeof d.pais === 'string' ? d.pais : '',
      categoria: typeof d.categoria === 'string' ? d.categoria : '',
      calidad: typeof d.calidad === 'string' ? d.calidad : '',
      soloFavoritos: typeof d.soloFavoritos === 'boolean' ? d.soloFavoritos : false,
    }
  } catch {
    return null
  }
}

export function escribirFiltrosGuardados(f: FiltrosGuardados): void {
  try {
    localStorage.setItem(CLAVE_FILTROS, JSON.stringify(f))
  } catch {
    // Ver arriba.
  }
}

export function borrarFiltrosGuardados(): void {
  try {
    localStorage.removeItem(CLAVE_FILTROS)
  } catch {
    // Ver arriba.
  }
}
