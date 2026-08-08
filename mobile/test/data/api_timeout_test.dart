import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import 'package:iptv_ecosystem/data/api_error.dart';
import 'package:iptv_ecosystem/data/repositories/channel_repository.dart';
import 'package:iptv_ecosystem/data/repositories/epg_repository.dart';

/// Cliente que acepta la petición y no responde jamás: reproduce un gateway
/// que acepta el TCP y se queda colgado, que es el caso que la app no cubría.
class ClienteQueNuncaResponde extends http.BaseClient {
  final _nunca = Completer<http.StreamedResponse>();

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) => _nunca.future;
}

void main() {
  final tardo = isA<ApiError>().having(
    (e) => e.mensaje.toLowerCase(),
    'mensaje',
    contains('tardó'),
  );

  test('getChannels aborta si el gateway no responde', () {
    final repo = ChannelRepository(
      baseUrl: 'http://127.0.0.1:9',
      client: ClienteQueNuncaResponde(),
      timeout: const Duration(milliseconds: 50),
    );
    expect(() => repo.getChannels(), throwsA(tardo));
  });

  test('getStreamUrl aborta si el gateway no responde', () {
    final repo = ChannelRepository(
      baseUrl: 'http://127.0.0.1:9',
      client: ClienteQueNuncaResponde(),
      timeout: const Duration(milliseconds: 50),
    );
    expect(() => repo.getStreamUrl('ch-1'), throwsA(tardo));
  });

  test('el EPG aborta si el gateway no responde', () {
    final repo = EPGRepository(
      baseUrl: 'http://127.0.0.1:9',
      client: ClienteQueNuncaResponde(),
      timeout: const Duration(milliseconds: 50),
    );
    expect(
      () => repo.getForChannel('ch-1',
          from: DateTime(2026), to: DateTime(2026, 1, 2)),
      throwsA(tardo),
    );
  });
}
