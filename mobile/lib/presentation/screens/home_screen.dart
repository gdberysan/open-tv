import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/api_error.dart';
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';
import '../providers/channel_provider.dart';
import '../widgets/signal_bars.dart';
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
    // Cargar la siguiente página cuando queda poco por debajo del viewport
    if (_scrollCtrl.position.extentAfter < 600) {
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

  @override
  Widget build(BuildContext context) {
    final listAsync = ref.watch(channelListProvider);
    final filter = ref.watch(channelFilterProvider);

    return Scaffold(
      appBar: AppBar(
        title: _searching
            ? TextField(
                controller: _searchCtrl,
                autofocus: true,
                decoration: const InputDecoration(
                  hintText: 'Buscar canal…',
                  border: InputBorder.none,
                ),
                onChanged: _onSearchChanged,
              )
            : const Text('IPTV'),
        actions: [
          if (_searching)
            IconButton(
              icon: const Icon(Icons.close),
              tooltip: 'Cerrar búsqueda',
              onPressed: _closeSearch,
            )
          else ...[
            IconButton(
              icon: const Icon(Icons.search),
              tooltip: 'Buscar canal',
              onPressed: () => setState(() => _searching = true),
            ),
            IconButton(
              icon: Icon(ref.watch(showOfflineProvider)
                  ? Icons.visibility
                  : Icons.visibility_off),
              tooltip: ref.watch(showOfflineProvider)
                  ? 'Ocultar canales offline'
                  : 'Mostrar canales offline',
              onPressed: () =>
                  ref.read(showOfflineProvider.notifier).toggle(),
            ),
            IconButton(
              icon: const Icon(Icons.refresh),
              tooltip: 'Recargar',
              onPressed: () => ref.invalidate(channelListProvider),
            ),
          ],
        ],
      ),
      body: Column(
        children: [
          const _FilterBar(),
          Expanded(
            child: listAsync.when(
              data: (state) {
                if (state.channels.isEmpty) {
                  return _EmptyState(
                    message: filter.hasActiveFilters
                        ? 'Sin canales con estos filtros'
                        : 'El gateway está sincronizando canales.\nEspera y recarga.',
                    onRetry: () => ref.invalidate(channelListProvider),
                  );
                }
                return _ChannelList(
                  channels: state.channels,
                  showTailLoader: state.hasMore,
                  controller: _scrollCtrl,
                );
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => _EmptyState(
                message: ApiError.desde(e).mensaje,
                icon: Icons.error_outline,
                onRetry: () => ref.invalidate(channelListProvider),
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
  final ScrollController controller;

  const _ChannelList({
    required this.channels,
    required this.showTailLoader,
    required this.controller,
  });

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      controller: controller,
      itemCount: channels.length + (showTailLoader ? 1 : 0),
      itemBuilder: (ctx, i) {
        if (i >= channels.length) {
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
        return ListTile(
          leading: _ChannelLogo(url: ch.logoUrl),
          title: Text(ch.name),
          subtitle: ch.categoryId.isNotEmpty
              ? Text(ch.categoryId, style: const TextStyle(fontSize: 11))
              : null,
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              SignalBars(alive: ch.alive, latencyMs: ch.latencyMs),
              const SizedBox(width: 8),
              if (ch.countryCode.isNotEmpty)
                Text(_flag(ch.countryCode),
                    style: const TextStyle(fontSize: 16)),
              const SizedBox(width: 4),
              const Icon(Icons.play_circle_outline),
            ],
          ),
          onTap: () => Navigator.of(ctx).push(
            MaterialPageRoute(
              builder: (_) => PlayerScreen(
                channelId: ch.id,
                channelName: ch.name,
                countryCode: ch.countryCode,
              ),
            ),
          ),
        );
      },
    );
  }

  static String _flag(String cc) {
    if (cc.length != 2) return '';
    final a = cc.toUpperCase().codeUnitAt(0) - 0x41 + 0x1F1E6;
    final b = cc.toUpperCase().codeUnitAt(1) - 0x41 + 0x1F1E6;
    return String.fromCharCode(a) + String.fromCharCode(b);
  }
}

// ── Filter bar ───────────────────────────────────────────────────────────────

class _FilterBar extends ConsumerWidget {
  const _FilterBar();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final filter = ref.watch(channelFilterProvider);

    return Container(
      color: Theme.of(context).colorScheme.surface,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            // Quality chips
            ..._qualityOptions.map((opt) {
              final selected = filter.quality == opt.$1;
              return Padding(
                padding: const EdgeInsets.only(right: 6),
                child: FilterChip(
                  label: Text(opt.$2),
                  selected: selected,
                  onSelected: (_) => ref
                      .read(channelFilterProvider.notifier)
                      .state = filter.copyWith(quality: opt.$1),
                ),
              );
            }),
            const SizedBox(width: 4),
            // Country filter
            _FilterButton(
              label: filter.country.isEmpty ? 'País' : filter.country,
              icon: Icons.flag_outlined,
              active: filter.country.isNotEmpty,
              onTap: () => _pickCountry(context, ref, filter),
            ),
            const SizedBox(width: 6),
            // Category filter
            _FilterButton(
              label: filter.category.isEmpty ? 'Categoría' : filter.category,
              icon: Icons.category_outlined,
              active: filter.category.isNotEmpty,
              onTap: () => _pickCategory(context, ref, filter),
            ),
            // Clear all filters
            if (filter.hasActiveFilters) ...[
              const SizedBox(width: 6),
              IconButton(
                icon: const Icon(Icons.close, size: 18),
                tooltip: 'Limpiar filtros',
                onPressed: () => ref
                    .read(channelFilterProvider.notifier)
                    .state = const ChannelFilter(),
              ),
            ],
          ],
        ),
      ),
    );
  }

  // Quality: value, label
  static const _qualityOptions = [
    ('4k', '4K'),
    ('fhd', '1080p+'),
    ('hd', 'HD 720p+'),
    ('', 'Todos'),
  ];

  Future<void> _pickCountry(
      BuildContext context, WidgetRef ref, ChannelFilter filter) async {
    final picked = await showDialog<String>(
      context: context,
      builder: (_) => _PickerDialog(
        title: 'Filtrar por país',
        options: _popularCountries,
        selected: filter.country,
      ),
    );
    if (picked != null) {
      ref.read(channelFilterProvider.notifier).state =
          filter.copyWith(country: picked == filter.country ? '' : picked);
    }
  }

  Future<void> _pickCategory(
      BuildContext context, WidgetRef ref, ChannelFilter filter) async {
    final picked = await showDialog<String>(
      context: context,
      builder: (_) => _PickerDialog(
        title: 'Filtrar por categoría',
        options: _popularCategories,
        selected: filter.category,
      ),
    );
    if (picked != null) {
      ref.read(channelFilterProvider.notifier).state =
          filter.copyWith(category: picked == filter.category ? '' : picked);
    }
  }

  static const _popularCountries = [
    'GB', 'US', 'FR', 'DE', 'ES', 'IT', 'AU', 'CA', 'JP', 'BR',
    'MX', 'AR', 'NL', 'BE', 'CH', 'AT', 'PL', 'PT', 'RU', 'IN',
  ];

  static const _popularCategories = [
    'News', 'Sports', 'Entertainment', 'Movies', 'Kids',
    'Documentary', 'Music', 'Lifestyle', 'Science', 'Travel',
  ];
}

class _FilterButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final bool active;
  final VoidCallback onTap;

  const _FilterButton({
    required this.label,
    required this.icon,
    required this.active,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final color = active
        ? Theme.of(context).colorScheme.primary
        : Theme.of(context).colorScheme.outline;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(20),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: color),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 14, color: color),
            const SizedBox(width: 4),
            Text(label,
                style: TextStyle(
                    fontSize: 12,
                    color: color,
                    fontWeight:
                        active ? FontWeight.w600 : FontWeight.normal)),
          ],
        ),
      ),
    );
  }
}

