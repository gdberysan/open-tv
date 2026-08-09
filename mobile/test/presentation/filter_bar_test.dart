import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/domain/models/channel_filter.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:korven_open_tv/presentation/widgets/filter_bar.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

Future<ProviderContainer> _pump(WidgetTester tester, FakeRepo repo) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  final container = ProviderContainer(overrides: [
    channelRepositoryProvider.overrideWithValue(repo),
    sharedPreferencesProvider.overrideWithValue(prefs),
  ]);
  addTearDown(container.dispose);
  await tester.pumpWidget(UncontrolledProviderScope(
    container: container,
    child: const MaterialApp(home: Scaffold(body: FilterBar())),
  ));
  await tester.pumpAndSettle();
  return container;
}

void main() {
  testWidgets('una faceta activa se muestra como chip removible',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(country: 'MX');
    await tester.pumpAndSettle();

    expect(find.textContaining('MX'), findsOneWidget);

    // La × la quita, sin abrir ningún diálogo.
    await tester.tap(find.byTooltip('Quitar filtro de país'));
    await tester.pumpAndSettle();
    expect(container.read(channelFilterProvider).country, isEmpty);
  });

  testWidgets('limpiar solo aparece cuando hay algo que limpiar',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));
    expect(find.text('limpiar'), findsNothing);

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(country: 'MX', category: 'News');
    await tester.pumpAndSettle();
    expect(find.text('limpiar'), findsOneWidget);

    await tester.tap(find.text('limpiar'));
    await tester.pumpAndSettle();

    final f = container.read(channelFilterProvider);
    expect(f.country, isEmpty);
    expect(f.category, isEmpty);
    expect(f.quality, 'fhd', reason: 'limpiar devuelve la calidad al default');
  });

  testWidgets('el contador muestra el total del gateway, no lo cargado',
      (tester) async {
    // FakeRepo entrega páginas de 500 pero conoce un total de 1200.
    await _pump(tester, FakeRepo(total: 1200));
    expect(find.textContaining('1200'), findsOneWidget);
  });
}
