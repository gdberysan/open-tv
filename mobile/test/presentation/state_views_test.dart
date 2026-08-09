import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/presentation/widgets/korven_emblem.dart';
import 'package:korven_open_tv/presentation/widgets/state_views.dart';

void main() {
  testWidgets('muestra eyebrow, mensaje y emblema', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: KorvenStateView(
          eyebrow: '// sin resultados',
          message: 'Ningún canal casa con estos filtros.',
        ),
      ),
    ));

    expect(find.text('// sin resultados'), findsOneWidget);
    expect(find.text('Ningún canal casa con estos filtros.'), findsOneWidget);
    expect(find.byType(KorvenEmblem), findsOneWidget);
  });

  testWidgets('la acción es opcional', (tester) async {
    var pulsado = false;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: KorvenStateView(
          eyebrow: '// error',
          message: 'No se pudo contactar con el gateway.',
          action: ElevatedButton(
            onPressed: () => pulsado = true,
            child: const Text('Reintentar'),
          ),
        ),
      ),
    ));

    await tester.tap(find.text('Reintentar'));
    expect(pulsado, isTrue);
  });
}
