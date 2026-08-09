import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/presentation/widgets/korven_wordmark.dart';

void main() {
  testWidgets('muestra el nombre de marca partido alrededor de la O hexagonal',
      (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: Center(child: KorvenWordmark())),
    ));

    // La "O" es un hexágono dibujado, así que el texto va en dos piezas.
    expect(find.text('K'), findsOneWidget);
    expect(find.text('RVEN'), findsOneWidget);
    expect(find.text('open tv'), findsOneWidget);
  });

  testWidgets('el sufijo es configurable', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: Center(child: KorvenWordmark(suffix: '.dev'))),
    ));
    expect(find.text('.dev'), findsOneWidget);
  });
}
