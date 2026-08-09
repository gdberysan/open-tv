import 'dart:async';

import '../../data/airplay/airplay_platform.dart';

/// Vigila la carga de una emisión AirPlay.
///
/// Hermano de PlaybackGuard, no una refactorización suya: los dos clasifican al
/// revés. mpv escupe errores recuperables constantemente en HLS en vivo y
/// tratarlos como fatales mataba el vídeo, así que PlaybackGuard los ignora
/// mientras la posición avance. AVPlayer no hace eso: un
/// AVPlayerItem.status == .failed es terminal a la primera. Fundir ambos
/// corrompería el comportamiento de uno de los dos.
class AirplayGuard {
  AirplayGuard({
    required this.onFatal,
    this.onPlaybackConfirmed,
    this.loadTimeout = const Duration(seconds: 15),
  });

  /// [formatError] distingue "no puedo decodificar esto" de "no llegué al
  /// servidor". Solo lo primero autoriza a recordar el canal como incompatible.
  final void Function(String message, {bool formatError}) onFatal;

  final void Function()? onPlaybackConfirmed;
  final Duration loadTimeout;

  Timer? _loadTimer;
  bool _confirmado = false;
  bool _disposed = false;

  /// Armar ANTES de invocar start(): resolver la URL contra el gateway puede
  /// colgarse igual que la carga del stream, y el presupuesto cubre todo el
  /// proceso. Misma razón que en PlaybackGuard.
  void armLoadTimeout() {
    _loadTimer?.cancel();
    _loadTimer = Timer(loadTimeout, () {
      if (_disposed || _confirmado) return;
      onFatal(
        'El canal no llegó a reproducirse en el televisor '
        'en ${loadTimeout.inSeconds}s.',
        formatError: false,
      );
    });
  }

  void onStatus(AirplayStatusEvent e) {
    if (_disposed) return;
    switch (e.state) {
      case AirplayPlaybackState.playing:
        if (_confirmado) return;
        _confirmado = true;
        _loadTimer?.cancel();
        onPlaybackConfirmed?.call();
      case AirplayPlaybackState.failed:
        _loadTimer?.cancel();
        onFatal(e.error ?? 'Fallo de reproducción en el televisor.',
            formatError: e.formatError);
      case AirplayPlaybackState.loading:
        break;
    }
  }

  void dispose() {
    _disposed = true;
    _loadTimer?.cancel();
  }
}
