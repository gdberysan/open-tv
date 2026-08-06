import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../domain/models/channel.dart';
import '../../domain/models/epg_entry.dart';
import '../providers/channel_provider.dart';
import '../providers/epg_provider.dart';
import 'player_screen.dart';

// Geometría de la parrilla: 4 px por minuto → 120 px por slot de 30 min.
const _pxPerMin = 4.0;
const _slotMinutes = 30;
const _slots = 12; // ventana de 6h
const _labelWidth = 150.0;
const _rowHeight = 56.0;
const _headerHeight = 28.0;
const _gridWidth = _slots * _slotMinutes * _pxPerMin;

/// Guía de programación (Fase 6.2): parrilla canales × slots de 30 min.
/// Toda la parrilla (cabecera + filas) vive dentro de UN scroll horizontal,
/// así el eje temporal queda alineado sin sincronizar controllers; las
/// etiquetas de canal se mantienen visibles compensando el desplazamiento.
class GuideScreen extends ConsumerStatefulWidget {
  const GuideScreen({super.key});

  @override
  ConsumerState<GuideScreen> createState() => _GuideScreenState();
}

class _GuideScreenState extends ConsumerState<GuideScreen> {
  final _hCtrl = ScrollController();
  final _vCtrl = ScrollController();

  @override
  void initState() {
    super.initState();
    _vCtrl.addListener(() {
      if (_vCtrl.position.extentAfter < 600) {
        ref.read(channelListProvider.notifier).loadMore();
      }
    });
  }

  @override
  void dispose() {
    _hCtrl.dispose();
    _vCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final listAsync = ref.watch(channelListProvider);
    final window = guideWindow(ref.watch(clockProvider)());

    return Scaffold(
      appBar: AppBar(title: const Text('Guía de programación')),
      body: listAsync.when(
        data: (state) {
          if (state.channels.isEmpty) {
            return const Center(child: Text('Sin canales'));
          }
          return SingleChildScrollView(
            controller: _hCtrl,
            scrollDirection: Axis.horizontal,
            child: SizedBox(
              width: _labelWidth + _gridWidth,
              child: Column(
                children: [
                  _TimeHeader(from: window.from),
                  Expanded(
                    child: ListView.builder(
                      controller: _vCtrl,
                      itemCount: state.channels.length,
                      itemExtent: _rowHeight,
                      itemBuilder: (_, i) => _ChannelRow(
                        channel: state.channels[i],
                        window: window,
                        hCtrl: _hCtrl,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Error: $e')),
      ),
    );
  }
}

class _TimeHeader extends StatelessWidget {
  final DateTime from;
  const _TimeHeader({required this.from});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: _headerHeight,
      child: Row(
        children: [
          const SizedBox(width: _labelWidth),
          for (var i = 0; i < _slots; i++)
            SizedBox(
              width: _slotMinutes * _pxPerMin,
              child: Text(
                _hhmm(from.add(Duration(minutes: i * _slotMinutes))),
                style: const TextStyle(fontSize: 11, color: Colors.grey),
              ),
            ),
        ],
      ),
    );
  }

  static String _hhmm(DateTime t) =>
      '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
}

class _ChannelRow extends ConsumerWidget {
  final Channel channel;
  final ({DateTime from, DateTime to}) window;
  final ScrollController hCtrl;

  const _ChannelRow({
    required this.channel,
    required this.window,
    required this.hCtrl,
  });

  void _play(BuildContext context) {
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => PlayerScreen(
        channelId: channel.id,
        channelName: channel.name,
        countryCode: channel.countryCode,
      ),
    ));
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final epgAsync = ref.watch(epgForChannelProvider(channel.id));
    final now = ref.watch(clockProvider)();
    final nowX = now.difference(window.from).inMinutes * _pxPerMin;

    return SizedBox(
      height: _rowHeight,
      child: Stack(
        children: [
          // Bloques de programas
          ...switch (epgAsync) {
            AsyncData(value: final entries) when entries.isEmpty => [
                const Positioned(
                  left: _labelWidth + 8,
                  top: 0,
                  bottom: 0,
                  child: Center(
                    child: Text('Sin programación',
                        style: TextStyle(color: Colors.grey, fontSize: 12)),
                  ),
                ),
              ],
            AsyncData(value: final entries) => [
                for (final e in entries) _programmeBlock(context, e),
              ],
            _ => [
                const Positioned(
                  left: _labelWidth + 8,
                  top: 20,
                  child: SizedBox(
                    width: 14,
                    height: 14,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                ),
              ],
          },
          // Línea de "ahora"
          if (nowX >= 0 && nowX <= _gridWidth)
            Positioned(
              left: _labelWidth + nowX,
              top: 0,
              bottom: 0,
              child: Container(width: 2, color: Colors.redAccent),
            ),
          // Etiqueta de canal fija: compensa el scroll horizontal
          AnimatedBuilder(
            animation: hCtrl,
            builder: (_, child) => Transform.translate(
              offset: Offset(hCtrl.hasClients ? hCtrl.offset : 0, 0),
              child: child,
            ),
            child: InkWell(
              onTap: () => _play(context),
              child: Container(
                width: _labelWidth,
                height: _rowHeight,
                padding: const EdgeInsets.symmetric(horizontal: 8),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.surface,
                  border: Border(
                    right: BorderSide(
                        color: Theme.of(context).dividerColor, width: 0.5),
                    bottom: BorderSide(
                        color: Theme.of(context).dividerColor, width: 0.5),
                  ),
                ),
                alignment: Alignment.centerLeft,
                child: Text(
                  channel.name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 12),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _programmeBlock(BuildContext context, EPGEntry e) {
    // Recortar el programa a la ventana visible
    final start = e.startAt.isBefore(window.from) ? window.from : e.startAt;
    final end = e.endAt.isAfter(window.to) ? window.to : e.endAt;
    if (!end.isAfter(start)) return const SizedBox.shrink();

    final left = start.difference(window.from).inMinutes * _pxPerMin;
    final width = end.difference(start).inMinutes * _pxPerMin;
    final airing = e.airsAt(DateTime.now());

    return Positioned(
      left: _labelWidth + left,
      top: 2,
      width: width,
      height: _rowHeight - 5,
      child: InkWell(
        onTap: () => _play(context),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 4),
          decoration: BoxDecoration(
            color: airing
                ? Theme.of(context).colorScheme.primaryContainer
                : Theme.of(context).colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(4),
            border: Border.all(color: Theme.of(context).dividerColor, width: 0.5),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(e.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 12, fontWeight: FontWeight.w600)),
              if (e.description.isNotEmpty)
                Text(e.description,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 10, color: Colors.grey)),
            ],
          ),
        ),
      ),
    );
  }
}
