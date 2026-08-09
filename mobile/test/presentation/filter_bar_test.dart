import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/data/repositories/facet_repository.dart';
import 'package:korven_open_tv/domain/models/channel_filter.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:korven_open_tv/presentation/providers/facet_provider.dart';
import 'package:korven_open_tv/presentation/widgets/filter_bar.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

class FakeFacetRepo implements IFacetRepository {
  @override
  Future<List<Faceta>> countries() async => const [
        Faceta(valor: 'MX', count: 120),
        Faceta(valor: 'ES', count: 188),
      ];

  @override
  Future<List<Faceta>> categories() async => const [
        Faceta(valor: 'Movies', count: 655),
        Faceta(valor: 'Kids', count: 337),
      ];
}

Future<ProviderContainer> _pump(WidgetTester tester, FakeRepo repo) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  final container = ProviderContainer(overrides: [
    channelRepositoryProvider.overrideWithValue(repo),
    sharedPreferencesProvider.overrideWithValue(prefs),
    facetRepositoryProvider.overrideWithValue(FakeFacetRepo()),
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
  testWidgets('los tres filtros se alinean en una línea y miden lo mismo',
      (tester) async {
    await _pump(tester, FakeRepo(total: 20));

    final pais = tester.getRect(find.byKey(const Key('filtro-pais')));
    final res = tester.getRect(find.byKey(const Key('filtro-resolucion')));
    final cat = tester.getRect(find.byKey(const Key('filtro-categoria')));

    expect(res.top, pais.top, reason: 'deben compartir línea');
    expect(cat.top, pais.top, reason: 'deben compartir línea');
    expect(res.width, closeTo(pais.width, 1));
    expect(cat.width, closeTo(pais.width, 1));
  });

  testWidgets('el botón de país muestra bandera y nombre completo',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));
    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(country: 'MX');
    await tester.pumpAndSettle();

    expect(find.textContaining('México'), findsOneWidget);
    expect(find.textContaining('🇲🇽'), findsOneWidget);
    // Nunca el código a secas.
    expect(find.text('MX'), findsNothing);
  });

  testWidgets('el selector de resolución abre y aplica', (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));

    await tester.tap(find.byKey(const Key('filtro-resolucion')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('4K'));
    await tester.pumpAndSettle();

    expect(container.read(channelFilterProvider).quality, '4k');
  });

  testWidgets('el selector de país busca por nombre y por código',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));

    await tester.tap(find.byKey(const Key('filtro-pais')));
    await tester.pumpAndSettle();
    // Por nombre.
    await tester.enterText(find.byType(TextField), 'Méx');
    await tester.pumpAndSettle();
    expect(find.text('México'), findsOneWidget);
    expect(find.text('España'), findsNothing);
    // Por código: quien sabe que México es MX no debería escribir el nombre.
    await tester.enterText(find.byType(TextField), 'mx');
    await tester.pumpAndSettle();
    expect(find.text('México'), findsOneWidget);

    await tester.tap(find.text('México'));
    await tester.pumpAndSettle();
    expect(container.read(channelFilterProvider).country, 'MX');
  });

  testWidgets('el selector de categoría muestra las atómicas con su recuento',
      (tester) async {
    await _pump(tester, FakeRepo(total: 20));

    await tester.tap(find.byKey(const Key('filtro-categoria')));
    await tester.pumpAndSettle();

    expect(find.text('Movies'), findsOneWidget);
    expect(find.text('655'), findsOneWidget);
  });

  testWidgets('limpiar solo aparece cuando hay algo que limpiar',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));
    expect(find.text('limpiar'), findsNothing);

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(country: 'MX', category: 'Kids');
    await tester.pumpAndSettle();
    expect(find.text('limpiar'), findsOneWidget);

    await tester.tap(find.text('limpiar'));
    await tester.pumpAndSettle();

    final f = container.read(channelFilterProvider);
    expect(f.country, isEmpty);
    expect(f.category, isEmpty);
    expect(f.quality, 'fhd');
  });

  testWidgets('el contador muestra el total del gateway, no lo cargado',
      (tester) async {
    await _pump(tester, FakeRepo(total: 1200));
    expect(find.textContaining('1200'), findsOneWidget);
  });

  testWidgets('el chip de favoritos activa y desactiva el filtro',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));
    expect(container.read(channelFilterProvider).onlyFavorites, isFalse);

    await tester.tap(find.byKey(const Key('chip-favoritos')));
    await tester.pumpAndSettle();
    expect(container.read(channelFilterProvider).onlyFavorites, isTrue);

    await tester.tap(find.byKey(const Key('chip-favoritos')));
    await tester.pumpAndSettle();
    expect(container.read(channelFilterProvider).onlyFavorites, isFalse);
  });

  testWidgets('los favoritos cuentan como filtro activo', (tester) async {
    // Si no contaran, "limpiar" no aparecería y el usuario se quedaría
    // encerrado en sus favoritos sin salida evidente.
    final container = await _pump(tester, FakeRepo(total: 20));
    expect(find.text('limpiar'), findsNothing);

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(onlyFavorites: true);
    await tester.pumpAndSettle();

    expect(find.text('limpiar'), findsOneWidget);
  });

  testWidgets('una búsqueda larga no desborda la barra', (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));

    container.read(channelFilterProvider.notifier).state = const ChannelFilter(
        query: 'un nombre de canal absurdamente largo que nadie escribiría '
            'pero que la barra tiene que aguantar sin romperse');
    await tester.pumpAndSettle();

    // Un desbordamiento de RenderFlex se reporta como excepción del framework.
    expect(tester.takeException(), isNull);
  });
}
