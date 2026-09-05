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

  // 'formato' SOLO para señales de códec de verdad. Antes bastaba con que el
  // tipo fuera mediaError, y en hls.js TODO atasco es mediaError
  // (bufferStalledError, fragParsingError, bufferAppendError,
  // bufferSeekOverHole, bufferNudgeOnStall) — así que un origen que servía
  // relleno acababa diciéndole al usuario que su navegador no puede con el
  // formato, y mandándolo a Safari a repetir el mismo fallo.
  it('códecs incompatibles del manifiesto → formato', () => {
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'manifestIncompatibleCodecsError' })).toBe('formato')
  })

  it('el buffer no acepta el códec → formato', () => {
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'bufferAddCodecError' })).toBe('formato')
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'bufferIncompatibleCodecsError' })).toBe('formato')
  })

  // El caso AXN Latin America South (2026-09-04): el origen devuelve 188 bytes
  // —un paquete TS nulo— con HTTP 200 para los segmentos ya caducados. hls.js
  // no puede demuxar eso y emite fragParsingError, que es mediaError. El canal
  // EMITE; lo que llega está roto. Decir «tu navegador no puede con el formato»
  // era falso y el consejo («prueba en Safari») no arregla nada: Safari
  // recibiría exactamente los mismos 188 bytes.
  it('un atasco NO es un problema de formato', () => {
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'bufferStalledError' })).toBe('inestable')
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'fragParsingError' })).toBe('inestable')
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'bufferAppendError' })).toBe('inestable')
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'bufferSeekOverHole' })).toBe('inestable')
    expect(clasificarFallo({ tipoHls: 'mediaError', detallesHls: 'bufferNudgeOnStall' })).toBe('inestable')
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
