import 'dart:convert';
import 'package:http/http.dart' as http;
import '../api_config.dart';
import '../api_error.dart';
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';

/// Una página de canales más el total de coincidencias del filtro. El total lo
/// da el gateway en X-Total-Count; sin él, la app solo sabría cuántos lleva
/// cargados y el contador de la barra de filtros mentiría.
class ChannelPage {
  const ChannelPage({required this.channels, required this.total});
  final List<Channel> channels;
  final int total;
}

abstract class IChannelRepository {
  Future<ChannelPage> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit,
    int offset,
    List<String>? ids,
  });
  Future<String> getStreamUrl(String channelId);

  /// Un canal al azar que case con el filtro. Lo resuelve el gateway: sortear
  /// entre las páginas cargadas sesgaría hacia el principio del catálogo.
  Future<Channel> getRandomChannel(ChannelFilter filter);
}

class ChannelRepository implements IChannelRepository {
  final Uri _base;
  final http.Client client;

  /// Inyectable para que los tests no esperen el deadline real de 10 s.
  final Duration timeout;

  ChannelRepository({String? baseUrl, http.Client? client, Duration? timeout})
      : _base = Uri.parse(baseUrl ?? ApiConfig.baseUrl),
        client = client ?? http.Client(),
        timeout = timeout ?? ApiConfig.timeout;

  Uri _endpoint(String path, Map<String, String> params) =>
      _base.replace(path: path, queryParameters: params);

  /// Toda petición pasa por aquí: impone el deadline y traduce cualquier fallo
  /// de red a un ApiError con mensaje legible.
  Future<http.Response> _get(Uri uri) async {
    try {
      return await client.get(uri).timeout(timeout);
    } catch (e) {
      throw ApiError.desde(e);
    }
  }

  /// El gateway responde {"error": "..."} en los fallos; aprovecharlo da un
  /// mensaje mejor que el código de estado a secas.
  String? _detalleDeError(http.Response r) {
    try {
      final body = json.decode(r.body);
      if (body is Map<String, dynamic>) return body['error'] as String?;
    } catch (_) {
      // cuerpo no-JSON: no hay detalle que extraer
    }
    return null;
  }

  @override
  Future<ChannelPage> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit = 500,
    int offset = 0,
    List<String>? ids,
  }) async {
    final params = <String, String>{
      'limit': '$limit',
      'offset': '$offset',
    };
    if (filter.quality.isNotEmpty) params['quality'] = filter.quality;
    if (filter.country.isNotEmpty) params['country'] = filter.country;
    if (filter.category.isNotEmpty) params['category'] = filter.category;
    if (filter.query.isNotEmpty) params['q'] = filter.query;
    if (filter.showOffline) params['alive'] = 'all';
    if (ids != null && ids.isNotEmpty) params['ids'] = ids.join(',');

    final response = await _get(_endpoint('/channels', params));

    if (response.statusCode == 200) {
      // El gateway (Go) serializa un slice nil como `null`, no como `[]`
      final data = json.decode(response.body) as List<dynamic>? ?? const [];
      final canales =
          data.map((j) => Channel.fromJson(j as Map<String, dynamic>)).toList();
      // Sin la cabecera caemos a lo recibido: mejor un total bajo que un cero
      // falso o un crash si el gateway es de una versión anterior.
      final total = int.tryParse(response.headers['x-total-count'] ?? '') ??
          canales.length;
      return ChannelPage(channels: canales, total: total);
    }
    throw ApiError.deRespuesta(response.statusCode, _detalleDeError(response));
  }

  @override
  Future<String> getStreamUrl(String channelId) async {
    // Usamos query param ?id= (no path param) porque los IDs pueden contener
    // "/" (ej: "24/7 News"), lo que rompe el routing de path segments.
    final response = await _get(_endpoint('/channels/stream', {'id': channelId}));

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return data['url'] as String;
    }
    throw ApiError.deRespuesta(response.statusCode, _detalleDeError(response));
  }

  @override
  Future<Channel> getRandomChannel(ChannelFilter filter) async {
    final params = <String, String>{};
    if (filter.quality.isNotEmpty) params['quality'] = filter.quality;
    if (filter.country.isNotEmpty) params['country'] = filter.country;
    if (filter.category.isNotEmpty) params['category'] = filter.category;
    if (filter.query.isNotEmpty) params['q'] = filter.query;

    final response = await _get(_endpoint('/channels/random', params));
    if (response.statusCode == 200) {
      return Channel.fromJson(
          json.decode(response.body) as Map<String, dynamic>);
    }
    throw ApiError.deRespuesta(response.statusCode, _detalleDeError(response));
  }
}

