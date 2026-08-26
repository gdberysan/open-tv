import type { Canal } from '../datos/catalogo'

// Heurística sobre el nombre: las listas FTA marcan el geo-bloqueo en el
// título con patrones habituales. Sin red, sin heurística de IP: solo lo que
// el catálogo ya dice. Informar, nunca eludir.
const PATRONES = [/\[geo/i, /\(geo/i, /geo-?block/i, /\bonly\b/i, /solo\s+\w+\b/i]

export function pareceGeoBloqueado(canal: Canal): boolean {
  return PATRONES.some((p) => p.test(canal.nombre))
}
