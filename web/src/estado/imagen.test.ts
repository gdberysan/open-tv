import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { get } from 'svelte/store'
import { crearImagen } from './imagen'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('crearImagen', () => {
  it('cargar() rellena el mapa desde fuente.imagen()', async () => {
    const fuente = { imagen: vi.fn(async () => ({ c1: { imagenMs: 1200, sinImagen: false } })) }
    const store = crearImagen(fuente)

    await store.cargar()

    expect(get(store).get('c1')).toEqual({ imagenMs: 1200, sinImagen: false })
    expect(fuente.imagen).toHaveBeenCalledOnce()
  })

  it('dos refrescar() seguidos dentro del debounce provocan UNA sola llamada', async () => {
    const fuente = { imagen: vi.fn(async () => ({ c1: { imagenMs: 500, sinImagen: false } })) }
    const store = crearImagen(fuente)

    store.refrescar()
    store.refrescar()

    await vi.advanceTimersByTimeAsync(1000)

    expect(fuente.imagen).toHaveBeenCalledOnce()
    expect(get(store).get('c1')).toEqual({ imagenMs: 500, sinImagen: false })
  })

  // M5 (revisión de rama completa): refrescar() sin tope se reprogramaba
  // para siempre si llegaba más rápido que el debounce (1s) — una racha de
  // fallos consecutivos podía dejar cargar() sin disparar nunca.
  it('refrescar() cada 500ms durante 6s dispara cargar() al llegar a los 5s', async () => {
    const fuente = { imagen: vi.fn(async () => ({ c1: { imagenMs: 700, sinImagen: false } })) }
    const store = crearImagen(fuente)

    for (let ms = 0; ms < 6000; ms += 500) {
      store.refrescar()
      await vi.advanceTimersByTimeAsync(500)
    }

    expect(fuente.imagen).toHaveBeenCalled()
  })

  it('un imagen() que rechaza deja el mapa anterior intacto', async () => {
    const fuente = { imagen: vi.fn(async () => ({ c1: { imagenMs: 900, sinImagen: false } })) }
    const store = crearImagen(fuente)
    await store.cargar()
    expect(get(store).get('c1')).toEqual({ imagenMs: 900, sinImagen: false })

    fuente.imagen.mockRejectedValueOnce(new Error('gateway inalcanzable'))
    await store.cargar()

    expect(get(store).get('c1')).toEqual({ imagenMs: 900, sinImagen: false })
  })
})
