import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_motion.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';

/// Botón de filtro. Los tres van dentro de un Row con Expanded, así que miden
/// lo mismo y quedan alineados en una sola línea.
///
/// Muestra la etiqueta neutra cuando no hay selección y el valor cuando la hay,
/// en ámbar — la regla del sistema: el acento marca decisión del usuario.
class FilterButton extends StatefulWidget {
  const FilterButton({
    super.key,
    required this.icon,
    required this.label,
    required this.onTap,
    this.value,
    this.leading,
    this.onClear,
  });

  final IconData icon;

  /// Etiqueta neutra, visible cuando no hay valor.
  final String label;

  /// Valor activo. Si es null, el filtro está sin poner.
  final String? value;

  /// Widget opcional antes del texto (la bandera del país).
  final Widget? leading;

  final VoidCallback onTap;
  final VoidCallback? onClear;

  @override
  State<FilterButton> createState() => _FilterButtonState();
}

class _FilterButtonState extends State<FilterButton> {
  bool _hover = false;

  @override
  Widget build(BuildContext context) {
    final activo = widget.value != null;
    final color = activo ? KorvenColors.accent : KorvenColors.textMuted;
    final borde = (activo || _hover)
        ? KorvenColors.accent
        : KorvenColors.borderDefault;

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: KorvenMotion.fast,
          curve: KorvenMotion.easeOut,
          height: 40,
          padding: const EdgeInsets.symmetric(horizontal: KorvenSpacing.s3),
          decoration: BoxDecoration(
            color: activo
                ? KorvenColors.accent.withValues(alpha: 0.10)
                : Colors.transparent,
            border: Border.all(color: borde),
            borderRadius: BorderRadius.circular(KorvenRadius.sm),
          ),
          child: Row(
            children: [
              if (widget.leading != null)
                Padding(
                  padding: const EdgeInsets.only(right: KorvenSpacing.s2),
                  child: widget.leading,
                )
              else
                Padding(
                  padding: const EdgeInsets.only(right: KorvenSpacing.s2),
                  child: Icon(widget.icon, size: 16, color: color),
                ),
              Expanded(
                child: Text(
                  widget.value ?? widget.label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: KorvenType.mono.copyWith(fontSize: 13, color: color),
                ),
              ),
              if (activo && widget.onClear != null)
                Tooltip(
                  message: 'Quitar filtro de ${widget.label}',
                  child: GestureDetector(
                    onTap: widget.onClear,
                    child: Icon(Icons.close, size: 14, color: color),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
