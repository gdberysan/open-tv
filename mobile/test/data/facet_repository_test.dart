import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:korven_open_tv/data/api_error.dart';
import 'package:korven_open_tv/data/repositories/facet_repository.dart';

void main() {
  test('parsea las facetas con su recuento', () async {
    final repo = FacetRepository(
      baseUrl: 'http://x',
      client: MockClient((_) async =>
          http.Response('[{"Valor":"US","Count":1920}]', 200)),
    );

    final got = await repo.countries();
    expect(got, hasLength(1));
    expect(got.first.valor, 'US');
    expect(got.first.count, 1920);
  });

  test('el gateway puede devolver null en vez de lista vacía', () async {
    final repo = FacetRepository(
      baseUrl: 'http://x',
      client: MockClient((_) async => http.Response('null', 200)),
    );
    expect(await repo.categories(), isEmpty);
  });

  test('un fallo se traduce a mensaje legible', () async {
    final repo = FacetRepository(
      baseUrl: 'http://x',
      client: MockClient((_) async => http.Response('boom', 500)),
    );
    expect(() => repo.countries(), throwsA(isA<ApiError>()));
  });
}
