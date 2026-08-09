import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/domain/countries.dart';

void main() {
  test('traduce los códigos ISO a nombre completo en español', () {
    expect(nombrePais('MX'), 'México');
    expect(nombrePais('US'), 'Estados Unidos');
    expect(nombrePais('GB'), 'Reino Unido');
    expect(nombrePais('DE'), 'Alemania');
    expect(nombrePais('KR'), 'Corea del Sur');
  });

  test('acepta el código en minúsculas', () {
    expect(nombrePais('mx'), 'México');
  });

  test('un código desconocido cae al propio código, no rompe la UI', () {
    expect(nombrePais('ZZ'), 'ZZ');
    expect(nombrePais(''), '');
  });

  test('deriva la bandera de los indicadores regionales', () {
    expect(banderaPais('MX'), '🇲🇽');
    expect(banderaPais('ES'), '🇪🇸');
    expect(banderaPais('X'), '');
    expect(banderaPais(''), '');
  });

  test('cubre los 178 países que hay en el catálogo', () {
    // Estos son los códigos reales de la DB. Si el proveedor añade uno nuevo la
    // UI no se rompe (cae al código), pero estos deben tener nombre de verdad.
    const enCatalogo = 'AD AE AF AG AL AM AO AR AT AU AW AZ BA BB BD BE BF BG '
        'BH BJ BN BO BQ BR BS BY BZ CA CD CG CH CI CL CM CN CO CR CU CV CW CY '
        'CZ DE DK DO DZ EC EE EG EH ER ES ET FI FO FR GB GE GF GH GM GN GP GQ '
        'GR GT GU GY HK HN HR HT HU ID IE IL IN IQ IR IS IT JM JO JP KE KG KH '
        'KN KP KR KW KZ LA LB LC LK LT LU LV LY MA MC MD ME MK ML MM MN MO MQ '
        'MT MV MX MY MZ NA NE NG NI NL NO NP NZ OM PA PE PF PG PH PK PL PR PS '
        'PT PY QA RO RS RU RW SA SD SE SG SI SK SL SM SN SO SR SV SX SY TD TG '
        'TH TJ TM TN TR TT TW TZ UA UG US UY UZ VC VE VG VN WS XK YE ZA ZW';

    final sinNombre = enCatalogo
        .split(' ')
        .where((c) => c.isNotEmpty && nombrePais(c) == c)
        .toList();

    expect(sinNombre, isEmpty,
        reason: 'estos códigos del catálogo no tienen nombre: $sinNombre');
  });
}
