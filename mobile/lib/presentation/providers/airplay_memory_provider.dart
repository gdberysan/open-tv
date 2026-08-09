import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'channel_provider.dart';

/// Canales que AVFoundation ya rechazó por formato, como conjunto de IDs en
/// SharedPreferences. Mismo patrón que FavoritesNotifier: para un conjunto de
/// identificadores no hace falta sqflite.
///
/// Vive en el cliente y no en el gateway a propósito. Los handlers reciben un
/// pool de solo lectura, y abrir un endpoint de escritura en una API sin
/// autenticación para guardar esto sería un mal cambio. Además es la única
/// fuente que tendrá datos: sin relleno de fondo, /channels no trae
/// compatibilidad para casi ningún canal.
class AirplayMemoryNotifier extends Notifier<Set<String>> {
  static const _key = 'airplay_incompatibles';

  @override
  Set<String> build() =>
      ref.watch(sharedPreferencesProvider).getStringList(_key)?.toSet() ??
      <String>{};

  void marcarIncompatible(String channelId) {
    if (state.contains(channelId)) return;
    final nuevo = Set<String>.from(state)..add(channelId);
    state = nuevo;
    ref.read(sharedPreferencesProvider).setStringList(_key, nuevo.toList());
  }

  bool esIncompatible(String channelId) => state.contains(channelId);
}

final airplayMemoryProvider =
    NotifierProvider<AirplayMemoryNotifier, Set<String>>(
  AirplayMemoryNotifier.new,
);
