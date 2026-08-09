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
    registrar.register(
      RoutePickerFactory(),
      withId: "dev.korven.opentv/route-picker")

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
      sesionViva().start(url: url, title: args["title"] as? String ?? "")
      result(nil)
    case "stop":
      session?.stop()
      result(nil)
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