// ── Picker dialog ────────────────────────────────────────────────────────────

class _PickerDialog extends StatefulWidget {
  final String title;
  final List<String> options;
  final String selected;

  const _PickerDialog({
    required this.title,
    required this.options,
    required this.selected,
  });

  @override
  State<_PickerDialog> createState() => _PickerDialogState();
}

class _PickerDialogState extends State<_PickerDialog> {
  late List<String> _filtered;
  final _ctrl = TextEditingController();

  @override
  void initState() {
    super.initState();
    _filtered = widget.options;
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  void _onSearch(String q) {
    setState(() {
      _filtered = widget.options
          .where((o) => o.toLowerCase().contains(q.toLowerCase()))
          .toList();
    });
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: SizedBox(
        width: 320,
        height: 400,
        child: Column(
          children: [
            TextField(
              controller: _ctrl,
              decoration: const InputDecoration(
                hintText: 'Buscar…',
                prefixIcon: Icon(Icons.search, size: 18),
                isDense: true,
              ),
              onChanged: _onSearch,
            ),
            const SizedBox(height: 8),
            Expanded(
              child: ListView.builder(
                itemCount: _filtered.length,
                itemBuilder: (_, i) {
                  final opt = _filtered[i];
                  final sel = opt == widget.selected;
                  return ListTile(
                    dense: true,
                    title: Text(opt),
                    selected: sel,
                    trailing: sel
                        ? const Icon(Icons.check, size: 16)
                        : null,
                    onTap: () => Navigator.of(context).pop(opt),
                  );
                },
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancelar'),
        ),
      ],
    );
  }
}

// ── Empty state ──────────────────────────────────────────────────────────────

class _EmptyState extends StatelessWidget {
  final String message;
  final IconData icon;
  final VoidCallback onRetry;

  const _EmptyState({
    required this.message,
    this.icon = Icons.tv_off,
    required this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(icon, size: 56, color: Colors.grey),
          const SizedBox(height: 16),
          Text(
            message,
            textAlign: TextAlign.center,
            style: const TextStyle(color: Colors.grey, fontSize: 13),
          ),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: onRetry,
            child: const Text('Reintentar'),
          ),
        ],
      ),
    );
  }
}

// ── Logo widget ──────────────────────────────────────────────────────────────

class _ChannelLogo extends StatelessWidget {
  final String url;
  const _ChannelLogo({required this.url});

  @override
  Widget build(BuildContext context) {
    if (url.isEmpty) return const Icon(Icons.tv, size: 40);
    return Image.network(
      url,
      width: 40,
      height: 40,
      fit: BoxFit.contain,
      errorBuilder: (_, __, ___) => const Icon(Icons.tv, size: 40),
    );
  }
}
