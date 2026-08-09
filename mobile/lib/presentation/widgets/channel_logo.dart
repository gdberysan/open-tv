import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_typography.dart';
import 'korven_shapes.dart';

/// Logo del canal, o su marcador si no lo tiene.
///
/// El 15,6% de los canales no trae logo — 1 de cada 6 celdas de la rejilla —
/// así que el marcador no es un detalle: es la inicial del canal sobre el
/// hexágono de la marca, no un icono genérico repetido mil veces.
class ChannelLogo extends StatelessWidget {
  const ChannelLogo({super.key, required this.url, required this.name, required this.size});

  final String url;
  final String name;
  final double size;

  @override
  Widget build(BuildContext context) {
    if (url.isEmpty) return _Marcador(name: name, size: size);

    return Image.network(
      url,
      fit: BoxFit.contain,
      // Sin cacheWidth cada logo se decodifica a resolución completa; con 12k
      // canales eso es memoria tirada. 2x por densidad de pantalla.
      cacheWidth: (size * 2).round(),
      cacheHeight: (size * 2).round(),
      errorBuilder: (_, __, ___) => _Marcador(name: name, size: size),
    );
  }
}

class _Marcador extends StatelessWidget {
  const _Marcador({required this.name, required this.size});

  final String name;
  final double size;

  @override
  Widget build(BuildContext context) {
    final inicial = name.trim().isEmpty ? '?' : name.trim()[0].toUpperCase();
    final lado = size * 0.72;

    return Center(
      child: SizedBox(
        width: lado,
        height: lado,
        child: Stack(
          alignment: Alignment.center,
          children: [
            ClipPath(
              clipper: const HexClipper(),
              child: Container(color: KorvenColors.surfaceRaised),
            ),
            Text(
              inicial,
              style: TextStyle(
                fontFamily: KorvenType.familyDisplay,
                fontWeight: FontWeight.w700,
                fontVariations: const [FontVariation('wght', 700)],
                fontSize: lado * 0.42,
                color: KorvenColors.textFaint,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
