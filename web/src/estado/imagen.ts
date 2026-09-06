import { writable, type Readable } from 'svelte/store'
import type { CatalogSource } from '../datos/catalogo'

const DEBOUNCE_MS = 1000
// Tope máximo de espera (M5, revisión de rama completa): sin él, un flujo de
// refrescar() cada <1s (fallos consecutivos, p.ej.) postergaba cargar() para
// siempre — el debounce nunca se vaciaba. 5 s es plazo suficiente para que
// el usuario vea el tiempo-hasta-la-imagen actualizarse incluso en racha.
const MAX_ESPERA_MS = 5000

export interface ImagenStore extends Readable<Map<string, { imagenMs: number; sinImagen: boolean }>> {
  /** Pide el mapa entero y sustituye el store. Si fuente.imagen() rechaza,
   *  el mapa anterior queda intacto: más vale un dato viejo que ninguno. */
  cargar(): Promise<void>
  /** Debounced 1 s: varias llamadas seguidas dentro de la ventana colapsan
   *  en UNA sola cargar(). Pensado para refrescos disparados por eventos
   *  frecuentes (p.ej. cada fallo de reproducción). */
  refrescar(): void
}

/**
 * crearImagen mantiene un Map id → tiempo-hasta-la-imagen, alimentado por
 * GET /channels/imagen (Task 1-4). Sin localStorage: es un dato vivo del
 * gateway, no una preferencia del usuario — no tiene sentido persistirlo
 * entre sesiones.
 */
export function crearImagen(fuente: Pick<CatalogSource, 'imagen'>): ImagenStore {
  const store = writable(new Map<string, { imagenMs: number; sinImagen: boolean }>())
  let temporizador: ReturnType<typeof setTimeout> | null = null
  // Marca de la PRIMERA llamada sin resolver de la racha actual: sostiene el
  // tope de 5 s aunque refrescar() se siga llamando antes de que dispare.
  let inicioRacha: number | null = null

  async function cargar(): Promise<void> {
    try {
      const datos = await fuente.imagen?.()
      if (!datos) return
      store.set(new Map(Object.entries(datos)))
    } catch {
      // El mapa anterior queda tal cual: un fallo de red no debe vaciar la
      // única señal de tiempo-hasta-la-imagen que ya teníamos.
    }
  }

  function disparar(): void {
    if (temporizador) clearTimeout(temporizador)
    temporizador = null
    inicioRacha = null
    void cargar()
  }

  function refrescar(): void {
    const ahora = Date.now()
    if (inicioRacha === null) inicioRacha = ahora
    if (ahora - inicioRacha >= MAX_ESPERA_MS) {
      // El tope ya se cumplió: dispara YA en vez de reprogramar otra vez.
      disparar()
      return
    }
    if (temporizador) clearTimeout(temporizador)
    temporizador = setTimeout(disparar, DEBOUNCE_MS)
  }

  return { subscribe: store.subscribe, cargar, refrescar }
}
