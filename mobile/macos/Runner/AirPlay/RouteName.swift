import CoreAudio
import Foundation

/// Nombre del destino AirPlay, best-effort.
///
/// macOS no expone públicamente el nombre de la ruta de un AVPlayer. Cuando
/// AirPlay se activa, el dispositivo de salida por defecto del sistema pasa a
/// ser el receptor, así que CoreAudio da el nombre correcto en la práctica. Si
/// falla, la UI dice "AirPlay" y no se pierde nada.
enum RouteName {
  static func salidaPorDefecto() -> String? {
    var deviceID = AudioDeviceID(0)
    var size = UInt32(MemoryLayout<AudioDeviceID>.size)
    // kAudioObjectPropertyElementMain exige macOS 12 y el target es 10.15.
    var addr = AudioObjectPropertyAddress(
      mSelector: kAudioHardwarePropertyDefaultOutputDevice,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    guard AudioObjectGetPropertyData(
      AudioObjectID(kAudioObjectSystemObject), &addr, 0, nil, &size, &deviceID) == noErr
    else { return nil }

    var nombre: CFString = "" as CFString
    var nombreSize = UInt32(MemoryLayout<CFString>.size)
    var nombreAddr = AudioObjectPropertyAddress(
      mSelector: kAudioObjectPropertyName,
      mScope: kAudioObjectPropertyScopeGlobal,
      mElement: kAudioObjectPropertyElementMaster)

    guard AudioObjectGetPropertyData(
      deviceID, &nombreAddr, 0, nil, &nombreSize, &nombre) == noErr
    else { return nil }

    let s = nombre as String
    return s.isEmpty ? nil : s
  }
}
