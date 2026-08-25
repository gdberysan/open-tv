export interface Debounced<A extends unknown[]> {
  (...args: A): void
  cancelar(): void
}

/** debounce para la búsqueda: sin él, cada tecla es una consulta al catálogo. */
export function debounce<A extends unknown[]>(fn: (...args: A) => void, ms: number): Debounced<A> {
  let temporizador: ReturnType<typeof setTimeout> | undefined

  const envuelto = (...args: A) => {
    if (temporizador) clearTimeout(temporizador)
    temporizador = setTimeout(() => fn(...args), ms)
  }
  envuelto.cancelar = () => {
    if (temporizador) clearTimeout(temporizador)
    temporizador = undefined
  }
  return envuelto as Debounced<A>
}
