import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:korven_open_tv/presentation/widgets/signal_bars.dart';

void main() {
  Future<void> pump(WidgetTester tester, bool? alive, int latencyMs) {
    return tester.pumpWidget(MaterialApp(
      home: Scaffold(body: SignalBars(alive: alive, latencyMs: latencyMs)),
    ));
  }

  testWidgets('verde <200ms', (tester) async {
    await pump(tester, true, 150);
    expect(find.bySemanticsLabel('Señal buena'), findsOneWidget);
  });

  testWidgets('naranja 200–800ms', (tester) async {
    await pump(tester, true, 500);
    expect(find.bySemanticsLabel('Señal media'), findsOneWidget);
  });

  testWidgets('rojo >800ms', (tester) async {
    await pump(tester, true, 1200);
    expect(find.bySemanticsLabel('Señal baja'), findsOneWidget);
  });

  testWidgets('gris muerto', (tester) async {
    await pump(tester, false, 0);
    expect(find.bySemanticsLabel('Sin señal'), findsOneWidget);
  });

  testWidgets('sin chequear', (tester) async {
    await pump(tester, null, 0);
    expect(find.bySemanticsLabel('Señal sin datos'), findsOneWidget);
  });
}
