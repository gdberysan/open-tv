import { describe, expect, it } from 'vitest'
import { fireEvent, render } from '@testing-library/svelte'
import RejillaCanales from './RejillaCanales.svelte'
import type { Canal } from '../datos/catalogo'

// Mismo doble que RejillaVirtual.test.ts / App.integracion.test.ts: jsdom no
// implementa IntersectionObserver, y RejillaCanales lo usa para el scroll
// infinito en modo lista.
class FalsoIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return []
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

// Fix final, hallazgo 1: la Tarea 7 arregló el fallback de logo roto
// (logoRoto/onerror → iniciales) en la rejilla (TarjetaCanal.svelte), pero la
// fila de la vista LISTA se quedó con un <img> suelto sin onerror — una
// logoUrl muerta (frecuente en el catálogo real) mostraba el icono de imagen
// rota del navegador en vez de las iniciales. Este test cubre justo eso:
// contra el código de antes (sin LogoCanal.svelte compartido, <img> suelto
// en la fila de lista) fallaría, porque fireEvent.error no dispararía ningún
// cambio de estado y el <img> roto seguiría en el DOM.
const canal: Canal = {
  id: 'x', nombre: 'BBC One', logoUrl: 'http://muerto/logo.png', categoriaId: '',
  idioma: '', pais: 'GB', vivo: true, latenciaMs: 100, webOk: true,
}

describe('RejillaCanales — modo lista', () => {
  it('cae al fallback de iniciales cuando la imagen de una fila falla', async () => {
    const { container } = render(RejillaCanales, {
      canales: [canal], vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {},
    })

    const img = container.querySelector('article.fila img')
    if (!img) throw new Error('no hay img')
    await fireEvent.error(img)

    expect(container.querySelector('article.fila img')).toBeNull()
    expect(container.querySelector('article.fila .sinlogo')?.textContent).toContain('BB')
  })

  it('renderiza el <img> normalmente mientras no falle', () => {
    const { container } = render(RejillaCanales, {
      canales: [canal], vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {},
    })

    expect(container.querySelector('article.fila img')?.getAttribute('src')).toBe(canal.logoUrl)
    expect(container.querySelector('article.fila .sinlogo')).toBeNull()
  })
})
