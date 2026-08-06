import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:iptv_ecosystem/data/repositories/epg_repository.dart';
import 'package:iptv_ecosystem/domain/models/epg_entry.dart';
import 'package:iptv_ecosystem/presentation/providers/epg_provider.dart';

class FakeEPGRepo implements IEPGRepository {
  int calls = 0;
  DateTime? lastFrom;
  DateTime? lastTo;
  String? lastChannel;

  @override
  Future<List<EPGEntry>> getForChannel(String channelId,
      {required DateTime from, required DateTime to}) async {
    calls++;
    lastChannel = channelId;
    lastFrom = from;
    lastTo = to;
    return [
      EPGEntry(
        channelId: channelId,
        title: 'Programa de $channelId',
        description: '',
        startAt: from,
        endAt: from.add(const Duration(hours: 1)),
      ),
    ];
  }
}

void main() {
  test('pide la ventana desde la media hora en curso, 6h en adelante', () async {
    final repo = FakeEPGRepo();
    final fixedNow = DateTime(2026, 8, 6, 14, 47);
    final container = ProviderContainer(overrides: [
      epgRepositoryProvider.overrideWithValue(repo),
      clockProvider.overrideWithValue(() => fixedNow),
    ]);
    addTearDown(container.dispose);

    final sub = container.listen(epgForChannelProvider('ch-1'), (_, __) {});
    addTearDown(sub.close);
    final entries = await container.read(epgForChannelProvider('ch-1').future);

    expect(entries, hasLength(1));
    expect(repo.lastChannel, 'ch-1');
    expect(repo.lastFrom, DateTime(2026, 8, 6, 14, 30));
    expect(repo.lastTo, DateTime(2026, 8, 6, 20, 30));
  });

  test('se auto-refresca en cada intervalo', () async {
    final repo = FakeEPGRepo();
    final container = ProviderContainer(overrides: [
      epgRepositoryProvider.overrideWithValue(repo),
      epgRefreshIntervalProvider.overrideWithValue(
          const Duration(milliseconds: 60)),
    ]);
    addTearDown(container.dispose);

    // listen mantiene vivo el provider autoDispose durante el test
    final sub = container.listen(epgForChannelProvider('ch-1'), (_, __) {});
    addTearDown(sub.close);
    await container.read(epgForChannelProvider('ch-1').future);
    expect(repo.calls, 1);

    await Future<void>.delayed(const Duration(milliseconds: 160));

    // Al menos un refresh tras el fetch inicial
    expect(repo.calls, greaterThanOrEqualTo(2));
  });
}
