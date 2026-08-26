import { describe, expect, it } from 'vitest'
import { pareceGeoBloqueado } from './geo'
import type { Canal } from '../datos/catalogo'

const c = (nombre: string): Canal => ({
  id: 'x', nombre, logoUrl: '', categoriaId: '', idioma: '', pais: 'GB',
  vivo: true, latenciaMs: 100, webOk: true,
})

describe('pareceGeoBloqueado', () => {
  it('detecta marcas habituales en el nombre', () => {
    expect(pareceGeoBloqueado(c('BBC One [Geo-blocked]'))).toBe(true)
    expect(pareceGeoBloqueado(c('Canal (Geo)'))).toBe(true)
    expect(pareceGeoBloqueado(c('ITV UK only'))).toBe(true)
  })
  it('no marca canales normales', () => {
    expect(pareceGeoBloqueado(c('BBC One (1080p)'))).toBe(false)
  })
})
