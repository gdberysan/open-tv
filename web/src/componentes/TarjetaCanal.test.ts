import { describe, expect, it } from 'vitest'
import { fireEvent } from '@testing-library/svelte'
import { render, screen } from '@testing-library/svelte'
import TarjetaCanal from './TarjetaCanal.svelte'
import type { Canal } from '../datos/catalogo'

const base: Canal = {
  id: 'x', nombre: 'BBC One', logoUrl: '', categoriaId: 'General',
  idioma: 'en', pais: 'GB', vivo: true, latenciaMs: 120, webOk: true,
}

describe('TarjetaCanal', () => {
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
