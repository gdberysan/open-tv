import 'dart:async';
import 'dart:io';

import 'package:http/http.dart' as http;

/// Error de la capa de datos con un mensaje apto para enseñar al usuario.
///
/// Sin esto las pantallas pintan e.toString() y el usuario acaba leyendo
/// "ClientException with SocketException: Connection refused (OS Error:
/// Connection refused, errno = 61), address = 127.0.0.1, port = 8080".
/// VoiceOver además lo lee literal.
class ApiError implements Exception {
  const ApiError(this.mensaje);

  final String mensaje;

  factory ApiError.desde(Object error) {
    if (error is ApiError) return error;

    if (error is TimeoutException) {
      return const ApiError('El gateway tardó demasiado en responder.');
    }
    if (error is SocketException) {
      return const ApiError('Sin conexión de red.');
    }
    if (error is http.ClientException) {
      return const ApiError(
        'No se pudo contactar con el gateway. ¿Está arrancado en el puerto 8080?',
      );
    }
    return const ApiError('Ha ocurrido un error inesperado.');
  }

  /// Construye el error de una respuesta HTTP no exitosa, aprovechando el
  /// cuerpo {"error": "..."} que devuelve el gateway.
  factory ApiError.deRespuesta(int statusCode, String? detalle) {
    if (statusCode == 404) {
      return const ApiError('No disponible.');
    }
    if (statusCode >= 500) {
      return ApiError(detalle != null && detalle.isNotEmpty
          ? detalle
          : 'El gateway ha fallado. Inténtalo de nuevo.');
    }
    return ApiError(detalle != null && detalle.isNotEmpty
        ? detalle
        : 'La petición no se pudo completar ($statusCode).');
  }

  @override
  String toString() => mensaje;
}
