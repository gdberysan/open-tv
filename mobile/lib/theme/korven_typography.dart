import 'package:flutter/material.dart';
import 'korven_colors.dart';

/// Tipografía de Korven (tokens/typography.css): Space Grotesk para display,
/// Inter para lectura, JetBrains Mono para lo que es literalmente dato.
abstract final class KorvenType {
  static const familyDisplay = 'SpaceGrotesk';
  static const familyText = 'Inter';
  static const familyMono = 'JetBrainsMono';

  // Escala de tercera mayor 1.250, base 16.
  static const size2xs = 11.0;
  static const sizeXs = 12.0;
  static const sizeSm = 14.0;
  static const sizeBase = 16.0;
  static const sizeMd = 18.0;
  static const sizeLg = 20.0;
  static const sizeXl = 25.0;
  static const size2xl = 31.0;
  static const size3xl = 39.0;

  static const h1 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w600,
    fontSize: size3xl,
    height: 1.05,
    letterSpacing: -0.02 * size3xl,
    color: KorvenColors.textStrong,
  );

  static const h2 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w600,
    fontSize: size2xl,
    height: 1.2,
    letterSpacing: -0.01 * size2xl,
    color: KorvenColors.textStrong,
  );

  static const h3 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w500,
    fontSize: sizeXl,
    height: 1.2,
    color: KorvenColors.textStrong,
  );

  static const h4 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w500,
    fontSize: sizeMd,
    height: 1.2,
    color: KorvenColors.textStrong,
  );

  static const body = TextStyle(
    fontFamily: familyText,
    fontWeight: FontWeight.w400,
    fontSize: sizeBase,
    height: 1.5,
    color: KorvenColors.textBody,
  );

  static const bodySm = TextStyle(
    fontFamily: familyText,
    fontWeight: FontWeight.w400,
    fontSize: sizeSm,
    height: 1.5,
    color: KorvenColors.textBody,
  );

  static const label = TextStyle(
    fontFamily: familyText,
    fontWeight: FontWeight.w500,
    fontSize: sizeSm,
    height: 1.2,
    color: KorvenColors.textMuted,
  );

  static const mono = TextStyle(
    fontFamily: familyMono,
    fontWeight: FontWeight.w400,
    fontSize: sizeSm,
    height: 1.5,
    letterSpacing: 0.02 * sizeSm,
    color: KorvenColors.textMuted,
  );

  /// Eyebrows y etiquetas de consola: mono pequeño con tracking ancho.
  static const monoLabel = TextStyle(
    fontFamily: familyMono,
    fontWeight: FontWeight.w500,
    fontSize: sizeXs,
    height: 1.2,
    letterSpacing: 0.06 * sizeXs,
    color: KorvenColors.textMuted,
  );
}
