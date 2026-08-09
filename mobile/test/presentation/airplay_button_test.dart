import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/presentation/widgets/airplay_button.dart';

void main() {
  testWidgets('fuera de macOS no ocupa sitio', (tester) async {
    // El binding de test no declara macOS, que es exactamente el caso que debe
    // colapsar. Sin ese colapso, CI —que corre en ubuntu-latest— reventaría al
    // intentar crear un AppKitView.
    //
    // No se toca debugDefaultTargetPlatformOverride: el framework verifica que
    // las variables de depuración queden sin tocar al acabar el cuerpo del
    // test, y esa comprobación corre ANTES que addTearDown.
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: AirplayButton())),
    );
    expect(find.byType(AirplayButton), findsOneWidget);
    final caja = tester.getSize(find.byType(AirplayButton));
    expect(caja.width, 0);
    expect(caja.height, 0);
  });
}
