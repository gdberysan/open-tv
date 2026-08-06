import 'dart:convert';
import 'package:http/http.dart' as http;
import '../../domain/models/epg_entry.dart';

abstract class IEPGRepository {
  /// Programación de un canal que solapa la ventana [from, to).
  Future<List<EPGEntry>> getForChannel(String channelId,
      {required DateTime from, required DateTime to});
}

class EPGRepository implements IEPGRepository {
  static const _defaultBaseUrl = String.fromEnvironment(
    'GATEWAY_URL',
    defaultValue: 'http://127.0.0.1:8080',
  );

  final Uri _base;
  final http.Client client;

  EPGRepository({String? baseUrl, http.Client? client})
      : _base = Uri.parse(baseUrl ?? _defaultBaseUrl),
        client = client ?? http.Client();

  @override
  Future<List<EPGEntry>> getForChannel(String channelId,
      {required DateTime from, required DateTime to}) async {
    final uri = _base.replace(path: '/epg', queryParameters: {
      'channel': channelId,
      'from': from.toUtc().toIso8601String(),
      'to': to.toUtc().toIso8601String(),
    });
    final response = await client.get(uri);

    if (response.statusCode == 200) {
      final data = json.decode(response.body) as List<dynamic>? ?? const [];
      return data
          .map((j) => EPGEntry.fromJson(j as Map<String, dynamic>))
          .toList();
    }
    throw Exception('Failed to load EPG: ${response.statusCode}');
  }
}
