import { describe, expect, it } from 'vitest'
import { ICONO_DEFECTO, iconoDeCategoria } from './categorias'

describe('iconoDeCategoria', () => {
  it('devuelve el icono de una categoría conocida', () => {
    expect(iconoDeCategoria('News')).toBe('📰')
    expect(iconoDeCategoria('Sports')).toBe('⚽')
    expect(iconoDeCategoria('Movies')).toBe('🎬')
  })

  it('es case-insensitive', () => {
    expect(iconoDeCategoria('news')).toBe('📰')
    expect(iconoDeCategoria('MUSIC')).toBe('🎵')
  })

  it('para categorías compuestas gana el primer segmento conocido', () => {
    expect(iconoDeCategoria('News;General')).toBe('📰')
    expect(iconoDeCategoria('Movies,Series')).toBe('🎬')
    // primer segmento desconocido, segundo conocido
    expect(iconoDeCategoria('Foo;Sports')).toBe('⚽')
  })

  it('cae al icono por defecto para "Undefined", desconocidas o vacías', () => {
    expect(iconoDeCategoria('Undefined')).toBe(ICONO_DEFECTO)
    expect(iconoDeCategoria('AlgoQueNoExiste')).toBe(ICONO_DEFECTO)
    expect(iconoDeCategoria('')).toBe(ICONO_DEFECTO)
  })

  it('nunca devuelve cadena vacía (toda fila lleva icono)', () => {
    for (const c of ['News', 'X', '', 'a;b;c', 'General']) {
      expect(iconoDeCategoria(c).length).toBeGreaterThan(0)
    }
  })
})
