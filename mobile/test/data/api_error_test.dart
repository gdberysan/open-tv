import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'package:korven_open_tv/data/api_error.dart';

void main() {
  test('el gateway caído se explica, no se vuelca', () {
    final err = ApiError.desde(
      http.ClientException(
          'Connection refused', Uri.parse('http://127.0.0.1:8080/channels')),
    );
    expect(err.mensaje, contains('gateway'));
    expect(err.mensaje, isNot(contains('errno')));
    expect(err.mensaje, isNot(contains('SocketException')));
  });

  test('el timeout se explica', () {
    final err = ApiError.desde(TimeoutException('agotado'));
    expect(err.mensaje.toLowerCase(), contains('tardó'));
    expect(err.mensaje, isNot(contains('TimeoutException')));
  });

  test('sin red se distingue del gateway caído', () {
    final err = ApiError.desde(const SocketException('Network is unreachable'));
    expect(err.mensaje.toLowerCase(), contains('conexión'));
  });

  test('un ApiError existente no se re-envuelve', () {
    const original = ApiError('mensaje propio');
    expect(identical(ApiError.desde(original), original), isTrue);
  });

  test('un 404 no se muestra como fallo del gateway', () {
    final err = ApiError.deRespuesta(404, null);
    expect(err.mensaje.toLowerCase(), contains('no disponible'));
  });

  test('un 5xx aprovecha el cuerpo {"error": ...} si lo hay', () {
    final err = ApiError.deRespuesta(500, 'Error obteniendo canales');
    expect(err.mensaje, isNotEmpty);
    expect(err.mensaje, isNot(contains('500')));
  });

  test('toString devuelve el mensaje legible, no el nombre del tipo', () {
    const err = ApiError('algo legible');
    expect(err.toString(), 'algo legible');
  });
}
