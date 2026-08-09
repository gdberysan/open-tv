import AVKit
import Cocoa

/// Abre el selector de rutas AirPlay del sistema.
///
/// El nombre del fichero dice "Factory" por historia: aquí vivía una
/// FlutterPlatformViewFactory que incrustaba el AVRoutePickerView con AppKitView.
/// No funcionaba, y no por un fallo nuestro: Flutter **no implementa** el reenvío
/// de gestos a las vistas de plataforma en macOS. En
/// `rendering/platform_view.dart`, `RenderAppKitView.updateGestureRecognizers`
/// tiene el cuerpo vacío y un TODO apuntando a flutter/flutter#128519. La vista
/// se dibujaba, pero ningún clic llegaba nunca al NSView.
///
/// Así que el botón lo pinta Flutter —lo que además lo alinea con el resto de
/// acciones de la barra— y aquí solo se abre el popover: se mantiene un
/// AVRoutePickerView real en la jerarquía, oculto DEBAJO de la vista de Flutter,
/// y se le pulsa su NSButton interno por código.
/// Escucha el cierre del popover. Es la única señal pública que hay: el
/// protocolo solo ofrece willBegin/didEndPresentingRoutes, sin ningún callback
/// de "se eligió esta ruta" ni forma pública de consultar el estado del botón
/// (`_setAirPlayActive:`, que sí lo sabe, es API privada).
final class PickerDelegate: NSObject, AVRoutePickerViewDelegate {
  var alCerrar: (() -> Void)?

  func routePickerViewDidEndPresentingRoutes(_ routePickerView: AVRoutePickerView) {
    NSLog("[airplay] popover cerrado → armando sesión")
    alCerrar?()
  }
}

enum RoutePicker {
  /// El selector vive entre llamadas: recrearlo en cada clic hace que el
  /// popover parpadee y pierda su anclaje.
  private static var picker: AVRoutePickerView?
  private static let delegado = PickerDelegate()

  /// Se llama al cerrarse el popover. Arma la sesión de forma optimista: no hay
  /// manera pública de saber si el usuario eligió algo o solo miró. Si no eligió,
  /// la reproducción no se desvía y AirplayGuard hace el traspaso a local.
  static var alCerrarPopover: (() -> Void)? {
    get { delegado.alCerrar }
    set { delegado.alCerrar = newValue }
  }

  /// Abre el popover anclado a (x, y), que llegan en píxeles lógicos y con el
  /// origen arriba a la izquierda, como los da Flutter.
  ///
  /// [player] es obligatorio, no decorativo: en macOS `AVRoutePickerView.player`
  /// es LA forma de decirle al selector qué enrutar. Sin él el popover lista
  /// destinos y los conecta, pero no viaja ningún vídeo — el televisor dice
  /// "conectado" y se queda en negro.
  static func mostrar(x: Double, y: Double, lado: Double, player: AVPlayer) -> Bool {
    guard let ventana = NSApplication.shared.keyWindow ?? NSApplication.shared.windows.first,
          let contentView = ventana.contentView
    else { return false }

    let p = pickerVivo(en: contentView)
    p.player = player

    // AppKit tiene el origen abajo a la izquierda; Flutter, arriba. Sin este
    // volteo el popover sale anclado al extremo opuesto de la ventana.
    let yAppKit = contentView.bounds.height - y - lado
    p.frame = NSRect(x: x, y: yAppKit, width: lado, height: lado)

    guard let boton = botonInterno(de: p) else { return false }
    boton.performClick(nil)
    return true
  }

  private static func pickerVivo(en contentView: NSView) -> AVRoutePickerView {
    if let p = picker, p.superview === contentView { return p }
    let p = AVRoutePickerView()
    p.isRoutePickerButtonBordered = false
    p.delegate = delegado
    // Debajo de la vista de Flutter: queda tapado, pero sigue siendo una vista
    // real y colocada, que es lo que el popover necesita para anclarse. Con
    // alphaValue = 0 AppKit puede saltarse su disposición.
    contentView.addSubview(p, positioned: .below, relativeTo: nil)
    picker = p
    return p
  }

  /// AVRoutePickerView contiene un AVRoutePickerButton, que es un NSButton.
  /// Se busca por tipo y no por índice: la jerarquía interna es de Apple y
  /// puede cambiar entre versiones. Si algún día no aparece, `mostrar` devuelve
  /// false y la app lo dice en vez de fingir que abrió algo.
  private static func botonInterno(de vista: NSView) -> NSButton? {
    if let b = vista as? NSButton { return b }
    for sub in vista.subviews {
      if let b = botonInterno(de: sub) { return b }
    }
    return nil
  }
}
