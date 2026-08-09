import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/airplay/airplay_platform.dart';
import '../../domain/models/cast_session.dart';
import '../../theme/korven_colors.dart';
import '../providers/cast_provider.dart';

/// Abre el selector de rutas AirPlay del sistema.
///
/// El icono lo dibuja Flutter en vez de incrustar el AVRoutePickerView de Apple
/// con AppKitView. No es preferencia estética: Flutter **no implementa** el
/// reenvío de gestos a vistas de plataforma en macOS —
/// `RenderAppKitView.updateGestureRecognizers` tiene el cuerpo vacío, ver
/// flutter/flutter#128519— así que la vista de Apple se dibujaba pero no
/// recibía un solo clic. Dibujarlo aquí tiene además la ventaja de que encaja
/// con el resto de acciones de la barra en vez de traer su propio estilo.
///
/// Fuera de macOS colapsa a cero: los targets de la Fase 9 compilan sin tocarlo
/// y CI, que corre en Linux, nunca llama al canal.
class AirplayButton extends ConsumerWidget {
  const AirplayButton({super.key});

  static const _lado = 28.0;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.macOS) {
      return const SizedBox.shrink();
    }

    // Ámbar solo cuando hay sesión, según la regla del sistema: las acciones
    // son mudas por defecto y solo se encienden si están activas.
    final activa = ref.watch(castProvider).state != CastState.idle;

    return SizedBox(
      width: 40,
      height: 40,
      child: IconButton(
        iconSize: 18,
        splashRadius: 18,
        padding: EdgeInsets.zero,
        tooltip: 'Emitir a un televisor',
        icon: Icon(
          Icons.airplay,
          color: activa ? KorvenColors.accent : KorvenColors.textMuted,
        ),
        onPressed: () => _abrir(context, ref),
      ),
    );
  }

  Future<void> _abrir(BuildContext context, WidgetRef ref) async {
    // El popover nativo se ancla por coordenadas, así que hay que decirle dónde
    // está este botón dentro de la ventana.
    final box = context.findRenderObject() as RenderBox?;
    final origen = box?.localToGlobal(Offset.zero) ?? Offset.zero;

    final abierto = await ref.read(airplayPlatformProvider).showRoutePicker(
          x: origen.dx,
          y: origen.dy,
          lado: _lado,
        );

    if (abierto || !context.mounted) return;
    // Fallar en silencio dejaría el botón pareciendo roto, que es exactamente
    // el síntoma que trajo aquí.
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('// no se pudo abrir el selector de AirPlay'),
      ),
    );
  }
}
