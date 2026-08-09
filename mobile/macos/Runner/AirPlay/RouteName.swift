import Foundation

/// Por qué no hay aquí una detección de ruta por CoreAudio.
///
/// Hubo una, y estaba mal. La idea era: al elegir destino en el selector, macOS
/// pasaría la salida por defecto del sistema al receptor, y observando
/// `kAudioHardwarePropertyDefaultOutputDevice` se sabría cuándo hay ruta puesta
/// y cómo se llama.
///
/// Es falso. Enumerando los dispositivos de CoreAudio con el televisor ya
/// conectado y emitiendo, la salida por defecto seguía siendo `MacBook Pro
/// Speakers` (transporte `bltn`) y el receptor **no aparecía en la lista**:
///
///     SALIDA POR DEFECTO: MacBook Pro Speakers  transporte=bltn
///     hdmi  BenQ GW2790
///     usb   HD Pro Webcam C920
///     bltn  MacBook Pro Microphone
///     bltn  MacBook Pro Speakers   <-- por defecto
///
/// `AVRoutePickerView` no enruta el sistema: enruta **un AVPlayer concreto**,
/// el que se le asigne en su propiedad `player` (solo macOS). El estado de la
/// ruta, por tanto, solo lo conoce ese reproductor, vía
/// `isExternalPlaybackActive`. Está en `AirPlaySession`.
///
/// Consecuencia para la UI: macOS no expone públicamente el nombre del destino,
/// así que la barra dice "AirPlay" a secas. `CastSession.etiquetaDispositivo` ya
/// contempla ese caso.
///
/// El fichero se conserva vacío de lógica para no tener que tocar el target de
/// Xcode a mano solo por borrarlo.
enum RouteName {}
