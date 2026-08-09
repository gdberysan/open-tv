import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:iptv_ecosystem/data/repositories/channel_repository.dart';
import 'package:iptv_ecosystem/domain/models/channel.dart';
import 'package:iptv_ecosystem/domain/models/channel_filter.dart';

void main() {
  // Captura la request y responde con el body dado.
  MockClient clientRespondiendo(String body,
      {int status = 200, void Function(http.Request)? onRequest}) {
    return MockClient((request) async {
      onRequest?.call(request);
      return http.Response(body, status);
    });
  }

  const canalJson =
      '{"ID":"opensource-BBC One","Name":"BBC One","LogoURL":"http://l/1.png",'
      '"CategoryID":"News","LanguageCode":"en","CountryCode":"GB",'
      '"ProviderType":"opensource"}';

  group('baseUrl inyectable', () {
    test('usa el baseUrl inyectado en getChannels', () async {
      Uri? visto;
      final repo = ChannelRepository(
        baseUrl: 'http://gateway.local:9999',
        client: clientRespondiendo('[]', onRequest: (r) => visto = r.url),
      );

      await repo.getChannels();

      expect(visto, isNotNull);
      expect(visto!.host, 'gateway.local');
      expect(visto!.port, 9999);
      expect(visto!.path, '/channels');
    });

    test('usa el baseUrl inyectado en getStreamUrl', () async {
      Uri? visto;
      final repo = ChannelRepository(
        baseUrl: 'http://gateway.local:9999',
        client: clientRespondiendo('{"url":"http://s/x.m3u8"}',
            onRequest: (r) => visto = r.url),
      );

      final url = await repo.getStreamUrl('opensource-24/7 News');

      expect(url, 'http://s/x.m3u8');
      expect(visto!.host, 'gateway.local');
      expect(visto!.path, '/channels/stream');
      // IDs con "/" viajan como query param, no como path segment
      expect(visto!.queryParameters['id'], 'opensource-24/7 News');
    });

    test('sin baseUrl explícito apunta al gateway local por defecto', () async {
      Uri? visto;
      final repo = ChannelRepository(
        client: clientRespondiendo('[]', onRequest: (r) => visto = r.url),
      );

      await repo.getChannels();

      expect(visto!.host, '127.0.0.1');
      expect(visto!.port, 8080);
    });
  });

  group('paginación y filtros', () {
    test('envía limit y offset', () async {
      Uri? visto;
      final repo = ChannelRepository(
        baseUrl: 'http://x',
        client: clientRespondiendo('[]', onRequest: (r) => visto = r.url),
      );

      await repo.getChannels(limit: 200, offset: 400);

      expect(visto!.queryParameters['limit'], '200');
      expect(visto!.queryParameters['offset'], '400');
    });

    test('propaga los filtros como query params', () async {
      Uri? visto;
      final repo = ChannelRepository(
        baseUrl: 'http://x',
        client: clientRespondiendo('[]', onRequest: (r) => visto = r.url),
      );

      await repo.getChannels(
        filter: const ChannelFilter(
            query: 'bbc', country: 'GB', category: 'News', quality: 'hd'),
      );

      expect(visto!.queryParameters['q'], 'bbc');
      expect(visto!.queryParameters['country'], 'GB');
      expect(visto!.queryParameters['category'], 'News');
      expect(visto!.queryParameters['quality'], 'hd');
    });

    test('con showOffline pide alive=all; por defecto no envía alive',
        () async {
      Uri? visto;
      final repo = ChannelRepository(
        baseUrl: 'http://x',
        client: clientRespondiendo('[]', onRequest: (r) => visto = r.url),
      );

      await repo.getChannels();
      expect(visto!.queryParameters.containsKey('alive'), isFalse,
          reason: 'el default del gateway ya oculta los muertos');

      await repo.getChannels(
          filter: const ChannelFilter(showOffline: true));
      expect(visto!.queryParameters['alive'], 'all');
    });

    test('parsea la respuesta a modelos Channel', () async {
      final repo = ChannelRepository(
        baseUrl: 'http://x',
        client: clientRespondiendo('[$canalJson]'),
      );

      final channels = await repo.getChannels();

      expect(channels.channels, hasLength(1));
      expect(channels.channels.first.id, 'opensource-BBC One');
      expect(channels.channels.first.name, 'BBC One');
      expect(channels.channels.first.countryCode, 'GB');
    });

    test('el gateway puede devolver null (sin resultados)', () async {
      // Go serializa un slice nil como `null`, no como `[]`
      final repo = ChannelRepository(
        baseUrl: 'http://x',
        client: clientRespondiendo('null'),
      );

      final page = await repo.getChannels();

      expect(page.channels, isEmpty);
    });

    test('lanza en respuesta no-200', () async {
      final repo = ChannelRepository(
        baseUrl: 'http://x',
        client: clientRespondiendo('boom', status: 500),
      );

      expect(repo.getChannels(), throwsException);
    });
  });

  group('parseo del modelo', () {
    test('fromJson tolera campos ausentes', () {
      final ch = Channel.fromJson(json.decode('{"ID":"x"}'));
      expect(ch.id, 'x');
      expect(ch.name, 'Unknown');
      expect(ch.logoUrl, '');
    });
  });

  test('lee el total de la cabecera X-Total-Count', () async {
    final repo = ChannelRepository(
      baseUrl: 'http://x',
      client: MockClient((req) async => http.Response(
            '[]',
            200,
            headers: {'x-total-count': '1284'},
          )),
    );

    final page = await repo.getChannels();
    expect(page.total, 1284);
  });

  test('sin la cabecera, el total cae al número de canales recibidos', () async {
    final repo = ChannelRepository(
      baseUrl: 'http://x',
      client: MockClient((req) async => http.Response(
            '[{"ID":"a","Name":"A"}]',
            200,
          )),
    );

    // Degradación honesta: mejor un total bajo que un crash o un cero falso.
    final page = await repo.getChannels();
    expect(page.total, 1);
  });
}
