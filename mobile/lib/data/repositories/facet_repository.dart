import 'dart:convert';
import 'package:http/http.dart' as http;
import '../api_config.dart';
import '../api_error.dart';

/// Un valor de filtro con su número de canales. Lo que alimenta a los
/// selectores: enseñar el recuento evita elegir una opción que no tiene nada.
class Faceta {
  const Faceta({required this.valor, required this.count});

  final String valor;
  final int count;

  factory Faceta.fromJson(Map<String, dynamic> j) => Faceta(
        valor: j['Valor'] as String? ?? '',
        count: (j['Count'] as num?)?.toInt() ?? 0,
      );
}

abstract class IFacetRepository {
  Future<List<Faceta>> countries();
  Future<List<Faceta>> categories();
}

class FacetRepository implements IFacetRepository {
  FacetRepository({String? baseUrl, http.Client? client, Duration? timeout})
      : _base = Uri.parse(baseUrl ?? ApiConfig.baseUrl),
        client = client ?? http.Client(),
        timeout = timeout ?? ApiConfig.timeout;

  final Uri _base;
  final http.Client client;
  final Duration timeout;

  Future<List<Faceta>> _facetas(String path) async {
    final http.Response r;
    try {
      r = await client.get(_base.replace(path: path)).timeout(timeout);
    } catch (e) {
      throw ApiError.desde(e);
    }
    if (r.statusCode != 200) {
      throw ApiError.deRespuesta(r.statusCode, null);
    }
    final data = json.decode(r.body) as List<dynamic>? ?? const [];
    return data
        .map((j) => Faceta.fromJson(j as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<List<Faceta>> countries() => _facetas('/channels/countries');

  @override
  Future<List<Faceta>> categories() => _facetas('/channels/categories');
}
