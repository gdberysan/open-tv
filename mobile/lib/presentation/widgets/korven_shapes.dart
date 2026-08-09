import 'package:flutter/material.dart';

/// Recorta un hexágono con la geometría del sistema:
/// polygon(50% 0, 100% 27%, 100% 73%, 50% 100%, 0 73%, 0 27%).
///
/// Compartido por el wordmark y por el marcador de los canales sin logo.
class HexClipper extends CustomClipper<Path> {
  const HexClipper();

  @override
  Path getClip(Size s) => Path()
    ..moveTo(s.width * .5, 0)
    ..lineTo(s.width, s.height * .27)
    ..lineTo(s.width, s.height * .73)
    ..lineTo(s.width * .5, s.height)
    ..lineTo(0, s.height * .73)
    ..lineTo(0, s.height * .27)
    ..close();

  @override
  bool shouldReclip(covariant CustomClipper<Path> old) => false;
}
