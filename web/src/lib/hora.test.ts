import { describe, expect, it } from 'vitest'
import { formatearHoraLocal } from './hora'

// El valor esperado se calcula A MANO con Date (no reutilizando la propia
// función bajo prueba) para que el test sea falsable de verdad, y sin fijar
// un huso horario concreto (corre igual en CI que en la máquina de
// cualquiera): sea cual sea el TZ del entorno, getHours()/getMinutes() sobre
// el MISMO epoch tienen que coincidir con lo que devuelve formatearHoraLocal.
function horaEsperadaAMano(epochSegundos: number): string {
  const d = new Date(epochSegundos * 1000)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}

describe('formatearHoraLocal', () => {
  it('formatea un epoch en segundos a HH:MM de 24h, en hora local', () => {
    const epoch = 1_700_003_400 // instante arbitrario, no redondo
    expect(formatearHoraLocal(epoch)).toBe(horaEsperadaAMano(epoch))
  })

  it('rellena con cero a la izquierda horas y minutos de un dígito', () => {
    // Medianoche exacta en UTC: en cualquier TZ da una hora de un solo
    // dígito salvo que el TZ local caiga justo en una hora de dos dígitos —
    // por eso se compara contra el mismo cálculo a mano, no contra un
    // literal '00:00'.
    const epoch = 1_700_000_000
    expect(formatearHoraLocal(epoch)).toBe(horaEsperadaAMano(epoch))
    expect(formatearHoraLocal(epoch)).toMatch(/^\d{2}:\d{2}$/)
  })

  it('nunca usa formato 12h (sin AM/PM)', () => {
    expect(formatearHoraLocal(1_700_000_000)).not.toMatch(/[ap]\.?\s*m\.?/i)
  })
})
