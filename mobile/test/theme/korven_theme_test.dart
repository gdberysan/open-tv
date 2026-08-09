import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/theme/korven_colors.dart';
import 'package:korven_open_tv/theme/korven_theme.dart';

void main() {
  test('el tema es oscuro y su canvas es grafito', () {
    final t = korvenTheme();
    expect(t.brightness, Brightness.dark);
    expect(t.scaffoldBackgroundColor, KorvenColors.surfaceBase);
  });

  test('el acento es ámbar, no el morado de la plantilla', () {
    final t = korvenTheme();
    expect(t.colorScheme.primary, KorvenColors.accent);
    // Regresión concreta: el seed deepPurple de la plantilla de Flutter.
    expect(t.colorScheme.primary, isNot(Colors.deepPurple));
  });

  test('el texto sobre ámbar es grafito, para que se lea', () {
    expect(korvenTheme().colorScheme.onPrimary, KorvenColors.textOnAmber);
  });

  test('el error usa el óxido de la marca, no el rojo de Material', () {
    expect(korvenTheme().colorScheme.error, KorvenColors.signalError);
  });
}
