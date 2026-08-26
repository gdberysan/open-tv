import { beforeEach, describe, expect, it } from 'vitest'
import { fireEvent, render } from '@testing-library/svelte'
import { writable } from 'svelte/store'
import RejillaCanales from './RejillaCanales.svelte'
import type { AhoraDespues, Canal } from '../datos/catalogo'
import { idioma } from '../i18n'

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
  // El idioma inicial sale de navigator.language (jsdom suele reportar
  // en-US); se fija a 'es' para que la etiqueta esperada de SenalCanal no
  // dependa del entorno de test (mismo patrón que TarjetaCanal.test.ts).
  beforeEach(() => {
    idioma.actual = 'es'
  })

  // Fix 1 (Tarea 8): la fila de lista usaba BarrasSenal (el medidor de 3
  // barras) mientras la rejilla ya hablaba en punto+ms — dos lenguajes de
  // señal distintos para el mismo dato. Falsable contra el código de antes
  // (BarrasSenal en la fila): ese componente no pinta ningún .punto ni
  // aria-label de estado, y SÍ marca su nivel con [data-nivel] en las tres
  // barras — justo lo que este test comprueba que ya no está.
  it('muestra el punto de SenalCanal en la fila (no las barras de BarrasSenal)', () => {
    const { container } = render(RejillaCanales, {
      canales: [canal], vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {},
    })

    const punto = container.querySelector('article.fila .punto')
    expect(punto?.classList.contains('vivo')).toBe(true)
    expect(punto?.getAttribute('role')).toBe('img')
    expect(punto?.getAttribute('aria-label')).toBe('Señal viva')
    expect(container.querySelector('article.fila .ms')?.textContent).toBe('100 ms')
    // BarrasSenal (retirado) marcaba su nivel con data-nivel en un <span
    // class="senal">: su ausencia confirma que la fila ya no lo usa.
    expect(container.querySelector('article.fila [data-nivel]')).toBeNull()
    expect(container.querySelector('article.fila .senal')).toBeNull()
  })

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

  // Tarea (paridad de EPG): la insignia ahora/después de TarjetaCanal (P2)
  // también en la fila de la vista lista. Mismo doble de test que
  // TarjetaCanal.test.ts / RejillaVirtual.test.ts: un `writable` de
  // svelte/store basta como Readable<Map<string, AhoraDespues>>.
  describe('insignia ahora/después (EPG) — modo lista', () => {
    it('con guía presente: muestra "Ahora: <título>" en la fila', () => {
      const ahoraDespues: AhoraDespues = {
        ahora: { titulo: 'Telediario', inicioSeg: 1_700_000_000, finSeg: 1_700_003_000 },
        siguiente: null,
      }
      const epg = writable(new Map([['x', ahoraDespues]]))
      const { container } = render(RejillaCanales, {
        canales: [canal], vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {}, epg,
      })

      const texto = container.querySelector('article.fila .linea-epg')?.textContent ?? ''
      expect(texto).toContain('Ahora: Telediario')
      // La insignia visual es aria-hidden; el nombre accesible del botón lleva
      // la guía (un aria-label explícito anula el texto interno para el lector
      // de pantalla, así que sin esto la guía no se anunciaría en la lista).
      const boton = container.querySelector('article.fila button.abrir')
      expect(container.querySelector('article.fila .linea-epg')?.getAttribute('aria-hidden')).toBe('true')
      expect(boton?.getAttribute('aria-label')).toContain('Ahora: Telediario')
    })

    it('sin guía para este canal: la fila no muestra texto de EPG', () => {
      const epg = writable(new Map<string, AhoraDespues>())
      const { container } = render(RejillaCanales, {
        canales: [canal], vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {}, epg,
      })

      expect(container.querySelector('article.fila .linea-epg')?.textContent?.trim() ?? '').toBe('')
    })

    it('modo lista: llama a alVisiblesCambiar con los ids de los canales cargados', () => {
      const llamadas: string[][] = []
      render(RejillaCanales, {
        canales: [canal], vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {},
        alVisiblesCambiar: (ids: string[]) => llamadas.push(ids),
      })

      expect(llamadas.length).toBeGreaterThan(0)
      expect(llamadas[llamadas.length - 1]).toEqual(['x'])
    })
  })
})
