import { describe, expect, it } from 'vitest'
import { parsearResolucion } from './resolucion'

describe('parsearResolucion', () => {
  it('extrae NNNp entre paréntesis', () => {
    expect(parsearResolucion('BBC One (1080p)')).toBe('1080p')
  })

  it('extrae NNNp sin paréntesis', () => {
    expect(parsearResolucion('Canal 24 720p')).toBe('720p')
  })

  it('extrae etiquetas de calidad (4K, FHD, HD...)', () => {
    expect(parsearResolucion('Canal 4K')).toBe('4K')
    expect(parsearResolucion('Canal FHD')).toBe('FHD')
  })

  it('devuelve null cuando el nombre no trae resolución', () => {
    expect(parsearResolucion('BBC One')).toBeNull()
  })
})
