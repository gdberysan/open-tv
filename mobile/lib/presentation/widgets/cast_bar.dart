import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/models/cast_session.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../providers/cast_provider.dart';

/// Barra persistente de la sesión de emisión. Se dibuja en cuanto hay una ruta
/// puesta, aunque todavía no suene nada: elegir dispositivo y elegir canal son
/// dos actos distintos.
class CastBar extends ConsumerWidget {
  const CastBar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sesion = ref.watch(castProvider);
    if (!sesion.visible) return const SizedBox.shrink();

    return Container(
      height: 44,
      padding: const EdgeInsets.symmetric(horizontal: KorvenSpacing.s4),
      decoration: const BoxDecoration(
        color: KorvenColors.surfaceBase,
        border: Border(top: BorderSide(color: KorvenColors.borderSubtle)),
      ),
      child: Row(
        children: [
          const Icon(Icons.airplay, size: 16, color: KorvenColors.textMuted),
          const SizedBox(width: KorvenSpacing.s3),
          Expanded(
            child: Text(
              _texto(sesion),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontFamily: 'JetBrainsMono',
                fontSize: 12,
                color: KorvenColors.textMuted,
              ),
            ),
          ),
          IconButton(
            iconSize: 18,
            splashRadius: 16,
            tooltip: 'Terminar sesión',
            icon: const Icon(Icons.stop, color: KorvenColors.textMuted),
            onPressed: () => ref.read(castProvider.notifier).detener(),
          ),
        ],
      ),
    );
  }

  String _texto(CastSession s) {
    final destino = s.etiquetaDispositivo;
    return switch (s.state) {
      CastState.armed => '$destino · listo para emitir',
      CastState.connecting => '$destino · abriendo ${s.channelName ?? ''}',
      CastState.casting => '$destino · ${s.channelName ?? ''}',
      CastState.failed => '$destino · ${s.error ?? 'fallo'}',
      CastState.idle => destino,
    };
  }
}
