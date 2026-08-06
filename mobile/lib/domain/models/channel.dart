class Channel {
  final String id;
  final String name;
  final String logoUrl;
  final String categoryId;
  final String languageCode;
  final String countryCode;
  final String providerType;

  /// Salud agregada que incrusta el gateway en /channels.
  /// null = ningún stream chequeado aún por el health-worker.
  final bool? alive;

  /// Mejor latencia entre los streams vivos del canal (0 si no aplica).
  final int latencyMs;

  const Channel({
    required this.id,
    required this.name,
    required this.logoUrl,
    required this.categoryId,
    required this.languageCode,
    required this.countryCode,
    required this.providerType,
    this.alive,
    this.latencyMs = 0,
  });

  factory Channel.fromJson(Map<String, dynamic> json) {
    return Channel(
      id: json['ID'] as String? ?? '',
      name: json['Name'] as String? ?? 'Unknown',
      logoUrl: json['LogoURL'] as String? ?? '',
      categoryId: json['CategoryID'] as String? ?? '',
      languageCode: json['LanguageCode'] as String? ?? '',
      countryCode: json['CountryCode'] as String? ?? '',
      providerType: json['ProviderType'] as String? ?? '',
      alive: json['Alive'] as bool?,
      latencyMs: (json['LatencyMs'] as num?)?.toInt() ?? 0,
    );
  }
}

/// Nivel de señal para el indicador de 3 barras (Fase 7.2).
enum SignalLevel { unknown, dead, good, medium, poor }

/// Umbrales del roadmap: verde <200ms, naranja 200–800ms, rojo >800ms,
/// gris muerto; sin chequear → unknown (barras apagadas).
SignalLevel signalLevel(bool? alive, int latencyMs) {
  if (alive == null) return SignalLevel.unknown;
  if (!alive) return SignalLevel.dead;
  if (latencyMs < 200) return SignalLevel.good;
  if (latencyMs <= 800) return SignalLevel.medium;
  return SignalLevel.poor;
}
