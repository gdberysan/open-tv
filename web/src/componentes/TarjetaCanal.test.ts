import { beforeEach, describe, expect, it } from 'vitest'
import { fireEvent } from '@testing-library/svelte'
import { render, screen } from '@testing-library/svelte'
import TarjetaCanal from './TarjetaCanal.svelte'
import type { Canal } from '../datos/catalogo'
import { idioma } from '../i18n'

const base: Canal = {
  id: 'x', nombre: 'BBC One', logoUrl: '', categoriaId: 'General',
  idioma: 'en', pais: 'GB', vivo: true, latenciaMs: 120, webOk: true,
}

describe('TarjetaCanal', () => {
  // El idioma inicial sale de navigator.language (jsdom suele reportar
  // en-US); se fija a 'es' para que las etiquetas esperadas del punto de
  // salud no dependan del entorno de test (mismo patrón que
  // IndicadorSenal.test.ts).
  beforeEach(() => {
    idioma.actual = 'es'
  })

  // Tarea 8 (P0.6): el punto de salud + ms sustituye a BarrasSenal. Falsable
  // porque BarrasSenal (la tarjeta vieja) no tenía ningún punto con
  // aria-label por estado — solo tres barras sin nombre accesible propio.
  it('vivo===true: punto con clase "vivo" y etiqueta "Señal viva"', () => {
    const { container } = render(TarjetaCanal, { canal: base, alAbrir: () => {} })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('vivo')).toBe(true)
    expect(punto?.getAttribute('aria-label')).toBe('Señal viva')
  })

  it('vivo===false: punto con clase "muerta" y etiqueta "Sin respuesta"', () => {
    const { container } = render(TarjetaCanal, { canal: { ...base, vivo: false }, alAbrir: () => {} })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('muerta')).toBe(true)
    expect(punto?.getAttribute('aria-label')).toBe('Sin respuesta')
  })

  it('vivo===null: punto con clase "desconocido" y etiqueta "Sin comprobar"', () => {
    const { container } = render(TarjetaCanal, { canal: { ...base, vivo: null }, alAbrir: () => {} })
    const punto = container.querySelector('.punto')
    expect(punto?.classList.contains('desconocido')).toBe(true)
    expect(punto?.getAttribute('aria-label')).toBe('Sin comprobar')
  })

  it('cada estado de salud produce una etiqueta accesible distinta (el color no basta)', () => {
    const etiquetas = ([true, false, null] as const).map((vivo) => {
      const { container, unmount } = render(TarjetaCanal, { canal: { ...base, vivo }, alAbrir: () => {} })
      const etiqueta = container.querySelector('.punto')?.getAttribute('aria-label')
      unmount()
      return etiqueta
    })
    expect(new Set(etiquetas).size).toBe(3)
  })

  it('muestra la latencia en ms cuando hay dato', () => {
    render(TarjetaCanal, { canal: { ...base, latenciaMs: 120 }, alAbrir: () => {} })
    expect(screen.getByText('120 ms')).toBeTruthy()
  })

  it('omite el badge de ms cuando no hay latencia', () => {
    render(TarjetaCanal, { canal: { ...base, latenciaMs: 0 }, alAbrir: () => {} })
    expect(screen.queryByText('0 ms')).toBeNull()
  })

  it('muestra la resolución parseada del nombre cuando existe', () => {
    const { container } = render(TarjetaCanal, { canal: { ...base, nombre: 'BBC One (1080p)' }, alAbrir: () => {} })
    expect(container.querySelector('.insignia-resolucion')?.textContent).toBe('1080p')
  })

  it('omite el badge de resolución cuando el nombre no la trae', () => {
    const { container } = render(TarjetaCanal, { canal: base, alAbrir: () => {} })
    expect(container.querySelector('.insignia-resolucion')).toBeNull()
  })

  it('retiró BarrasSenal: ya no hay barras de señal en la tarjeta', () => {
    const { container } = render(TarjetaCanal, { canal: base, alAbrir: () => {} })
    expect(container.querySelector('.senal')).toBeNull()
  })

  // P0.5 (roving tabindex): no regresión. Ver TarjetaCanal.svelte.
  it('conserva data-indice y gatilla tabindex de ambos controles con focoActivo', () => {
    const { container, rerender } = render(TarjetaCanal, { canal: base, alAbrir: () => {}, indice: 3, focoActivo: true })
    const articulo = container.querySelector('article.tarjeta')
    expect(articulo?.getAttribute('data-indice')).toBe('3')
    expect(container.querySelector('.abrir')?.getAttribute('tabindex')).toBe('0')
    expect(container.querySelector('.favorito')?.getAttribute('tabindex')).toBe('0')

    rerender({ canal: base, alAbrir: () => {}, indice: 3, focoActivo: false })
    expect(container.querySelector('.abrir')?.getAttribute('tabindex')).toBe('-1')
    expect(container.querySelector('.favorito')?.getAttribute('tabindex')).toBe('-1')
  })

  it('no marca APP cuando el canal se ve en la web', () => {
    render(TarjetaCanal, { canal: base, alAbrir: () => {} })
    expect(screen.queryByText('APP')).toBeNull()
  })

  it('marca APP solo con un no explícito', () => {
    render(TarjetaCanal, { canal: { ...base, webOk: false }, alAbrir: () => {} })
    expect(screen.getByText('APP')).toBeTruthy()
  })

  // null NO es false: antes de la primera pasada del health-worker todo el
  // catálogo es null y marcarlo entero sería mentir.
  it('no marca APP cuando no se ha comprobado', () => {
    render(TarjetaCanal, { canal: { ...base, webOk: null }, alAbrir: () => {} })
    expect(screen.queryByText('APP')).toBeNull()
  })

  it('cae al fallback de iniciales cuando la imagen falla', async () => {
    const canal = { id: 'x', nombre: 'BBC One', logoUrl: 'http://muerto/logo.png', categoriaId: '', idioma: '', pais: 'GB', vivo: true, latenciaMs: 100, webOk: true } as const
    const { container } = render(TarjetaCanal, { canal, alAbrir: () => {} })
    const img = container.querySelector('img')
    if (!img) throw new Error('no hay img')
    await fireEvent.error(img)
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('.sinlogo')?.textContent).toContain('BB')
  })

  it('resetea logoRoto al cambiar de canal (reutilización en virtualización)', async () => {
    // Misma instancia, prop canal cambia → $effect dispara y logoRoto se resetea a false
    const canalConLogoRoto = { id: 'x', nombre: 'BBC One', logoUrl: 'http://muerto/logo.png', categoriaId: '', idioma: '', pais: 'GB', vivo: true, latenciaMs: 100, webOk: true } as const
    const canalBueno = { id: 'y', nombre: 'Sky News', logoUrl: 'http://bueno/logo.png', categoriaId: '', idioma: '', pais: 'GB', vivo: true, latenciaMs: 100, webOk: true } as const

    const { container, rerender } = render(TarjetaCanal, { canal: canalConLogoRoto, alAbrir: () => {} })

    // Simular error en la imagen: logoRoto → true
    const img = container.querySelector('img')
    if (!img) throw new Error('no hay img')
    await fireEvent.error(img)
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('.sinlogo')?.textContent).toContain('BB')

    // Cambiar prop canal en la MISMA instancia (virtualización: reutilización de tarjeta)
    // El $effect observa canal.id, ve el cambio, y resetea logoRoto = false
    rerender({ canal: canalBueno, alAbrir: () => {} })
    await new Promise(resolve => setTimeout(resolve, 0))

    // La img debe reaparecer porque logoRoto se reseteó
    expect(container.querySelector('img')).not.toBeNull()
    expect(container.querySelector('img')?.getAttribute('src')).toBe('http://bueno/logo.png')
    expect(container.querySelector('.sinlogo')).toBeNull()
  })
})
