import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';
import '../../data/repositories/channel_repository.dart';

final channelRepositoryProvider = Provider<IChannelRepository>((ref) {
  return ChannelRepository();
});

final channelFilterProvider = StateProvider<ChannelFilter>(
  (_) => const ChannelFilter(),
);

/// Estado de la lista paginada: el gateway tiene ~12k canales y sirve
/// páginas de hasta 1000; la app va acumulando páginas al hacer scroll.
class ChannelListState {
  final List<Channel> channels;
  final bool hasMore;
  final bool isLoadingMore;

  const ChannelListState({
    required this.channels,
    required this.hasMore,
    this.isLoadingMore = false,
  });

  ChannelListState copyWith({
    List<Channel>? channels,
    bool? hasMore,
    bool? isLoadingMore,
  }) =>
      ChannelListState(
        channels: channels ?? this.channels,
        hasMore: hasMore ?? this.hasMore,
        isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      );
}

class ChannelListNotifier extends AsyncNotifier<ChannelListState> {
  static const pageSize = 500;

  @override
  Future<ChannelListState> build() async {
    // Watch del filtro: cualquier cambio (búsqueda, país, etc.) re-ejecuta
    // build() y por tanto resetea la lista a la primera página.
    final filter = ref.watch(channelFilterProvider);
    final repo = ref.watch(channelRepositoryProvider);

    final page = await repo.getChannels(filter: filter, limit: pageSize);
    return ChannelListState(
      channels: page,
      hasMore: page.length == pageSize,
    );
  }

  /// Añade la siguiente página. Reentrante-seguro: los loadMore disparados
  /// varias veces por el scroll mientras uno está en vuelo no duplican.
  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || !current.hasMore || current.isLoadingMore) return;

    state = AsyncData(current.copyWith(isLoadingMore: true));
    try {
      final filter = ref.read(channelFilterProvider);
      final repo = ref.read(channelRepositoryProvider);
      final page = await repo.getChannels(
        filter: filter,
        limit: pageSize,
        offset: current.channels.length,
      );
      state = AsyncData(ChannelListState(
        channels: [...current.channels, ...page],
        hasMore: page.length == pageSize,
      ));
    } catch (_) {
      // Conservar lo ya cargado; el usuario puede reintentar con más scroll
      state = AsyncData(current.copyWith(isLoadingMore: false));
    }
  }
}

final channelListProvider =
    AsyncNotifierProvider<ChannelListNotifier, ChannelListState>(
  ChannelListNotifier.new,
);

final streamUrlProvider =
    FutureProvider.family<String, String>((ref, channelId) async {
  final repo = ref.watch(channelRepositoryProvider);
  return repo.getStreamUrl(channelId);
});
