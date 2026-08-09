import 'package:flutter/material.dart';

/// Tokens de color de Korven, portados literalmente del brand board
/// (design_handoff_korven_sitio/tokens/colors.css). Valores finales: no se
/// reinterpretan.
///
/// La regla del sistema: el canvas es grafito y acero, y el ámbar es el único
/// acento — la señal viva. En esta app significa canal vivo, filtro activo o
/// foco. Si algo no es una de esas tres cosas, no lleva ámbar.
abstract final class KorvenColors {
  // ── Núcleo de marca ──
  static const grafito = Color(0xFF0E131B);
  static const carbon = Color(0xFF171E29);
  static const ambar = Color(0xFFFF8A2B);
  static const acero = Color(0xFF97A3B2);
  static const hueso = Color(0xFFEFF3F8);

  // ── Rampa grafito ──
  static const graphite900 = Color(0xFF0A0E15);
  static const graphite850 = Color(0xFF0E131B);
  static const graphite800 = Color(0xFF121826);
  static const graphite750 = Color(0xFF171E29);
  static const graphite700 = Color(0xFF1D2533);
  static const graphite600 = Color(0xFF283142);
  static const graphite300 = Color(0xFF6B788C);
  static const graphite200 = Color(0xFF97A3B2);
  static const graphite100 = Color(0xFFC3CCD7);
  static const graphite050 = Color(0xFFEFF3F8);

  // ── Rampa ámbar ──
  static const amber600 = Color(0xFFE06E1C);
  static const amber500 = Color(0xFFFF8A2B);
  static const amber400 = Color(0xFFFFA255);

  /// rgba(255,138,43,.16) — el halo del nodo y el anillo de foco.
  static const amberGlow = Color(0x29FF8A2B);

  // ── Semánticos ──
  static const signalOk = Color(0xFF4FB286);
  static const signalError = Color(0xFFE5604D);
  static const signalInfo = Color(0xFF5E9BD6);

  // ── Alias semánticos: usar estos en los componentes ──
  static const surfaceBase = graphite850;
  static const surfaceSunken = graphite900;
  static const surfaceCard = graphite750;
  static const surfaceRaised = graphite700;
  static const surfaceInset = graphite800;

  static const textStrong = graphite050;
  static const textBody = graphite100;
  static const textMuted = graphite200;
  static const textFaint = graphite300;
  static const textOnAmber = graphite900;

  static const borderSubtle = Color(0x1F97A3B2); // .12
  static const borderDefault = Color(0x3397A3B2); // .20
  static const borderStrong = Color(0x5C97A3B2); // .36

  static const accent = amber500;
  static const accentHover = amber400;
  static const accentPress = amber600;

  static const codeBg = graphite900;
}
