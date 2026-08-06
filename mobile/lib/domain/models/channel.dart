class Channel {
  final String id;
  final String name;
  final String logoUrl;
  final String categoryId;
  final String languageCode;
  final String countryCode;
  final String providerType;

  const Channel({
    required this.id,
    required this.name,
    required this.logoUrl,
    required this.categoryId,
    required this.languageCode,
    required this.countryCode,
    required this.providerType,
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
    );
  }
}
