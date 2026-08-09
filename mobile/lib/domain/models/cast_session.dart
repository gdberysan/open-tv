/// Estados de la sesión de emisión.
///
/// [armed] existe porque elegir dispositivo y elegir canal son dos actos
/// distintos: con una ruta seleccionada y nada reproduciendo, la barra ya debe
/// verse y el toque en la rejilla ya debe emitir en vez de abrir el reproductor.
enum CastState { idle, armed, connecting, casting, failed }

class CastSession {
  const CastSession({
    this.state = CastState.idle,
    this.deviceName,
    this.channelId,
    this.channelName,
    this.error,
  });

  final CastState state;
  final String? deviceName;
  final String? channelId;
  final String? channelName;
  final String? error;

  /// Si se dibuja la CastBar.
  bool get visible => state != CastState.idle;

  /// Si un toque en la rejilla emite en vez de abrir el reproductor. En
  /// [CastState.failed] es false a propósito: ese estado significa que el
  /// canal se va a reproducir en local.
  bool get intercepta =>
      state == CastState.armed ||
      state == CastState.connecting ||
      state == CastState.casting;

  /// macOS no expone públicamente el nombre de la ruta; cuando CoreAudio no lo
  /// da, decir "AirPlay" es mejor que dejar el hueco vacío.
  String get etiquetaDispositivo => deviceName ?? 'AirPlay';

  CastSession copyWith({
    CastState? state,
    String? deviceName,
    String? channelId,
    String? channelName,
    String? error,
    bool clearError = false,
    bool clearChannel = false,
  }) =>
      CastSession(
        state: state ?? this.state,
        deviceName: deviceName ?? this.deviceName,
        channelId: clearChannel ? null : (channelId ?? this.channelId),
        channelName: clearChannel ? null : (channelName ?? this.channelName),
        error: clearError ? null : (error ?? this.error),
      );
}
