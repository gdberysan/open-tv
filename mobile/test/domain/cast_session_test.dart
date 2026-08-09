import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';

void main() {
  test('idle no se dibuja ni intercepta toques', () {
    const s = CastSession();
    expect(s.visible, isFalse);
    expect(s.intercepta, isFalse);
  });

  test('armed se dibuja e intercepta: hay dispositivo esperando', () {
    const s = CastSession(state: CastState.armed, deviceName: 'Salón Apple TV');
    expect(s.visible, isTrue);
    expect(s.intercepta, isTrue);
    expect(s.etiquetaDispositivo, 'Salón Apple TV');
  });

  test('casting se dibuja e intercepta', () {
    const s = CastSession(state: CastState.casting, channelName: 'BBC News');
    expect(s.visible, isTrue);
    expect(s.intercepta, isTrue);
  });

  test('failed se dibuja pero ya no intercepta: el traspaso va a local', () {
    const s = CastSession(state: CastState.failed, error: 'sin códec');
    expect(s.visible, isTrue);
    expect(s.intercepta, isFalse);
  });

  test('sin nombre de dispositivo la etiqueta cae a AirPlay', () {
    const s = CastSession(state: CastState.armed);
    expect(s.etiquetaDispositivo, 'AirPlay');
  });

  test('copyWith limpia el error cuando se le pide', () {
    const s = CastSession(state: CastState.failed, error: 'x');
    final limpio = s.copyWith(state: CastState.armed, clearError: true);
    expect(limpio.error, isNull);
    expect(limpio.state, CastState.armed);
  });
}
