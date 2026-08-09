import 'package:flutter/material.dart';
import '../../domain/models/channel.dart';
import '../../theme/korven_colors.dart';

/// Indicador de señal de 3 barras (Fase 7.2): verde <200ms, naranja
/// 200–800ms, rojo >800ms, gris muerto, apagado sin chequear.
class SignalBars extends StatelessWidget {
  final bool? alive;
  final int latencyMs;

  const SignalBars({super.key, required this.alive, required this.latencyMs});

  @override
  Widget build(BuildContext context) {
    final level = signalLevel(alive, latencyMs);
    final (color, litBars, label) = switch (level) {
      // Semánticos de la marca, no los de Material: pino para lo que va bien,
      // ámbar para el aviso, óxido para el fallo.
      SignalLevel.good => (KorvenColors.signalOk, 3, 'Señal buena'),
      SignalLevel.medium => (KorvenColors.accent, 2, 'Señal media'),
      SignalLevel.poor => (KorvenColors.signalError, 1, 'Señal baja'),
      SignalLevel.dead => (KorvenColors.textFaint, 3, 'Sin señal'),
      SignalLevel.unknown => (KorvenColors.borderStrong, 0, 'Señal sin datos'),
    };
    final dim = color.withValues(alpha: 0.25);

    return Semantics(
      // container: nodo semántico propio aunque el ListTile fusione los hijos
      container: true,
      label: label,
      child: Tooltip(
        message: level == SignalLevel.good ||
                level == SignalLevel.medium ||
                level == SignalLevel.poor
            ? '$label (${latencyMs}ms)'
            : label,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            for (final (i, height) in const [(0, 6.0), (1, 10.0), (2, 14.0)])
              Padding(
                padding: const EdgeInsets.only(right: 2),
                child: Container(
                  width: 3,
                  height: height,
                  decoration: BoxDecoration(
                    color: i < litBars ? color : dim,
                    borderRadius: BorderRadius.circular(1),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
