import { describe, expect, it } from 'vitest'
import { es } from './es'
import { en } from './en'
import { t, idioma } from './index'

describe('diccionario', () => {
  // El tipo Record<ClaveMensaje, string> ya impide que falte una clave en
  // inglés: esto atrapa el caso contrario, una clave de MÁS en inglés que
  // nadie usa y que se queda ahí para siempre.
  it('tiene exactamente las mismas claves en los dos idiomas', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(es).sort())
  })

  it('no deja ningún texto vacío', () => {
    for (const [clave, valor] of Object.entries({ ...es, ...en })) {
      expect(valor, `la clave ${clave} está vacía`).not.toBe('')
    }
  })

  it('interpola parámetros', () => {
    idioma.actual = 'es'
    expect(t('catalogo.total', { n: '12639' })).toContain('12639')
  })

  it('cambia de idioma', () => {
    idioma.actual = 'en'
    const ingles = t('accion.aleatorio')
    idioma.actual = 'es'
    expect(t('accion.aleatorio')).not.toBe(ingles)
  })
})
