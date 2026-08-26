import { writable, type Readable } from 'svelte/store'
import type { AhoraDespues, CatalogSource } from '../datos/catalogo'

// La guía es un dato que envejece rápido ("ahora" cambia de programa varias
// veces por hora) pero no vale la pena refrescar en cada render: 5 min de
// TTL amortiguan de sobra el coste de un /channels/epg en lote sin que la
// tarjeta muestre un "ahora" claramente caducado.
const TTL_MS = 5 * 60_000
// El refresco periódico existe SOLO para que "ahora" no se quede viejo
// cuando el usuario no toca nada (no cambia de filtro, no hace scroll): sin
// esto, un canal cuyo set visible es estable nunca volvería a pedir tras el
// primer TTL.
const INTERVALO_REFRESCO_MS = 2 * 60_000

interface EntradaCache {
  // null = "consultado, sin guía" (id ausente de la respuesta del gateway).
  // Se cachea igual que un valor real para no re-pedir ese id en cada
  // llamada a asegurar() dentro del mismo TTL.
  valor: AhoraDespues | null
  expiraEn: number
}

export interface CrearEpgOpciones {
  ttlMs?: number
  intervaloMs?: number
  /** Inyectable para tests: evita depender de Date.now() real. */
  reloj?: () => number
}

export interface Epg extends Readable<Map<string, AhoraDespues>> {
  /**
   * Dado el conjunto de channelIDs visibles, pide en UN solo lote
   * (epgDeCanales) los que falten en caché o cuyo TTL venció — nunca una
   * petición por id. Un fallo de la fuente se traga (try/catch, mismo
   * criterio que favoritos.ts/salud.ts): esos ids quedan sin dato en vez de
   * romper el render, y se reintentan en la siguiente llamada.
   */
  asegurar(ids: string[]): Promise<void>
  /** Lectura puntual y síncrona. undefined = sin guía (id nunca visto o
   * confirmado ausente de la respuesta del gateway) — ambos casos se leen
   * igual desde aquí; ver el store (Readable) para reactividad de verdad. */
  ahoraDespuesDe(id: string): AhoraDespues | undefined
  /** Para el intervalo de refresco periódico. Idempotente. Para tests y al
   * desmontar el componente que posee este store. */
  detener(): void
}

/**
 * crearEpg es una factoría, no un singleton (a diferencia de favoritos.ts o
 * historial.ts): a diferencia de esos, este store necesita un CatalogSource
 * para hablar con el gateway, y el CatalogSource es inyectable por quien
 * monta la app (App.svelte, Tarea 15) — un singleton atado a
 * crearHttpCatalog('') no sería sustituible en tests de componente. Quien
 * cree el store es responsable de llamar a detener() al desmontar.
 */
export function crearEpg(fuente: CatalogSource, opciones: CrearEpgOpciones = {}): Epg {
  const ttlMs = opciones.ttlMs ?? TTL_MS
  const intervaloMs = opciones.intervaloMs ?? INTERVALO_REFRESCO_MS
  const reloj = opciones.reloj ?? (() => Date.now())

  const cache = new Map<string, EntradaCache>()
  const store = writable<Map<string, AhoraDespues>>(new Map())
  // Último set de ids pedido: el intervalo lo re-pide sin que el llamador
  // tenga que acordarse de pasarlo de nuevo.
  let visibles: string[] = []

  function publicar(): void {
    const publico = new Map<string, AhoraDespues>()
    for (const [id, entrada] of cache) {
      if (entrada.valor) publico.set(id, entrada.valor)
    }
    store.set(publico)
  }

  async function asegurar(ids: string[]): Promise<void> {
    visibles = ids
    const ahora = reloj()
    // Coalescencia: se calcula UNA lista de ids que faltan y se pide en UNA
    // sola llamada a epgDeCanales — nunca una petición por id suelto.
    const faltan = ids.filter((id) => {
      const entrada = cache.get(id)
      return !entrada || entrada.expiraEn <= ahora
    })
    if (faltan.length === 0) return

    try {
      const resultado = await fuente.epgDeCanales(faltan)
      const expiraEn = reloj() + ttlMs
      for (const id of faltan) {
        // resultado.get(id) === undefined → sin guía posible (ausente del
        // cable); se cachea como null, no se deja "sin visitar", para no
        // volver a pedirlo en cada asegurar() dentro del TTL.
        cache.set(id, { valor: resultado.get(id) ?? null, expiraEn })
      }
      publicar()
    } catch {
      // Fallo de red o del gateway: no se cachea nada para `faltan` — quedan
      // sin dato (ahoraDespuesDe → undefined) y se reintentan la próxima vez
      // que algo llame a asegurar() (el propio intervalo, o un cambio del
      // set visible). Igual que el resto de stores: un fallo de red no
      // puede romper el render.
    }
  }

  function ahoraDespuesDe(id: string): AhoraDespues | undefined {
    return cache.get(id)?.valor ?? undefined
  }

  const intervalo = setInterval(() => {
    void asegurar(visibles)
  }, intervaloMs)

  function detener(): void {
    clearInterval(intervalo)
  }

  return { ...store, asegurar, ahoraDespuesDe, detener }
}
