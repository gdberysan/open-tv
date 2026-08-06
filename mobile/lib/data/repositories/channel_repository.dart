import 'dart:convert';
import 'package:http/http.dart' as http;
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';

abstract class IChannelRepository {
  Future<List<Channel>> getChannels({ChannelFilter filter = const ChannelFilter()});
  Future<String> getStreamUrl(String channelId);
}

class ChannelRepository implements IChannelRepository {
  final String baseUrl = 'http://127.0.0.1:8080';
  final http.Client client;

  ChannelRepository({http.Client? client}) : client = client ?? http.Client();

  @override
  Future<List<Channel>> getChannels({ChannelFilter filter = const ChannelFilter()}) async {
    final params = <String, String>{'limit': '500'};
    if (filter.quality.isNotEmpty) params['quality'] = filter.quality;
    if (filter.country.isNotEmpty) params['country'] = filter.country;
    if (filter.category.isNotEmpty) params['category'] = filter.category;
    if (filter.query.isNotEmpty) params['q'] = filter.query;

    final uri = Uri(
      scheme: 'http',
      host: '127.0.0.1',
      port: 8080,
      path: '/channels',
      queryParameters: params,
    );
    final response = await client.get(uri);

    if (response.statusCode == 200) {
      final List<dynamic> data = json.decode(response.body);
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
    final uri = Uri(
      scheme: 'http',
      host: '127.0.0.1',
      port: 8080,
      path: '/channels/stream',
      queryParameters: {'id': channelId},
    );
    final response = await client.get(uri);

    if (response.statusCode == 200) {
      final data = json.decode(response.body);
      return data['url'] as String;
    }
    throw Exception('Failed to get stream url: ${response.statusCode}');
  }
}
