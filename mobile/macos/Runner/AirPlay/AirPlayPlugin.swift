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
      sesionViva().start(url: url, title: args["title"] as? String ?? "")
      result(nil)
    case "stop":
      session?.stop()
      result(nil)
    case "showRoutePicker":
      let args = call.arguments as? [String: Any]
      let abierto = RoutePicker.mostrar(
        x: args?["x"] as? Double ?? 0,
        y: args?["y"] as? Double ?? 0,
        lado: args?["lado"] as? Double ?? 28)
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

    // El estado de la ruta lo da CoreAudio, no el AVPlayer: encadenarlo al
    // reproductor daba una dependencia circular en la que la sesión no salía
    // nunca de idle. Ver RouteName.
    RouteName.observar { [weak self] esAirPlay, nombre in
      var evento: [String: Any] = ["type": "route", "active": esAirPlay]
      if let nombre = nombre { evento["name"] = nombre }
      self?.sink?(evento)
    }

    // Emitir el estado de arranque: si ya había una ruta puesta antes de abrir
    // la app, sin esto no llegaría ningún evento hasta el siguiente cambio.
    let (esAirPlay, nombre) = RouteName.estadoActual()
    var inicial: [String: Any] = ["type": "route", "active": esAirPlay]
    if let nombre = nombre { inicial["name"] = nombre }
    eventSink(inicial)

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
