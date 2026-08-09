import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/api_error.dart';
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';
import '../providers/channel_provider.dart';
import '../providers/view_mode_provider.dart';
import '../widgets/channel_grid.dart';
import '../widgets/channel_row.dart';
import '../widgets/console_bar.dart';
import '../widgets/filter_bar.dart';
import '../widgets/state_views.dart';
import 'player_screen.dart';

class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  final _scrollCtrl = ScrollController();
  final _searchCtrl = TextEditingController();
  Timer? _debounce;
  bool _searching = false;

  @override
  void initState() {
    super.initState();
    _scrollCtrl.addListener(_onScroll);
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _scrollCtrl.dispose();
    _searchCtrl.dispose();
    super.dispose();
  }

  void _onScroll() {
    // Cargar la siguiente página cuando queda poco por debajo del viewport.
    // Con un error pendiente no se reintenta solo: el scroll dispara este
    // callback en cada frame y machacaría el gateway. El reintento es
    // explícito, con el botón de la fila de error.
    if (_scrollCtrl.position.extentAfter < 600) {
      final estado = ref.read(channelListProvider).valueOrNull;
      if (estado?.loadMoreError != null) return;
      ref.read(channelListProvider.notifier).loadMore();
    }
  }

  /// La búsqueda viaja al gateway (param q): busca sobre los ~12k canales
  /// de la DB, no solo sobre la página ya descargada.
  void _onSearchChanged(String q) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), () {
      final filter = ref.read(channelFilterProvider);
      ref.read(channelFilterProvider.notifier).state =
          filter.copyWith(query: q.trim());
    });
  }

  void _closeSearch() {
    _debounce?.cancel();
    _searchCtrl.clear();
    setState(() => _searching = false);
    final filter = ref.read(channelFilterProvider);
    if (filter.query.isNotEmpty) {
      ref.read(channelFilterProvider.notifier).state =
          filter.copyWith(query: '');
    }
  }

  void _abrirCanal(BuildContext context, Channel ch) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => PlayerScreen(
          channelId: ch.id,
          channelName: ch.name,
          countryCode: ch.countryCode,
        ),
      ),
    );
  }

  Future<void> _canalAleatorio() async {
    try {
      final ch = await ref.read(randomChannelProvider)();
      if (!mounted) return;
      _abrirCanal(context, ch);
    } catch (e) {
      if (!mounted) return;
      // Con filtros imposibles el gateway responde 404; decirlo es mejor que
      // no hacer nada al pulsar.
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(ApiError.desde(e).mensaje)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final listAsync = ref.watch(channelListProvider);
    final filter = ref.watch(channelFilterProvider);
    final modo = ref.watch(viewModeProvider);

    return Scaffold(
      appBar: ConsoleBar(
        searching: _searching,
        searchController: _searchCtrl,
        onSearchChanged: _onSearchChanged,
        onOpenSearch: () => setState(() => _searching = true),
        onCloseSearch: _closeSearch,
        showOffline: ref.watch(showOfflineProvider),
        onToggleOffline: () => ref.read(showOfflineProvider.notifier).toggle(),
        onReload: () => ref.invalidate(channelListProvider),
        gridMode: modo == ViewMode.grid,
        onToggleView: () => ref.read(viewModeProvider.notifier).toggle(),
        onRandom: _canalAleatorio,
      ),
      body: Column(
        children: [
          const FilterBar(),
          Expanded(
            child: listAsync.when(
              data: (state) {
                if (state.channels.isEmpty) {
                  // Tres situaciones distintas merecen tres mensajes: no es lo
                  // mismo que los filtros no casen a que el gateway no haya
                  // sincronizado. Antes ambas culpaban al sync.
                  return filter.hasActiveFilters
                      ? KorvenStateView(
                          eyebrow: '// sin resultados',
                          message: 'Ningún canal casa con estos filtros.',
                          action: TextButton(
                            onPressed: () => ref
                                .read(channelFilterProvider.notifier)
                                .state = const ChannelFilter(),
                            child: const Text('limpiar filtros'),
                          ),
                        )
                      : KorvenStateView(
                          eyebrow: '// catálogo vacío',
                          message:
                              'El gateway todavía no ha sincronizado el catálogo.',
                          action: ElevatedButton(
                            onPressed: () =>
                                ref.invalidate(channelListProvider),
                            child: const Text('Recargar'),
                          ),
                        );
                }
                // isLoadingMore, no hasMore: el spinner giraba siempre que
                // hubiera más páginas, cargando o no.
                if (modo == ViewMode.grid) {
                  return ChannelGrid(
                    channels: state.channels,
                    showTailLoader: state.isLoadingMore,
                    loadMoreError: state.loadMoreError,
                    onRetry: () =>
                        ref.read(channelListProvider.notifier).loadMore(),
                    controller: _scrollCtrl,
                    onSelect: (ch) => _abrirCanal(context, ch),
                  );
                }
                return _ChannelList(
                  channels: state.channels,
                  showTailLoader: state.isLoadingMore,
                  loadMoreError: state.loadMoreError,
                  onRetry: () =>
                      ref.read(channelListProvider.notifier).loadMore(),
                  controller: _scrollCtrl,
                  onSelect: (ch) => _abrirCanal(context, ch),
                );
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => KorvenStateView(
                eyebrow: '// error',
                message: ApiError.desde(e).mensaje,
                action: ElevatedButton(
                  onPressed: () => ref.invalidate(channelListProvider),
                  child: const Text('Reintentar'),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

// ── Channel list ─────────────────────────────────────────────────────────────

class _ChannelList extends StatelessWidget {
  final List<Channel> channels;
  final bool showTailLoader;
  final String? loadMoreError;
  final VoidCallback? onRetry;
  final ScrollController controller;
  final void Function(Channel) onSelect;

  const _ChannelList({
    required this.channels,
    required this.showTailLoader,
    this.loadMoreError,
    this.onRetry,
    required this.controller,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      controller: controller,
      itemCount: channels.length + (showTailLoader || loadMoreError != null ? 1 : 0),
      itemBuilder: (ctx, i) {
        if (i >= channels.length) {
          if (loadMoreError != null) {
            return Padding(
              padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 24),
              child: Column(
                children: [
                  Text(
                    loadMoreError!,
                    textAlign: TextAlign.center,
                    style: const TextStyle(color: Colors.grey, fontSize: 12),
                  ),
                  const SizedBox(height: 8),
                  TextButton.icon(
                    onPressed: onRetry,
                    icon: const Icon(Icons.refresh, size: 18),
                    label: const Text('Reintentar'),
                  ),
                ],
              ),
            );
          }
          return const Padding(
            padding: EdgeInsets.symmetric(vertical: 16),
            child: Center(
              child: SizedBox(
                width: 24,
                height: 24,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),
          );
        }
        final ch = channels[i];
        return ChannelRow(channel: ch, onTap: () => onSelect(ch));
      },
    );
  }

}

