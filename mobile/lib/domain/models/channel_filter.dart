class ChannelFilter {
  final String query;
  final String country;
  final String category;

  /// '' = all, 'hd' = 720p+, 'fhd' = 1080p+ (default), '4k' = 4K+
  final String quality;

  /// Mostrar también canales cuyos streams están todos muertos.
  /// El gateway los oculta por defecto (Fase 7).
  final bool showOffline;

  /// Acota a los canales marcados como favoritos. Se resuelve pidiendo sus ids
  /// al gateway, no paginando: un favorito puede estar en la página 20.
  final bool onlyFavorites;

  const ChannelFilter({
    this.query = '',
    this.country = '',
    this.category = '',
    this.quality = 'fhd',
    this.showOffline = false,
    this.onlyFavorites = false,
  });

  ChannelFilter copyWith({
    String? query,
    String? country,
    String? category,
    String? quality,
    bool? showOffline,
    bool? onlyFavorites,
  }) =>
      ChannelFilter(
        query: query ?? this.query,
        country: country ?? this.country,
        category: category ?? this.category,
        quality: quality ?? this.quality,
        showOffline: showOffline ?? this.showOffline,
        onlyFavorites: onlyFavorites ?? this.onlyFavorites,
      );

  bool get hasActiveFilters =>
      query.isNotEmpty ||
      country.isNotEmpty ||
      category.isNotEmpty ||
      quality != 'fhd' ||
      onlyFavorites;

  // Igualdad por valor: StateProvider compara con == para decidir si notifica,
  // y copyWith siempre construye un objeto nuevo. Sin esto, volver a tocar el
  // chip ya activo se veía como un cambio de filtro y descartaba todas las
  // páginas acumuladas, devolviendo al usuario al principio de la lista.
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ChannelFilter &&
          other.query == query &&
          other.country == country &&
          other.category == category &&
          other.quality == quality &&
          other.showOffline == showOffline &&
          other.onlyFavorites == onlyFavorites;

  @override
  int get hashCode =>
      Object.hash(query, country, category, quality, showOffline, onlyFavorites);
}
