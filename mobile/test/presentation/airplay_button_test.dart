import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/data/airplay/airplay_platform.dart';
import 'package:korven_open_tv/presentation/widgets/airplay_button.dart';

class _PlataformaEspia implements AirplayPlatform {
  int aperturas = 0;
  double? ultimaX;
  double? ultimaY;
  bool devuelve = true;

  @override
  Stream<AirplayEvent> get events => const Stream<AirplayEvent>.empty();

  @override
  Future<void> start({required String url, required String title}) async {}

  @override
  Future<void> stop() async {}

  @override
  Future<bool> showRoutePicker({
    required double x,
    required double y,
    required double lado,
  }) async {
    aperturas++;
    ultimaX = x;
    ultimaY = y;
    return devuelve;
  }
}

void main() {
  testWidgets('fuera de macOS no ocupa sitio', (tester) async {
    // El binding de test no declara macOS, que es exactamente el caso que debe
    // colapsar. Sin ese colapso, CI —que corre en ubuntu-latest— intentaría
    // hablar por un MethodChannel que allí no existe.
    //
    // No se toca debugDefaultTargetPlatformOverride: el framework verifica que
    // las variables de depuración queden sin tocar al acabar el cuerpo del
    // test, y esa comprobación corre ANTES que addTearDown.
    final espia = _PlataformaEspia();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [airplayPlatformProvider.overrideWithValue(espia)],
        child: const MaterialApp(home: Scaffold(body: AirplayButton())),
      ),
    );

    expect(find.byType(AirplayButton), findsOneWidget);
    final caja = tester.getSize(find.byType(AirplayButton));
    expect(caja.width, 0);
    expect(caja.height, 0);
    expect(espia.aperturas, 0);
  });
}
