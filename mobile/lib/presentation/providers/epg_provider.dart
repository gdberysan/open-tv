import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/repositories/epg_repository.dart';
import '../../domain/models/epg_entry.dart';

final epgRepositoryProvider = Provider<IEPGRepository>((ref) {
  return EPGRepository();
});

/// Reloj inyectable para que los tests fijen "ahora".
final clockProvider = Provider<DateTime Function()>((_) => DateTime.now);

/// Cadencia de refresco de la guía (roadmap 6.2: cada 5 min).
final epgRefreshIntervalProvider =
    Provider<Duration>((_) => const Duration(minutes: 5));

/// Ventana visible de la guía: desde la media hora en curso, 6h en adelante.
({DateTime from, DateTime to}) guideWindow(DateTime now) {
  final from = DateTime(
      now.year, now.month, now.day, now.hour, now.minute >= 30 ? 30 : 0);
  return (from: from, to: from.add(const Duration(hours: 6)));
}

/// Programación de un canal para la ventana de la guía. autoDispose: las
/// filas fuera de pantalla sueltan su caché y su timer de refresco.
final epgForChannelProvider = FutureProvider.autoDispose
    .family<List<EPGEntry>, String>((ref, channelId) async {
  final timer = Timer(ref.watch(epgRefreshIntervalProvider), ref.invalidateSelf);
  ref.onDispose(timer.cancel);

  final window = guideWindow(ref.watch(clockProvider)());
  return ref
      .read(epgRepositoryProvider)
      .getForChannel(channelId, from: window.from, to: window.to);
});
