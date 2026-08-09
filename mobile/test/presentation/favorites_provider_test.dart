import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/domain/models/channel_filter.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:korven_open_tv/presentation/providers/favorites_provider.dart';
import 'package:korven_open_tv/presentation/providers/view_mode_provider.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

Future<ProviderContainer> _contenedor([Map<String, Object> prefs = const {}]) async {
  SharedPreferences.setMockInitialValues(prefs);
  final sp = await SharedPreferences.getInstance();
  final c = ProviderContainer(
    overrides: [sharedPreferencesProvider.overrideWithValue(sp)],
  );
  addTearDown(c.dispose);
  return c;
}

void main() {
  group('favoritos', () {
    test('empieza vacío', () async {
      final c = await _contenedor();
      expect(c.read(favoritesProvider), isEmpty);
    });

    test('toggle marca y desmarca', () async {
      final c = await _contenedor();
      final n = c.read(favoritesProvider.notifier);

      n.toggle('ch-1');
      expect(c.read(favoritesProvider), {'ch-1'});
      n.toggle('ch-2');
      expect(c.read(favoritesProvider), {'ch-1', 'ch-2'});
      n.toggle('ch-1');
      expect(c.read(favoritesProvider), {'ch-2'});
    });

    test('persisten entre reinicios', () async {
      final c = await _contenedor();
      c.read(favoritesProvider.notifier).toggle('ch-7');

      // Un contenedor nuevo sobre las mismas SharedPreferences simula reabrir
      // la app: si no se leyeran de disco, los favoritos se perderían.
      final sp = await SharedPreferences.getInstance();
      final c2 = ProviderContainer(
        overrides: [sharedPreferencesProvider.overrideWithValue(sp)],
      );
      addTearDown(c2.dispose);

      expect(c2.read(favoritesProvider), {'ch-7'});
    });
  });

  group('modo de vista', () {
    test('la rejilla es el default', () async {
      final c = await _contenedor();
      expect(c.read(viewModeProvider), ViewMode.grid);
    });

    test('el toggle alterna y persiste', () async {
      final c = await _contenedor();
      c.read(viewModeProvider.notifier).toggle();
      expect(c.read(viewModeProvider), ViewMode.list);

      final sp = await SharedPreferences.getInstance();
      expect(sp.getString('view_mode'), 'list');

      final c2 = ProviderContainer(
        overrides: [sharedPreferencesProvider.overrideWithValue(sp)],
      );
      addTearDown(c2.dispose);
      expect(c2.read(viewModeProvider), ViewMode.list);
    });
  });

  group('canal aleatorio', () {
    Future<ProviderContainer> conRepo(FakeRepo repo) async {
      SharedPreferences.setMockInitialValues({});
      final sp = await SharedPreferences.getInstance();
      final c = ProviderContainer(overrides: [
        sharedPreferencesProvider.overrideWithValue(sp),
        channelRepositoryProvider.overrideWithValue(repo),
      ]);
      addTearDown(c.dispose);
      return c;
    }

    test('el sorteo viaja al gateway con los filtros activos', () async {
      // Sortear entre las páginas ya cargadas sesgaría el resultado hacia el
      // principio del catálogo, así que el filtro tiene que ir con la petición.
      final repo = FakeRepo(total: 20);
      final c = await conRepo(repo);
      c.read(channelFilterProvider.notifier).state =
          const ChannelFilter(country: 'MX', category: 'Sports');

      await c.read(randomChannelProvider)();

      expect(repo.lastRandomFilter?.country, 'MX');
      expect(repo.lastRandomFilter?.category, 'Sports');
    });

    test('el sorteo respeta la preferencia de offline', () async {
      final repo = FakeRepo(total: 20);
      final c = await conRepo(repo);
      await c.read(randomChannelProvider)();
      expect(repo.lastRandomFilter?.showOffline, isFalse,
          reason: 'un aleatorio muerto arruina la función');

      c.read(showOfflineProvider.notifier).toggle();
      await c.read(randomChannelProvider)();
      expect(repo.lastRandomFilter?.showOffline, isTrue);
    });
  });
}
