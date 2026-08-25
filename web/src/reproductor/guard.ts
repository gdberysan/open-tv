export interface OpcionesGuard {
  alFallar: (mensaje: string) => void
  /** Se llama UNA vez, con prueba real de reproducción. Es la señal buena
   *  para quitar el indicador de carga. */
  alConfirmar?: () => void
  timeoutCarga?: number
  timeoutAtasco?: number
}

/**
 * Decide cuándo un error del reproductor es realmente fatal.
 *
 * Puerto 1:1 de mobile/lib/presentation/player/playback_guard.dart. El bug que
 * lo originó: cualquier error de mpv —incluido un EOF transitorio de HLS en
 * vivo— mataba el vídeo dejando el audio sonando. hls.js tiene la misma
 * naturaleza: emite bufferStalledError y fragParsingError constantemente en
 * directos que se ven perfectamente.
 *
 * Reglas:
 * - Error antes de la primera reproducción → fatal inmediato.
 * - Nada reproduce dentro de timeoutCarga → fatal.
 * - Error durante la reproducción → solo es fatal si la posición deja de
 *   avanzar durante timeoutAtasco.
 *
 * "Reproducir" significa que la POSICIÓN AVANZA, no que el elemento diga que
 * está reproduciendo.
 */
export class PlaybackGuard {
  private readonly alFallar: (m: string) => void
  private readonly alConfirmar?: () => void
  private readonly timeoutCarga: number
  private readonly timeoutAtasco: number

  private tCarga?: ReturnType<typeof setTimeout>
  private tAtasco?: ReturnType<typeof setTimeout>
  private arrancado = false
  private destruido = false
  private ultimaPosicion = 0
  private avanzoDesdeElError = false
  private posicionReferencia: number | null = null

  constructor(o: OpcionesGuard) {
    this.alFallar = o.alFallar
    this.alConfirmar = o.alConfirmar
    this.timeoutCarga = o.timeoutCarga ?? 15_000
    this.timeoutAtasco = o.timeoutAtasco ?? 8_000
  }

  /** Armar ANTES de asignar la fuente: si la carga se cuelga, el timeout tiene
   *  que saltar igualmente. El watchdog original se armaba después y por eso
   *  nunca saltaba. */
  armarTimeoutDeCarga(): void {
    if (this.tCarga) clearTimeout(this.tCarga)
    this.tCarga = setTimeout(() => {
      if (this.destruido || this.arrancado) return
      this.alFallar('timeout-de-carga')
    }, this.timeoutCarga)
  }

  alPosicion(segundos: number): void {
    if (segundos !== this.ultimaPosicion) {
      this.ultimaPosicion = segundos
      this.avanzoDesdeElError = true
    }
    if (this.arrancado || this.destruido) return

    // La primera muestra solo fija la referencia: una posición que se repite
    // es exactamente lo que se ve cuando el reproductor está atascado.
    if (this.posicionReferencia === null) {
      this.posicionReferencia = segundos
      return
    }
    if (segundos !== this.posicionReferencia) {
      this.arrancado = true
      if (this.tCarga) clearTimeout(this.tCarga)
      this.alConfirmar?.()
    }
  }

  alError(mensaje: string): void {
    if (this.destruido || mensaje === '') return

    if (!this.arrancado) {
      if (this.tCarga) clearTimeout(this.tCarga)
      this.alFallar(mensaje)
      return
    }

    // Ya en reproducción: vigilar en vez de matar. Si ya hay una vigilancia en
    // curso, no re-armar — errores repetidos no deben extender la ventana.
    if (this.tAtasco) return
    this.avanzoDesdeElError = false
    this.tAtasco = setTimeout(() => {
      this.tAtasco = undefined
      if (this.destruido) return
      if (!this.avanzoDesdeElError) this.alFallar(mensaje)
    }, this.timeoutAtasco)
  }

  destruir(): void {
    this.destruido = true
    if (this.tCarga) clearTimeout(this.tCarga)
    if (this.tAtasco) clearTimeout(this.tAtasco)
  }
}
