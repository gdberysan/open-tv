import CoreAudio
import Foundation

/// Detecta si la salida del sistema es un destino AirPlay, y cómo se llama.
///
/// Existe porque el estado de la ruta NO se puede sacar del AVPlayer: su
/// `isExternalPlaybackActive` solo se puede observar cuando ya hay un AVPlayer,
/// y el AVPlayer solo se crea cuando ya hay sesión de emisión. Encadenar las dos
/// cosas daba una dependencia circular en la que la sesión no salía nunca de
/// `idle`: el televisor decía "conectado" y la app no se enteraba de nada.
///
/// Al elegir un destino en el selector, macOS pasa la salida por defecto del
/// sistema a ese receptor. Eso sí es observable sin reproducir nada, y el tipo
/// de transporte distingue un AirPlay de unos cascos USB o Bluetooth.
enum RouteName {
  private static var escuchando = false
  private static var alCambiar: ((Bool, String?) -> Void)?

  /// (esAirPlay, nombre) de la salida por defecto del sistema ahora mismo.
  static func estadoActual() -> (Bool, String?) {
    guard let id = dispositivoPorDefecto() else { return (false, nil) }
    let esAirPlay = transporte(de: id) == kAudioDeviceTransportTypeAirPlay
    return (esAirPlay, esAirPlay ? nombre(de: id) : nil)
  }

  /// Instala un observador del dispositivo de salida por defecto. Idempotente:
  /// llamarlo dos veces no duplica el listener.
  static func observar(_ cb: @escaping (Bool, String?) -> Void) {
    alCambiar = cb
    guard !escuchando else { return }
    escuchando = true

    var addr = AudioObjectPropertyAddress(
      mSelector: kAudioHardwarePropertyDefaultOutputDevice,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    AudioObjectAddPropertyListenerBlock(
      AudioObjectID(kAudioObjectSystemObject), &addr, DispatchQueue.main
    ) { _, _ in
      let (esAirPlay, nombre) = estadoActual()
      alCambiar?(esAirPlay, nombre)
    }
  }

  // ── CoreAudio ──────────────────────────────────────────────────────────────
  // kAudioObjectPropertyElementMain exige macOS 12 y el target es 10.15.

  private static func dispositivoPorDefecto() -> AudioDeviceID? {
    var deviceID = AudioDeviceID(0)
    var size = UInt32(MemoryLayout<AudioDeviceID>.size)
    var addr = AudioObjectPropertyAddress(
      mSelector: kAudioHardwarePropertyDefaultOutputDevice,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    guard AudioObjectGetPropertyData(
      AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size, &deviceID) == noErr
    else { return nil }
    return deviceID
  }

  private static func transporte(de id: AudioDeviceID) -> UInt32 {
    var tipo = UInt32(0)
    var size = UInt32(MemoryLayout<UInt32>.size)
    var addr = AudioObjectPropertyAddress(
      mSelector: kAudioDevicePropertyTransportType,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    guard AudioObjectGetPropertyData(id, &addr, 0, nil, &size, &tipo) == noErr else {
      return 0
    }
    return tipo
  }

  private static func nombre(de id: AudioDeviceID) -> String? {
    var nombre: CFString = "" as CFString
    var size = UInt32(MemoryLayout<CFString>.size)
    var addr = AudioObjectPropertyAddress(
      mSelector: kAudioObjectPropertyName,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    guard AudioObjectGetPropertyData(id, &addr, 0, nil, &size, &nombre) == noErr else {
      return nil
    }
    let s = nombre as String
    return s.isEmpty ? nil : s
  }
}
