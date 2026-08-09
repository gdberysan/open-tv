import 'dart:async';
import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';

/// Línea de terminal de la marca, con el cursor ámbar parpadeante. Sustituye al
/// spinner en el reproductor: además de estar en la voz de Korven, nombra lo que
/// está pasando en vez de limitarse a girar.
class ConsoleLine extends StatefulWidget {
  const ConsoleLine({super.key, required this.text, this.blinking = true});

  final String text;
  final bool blinking;

  @override
  State<ConsoleLine> createState() => _ConsoleLineState();
}

class _ConsoleLineState extends State<ConsoleLine> {
  Timer? _timer;
  bool _visible = true;

  @override
  void initState() {
    super.initState();
    if (widget.blinking) {
      // Conmuta, no interpola: un cursor de terminal no se desvanece.
      _timer = Timer.periodic(const Duration(milliseconds: 550), (_) {
        if (mounted) setState(() => _visible = !_visible);
      });
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
          horizontal: KorvenSpacing.s4, vertical: KorvenSpacing.s3),
      decoration: BoxDecoration(
        color: KorvenColors.codeBg,
        border: Border.all(color: KorvenColors.borderDefault),
        borderRadius: BorderRadius.circular(KorvenRadius.md),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Flexible(
            child: Text(
              widget.text,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: KorvenType.mono.copyWith(color: KorvenColors.textBody),
            ),
          ),
          const SizedBox(width: 6),
          Opacity(
            key: const Key('console-cursor'),
            opacity: _visible ? 1 : 0,
            child: Container(width: 8, height: 16, color: KorvenColors.accent),
          ),
        ],
      ),
    );
  }
}
