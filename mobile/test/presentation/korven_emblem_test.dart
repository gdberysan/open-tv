import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/presentation/widgets/korven_emblem.dart';

void main() {
  testWidgets('se dibuja en el tamaño pedido y es accesible', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: Center(child: KorvenEmblem(size: 96))),
    ));

    final box = tester.getSize(find.byType(KorvenEmblem));
    expect(box.width, 96);
    expect(box.height, 96);
    // El emblema es la marca; para un lector de pantalla tiene que anunciarse.
    expect(find.bySemanticsLabel('Emblema de Korven'), findsOneWidget);
  });
}
