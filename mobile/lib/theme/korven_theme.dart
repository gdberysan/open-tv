import 'package:flutter/material.dart';
import 'korven_colors.dart';
import 'korven_spacing.dart';
import 'korven_typography.dart';

/// ThemeData de Korven Open TV. Solo oscuro: la marca es un canvas de grafito
/// y un reproductor de vídeo se mira a oscuras.
///
/// El acento ámbar se reserva para la señal viva — canal vivo, filtro activo,
/// foco. Es la regla que le faltaba al tema anterior: un seed deepPurple no
/// significa nada, así que Material lo repartía sin criterio.
ThemeData korvenTheme() {
  const scheme = ColorScheme.dark(
    primary: KorvenColors.accent,
    onPrimary: KorvenColors.textOnAmber,
    secondary: KorvenColors.accent,
    onSecondary: KorvenColors.textOnAmber,
    surface: KorvenColors.surfaceBase,
    onSurface: KorvenColors.textBody,
    error: KorvenColors.signalError,
    onError: KorvenColors.textStrong,
    outline: KorvenColors.borderDefault,
  );

  return ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    colorScheme: scheme,
    scaffoldBackgroundColor: KorvenColors.surfaceBase,
    canvasColor: KorvenColors.surfaceBase,
    dividerColor: KorvenColors.borderSubtle,
    fontFamily: KorvenType.familyText,
    textTheme: const TextTheme(
      headlineLarge: KorvenType.h2,
      headlineMedium: KorvenType.h3,
      titleLarge: KorvenType.h4,
      bodyLarge: KorvenType.body,
      bodyMedium: KorvenType.bodySm,
      labelLarge: KorvenType.label,
    ),
    appBarTheme: const AppBarTheme(
      backgroundColor: KorvenColors.surfaceSunken,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      foregroundColor: KorvenColors.textStrong,
    ),
    iconTheme: const IconThemeData(color: KorvenColors.textMuted),
    dialogTheme: DialogThemeData(
      backgroundColor: KorvenColors.surfaceCard,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(KorvenRadius.lg),
        side: const BorderSide(color: KorvenColors.borderDefault),
      ),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: KorvenColors.accent,
        foregroundColor: KorvenColors.textOnAmber,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(KorvenRadius.md),
        ),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(foregroundColor: KorvenColors.accent),
    ),
    progressIndicatorTheme:
        const ProgressIndicatorThemeData(color: KorvenColors.accent),
  );
}
