import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:iptv_ecosystem/presentation/player/playback_guard.dart';

void main() {
  PlaybackGuard guard({required List<String> fatals}) => PlaybackGuard(
        onFatal: fatals.add,
        loadTimeout: const Duration(seconds: 15),
        stallTimeout: const Duration(seconds: 8),
      );

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

  test('cuando empieza a reproducir se desarma el timeout de carga', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      g.armLoadTimeout();
      async.elapse(const Duration(seconds: 5));
      g.onPlaying(true);
      async.elapse(const Duration(seconds: 30));

      expect(fatals, isEmpty);
    });
  });

  // El caso A&E: AVERROR_EOF transitorio en HLS en vivo con el playback
  // avanzando — NO debe matar el vídeo.
  test('error transitorio mientras avanza la posición se ignora', () {
    fakeAsync((async) {
      final fatals = <String>[];
      final g = guard(fatals: fatals);

      g.onPlaying(true);
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

      g.onPlaying(true);
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

      g.onPlaying(true);
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
