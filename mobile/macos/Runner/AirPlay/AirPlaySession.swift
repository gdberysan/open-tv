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

  func start(url: String, title: String) {
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
        p.play()
      case .failed:
        let err = it.error
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

    observaciones.append(p.observe(\.isExternalPlaybackActive, options: [.new]) { [weak self] pl, _ in
      guard let self = self else { return }
      var evento: [String: Any] = ["type": "route", "active": pl.isExternalPlaybackActive]
      if let nombre = RouteName.salidaPorDefecto() {
        evento["name"] = nombre
      }
      self.emitir(evento)
    })
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
