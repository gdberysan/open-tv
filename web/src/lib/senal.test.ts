import { describe, expect, it } from 'vitest'
import { nivelSenal } from './senal'

// Mismos umbrales que la app de macOS (mobile/lib/domain/models/channel.dart):
// verde <200 ms, naranja 200–800, rojo >800, gris muerto, apagado sin datos.
// Divergir sería que el mismo canal se vea "bien" en un sitio y "regular" en
// el otro.
describe('nivelSenal', () => {
  it.each([
    [null, 0, 'desconocido'],
    [false, 0, 'muerta'],
    [true, 120, 'buena'],
    [true, 199, 'buena'],
    [true, 200, 'media'],
    [true, 800, 'media'],
    [true, 801, 'pobre'],
  ] as const)('vivo=%s latencia=%s → %s', (vivo, latencia, quiero) => {
    expect(nivelSenal(vivo, latencia)).toBe(quiero)
  })
})
