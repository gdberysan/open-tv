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
})
