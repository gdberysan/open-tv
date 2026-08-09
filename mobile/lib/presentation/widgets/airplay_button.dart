import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

/// El AVRoutePickerView nativo, embebido.
///
/// Es el botón real de Apple y no una réplica: el descubrimiento de
/// dispositivos, el emparejamiento con tvOS y la lista de rutas los resuelve
/// AVFoundation. Reimplementarlo exigiría el handshake HAP, que es frágil y no
/// tiene soporte.
///
/// Fuera de macOS colapsa a cero para que los targets de la Fase 9 compilen sin
/// tocarlo — y para que CI, que corre en Linux, no intente crear un AppKitView.
class AirplayButton extends StatelessWidget {
  const AirplayButton({super.key});

  static const _viewType = 'dev.korven.opentv/route-picker';
  static const _lado = 28.0;

  @override
  Widget build(BuildContext context) {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.macOS) {
      return const SizedBox.shrink();
    }
    return const SizedBox(
      width: _lado,
      height: _lado,
      // Sin creationParams: la vista nativa no recibe argumentos. El selector
      // abre su propio popover al hacer clic, y hitTestBehavior.opaque, que es
      // el valor por defecto, ya deja que los clics lleguen al NSView.
      child: AppKitView(viewType: _viewType),
    );
  }
}
