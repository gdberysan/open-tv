import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/domain/models/channel.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:korven_open_tv/presentation/widgets/channel_card.dart';
import 'package:korven_open_tv/presentation/widgets/channel_grid.dart';

List<Channel> _canales(int n) => List.generate(
      n,
      (i) => Channel(
        id: 'ch-$i',
        name: 'Canal $i',
        logoUrl: '',
        categoryId: 'News',
        languageCode: 'es',
        countryCode: 'MX',
        providerType: 'opensource',
        alive: true,
        latencyMs: 100,
      ),
    );

Future<void> _pumpGrid(
  WidgetTester tester, {
  int total = 12,
  bool cargando = false,
  String? error,
  VoidCallback? onRetry,
  void Function(Channel)? onSelect,
}) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  await tester.pumpWidget(ProviderScope(
    overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
    child: MaterialApp(
      home: Scaffold(
        body: ChannelGrid(
          channels: _canales(total),
          showTailLoader: cargando,
          loadMoreError: error,
          onRetry: onRetry,
          controller: ScrollController(),
          onSelect: onSelect ?? (_) {},
        ),
      ),
    ),
  ));
  // El spinner de cola anima sin fin, así que con él en pantalla pumpAndSettle
  // nunca vuelve.
  if (cargando) {
    await tester.pump();
  } else {
    await tester.pumpAndSettle();
  }
}

void main() {
  testWidgets('pinta una tarjeta por canal', (tester) async {
    await _pumpGrid(tester, total: 6);

    expect(find.byType(ChannelCard), findsNWidgets(6));
    expect(find.text('Canal 0'), findsOneWidget);
  });

  testWidgets('el número de columnas se adapta al ancho', (tester) async {
    // Las tarjetas de la primera fila comparten su borde superior, así que
    // contarlas da el número de columnas. Con una rejilla de columnas fijas
    // este número no cambiaría al estrechar la ventana, que es justo el
    // desperdicio de espacio que veníamos a arreglar.
    int columnas() {
      final tops = tester
          .widgetList<ChannelCard>(find.byType(ChannelCard))
          .map((w) => tester.getTopLeft(find.byWidget(w)).dy);
      final primera = tops.reduce((a, b) => a < b ? a : b);
      return tops.where((t) => t == primera).length;
    }

    addTearDown(tester.view.reset);
    tester.view.devicePixelRatio = 1;

    tester.view.physicalSize = const Size(1400, 1000);
    await _pumpGrid(tester, total: 24);
    final anchas = columnas();

    tester.view.physicalSize = const Size(500, 1000);
    await tester.pumpAndSettle();
    final estrechas = columnas();

    expect(anchas, greaterThan(estrechas),
        reason: 'una ventana ancha tiene que caber más columnas');
    expect(tester.getSize(find.byType(ChannelCard).first).width,
        lessThanOrEqualTo(190),
        reason: 'maxCrossAxisExtent acota el ancho de la tarjeta');
  });

  testWidgets('el spinner de cola solo aparece cargando', (tester) async {
    await _pumpGrid(tester, total: 4);
    expect(find.byType(CircularProgressIndicator), findsNothing);

    await _pumpGrid(tester, total: 4, cargando: true);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });

  testWidgets('un error de página se ve en la rejilla y deja reintentar',
      (tester) async {
    var reintentos = 0;
    await _pumpGrid(tester,
        total: 4, error: 'El gateway no responde', onRetry: () => reintentos++);

    expect(find.text('El gateway no responde'), findsOneWidget);
    // Fila de error, no spinner eterno.
    expect(find.byType(CircularProgressIndicator), findsNothing);

    await tester.tap(find.text('Reintentar'));
    expect(reintentos, 1);
  });

  testWidgets('seleccionar una tarjeta devuelve su canal', (tester) async {
    Channel? elegido;
    await _pumpGrid(tester, total: 6, onSelect: (c) => elegido = c);

    await tester.tap(find.text('Canal 2'));
    expect(elegido?.id, 'ch-2');
  });
}
