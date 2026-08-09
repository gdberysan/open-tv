import 'package:flutter/material.dart';
import '../../domain/models/channel.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import 'channel_card.dart';

/// Rejilla de canales. Con maxCrossAxisExtent el número de columnas se adapta
/// al ancho de la ventana en vez de fijarse a un número.
class ChannelGrid extends StatelessWidget {
  const ChannelGrid({
    super.key,
    required this.channels,
    required this.showTailLoader,
    required this.controller,
    required this.onSelect,
    this.loadMoreError,
    this.onRetry,
  });

  final List<Channel> channels;
  final bool showTailLoader;
  final String? loadMoreError;
  final VoidCallback? onRetry;
  final ScrollController controller;
  final void Function(Channel) onSelect;

  @override
  Widget build(BuildContext context) {
    final hayCola = showTailLoader || loadMoreError != null;

    return CustomScrollView(
      controller: controller,
      slivers: [
        SliverPadding(
          padding: const EdgeInsets.all(KorvenSpacing.s5),
          sliver: SliverGrid(
            gridDelegate:
                const SliverGridDelegateWithMaxCrossAxisExtent(
              maxCrossAxisExtent: 190,
              mainAxisSpacing: KorvenSpacing.s3,
              crossAxisSpacing: KorvenSpacing.s3,
              childAspectRatio: 0.82,
            ),
            delegate: SliverChildBuilderDelegate(
              (_, i) => ChannelCard(
                channel: channels[i],
                onTap: () => onSelect(channels[i]),
              ),
              childCount: channels.length,
            ),
          ),
        ),
        if (hayCola)
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.only(bottom: KorvenSpacing.s6),
              child: loadMoreError != null
                  ? Column(
                      children: [
                        Text(loadMoreError!,
                            textAlign: TextAlign.center,
                            style: KorvenType.monoLabel
                                .copyWith(color: KorvenColors.textFaint)),
                        const SizedBox(height: KorvenSpacing.s2),
                        TextButton.icon(
                          onPressed: onRetry,
                          icon: const Icon(Icons.refresh, size: 18),
                          label: const Text('Reintentar'),
                        ),
                      ],
                    )
                  : const Center(
                      child: SizedBox(
                        width: 24,
                        height: 24,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      ),
                    ),
            ),
          ),
      ],
    );
  }
}
