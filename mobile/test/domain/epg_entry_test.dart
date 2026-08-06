import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/domain/models/epg_entry.dart';

void main() {
  test('parsea la respuesta del gateway (nombres Go, RFC3339)', () {
    final e = EPGEntry.fromJson(json.decode(
        '{"ChannelID":"opensource-BBC News","Title":"Noticias",'
        '"Description":"Resumen","StartAt":"2026-08-06T14:00:00-06:00",'
        '"EndAt":"2026-08-06T15:00:00-06:00"}'));

    expect(e.channelId, 'opensource-BBC News');
    expect(e.title, 'Noticias');
    expect(e.description, 'Resumen');
    expect(e.endAt.difference(e.startAt), const Duration(hours: 1));
  });

  test('tolera descripción ausente', () {
    final e = EPGEntry.fromJson(json.decode(
        '{"ChannelID":"x","Title":"T","StartAt":"2026-08-06T14:00:00Z",'
        '"EndAt":"2026-08-06T15:00:00Z"}'));
    expect(e.description, '');
  });

  test('airsAt indica si el programa está en emisión en un instante', () {
    final e = EPGEntry.fromJson(json.decode(
        '{"ChannelID":"x","Title":"T","StartAt":"2026-08-06T14:00:00Z",'
        '"EndAt":"2026-08-06T15:00:00Z"}'));

    expect(e.airsAt(DateTime.parse('2026-08-06T14:30:00Z')), isTrue);
    expect(e.airsAt(DateTime.parse('2026-08-06T15:00:00Z')), isFalse);
    expect(e.airsAt(DateTime.parse('2026-08-06T13:59:00Z')), isFalse);
  });
}
