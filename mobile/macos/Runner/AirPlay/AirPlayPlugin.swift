import Cocoa
import FlutterMacOS

/// Solo cableado: registra la factoría de vistas y los dos canales de la
/// sesión. Cero lógica de negocio — esta capa no pasa nunca por CI (los dos
/// jobs corren en ubuntu-latest), así que todo lo que pueda decidirse en Dart
/// se decide en Dart.
final class AirPlayPlugin: NSObject, FlutterStreamHandler {
  private var sink: FlutterEventSink?
  private var session: AirPlaySession?
  private static var instancia: AirPlayPlugin?

  static func register(with registrar: FlutterPluginRegistrar) {
    let plugin = AirPlayPlugin()
    instancia = plugin

    let metodos = FlutterMethodChannel(
      name: "dev.korven.opentv/airplay",
      binaryMessenger: registrar.messenger)
    metodos.setMethodCallHandler { call, result in
      plugin.atender(call, result)
    }

    let eventos = FlutterEventChannel(
      name: "dev.korven.opentv/airplay/events",
      binaryMessenger: registrar.messenger)
    eventos.setStreamHandler(plugin)
  }

  private func atender(_ call: FlutterMethodCall, _ result: FlutterResult) {
    switch call.method {
    case "start":
      guard let args = call.arguments as? [String: Any],
            let url = args["url"] as? String
      else {
        result(FlutterError(code: "args", message: "url requerida", details: nil))
        return
      }
      NSLog("[airplay] Dart pidió start")
      sesionViva().start(url: url, title: args["title"] as? String ?? "")
      result(nil)
    case "stop":
      session?.stop()
      result(nil)
    case "showRoutePicker":
      let args = call.arguments as? [String: Any]
      // sesionViva() y no session?: el selector necesita un AVPlayer al que
      // apuntar ANTES de que se elija destino.
      let abierto = RoutePicker.mostrar(
        x: args?["x"] as? Double ?? 0,
        y: args?["y"] as? Double ?? 0,
        lado: args?["lado"] as? Double ?? 28,
        player: sesionViva().player)
      result(abierto)
    default:
      result(FlutterMethodNotImplemented)
    }
  }

  private func sesionViva() -> AirPlaySession {
    if let s = session { return s }
    let s = AirPlaySession { [weak self] evento in
      DispatchQueue.main.async { self?.sink?(evento) }
    }
    session = s
    return s
  }

  func onListen(withArguments _: Any?, eventSink: @escaping FlutterEventSink) -> FlutterError? {
    sink = eventSink
    // Crear ya la sesión: su AVPlayer tiene que existir antes de que se abra el
    // popover, porque es a él a quien apunta el selector.
    _ = sesionViva()

    // Al cerrarse el popover se arma la sesión. Es la única señal pública que
    // da AVKit; no hay callback de "se eligió esta ruta". Si el usuario no
    // eligió nada, la reproducción no se desviará y AirplayGuard hará el
    // traspaso a local dentro de su presupuesto.
    RoutePicker.alCerrarPopover = { [weak self] in
      self?.sink?(["type": "route", "active": true])
    }
    return nil
  }

  func onCancel(withArguments _: Any?) -> FlutterError? {
    sink = nil
    return nil
  }

  /// Sin esto, el AVPlayer retiene la ruta después de cerrar la app.
  static func alTerminar() {
    instancia?.session?.stop()
  }
}
