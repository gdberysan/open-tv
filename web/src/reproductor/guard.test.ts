import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PlaybackGuard, type OpcionesGuard } from './guard'

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

  it('abortar() cancela sin declarar fatal y libera los timers', () => {
    vi.useFakeTimers()
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 6_000 })
    g.armarTimeoutDeCarga()
    g.abortar()
    vi.advanceTimersByTime(60_000)
    expect(fatal).not.toHaveBeenCalled()
    vi.useRealTimers()
  })
})

// ── Regresión: canales vivos que el guard mataba antes de tiempo ──
// Evidencia real (Chrome, 2026-09-04, muestra de 30 canales del catálogo):
// el arranque real tiene p50 2,2 s y p90 6,6 s, así que el presupuesto de 7 s
// caía JUSTO sobre la cola. Con la contención del arranque de la propia app
// (catálogo + cientos de logos cargando a la vez que el primer canal), dos
// canales de la muestra que reproducen perfectamente —111 TV y A Spor—
// tardaron 13,3 s y 7,9 s en dar la segunda posición y el guard los declaró
// "no llegó a reproducir". Medidos en solitario arrancaban en 5,0 s: no
// estaban caídos, estaban compitiendo por ancho de banda.
describe('PlaybackGuard: carga lenta pero viva', () => {
  it('un error NO fatal antes de arrancar no mata el intento', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal })
    g.armarTimeoutDeCarga()

    // hls.js emite estos constantemente en directos que se ven perfectamente;
    // se recupera solo. Matar aquí es matar un canal sano.
    g.alError('networkError:fragLoadError', false)

    expect(fatal).not.toHaveBeenCalled()
  })

  it('el progreso del pipeline aplaza el timeout de carga', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 7_000 })
    g.armarTimeoutDeCarga()

    // Segmentos llegando y buffer creciendo: está vivo, solo es lento.
    vi.advanceTimersByTime(6_000)
    g.alProgreso()
    vi.advanceTimersByTime(6_000)

    expect(fatal).not.toHaveBeenCalled()
  })

  it('sin progreso, el timeout de carga sigue disparando a los 7 s', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 7_000 })
    g.armarTimeoutDeCarga()

    vi.advanceTimersByTime(7_000)

    expect(fatal).toHaveBeenCalledOnce()
  })

  it('el progreso no aplaza la carga más allá del techo absoluto', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 7_000, timeoutCargaTotal: 20_000 })
    g.armarTimeoutDeCarga()

    // Un origen que descarga sin parar pero nunca llega a reproducir no puede
    // colgar la UI para siempre: el techo manda.
    for (let i = 0; i < 10; i++) {
      vi.advanceTimersByTime(5_000)
      g.alProgreso()
    }

    expect(fatal).toHaveBeenCalledOnce()
  })
})

// ── Regresión: pestaña oculta ──
// Chrome NO abre un MediaSource en una pestaña oculta: el <video> se queda en
// networkState 2 con un blob que nunca llega a 'sourceopen', hls.js sigue
// sondeando la playlist (o sea, el canal está VIVO) pero no pide un solo
// segmento y readyState no pasa de 0. Comprobado aislando las variables en
// Chrome real el 2026-09-04: con userActivation.hasBeenActive=true y
// visibilityState='hidden', MediaSource.readyState se queda en 'closed'.
// Gastar el presupuesto de carga ahí es cronometrar un tiempo que el navegador
// no deja usar, y el resultado era declarar caído un canal sano.
describe('PlaybackGuard: pestaña oculta', () => {
  it('con la pestaña oculta el presupuesto de carga no corre', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 7_000 })
    g.armarTimeoutDeCarga()

    g.pausar()
    vi.advanceTimersByTime(60_000)

    expect(fatal).not.toHaveBeenCalled()
  })

  it('al reanudar solo queda el presupuesto que faltaba', () => {
    const fatal = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, timeoutCarga: 7_000 })
    g.armarTimeoutDeCarga()

    vi.advanceTimersByTime(5_000) // quedan 2 s
    g.pausar()
    vi.advanceTimersByTime(60_000) // oculta: no cuenta
    g.reanudar()

    vi.advanceTimersByTime(1_999)
    expect(fatal).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(fatal).toHaveBeenCalledOnce()
  })

  it('pausar después de arrancar no derriba una reproducción en curso', () => {
    const fatal = vi.fn()
    const confirmado = vi.fn()
    const g = new PlaybackGuard({ alFallar: fatal, alConfirmar: confirmado })
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1.5)

    g.pausar()
    g.reanudar()
    vi.advanceTimersByTime(60_000)

    expect(confirmado).toHaveBeenCalledOnce()
    expect(fatal).not.toHaveBeenCalled()
  })
})

// ── Regresión: el watchdog de atasco era ciego al progreso ──
// Reportado por el dueño (2026-09-04) con AXN Latin America South: el canal
// arrancaba, se quedaba a tirones y acababa en «El canal dejó de emitir».
// Medido contra su origen (Astra): la ventana en vivo es cortísima y los
// segmentos rotan; al pedir uno ya caducado NO devuelve 404, devuelve HTTP 200
// con Content-Type video/MP2T y exactamente 188 bytes — UN paquete TS nulo. El
// cliente se descuelga del directo y hls.js intenta recuperarse saltando al
// borde, pero el watchdog lo mataba a los 8 s por mirar SOLO la posición.
// alProgreso() ya distinguía «lento pero vivo» de «muerto» al CARGAR; después
// de arrancar volvía a ser ciego. Esto lo hace simétrico.
describe('PlaybackGuard: atasco consciente del progreso', () => {
  const arrancado = (o: OpcionesGuard) => {
    const g = new PlaybackGuard(o)
    g.armarTimeoutDeCarga()
    g.alPosicion(0)
    g.alPosicion(1) // confirma reproducción
    return g
  }

  it('el progreso durante un atasco aplaza el corte', () => {
    const fatal = vi.fn()
    const g = arrancado({ alFallar: fatal, timeoutAtasco: 8_000 })

    g.alError('bufferStalledError')
    vi.advanceTimersByTime(6_000)
    g.alProgreso() // hls.js sigue trayendo segmentos: se está recuperando
    vi.advanceTimersByTime(6_000)

    expect(fatal).not.toHaveBeenCalled()
  })

  it('sin progreso, el corte sigue disparando a los 8 s', () => {
    const fatal = vi.fn()
    const g = arrancado({ alFallar: fatal, timeoutAtasco: 8_000 })

    g.alError('bufferStalledError')
    vi.advanceTimersByTime(8_000)

    expect(fatal).toHaveBeenCalledOnce()
  })

  it('el techo absoluto manda: progreso eterno no evita el corte para siempre', () => {
    const fatal = vi.fn()
    const g = arrancado({ alFallar: fatal, timeoutAtasco: 8_000, timeoutAtascoTotal: 30_000 })

    g.alError('bufferStalledError')
    for (let i = 0; i < 12; i++) {
      vi.advanceTimersByTime(5_000)
      g.alProgreso()
    }

    expect(fatal).toHaveBeenCalledOnce()
  })

  it('si la posición vuelve a avanzar, no hay corte: es la recuperación real', () => {
    const fatal = vi.fn()
    const g = arrancado({ alFallar: fatal, timeoutAtasco: 8_000 })

    g.alError('bufferStalledError')
    vi.advanceTimersByTime(4_000)
    g.alPosicion(2.5) // el vídeo volvió a correr
    vi.advanceTimersByTime(30_000)

    expect(fatal).not.toHaveBeenCalled()
  })
})
