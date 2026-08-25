import { describe, expect, it } from 'vitest'
import { planDeFailover } from './failover'
import { urlProxy } from './plan'
import type { Mirror } from '../datos/catalogo'

const m = (url: string, webOk: boolean | null): Mirror => ({ url, vivo: true, latenciaMs: 100, webOk })

describe('planDeFailover', () => {
  // Con hls.js: un mirror webOk=false va solo por proxy; uno webOk=true directo
  // y luego proxy. La secuencia recorre los mirrors en orden, y dentro de cada
  // uno su política.
  it('encadena los intentos de todos los mirrors en orden', () => {
    const mirrors = [m('https://a/x.m3u8', true), m('https://b/x.m3u8', false)]
    const plan = planDeFailover(mirrors, 'hlsjs', true)
    expect(plan).toEqual([
      { url: 'https://a/x.m3u8', viaProxy: false, mirrorIndex: 0 },
      { url: urlProxy('https://a/x.m3u8'), viaProxy: true, mirrorIndex: 0 },
      { url: urlProxy('https://b/x.m3u8'), viaProxy: true, mirrorIndex: 1 },
    ])
  })

  // En motor nativo (Safari) cada mirror intenta directo primero.
  it('en nativo cada mirror intenta directo aunque webOk sea falso', () => {
    const plan = planDeFailover([m('https://a/x.m3u8', false)], 'nativo', false)
    expect(plan).toEqual([{ url: 'https://a/x.m3u8', viaProxy: false, mirrorIndex: 0 }])
  })

  // Un mirror sin salida útil (aviso solo-app-o-safari) no aporta intentos,
  // pero NO corta la cadena: el siguiente mirror sí puede tener.
  it('un mirror sin intentos no corta la cadena', () => {
    const mirrors = [m('https://a/x.m3u8', false), m('https://b/x.m3u8', true)]
    const plan = planDeFailover(mirrors, 'hlsjs', false) // sin proxy
    // a: webOk=false + sin proxy → aviso, 0 intentos. b: webOk=true → directo.
    expect(plan).toEqual([{ url: 'https://b/x.m3u8', viaProxy: false, mirrorIndex: 1 }])
  })

  it('sin mirrors, sin intentos', () => {
    expect(planDeFailover([], 'hlsjs', true)).toEqual([])
  })
})
