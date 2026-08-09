import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/api_error.dart';
import '../../data/repositories/facet_repository.dart';
import '../../domain/countries.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import '../providers/facet_provider.dart';
import 'category_icons.dart';
import 'state_views.dart';

/// Fila de un selector: un icono o bandera, la etiqueta y su recuento.
class _Opcion {
  const _Opcion({
    required this.valor,
    required this.etiqueta,
    required this.count,
    this.bandera,
    this.icono,
  });

  final String valor;
  final String etiqueta;
  final int count;
  final String? bandera;
  final IconData? icono;
}

/// Abre el selector de país. Muestra los 178 países con bandera y nombre
/// completo; con esa cantidad la búsqueda deja de ser opcional.
Future<String?> mostrarSelectorPais(BuildContext context, String seleccionado) {
  return showDialog<String>(
    context: context,
    builder: (_) => _FacetDialog(
      titulo: '// país',
      seleccionado: seleccionado,
      provider: countriesProvider,
      construir: (f) => _Opcion(
        valor: f.valor,
        etiqueta: nombrePais(f.valor),
        count: f.count,
        bandera: banderaPais(f.valor),
      ),
    ),
  );
}

/// Abre el selector de categoría, con las 30 atómicas y su icono.
Future<String?> mostrarSelectorCategoria(
    BuildContext context, String seleccionado) {
  return showDialog<String>(
    context: context,
    builder: (_) => _FacetDialog(
      titulo: '// categoría',
      seleccionado: seleccionado,
      provider: categoriesProvider,
      construir: (f) => _Opcion(
        valor: f.valor,
        etiqueta: f.valor,
        count: f.count,
        icono: iconoCategoria(f.valor),
      ),
    ),
  );
}

class _FacetDialog extends ConsumerStatefulWidget {
  const _FacetDialog({
    required this.titulo,
    required this.seleccionado,
    required this.provider,
    required this.construir,
  });

  final String titulo;
  final String seleccionado;
  final FutureProvider<List<Faceta>> provider;
  final _Opcion Function(Faceta) construir;

  @override
  ConsumerState<_FacetDialog> createState() => _FacetDialogState();
}

class _FacetDialogState extends ConsumerState<_FacetDialog> {
  final _ctrl = TextEditingController();
  String _busqueda = '';

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final asyncFacetas = ref.watch(widget.provider);

    return AlertDialog(
      title: Text(widget.titulo,
          style: KorvenType.monoLabel.copyWith(color: KorvenColors.textFaint)),
      content: SizedBox(
        width: 380,
        // Alto relativo al viewport: un alto fijo desborda en ventana baja.
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.of(context).size.height * 0.6,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: _ctrl,
                autofocus: true,
                style: KorvenType.body,
                cursorColor: KorvenColors.accent,
                decoration: InputDecoration(
                  hintText: 'buscar…',
                  hintStyle:
                      KorvenType.mono.copyWith(color: KorvenColors.textFaint),
                  prefixIcon: const Icon(Icons.search,
                      size: 18, color: KorvenColors.textFaint),
                  isDense: true,
                  enabledBorder: const UnderlineInputBorder(
                    borderSide: BorderSide(color: KorvenColors.borderSubtle),
                  ),
                  focusedBorder: const UnderlineInputBorder(
                    borderSide: BorderSide(color: KorvenColors.accent),
                  ),
                ),
                onChanged: (v) => setState(() => _busqueda = v.toLowerCase()),
              ),
              const SizedBox(height: KorvenSpacing.s2),
              Flexible(
                child: switch (asyncFacetas) {
                  AsyncData(value: final facetas) =>
                    _lista(facetas.map(widget.construir).toList()),
                  AsyncError(:final error) => KorvenStateView(
                      eyebrow: '// error',
                      message: ApiError.desde(error).mensaje,
                      showEmblem: false,
                    ),
                  _ => const Center(
                      child: Padding(
                        padding: EdgeInsets.all(KorvenSpacing.s6),
                        child: CircularProgressIndicator(),
                      ),
                    ),
                },
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text('cancelar',
              style: KorvenType.mono
                  .copyWith(fontSize: 13, color: KorvenColors.textMuted)),
        ),
      ],
    );
  }

  Widget _lista(List<_Opcion> todas) {
    // Se busca por etiqueta y por código: quien sabe que México es MX no debería
    // tener que escribir el nombre.
    final visibles = _busqueda.isEmpty
        ? todas
        : todas
            .where((o) =>
                o.etiqueta.toLowerCase().contains(_busqueda) ||
                o.valor.toLowerCase().contains(_busqueda))
            .toList();

    if (visibles.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(KorvenSpacing.s5),
        child: Text('Sin coincidencias',
            style: KorvenType.mono.copyWith(color: KorvenColors.textFaint)),
      );
    }

    return ListView.builder(
      shrinkWrap: true,
      itemCount: visibles.length,
      itemBuilder: (_, i) {
        final o = visibles[i];
        final sel = o.valor.toLowerCase() == widget.seleccionado.toLowerCase();
        final color = sel ? KorvenColors.accent : KorvenColors.textBody;

        return Semantics(
          // La bandera son indicadores regionales, que un lector de pantalla
          // deletrea; el nombre real va aquí.
          label: '${o.etiqueta}, ${o.count} canales',
          button: true,
          child: ExcludeSemantics(
            child: ListTile(
              dense: true,
              leading: o.bandera != null
                  ? Text(o.bandera!, style: const TextStyle(fontSize: 20))
                  : Icon(o.icono, size: 20, color: color),
              title: Text(o.etiqueta,
                  style: KorvenType.bodySm.copyWith(color: color)),
              trailing: Text('${o.count}',
                  style: KorvenType.monoLabel
                      .copyWith(color: KorvenColors.textFaint)),
              onTap: () => Navigator.of(context).pop(o.valor),
            ),
          ),
        );
      },
    );
  }
}
