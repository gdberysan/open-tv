import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PlaybackGuard } from './guard'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('PlaybackGuard', () => {
  it('un error antes de la primera reproducción es fatal al instante', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal })
    g.armarTimeoutDeCarga()
    g.alError('manifiesto 403')
    expect(fatal).toHaveBeenCalledOnce()
  })

  it('si nada reproduce en el timeout de carga, es fatal', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()
    vi.advanceTimersByTime(14_999)
    expect(fatal).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(fatal).toHaveBeenCalledOnce()
  })

  // La prueba de reproducción es que la POSICIÓN AVANZA. `playing` es una
  // declaración de intención: se emite antes de decodificar un fotograma, y
  // desarmar el watchdog ahí dejaba la pantalla colgada para siempre en
  // cualquier canal cuyo manifiesto carga pero cuyos segmentos nunca llegan.
  it('la primera posición solo fija la referencia', () => {
    const fatal = vi.fn()
    const confirmado = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, alConfirmar: confirmado, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()

    g.alPosicion(0)
    g.alPosicion(0) // congelado: sigue siendo la referencia
    vi.advanceTimersByTime(15_000)

    expect(confirmado).not.toHaveBeenCalled()
    expect(fatal).toHaveBeenCalledOnce()
  })

  it('una posición que avanza confirma la reproducción y desarma la carga', () => {
    const fatal = vi.fn()
    const confirmado = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, alConfirmar: confirmado, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()

    g.alPosicion(0)
    g.alPosicion(1.2)
    vi.advanceTimersByTime(60_000)

    expect(confirmado).toHaveBeenCalledOnce()
    expect(fatal).not.toHaveBeenCalled()
  })

  // El caso del bug: error transitorio DURANTE la reproducción. Si el vídeo
  // sigue avanzando, no se toca nada.
  it('un error en reproducción se ignora si la posición sigue avanzando', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000, timeoutAtasco: 8_000 })
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1)

    g.alError('bufferStalledError')
    vi.advanceTimersByTime(4_000)
    g.alPosicion(5) // sigue vivo
    vi.advanceTimersByTime(4_000)

    expect(fatal).not.toHaveBeenCalled()
  })

  it('un error en reproducción es fatal si la posición se congela', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000, timeoutAtasco: 8_000 })
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1)

    g.alError('networkError')
    vi.advanceTimersByTime(8_000)

    expect(fatal).toHaveBeenCalledExactlyOnceWith('networkError')
  })

  // Errores repetidos no pueden extender la ventana de vigilancia una y otra
  // vez: eso convertiría el guard en un observador eterno.
  it('un segundo error no re-arma la vigilancia', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000, timeoutAtasco: 8_000 })
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1)

    g.alError('e1')
    vi.advanceTimersByTime(5_000)
    g.alError('e2')
    vi.advanceTimersByTime(3_000)

    expect(fatal).toHaveBeenCalledOnce()
  })

  it('destruir cancela todo', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 15_000 })
    g.armarTimeoutDeCarga()
    g.destruir()
    vi.advanceTimersByTime(60_000)
    expect(fatal).not.toHaveBeenCalled()
  })
})
