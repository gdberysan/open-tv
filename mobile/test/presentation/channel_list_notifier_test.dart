import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:iptv_ecosystem/data/repositories/channel_repository.dart';
import 'package:iptv_ecosystem/domain/models/channel.dart';
import 'package:iptv_ecosystem/domain/models/channel_filter.dart';
import 'package:iptv_ecosystem/presentation/providers/channel_provider.dart';

/// Repo fake con [total] canales; aplica q/limit/offset como el gateway.
class FakeRepo implements IChannelRepository {
  FakeRepo({this.total = 600});

  final int total;
  int getChannelsCalls = 0;
  bool failNext = false;
  ChannelFilter? lastFilter;

  /// Cuando está activo, getChannels deja la petición en vuelo hasta que el
  /// test llame a completarPendiente(). Sirve para reproducir la carrera entre
  /// una página en curso y un cambio de filtro.
  bool retenerSiguiente = false;
  Completer<ChannelPage>? _pendiente;

  void completarPendiente() {
    final p = _pendiente;
    _pendiente = null;
    if (p != null && !p.isCompleted) {
      p.complete(const ChannelPage(channels: [], total: 0));
    }
  }

  // Salud rotativa: i%3==0 vivo (latencia 100·i), i%3==1 muerto, i%3==2 sin
  // chequear — cubre los tres estados del indicador de señal.
  late final List<Channel> _all = List.generate(
    total,
    (i) => Channel(
      id: 'ch-$i',
      name: i.isEven ? 'Canal Par $i' : 'Canal Impar $i',
      logoUrl: '',
      categoryId: '',
      languageCode: '',
      countryCode: '',
      providerType: 'opensource',
      alive: i % 3 == 0 ? true : (i % 3 == 1 ? false : null),
      latencyMs: i % 3 == 0 ? 100 * (i % 12 + 1) : 0,
    ),
  );

  @override
  Future<ChannelPage> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit = 500,
    int offset = 0,
  }) async {
    getChannelsCalls++;
    lastFilter = filter;
    if (retenerSiguiente) {
      retenerSiguiente = false;
      _pendiente = Completer<ChannelPage>();
      return _pendiente!.future;
    }
    if (failNext) {
      failNext = false;
      throw Exception('gateway caído');
    }
    var hits = _all;
    if (filter.query.isNotEmpty) {
      hits = hits
          .where((c) =>
              c.name.toLowerCase().contains(filter.query.toLowerCase()))
          .toList();
    }
    return ChannelPage(
      channels: hits.skip(offset).take(limit).toList(),
      total: hits.length,
    );
  }

  @override
  Future<String> getStreamUrl(String channelId) async => 'http://x/$channelId';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> containerCon(FakeRepo repo) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    final container = ProviderContainer(overrides: [
      channelRepositoryProvider.overrideWithValue(repo),
      sharedPreferencesProvider.overrideWithValue(prefs),
    ]);
    addTearDown(container.dispose);
    return container;
  }

  test('la carga inicial trae la primera página y detecta que hay más', () async {
    final container = await containerCon(FakeRepo(total: 600));

    final state = await container.read(channelListProvider.future);

    expect(state.channels, hasLength(500));
    expect(state.hasMore, isTrue);
  });

  test('loadMore añade la siguiente página y detecta el final', () async {
    final container = await containerCon(FakeRepo(total: 600));
    await container.read(channelListProvider.future);

    await container.read(channelListProvider.notifier).loadMore();

    final state = container.read(channelListProvider).requireValue;
    expect(state.channels, hasLength(600));
    expect(state.channels.last.id, 'ch-599');
    expect(state.hasMore, isFalse);
  });

  test('loadMore sin más páginas no vuelve a pedir', () async {
    final repo = FakeRepo(total: 600);
    final container = await containerCon(repo);
    await container.read(channelListProvider.future);
    await container.read(channelListProvider.notifier).loadMore();
    final llamadasAntes = repo.getChannelsCalls;

    await container.read(channelListProvider.notifier).loadMore();

    expect(repo.getChannelsCalls, llamadasAntes);
  });

  test('cambiar el filtro resetea la lista y busca en servidor', () async {
    final repo = FakeRepo(total: 600);
    final container = await containerCon(repo);
    await container.read(channelListProvider.future);

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(query: 'Impar');

    final state = await container.read(channelListProvider.future);
    // 300 impares de 600; una sola página, todos matchean el query
    expect(state.channels, hasLength(300));
    expect(state.hasMore, isFalse);
    expect(state.channels.every((c) => c.name.contains('Impar')), isTrue);
  });

  test('un error en loadMore conserva lo ya cargado y lo expone', () async {
    final repo = FakeRepo(total: 600);
    final container = await containerCon(repo);
    await container.read(channelListProvider.future);

    repo.failNext = true;
    await container.read(channelListProvider.notifier).loadMore();

    final state = container.read(channelListProvider).requireValue;
    expect(state.channels, hasLength(500));
    expect(state.isLoadingMore, isFalse);
    // Sin esto el error se tragaba y la UI seguía mostrando un spinner.
    expect(state.loadMoreError, isNotNull);
    expect(state.hasMore, isTrue, reason: 'sigue habiendo páginas que cargar');
  });

  test('un loadMore posterior con éxito limpia el error', () async {
    final repo = FakeRepo(total: 600);
    final container = await containerCon(repo);
    await container.read(channelListProvider.future);

    repo.failNext = true;
    await container.read(channelListProvider.notifier).loadMore();
    expect(container.read(channelListProvider).requireValue.loadMoreError,
        isNotNull);

    await container.read(channelListProvider.notifier).loadMore();

    final state = container.read(channelListProvider).requireValue;
    expect(state.loadMoreError, isNull);
    expect(state.channels, hasLength(600));
  });

  test('loadMore no pisa el resultado de un filtro cambiado a media carga',
      () async {
    final repo = FakeRepo(total: 1200);
    final container = await containerCon(repo);
    await container.read(channelListProvider.future);

    // Página en vuelo que no resolverá hasta que lo digamos.
    repo.retenerSiguiente = true;
    final enVuelo = container.read(channelListProvider.notifier).loadMore();

    // El usuario escribe en el buscador mientras tanto.
    // "Impar" discrimina de verdad: "Canal Par N" NO lo contiene, mientras
    // que query 'Par' casaría también con "Impar" y el test no probaría nada.
    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(query: 'Impar');
    await container.read(channelListProvider.future);
    final trasFiltro = container.read(channelListProvider).requireValue;

    // Ahora resuelve la página vieja: no debe tocar nada.
    repo.completarPendiente();
    await enVuelo;

    final estadoFinal = container.read(channelListProvider).requireValue;
    expect(estadoFinal.channels.length, trasFiltro.channels.length,
        reason: 'la continuación obsoleta no puede reintroducir la lista vieja');
    // El fake filtra sin distinguir mayúsculas, igual que el gateway.
    expect(
      estadoFinal.channels.every((c) => c.name.toLowerCase().contains('impar')),
      isTrue,
      reason: 'solo deben quedar los canales del filtro vigente',
    );
  });

  test('ChannelFilter compara por valor', () {
    expect(const ChannelFilter(query: 'a', country: 'ES'),
        const ChannelFilter(query: 'a', country: 'ES'));
    expect(const ChannelFilter(query: 'a').hashCode,
        const ChannelFilter(query: 'a').hashCode);
    expect(const ChannelFilter(query: 'a'),
        isNot(const ChannelFilter(query: 'b')));
  });
}
