import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';

/// Emblema hexagonal facetado de Korven, portado de assets/korven-emblem.svg
/// (viewBox 128×128). Se dibuja en vez de cargarse como SVG: son cuatro trazos
/// y evita una dependencia entera.
///
/// El ojo se queda quieto. En el sitio sigue al cursor, que es encantador en
/// una one-page y molesto en una herramienta.
class _EmblemPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final k = size.width / 128.0;
    Offset p(double x, double y) => Offset(x * k, y * k);

    final hex = Path()
      ..moveTo(64 * k, 12 * k)
      ..lineTo(110 * k, 38 * k)
      ..lineTo(110 * k, 90 * k)
      ..lineTo(64 * k, 116 * k)
      ..lineTo(18 * k, 90 * k)
      ..lineTo(18 * k, 38 * k)
      ..close();

    canvas.drawPath(hex, Paint()..color = KorvenColors.carbon);
    canvas.drawPath(
      hex,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3 * k
        ..strokeJoin = StrokeJoin.round
        ..color = KorvenColors.hueso,
    );

    // Facetas desde el centro: le dan el volumen tallado.
    final faceta = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2 * k
      ..strokeJoin = StrokeJoin.round
      ..color = KorvenColors.graphite600;
    for (final destino in const [
      Offset(64, 12),
      Offset(110, 38),
      Offset(110, 90),
      Offset(18, 90),
      Offset(18, 38),
    ]) {
      canvas.drawLine(p(64, 64), p(destino.dx, destino.dy), faceta);
    }

    // El ojo: halo, anillo y núcleo — el único ámbar del emblema.
    canvas.drawCircle(p(64, 64), 22 * k,
        Paint()..color = KorvenColors.accent.withValues(alpha: 0.16));
    canvas.drawCircle(
      p(64, 64),
      15 * k,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3 * k
        ..color = KorvenColors.accent.withValues(alpha: 0.30),
    );
    canvas.drawCircle(p(64, 64), 9 * k, Paint()..color = KorvenColors.accent);
  }

  @override
  bool shouldRepaint(covariant CustomPainter old) => false;
}

class KorvenEmblem extends StatelessWidget {
  const KorvenEmblem({super.key, this.size = 128});

  final double size;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: 'Emblema de Korven',
      image: true,
      child: SizedBox(
        width: size,
        height: size,
        child: CustomPaint(painter: _EmblemPainter()),
      ),
    );
  }
}
