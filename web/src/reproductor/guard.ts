export interface OpcionesGuard {
  alFallar: (mensaje: string) => void
  /** Se llama UNA vez, con prueba real de reproducción. Es la señal buena
   *  para quitar el indicador de carga. */
  alConfirmar?: () => void
  timeoutCarga?: number
  /** Techo ABSOLUTO de la carga: ni con progreso continuo se pasa de aquí. */
  timeoutCargaTotal?: number
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
 * - Error FATAL antes de la primera reproducción → fatal inmediato. Un error
 *   no fatal no mata nada: hls.js se recupera solo de la mayoría, y si no se
 *   recupera ya lo caza el timeout de carga.
 * - Nada PROGRESA dentro de timeoutCarga → fatal. "Progresar" incluye la
 *   tubería de carga (segmentos que llegan, buffer que crece), no solo la
 *   reproducción: un canal lento sigue vivo, y matarlo a los 7 s fijos era
 *   declarar caído lo que solo estaba tardando.
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
  private readonly timeoutCargaTotal: number
  private readonly timeoutAtasco: number

  private tCarga?: ReturnType<typeof setTimeout>
  private tAtasco?: ReturnType<typeof setTimeout>
  private arrancado = false
  private destruido = false
  private ultimaPosicion = 0
  private armadoEn = 0
  private avanzoDesdeElError = false
  private posicionReferencia: number | null = null

  constructor(o: OpcionesGuard) {
    this.alFallar = o.alFallar
    this.alConfirmar = o.alConfirmar
    // Con failover cada mirror muerto cuesta este tiempo: 7s es holgado para
    // un manifiesto vivo y rápido para cruzar al siguiente intento.
    this.timeoutCarga = o.timeoutCarga ?? 7_000
    // Techo absoluto. El presupuesto de 7 s se mide SIN progreso, así que un
    // canal caído sigue muriendo en 7 s; este techo solo acota al que descarga
    // sin parar y nunca llega a reproducir, para que no cuelgue la UI.
    this.timeoutCargaTotal = o.timeoutCargaTotal ?? 20_000
    this.timeoutAtasco = o.timeoutAtasco ?? 8_000
  }

  /** Armar ANTES de asignar la fuente: si la carga se cuelga, el timeout tiene
   *  que saltar igualmente. El watchdog original se armaba después y por eso
   *  nunca saltaba. */
  armarTimeoutDeCarga(): void {
    this.armadoEn = Date.now()
    this.programarCarga(this.timeoutCarga)
  }

  private programarCarga(ms: number): void {
    if (this.tCarga) clearTimeout(this.tCarga)
    this.tCarga = setTimeout(() => {
      if (this.destruido || this.arrancado) return
      this.alFallar('timeout-de-carga')
    }, ms)
  }

  /**
   * Señal de que la tubería avanza aunque todavía no se vea nada: un segmento
   * que llega, buffer que se anexa, metadatos que se leen. Aplaza el timeout
   * de carga hasta el techo absoluto.
   *
   * Existe porque la ÚNICA prueba de vida que tenía el guard era `timeupdate`,
   * que no llega hasta que el elemento reproduce de verdad. Medido en Chrome
   * sobre 30 canales del catálogo: arranque real p50 2,2 s / p90 6,6 s, o sea
   * que el corte fijo de 7 s caía justo sobre la cola sana.
   */
  alProgreso(): void {
    if (this.destruido || this.arrancado) return
    const restante = this.timeoutCargaTotal - (Date.now() - this.armadoEn)
    if (restante <= 0) return
    this.programarCarga(Math.min(this.timeoutCarga, restante))
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

  alError(mensaje: string, esFatal = true): void {
    if (this.destruido || mensaje === '') return

    if (!this.arrancado) {
      // Un no-fatal antes de arrancar NO mata: hls.js reintenta y se recupera
      // de la mayoría (fragLoadError, bufferStalledError, aborted…). Si de
      // verdad no se recupera, no habrá progreso y el timeout de carga lo caza.
      if (!esFatal) return
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

  /** abortar cancela la vigilancia sin declarar fatal: lo usa el failover al
   *  saltar al siguiente mirror por decisión propia, no por fallo del guard. */
  abortar(): void {
    this.destruido = true
    if (this.tCarga) clearTimeout(this.tCarga)
    if (this.tAtasco) clearTimeout(this.tAtasco)
  }
}
