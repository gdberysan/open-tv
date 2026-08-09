import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/domain/models/channel.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:korven_open_tv/presentation/widgets/channel_card.dart';

Channel _canal({
  String name = 'BBC One (1080p)',
  String logo = '',
  String pais = 'GB',
  String cat = 'General',
}) =>
    Channel(
      id: 'ch-1',
      name: name,
      logoUrl: logo,
      categoryId: cat,
      languageCode: 'en',
      countryCode: pais,
      providerType: 'opensource',
      alive: true,
      latencyMs: 90,
    );

Future<void> _pump(WidgetTester tester, Widget hijo) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  await tester.pumpWidget(ProviderScope(
    overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
    child: MaterialApp(
      home: Scaffold(body: Center(child: SizedBox(width: 180, height: 220, child: hijo))),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('la tarjeta pinta la inicial cuando no hay logo', (tester) async {
    // 1 de cada 6 canales no tiene logo: el marcador no es un detalle.
    await _pump(tester, ChannelCard(channel: _canal(), onTap: () {}));

    expect(find.text('B'), findsOneWidget); // inicial de "BBC One"
    expect(find.byType(Image), findsNothing);
  });

  testWidgets('con logo no pinta la inicial', (tester) async {
    await _pump(
        tester,
        ChannelCard(
            channel: _canal(logo: 'http://x/bbc.png'), onTap: () {}));

    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('la línea de specs junta país, categoría y resolución',
      (tester) async {
    await _pump(tester, ChannelCard(channel: _canal(), onTap: () {}));

    expect(find.text('GB · General · 1080p'), findsOneWidget);
  });

  testWidgets('sin metadatos no deja separadores huérfanos', (tester) async {
    await _pump(
        tester,
        ChannelCard(
            channel: _canal(name: 'Canal', pais: '', cat: ''), onTap: () {}));

    expect(find.textContaining('·'), findsNothing);
  });

  testWidgets('pulsar la tarjeta avisa', (tester) async {
    var pulsado = false;
    await _pump(
        tester, ChannelCard(channel: _canal(), onTap: () => pulsado = true));

    await tester.tap(find.byType(ChannelCard));
    expect(pulsado, isTrue);
  });

  test('la resolución sale del nombre y solo si está entre paréntesis', () {
    expect(resolucionDelNombre('BBC One (1080p)'), '1080p');
    expect(resolucionDelNombre('Canal 4K Deportes (4K)'), '4K');
    // Sin paréntesis es parte del nombre, no una especificación.
    expect(resolucionDelNombre('Canal 1080p Deportes'), isNull);
    expect(resolucionDelNombre('Canal Uno'), isNull);
  });
}
