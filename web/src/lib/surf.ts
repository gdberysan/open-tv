// Gesto "surf" (Tarea 11, P0.6): barra espaciadora → canal vivo al azar
// dentro del filtro actual. debeHacerSurf es la guarda de foco a11y, PURA y
// testeable sin montar Svelte: separa "es un Space" de "puede robarle el
// espacio a un control real". Sin esta guarda, pulsar espacio para escribir
// un espacio en el buscador, para tildar una casilla o para activar un botón
// enfocado con teclado dispararía el salto — justo lo que un lector de
// pantalla o cualquier navegación por teclado no puede permitirse.
const ETIQUETAS_INTERACTIVAS = new Set(['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON'])

// Extraído en el pase de a11y de P0.8 (Tarea 8): el mismo criterio de "¿este
// target ya está en manos de un control interactivo?" lo necesita CUALQUIER
// atajo global de teclado, no solo el surf — el ⌘K de la paleta de comandos
// (App.svelte, alTeclaVentana) tenía su propio guard ad-hoc que solo excluía
// INPUT/TEXTAREA (no SELECT/BUTTON/contenteditable, un hueco real: pulsar
// ⌘K con el foco en un <select> o un <button> — p. ej. el de densidad de
// Ajustes.svelte — abría la paleta encima). Un solo criterio, compartido.
export function esObjetivoInteractivo(objetivo: EventTarget | null): boolean {
  if (!(objetivo instanceof HTMLElement)) return false
  if (ETIQUETAS_INTERACTIVAS.has(objetivo.tagName)) return true
  return objetivo.isContentEditable === true
}

export function debeHacerSurf(e: KeyboardEvent): boolean {
  if (e.key !== ' ' && e.code !== 'Space') return false
  return !esObjetivoInteractivo(e.target)
}
