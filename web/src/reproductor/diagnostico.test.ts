import { describe, expect, it } from 'vitest'
import { clasificarFallo, claseConsensuada } from './diagnostico'

describe('clasificarFallo', () => {
  // 403: geo-bloqueo o token caducado — ambiguo, el mensaje de 'geo' lo
  // reconoce sin afirmar de más (ver plan de la Tarea 21).
  it('403 → geo', () => {
    expect(clasificarFallo({ httpStatus: 403 })).toBe('geo')
  })

  // 404/410: el mirror ya no existe (dirección caducada).
  it('404 → caducado', () => {
    expect(clasificarFallo({ httpStatus: 404 })).toBe('caducado')
  })

  it('410 → caducado', () => {
    expect(clasificarFallo({ httpStatus: 410 })).toBe('caducado')
  })

  // Error de red de hls.js sin un status HTTP útil: el canal está caído.
  it('tipoHls de red sin status útil → caido', () => {
    expect(clasificarFallo({ tipoHls: 'networkError', detallesHls: 'manifestLoadError' })).toBe('caido')
  })

  // Nativo: MEDIA_ERR_NETWORK (2).
  it('mediaErrorCode 2 (MEDIA_ERR_NETWORK) → caido', () => {
    expect(clasificarFallo({ mediaErrorCode: 2 })).toBe('caido')
  })

  // hls.js: error de media/decodificación.
  it('tipoHls de media → formato', () => {
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'bufferAppendError' })).toBe('formato')
  })

  it('detallesHls con DECODE → formato', () => {
    expect(clasificarFallo({ tipoHls: 'otherError', detallesHls: 'fragParsingErrorDECODE' })).toBe('formato')
  })

  // Nativo: MEDIA_ERR_DECODE (3) y MEDIA_ERR_SRC_NOT_SUPPORTED (4).
  it('mediaErrorCode 3 (MEDIA_ERR_DECODE) → formato', () => {
    expect(clasificarFallo({ mediaErrorCode: 3 })).toBe('formato')
  })

  it('mediaErrorCode 4 (MEDIA_ERR_SRC_NOT_SUPPORTED) → formato', () => {
    expect(clasificarFallo({ mediaErrorCode: 4 })).toBe('formato')
  })

  // Sin ninguna señal útil: no se puede afirmar nada específico.
  it('vacío → desconocido', () => {
    expect(clasificarFallo({})).toBe('desconocido')
  })

  // httpStatus 403 manda sobre cualquier otra señal (prioridad de reglas).
  it('403 manda incluso con tipoHls de red presente', () => {
    expect(clasificarFallo({ httpStatus: 403, tipoHls: 'networkError' })).toBe('geo')
  })
})

// Bug real reportado por el dueño (2026-09-04) con AMC (720p): el reproductor
// mostraba «La dirección del canal caducó» porque enseñaba la clase del ÚLTIMO
// intento, y el último mirror daba 404. El PRIMERO —el mejor, el que la salud
// puso delante— estaba vivo pero servía segmentos de 4 s en más de 12 s. Con dos
// causas distintas, afirmar cualquiera de las dos es mentir.
describe('claseConsensuada', () => {
  it('si todos los intentos fallaron por lo mismo, esa es la causa', () => {
    expect(claseConsensuada(['geo', 'geo'])).toBe('geo')
    expect(claseConsensuada(['caducado'])).toBe('caducado')
  })

  it('el caso AMC: causas distintas -> no se afirma ninguna', () => {
    expect(claseConsensuada(['desconocido', 'caducado'])).toBe('desconocido')
    expect(claseConsensuada(['caducado', 'desconocido'])).toBe('desconocido')
  })

  it('tampoco se elige la más específica cuando hay desacuerdo', () => {
    expect(claseConsensuada(['geo', 'caducado'])).toBe('desconocido')
    expect(claseConsensuada(['formato', 'caido'])).toBe('desconocido')
  })

  it('sin intentos, desconocido', () => {
    expect(claseConsensuada([])).toBe('desconocido')
  })
})
