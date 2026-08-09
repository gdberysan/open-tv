import 'package:flutter/material.dart';

/// Icono por categoría atómica. Se usan los vectoriales de Material en vez de
/// encargar 30 SVG: ya son vectores, ya están en el binario, y se tiñen con la
/// regla del ámbar sin trabajo extra.
const _iconos = <String, IconData>{
  'General': Icons.tv,
  'Undefined': Icons.help_outline,
  'News': Icons.newspaper,
  'Entertainment': Icons.theater_comedy,
  'Religious': Icons.church,
  'Music': Icons.music_note,
  'Movies': Icons.movie,
  'Sports': Icons.sports_soccer,
  'Series': Icons.subscriptions,
  'Kids': Icons.child_care,
  'Education': Icons.school,
  'Documentary': Icons.menu_book,
  'Culture': Icons.museum,
  'Legislative': Icons.account_balance,
  'Comedy': Icons.sentiment_very_satisfied,
  'Lifestyle': Icons.spa,
  'Animation': Icons.animation,
  'Shop': Icons.shopping_bag,
  'Business': Icons.trending_up,
  'Classic': Icons.auto_stories,
  'Outdoor': Icons.terrain,
  'Family': Icons.family_restroom,
  'Travel': Icons.flight_takeoff,
  'Cooking': Icons.restaurant,
  'Public': Icons.public,
  'Auto': Icons.directions_car,
  'Science': Icons.science,
  'Weather': Icons.cloud,
  'Relax': Icons.self_improvement,
  'Interactive': Icons.touch_app,
};

IconData iconoCategoria(String categoria) => _iconos[categoria] ?? Icons.tv;
