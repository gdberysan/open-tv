import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import 'korven_emblem.dart';

/// Vista de estado para vacío, error y carga. Los tres comparten tratamiento
/// —emblema, eyebrow de consola y mensaje— pero NO comparten texto: distinguir
/// "sin resultados con estos filtros" de "el gateway no responde" es la mitad
/// del valor. El estado vacío anterior siempre culpaba al sync.
class KorvenStateView extends StatelessWidget {
  const KorvenStateView({
    super.key,
    required this.eyebrow,
    required this.message,
    this.action,
    this.showEmblem = true,
  });

  final String eyebrow;
  final String message;
  final Widget? action;
  final bool showEmblem;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(KorvenSpacing.s6),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (showEmblem) ...[
              const Opacity(opacity: 0.5, child: KorvenEmblem(size: 72)),
              const SizedBox(height: KorvenSpacing.s5),
            ],
            Text(eyebrow,
                style: KorvenType.monoLabel
                    .copyWith(color: KorvenColors.textFaint)),
            const SizedBox(height: KorvenSpacing.s3),
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Text(message,
                  textAlign: TextAlign.center, style: KorvenType.body),
            ),
            if (action != null) ...[
              const SizedBox(height: KorvenSpacing.s5),
              action!,
            ],
          ],
        ),
      ),
    );
  }
}
