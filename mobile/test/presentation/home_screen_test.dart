import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:iptv_ecosystem/presentation/providers/channel_provider.dart';
import 'package:iptv_ecosystem/presentation/screens/home_screen.dart';
import 'package:iptv_ecosystem/presentation/widgets/signal_bars.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

void main() {
  Future<void> pumpHome(WidgetTester tester, FakeRepo repo) async {
    SharedPreferences.setMockInitialValues({});
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
    await pumpHome(tester, repo);

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
    await tester.dragUntilVisible(
      find.text('Reintentar'),
      find.byType(ListView),
      const Offset(0, -600),
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
}
