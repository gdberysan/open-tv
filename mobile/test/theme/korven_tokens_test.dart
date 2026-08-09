import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/theme/korven_colors.dart';
import 'package:korven_open_tv/theme/korven_spacing.dart';

void main() {
  // Los valores vienen del brand board de Korven (tokens/colors.css). Este test
  // no prueba lógica: fija la identidad de marca para que un dedazo en un hex
  // no pase desapercibido.
  test('los colores del núcleo de marca son los del brand board', () {
    expect(KorvenColors.grafito, const Color(0xFF0E131B));
    expect(KorvenColors.carbon, const Color(0xFF171E29));
    expect(KorvenColors.ambar, const Color(0xFFFF8A2B));
    expect(KorvenColors.acero, const Color(0xFF97A3B2));
    expect(KorvenColors.hueso, const Color(0xFFEFF3F8));
  });

  test('las superficies y textos semánticos apuntan a la rampa correcta', () {
    expect(KorvenColors.surfaceBase, const Color(0xFF0E131B));
    expect(KorvenColors.surfaceSunken, const Color(0xFF0A0E15));
    expect(KorvenColors.surfaceCard, const Color(0xFF171E29));
    expect(KorvenColors.surfaceInset, const Color(0xFF121826));
    expect(KorvenColors.textStrong, const Color(0xFFEFF3F8));
    expect(KorvenColors.textBody, const Color(0xFFC3CCD7));
    expect(KorvenColors.textMuted, const Color(0xFF97A3B2));
    expect(KorvenColors.textFaint, const Color(0xFF6B788C));
  });

  test('el acento tiene sus estados y su glow', () {
    expect(KorvenColors.accent, const Color(0xFFFF8A2B));
    expect(KorvenColors.accentHover, const Color(0xFFFFA255));
    expect(KorvenColors.accentPress, const Color(0xFFE06E1C));
    expect(KorvenColors.amberGlow.a, closeTo(0.16, 0.01));
  });

  test('los hairlines mantienen las tres opacidades del sistema', () {
    expect(KorvenColors.borderSubtle.a, closeTo(0.12, 0.01));
    expect(KorvenColors.borderDefault.a, closeTo(0.20, 0.01));
    expect(KorvenColors.borderStrong.a, closeTo(0.36, 0.01));
  });

  test('el espaciado sigue la escala de 8px', () {
    expect(KorvenSpacing.s1, 4.0);
    expect(KorvenSpacing.s2, 8.0);
    expect(KorvenSpacing.s4, 16.0);
    expect(KorvenSpacing.s5, 24.0);
    expect(KorvenSpacing.s7, 48.0);
  });
}
