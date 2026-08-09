import 'dart:async';

/// Decide cuándo un error del reproductor es realmente fatal.
///
/// mpv emite errores recuperables constantemente en HLS en vivo (EOF de
/// segmento, reconexiones, cambios de pista). Tratarlos todos como fatales
/// mataba el vídeo mientras el audio seguía sonando (bug cazado en vivo con
/// "tcp: ffurl_read returned 0xdfb9b0bb" = AVERROR_EOF).
///
/// Reglas:
/// - Error antes de la primera reproducción → fatal inmediato.
/// - Nada reproduce dentro de [loadTimeout] tras armar la carga → fatal.
/// - Error durante la reproducción → solo es fatal si la posición deja de
///   avanzar durante [stallTimeout]; si el playback sigue, se ignora.
///
/// "Reproducir" significa que la posición AVANZA, no que media_kit diga
/// `playing`. Ver [onPlaying].
class PlaybackGuard {
  PlaybackGuard({
    required this.onFatal,
    this.onPlaybackConfirmed,
    this.loadTimeout = const Duration(seconds: 15),
    this.stallTimeout = const Duration(seconds: 8),
  });

  final void Function(String message) onFatal;

  /// Se llama una sola vez, cuando hay prueba real de reproducción. Es la señal
  /// buena para quitar el indicador de carga.
  final void Function()? onPlaybackConfirmed;

  final Duration loadTimeout;
  final Duration stallTimeout;

  Timer? _loadTimer;
  Timer? _stallTimer;
  bool _started = false;
  bool _disposed = false;
  Duration _lastPosition = Duration.zero;
  bool _advancedSinceError = false;

  /// Primera posición observada. Sirve de referencia: hace falta que la
  /// posición cambie respecto a ella para dar la reproducción por buena.
  Duration? _posicionReferencia;

  /// Armar ANTES de player.open(): si open() se cuelga, el timeout salta
  /// igualmente (el watchdog original se armaba después y nunca saltaba).
  void armLoadTimeout() {
    _loadTimer?.cancel();
    _loadTimer = Timer(loadTimeout, () {
      if (_disposed || _started) return;
      onFatal('El canal no llegó a reproducir en ${loadTimeout.inSeconds}s.\n'
          'Puede estar caído, geo-bloqueado o la URL expiró.');
    });
  }

  /// OJO: `playing` NO es prueba de que se esté reproduciendo. media_kit lo
  /// emite dentro de `open()` cuando `play: true` y otra vez en
  /// MPV_EVENT_START_FILE, es decir, en cuanto mpv EMPIEZA a abrir el archivo
  /// y antes de decodificar un solo fotograma. Es una declaración de
  /// intención.
  ///
  /// Desarmar el watchdog aquí dejaba la pantalla colgada para siempre en
  /// cualquier stream cuyo manifiesto carga pero cuyos segmentos nunca llegan
  /// —el caso real fue un canal geo-bloqueado cuyo CDN devuelve 405 a cada
  /// segmento—, porque libavformat se queda reintentando sin emitir error y el
  /// guard nunca oía un fallo que le hiciera reaccionar.
  void onPlaying(bool playing) {
    // Deliberadamente no confirma nada: la prueba llega por onPosition.
  }

  void onPosition(Duration position) {
    if (position != _lastPosition) {
      _lastPosition = position;
      _advancedSinceError = true;
    }

    if (_started || _disposed) return;
    // La primera muestra solo fija la referencia; una posición que se repite
    // es exactamente lo que se ve cuando mpv está atascado.
    if (_posicionReferencia == null) {
      _posicionReferencia = position;
      return;
    }
    if (position != _posicionReferencia) {
      _started = true;
      _loadTimer?.cancel();
      onPlaybackConfirmed?.call();
    }
  }

  void onError(String message) {
    if (_disposed || message.isEmpty) return;

    if (!_started) {
      _loadTimer?.cancel();
      onFatal(message);
      return;
    }

    // Ya en reproducción: vigilar en vez de matar. Si ya hay una vigilancia
    // en curso, no re-armar (errores repetidos no deben extender la ventana).
    if (_stallTimer?.isActive ?? false) return;
    _advancedSinceError = false;
    _stallTimer = Timer(stallTimeout, () {
      if (_disposed) return;
      if (!_advancedSinceError) {
        onFatal(message);
      }
    });
  }

  void dispose() {
    _disposed = true;
    _loadTimer?.cancel();
    _stallTimer?.cancel();
  }
}
