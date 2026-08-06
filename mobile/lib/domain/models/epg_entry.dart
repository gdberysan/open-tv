/// Entrada de la guía de programación servida por el gateway (/epg).
class EPGEntry {
  final String channelId;
  final String title;
  final String description;
  final DateTime startAt;
  final DateTime endAt;

  const EPGEntry({
    required this.channelId,
    required this.title,
    required this.description,
    required this.startAt,
    required this.endAt,
  });

  factory EPGEntry.fromJson(Map<String, dynamic> json) {
    return EPGEntry(
      channelId: json['ChannelID'] as String? ?? '',
      title: json['Title'] as String? ?? '',
      description: json['Description'] as String? ?? '',
      startAt: DateTime.parse(json['StartAt'] as String),
      endAt: DateTime.parse(json['EndAt'] as String),
    );
  }

  /// true si el programa está en emisión en [t] (inicio inclusivo, fin exclusivo).
  bool airsAt(DateTime t) => !t.isBefore(startAt) && t.isBefore(endAt);
}
