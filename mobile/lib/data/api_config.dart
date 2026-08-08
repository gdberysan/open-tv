/// Configuración de acceso al gateway, compartida por todos los repositorios.
///
/// Vivía duplicada en channel_repository.dart y epg_repository.dart.
class ApiConfig {
  const ApiConfig._();

  /// URL del gateway. Sobreescribible en build con
  /// --dart-define=GATEWAY_URL=http://host:puerto
  static const String baseUrl = String.fromEnvironment(
    'GATEWAY_URL',
    defaultValue: 'http://127.0.0.1:8080',
  );

  /// Deadline por petición. Sin esto, un gateway que acepta la conexión TCP y
  /// no responde deja cualquier pantalla cargando para siempre: http.Client no
  /// impone ningún límite por su cuenta.
  static const Duration timeout = Duration(seconds: 10);
}
