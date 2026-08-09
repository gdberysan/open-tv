import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/countries.dart';
import '../../domain/models/channel_filter.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import '../providers/channel_provider.dart';
import 'category_icons.dart';
import 'facet_picker.dart';
import 'filter_button.dart';
import 'korven_chip.dart';

const _resoluciones = <String, String>{
  '4k': '4K',
  'fhd': '1080p+',
  'hd': 'HD 720p+',
  '': 'todos',
};

/// Barra de filtros: tres botones del mismo ancho en una sola línea, cada uno
/// abriendo su modal. Sustituye a los chips de calidad siempre visibles y a los
/// selectores escondidos, que no dejaban ver qué estaba activo.
class FilterBar extends ConsumerWidget {
  const FilterBar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final filtro = ref.watch(channelFilterProvider);
    final total = ref.watch(channelListProvider).valueOrNull?.total;

    void aplicar(ChannelFilter f) =>
        ref.read(channelFilterProvider.notifier).state = f;

    return Container(
      padding: const EdgeInsets.symmetric(
          horizontal: KorvenSpacing.s5, vertical: KorvenSpacing.s3),
      decoration: const BoxDecoration(
        border: Border(bottom: BorderSide(color: KorvenColors.borderSubtle)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text.rich(TextSpan(children: [
                TextSpan(
                    text: '// ',
                    style: KorvenType.monoLabel
                        .copyWith(color: KorvenColors.textFaint)),
                const TextSpan(text: 'filtros', style: KorvenType.monoLabel),
              ])),
              const SizedBox(width: KorvenSpacing.s4),
              if (filtro.query.isNotEmpty)
                KorvenChip(
                  label: 'busca: ${filtro.query}',
                  active: true,
                  removeTooltip: 'Quitar la búsqueda',
                  onRemove: () => aplicar(filtro.copyWith(query: '')),
                ),
              const Spacer(),
              if (filtro.hasActiveFilters)
                TextButton(
                  onPressed: () => aplicar(const ChannelFilter()),
                  child: Text('limpiar',
                      style: KorvenType.mono
                          .copyWith(fontSize: 13, color: KorvenColors.accent)),
                ),
              if (total != null)
                Padding(
                  padding: const EdgeInsets.only(left: KorvenSpacing.s3),
                  child: Text('$total canales',
                      style: KorvenType.monoLabel
                          .copyWith(color: KorvenColors.textFaint)),
                ),
            ],
          ),
          const SizedBox(height: KorvenSpacing.s3),
          // Expanded en los tres: mismo ancho y misma línea, que es lo que pide
          // el diseño.
          Row(
            children: [
              Expanded(
                child: FilterButton(
                  key: const Key('filtro-pais'),
                  icon: Icons.public,
                  label: 'país',
                  value: filtro.country.isEmpty
                      ? null
                      : nombrePais(filtro.country),
                  leading: filtro.country.isEmpty
                      ? null
                      : Text(banderaPais(filtro.country),
                          style: const TextStyle(fontSize: 16)),
                  onClear: () => aplicar(filtro.copyWith(country: '')),
                  onTap: () async {
                    final v =
                        await mostrarSelectorPais(context, filtro.country);
                    if (v != null) aplicar(filtro.copyWith(country: v));
                  },
                ),
              ),
              const SizedBox(width: KorvenSpacing.s2),
              Expanded(
                child: FilterButton(
                  key: const Key('filtro-resolucion'),
                  icon: Icons.high_quality,
                  label: 'resolución',
                  // 'fhd' es el default, así que no cuenta como decisión.
                  value: filtro.quality == 'fhd'
                      ? null
                      : _resoluciones[filtro.quality],
                  onClear: () => aplicar(filtro.copyWith(quality: 'fhd')),
                  onTap: () async {
                    final v = await _mostrarSelectorResolucion(
                        context, filtro.quality);
                    if (v != null) aplicar(filtro.copyWith(quality: v));
                  },
                ),
              ),
              const SizedBox(width: KorvenSpacing.s2),
              Expanded(
                child: FilterButton(
                  key: const Key('filtro-categoria'),
                  icon: filtro.category.isEmpty
                      ? Icons.category
                      : iconoCategoria(filtro.category),
                  label: 'categoría',
                  value: filtro.category.isEmpty ? null : filtro.category,
                  onClear: () => aplicar(filtro.copyWith(category: '')),
                  onTap: () async {
                    final v = await mostrarSelectorCategoria(
                        context, filtro.category);
                    if (v != null) aplicar(filtro.copyWith(category: v));
                  },
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// La resolución son cuatro opciones fijas: no necesita búsqueda ni recuentos.
Future<String?> _mostrarSelectorResolucion(
    BuildContext context, String seleccionado) {
  return showDialog<String>(
    context: context,
    builder: (_) => AlertDialog(
      title: Text('// resolución',
          style: KorvenType.monoLabel.copyWith(color: KorvenColors.textFaint)),
      content: SizedBox(
        width: 300,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final e in _resoluciones.entries)
              ListTile(
                dense: true,
                title: Text(e.value,
                    style: KorvenType.bodySm.copyWith(
                        color: e.key == seleccionado
                            ? KorvenColors.accent
                            : KorvenColors.textBody)),
                trailing: e.key == seleccionado
                    ? const Icon(Icons.check,
                        size: 16, color: KorvenColors.accent)
                    : null,
                onTap: () => Navigator.of(context).pop(e.key),
              ),
          ],
        ),
      ),
    ),
  );
}
