import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../domain/models/channel_filter.dart';
import '../providers/channel_provider.dart';
import 'player_screen.dart';

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final channelsAsync = ref.watch(channelsProvider);
    final filter = ref.watch(channelFilterProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('IPTV'),
        actions: [
          IconButton(
            icon: const Icon(Icons.search),
            tooltip: 'Buscar canal',
            onPressed: () => _showSearch(context, ref),
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: 'Recargar',
            onPressed: () => ref.invalidate(channelsProvider),
          ),
        ],
      ),
      body: Column(
        children: [
          _FilterBar(filter: filter, ref: ref),
          Expanded(
            child: channelsAsync.when(
              data: (channels) {
                if (channels.isEmpty) {
                  return _EmptyState(
                    message: filter.hasActiveFilters
                        ? 'Sin canales con estos filtros'
                        : 'El gateway está sincronizando canales.\nEspera y recarga.',
                    onRetry: () => ref.invalidate(channelsProvider),
                  );
                }
                return ListView.builder(
                  itemCount: channels.length,
                  itemBuilder: (ctx, i) {
                    final ch = channels[i];
                    return ListTile(
                      leading: _ChannelLogo(url: ch.logoUrl),
                      title: Text(ch.name),
                      subtitle: ch.categoryId.isNotEmpty
                          ? Text(ch.categoryId,
                              style: const TextStyle(fontSize: 11))
                          : null,
                      trailing: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
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
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (e, _) => _EmptyState(
                message: e.toString(),
                icon: Icons.error_outline,
                onRetry: () => ref.invalidate(channelsProvider),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showSearch(BuildContext context, WidgetRef ref) {
    showSearch(
      context: context,
      delegate: _ChannelSearchDelegate(ref),
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

class _FilterBar extends StatelessWidget {
  final ChannelFilter filter;
  final WidgetRef ref;

  const _FilterBar({required this.filter, required this.ref});

  @override
  Widget build(BuildContext context) {
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
              onTap: () => _pickCountry(context),
            ),
            const SizedBox(width: 6),
            // Category filter
            _FilterButton(
              label: filter.category.isEmpty ? 'Categoría' : filter.category,
              icon: Icons.category_outlined,
              active: filter.category.isNotEmpty,
              onTap: () => _pickCategory(context),
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

  Future<void> _pickCountry(BuildContext context) async {
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

  Future<void> _pickCategory(BuildContext context) async {
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

// ── Search delegate ──────────────────────────────────────────────────────────

class _ChannelSearchDelegate extends SearchDelegate<String> {
  final WidgetRef ref;
  _ChannelSearchDelegate(this.ref);

  @override
  String get searchFieldLabel => 'Buscar canal...';

  @override
  List<Widget> buildActions(BuildContext context) => [
        IconButton(
          icon: const Icon(Icons.clear),
          onPressed: () => query = '',
        ),
      ];

  @override
  Widget buildLeading(BuildContext context) => IconButton(
        icon: const Icon(Icons.arrow_back),
        onPressed: () => close(context, ''),
      );

  @override
  Widget buildResults(BuildContext context) => _buildList(context);

  @override
  Widget buildSuggestions(BuildContext context) => _buildList(context);

  Widget _buildList(BuildContext context) {
    final channelsAsync = ref.watch(channelsProvider);
    return channelsAsync.when(
      data: (channels) {
        final hits = query.isEmpty
            ? channels
            : channels
                .where((c) =>
                    c.name.toLowerCase().contains(query.toLowerCase()))
                .toList();

        if (hits.isEmpty) {
          return Center(child: Text('Sin resultados para "$query"'));
        }
        return ListView.builder(
          itemCount: hits.length,
          itemBuilder: (ctx, i) {
            final ch = hits[i];
            return ListTile(
              leading: _ChannelLogo(url: ch.logoUrl),
              title: Text(ch.name),
              onTap: () {
                close(context, ch.id);
                Navigator.of(ctx).push(MaterialPageRoute(
                  builder: (_) => PlayerScreen(
                    channelId: ch.id,
                    channelName: ch.name,
                    countryCode: ch.countryCode,
                  ),
                ));
              },
            );
          },
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('Error: $e')),
    );
  }
}
