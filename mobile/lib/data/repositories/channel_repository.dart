import 'dart:convert';
import 'package:http/http.dart' as http;
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';

abstract class IChannelRepository {
  Future<List<Channel>> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit,
    int offset,
  });
  Future<String> getStreamUrl(String channelId);
}

class ChannelRepository implements IChannelRepository {
  /// URL del gateway. Inyectable por constructor y sobreescribible en build
  /// con --dart-define=GATEWAY_URL=http://host:puerto
  static const _defaultBaseUrl = String.fromEnvironment(
    'GATEWAY_URL',
    defaultValue: 'http://127.0.0.1:8080',
  );

  final Uri _base;
  final http.Client client;

  ChannelRepository({String? baseUrl, http.Client? client})
      : _base = Uri.parse(baseUrl ?? _defaultBaseUrl),
        client = client ?? http.Client();

  Uri _endpoint(String path, Map<String, String> params) =>
      _base.replace(path: path, queryParameters: params);

  @override
  Future<List<Channel>> getChannels({
    ChannelFilter filter = const ChannelFilter(),
    int limit = 500,
    int offset = 0,
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

    final response = await client.get(_endpoint('/channels', params));

    if (response.statusCode == 200) {
      // El gateway (Go) serializa un slice nil como `null`, no como `[]`
      final data = json.decode(response.body) as List<dynamic>? ?? const [];
      return data
          .map((j) => Channel.fromJson(j as Map<String, dynamic>))
          .toList();
    }
    throw Exception('Failed to load channels: ${response.statusCode}');
  }

  @override
  Future<String> getStreamUrl(String channelId) async {
    // Usamos query param ?id= (no path param) porque los IDs pueden contener
    // "/" (ej: "24/7 News"), lo que rompe el routing de path segments.
    final response =
        await client.get(_endpoint('/channels/stream', {'id': channelId}));

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return data['url'] as String;
    }
    throw Exception('Failed to get stream url: ${response.statusCode}');
  }
}
