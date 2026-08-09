import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/models/channel_filter.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import '../providers/channel_provider.dart';
import 'korven_chip.dart';
import 'picker_dialog.dart';

const _calidades = <String, String>{
  '4k': '4K',
  'fhd': '1080p+',
  'hd': 'HD 720p+',
  '': 'todos',
};

/// Barra de filtros. Reemplaza a la anterior, donde la calidad ocupaba cuatro
/// chips permanentes, país y categoría se escondían tras diálogos, y nada
/// indicaba qué estaba activo ni permitía limpiarlo de un toque.
class FilterBar extends ConsumerWidget {
  const FilterBar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final filtro = ref.watch(channelFilterProvider);
    final lista = ref.watch(channelListProvider);
    final total = lista.valueOrNull?.total;

    void aplicar(ChannelFilter f) =>
        ref.read(channelFilterProvider.notifier).state = f;

    final facetas = <Widget>[
      if (filtro.country.isNotEmpty)
        KorvenChip(
          label: 'país: ${filtro.country}',
          active: true,
          removeTooltip: 'Quitar filtro de país',
          onRemove: () => aplicar(filtro.copyWith(country: '')),
        ),
      if (filtro.category.isNotEmpty)
        KorvenChip(
          label: 'categoría: ${filtro.category}',
          active: true,
          removeTooltip: 'Quitar filtro de categoría',
          onRemove: () => aplicar(filtro.copyWith(category: '')),
        ),
      if (filtro.query.isNotEmpty)
        KorvenChip(
          label: 'busca: ${filtro.query}',
          active: true,
          removeTooltip: 'Quitar la búsqueda',
          onRemove: () => aplicar(filtro.copyWith(query: '')),
        ),
      // Añadir las facetas que aún no están puestas.
      if (filtro.country.isEmpty)
        KorvenChip(
          label: '+ país',
          onTap: () async {
            final v = await mostrarPicker(context, PickerTipo.pais);
            if (v != null) aplicar(filtro.copyWith(country: v));
          },
        ),
      if (filtro.category.isEmpty)
        KorvenChip(
          label: '+ categoría',
          onTap: () async {
            final v = await mostrarPicker(context, PickerTipo.categoria);
            if (v != null) aplicar(filtro.copyWith(category: v));
          },
        ),
    ];

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
              const Spacer(),
              if (total != null)
                Text('$total canales',
                    style: KorvenType.monoLabel
                        .copyWith(color: KorvenColors.textFaint)),
            ],
          ),
          const SizedBox(height: KorvenSpacing.s3),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Wrap(
                  spacing: KorvenSpacing.s2,
                  runSpacing: KorvenSpacing.s2,
                  children: facetas,
                ),
              ),
              if (filtro.hasActiveFilters)
                TextButton(
                  onPressed: () => aplicar(const ChannelFilter()),
                  child: Text('limpiar',
                      style: KorvenType.mono
                          .copyWith(fontSize: 13, color: KorvenColors.accent)),
                ),
            ],
          ),
          const SizedBox(height: KorvenSpacing.s3),
          Wrap(
            spacing: KorvenSpacing.s2,
            runSpacing: KorvenSpacing.s2,
            children: [
              for (final e in _calidades.entries)
                KorvenChip(
                  label: e.value,
                  // Ámbar solo si el usuario se salió del default: el acento
                  // marca decisión, no estado por omisión.
                  active: filtro.quality == e.key && e.key != 'fhd',
                  onTap: () => aplicar(filtro.copyWith(quality: e.key)),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
