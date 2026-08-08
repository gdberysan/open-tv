import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';
import '../../data/repositories/channel_repository.dart';

final channelRepositoryProvider = Provider<IChannelRepository>((ref) {
  return ChannelRepository();
});

/// Instancia de SharedPreferences cargada en main() antes de runApp.
final sharedPreferencesProvider = Provider<SharedPreferences>(
  (_) => throw UnimplementedError('override en main() con la instancia real'),
);

/// Preferencia persistida "mostrar canales offline" (Fase 7.2).
/// El gateway oculta los muertos por defecto; esto lo desactiva.
class ShowOfflineNotifier extends Notifier<bool> {
  static const _key = 'show_offline';

  @override
  bool build() =>
      ref.watch(sharedPreferencesProvider).getBool(_key) ?? false;

  void toggle() {
    state = !state;
    ref.read(sharedPreferencesProvider).setBool(_key, state);
  }
}

final showOfflineProvider =
    NotifierProvider<ShowOfflineNotifier, bool>(ShowOfflineNotifier.new);

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

  /// Generación del filtro vigente. build() la incrementa; una continuación de
  /// loadMore que vuelva de su await con una generación vieja se descarta en
  /// vez de sobrescribir la lista del filtro nuevo. Riverpod no recrea el
  /// notifier al re-ejecutar build(), así que el campo sobrevive al cambio —
  /// que es justo lo que hacía posible la carrera: buscar mientras se scrollea
  /// mostraba los canales del filtro anterior mezclados con una página del
  /// nuevo.
  int _generacion = 0;

  /// Filtro efectivo: el transitorio (búsqueda, país…) + la preferencia
  /// persistida de mostrar canales offline.
  ChannelFilter _effectiveFilter() => ref
      .read(channelFilterProvider)
      .copyWith(showOffline: ref.read(showOfflineProvider));

  @override
  Future<ChannelListState> build() async {
    // Watch del filtro y la preferencia: cualquier cambio re-ejecuta
    // build() y por tanto resetea la lista a la primera página.
    ref.watch(channelFilterProvider);
    ref.watch(showOfflineProvider);
    final repo = ref.watch(channelRepositoryProvider);

    final generacion = ++_generacion;
    final page =
        await repo.getChannels(filter: _effectiveFilter(), limit: pageSize);
    if (generacion != _generacion) {
      // Otro build arrancó mientras este esperaba: manda el suyo.
      return state.valueOrNull ??
          const ChannelListState(channels: [], hasMore: false);
    }
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

    final generacion = _generacion;
    state = AsyncData(current.copyWith(isLoadingMore: true));
    try {
      final repo = ref.read(channelRepositoryProvider);
      final page = await repo.getChannels(
        filter: _effectiveFilter(),
        limit: pageSize,
        offset: current.channels.length,
      );
      if (generacion != _generacion) return;
      state = AsyncData(ChannelListState(
        channels: [...current.channels, ...page],
        hasMore: page.length == pageSize,
      ));
    } catch (_) {
      if (generacion != _generacion) return;
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
