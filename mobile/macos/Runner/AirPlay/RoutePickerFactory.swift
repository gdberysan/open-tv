import AVKit
import Cocoa
import FlutterMacOS

/// Devuelve el AVRoutePickerView real de Apple. El descubrimiento de
/// dispositivos, el emparejamiento y los cambios de protocolo de tvOS son
/// problema de AVFoundation, no nuestro: por eso no hay ni mDNS ni sockets en
/// todo este módulo.
final class RoutePickerFactory: NSObject, FlutterPlatformViewFactory {
  func create(withViewIdentifier viewId: Int64, arguments args: Any?) -> NSView {
    let picker = AVRoutePickerView()
    picker.isRoutePickerButtonBordered = false
    // Paleta Korven: gris de texto en reposo (graphite200), ámbar de acento
    // cuando hay ruta activa (amber500).
    picker.setRoutePickerButtonColor(
      NSColor(srgbRed: 0.592, green: 0.639, blue: 0.698, alpha: 1), for: .normal)
    picker.setRoutePickerButtonColor(
      NSColor(srgbRed: 1.0, green: 0.541, blue: 0.169, alpha: 1), for: .active)
    return picker
  }
}
