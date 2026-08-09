import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'channel_provider.dart';

/// Favoritos como conjunto de IDs en SharedPreferences. No hace falta sqflite
/// para un conjunto de identificadores.
class FavoritesNotifier extends Notifier<Set<String>> {
  static const _key = 'favorites';

  @override
  Set<String> build() =>
      ref.watch(sharedPreferencesProvider).getStringList(_key)?.toSet() ??
      <String>{};

  void toggle(String channelId) {
    final nuevo = Set<String>.from(state);
    if (!nuevo.remove(channelId)) nuevo.add(channelId);
    state = nuevo;
    ref.read(sharedPreferencesProvider).setStringList(_key, nuevo.toList());
  }

  bool esFavorito(String channelId) => state.contains(channelId);
}

final favoritesProvider =
    NotifierProvider<FavoritesNotifier, Set<String>>(FavoritesNotifier.new);
