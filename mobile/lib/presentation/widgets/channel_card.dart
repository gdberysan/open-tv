import 'package:flutter/material.dart';
import '../../domain/models/channel.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_motion.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import 'channel_logo.dart';
import 'favorite_star.dart';
import 'signal_bars.dart';

/// Tarjeta de canal para la rejilla: el logo como ancla visual, el nombre, y
/// una línea mono con las especificaciones.
class ChannelCard extends StatefulWidget {
  const ChannelCard({super.key, required this.channel, required this.onTap});

  final Channel channel;
  final VoidCallback? onTap;

  @override
  State<ChannelCard> createState() => _ChannelCardState();
}

class _ChannelCardState extends State<ChannelCard> {
  bool _hover = false;

  /// Solo las partes que existen, para no dejar separadores huérfanos.
  String get _specs {
    final res = resolucionDelNombre(widget.channel.name);
    return [
      widget.channel.countryCode,
      widget.channel.categoryId,
      if (res != null) res,
    ].where((p) => p.isNotEmpty).join(' · ');
  }

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
          decoration: BoxDecoration(
            color: _hover ? KorvenColors.surfaceCard : KorvenColors.surfaceInset,
            border: Border.all(
                color: _hover
                    ? KorvenColors.borderDefault
                    : KorvenColors.borderSubtle),
            borderRadius: BorderRadius.circular(KorvenRadius.md),
          ),
          clipBehavior: Clip.antiAlias,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(
                child: Stack(
                  children: [
                    Padding(
                      padding: const EdgeInsets.all(KorvenSpacing.s3),
                      child: ChannelLogo(
                        url: widget.channel.logoUrl,
                        name: widget.channel.name,
                        size: 72,
                      ),
                    ),
                    Positioned(
                      top: 0,
                      right: 0,
                      child: FavoriteStar(channelId: widget.channel.id),
                    ),
                    Positioned(
                      bottom: 4,
                      left: KorvenSpacing.s2,
                      child: SignalBars(
                          alive: widget.channel.alive,
                          latencyMs: widget.channel.latencyMs),
                    ),
                  ],
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(
                    horizontal: KorvenSpacing.s3, vertical: KorvenSpacing.s2),
                decoration: const BoxDecoration(
                  border: Border(
                      top: BorderSide(color: KorvenColors.borderSubtle)),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      widget.channel.name,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: KorvenType.bodySm.copyWith(fontSize: 13),
                    ),
                    if (_specs.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(
                        _specs,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: KorvenType.monoLabel
                            .copyWith(color: KorvenColors.textFaint),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// IPTV-org mete la resolución en el nombre entre paréntesis: "BBC One (1080p)".
/// Extraerla deja enseñarla como especificación en vez de ruido en el título.
String? resolucionDelNombre(String nombre) {
  final m = RegExp(r'\((4K|UHD|2160p|1080p|720p|576p|480p|360p|FHD)\)',
          caseSensitive: false)
      .firstMatch(nombre);
  return m?.group(1);
}
