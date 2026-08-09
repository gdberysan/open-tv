import 'package:flutter/animation.dart';

/// Motion del sistema: calmado y preciso, nunca rebote de juguete.
abstract final class KorvenMotion {
  /// cubic-bezier(.16, 1, .3, 1) — el "settle" preciso de la marca.
  static const easeOut = Cubic(0.16, 1, 0.3, 1);
  static const easeInOut = Cubic(0.65, 0, 0.35, 1);

  static const fast = Duration(milliseconds: 120);
  static const base = Duration(milliseconds: 200);
  static const slow = Duration(milliseconds: 360);
}
