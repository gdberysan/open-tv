export type NivelSenal = 'desconocido' | 'muerta' | 'buena' | 'media' | 'pobre'

/**
 * Mismos umbrales que la app de macOS. Si se cambian aquí hay que cambiarlos
 * allí: el mismo canal no puede verse "bien" en un cliente y "regular" en el
 * otro.
 */
export function nivelSenal(vivo: boolean | null, latenciaMs: number): NivelSenal {
  if (vivo == null) return 'desconocido'
  if (!vivo) return 'muerta'
  if (latenciaMs < 200) return 'buena'
  if (latenciaMs <= 800) return 'media'
  return 'pobre'
}
