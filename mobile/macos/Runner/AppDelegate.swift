import Cocoa
import FlutterMacOS

@main
class AppDelegate: FlutterAppDelegate {
  override func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
    return true
  }

  override func applicationSupportsSecureRestorableState(_ app: NSApplication) -> Bool {
    return true
  }

  /// Sin esto, el AVPlayer de la sesión de emisión retiene la ruta AirPlay
  /// después de cerrar la app.
  override func applicationWillTerminate(_ notification: Notification) {
    AirPlayPlugin.alTerminar()
  }
}
