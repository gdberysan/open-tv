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
enum RoutePicker {
  /// El selector vive entre llamadas: recrearlo en cada clic hace que el
  /// popover parpadee y pierda su anclaje.
  private static var picker: AVRoutePickerView?

  /// Abre el popover anclado a (x, y), que llegan en píxeles lógicos y con el
  /// origen arriba a la izquierda, como los da Flutter.
  static func mostrar(x: Double, y: Double, lado: Double) -> Bool {
    guard let ventana = NSApplication.shared.keyWindow ?? NSApplication.shared.windows.first,
          let contentView = ventana.contentView
    else { return false }

    let p = pickerVivo(en: contentView)

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
