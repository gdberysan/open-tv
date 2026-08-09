import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../theme/korven_colors.dart';
import '../providers/favorites_provider.dart';

/// Estrella de favorito. Ámbar cuando lo es — es una decisión del usuario, que
/// es justo lo que marca el acento en este sistema.
class FavoriteStar extends ConsumerWidget {
  const FavoriteStar({super.key, required this.channelId, this.size = 18});

  final String channelId;
  final double size;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final esFav = ref.watch(favoritesProvider).contains(channelId);

    return Tooltip(
      message: esFav ? 'Quitar de favoritos' : 'Añadir a favoritos',
      child: InkWell(
        onTap: () => ref.read(favoritesProvider.notifier).toggle(channelId),
        child: Padding(
          padding: const EdgeInsets.all(4),
          child: Icon(
            esFav ? Icons.star : Icons.star_border,
            size: size,
            color: esFav ? KorvenColors.accent : KorvenColors.textFaint,
          ),
        ),
      ),
    );
  }
}
