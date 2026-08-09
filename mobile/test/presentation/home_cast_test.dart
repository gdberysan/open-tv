import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';

void main() {
  test('con sesión interceptando, el toque emite en vez de abrir el player', () {
    // El comportamiento condicional del toque vive en CastSession.intercepta:
    // home_screen lo consulta en _abrirCanal. Verificarlo aquí, sobre el
    // modelo, evita montar el árbol entero con gateway falso.
    const armada = CastSession(state: CastState.armed, deviceName: 'Salón');
    const emitiendo = CastSession(state: CastState.casting, deviceName: 'Salón');
    const parada = CastSession();
    const fallida = CastSession(state: CastState.failed, error: 'x');

    expect(armada.intercepta, isTrue);
    expect(emitiendo.intercepta, isTrue);
    expect(parada.intercepta, isFalse);
    expect(fallida.intercepta, isFalse,
        reason: 'tras un traspaso el canal va a local, no al televisor');
  });
}
