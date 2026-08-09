import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

enum AirplayPlaybackState { loading, playing, failed }

sealed class AirplayEvent {
  const AirplayEvent();
}

/// La ruta del sistema cambió. [active] false a mitad de emisión significa que
/// el televisor se apagó o salió de la red.
class AirplayRouteEvent extends AirplayEvent {
  const AirplayRouteEvent({required this.active, this.name});
  final bool active;
  final String? name;
}

class AirplayStatusEvent extends AirplayEvent {
  const AirplayStatusEvent({
    required this.state,
    this.error,
    this.formatError = false,
  });

  final AirplayPlaybackState state;
  final String? error;

  /// True solo si AVFoundation falló por formato o códec, nunca por red. Es lo
  /// único que autoriza a recordar el canal como incompatible.
  final bool formatError;
}

/// La frontera con la capa nativa. Es una interfaz para que la máquina de
/// estados se pueda testear con un doble: CI corre en ubuntu-latest y allí no
/// hay ningún MethodChannel que responda.
abstract interface class AirplayPlatform {
  Stream<AirplayEvent> get events;
  Future<void> start({required String url, required String title});
  Future<void> stop();
}

class MethodChannelAirplay implements AirplayPlatform {
  static const _metodos = MethodChannel('dev.korven.opentv/airplay');
  static const _eventos = EventChannel('dev.korven.opentv/airplay/events');

  @override
  Stream<AirplayEvent> get events => _eventos
      .receiveBroadcastStream()
      .map(_traducir)
      .where((e) => e != null)
      .cast<AirplayEvent>();

  @override
  Future<void> start({required String url, required String title}) =>
      _metodos.invokeMethod<void>('start', {'url': url, 'title': title});

  @override
  Future<void> stop() => _metodos.invokeMethod<void>('stop');

  static AirplayEvent? _traducir(dynamic raw) {
    if (raw is! Map) return null;
    switch (raw['type']) {
      case 'route':
        return AirplayRouteEvent(
          active: raw['active'] as bool? ?? false,
          name: raw['name'] as String?,
        );
      case 'status':
        final estado = switch (raw['state']) {
          'playing' => AirplayPlaybackState.playing,
          'failed' => AirplayPlaybackState.failed,
          _ => AirplayPlaybackState.loading,
        };
        return AirplayStatusEvent(
          state: estado,
          error: raw['error'] as String?,
          formatError: raw['formatError'] as bool? ?? false,
        );
      default:
        return null;
    }
  }
}

final airplayPlatformProvider =
    Provider<AirplayPlatform>((_) => MethodChannelAirplay());
