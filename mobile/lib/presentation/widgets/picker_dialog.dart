import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';

enum PickerTipo { pais, categoria }

const _paises = [
  'GB', 'US', 'FR', 'DE', 'ES', 'IT', 'AU', 'CA', 'JP', 'BR', //
  'MX', 'AR', 'NL', 'BE', 'CH', 'AT', 'PL', 'PT', 'RU', 'IN',
];

const _categorias = [
  'News', 'Sports', 'Entertainment', 'Movies', 'Kids', //
  'Documentary', 'Music', 'Lifestyle', 'Science', 'Travel',
];

/// Abre el selector de país o categoría y devuelve la opción elegida, o null si
/// se cancela. Extraído de home_screen.dart, que se había comido ocho clases.
Future<String?> mostrarPicker(
  BuildContext context,
  PickerTipo tipo, {
  String seleccionado = '',
}) {
  final esPais = tipo == PickerTipo.pais;
  return showDialog<String>(
    context: context,
    builder: (_) => _PickerDialog(
      titulo: esPais ? '// país' : '// categoría',
      opciones: esPais ? _paises : _categorias,
      seleccionado: seleccionado,
    ),
  );
}

class _PickerDialog extends StatefulWidget {
  const _PickerDialog({
    required this.titulo,
    required this.opciones,
    required this.seleccionado,
  });

  final String titulo;
  final List<String> opciones;
  final String seleccionado;

  @override
  State<_PickerDialog> createState() => _PickerDialogState();
}

class _PickerDialogState extends State<_PickerDialog> {
  late List<String> _filtradas = widget.opciones;
  final _ctrl = TextEditingController();

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  void _buscar(String q) {
    setState(() {
      _filtradas = widget.opciones
          .where((o) => o.toLowerCase().contains(q.toLowerCase()))
          .toList();
    });
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.titulo,
          style: KorvenType.monoLabel.copyWith(color: KorvenColors.textFaint)),
      content: SizedBox(
        width: 320,
        // Alto relativo, no los 400px fijos de antes: en una ventana baja el
        // diálogo desbordaba.
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
                  hintStyle: KorvenType.mono
                      .copyWith(color: KorvenColors.textFaint),
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
                onChanged: _buscar,
              ),
              const SizedBox(height: KorvenSpacing.s2),
              Flexible(
                child: ListView.builder(
                  shrinkWrap: true,
                  itemCount: _filtradas.length,
                  itemBuilder: (_, i) {
                    final opt = _filtradas[i];
                    final sel = opt.toLowerCase() ==
                        widget.seleccionado.toLowerCase();
                    return ListTile(
                      dense: true,
                      title: Text(opt,
                          style: KorvenType.bodySm.copyWith(
                              color: sel
                                  ? KorvenColors.accent
                                  : KorvenColors.textBody)),
                      trailing: sel
                          ? const Icon(Icons.check,
                              size: 16, color: KorvenColors.accent)
                          : null,
                      onTap: () => Navigator.of(context).pop(opt),
                    );
                  },
                ),
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
}
