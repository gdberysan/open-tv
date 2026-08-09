import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/domain/models/channel_filter.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:korven_open_tv/presentation/screens/home_screen.dart';
import 'package:korven_open_tv/presentation/widgets/channel_card.dart';
import 'package:korven_open_tv/presentation/widgets/channel_row.dart';
import 'package:korven_open_tv/presentation/widgets/signal_bars.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

void main() {
  // La rejilla es la vista por defecto; los tests que van sobre la lista la
  // piden explícitamente por la preferencia persistida.
  Future<void> pumpHome(WidgetTester tester, FakeRepo repo,
      {String vista = 'grid'}) async {
    SharedPreferences.setMockInitialValues({'view_mode': vista});
    final prefs = await SharedPreferences.getInstance();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          channelRepositoryProvider.overrideWithValue(repo),
          sharedPreferencesProvider.overrideWithValue(prefs),
        ],
        child: const MaterialApp(home: HomeScreen()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('muestra los canales que devuelve el gateway', (tester) async {
    await pumpHome(tester, FakeRepo(total: 10));

    expect(find.text('Canal Par 0'), findsOneWidget);
    expect(find.text('Canal Impar 1'), findsOneWidget);
  });

  testWidgets('cada canal muestra su indicador de señal', (tester) async {
    await pumpHome(tester, FakeRepo(total: 6));

    // ch-0 vivo con 100ms → buena; ch-1 muerto; ch-2 sin chequear
    final bars = tester.widgetList<SignalBars>(find.byType(SignalBars)).toList();
    expect(bars, hasLength(6));
    expect(bars.any((b) => b.alive == true && b.latencyMs > 0), isTrue);
    expect(bars.any((b) => b.alive == false), isTrue);
    expect(bars.any((b) => b.alive == null), isTrue);
  });

  testWidgets('el toggle de offline pide alive=all y persiste', (tester) async {
    final repo = FakeRepo(total: 6);
    await pumpHome(tester, repo);
    expect(repo.lastFilter?.showOffline, isFalse);

    await tester.tap(find.byTooltip('Mostrar canales offline'));
    await tester.pumpAndSettle();

    expect(repo.lastFilter?.showOffline, isTrue);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getBool('show_offline'), isTrue);
  });

  testWidgets('la búsqueda filtra en el servidor, no sobre lo ya cargado',
      (tester) async {
    // 600 canales: los "Impar" de la cola (ch-501+) NO están en la primera
    // página de 500; solo aparecen si la búsqueda viaja al servidor.
    final repo = FakeRepo(total: 600);
    await pumpHome(tester, repo);

    await tester.tap(find.byIcon(Icons.search));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'Impar 599');
    // Esperar el debounce de la búsqueda
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pumpAndSettle();

    expect(find.text('Canal Impar 599'), findsOneWidget);
  });

  testWidgets('cerrar la búsqueda restaura la lista completa', (tester) async {
    final repo = FakeRepo(total: 10);
    await pumpHome(tester, repo);

    await tester.tap(find.byIcon(Icons.search));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'Impar');
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pumpAndSettle();
    expect(find.text('Canal Par 0'), findsNothing);

    await tester.tap(find.byIcon(Icons.close).first);
    await tester.pumpAndSettle();

    expect(find.text('Canal Par 0'), findsOneWidget);
  });

  testWidgets('una página fallida muestra fila de error y deja reintentar',
      (tester) async {
    final repo = FakeRepo(total: 1200);
    await pumpHome(tester, repo, vista: 'list');

    final container =
        ProviderScope.containerOf(tester.element(find.byType(HomeScreen)));

    // Provocar el fallo de la siguiente página.
    repo.failNext = true;
    await container.read(channelListProvider.notifier).loadMore();
    await tester.pumpAndSettle();

    expect(container.read(channelListProvider).requireValue.loadMoreError,
        isNotNull,
        reason: 'el notifier debe haber registrado el error');

    // Bajar hasta el final, donde vive la fila de error.
    // Paso grande y margen de iteraciones: la fila de error vive al final de
    // 500 filas, y depender de la altura exacta de cada una hace el test
    // frágil ante cualquier cambio de diseño.
    await tester.dragUntilVisible(
      find.text('Reintentar'),
      find.byType(ListView),
      const Offset(0, -3000),
      maxIteration: 200,
    );
    await tester.pumpAndSettle();

    expect(find.text('Reintentar'), findsOneWidget,
        reason: 'el fallo de página debe ser visible, no un spinner eterno');

    await tester.tap(find.text('Reintentar'));
    await tester.pumpAndSettle();

    expect(container.read(channelListProvider).requireValue.loadMoreError,
        isNull,
        reason: 'reintentar con éxito debe limpiar el error');
  });

  testWidgets('la barra muestra el lockup de marca', (tester) async {
    await pumpHome(tester, FakeRepo(total: 4));

    expect(find.text('RVEN'), findsOneWidget);
    expect(find.text('open tv'), findsOneWidget);
    // El título de plantilla ya no está.
    expect(find.text('IPTV'), findsNothing);
  });

  testWidgets('el toggle de offline se pinta ámbar solo cuando está activo',
      (tester) async {
    await pumpHome(tester, FakeRepo(total: 4));

    Color colorDelIcono(String tooltip) {
      final icon = tester.widget<Icon>(
        find.descendant(
            of: find.byTooltip(tooltip), matching: find.byType(Icon)),
      );
      return icon.color!;
    }

    final apagado = colorDelIcono('Mostrar canales offline');
    await tester.tap(find.byTooltip('Mostrar canales offline'));
    await tester.pumpAndSettle();
    final encendido = colorDelIcono('Ocultar canales offline');

    expect(apagado, isNot(encendido),
        reason: 'el estado activo tiene que distinguirse, y en ámbar');
  });

  testWidgets('con filtros activos y cero resultados culpa a los filtros, no al sync',
      (tester) async {
    await pumpHome(tester, FakeRepo(total: 0));

    final container =
        ProviderScope.containerOf(tester.element(find.byType(HomeScreen)));
    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(country: 'ZZ');
    await tester.pumpAndSettle();

    expect(find.text('// sin resultados'), findsOneWidget);
    expect(find.textContaining('filtros'), findsWidgets);
    // El mensaje viejo culpaba siempre al sync, incluso cuando el problema era
    // que los filtros no casaban.
    expect(find.textContaining('sincronizado'), findsNothing);
  });

  testWidgets('sin filtros y cero resultados sí culpa al catálogo',
      (tester) async {
    await pumpHome(tester, FakeRepo(total: 0));

    expect(find.text('// catálogo vacío'), findsOneWidget);
    expect(find.textContaining('sincronizado'), findsOneWidget);
  });

  testWidgets('el toggle de vista alterna entre rejilla y lista',
      (tester) async {
    await pumpHome(tester, FakeRepo(total: 8));
    expect(find.byType(ChannelCard), findsWidgets);
    expect(find.byType(ChannelRow), findsNothing);

    await tester.tap(find.byKey(const Key('accion-vista')));
    await tester.pumpAndSettle();

    expect(find.byType(ChannelRow), findsWidgets);
    expect(find.byType(ChannelCard), findsNothing);

    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('view_mode'), 'list');
  });

  testWidgets('si el aleatorio falla lo dice en vez de no hacer nada',
      (tester) async {
    final repo = FakeRepo(total: 20)..failRandom = true;
    await pumpHome(tester, repo);

    await tester.tap(find.byKey(const Key('accion-aleatorio')));
    await tester.pump();
    await tester.pump();

    expect(find.byType(SnackBar), findsOneWidget);
  });
}
