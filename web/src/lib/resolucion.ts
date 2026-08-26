// Heurística sobre el nombre del canal: el catálogo no trae un campo de
// resolución propio (ver Canal en datos/catalogo.ts), pero las listas FTA
// suelen anotarla en el título entre paréntesis o como sufijo — mismo
// enfoque que pareceGeoBloqueado en geo.ts. Sin match, sin badge: no
// inventamos una resolución que el nombre no dice.
const PATRON_NNNp = /\b(\d{3,4})p\b/i
const PATRON_ETIQUETA = /\b(4k|8k|uhd|fhd|hd|sd)\b/i

export function parsearResolucion(nombre: string): string | null {
  const nnnp = PATRON_NNNp.exec(nombre)
  if (nnnp) return `${nnnp[1]}p`
  const etiqueta = PATRON_ETIQUETA.exec(nombre)
  if (etiqueta) return etiqueta[1].toUpperCase()
  return null
}
