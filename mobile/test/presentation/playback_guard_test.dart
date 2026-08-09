import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:korven_open_tv/presentation/player/playback_guard.dart';

void main() {
  PlaybackGuard guard({required List<String> fatals}) => PlaybackGuard(
        onFatal: fatals.add,
        loadTimeout: const Duration(seconds: 15),
        stallTimeout: const Duration(seconds: 8),
      );

  /// Deja al guard en estado "reproduciendo de verdad": hacen falta dos
  /// posiciones distintas, porque una sola es solo la referencia.
  void reproduciendo(PlaybackGuard g) {
    g.onPlaying(true);
    g.onPosition(Duration.zero);
    g.onPosition(const Duration(milliseconds: 40));
  }

  test('error antes de reproducir es fatal inmediato', () {
    final fatals = <String>[];
    final g = guard(fatals: fatals);

    g.onError('Failed to open stream');

    expect(fatals, ['Failed to open stream']);
  });

  test('si nada reproduce en loadTimeout es fatal', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      g.armLoadTimeout();
      async.elapse(const Duration(seconds: 16));

      expect(fatals, hasLength(1));
      expect(fatals.first, contains('15'));
    });
  });

  test('cuando la posición avanza se desarma el timeout de carga', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      g.armLoadTimeout();
      async.elapse(const Duration(seconds: 5));
      g.onPlaying(true);
      g.onPosition(Duration.zero);
      g.onPosition(const Duration(milliseconds: 200));
      async.elapse(const Duration(seconds: 30));

      expect(fatals, isEmpty);
    });
  });

  // El bug de "Rakuten TV Romance Movies Italy": el manifiesto y la clave AES
  // se sirven bien, pero el CDN devuelve 405 a cada segmento fuera de Italia.
  // libavformat se queda bloqueado reintentando y NO emite ningún error, así
  // que el guard nunca oye un fallo. media_kit, mientras tanto, emite
  // playing=true dentro de open() y en MPV_EVENT_START_FILE — antes de
  // decodificar nada. Fiarse de esa señal desarmaba el watchdog y dejaba la
  // pantalla cargando para siempre.
  test('playing=true sin avance de posición no desarma el watchdog', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      g.armLoadTimeout();
      g.onPlaying(true); // media_kit lo emite nada más pedir la reproducción
      async.elapse(const Duration(seconds: 16));

      expect(fatals, hasLength(1),
          reason: 'sin fotogramas no hay reproducción, por mucho playing=true');
    });
  });

  test('una posición que se repite no cuenta como reproducir', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      g.armLoadTimeout();
      g.onPlaying(true);
      // mpv puede repetir la misma posición mientras está atascado.
      for (var i = 0; i < 5; i++) {
        g.onPosition(Duration.zero);
        async.elapse(const Duration(seconds: 2));
      }
      async.elapse(const Duration(seconds: 10));

      expect(fatals, hasLength(1));
    });
  });

  test('avisa cuando la reproducción queda confirmada', () {
    fakeAsync((async) {
      final confirmadas = <int>[];
      final g = PlaybackGuard(
        onFatal: (_) {},
        onPlaybackConfirmed: () => confirmadas.add(1),
      );

      g.armLoadTimeout();
      g.onPosition(Duration.zero);
      expect(confirmadas, isEmpty, reason: 'la primera muestra es referencia');

      g.onPosition(const Duration(milliseconds: 500));
      expect(confirmadas, hasLength(1));

      // No se repite en cada avance posterior.
      g.onPosition(const Duration(seconds: 1));
      expect(confirmadas, hasLength(1));
      async.elapse(const Duration(seconds: 30));
    });
  });

  // El caso A&E: AVERROR_EOF transitorio en HLS en vivo con el playback
  // avanzando — NO debe matar el vídeo.
  test('error transitorio mientras avanza la posición se ignora', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      reproduciendo(g);
      g.onPosition(const Duration(seconds: 1));
      g.onError('tcp: ffurl_read returned 0xdfb9b0bb');

      // La posición sigue avanzando durante la ventana de vigilancia
      async.elapse(const Duration(seconds: 4));
      g.onPosition(const Duration(seconds: 5));
      async.elapse(const Duration(seconds: 10));

      expect(fatals, isEmpty);
    });
  });

  test('error con la posición congelada durante stallTimeout es fatal', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      reproduciendo(g);
      g.onPosition(const Duration(seconds: 3));
      g.onError('tcp: connection reset');

      async.elapse(const Duration(seconds: 9)); // sin avances de posición

      expect(fatals, ['tcp: connection reset']);
    });
  });

  test('errores repetidos durante la vigilancia no re-arman el reloj', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      reproduciendo(g);
      g.onPosition(const Duration(seconds: 1));
      g.onError('primero');
      async.elapse(const Duration(seconds: 6));
      g.onError('segundo'); // no debe reiniciar la ventana de 8s

      async.elapse(const Duration(seconds: 3)); // 9s desde el primero

      expect(fatals, hasLength(1));
    });
  });

  test('dispose cancela cualquier vigilancia pendiente', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      g.armLoadTimeout();
      g.dispose();
      async.elapse(const Duration(seconds: 30));

      expect(fatals, isEmpty);
    });
  });
}
