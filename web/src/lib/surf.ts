// Gesto "surf" (Tarea 11, P0.6): barra espaciadora → canal vivo al azar
// dentro del filtro actual. debeHacerSurf es la guarda de foco a11y, PURA y
// testeable sin montar Svelte: separa "es un Space" de "puede robarle el
// espacio a un control real". Sin esta guarda, pulsar espacio para escribir
// un espacio en el buscador, para tildar una casilla o para activar un botón
// enfocado con teclado dispararía el salto — justo lo que un lector de
// pantalla o cualquier navegación por teclado no puede permitirse.
const ETIQUETAS_INTERACTIVAS = new Set(['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON'])

export function debeHacerSurf(e: KeyboardEvent): boolean {
  if (e.key !== ' ' && e.code !== 'Space') return false

  const objetivo = e.target
  if (!(objetivo instanceof HTMLElement)) return true

  if (ETIQUETAS_INTERACTIVAS.has(objetivo.tagName)) return false
  if (objetivo.isContentEditable) return false

  return true
}
