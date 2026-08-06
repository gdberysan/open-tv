import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:iptv_ecosystem/data/repositories/epg_repository.dart';

void main() {
  MockClient clientRespondiendo(String body,
      {int status = 200, void Function(http.Request)? onRequest}) {
    return MockClient((request) async {
      onRequest?.call(request);
      return http.Response(body, status);
    });
  }

  const entradaJson =
      '{"ChannelID":"opensource-BBC News","Title":"Noticias","Description":"",'
      '"StartAt":"2026-08-06T14:00:00Z","EndAt":"2026-08-06T15:00:00Z"}';

  test('pide /epg con channel, from y to en RFC3339', () async {
    Uri? visto;
    final repo = EPGRepository(
      baseUrl: 'http://gw.local:9',
      client: clientRespondiendo('[$entradaJson]', onRequest: (r) => visto = r.url),
    );

    final from = DateTime.utc(2026, 8, 6, 14);
    final to = DateTime.utc(2026, 8, 6, 20);
    final entries = await repo.getForChannel('opensource-BBC News', from: from, to: to);

    expect(visto!.host, 'gw.local');
    expect(visto!.path, '/epg');
    expect(visto!.queryParameters['channel'], 'opensource-BBC News');
    expect(DateTime.parse(visto!.queryParameters['from']!), from);
    expect(DateTime.parse(visto!.queryParameters['to']!), to);
    expect(entries, hasLength(1));
    expect(entries.first.title, 'Noticias');
  });

  test('gateway sin datos devuelve lista vacía', () async {
    final repo = EPGRepository(
      baseUrl: 'http://x',
      client: clientRespondiendo('[]'),
    );

    final entries = await repo.getForChannel('x',
        from: DateTime.now(), to: DateTime.now().add(const Duration(hours: 6)));

    expect(entries, isEmpty);
  });

  test('lanza en respuesta no-200', () {
    final repo = EPGRepository(
      baseUrl: 'http://x',
      client: clientRespondiendo('boom', status: 500),
    );

    expect(
      repo.getForChannel('x',
          from: DateTime.now(), to: DateTime.now().add(const Duration(hours: 1))),
      throwsException,
    );
  });
}
