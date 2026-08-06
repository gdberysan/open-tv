import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

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
    ),
  );

  @override
  Future<List<Channel>> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit = 500,
    int offset = 0,
  }) async {
    getChannelsCalls++;
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
    return hits.skip(offset).take(limit).toList();
  }

  @override
  Future<String> getStreamUrl(String channelId) async => 'http://x/$channelId';
}

void main() {
  ProviderContainer containerCon(FakeRepo repo) {
    final container = ProviderContainer(overrides: [
      channelRepositoryProvider.overrideWithValue(repo),
    ]);
    addTearDown(container.dispose);
    return container;
  }

  test('la carga inicial trae la primera página y detecta que hay más', () async {
    final container = containerCon(FakeRepo(total: 600));

    final state = await container.read(channelListProvider.future);

    expect(state.channels, hasLength(500));
    expect(state.hasMore, isTrue);
  });

  test('loadMore añade la siguiente página y detecta el final', () async {
    final container = containerCon(FakeRepo(total: 600));
    await container.read(channelListProvider.future);

    await container.read(channelListProvider.notifier).loadMore();

    final state = container.read(channelListProvider).requireValue;
    expect(state.channels, hasLength(600));
    expect(state.channels.last.id, 'ch-599');
    expect(state.hasMore, isFalse);
  });

  test('loadMore sin más páginas no vuelve a pedir', () async {
    final repo = FakeRepo(total: 600);
    final container = containerCon(repo);
    await container.read(channelListProvider.future);
    await container.read(channelListProvider.notifier).loadMore();
    final llamadasAntes = repo.getChannelsCalls;

    await container.read(channelListProvider.notifier).loadMore();

    expect(repo.getChannelsCalls, llamadasAntes);
  });

  test('cambiar el filtro resetea la lista y busca en servidor', () async {
    final repo = FakeRepo(total: 600);
    final container = containerCon(repo);
    await container.read(channelListProvider.future);

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(query: 'Impar');

    final state = await container.read(channelListProvider.future);
    // 300 impares de 600; una sola página, todos matchean el query
    expect(state.channels, hasLength(300));
    expect(state.hasMore, isFalse);
    expect(state.channels.every((c) => c.name.contains('Impar')), isTrue);
  });

  test('un error en loadMore conserva lo ya cargado', () async {
    final repo = FakeRepo(total: 600);
    final container = containerCon(repo);
    await container.read(channelListProvider.future);

    repo.failNext = true;
    await container.read(channelListProvider.notifier).loadMore();

    final state = container.read(channelListProvider).requireValue;
    expect(state.channels, hasLength(500));
    expect(state.isLoadingMore, isFalse);
  });
}
