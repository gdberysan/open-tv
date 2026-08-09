import 'package:flutter/material.dart';
import '../../domain/models/channel.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_motion.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import 'channel_logo.dart';
import 'favorite_star.dart';
import 'signal_bars.dart';

const _logoLado = 40.0;

/// Fila de canal. Sustituye al ListTile de Material: separación por hairline,
/// metadatos en mono y hover a carbón, como las celdas de la retícula del sitio.
class ChannelRow extends StatefulWidget {
  const ChannelRow({super.key, required this.channel, required this.onTap});

  final Channel channel;
  final VoidCallback? onTap;

  @override
  State<ChannelRow> createState() => _ChannelRowState();
}

class _ChannelRowState extends State<ChannelRow> {
  bool _hover = false;

  /// Une solo las partes que existen, para no dejar separadores huérfanos
  /// cuando al canal le falta el país o la categoría.
  String get _meta => [
        widget.channel.countryCode,
        widget.channel.categoryId,
      ].where((p) => p.isNotEmpty).join(' · ');

  @override
  Widget build(BuildContext context) {
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
              horizontal: KorvenSpacing.s5, vertical: KorvenSpacing.s3),
          decoration: BoxDecoration(
            color: _hover ? KorvenColors.surfaceCard : Colors.transparent,
            border: const Border(
                bottom: BorderSide(color: KorvenColors.borderSubtle)),
          ),
          child: Row(
            children: [
              Container(
                width: _logoLado,
                height: _logoLado,
                decoration: BoxDecoration(
                  color: KorvenColors.surfaceInset,
                  border: Border.all(color: KorvenColors.borderSubtle),
                  borderRadius: BorderRadius.circular(KorvenRadius.sm),
                ),
                clipBehavior: Clip.antiAlias,
                child: ChannelLogo(
                  url: widget.channel.logoUrl,
                  name: widget.channel.name,
                  size: _logoLado,
                ),
              ),
              const SizedBox(width: KorvenSpacing.s4),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.channel.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: KorvenType.body.copyWith(fontSize: 15),
                    ),
                    if (_meta.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(_meta,
                          style: KorvenType.monoLabel
                              .copyWith(color: KorvenColors.textFaint)),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: KorvenSpacing.s4),
              SignalBars(
                  alive: widget.channel.alive,
                  latencyMs: widget.channel.latencyMs),
              const SizedBox(width: KorvenSpacing.s2),
              FavoriteStar(channelId: widget.channel.id),
            ],
          ),
        ),
      ),
    );
  }
}
