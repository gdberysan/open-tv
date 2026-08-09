import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/domain/models/channel.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:korven_open_tv/presentation/widgets/channel_row.dart';

Channel _canal({String pais = 'MX', String cat = 'News', String logo = ''}) =>
    Channel(
      id: 'ch-1',
      name: 'Canal Uno (1080p)',
      logoUrl: logo,
      categoryId: cat,
      languageCode: 'es',
      countryCode: pais,
      providerType: 'opensource',
      alive: true,
      latencyMs: 120,
    );

/// La fila lleva estrella de favorito, que lee SharedPreferences vía Riverpod.
Future<void> _pump(WidgetTester tester, Widget hijo) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  await tester.pumpWidget(ProviderScope(
    overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
    child: MaterialApp(home: Scaffold(body: hijo)),
  ));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('muestra nombre y línea de metadatos', (tester) async {
    await _pump(tester, ChannelRow(channel: _canal(), onTap: () {}));

    expect(find.text('Canal Uno (1080p)'), findsOneWidget);
    expect(find.text('MX · News'), findsOneWidget);
  });

  testWidgets('omite los metadatos vacíos sin dejar separadores sueltos',
      (tester) async {
    await _pump(
        tester, ChannelRow(channel: _canal(pais: '', cat: ''), onTap: () {}));

    expect(find.textContaining('·'), findsNothing);
  });

  testWidgets('con un solo metadato no pinta separador', (tester) async {
    await _pump(tester, ChannelRow(channel: _canal(cat: ''), onTap: () {}));

    expect(find.text('MX'), findsOneWidget);
  });

  testWidgets('sin logo pinta la inicial del canal', (tester) async {
    // 1 de cada 6 canales no tiene logo: el marcador no es un detalle.
    await _pump(tester, ChannelRow(channel: _canal(), onTap: () {}));

    expect(find.text('C'), findsOneWidget); // inicial de "Canal Uno"
    expect(find.byType(Image), findsNothing);
  });

  testWidgets('el logo se decodifica al tamaño del hueco', (tester) async {
    await _pump(
        tester,
        ChannelRow(
            channel: _canal(logo: 'http://x/logo.png'), onTap: () {}));

    // Sin cacheWidth cada logo se decodifica completo para un hueco de 40px.
    final img = tester.widget<Image>(find.byType(Image));
    expect(img.image, isA<ResizeImage>());
  });

  testWidgets('la estrella marca y desmarca el favorito', (tester) async {
    await _pump(tester, ChannelRow(channel: _canal(), onTap: () {}));

    expect(find.byIcon(Icons.star_border), findsOneWidget);
    await tester.tap(find.byIcon(Icons.star_border));
    await tester.pumpAndSettle();
    expect(find.byIcon(Icons.star), findsOneWidget);
  });
}
