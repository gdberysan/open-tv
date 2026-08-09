import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_motion.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';

/// Chip del sistema Korven: mono, borde hairline, ámbar cuando está activo.
/// Con [onRemove] se convierte en faceta removible (lleva una × al final).
class KorvenChip extends StatefulWidget {
  const KorvenChip({
    super.key,
    required this.label,
    this.active = false,
    this.onTap,
    this.onRemove,
    this.removeTooltip,
  });

  final String label;
  final bool active;
  final VoidCallback? onTap;
  final VoidCallback? onRemove;
  final String? removeTooltip;

  @override
  State<KorvenChip> createState() => _KorvenChipState();
}

class _KorvenChipState extends State<KorvenChip> {
  bool _hover = false;

  @override
  Widget build(BuildContext context) {
    final activo = widget.active;
    final borde = (activo || _hover)
        ? KorvenColors.accent
        : KorvenColors.borderDefault;
    final texto = activo ? KorvenColors.accent : KorvenColors.textMuted;

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: KorvenMotion.fast,
          curve: KorvenMotion.easeOut,
          padding: const EdgeInsets.symmetric(
              horizontal: KorvenSpacing.s4, vertical: 9),
          decoration: BoxDecoration(
            color: activo
                ? KorvenColors.accent.withValues(alpha: 0.10)
                : Colors.transparent,
            border: Border.all(color: borde),
            borderRadius: BorderRadius.circular(KorvenRadius.sm),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Flexible + ellipsis: la etiqueta de búsqueda lleva dentro lo que
              // haya escrito el usuario y sin esto desborda la barra.
              Flexible(
                child: Text(widget.label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style:
                        KorvenType.mono.copyWith(fontSize: 13, color: texto)),
              ),
              if (widget.onRemove != null) ...[
                const SizedBox(width: KorvenSpacing.s2),
                Tooltip(
                  message: widget.removeTooltip ?? 'Quitar filtro',
                  child: GestureDetector(
                    onTap: widget.onRemove,
                    child: Icon(Icons.close, size: 13, color: texto),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
