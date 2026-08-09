import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import 'korven_wordmark.dart';

/// Barra superior de Korven Open TV. Sustituye al AppBar de plantilla: a la
/// izquierda el lockup de marca, a la derecha las acciones — mudas por defecto
/// y ámbar solo cuando están activas, según la regla del sistema.
class ConsoleBar extends StatelessWidget implements PreferredSizeWidget {
  const ConsoleBar({
    super.key,
    required this.searching,
    required this.searchController,
    required this.onSearchChanged,
    required this.onOpenSearch,
    required this.onCloseSearch,
    required this.showOffline,
    required this.onToggleOffline,
    required this.onReload,
    required this.gridMode,
    required this.onToggleView,
    required this.onRandom,
    this.leadingActions = const <Widget>[],
  });

  final bool searching;
  final TextEditingController searchController;
  final ValueChanged<String> onSearchChanged;
  final VoidCallback onOpenSearch;
  final VoidCallback onCloseSearch;
  final bool showOffline;
  final VoidCallback onToggleOffline;
  final VoidCallback onReload;
  final bool gridMode;
  final VoidCallback onToggleView;
  final VoidCallback onRandom;

  /// Acciones que van antes de las propias de la barra. El botón de AirPlay
  /// entra por aquí en vez de hardcodearse: ConsoleBar no debe saber que
  /// existe una sesión de emisión.
  final List<Widget> leadingActions;

  @override
  Size get preferredSize => const Size.fromHeight(60);

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 60,
      padding: const EdgeInsets.symmetric(horizontal: KorvenSpacing.s5),
      decoration: const BoxDecoration(
        color: KorvenColors.surfaceSunken,
        border: Border(bottom: BorderSide(color: KorvenColors.borderSubtle)),
      ),
      child: Row(
        children: [
          if (searching)
            Expanded(
              child: TextField(
                controller: searchController,
                autofocus: true,
                style: KorvenType.body,
                cursorColor: KorvenColors.accent,
                decoration: InputDecoration(
                  hintText: 'buscar canal…',
                  hintStyle: KorvenType.mono
                      .copyWith(color: KorvenColors.textFaint),
                  border: InputBorder.none,
                ),
                onChanged: onSearchChanged,
              ),
            )
          else ...[
            const KorvenWordmark(),
            const Spacer(),
          ],
          if (searching)
            _AccionBarra(
              icon: Icons.close,
              tooltip: 'Cerrar búsqueda',
              onPressed: onCloseSearch,
            )
          else ...[
            // Solo fuera de la búsqueda: durante ella la barra deja sitio al
            // campo de texto y solo conserva el botón de cerrar.
            ...leadingActions,
            _AccionBarra(
              icon: Icons.search,
              tooltip: 'Buscar canal',
              onPressed: onOpenSearch,
            ),
            _AccionBarra(
              key: const Key('accion-aleatorio'),
              icon: Icons.shuffle,
              tooltip: 'Canal aleatorio',
              onPressed: onRandom,
            ),
            _AccionBarra(
              key: const Key('accion-vista'),
              // El icono anuncia el destino, no el estado actual: en rejilla
              // ofrece la lista. Por eso no lleva ámbar — no hay nada activo
              // que señalar, es un modo con dos caras iguales.
              icon: gridMode ? Icons.view_list : Icons.grid_view,
              tooltip: gridMode ? 'Ver como lista' : 'Ver como rejilla',
              onPressed: onToggleView,
            ),
            _AccionBarra(
              icon: showOffline ? Icons.visibility : Icons.visibility_off,
              tooltip: showOffline
                  ? 'Ocultar canales offline'
                  : 'Mostrar canales offline',
              // Activo = ámbar. Es una decisión del usuario en curso.
              active: showOffline,
              onPressed: onToggleOffline,
            ),
            _AccionBarra(
              icon: Icons.refresh,
              tooltip: 'Recargar',
              onPressed: onReload,
            ),
          ],
        ],
      ),
    );
  }
}

class _AccionBarra extends StatelessWidget {
  const _AccionBarra({
    super.key,
    required this.icon,
    required this.tooltip,
    required this.onPressed,
    this.active = false,
  });

  final IconData icon;
  final String tooltip;
  final VoidCallback onPressed;
  final bool active;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      icon: Icon(icon,
          size: 20,
          color: active ? KorvenColors.accent : KorvenColors.textMuted),
      tooltip: tooltip,
      onPressed: onPressed,
      hoverColor: KorvenColors.surfaceCard,
    );
  }
}
