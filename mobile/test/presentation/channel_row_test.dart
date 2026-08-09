import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/domain/models/channel.dart';
import 'package:iptv_ecosystem/presentation/widgets/channel_row.dart';

Channel _canal({String pais = 'MX', String cat = 'News', bool? vivo = true}) =>
    Channel(
      id: 'ch-1',
      name: 'Canal Uno (1080p)',
      logoUrl: '',
      categoryId: cat,
      languageCode: 'es',
      countryCode: pais,
      providerType: 'opensource',
      alive: vivo,
      latencyMs: 120,
    );

void main() {
  testWidgets('muestra nombre y línea de metadatos', (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(body: ChannelRow(channel: _canal(), onTap: () {})),
    ));

    expect(find.text('Canal Uno (1080p)'), findsOneWidget);
    expect(find.text('MX · News'), findsOneWidget);
  });

  testWidgets('omite los metadatos vacíos sin dejar separadores sueltos',
      (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: ChannelRow(channel: _canal(pais: '', cat: ''), onTap: () {}),
      ),
    ));

    expect(find.textContaining('·'), findsNothing);
  });

  testWidgets('con un solo metadato no pinta separador', (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: ChannelRow(channel: _canal(cat: ''), onTap: () {}),
      ),
    ));

    expect(find.text('MX'), findsOneWidget);
  });

  testWidgets('el logo se decodifica al tamaño del hueco, no a resolución completa',
      (tester) async {
    const c = Channel(
      id: 'ch-1',
      name: 'X',
      logoUrl: 'http://x/logo.png',
      categoryId: '',
      languageCode: '',
      countryCode: '',
      providerType: 'opensource',
    );
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: ChannelRow(channel: c, onTap: null)),
    ));

    // Sin cacheWidth cada logo se decodifica completo para un hueco de 40px, y
    // con 12k canales eso es memoria tirada.
    final img = tester.widget<Image>(find.byType(Image));
    expect(img.image, isA<ResizeImage>());
  });
}
