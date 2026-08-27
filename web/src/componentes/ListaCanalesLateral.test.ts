import { describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { tick } from 'svelte'
import ListaCanalesLateral from './ListaCanalesLateral.svelte'
import type { Canal } from '../datos/catalogo'
import { t } from '../i18n'

// jsdom no implementa IntersectionObserver — mismo doble que
// App.integracion.test.ts, reducido a lo que este fichero usa.
class FalsoIntersectionObserver {
  static instancias: FalsoIntersectionObserver[] = []
  private cb: IntersectionObserverCallback
  constructor(cb: IntersectionObserverCallback) {
    this.cb = cb
    FalsoIntersectionObserver.instancias.push(this)
  }
  observe() {}
  disconnect() {}
  unobserve() {}
  takeRecords() {
    return []
  }
  dispararInterseccion() {
    this.cb([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalDePrueba(id: string, extra: Partial<Canal> = {}): Canal {
  return {
    id,
    nombre: `Canal ${id}`,
    logoUrl: '',
    categoriaId: '',
    idioma: 'es',
    pais: '',
    vivo: true,
    latenciaMs: 120,
    webOk: true,
    ...extra,
  }
}
const muchos = Array.from({ length: 1000 }, (_, i) => canalDePrueba(String(i)))

describe('ListaCanalesLateral — virtualización con scroll interno', () => {
  it('con 1000 canales solo monta una ventana acotada de filas, nunca las 1000', () => {
    render(ListaCanalesLateral, { canales: muchos, alAbrir: vi.fn(), alPedirMas: vi.fn() })
    const filas = document.querySelectorAll('.lista-lateral article')
    expect(filas.length).toBeGreaterThan(0)
    expect(filas.length).toBeLessThan(100)
  })

  it('la fila muestra la salud (SenalCanal) y la resolución sacada del nombre', () => {
    render(ListaCanalesLateral, {
      canales: [canalDePrueba('a', { nombre: 'Tele Uno 1080p', latenciaMs: 88 })],
      alAbrir: vi.fn(),
      alPedirMas: vi.fn(),
    })
    expect(screen.getByText('88 ms')).toBeTruthy()
    expect(screen.getByText('1080p')).toBeTruthy()
  })

  it('el canal en curso lleva aria-current y la insignia «Reproduciendo»', () => {
    render(ListaCanalesLateral, {
      canales: [canalDePrueba('a'), canalDePrueba('b')],
      canalActualId: 'b',
      alAbrir: vi.fn(),
      alPedirMas: vi.fn(),
    })
    const activo = screen.getByRole('button', { name: /Canal b/ })
    expect(activo.getAttribute('aria-current')).toBe('true')
    expect(screen.getByText(t('escenario.reproduciendo'))).toBeTruthy()
  })

  it('clicar una fila llama a alAbrir con el canal', async () => {
    const alAbrir = vi.fn()
    render(ListaCanalesLateral, { canales: [canalDePrueba('a')], alAbrir, alPedirMas: vi.fn() })
    await fireEvent.click(screen.getByRole('button', { name: /Canal a/ }))
    expect(alAbrir).toHaveBeenCalledWith(expect.objectContaining({ id: 'a' }))
  })

  it('roving tabindex: solo una fila con tabindex=0; las flechas mueven el índice activo', async () => {
    render(ListaCanalesLateral, { canales: muchos.slice(0, 5), alAbrir: vi.fn(), alPedirMas: vi.fn() })
    const botones = () => Array.from(document.querySelectorAll<HTMLButtonElement>('.lista-lateral button.abrir'))
    expect(botones().filter((b) => b.tabIndex === 0)).toHaveLength(1)
    const primero = botones()[0]
    primero.focus()
    await fireEvent.keyDown(primero, { key: 'ArrowDown' })
    await tick()
    expect(botones().filter((b) => b.tabIndex === 0)).toHaveLength(1)
    expect(botones()[1].tabIndex).toBe(0)
  })

  it('el centinela al final dispara alPedirMas al intersectar', async () => {
    const alPedirMas = vi.fn()
    render(ListaCanalesLateral, { canales: muchos.slice(0, 5), alAbrir: vi.fn(), alPedirMas })
    await tick()
    FalsoIntersectionObserver.instancias.at(-1)?.dispararInterseccion()
    expect(alPedirMas).toHaveBeenCalled()
  })
})
