import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/svelte'
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
async function asentar() {
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
