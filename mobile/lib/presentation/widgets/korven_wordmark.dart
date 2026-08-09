import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_typography.dart';

/// Recorta un hexágono con la misma geometría que el clip-path del sistema:
/// polygon(50% 0, 100% 27%, 100% 73%, 50% 100%, 0 73%, 0 27%).
class _HexClipper extends CustomClipper<Path> {
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

/// Lockup de marca: KORVEN con la O como hexágono de acero con el nodo ámbar
/// al centro, más el sufijo en mono. Es la firma de Korven; no sustituir por
/// un texto plano.
class KorvenWordmark extends StatelessWidget {
  const KorvenWordmark({super.key, this.fontSize = 21, this.suffix = 'open tv'});

  final double fontSize;
  final String suffix;

  @override
  Widget build(BuildContext context) {
    final letra = TextStyle(
      fontFamily: KorvenType.familyDisplay,
      fontWeight: FontWeight.w700,
      fontSize: fontSize,
      height: 1,
      color: KorvenColors.textStrong,
    );
    final hex = fontSize * 0.86;
    final nodo = fontSize * 0.28;

    return Row(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Text('K', style: letra),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: fontSize * 0.04),
          child: SizedBox(
            width: hex,
            height: hex,
            child: Stack(
              alignment: Alignment.center,
              children: [
                ClipPath(
                  clipper: _HexClipper(),
                  child: Container(color: KorvenColors.graphite600),
                ),
                // El nodo: el punto focal de la marca.
                Container(
                  width: nodo,
                  height: nodo,
                  decoration: BoxDecoration(
                    color: KorvenColors.accent,
                    shape: BoxShape.circle,
                    boxShadow: [
                      BoxShadow(
                        color: KorvenColors.accent.withValues(alpha: 0.45),
                        blurRadius: nodo,
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
        Text('RVEN', style: letra),
        SizedBox(width: fontSize * 0.34),
        Text(
          suffix,
          style: KorvenType.monoLabel.copyWith(color: KorvenColors.textMuted),
        ),
      ],
    );
  }
}
