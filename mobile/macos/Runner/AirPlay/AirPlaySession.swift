import AVFoundation
import Foundation

/// Posee el AVPlayer que emite a AirPlay.
///
/// El reproductor se crea **al arrancar**, no al empezar a emitir, y vive toda
/// la sesión de la app. Es obligatorio: en macOS el selector de rutas enruta un
/// reproductor concreto vía `AVRoutePickerView.player`, así que el AVPlayer
/// tiene que existir ANTES de que se abra el popover. Crearlo perezosamente
/// dentro de start() dejaba al selector sin nada que enrutar — el televisor se
/// conectaba y no recibía vídeo — y además hacía inalcanzable el estado `armed`,
/// porque la única fuente de eventos de ruta era un observador sobre ese mismo
/// reproductor inexistente.
///
/// media_kit sigue siendo el único reproductor local: este solo tiene item
/// cargado mientras hay emisión.
final class AirPlaySession: NSObject {
  /// Vivo desde el arranque. Sin item no consume nada, pero existe para que el
  /// selector pueda apuntarle.
  let player = AVPlayer()

  private var observacionesItem: [NSKeyValueObservation] = []
  private var observacionRuta: NSKeyValueObservation?
  private let emitir: ([String: Any]) -> Void

  init(emitir: @escaping ([String: Any]) -> Void) {
    self.emitir = emitir
    super.init()

    player.allowsExternalPlayback = true

    // Única fuente de verdad del estado de la ruta. CoreAudio no sirve: al
    // elegir destino en el selector, macOS NO cambia la salida por defecto del
    // sistema ni registra el receptor como dispositivo de audio — comprobado
    // enumerando los dispositivos con el televisor ya conectado.
    observacionRuta = player.observe(\.isExternalPlaybackActive, options: [.new, .initial]) {
      [weak self] p, _ in
      NSLog("[airplay] ruta: externalPlaybackActive=%@", String(p.isExternalPlaybackActive))
      self?.emitir(["type": "route", "active": p.isExternalPlaybackActive])
    }
  }

  private func log(_ msg: String) {
    NSLog("[airplay] %@", msg)
  }

  func start(url: String, title: String) {
    log("start url=\(url)")
    limpiarItem()

    guard let u = URL(string: url) else {
      emitir(["type": "status", "state": "failed",
              "error": "URL inválida", "formatError": false])
      return
    }

    let item = AVPlayerItem(url: u)
    emitir(["type": "status", "state": "loading", "formatError": false])

    observacionesItem.append(item.observe(\.status, options: [.new]) { [weak self] it, _ in
      guard let self = self else { return }
      switch it.status {
      case .readyToPlay:
        self.log("item readyToPlay → play(); externalPlaybackActive=\(self.player.isExternalPlaybackActive)")
        self.player.play()
        // El desvío al receptor no es inmediato. Esta comprobación diferida
        // distingue "macOS descargó el vídeo en el televisor" de "sigue en
        // local con la ruta puesta".
        DispatchQueue.main.asyncAfter(deadline: .now() + 3) { [weak self] in
          guard let self = self else { return }
          self.log("a los 3s: externalPlaybackActive=\(self.player.isExternalPlaybackActive) timeControl=\(self.player.timeControlStatus.rawValue)")
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
    observacionesItem.append(
      player.observe(\.timeControlStatus, options: [.new]) { [weak self] pl, _ in
        guard let self = self, pl.timeControlStatus == .playing else { return }
        self.emitir(["type": "status", "state": "playing", "formatError": false])
      })

    player.replaceCurrentItem(with: item)
  }

  func stop() {
    log("stop")
    limpiarItem()
  }

  /// Descarga el item pero NO destruye el reproductor: si desapareciera, el
  /// selector se quedaría sin destino y la ruta se perdería.
  private func limpiarItem() {
    player.pause()
    player.replaceCurrentItem(with: nil)
    observacionesItem.forEach { $0.invalidate() }
    observacionesItem.removeAll()
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
