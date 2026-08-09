import AVFoundation
import Foundation

/// Posee el AVPlayer que emite a AirPlay. Existe solo mientras hay sesión: la
/// reproducción local sigue siendo de media_kit, y los dos nunca están abiertos
/// a la vez.
final class AirPlaySession: NSObject {
  private var player: AVPlayer?
  private var observaciones: [NSKeyValueObservation] = []
  private let emitir: ([String: Any]) -> Void

  init(emitir: @escaping ([String: Any]) -> Void) {
    self.emitir = emitir
  }

  /// Diagnóstico de la emisión. Se deja encendido a propósito mientras la capa
  /// nativa no tenga cobertura de CI: es la única ventana que hay sobre ella.
  private func log(_ msg: String) {
    NSLog("[airplay] %@", msg)
  }

  func start(url: String, title: String) {
    log("start url=\(url)")
    stop()

    guard let u = URL(string: url) else {
      emitir(["type": "status", "state": "failed",
              "error": "URL inválida", "formatError": false])
      return
    }

    let item = AVPlayerItem(url: u)
    let p = AVPlayer(playerItem: item)
    // allowsExternalPlayback es toda la API en macOS. El acompañante
    // usesExternalPlaybackWhileExternalScreenIsActive existe solo en iOS: allí
    // distingue emitir de espejar una pantalla conectada, distinción que macOS
    // no tiene.
    p.allowsExternalPlayback = true
    player = p

    emitir(["type": "status", "state": "loading", "formatError": false])

    observaciones.append(item.observe(\.status, options: [.new]) { [weak self] it, _ in
      guard let self = self else { return }
      switch it.status {
      case .readyToPlay:
        self.log("item readyToPlay → play(); allowsExternalPlayback=\(p.allowsExternalPlayback) externalPlaybackActive=\(p.isExternalPlaybackActive)")
        p.play()
        // El desvío al receptor no es inmediato. Esta comprobación diferida es
        // la que dice si macOS llegó a descargar el vídeo en el televisor o si
        // se quedó reproduciendo en local con el audio enrutado.
        DispatchQueue.main.asyncAfter(deadline: .now() + 3) { [weak self] in
          self?.log("a los 3s: externalPlaybackActive=\(p.isExternalPlaybackActive) timeControlStatus=\(p.timeControlStatus.rawValue)")
        }
      case .failed:
        let err = it.error
        self.log("item FAILED: \(err?.localizedDescription ?? "sin descripción")")
        self.emitir([
          "type": "status", "state": "failed",
          "error": err?.localizedDescription ?? "Fallo de reproducción",
          "formatError": Self.esErrorDeFormato(err),
        ])
      default:
        break
      }
    })

    // timeControlStatus == .playing es la prueba de reproducción real. El
    // equivalente de por qué PlaybackGuard no se fía de `playing` en mpv.
    observaciones.append(p.observe(\.timeControlStatus, options: [.new]) { [weak self] pl, _ in
      guard let self = self, pl.timeControlStatus == .playing else { return }
      self.emitir(["type": "status", "state": "playing", "formatError": false])
    })

    // Aquí NO se observa isExternalPlaybackActive. El estado de la ruta lo
    // publica RouteName vía CoreAudio, y tener dos fuentes que pueden
    // contradecirse es peor que tener una: este observador emite `active:
    // false` en el hueco entre crear el AVPlayer y que la reproducción se
    // desvíe al receptor, lo que tumbaría la sesión a idle justo al empezar.
  }

  func stop() {
    player?.pause()
    player?.replaceCurrentItem(with: nil)
    player = nil
    observaciones.forEach { $0.invalidate() }
    observaciones.removeAll()
  }

  /// Distingue "este stream no lo puedo decodificar" de "no llegué al servidor".
  /// Es la diferencia entre recordar el canal como incompatible para siempre y
  /// no escribir nada: un Apple TV dormido y un stream MPEG-2 afloran los dos
  /// como .failed.
  private static func esErrorDeFormato(_ error: Error?) -> Bool {
    guard let e = error as NSError?, e.domain == AVFoundationErrorDomain else {
      return false
    }
    switch e.code {
    case AVError.Code.decodeFailed.rawValue,
         AVError.Code.fileFormatNotRecognized.rawValue,
         AVError.Code.failedToLoadMediaData.rawValue,
         AVError.Code.decoderNotFound.rawValue:
      return true
    default:
      return false
    }
  }
}
