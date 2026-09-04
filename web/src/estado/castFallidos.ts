// Memoria de qué canales NO se pueden castear por AirPlay porque el motor
// nativo (forzado durante una sesión de cast, ver Reproductor.svelte) los
// rechaza por formato/códec — nunca por un fallo de red o timeout, que son
// transitorios (ver spec §6.2). Sin store de Svelte a propósito: se lee solo
// al INICIAR un intento de cast, no hace falta reactividad — mismo motivo
// por el que no hay insignia en la rejilla (fuera de alcance, spec §9).
//
// Mismo patrón defensivo que favoritos.ts: localStorage puede estar
// bloqueado (modo privado) o contener basura; en ningún caso debe romper el
// intento de castear.
const CLAVE = 'opentv.cast.sinFormato'

function leer(): Set<string> {
  try {
    const crudo = localStorage.getItem(CLAVE)
    if (!crudo) return new Set()
    const datos = JSON.parse(crudo)
    return Array.isArray(datos) ? new Set(datos.filter((x) => typeof x === 'string')) : new Set()
  } catch {
    return new Set()
  }
}

export function noCasteaPorFormato(canalId: string): boolean {
  return leer().has(canalId)
}

export function marcarFalloFormato(canalId: string): void {
  try {
    const s = leer()
    s.add(canalId)
    localStorage.setItem(CLAVE, JSON.stringify([...s]))
  } catch {
    // Ver arriba: localStorage bloqueado no es fatal, solo se pierde la
    // optimización de saltar el reintento la próxima vez.
  }
}
