import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'channel_provider.dart';

enum ViewMode { grid, list }

/// Modo de vista, persistido. La rejilla es el default —aprovecha mejor el
/// espacio y los logos son el ancla visual— pero la lista sigue siendo mejor
/// para escanear por nombre y para los canales sin logo, así que se conserva.
class ViewModeNotifier extends Notifier<ViewMode> {
  static const _key = 'view_mode';

  @override
  ViewMode build() {
    final v = ref.watch(sharedPreferencesProvider).getString(_key);
    return v == 'list' ? ViewMode.list : ViewMode.grid;
  }

  void toggle() {
    state = state == ViewMode.grid ? ViewMode.list : ViewMode.grid;
    ref
        .read(sharedPreferencesProvider)
        .setString(_key, state == ViewMode.list ? 'list' : 'grid');
  }
}

final viewModeProvider =
    NotifierProvider<ViewModeNotifier, ViewMode>(ViewModeNotifier.new);
