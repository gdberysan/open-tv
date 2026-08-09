import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/data/airplay/airplay_platform.dart';
import 'package:korven_open_tv/data/repositories/channel_repository.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';
import 'package:korven_open_tv/domain/models/channel.dart';
import 'package:korven_open_tv/domain/models/channel_filter.dart';
import 'package:korven_open_tv/presentation/providers/airplay_memory_provider.dart';
import 'package:korven_open_tv/presentation/providers/cast_provider.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

class FakeAirplayPlatform implements AirplayPlatform {
  final _ctrl = StreamController<AirplayEvent>.broadcast();
  final List<String> iniciados = [];
  int paradas = 0;

  @override
  Stream<AirplayEvent> get events => _ctrl.stream;

  @override
  Future<void> start({required String url, required String title}) async {
    iniciados.add(url);
  }

  @override
  Future<void> stop() async {
    paradas++;
  }

  int selectoresAbiertos = 0;

  @override
  Future<bool> showRoutePicker({
    required double x,
    required double y,
    required double lado,
  }) async {
    selectoresAbiertos++;
    return true;
  }

  void emitir(AirplayEvent e) => _ctrl.add(e);
  void cerrar() => _ctrl.close();
}

class FakeChannelRepo implements IChannelRepository {
  Object? error;

  @override
  Future<String> getStreamUrl(String channelId) async {
    if (error != null) throw error!;
    return 'http://stream/$channelId.m3u8';
  }

  // filter lleva default y no `required`: en un override no se puede endurecer
  // un parámetro opcional de la interfaz.
  @override
  Future<ChannelPage> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit = 500,
    int offset = 0,
    List<String>? ids,
  }) async =>
      const ChannelPage(channels: [], total: 0);

  @override
  Future<Channel> getRandomChannel(ChannelFilter filter) async => const Channel(
        id: 'x',
        name: 'x',
        logoUrl: '',
        categoryId: '',
        languageCode: '',
        countryCode: '',
        providerType: '',
      );
}

const _bbc = Channel(
  id: 'bbc',
  name: 'BBC News',
  logoUrl: '',
  categoryId: '',
  languageCode: '',
  countryCode: 'GB',
  providerType: 'opensource',
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late FakeAirplayPlatform plataforma;
  late FakeChannelRepo repo;

  Future<ProviderContainer> contenedor() async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    plataforma = FakeAirplayPlatform();
    repo = FakeChannelRepo();
    final c = ProviderContainer(overrides: [
      sharedPreferencesProvider.overrideWithValue(prefs),
      airplayPlatformProvider.overrideWithValue(plataforma),
      channelRepositoryProvider.overrideWithValue(repo),
    ]);
    addTearDown(() {
      c.dispose();
      plataforma.cerrar();
    });
    // Forzar build() para que se suscriba a los eventos.
    c.read(castProvider);
    return c;
  }

  test('una ruta activa arma la sesión', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await Future<void>.delayed(Duration.zero);

    final s = c.read(castProvider);
    expect(s.state, CastState.armed);
    expect(s.deviceName, 'Salón');
  });

  test('reproducir resuelve la URL y arranca la emisión', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);

    expect(plataforma.iniciados, ['http://stream/bbc.m3u8']);
    final s = c.read(castProvider);
    expect(s.state, CastState.connecting);
    expect(s.channelName, 'BBC News');
  });

  test('playing pasa a casting', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma
        .emitir(const AirplayStatusEvent(state: AirplayPlaybackState.playing));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.casting);
  });

  test('cambiar de canal no vuelve a pedir dispositivo', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await Future<void>.delayed(Duration.zero);

    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma
        .emitir(const AirplayStatusEvent(state: AirplayPlaybackState.playing));
    await Future<void>.delayed(Duration.zero);

    const cnn = Channel(
      id: 'cnn',
      name: 'CNN',
      logoUrl: '',
      categoryId: '',
      languageCode: '',
      countryCode: 'US',
      providerType: 'opensource',
    );
    await c.read(castProvider.notifier).reproducir(cnn);

    expect(plataforma.iniciados.length, 2);
    expect(c.read(castProvider).deviceName, 'Salón',
        reason: 'el dispositivo sobrevive al cambio de canal');
  });

  test('un fallo de formato pasa a failed y recuerda el canal', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayStatusEvent(
      state: AirplayPlaybackState.failed,
      error: 'códec no soportado',
      formatError: true,
    ));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.failed);
    expect(plataforma.paradas, greaterThan(0));
    expect(c.read(airplayMemoryProvider), contains('bbc'));
  });

  test('un fallo de red NO marca el canal como incompatible', () async {
    final c = await contenedor();
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayStatusEvent(
      state: AirplayPlaybackState.failed,
      error: 'sin conexión',
      formatError: false,
    ));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.failed);
    expect(c.read(airplayMemoryProvider), isEmpty,
        reason: 'un Apple TV dormido no vuelve incompatible un canal bueno');
  });

  test('perder la ruta a mitad de emisión devuelve a idle', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayRouteEvent(active: false));
    await Future<void>.delayed(Duration.zero);

    expect(c.read(castProvider).state, CastState.idle);
  });

  test('detener para la emisión y conserva la ruta armada', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await Future<void>.delayed(Duration.zero);
    await c.read(castProvider.notifier).reproducir(_bbc);
    await c.read(castProvider.notifier).detener();

    expect(plataforma.paradas, greaterThan(0));
    expect(c.read(castProvider).state, CastState.armed);
  });

  test('un gateway caído deja la sesión en failed', () async {
    final c = await contenedor();
    repo.error = Exception('conexión rechazada');
    await c.read(castProvider.notifier).reproducir(_bbc);

    final s = c.read(castProvider);
    expect(s.state, CastState.failed);
    expect(s.error, isNotNull);
  });

  test('reconocerFallo devuelve a armed si la ruta sigue puesta', () async {
    final c = await contenedor();
    plataforma.emitir(const AirplayRouteEvent(active: true, name: 'Salón'));
    await Future<void>.delayed(Duration.zero);
    await c.read(castProvider.notifier).reproducir(_bbc);
    plataforma.emitir(const AirplayStatusEvent(
        state: AirplayPlaybackState.failed, error: 'x'));
    await Future<void>.delayed(Duration.zero);

    c.read(castProvider.notifier).reconocerFallo();
    expect(c.read(castProvider).state, CastState.armed);
  });
}
