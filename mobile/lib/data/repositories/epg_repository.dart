import 'dart:convert';
import 'package:http/http.dart' as http;
import '../api_config.dart';
import '../api_error.dart';
import '../../domain/models/epg_entry.dart';

abstract class IEPGRepository {
  /// Programación de un canal que solapa la ventana [from, to).
  Future<List<EPGEntry>> getForChannel(String channelId,
      {required DateTime from, required DateTime to});
}

class EPGRepository implements IEPGRepository {
  final Uri _base;
  final http.Client client;

  /// Inyectable para que los tests no esperen el deadline real de 10 s.
  final Duration timeout;

  EPGRepository({String? baseUrl, http.Client? client, Duration? timeout})
      : _base = Uri.parse(baseUrl ?? ApiConfig.baseUrl),
        client = client ?? http.Client(),
        timeout = timeout ?? ApiConfig.timeout;

  @override
  Future<List<EPGEntry>> getForChannel(String channelId,
      {required DateTime from, required DateTime to}) async {
    final uri = _base.replace(path: '/epg', queryParameters: {
      'channel': channelId,
      'from': from.toUtc().toIso8601String(),
      'to': to.toUtc().toIso8601String(),
    });

    final http.Response response;
    try {
      response = await client.get(uri).timeout(timeout);
    } catch (e) {
      throw ApiError.desde(e);
    }

    if (response.statusCode == 200) {
      final data = json.decode(response.body) as List<dynamic>? ?? const [];
      return data
          .map((j) => EPGEntry.fromJson(j as Map<String, dynamic>))
          .toList();
    }
    throw ApiError.deRespuesta(response.statusCode, null);
  }
}
