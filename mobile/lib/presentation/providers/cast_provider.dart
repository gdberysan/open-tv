import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/airplay/airplay_platform.dart';
import '../../data/api_error.dart';
import '../../domain/models/cast_session.dart';
import '../../domain/models/channel.dart';
import '../player/airplay_guard.dart';
import 'airplay_memory_provider.dart';
import 'channel_provider.dart';

/// Sesión de emisión AirPlay a nivel de app.
///
/// No navega nunca. Publica [CastState.failed] con el motivo y es la pantalla
/// activa quien decide qué hacer con eso. Mantener la navegación fuera de aquí
/// es lo que permite testear el traspaso sin WidgetTester ni árbol de widgets,
/// que es obligatorio: CI corre en ubuntu-latest.
class CastNotifier extends Notifier<CastSession> {
  late final AirplayPlatform _plataforma;
  StreamSubscription<AirplayEvent>? _sub;
  AirplayGuard? _guard;

  /// Generación de carga, por el mismo motivo que en PlayerScreen: dos toques
  /// rápidos en canales distintos dejaban que la resolución más lenta pisara el
  /// estado de la más reciente al volver de su await.
  int _generacion = 0;

  @override
  CastSession build() {
    _plataforma = ref.watch(airplayPlatformProvider);
    _sub = _plataforma.events.listen(_alEvento);
    ref.onDispose(() {
      _sub?.cancel();
      _guard?.dispose();
    });
    return const CastSession();
  }

  Future<void> reproducir(Channel ch) => reproducirPorId(ch.id, ch.name);

  /// Por id y nombre porque PlayerScreen no tiene el Channel entero: solo
  /// recibe esos dos campos. Sin esto, ceder a la tele lo que ya se está viendo
  /// en local exigiría volver a pedir el canal al gateway.
  Future<void> reproducirPorId(String channelId, String channelName) async {
    final generacion = ++_generacion;

    _guard?.dispose();
    final guard = AirplayGuard(
      onFatal: (mensaje, {formatError = false}) =>
          _alFallo(channelId, mensaje, formatError: formatError),
    );
    _guard = guard;
    // Armar antes de resolver la URL: el gateway puede colgarse igual que la
    // carga del stream, y el presupuesto cubre todo el proceso.
    guard.armLoadTimeout();

    state = state.copyWith(
      state: CastState.connecting,
      channelId: channelId,
      channelName: channelName,
      clearError: true,
    );

    try {
      final url = await ref.read(channelRepositoryProvider).getStreamUrl(channelId);
      if (generacion != _generacion) return;
      await _plataforma.start(url: url, title: channelName);
    } catch (e) {
      if (generacion != _generacion) return;
      guard.dispose();
      _alFallo(channelId, ApiError.desde(e).mensaje, formatError: false);
    }
  }

  Future<void> detener() async {
    _guard?.dispose();
    _guard = null;
    await _plataforma.stop();
    // A armed y no a idle: la ruta del sistema sigue seleccionada — una app no
    // puede deseleccionarla, esa UI es de Apple — así que decir "desconectado"
    // sería mentir.
    state = state.copyWith(
      state: state.deviceName == null ? CastState.idle : CastState.armed,
      clearChannel: true,
      clearError: true,
    );
  }

  /// La pantalla llama a esto cuando ya ha consumido el traspaso y ha puesto el
  /// canal en local.
  void reconocerFallo() {
    if (state.state != CastState.failed) return;
    state = state.copyWith(
      state: state.deviceName == null ? CastState.idle : CastState.armed,
      clearChannel: true,
      clearError: true,
    );
  }

  void _alFallo(String channelId, String mensaje, {required bool formatError}) {
    // Solo los fallos de formato enseñan algo sobre el canal. Un Apple TV
    // dormido y un stream MPEG-2 llegan los dos como fallo, y marcar por red
    // etiquetaría canales buenos para siempre.
    if (formatError) {
      ref.read(airplayMemoryProvider.notifier).marcarIncompatible(channelId);
    }
    unawaited(_plataforma.stop());
    state = state.copyWith(state: CastState.failed, error: mensaje);
  }

  void _alEvento(AirplayEvent e) {
    switch (e) {
      case AirplayRouteEvent(active: final activa, name: final nombre):
        if (!activa) {
          _guard?.dispose();
          _guard = null;
          state = const CastSession();
          return;
        }
        state = state.copyWith(
          state: state.state == CastState.idle ? CastState.armed : state.state,
          deviceName: nombre,
        );
      case AirplayStatusEvent():
        _guard?.onStatus(e);
        if (e.state == AirplayPlaybackState.playing) {
          state = state.copyWith(state: CastState.casting, clearError: true);
        }
    }
  }
}

final castProvider =
    NotifierProvider<CastNotifier, CastSession>(CastNotifier.new);
