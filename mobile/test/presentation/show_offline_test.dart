import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:korven_open_tv/presentation/providers/channel_provider.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

Future<ProviderContainer> containerConPrefs(
    {Map<String, Object> valores = const {}}) async {
  SharedPreferences.setMockInitialValues(valores);
  final prefs = await SharedPreferences.getInstance();
  final container = ProviderContainer(overrides: [
    sharedPreferencesProvider.overrideWithValue(prefs),
    channelRepositoryProvider.overrideWithValue(FakeRepo(total: 10)),
  ]);
  addTearDown(container.dispose);
  return container;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('por defecto los offline se ocultan (showOffline false)', () async {
    final container = await containerConPrefs();
    expect(container.read(showOfflineProvider), isFalse);
  });

  test('toggle cambia el estado y lo persiste', () async {
    final container = await containerConPrefs();

    container.read(showOfflineProvider.notifier).toggle();

    expect(container.read(showOfflineProvider), isTrue);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getBool('show_offline'), isTrue);
  });

  test('arranca con el valor persistido', () async {
    final container =
        await containerConPrefs(valores: {'show_offline': true});
    expect(container.read(showOfflineProvider), isTrue);
  });

  test('la lista de canales pide alive=all cuando showOffline está activo',
      () async {
    final container = await containerConPrefs();
    final repo =
        container.read(channelRepositoryProvider) as FakeRepo;

    await container.read(channelListProvider.future);
    expect(repo.lastFilter?.showOffline, isFalse);

    container.read(showOfflineProvider.notifier).toggle();
    await container.read(channelListProvider.future);
    expect(repo.lastFilter?.showOffline, isTrue);
  });
}
