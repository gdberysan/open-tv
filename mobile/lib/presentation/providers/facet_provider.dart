import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/repositories/facet_repository.dart';

final facetRepositoryProvider =
    Provider<IFacetRepository>((ref) => FacetRepository());

/// Países y categorías con su recuento. Se piden una vez y se cachean: el
/// catálogo solo cambia cuando el syncer corre, cada 12h.
final countriesProvider = FutureProvider<List<Faceta>>(
    (ref) => ref.watch(facetRepositoryProvider).countries());

final categoriesProvider = FutureProvider<List<Faceta>>(
    (ref) => ref.watch(facetRepositoryProvider).categories());
