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
    // Renderizar con un canal que tiene logo roto
    const canalRoto = { id: 'x', nombre: 'BBC One', logoUrl: 'http://muerto/logo.png', categoriaId: '', idioma: '', pais: 'GB', vivo: true, latenciaMs: 100, webOk: true } as const
    const canalBueno = { id: 'y', nombre: 'Sky News', logoUrl: 'http://bueno/logo.png', categoriaId: '', idioma: '', pais: 'GB', vivo: true, latenciaMs: 100, webOk: true } as const

    // Renderizar instancia 1: canalRoto con error
    const instance1 = render(TarjetaCanal, { canal: canalRoto, alAbrir: () => {} })
    let img = instance1.container.querySelector('img')
    if (!img) throw new Error('no hay img en canalRoto')
    await fireEvent.error(img)
    expect(instance1.container.querySelector('img')).toBeNull()
    expect(instance1.container.querySelector('.sinlogo')?.textContent).toContain('BB')

    // Renderizar instancia 2: canalBueno (simula reutilización en virtualización con $effect)
    const instance2 = render(TarjetaCanal, { canal: canalBueno, alAbrir: () => {} })
    await new Promise(resolve => setTimeout(resolve, 0)) // Dejar que el efecto se ejecute

    // La img debe estar visible porque logoRoto arranca en false cuando el componente se crea
    const img2 = instance2.container.querySelector('img')
    expect(img2).not.toBeNull()
    expect(img2?.getAttribute('src')).toBe('http://bueno/logo.png')
  })
})
