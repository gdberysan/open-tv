import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'package:korven_open_tv/data/api_error.dart';

void main() {
  test('el gateway caído se explica, no se vuelca', () async {
    // Rechazo real en vez de un ClientException construido a mano: package:http
    // no lanza ese tipo a secas al encontrarse el puerto cerrado, sino una
    // _ClientSocketException —privada— que extiende ClientException y a la vez
    // implementa SocketException. El test anterior fabricaba un tipo que en
    // producción no se da nunca, y por eso daba por bueno un mensaje que el
    // usuario no llegaba a ver.
    final error = await _errorDePuertoCerrado();

    expect(error, isA<http.ClientException>());
    expect(error, isA<SocketException>(),
        reason: 'si esto deja de cumplirse, el orden de ApiError.desde '
            'ya no importa y este test sobra');

    final err = ApiError.desde(error);
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

  test('el ApiError de un puerto cerrado nombra el puerto que hay que mirar',
      () async {
    final err = ApiError.desde(await _errorDePuertoCerrado());
    expect(err.mensaje, contains('8080'));
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

/// Provoca el error que da package:http cuando nadie escucha al otro lado.
///
/// Abre un puerto efímero solo para saber cuál está libre y lo cierra antes de
/// llamar: así el rechazo es del sistema, no simulado, y no hace falta red.
Future<Object> _errorDePuertoCerrado() async {
  final sonda = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
  final puerto = sonda.port;
  await sonda.close();

  final cliente = http.Client();
  try {
    await cliente.get(Uri.parse('http://127.0.0.1:$puerto/channels'));
    fail('el puerto $puerto debía rechazar la conexión, pero respondió');
  } catch (e) {
    if (e is TestFailure) rethrow;
    return e;
  } finally {
    cliente.close();
  }
}
