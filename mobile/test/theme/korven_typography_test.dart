import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/theme/korven_typography.dart';

void main() {
  test('cada rol usa la familia que le corresponde', () {
    // Display en Space Grotesk, lectura en Inter, y mono para lo que es
    // literalmente dato: etiquetas, metadatos y consola.
    expect(KorvenType.h1.fontFamily, 'SpaceGrotesk');
    expect(KorvenType.h2.fontFamily, 'SpaceGrotesk');
    expect(KorvenType.body.fontFamily, 'Inter');
    expect(KorvenType.bodySm.fontFamily, 'Inter');
    expect(KorvenType.mono.fontFamily, 'JetBrainsMono');
    expect(KorvenType.monoLabel.fontFamily, 'JetBrainsMono');
  });

  test('la escala de tipo es la tercera mayor 1.250 con base 16', () {
    expect(KorvenType.sizeBase, 16.0);
    expect(KorvenType.sizeSm, 14.0);
    expect(KorvenType.sizeXs, 12.0);
    expect(KorvenType.sizeLg, 20.0);
    expect(KorvenType.size2xl, 31.0);
  });

  test('los eyebrows mono llevan el tracking ancho de la marca', () {
    expect(KorvenType.monoLabel.letterSpacing, greaterThan(0));
  });
}
