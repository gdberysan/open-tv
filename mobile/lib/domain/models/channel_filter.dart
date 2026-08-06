class ChannelFilter {
  final String query;
  final String country;
  final String category;

  /// '' = all, 'hd' = 720p+, 'fhd' = 1080p+ (default), '4k' = 4K+
  final String quality;

  const ChannelFilter({
    this.query = '',
    this.country = '',
    this.category = '',
    this.quality = 'fhd',
  });

  ChannelFilter copyWith({
    String? query,
    String? country,
    String? category,
    String? quality,
  }) =>
      ChannelFilter(
        query: query ?? this.query,
        country: country ?? this.country,
        category: category ?? this.category,
        quality: quality ?? this.quality,
      );

  bool get hasActiveFilters =>
      country.isNotEmpty || category.isNotEmpty || quality != 'fhd';
}
