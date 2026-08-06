import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:iptv_ecosystem/presentation/providers/channel_provider.dart';
import 'package:iptv_ecosystem/presentation/screens/home_screen.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

void main() {
  Future<void> pumpHome(WidgetTester tester, FakeRepo repo) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [channelRepositoryProvider.overrideWithValue(repo)],
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
}
