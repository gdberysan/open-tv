import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/domain/models/channel.dart';

void main() {
  group('Channel.fromJson salud', () {
    test('parsea Alive true con latencia', () {
      final ch = Channel.fromJson(json.decode(
          '{"ID":"x","Name":"X","Alive":true,"LatencyMs":150}'));
      expect(ch.alive, isTrue);
      expect(ch.latencyMs, 150);
    });

    test('parsea Alive false (muerto)', () {
      final ch = Channel.fromJson(
          json.decode('{"ID":"x","Name":"X","Alive":false,"LatencyMs":0}'));
      expect(ch.alive, isFalse);
    });

    test('Alive null = sin chequear', () {
      final ch = Channel.fromJson(
          json.decode('{"ID":"x","Name":"X","Alive":null,"LatencyMs":0}'));
      expect(ch.alive, isNull);
    });

    test('sin campos de salud (gateway viejo) no rompe', () {
      final ch = Channel.fromJson(json.decode('{"ID":"x","Name":"X"}'));
      expect(ch.alive, isNull);
      expect(ch.latencyMs, 0);
    });
  });

  group('signalLevel', () {
    test('sin chequear', () {
      expect(signalLevel(null, 0), SignalLevel.unknown);
    });
    test('muerto', () {
      expect(signalLevel(false, 0), SignalLevel.dead);
    });
    test('verde <200ms', () {
      expect(signalLevel(true, 199), SignalLevel.good);
    });
    test('naranja 200-800ms', () {
      expect(signalLevel(true, 200), SignalLevel.medium);
      expect(signalLevel(true, 800), SignalLevel.medium);
    });
    test('rojo >800ms', () {
      expect(signalLevel(true, 801), SignalLevel.poor);
    });
  });
}
