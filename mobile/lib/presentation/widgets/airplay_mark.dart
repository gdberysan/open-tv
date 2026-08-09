import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../theme/korven_colors.dart';
import '../../theme/korven_typography.dart';
import '../providers/airplay_memory_provider.dart';
import '../providers/cast_provider.dart';

/// Marca un canal que AVFoundation ya rechazó por formato.
///
/// Solo aparece con sesión de emisión activa Y memoria de fallo. Nunca por
/// ausencia de dato: el 42 % del catálogo no se puede clasificar desde el
/// manifiesto, y marcar eso convertiría la rejilla en ruido.
///
/// Lee sus propios providers en vez de recibirlos, igual que FavoriteStar, para
/// no obligar a ChannelRow y ChannelCard a volverse Consumer.
class AirplayMark extends ConsumerWidget {
  const AirplayMark({super.key, required this.channelId});

  final String channelId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!ref.watch(castProvider).intercepta) return const SizedBox.shrink();
    if (!ref.watch(airplayMemoryProvider).contains(channelId)) {
      return const SizedBox.shrink();
    }
    return Text(
      'sin airplay',
      style: KorvenType.monoLabel.copyWith(color: KorvenColors.textFaint),
    );
  }
}
