import { describe, expect, it } from 'vitest'
import { coincideDifuso } from './fuzzy'

describe('coincideDifuso', () => {
  it('casa "mex" con "México" y devuelve un puntaje numérico', () => {
    const puntaje = coincideDifuso('mex', 'México')
    expect(puntaje).not.toBeNull()
    expect(typeof puntaje).toBe('number')
  })

  it('casa "méxico" con "México" (acento en el término, igual que en el texto)', () => {
    expect(coincideDifuso('méxico', 'México')).not.toBeNull()
  })

  it('"xyz" no casa con "México"', () => {
    expect(coincideDifuso('xyz', 'México')).toBeNull()
  })

  it('casa por subsecuencia no contigua: "cnn" en "CNN en Español"', () => {
    expect(coincideDifuso('cnn', 'CNN en Español')).not.toBeNull()
  })

  it('"mx" casa con "México" (subsecuencia, no substring)', () => {
    expect(coincideDifuso('mx', 'México')).not.toBeNull()
  })

  it('es insensible a mayúsculas: mismo puntaje en mayúsculas y minúsculas', () => {
    const minuscula = coincideDifuso('mex', 'méxico')
    const mayuscula = coincideDifuso('MEX', 'MÉXICO')
    expect(minuscula).not.toBeNull()
    expect(mayuscula).toBe(minuscula)
  })

  it('es insensible a acentos: el término sin acento casa con el texto con acento', () => {
    expect(coincideDifuso('espanol', 'CNN en Español')).not.toBeNull()
  })

  it('un match al inicio del texto puntúa más que uno disperso en medio', () => {
    // "can" es prefijo contiguo y al inicio en "Canal Once"; en "Vaticano
    // News" también aparece contiguo ("vati-CAN-o") pero ni al inicio ni
    // tras un límite de palabra, y el texto es más largo.
    const alInicio = coincideDifuso('can', 'Canal Once')
    const disperso = coincideDifuso('can', 'Vaticano News')
    expect(alInicio).not.toBeNull()
    expect(disperso).not.toBeNull()
    expect(alInicio as number).toBeGreaterThan(disperso as number)
  })

  it('un match tras un límite de palabra puntúa más que uno sin límite, a igual largo de texto', () => {
    // "once" empieza justo tras un espacio en "Canal Once"; en "Canalonce"
    // (mismo largo) el mismo match no tiene ningún límite de palabra detrás.
    const conLimite = coincideDifuso('once', 'Canal Once')
    const sinLimite = coincideDifuso('once', 'Canalonce9')
    expect(conLimite).not.toBeNull()
    expect(sinLimite).not.toBeNull()
    expect(conLimite as number).toBeGreaterThan(sinLimite as number)
  })

  it('término vacío casa con todo con puntaje neutro (0)', () => {
    expect(coincideDifuso('', 'México')).toBe(0)
    expect(coincideDifuso('   ', 'cualquier cosa')).toBe(0)
  })
})
