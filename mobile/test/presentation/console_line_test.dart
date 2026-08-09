import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/presentation/widgets/console_line.dart';

void main() {
  testWidgets('muestra el comando y un cursor', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: ConsoleLine(text: r'$ korven tune --channel "BBC One"'),
      ),
    ));

    expect(find.text(r'$ korven tune --channel "BBC One"'), findsOneWidget);
    expect(find.byKey(const Key('console-cursor')), findsOneWidget);

    // El cursor parpadea con un Timer: comprobar que no cuelga el test.
    await tester.pump(const Duration(seconds: 2));
  });

  testWidgets('sin parpadeo no arranca ningún timer', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: ConsoleLine(text: r'$ listo', blinking: false)),
    ));
    expect(find.byKey(const Key('console-cursor')), findsOneWidget);
  });
}
