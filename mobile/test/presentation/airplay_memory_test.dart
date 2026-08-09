import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/presentation/providers/airplay_memory_provider.dart';
import 'package:korven_open_tv/presentation/providers/channel_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<ProviderContainer> contenedor(
      [Map<String, Object> inicial = const {}]) async {
    SharedPreferences.setMockInitialValues(inicial);
    final prefs = await SharedPreferences.getInstance();
    return ProviderContainer(
      overrides: [sharedPreferencesProvider.overrideWithValue(prefs)],
    );
  }

  test('arranca vacío', () async {
    final c = await contenedor();
    expect(c.read(airplayMemoryProvider), isEmpty);
  });

  test('marcar persiste y se consulta', () async {
    final c = await contenedor();
    c.read(airplayMemoryProvider.notifier).marcarIncompatible('bbc');
    expect(c.read(airplayMemoryProvider), contains('bbc'));
    expect(c.read(airplayMemoryProvider.notifier).esIncompatible('bbc'), isTrue);
    expect(
        c.read(airplayMemoryProvider.notifier).esIncompatible('cnn'), isFalse);
  });

  test('lee lo persistido en un arranque anterior', () async {
    final c = await contenedor({
      'airplay_incompatibles': <String>['cnn'],
    });
    expect(c.read(airplayMemoryProvider), contains('cnn'));
  });

  test('marcar dos veces no duplica ni rompe', () async {
    final c = await contenedor();
    final n = c.read(airplayMemoryProvider.notifier);
    n.marcarIncompatible('bbc');
    n.marcarIncompatible('bbc');
    expect(c.read(airplayMemoryProvider).length, 1);
  });
}
