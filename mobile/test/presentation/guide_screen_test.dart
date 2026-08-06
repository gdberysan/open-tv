import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:iptv_ecosystem/data/repositories/epg_repository.dart';
import 'package:iptv_ecosystem/domain/models/epg_entry.dart';
import 'package:iptv_ecosystem/presentation/providers/channel_provider.dart';
import 'package:iptv_ecosystem/presentation/providers/epg_provider.dart';
import 'package:iptv_ecosystem/presentation/screens/guide_screen.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

/// EPG fake: un programa de 1h por canal dentro de la ventana; los canales
/// en [sinDatos] devuelven guía vacía.
class FakeEPGRepo implements IEPGRepository {
  FakeEPGRepo({this.sinDatos = const {}});
  final Set<String> sinDatos;

  @override
  Future<List<EPGEntry>> getForChannel(String channelId,
      {required DateTime from, required DateTime to}) async {
    if (sinDatos.contains(channelId)) return [];
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
  final fixedNow = DateTime(2026, 8, 6, 14, 47);

  Future<void> pumpGuide(WidgetTester tester,
      {FakeRepo? repo, FakeEPGRepo? epg}) async {
    SharedPreferences.setMockInitialValues({});
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          channelRepositoryProvider
              .overrideWithValue(repo ?? FakeRepo(total: 4)),
          sharedPreferencesProvider.overrideWithValue(prefs),
          epgRepositoryProvider.overrideWithValue(epg ?? FakeEPGRepo()),
          clockProvider.overrideWithValue(() => fixedNow),
        ],
        child: const MaterialApp(home: GuideScreen()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('muestra canales con sus programas', (tester) async {
    await pumpGuide(tester);

    expect(find.text('Canal Par 0'), findsOneWidget);
    expect(find.text('Programa de ch-0'), findsOneWidget);
    expect(find.text('Programa de ch-1'), findsOneWidget);
  });

  testWidgets('la cabecera muestra los slots de 30 min desde la media hora',
      (tester) async {
    await pumpGuide(tester);

    // now = 14:47 → ventana desde 14:30
    expect(find.text('14:30'), findsOneWidget);
    expect(find.text('15:00'), findsOneWidget);
    expect(find.text('15:30'), findsOneWidget);
  });

  testWidgets('canal sin guía muestra "Sin programación"', (tester) async {
    await pumpGuide(tester, epg: FakeEPGRepo(sinDatos: {'ch-1'}));

    expect(find.text('Programa de ch-0'), findsOneWidget);
    expect(find.text('Sin programación'), findsOneWidget);
  });
}
