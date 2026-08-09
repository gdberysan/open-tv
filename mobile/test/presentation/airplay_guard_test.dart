import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/data/airplay/airplay_platform.dart';
import 'package:korven_open_tv/presentation/player/airplay_guard.dart';

void main() {
  test('sin reproducción dentro del presupuesto, fatal', () {
    fakeAsync((async) {
      String? fatal;
      final g = AirplayGuard(onFatal: (m, {formatError = false}) => fatal = m);
      g.armLoadTimeout();

      async.elapse(const Duration(seconds: 14));
      expect(fatal, isNull, reason: 'aún dentro del presupuesto');

      async.elapse(const Duration(seconds: 2));
      expect(fatal, isNotNull);
      g.dispose();
    });
  });

  test('playing confirma y desarma el watchdog', () {
    fakeAsync((async) {
      String? fatal;
      var confirmado = 0;
      final g = AirplayGuard(
        onFatal: (m, {formatError = false}) => fatal = m,
        onPlaybackConfirmed: () => confirmado++,
      );
      g.armLoadTimeout();
      g.onStatus(const AirplayStatusEvent(state: AirplayPlaybackState.playing));

      async.elapse(const Duration(seconds: 30));
      expect(fatal, isNull);
      expect(confirmado, 1);
      g.dispose();
    });
  });

  test('un fallo es terminal a la primera, incluso ya reproduciendo', () {
    fakeAsync((async) {
      String? fatal;
      final g = AirplayGuard(onFatal: (m, {formatError = false}) => fatal = m);
      g.armLoadTimeout();
      g.onStatus(const AirplayStatusEvent(state: AirplayPlaybackState.playing));
      g.onStatus(const AirplayStatusEvent(
          state: AirplayPlaybackState.failed, error: 'roto'));

      expect(fatal, 'roto',
          reason: 'AVPlayer no emite errores transitorios como mpv');
      g.dispose();
    });
  });

  test('propaga si el fallo fue de formato', () {
    fakeAsync((async) {
      bool? formato;
      final g = AirplayGuard(
          onFatal: (m, {formatError = false}) => formato = formatError);
      g.armLoadTimeout();
      g.onStatus(const AirplayStatusEvent(
        state: AirplayPlaybackState.failed,
        error: 'códec',
        formatError: true,
      ));
      expect(formato, isTrue);
      g.dispose();
    });
  });

  test('tras dispose no llama a nadie', () {
    fakeAsync((async) {
      var llamadas = 0;
      final g = AirplayGuard(onFatal: (m, {formatError = false}) => llamadas++);
      g.armLoadTimeout();
      g.dispose();
      async.elapse(const Duration(seconds: 30));
      expect(llamadas, 0);
    });
  });
}
