import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { render, fireEvent } from '@testing-library/svelte'
import RejillaVirtual from './RejillaVirtual.svelte'
import type { Canal } from '../datos/catalogo'

// RejillaVirtual observa un centinela con IntersectionObserver para el
// scroll infinito (igual que RejillaCanales); jsdom no lo implementa. Mismo
// doble que usa App.integracion.test.ts.
class FalsoIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return []
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalFalso(i: number): Canal {
  return {
    id: String(i), nombre: `Canal ${i}`, logoUrl: '', categoriaId: 'General',
    idioma: 'en', pais: 'GB', vivo: true, latenciaMs: 100, webOk: true,
  }
}

// jsdom no hace layout real (getBoundingClientRect/clientWidth siempre dan
// 0), así que la ventana cae al respaldo (ALTO_RESPALDO=220, 1 columna).
// Da igual el número exacto: lo que importa es que sea muchísimo menor que
// el total, y que no reviente con 0 tarjetas.
//
// El cálculo de la ventana se agenda con requestAnimationFrame (fix ronda 1,
// coalescencia por rAF): no basta con un setTimeout(0), porque el rAF de
// jsdom no resuelve hasta su propio tick — hay que esperarlo explícitamente
// antes de esperar también un tick de Svelte para que el estado se propague.
async function asentar() {
  await new Promise<number>((resolve) => requestAnimationFrame(resolve))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

describe('RejillaVirtual', () => {
  it('solo renderiza la ventana visible, no los 5000 canales', async () => {
    const canales = Array.from({ length: 5000 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const tarjetas = container.querySelectorAll('article')
    expect(tarjetas.length).toBeGreaterThan(0)
    expect(tarjetas.length).toBeLessThan(200)
  })

  it('renderiza todos los canales cuando son pocos', async () => {
    const canales = Array.from({ length: 3 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    expect(container.querySelectorAll('article').length).toBe(3)
  })

  it('sigue keyed por canal.id: cada tarjeta refleja el canal correcto', async () => {
    const canales = Array.from({ length: 5 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const nombres = [...container.querySelectorAll('.nombre')].map((n) => n.textContent)
    expect(nombres).toEqual(['Canal 0', 'Canal 1', 'Canal 2', 'Canal 3', 'Canal 4'])
  })

  it('no revienta con la lista vacía', async () => {
    const { container } = render(RejillaVirtual, { canales: [], alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    expect(container.querySelectorAll('article').length).toBe(0)
  })
})

// Tarea 5 (P0.8): densidad de la rejilla. jsdom no hace layout real (ver el
// comentario de arriba: clientWidth siempre da 0), así que sin más el
// contenedor mide 0 pase lo que pase ANCHO_MIN/anchoMin y columnas se queda
// en 1 siempre — no hay forma de que el test observe la diferencia. Se
// sobrescribe clientWidth en el PROTOTIPO de HTMLElement con un ancho fijo
// ANTES de renderizar (para que la primera medición del componente, dentro
// de su propio $effect, ya lo vea) y se restaura al terminar cada test, para
// no filtrar el mock a los tests de arriba (que dependen del 0 real).
describe('RejillaVirtual — densidad', () => {
  let descriptorOriginal: PropertyDescriptor | undefined

  beforeEach(() => {
    descriptorOriginal = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientWidth')
    Object.defineProperty(HTMLElement.prototype, 'clientWidth', { configurable: true, get: () => 800 })
  })

  afterEach(() => {
    if (descriptorOriginal) Object.defineProperty(HTMLElement.prototype, 'clientWidth', descriptorOriginal)
  })

  async function columnasCon(densidad: 'comoda' | 'compacta'): Promise<number> {
    const canales = Array.from({ length: 200 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {}, densidad })
    await asentar()
    const rejilla = container.querySelector('.rejilla-virtual') as HTMLElement
    return Number(rejilla.dataset.columnas)
  }

  it('compacta cabe más columnas que comoda para el mismo ancho de contenedor', async () => {
    const columnasComoda = await columnasCon('comoda')
    const columnasCompacta = await columnasCon('compacta')
    expect(columnasCompacta).toBeGreaterThan(columnasComoda)
  })

  // Mismo patrón que el roving tabindex de a11y.test.ts (Tarea 18): cambiar
  // la densidad no puede degradar el roving tabindex a 2·N tab stops, ni
  // dejar sin [data-indice] a la tarjeta activa.
  it('con densidad compacta, el roving tabindex sigue reduciendo la rejilla a 2 tab stops (los de la tarjeta activa)', async () => {
    const canales = Array.from({ length: 5 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, {
      canales, alAbrir: () => {}, alPedirMas: () => {}, densidad: 'compacta',
    })
    await asentar()

    const abrir = () => [...container.querySelectorAll<HTMLElement>('article .abrir')]
    expect(abrir()[0].getAttribute('tabindex')).toBe('0')
    expect(abrir()[1].getAttribute('tabindex')).toBe('-1')
    expect(container.querySelectorAll('article [tabindex="0"]').length).toBe(2) // abrir + favorito de la activa, nada más
    expect(container.querySelector('article')?.getAttribute('data-indice')).toBe('0')

    abrir()[0].focus()
    await fireEvent.keyDown(abrir()[0], { key: 'ArrowRight' })
    await asentar()

    expect(document.activeElement).toBe(abrir()[1])
    expect(abrir()[0].getAttribute('tabindex')).toBe('-1')
    expect(abrir()[1].getAttribute('tabindex')).toBe('0')
    expect(container.querySelectorAll('article [tabindex="0"]').length).toBe(2)
  })
})
