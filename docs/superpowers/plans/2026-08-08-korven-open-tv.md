# Korven Open TV — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebrand the project to Korven Open TV and rebuild its interface on the korven.dev design system, so the app looks like the tool it already is.

**Architecture:** Seis fases. La 1 crea la capa de tokens y el `ThemeData`, de la que depende todo lo demás. La 2 añade las marcas de la casa (wordmark hexagonal y emblema) y la barra de consola. La 3 reestructura los filtros, que es el punto débil real, e incluye el cambio de gateway que hace honesto el contador. La 4 rehace filas y estados. La 5, el reproductor. La 6 renombra el proyecto entero, dejando la carpeta para el final por sus efectos fuera del repositorio.

`home_screen.dart` tiene hoy 562 líneas y 8 clases. Se descompone a lo largo de las fases 2–4 en widgets con una responsabilidad cada uno; al terminar debe quedar como composición.

**Tech Stack:** Flutter 3.x (Riverpod, media_kit), Go 1.25 (chi, modernc/sqlite). **Cero dependencias nuevas**: las fuentes se empaquetan como assets y las marcas se dibujan con `CustomPainter`/`ClipPath`.

**Spec:** `docs/superpowers/specs/2026-08-08-korven-open-tv-design.md`

## Global Constraints

- El design system es **alta fidelidad**: los valores de `/Users/usuario/Dev/korven/design/design_handoff_korven_sitio/tokens/*.css` son finales y se portan literales. No reinterpretar colores, tamaños ni easings.
- **Regla del ámbar:** `#FF8A2B` significa señal viva — canal vivo, filtro activo, foco. Si un elemento no es ninguna de esas tres cosas, no lleva ámbar.
- Solo tema oscuro.
- Cero dependencias nuevas en `pubspec.yaml` salvo las fuentes como assets.
- Tras cada tarea: `flutter analyze` sin incidencias y `flutter test` en verde; en el gateway `go build ./... && go vet ./... && gofmt -l . && go test -race ./...`.
- Ningún widget vuelve a hardcodear un color: todo sale del tema o de los tokens.
- Comentarios y mensajes de commit en español, como el resto del repo.
- Un commit por tarea.

---

# FASE 1 — Capa de tokens y tema

### Task 1: Portar los tokens a Dart

**Files:**
- Create: `mobile/lib/theme/korven_colors.dart`, `korven_spacing.dart`, `korven_motion.dart`
- Test: `mobile/test/theme/korven_tokens_test.dart`

**Interfaces:**
- Produces: `KorvenColors` (clase con constantes estáticas), `KorvenSpacing`, `KorvenRadius`, `KorvenMotion`. Todo `abstract final class` con miembros `static const` — sin instanciación.

- [ ] **Step 1: Escribir el test de fidelidad**

El riesgo real aquí no es que el código falle, es que alguien teclee mal un hex y nadie lo note. El test fija los valores contra el brand board.

```dart
// mobile/test/theme/korven_tokens_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/theme/korven_colors.dart';
import 'package:korven_open_tv/theme/korven_spacing.dart';

void main() {
  // Los valores vienen del brand board de Korven (tokens/colors.css). Este test
  // no prueba lógica: fija la identidad de marca para que un dedazo en un hex
  // no pase desapercibido.
  test('los colores del núcleo de marca son los del brand board', () {
    expect(KorvenColors.grafito, const Color(0xFF0E131B));
    expect(KorvenColors.carbon, const Color(0xFF171E29));
    expect(KorvenColors.ambar, const Color(0xFFFF8A2B));
    expect(KorvenColors.acero, const Color(0xFF97A3B2));
    expect(KorvenColors.hueso, const Color(0xFFEFF3F8));
  });

  test('las superficies y textos semánticos apuntan a la rampa correcta', () {
    expect(KorvenColors.surfaceBase, const Color(0xFF0E131B));
    expect(KorvenColors.surfaceSunken, const Color(0xFF0A0E15));
    expect(KorvenColors.surfaceCard, const Color(0xFF171E29));
    expect(KorvenColors.surfaceInset, const Color(0xFF121826));
    expect(KorvenColors.textStrong, const Color(0xFFEFF3F8));
    expect(KorvenColors.textBody, const Color(0xFFC3CCD7));
    expect(KorvenColors.textMuted, const Color(0xFF97A3B2));
    expect(KorvenColors.textFaint, const Color(0xFF6B788C));
  });

  test('el acento tiene sus estados y su glow', () {
    expect(KorvenColors.accent, const Color(0xFFFF8A2B));
    expect(KorvenColors.accentHover, const Color(0xFFFFA255));
    expect(KorvenColors.accentPress, const Color(0xFFE06E1C));
    expect(KorvenColors.amberGlow.a, closeTo(0.16, 0.001));
  });

  test('los hairlines mantienen las tres opacidades del sistema', () {
    expect(KorvenColors.borderSubtle.a, closeTo(0.12, 0.001));
    expect(KorvenColors.borderDefault.a, closeTo(0.20, 0.001));
    expect(KorvenColors.borderStrong.a, closeTo(0.36, 0.001));
  });

  test('el espaciado sigue la escala de 8px', () {
    expect(KorvenSpacing.s1, 4.0);
    expect(KorvenSpacing.s2, 8.0);
    expect(KorvenSpacing.s4, 16.0);
    expect(KorvenSpacing.s5, 24.0);
    expect(KorvenSpacing.s7, 48.0);
  });
}
```

- [ ] **Step 2: Correr el test y verlo fallar**

Run: `cd mobile && flutter test test/theme/korven_tokens_test.dart`
Esperado: FAIL, no existen los archivos.

- [ ] **Step 3: Escribir los tokens**

`mobile/lib/theme/korven_colors.dart` — portado de `tokens/colors.css`:

```dart
import 'package:flutter/material.dart';

/// Tokens de color de Korven, portados literalmente del brand board
/// (design_handoff_korven_sitio/tokens/colors.css). Valores finales: no se
/// reinterpretan.
///
/// La regla del sistema: el canvas es grafito y acero, y el ámbar es el único
/// acento — la señal viva. En esta app significa canal vivo, filtro activo o
/// foco. Si algo no es una de esas tres cosas, no lleva ámbar.
abstract final class KorvenColors {
  // ── Núcleo de marca ──
  static const grafito = Color(0xFF0E131B);
  static const carbon = Color(0xFF171E29);
  static const ambar = Color(0xFFFF8A2B);
  static const acero = Color(0xFF97A3B2);
  static const hueso = Color(0xFFEFF3F8);

  // ── Rampa grafito ──
  static const graphite900 = Color(0xFF0A0E15);
  static const graphite850 = Color(0xFF0E131B);
  static const graphite800 = Color(0xFF121826);
  static const graphite750 = Color(0xFF171E29);
  static const graphite700 = Color(0xFF1D2533);
  static const graphite600 = Color(0xFF283142);
  static const graphite300 = Color(0xFF6B788C);
  static const graphite200 = Color(0xFF97A3B2);
  static const graphite100 = Color(0xFFC3CCD7);
  static const graphite050 = Color(0xFFEFF3F8);

  // ── Rampa ámbar ──
  static const amber600 = Color(0xFFE06E1C);
  static const amber500 = Color(0xFFFF8A2B);
  static const amber400 = Color(0xFFFFA255);
  static const amberGlow = Color(0x29FF8A2B); // rgba(255,138,43,.16)

  // ── Semánticos ──
  static const signalOk = Color(0xFF4FB286);
  static const signalError = Color(0xFFE5604D);
  static const signalInfo = Color(0xFF5E9BD6);

  // ── Alias semánticos: usar estos en los componentes ──
  static const surfaceBase = graphite850;
  static const surfaceSunken = graphite900;
  static const surfaceCard = graphite750;
  static const surfaceRaised = graphite700;
  static const surfaceInset = graphite800;

  static const textStrong = graphite050;
  static const textBody = graphite100;
  static const textMuted = graphite200;
  static const textFaint = graphite300;
  static const textOnAmber = graphite900;

  static const borderSubtle = Color(0x1F97A3B2); // .12
  static const borderDefault = Color(0x3397A3B2); // .20
  static const borderStrong = Color(0x5C97A3B2); // .36

  static const accent = amber500;
  static const accentHover = amber400;
  static const accentPress = amber600;

  static const codeBg = graphite900;
}
```

`mobile/lib/theme/korven_spacing.dart`:

```dart
/// Escala de 8px y radios del sistema Korven (tokens/spacing.css).
abstract final class KorvenSpacing {
  static const s1 = 4.0;
  static const s2 = 8.0;
  static const s3 = 12.0;
  static const s4 = 16.0;
  static const s5 = 24.0;
  static const s6 = 32.0;
  static const s7 = 48.0;
  static const s8 = 64.0;
  static const s9 = 96.0;
}

/// Radios pequeños y precisos: la marca es facetada, no redondeada.
abstract final class KorvenRadius {
  static const xs = 3.0;
  static const sm = 5.0;
  static const md = 8.0;
  static const lg = 12.0;
  static const xl = 18.0;
}
```

`mobile/lib/theme/korven_motion.dart`:

```dart
import 'package:flutter/animation.dart';

/// Motion del sistema: calmado y preciso, nunca rebote de juguete.
abstract final class KorvenMotion {
  /// cubic-bezier(.16, 1, .3, 1) — el "settle" preciso de la marca.
  static const easeOut = Cubic(0.16, 1, 0.3, 1);
  static const easeInOut = Cubic(0.65, 0, 0.35, 1);

  static const fast = Duration(milliseconds: 120);
  static const base = Duration(milliseconds: 200);
  static const slow = Duration(milliseconds: 360);
}
```

- [ ] **Step 4: Correr el test y verlo pasar**

Run: `flutter test test/theme/korven_tokens_test.dart`
Esperado: PASS los 5.

Nota: el import del test usa `package:korven_open_tv/`, que aún no existe — el paquete se llama `iptv_ecosystem` hasta la Fase 6. **Usar `package:iptv_ecosystem/` en este paso** y dejar el renombrado para su fase; el `sed` global de la Task 20 lo actualizará junto al resto.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/theme/ mobile/test/theme/
git commit -m "mobile: portar los tokens de color, espaciado y motion de Korven"
```

---

### Task 2: Empaquetar las fuentes de marca

**Files:**
- Create: `mobile/assets/fonts/` (9 ficheros `.ttf`)
- Modify: `mobile/pubspec.yaml`
- Create: `mobile/lib/theme/korven_typography.dart`
- Test: `mobile/test/theme/korven_typography_test.dart`

**Interfaces:**
- Produces: `KorvenType` con roles semánticos (`display`, `h1`…`h4`, `body`, `bodySm`, `label`, `mono`, `monoLabel`) como `TextStyle`, y las constantes de familia `KorvenType.display/text/mono`.

**Contexto:** se empaquetan en vez de usar el paquete `google_fonts` porque ese descarga en el primer arranque — dependencia de red y parpadeo tipográfico innecesarios en una app de escritorio. El propio handoff contempla el self-hosting.

- [ ] **Step 1: Descargar las fuentes**

Pesos que usa el sistema: Space Grotesk 500/600/700, Inter 400/500/600, JetBrains Mono 400/500/700.

```bash
cd /Users/usuario/Dev/ip-tv/mobile && mkdir -p assets/fonts && cd assets/fonts
for repo in \
  "google/fonts/main/ofl/spacegrotesk/SpaceGrotesk%5Bwght%5D.ttf" \
  "google/fonts/main/ofl/jetbrainsmono/JetBrainsMono%5Bwght%5D.ttf"; do
  curl -sL -O "https://raw.githubusercontent.com/$repo"
done
curl -sL -o "Inter.ttf" "https://raw.githubusercontent.com/google/fonts/main/ofl/inter/Inter%5Bopsz,wght%5D.ttf"
ls -la
```

Son fuentes variables (un fichero por familia cubre todos los pesos), lo que simplifica el `pubspec`. Verificar que los tres ficheros pesan >100KB; si alguna URL cambió, buscar la ruta actual en `github.com/google/fonts/tree/main/ofl`.

- [ ] **Step 2: Declararlas en pubspec**

En `mobile/pubspec.yaml`, dentro de `flutter:`:

```yaml
  fonts:
    - family: SpaceGrotesk
      fonts:
        - asset: assets/fonts/SpaceGrotesk[wght].ttf
    - family: Inter
      fonts:
        - asset: assets/fonts/Inter.ttf
    - family: JetBrainsMono
      fonts:
        - asset: assets/fonts/JetBrainsMono[wght].ttf
```

- [ ] **Step 3: Escribir el test**

```dart
// mobile/test/theme/korven_typography_test.dart
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
```

- [ ] **Step 4: Correr y ver fallar**

Run: `flutter test test/theme/korven_typography_test.dart` → FAIL (no existe el archivo).

- [ ] **Step 5: Escribir la tipografía**

```dart
// mobile/lib/theme/korven_typography.dart
import 'package:flutter/material.dart';
import 'korven_colors.dart';

/// Tipografía de Korven (tokens/typography.css): Space Grotesk para display,
/// Inter para lectura, JetBrains Mono para lo que es literalmente dato.
abstract final class KorvenType {
  static const familyDisplay = 'SpaceGrotesk';
  static const familyText = 'Inter';
  static const familyMono = 'JetBrainsMono';

  // Escala de tercera mayor 1.250, base 16.
  static const size2xs = 11.0;
  static const sizeXs = 12.0;
  static const sizeSm = 14.0;
  static const sizeBase = 16.0;
  static const sizeMd = 18.0;
  static const sizeLg = 20.0;
  static const sizeXl = 25.0;
  static const size2xl = 31.0;
  static const size3xl = 39.0;

  static const h1 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w600,
    fontSize: size3xl,
    height: 1.05,
    letterSpacing: -0.02 * size3xl,
    color: KorvenColors.textStrong,
  );

  static const h2 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w600,
    fontSize: size2xl,
    height: 1.2,
    letterSpacing: -0.01 * size2xl,
    color: KorvenColors.textStrong,
  );

  static const h3 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w500,
    fontSize: sizeXl,
    height: 1.2,
    color: KorvenColors.textStrong,
  );

  static const h4 = TextStyle(
    fontFamily: familyDisplay,
    fontWeight: FontWeight.w500,
    fontSize: sizeMd,
    height: 1.2,
    color: KorvenColors.textStrong,
  );

  static const body = TextStyle(
    fontFamily: familyText,
    fontWeight: FontWeight.w400,
    fontSize: sizeBase,
    height: 1.5,
    color: KorvenColors.textBody,
  );

  static const bodySm = TextStyle(
    fontFamily: familyText,
    fontWeight: FontWeight.w400,
    fontSize: sizeSm,
    height: 1.5,
    color: KorvenColors.textBody,
  );

  static const label = TextStyle(
    fontFamily: familyText,
    fontWeight: FontWeight.w500,
    fontSize: sizeSm,
    height: 1.2,
    color: KorvenColors.textMuted,
  );

  static const mono = TextStyle(
    fontFamily: familyMono,
    fontWeight: FontWeight.w400,
    fontSize: sizeSm,
    height: 1.5,
    letterSpacing: 0.02 * sizeSm,
    color: KorvenColors.textMuted,
  );

  /// Eyebrows y etiquetas de consola: mono pequeño con tracking ancho.
  static const monoLabel = TextStyle(
    fontFamily: familyMono,
    fontWeight: FontWeight.w500,
    fontSize: sizeXs,
    height: 1.2,
    letterSpacing: 0.06 * sizeXs,
    color: KorvenColors.textMuted,
  );
}
```

- [ ] **Step 6: Correr y ver pasar**

Run: `flutter test test/theme/` → PASS.

- [ ] **Step 7: Commit**

```bash
git add mobile/assets/fonts mobile/pubspec.yaml mobile/lib/theme/ mobile/test/theme/
git commit -m "mobile: empaquetar Space Grotesk, Inter y JetBrains Mono como assets"
```

---

### Task 3: Construir el ThemeData y enchufarlo

**Files:**
- Create: `mobile/lib/theme/korven_theme.dart`
- Modify: `mobile/lib/main.dart`
- Test: `mobile/test/theme/korven_theme_test.dart`

**Interfaces:**
- Produces: `korvenTheme()` → `ThemeData`. Consume `KorvenColors`, `KorvenType`, `KorvenRadius`.

**Contexto:** hoy `main.dart` usa `ThemeData.dark(useMaterial3: true)` con `ColorScheme.fromSeed(seedColor: Colors.deepPurple)`. Ese morado es el que hace que la app parezca una plantilla.

- [ ] **Step 1: Escribir el test**

```dart
// mobile/test/theme/korven_theme_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/theme/korven_colors.dart';
import 'package:iptv_ecosystem/theme/korven_theme.dart';

void main() {
  test('el tema es oscuro y su canvas es grafito', () {
    final t = korvenTheme();
    expect(t.brightness, Brightness.dark);
    expect(t.scaffoldBackgroundColor, KorvenColors.surfaceBase);
  });

  test('el acento es ámbar, no el morado de la plantilla', () {
    final t = korvenTheme();
    expect(t.colorScheme.primary, KorvenColors.accent);
    // Regresión concreta: el seed deepPurple de ThemeData.dark por defecto.
    expect(t.colorScheme.primary, isNot(Colors.deepPurple));
  });

  test('el texto sobre ámbar es grafito, para que se lea', () {
    expect(korvenTheme().colorScheme.onPrimary, KorvenColors.textOnAmber);
  });

  test('el error usa el óxido de la marca, no el rojo de Material', () {
    expect(korvenTheme().colorScheme.error, KorvenColors.signalError);
  });
}
```

- [ ] **Step 2: Correr y ver fallar**

Run: `flutter test test/theme/korven_theme_test.dart` → FAIL.

- [ ] **Step 3: Escribir el tema**

```dart
// mobile/lib/theme/korven_theme.dart
import 'package:flutter/material.dart';
import 'korven_colors.dart';
import 'korven_spacing.dart';
import 'korven_typography.dart';

/// ThemeData de Korven Open TV. Solo oscuro: la marca es un canvas de grafito
/// y un reproductor de vídeo se mira a oscuras.
ThemeData korvenTheme() {
  const scheme = ColorScheme.dark(
    primary: KorvenColors.accent,
    onPrimary: KorvenColors.textOnAmber,
    secondary: KorvenColors.accent,
    onSecondary: KorvenColors.textOnAmber,
    surface: KorvenColors.surfaceBase,
    onSurface: KorvenColors.textBody,
    error: KorvenColors.signalError,
    onError: KorvenColors.textStrong,
    outline: KorvenColors.borderDefault,
  );

  return ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    colorScheme: scheme,
    scaffoldBackgroundColor: KorvenColors.surfaceBase,
    canvasColor: KorvenColors.surfaceBase,
    dividerColor: KorvenColors.borderSubtle,
    fontFamily: KorvenType.familyText,
    textTheme: const TextTheme(
      headlineLarge: KorvenType.h2,
      headlineMedium: KorvenType.h3,
      titleLarge: KorvenType.h4,
      bodyLarge: KorvenType.body,
      bodyMedium: KorvenType.bodySm,
      labelLarge: KorvenType.label,
    ),
    appBarTheme: const AppBarTheme(
      backgroundColor: KorvenColors.surfaceSunken,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      foregroundColor: KorvenColors.textStrong,
    ),
    iconTheme: const IconThemeData(color: KorvenColors.textMuted),
    dialogTheme: DialogThemeData(
      backgroundColor: KorvenColors.surfaceCard,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(KorvenRadius.lg),
        side: const BorderSide(color: KorvenColors.borderDefault),
      ),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: KorvenColors.accent,
        foregroundColor: KorvenColors.textOnAmber,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(KorvenRadius.md),
        ),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(foregroundColor: KorvenColors.accent),
    ),
    progressIndicatorTheme:
        const ProgressIndicatorThemeData(color: KorvenColors.accent),
  );
}
```

- [ ] **Step 4: Enchufarlo en main.dart**

Sustituir el bloque `theme:` de `MyApp` por `theme: korvenTheme()`, añadir el import, y quitar el `ColorScheme.fromSeed` con `deepPurple`.

- [ ] **Step 5: Correr todo**

Run: `flutter test && flutter analyze`
Esperado: verde. Algunos tests de widget existentes pueden depender de colores por defecto; si alguno falla por color, actualizarlo — el tema nuevo es la referencia correcta ahora.

- [ ] **Step 6: Verificar a ojo**

Arrancar la app. Debe verse ya grafito en vez de gris Material, sin morado en ningún sitio. Todavía sin rediseñar los componentes: es esperado.

- [ ] **Step 7: Commit**

```bash
git add mobile/lib/main.dart mobile/lib/theme/ mobile/test/theme/
git commit -m "mobile: ThemeData de Korven, fuera el morado de plantilla"
```

---

# FASE 2 — Marcas de la casa y barra de consola

### Task 4: Wordmark con la "O" hexagonal

**Files:**
- Create: `mobile/lib/presentation/widgets/korven_wordmark.dart`
- Test: `mobile/test/presentation/korven_wordmark_test.dart`

**Interfaces:**
- Produces: `KorvenWordmark({double fontSize = 21, String suffix = 'open tv'})`.

**Contexto:** el lockup de marca es `KORVEN` en Space Grotesk 700 donde la **O es un hexágono** relleno de acero con un nodo ámbar de 6px al centro, seguido del sufijo en mono. En el sitio el sufijo es `.dev`; aquí es `open tv`. El hexágono se dibuja con `ClipPath`, replicando el `clip-path: polygon(50% 0,100% 27%,100% 73%,50% 100%,0 73%,0 27%)` del CSS — sin assets, conservando la webfont.

- [ ] **Step 1: Escribir el test**

```dart
// mobile/test/presentation/korven_wordmark_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/presentation/widgets/korven_wordmark.dart';

void main() {
  testWidgets('muestra el nombre de marca partido alrededor de la O hexagonal',
      (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: Center(child: KorvenWordmark())),
    ));

    // La "O" es un hexágono dibujado, así que el texto va en dos piezas.
    expect(find.text('K'), findsOneWidget);
    expect(find.text('RVEN'), findsOneWidget);
    expect(find.text('open tv'), findsOneWidget);
  });

  testWidgets('el sufijo es configurable', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: Center(child: KorvenWordmark(suffix: '.dev'))),
    ));
    expect(find.text('.dev'), findsOneWidget);
  });
}
```

- [ ] **Step 2: Correr y ver fallar**

Run: `flutter test test/presentation/korven_wordmark_test.dart` → FAIL.

- [ ] **Step 3: Implementar**

```dart
// mobile/lib/presentation/widgets/korven_wordmark.dart
import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_typography.dart';

/// Recorta un hexágono con la misma geometría que el clip-path del sistema:
/// polygon(50% 0, 100% 27%, 100% 73%, 50% 100%, 0 73%, 0 27%).
class _HexClipper extends CustomClipper<Path> {
  @override
  Path getClip(Size s) => Path()
    ..moveTo(s.width * .5, 0)
    ..lineTo(s.width, s.height * .27)
    ..lineTo(s.width, s.height * .73)
    ..lineTo(s.width * .5, s.height)
    ..lineTo(0, s.height * .73)
    ..lineTo(0, s.height * .27)
    ..close();

  @override
  bool shouldReclip(covariant CustomClipper<Path> old) => false;
}

/// Lockup de marca: KORVEN con la O como hexágono de acero con el nodo ámbar
/// al centro, más el sufijo en mono. Es la firma de Korven; no sustituir por
/// un texto plano.
class KorvenWordmark extends StatelessWidget {
  const KorvenWordmark({super.key, this.fontSize = 21, this.suffix = 'open tv'});

  final double fontSize;
  final String suffix;

  @override
  Widget build(BuildContext context) {
    final letra = TextStyle(
      fontFamily: KorvenType.familyDisplay,
      fontWeight: FontWeight.w700,
      fontSize: fontSize,
      height: 1,
      color: KorvenColors.textStrong,
    );
    final hex = fontSize * 0.86;
    final nodo = fontSize * 0.28;

    return Row(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Text('K', style: letra),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: fontSize * 0.04),
          child: SizedBox(
            width: hex,
            height: hex,
            child: Stack(
              alignment: Alignment.center,
              children: [
                ClipPath(
                  clipper: _HexClipper(),
                  child: Container(color: KorvenColors.graphite600),
                ),
                // El nodo: el punto focal de la marca.
                Container(
                  width: nodo,
                  height: nodo,
                  decoration: BoxDecoration(
                    color: KorvenColors.accent,
                    shape: BoxShape.circle,
                    boxShadow: [
                      BoxShadow(
                        color: KorvenColors.accent.withValues(alpha: 0.45),
                        blurRadius: nodo,
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
        Text('RVEN', style: letra),
        SizedBox(width: fontSize * 0.34),
        Text(
          suffix,
          style: KorvenType.monoLabel.copyWith(color: KorvenColors.textMuted),
        ),
      ],
    );
  }
}
```

- [ ] **Step 4: Correr y ver pasar**

Run: `flutter test test/presentation/korven_wordmark_test.dart` → PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/presentation/widgets/korven_wordmark.dart mobile/test/presentation/korven_wordmark_test.dart
git commit -m "mobile: wordmark de Korven con la O hexagonal y el nodo ámbar"
```

---

### Task 5: Emblema del cuervo

**Files:**
- Create: `mobile/lib/presentation/widgets/korven_emblem.dart`
- Test: `mobile/test/presentation/korven_emblem_test.dart`

**Interfaces:**
- Produces: `KorvenEmblem({double size = 128})`.

**Contexto:** `korven-emblem.svg` son 949 bytes: un hexágono relleno carbón con borde hueso, cinco polilíneas facetadas desde el centro, y el ojo ámbar. Se replica con `CustomPainter` — sin `flutter_svg`, porque en una app de escritorio cada dependencia es pasivo. Fuente de verdad: `/Users/usuario/Dev/korven/design/design_handoff_korven_sitio/assets/korven-emblem.svg` (viewBox 0 0 128 128).

- [ ] **Step 1: Escribir el test**

```dart
// mobile/test/presentation/korven_emblem_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/presentation/widgets/korven_emblem.dart';

void main() {
  testWidgets('se dibuja en el tamaño pedido y es accesible', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(body: Center(child: KorvenEmblem(size: 96))),
    ));

    final box = tester.getSize(find.byType(KorvenEmblem));
    expect(box.width, 96);
    expect(box.height, 96);
    // El emblema es la marca; para un lector de pantalla tiene que anunciarse.
    expect(find.bySemanticsLabel('Emblema de Korven'), findsOneWidget);
  });
}
```

- [ ] **Step 2: Correr y ver fallar**

Run: `flutter test test/presentation/korven_emblem_test.dart` → FAIL.

- [ ] **Step 3: Implementar**

```dart
// mobile/lib/presentation/widgets/korven_emblem.dart
import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';

/// Emblema hexagonal facetado de Korven, portado de assets/korven-emblem.svg
/// (viewBox 128×128). Se dibuja en vez de cargarse como SVG: son cuatro trazos
/// y evita una dependencia entera.
///
/// El ojo se queda quieto. En el sitio sigue al cursor, que es encantador en
/// una one-page y molesto en una herramienta.
class _EmblemPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final k = size.width / 128.0;
    Offset p(double x, double y) => Offset(x * k, y * k);

    final hex = Path()
      ..moveTo(64 * k, 12 * k)
      ..lineTo(110 * k, 38 * k)
      ..lineTo(110 * k, 90 * k)
      ..lineTo(64 * k, 116 * k)
      ..lineTo(18 * k, 90 * k)
      ..lineTo(18 * k, 38 * k)
      ..close();

    canvas.drawPath(hex, Paint()..color = KorvenColors.carbon);
    canvas.drawPath(
      hex,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3 * k
        ..strokeJoin = StrokeJoin.round
        ..color = KorvenColors.hueso,
    );

    // Facetas desde el centro: le dan el volumen tallado.
    final faceta = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2 * k
      ..strokeJoin = StrokeJoin.round
      ..color = KorvenColors.graphite600;
    const centro = Offset(64, 64);
    for (final destino in const [
      Offset(64, 12),
      Offset(110, 38),
      Offset(110, 90),
      Offset(18, 90),
      Offset(18, 38),
    ]) {
      canvas.drawLine(p(centro.dx, centro.dy), p(destino.dx, destino.dy), faceta);
    }

    // El ojo: halo, anillo y núcleo — el único ámbar del emblema.
    canvas.drawCircle(p(64, 64), 22 * k,
        Paint()..color = KorvenColors.accent.withValues(alpha: 0.16));
    canvas.drawCircle(
      p(64, 64),
      15 * k,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3 * k
        ..color = KorvenColors.accent.withValues(alpha: 0.30),
    );
    canvas.drawCircle(p(64, 64), 9 * k, Paint()..color = KorvenColors.accent);
  }

  @override
  bool shouldRepaint(covariant CustomPainter old) => false;
}

class KorvenEmblem extends StatelessWidget {
  const KorvenEmblem({super.key, this.size = 128});

  final double size;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: 'Emblema de Korven',
      image: true,
      child: SizedBox(
        width: size,
        height: size,
        child: CustomPaint(painter: _EmblemPainter()),
      ),
    );
  }
}
```

- [ ] **Step 4: Correr y ver pasar**

Run: `flutter test test/presentation/korven_emblem_test.dart` → PASS.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib/presentation/widgets/korven_emblem.dart mobile/test/presentation/korven_emblem_test.dart
git commit -m "mobile: emblema del cuervo con CustomPainter, sin dependencias"
```

---

### Task 6: Barra de consola (AppBar)

**Files:**
- Create: `mobile/lib/presentation/widgets/console_bar.dart`
- Modify: `mobile/lib/presentation/screens/home_screen.dart` (líneas ~76-125)
- Modify: `mobile/test/presentation/home_screen_test.dart`

**Interfaces:**
- Consumes: `KorvenWordmark`.
- Produces: `ConsoleBar` como `PreferredSizeWidget`, con los mismos callbacks que el AppBar actual: buscar, toggle offline, recargar.

**Contexto:** el AppBar actual muestra `Text('IPTV')` y cuatro `IconButton` sin tratamiento. Los tests existentes lo localizan por tooltip (`'Buscar canal'`, `'Mostrar canales offline'`, `'Recargar'`) — **conservar esos tooltips** o los tests rompen; se conservan porque además son correctos.

- [ ] **Step 1: Escribir el test**

Añadir a `mobile/test/presentation/home_screen_test.dart`:

```dart
  testWidgets('la barra muestra el lockup de marca', (tester) async {
    await pumpHome(tester, FakeRepo(total: 4));

    expect(find.text('RVEN'), findsOneWidget);
    expect(find.text('open tv'), findsOneWidget);
    // El título de plantilla ya no está.
    expect(find.text('IPTV'), findsNothing);
  });

  testWidgets('el toggle de offline se pinta ámbar solo cuando está activo',
      (tester) async {
    await pumpHome(tester, FakeRepo(total: 4));

    Color colorDelIcono(String tooltip) {
      final icon = tester.widget<Icon>(
        find.descendant(
            of: find.byTooltip(tooltip), matching: find.byType(Icon)),
      );
      return icon.color!;
    }

    final apagado = colorDelIcono('Mostrar canales offline');
    await tester.tap(find.byTooltip('Mostrar canales offline'));
    await tester.pumpAndSettle();
    final encendido = colorDelIcono('Ocultar canales offline');

    expect(apagado, isNot(encendido),
        reason: 'el estado activo tiene que distinguirse, y en ámbar');
  });
```

- [ ] **Step 2: Correr y ver fallar**

Run: `flutter test test/presentation/home_screen_test.dart` → FAIL (sigue el título 'IPTV').

- [ ] **Step 3: Implementar `ConsoleBar`**

```dart
// mobile/lib/presentation/widgets/console_bar.dart
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
  });

  final bool searching;
  final TextEditingController searchController;
  final ValueChanged<String> onSearchChanged;
  final VoidCallback onOpenSearch;
  final VoidCallback onCloseSearch;
  final bool showOffline;
  final VoidCallback onToggleOffline;
  final VoidCallback onReload;

  @override
  Size get preferredSize => const Size.fromHeight(60);

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 60,
      padding: const EdgeInsets.symmetric(horizontal: KorvenSpacing.s5),
      decoration: const BoxDecoration(
        color: KorvenColors.surfaceSunken,
        border: Border(
          bottom: BorderSide(color: KorvenColors.borderSubtle),
        ),
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
            _AccionBarra(
              icon: Icons.search,
              tooltip: 'Buscar canal',
              onPressed: onOpenSearch,
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
```

- [ ] **Step 4: Sustituir en `home_screen.dart`**

Cambiar `appBar: AppBar(...)` (el bloque de ~50 líneas) por `appBar: ConsoleBar(...)` pasando el estado y los callbacks que ya existen en `_HomeScreenState` (`_searching`, `_searchCtrl`, `_onSearchChanged`, `_closeSearch`, `showOfflineProvider`, `ref.invalidate(channelListProvider)`).

- [ ] **Step 5: Correr y ver pasar**

Run: `flutter test && flutter analyze` → verde.

- [ ] **Step 6: Commit**

```bash
git add mobile/lib/presentation/ mobile/test/presentation/
git commit -m "mobile: barra de consola con el lockup de marca"
```

---

# FASE 3 — Filtros

### Task 7: Contador de resultados honesto en el gateway

**Files:**
- Modify: `gateway/internal/ports/channel_repository.go`
- Modify: `gateway/internal/adapters/db/channel_repository.go`
- Modify: `gateway/internal/api/handlers/channel_handler.go`
- Test: `gateway/internal/adapters/db/channel_repository_test.go`, `gateway/internal/api/handlers/channel_handler_test.go`

**Interfaces:**
- Produces: `ChannelRepository.CountFiltered(ctx, ports.ChannelFilter) (int, error)`, y la cabecera de respuesta `X-Total-Count` en `GET /channels`.

**Contexto — por qué esta tarea existe.** El diseño pide un contador de resultados en la barra de filtros. La app solo conoce `state.channels.length`, que son las páginas ya cargadas, no cuántos canales casan con el filtro. Mostrar ese número diría "500 canales" con 12 000 detrás: sería mentira. Hace falta el total real.

Se devuelve como cabecera y no como cuerpo para no romper el contrato JSON congelado por `domain/channel_test.go`.

- [ ] **Step 1: Escribir el test del repositorio**

Añadir a `gateway/internal/adapters/db/channel_repository_test.go`:

```go
// CountFiltered tiene que aplicar exactamente los mismos filtros que
// FindFiltered, o el contador de la app mentiría.
func TestCountFilteredCoincideConFindFiltered(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)

	for i, pais := range []string{"ES", "ES", "MX", "GB"} {
		id := "ch-" + strconv.Itoa(i)
		ch := makeChannel(id, "Canal "+id+" (1080p)", pais, "news")
		if err := chRepo.Save(ctx, ch); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	casos := []ports.ChannelFilter{
		{},
		{Country: "ES"},
		{Country: "ES", MinQuality: "fhd"},
		{Query: "Canal", AliveOnly: true},
		{Country: "NO-EXISTE"},
	}
	for _, f := range casos {
		// Limit alto para que FindFiltered devuelva todo lo que casa.
		conLimite := f
		conLimite.Limit = 1000
		encontrados, err := chRepo.FindFiltered(ctx, conLimite)
		if err != nil {
			t.Fatalf("FindFiltered(%+v): %v", f, err)
		}
		total, err := chRepo.CountFiltered(ctx, f)
		if err != nil {
			t.Fatalf("CountFiltered(%+v): %v", f, err)
		}
		if total != len(encontrados) {
			t.Errorf("filtro %+v: count = %d, FindFiltered devolvió %d", f, total, len(encontrados))
		}
	}
}

// El contador ignora paginación: es el total que casa, no la página.
func TestCountFilteredIgnoraLimitYOffset(t *testing.T) {
	ctx := context.Background()
	chRepo, stRepo := openStreamTestRepos(t)
	for i := 0; i < 5; i++ {
		id := "ch-" + strconv.Itoa(i)
		if err := chRepo.Save(ctx, makeChannel(id, "Canal "+id, "ES", "news")); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := stRepo.Save(ctx, makeStream("st-"+id, id, "http://a/"+id+".m3u8")); err != nil {
			t.Fatalf("Save stream: %v", err)
		}
	}

	total, err := chRepo.CountFiltered(ctx, ports.ChannelFilter{Limit: 2, Offset: 3})
	if err != nil {
		t.Fatalf("CountFiltered: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, quiero 5 (limit y offset no deben afectar)", total)
	}
}
```

Añadir el import de `strconv` si falta.

- [ ] **Step 2: Correr y ver fallar**

Run: `cd gateway && go test ./internal/adapters/db/ -run TestCountFiltered` → FAIL, `CountFiltered undefined`.

- [ ] **Step 3: Extraer la construcción del WHERE y añadir `CountFiltered`**

En `channel_repository.go`, `FindFiltered` construye hoy su `where`/`args` en línea. Extraer ese bloque a un helper para que `CountFiltered` use **exactamente** la misma lógica — duplicarla es justo cómo el contador acabaría mintiendo:

```go
// buildChannelWhere arma la cláusula WHERE compartida por FindFiltered y
// CountFiltered. Vive en un solo sitio a propósito: si los dos construyeran el
// filtro por su cuenta, el contador acabaría discrepando de la lista.
func buildChannelWhere(f ports.ChannelFilter) (string, []any) {
	var where []string
	var args []any

	if f.Query != "" {
		where = append(where, "name LIKE ? COLLATE NOCASE")
		args = append(args, "%"+f.Query+"%")
	}
	if f.Country != "" {
		where = append(where, "country_code = ?")
		args = append(args, f.Country)
	}
	if f.Category != "" {
		where = append(where, "category_id = ?")
		args = append(args, f.Category)
	}
	if clause, qargs := qualityWhereClause(f.MinQuality); clause != "" {
		where = append(where, clause)
		args = append(args, qargs...)
	}
	if f.AliveOnly {
		where = append(where, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM streams s
			WHERE s.channel_id = channels.id
			  AND (s.is_alive = 1 OR s.fail_count < %d)
		)`, DeadFailThreshold))
	}

	if len(where) == 0 {
		return "1=1", args
	}
	return strings.Join(where, " AND "), args
}

// CountFiltered devuelve cuántos canales casan con el filtro, ignorando
// paginación. Lo consume la barra de filtros de la app para mostrar un total
// veraz en vez del número de páginas ya cargadas.
func (r *SQLiteChannelRepository) CountFiltered(ctx context.Context, f ports.ChannelFilter) (int, error) {
	f = f.Normalize()
	whereSQL, args := buildChannelWhere(f)

	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM channels WHERE "+whereSQL, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("db.CountFiltered: %w", err)
	}
	return n, nil
}
```

Reescribir `FindFiltered` para que llame a `buildChannelWhere` en vez de construir el WHERE en línea, conservando el resto (columnas de salud incrustadas, `ORDER BY name COLLATE NOCASE, id LIMIT ? OFFSET ?`).

Añadir a la interfaz en `ports/channel_repository.go`:

```go
	// CountFiltered cuenta los canales que casan con el filtro, sin paginar.
	CountFiltered(ctx context.Context, f ChannelFilter) (int, error)
```

Actualizar los fakes que implementan `ChannelRepository` en tests (`services/syncer_test.go`, `api/handlers/channel_handler_test.go`) con una implementación que devuelva `len(...)` o 0.

- [ ] **Step 4: Correr y ver pasar**

Run: `go test ./internal/adapters/db/ -run TestCountFiltered -v` → PASS los dos.

- [ ] **Step 5: Exponerlo en el handler**

En `GetChannels`, tras obtener los canales:

```go
	// Total real de coincidencias, para que la app muestre un contador veraz en
	// vez del número de canales que lleva cargados. Va en cabecera y no en el
	// cuerpo para no tocar el contrato JSON congelado.
	if total, err := h.repo.CountFiltered(r.Context(), filtro); err == nil {
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
	} else {
		h.logger.Error("GetChannels: fallo contando el total", slog.Any("error", err))
	}
```

Extraer el `ports.ChannelFilter` a una variable `filtro` para poder pasarlo a ambas llamadas.

Test en `channel_handler_test.go`:

```go
func TestGetChannelsDevuelveElTotalEnCabecera(t *testing.T) {
	r := setupRouter()
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/channels", nil))

	if got := rr.Header().Get("X-Total-Count"); got == "" {
		t.Error("falta X-Total-Count; la app lo necesita para el contador")
	}
}
```

- [ ] **Step 6: Suite completa y commit**

```bash
go test -race -count=1 ./... && go vet ./... && gofmt -l .
git add gateway/
git commit -m "gateway: X-Total-Count en /channels para un contador veraz"
```

---

### Task 8: Consumir el total en la app

**Files:**
- Modify: `mobile/lib/data/repositories/channel_repository.dart`
- Modify: `mobile/lib/presentation/providers/channel_provider.dart`
- Test: `mobile/test/data/channel_repository_test.dart`, `mobile/test/presentation/channel_list_notifier_test.dart`

**Interfaces:**
- Produces: `ChannelPage({required List<Channel> channels, required int total})`. `IChannelRepository.getChannels` pasa a devolver `Future<ChannelPage>`. `ChannelListState` gana `total`.

**Contexto:** es un cambio de firma que toca los fakes de test. Se hace en su propia tarea para que el fallo, si lo hay, sea localizable.

- [ ] **Step 1: Escribir el test del repositorio**

Añadir a `mobile/test/data/channel_repository_test.dart`:

```dart
  test('lee el total de la cabecera X-Total-Count', () async {
    final repo = ChannelRepository(
      baseUrl: 'http://x',
      client: MockClient((req) async => http.Response(
            '[]',
            200,
            headers: {'x-total-count': '1284'},
          )),
    );

    final page = await repo.getChannels();
    expect(page.total, 1284);
  });

  test('sin la cabecera, el total cae al número de canales recibidos', () async {
    final repo = ChannelRepository(
      baseUrl: 'http://x',
      client: MockClient((req) async => http.Response(
            '[{"ID":"a","Name":"A"}]',
            200,
          )),
    );

    // Degradación honesta: mejor un total bajo que un crash o un cero falso.
    final page = await repo.getChannels();
    expect(page.total, 1);
  });
```

- [ ] **Step 2: Correr y ver fallar**

Run: `flutter test test/data/channel_repository_test.dart` → FAIL.

- [ ] **Step 3: Implementar**

En `channel_repository.dart`, añadir el tipo y cambiar la firma:

```dart
/// Una página de canales más el total de coincidencias del filtro. El total lo
/// da el gateway en X-Total-Count; sin él, la app solo sabría cuántos lleva
/// cargados y el contador mentiría.
class ChannelPage {
  const ChannelPage({required this.channels, required this.total});
  final List<Channel> channels;
  final int total;
}
```

`getChannels` devuelve `ChannelPage`, leyendo `response.headers['x-total-count']` con `int.tryParse` y cayendo a `data.length` si no está.

En `channel_provider.dart`: `ChannelListState` gana `final int total`, y `build()`/`loadMore()` lo propagan desde la página.

Actualizar `IChannelRepository`, el `FakeRepo` de los tests y `RepoColgado` de `player_screen_test`.

- [ ] **Step 4: Correr y ver pasar**

Run: `flutter test && flutter analyze` → verde.

- [ ] **Step 5: Commit**

```bash
git add mobile/lib mobile/test
git commit -m "mobile: la capa de datos expone el total de coincidencias"
```

---

### Task 9: La barra de filtros nueva

**Files:**
- Create: `mobile/lib/presentation/widgets/filter_bar.dart`
- Create: `mobile/lib/presentation/widgets/korven_chip.dart`
- Modify: `mobile/lib/presentation/screens/home_screen.dart` (borrar `_FilterBar`, `_FilterButton`, líneas ~258-416)
- Test: `mobile/test/presentation/filter_bar_test.dart`

**Interfaces:**
- Consumes: `channelFilterProvider`, `channelListProvider`, `KorvenChip`.
- Produces: `FilterBar` (ConsumerWidget), `KorvenChip({label, active, onTap, onRemove})`.

**Contexto — el problema.** Hoy la calidad son cuatro chips siempre visibles, país y categoría se esconden tras diálogos, no hay forma de ver qué está activo ni de limpiarlo. Es el punto débil real de la experiencia.

**El diseño**, tomado de la consola de casos del sitio:

```
// filtros                                         1 284 canales

[ país: MX × ]  [ categoría: noticias × ]                 limpiar

[ 4K ]  [ 1080p+ ]  [ HD 720p+ ]  [ todos ]
```

- [ ] **Step 1: Escribir los tests**

```dart
// mobile/test/presentation/filter_bar_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:iptv_ecosystem/domain/models/channel_filter.dart';
import 'package:iptv_ecosystem/presentation/providers/channel_provider.dart';
import 'package:iptv_ecosystem/presentation/widgets/filter_bar.dart';

import 'channel_list_notifier_test.dart' show FakeRepo;

Future<ProviderContainer> _pump(WidgetTester tester, FakeRepo repo) async {
  SharedPreferences.setMockInitialValues({});
  final prefs = await SharedPreferences.getInstance();
  final container = ProviderContainer(overrides: [
    channelRepositoryProvider.overrideWithValue(repo),
    sharedPreferencesProvider.overrideWithValue(prefs),
  ]);
  addTearDown(container.dispose);
  await tester.pumpWidget(UncontrolledProviderScope(
    container: container,
    child: const MaterialApp(home: Scaffold(body: FilterBar())),
  ));
  await tester.pumpAndSettle();
  return container;
}

void main() {
  testWidgets('una faceta activa se muestra como chip removible',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(country: 'MX');
    await tester.pumpAndSettle();

    expect(find.textContaining('MX'), findsOneWidget);

    // La × la quita, sin abrir ningún diálogo.
    await tester.tap(find.byTooltip('Quitar filtro de país'));
    await tester.pumpAndSettle();
    expect(container.read(channelFilterProvider).country, isEmpty);
  });

  testWidgets('limpiar solo aparece cuando hay algo que limpiar',
      (tester) async {
    final container = await _pump(tester, FakeRepo(total: 20));
    expect(find.text('limpiar'), findsNothing);

    container.read(channelFilterProvider.notifier).state =
        const ChannelFilter(country: 'MX', category: 'news');
    await tester.pumpAndSettle();
    expect(find.text('limpiar'), findsOneWidget);

    await tester.tap(find.text('limpiar'));
    await tester.pumpAndSettle();

    final f = container.read(channelFilterProvider);
    expect(f.country, isEmpty);
    expect(f.category, isEmpty);
    expect(f.quality, 'fhd', reason: 'limpiar devuelve la calidad al default');
  });

  testWidgets('el contador muestra el total del gateway, no lo cargado',
      (tester) async {
    // FakeRepo devuelve 500 por página pero 1200 de total.
    await _pump(tester, FakeRepo(total: 1200));
    expect(find.textContaining('1200'), findsOneWidget);
  });
}
```

- [ ] **Step 2: Correr y ver fallar**

Run: `flutter test test/presentation/filter_bar_test.dart` → FAIL.

- [ ] **Step 3: Implementar `KorvenChip`**

```dart
// mobile/lib/presentation/widgets/korven_chip.dart
import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_motion.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';

/// Chip del sistema Korven: mono, borde hairline, ámbar cuando está activo.
/// Con [onRemove] se convierte en faceta removible (lleva una × al final).
class KorvenChip extends StatefulWidget {
  const KorvenChip({
    super.key,
    required this.label,
    this.active = false,
    this.onTap,
    this.onRemove,
    this.removeTooltip,
  });

  final String label;
  final bool active;
  final VoidCallback? onTap;
  final VoidCallback? onRemove;
  final String? removeTooltip;

  @override
  State<KorvenChip> createState() => _KorvenChipState();
}

class _KorvenChipState extends State<KorvenChip> {
  bool _hover = false;

  @override
  Widget build(BuildContext context) {
    final activo = widget.active;
    final borde = activo
        ? KorvenColors.accent
        : (_hover ? KorvenColors.accent : KorvenColors.borderDefault);
    final texto = activo ? KorvenColors.accent : KorvenColors.textMuted;

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: KorvenMotion.fast,
          curve: KorvenMotion.easeOut,
          padding: const EdgeInsets.symmetric(
              horizontal: KorvenSpacing.s4, vertical: 9),
          decoration: BoxDecoration(
            color: activo
                ? KorvenColors.accent.withValues(alpha: 0.10)
                : Colors.transparent,
            border: Border.all(color: borde),
            borderRadius: BorderRadius.circular(KorvenRadius.sm),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(widget.label,
                  style: KorvenType.mono.copyWith(fontSize: 13, color: texto)),
              if (widget.onRemove != null) ...[
                const SizedBox(width: KorvenSpacing.s2),
                Tooltip(
                  message: widget.removeTooltip ?? 'Quitar filtro',
                  child: InkWell(
                    onTap: widget.onRemove,
                    child: Icon(Icons.close, size: 13, color: texto),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
```

- [ ] **Step 4: Implementar `FilterBar`**

```dart
// mobile/lib/presentation/widgets/filter_bar.dart
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
/// indicaba qué estaba activo ni permitía limpiarlo.
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
      // Añadir facetas que aún no están puestas.
      if (filtro.country.isEmpty)
        KorvenChip(
          label: '+ país',
          onTap: () async {
            final v = await mostrarPicker(context, ref, PickerTipo.pais);
            if (v != null) aplicar(filtro.copyWith(country: v));
          },
        ),
      if (filtro.category.isEmpty)
        KorvenChip(
          label: '+ categoría',
          onTap: () async {
            final v = await mostrarPicker(context, ref, PickerTipo.categoria);
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
                TextSpan(text: 'filtros', style: KorvenType.monoLabel),
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
```

El selector de país/categoría se extrae de `home_screen.dart` a
`picker_dialog.dart`, exponiendo `Future<String?> mostrarPicker(context, ref,
PickerTipo)` y un `enum PickerTipo { pais, categoria }`. Se conserva su lógica
(lista filtrable) y se le aplica el tema; el diálogo actual tiene un
`SizedBox(width: 320, height: 400)` fijo que desborda en ventana baja, así que
pasa a `ConstrainedBox` con `maxHeight: MediaQuery.of(context).size.height * .6`.

- [ ] **Step 5: Borrar el código viejo de `home_screen.dart`**

Eliminar `_FilterBar` y `_FilterButton` (líneas ~258-416) y usar `const FilterBar()`.

- [ ] **Step 6: Correr y ver pasar**

Run: `flutter test && flutter analyze` → verde.

- [ ] **Step 7: Verificación por mutación**

El test del contador tiene que caerse si alguien vuelve a usar lo cargado en vez del total. Cambiar temporalmente el widget para pintar `state.channels.length`, comprobar que el test falla, y restaurar.

- [ ] **Step 8: Commit**

```bash
git add mobile/lib/presentation mobile/test/presentation
git commit -m "mobile: filtros con facetas visibles, removibles y contador real"
```

---

# FASE 4 — Filas y estados

### Task 10: Fila de canal

**Files:**
- Create: `mobile/lib/presentation/widgets/channel_row.dart`
- Modify: `mobile/lib/presentation/screens/home_screen.dart` (`_ChannelList`, `_ChannelLogo`)
- Test: `mobile/test/presentation/channel_row_test.dart`

**Interfaces:**
- Produces: `ChannelRow({required Channel channel, required VoidCallback onTap})`.

- [ ] **Step 1: Escribir el test**

```dart
// mobile/test/presentation/channel_row_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/domain/models/channel.dart';
import 'package:iptv_ecosystem/presentation/widgets/channel_row.dart';

Channel _canal({String pais = 'MX', String cat = 'news', bool? vivo = true}) =>
    Channel(
      id: 'ch-1',
      name: 'Canal Uno (1080p)',
      logoUrl: '',
      categoryId: cat,
      languageCode: 'es',
      countryCode: pais,
      providerType: 'opensource',
      alive: vivo,
      latencyMs: 120,
    );

void main() {
  testWidgets('muestra nombre y línea de metadatos en mono', (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(body: ChannelRow(channel: _canal(), onTap: () {})),
    ));

    expect(find.text('Canal Uno (1080p)'), findsOneWidget);
    // Metadatos compactos: país · categoría · calidad extraída del nombre.
    expect(find.textContaining('MX'), findsOneWidget);
    expect(find.textContaining('news'), findsOneWidget);
  });

  testWidgets('omite los metadatos vacíos sin dejar separadores sueltos',
      (tester) async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: ChannelRow(channel: _canal(pais: '', cat: ''), onTap: () {}),
      ),
    ));

    expect(find.textContaining('·'), findsNothing);
  });

  testWidgets('el logo se decodifica al tamaño del hueco, no a resolución completa',
      (tester) async {
    final c = Channel(
      id: 'ch-1', name: 'X', logoUrl: 'http://x/logo.png', categoryId: '',
      languageCode: '', countryCode: '', providerType: 'opensource',
    );
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(body: ChannelRow(channel: c, onTap: () {})),
    ));

    final img = tester.widget<Image>(find.byType(Image));
    expect(img.width, isNotNull);
    // Sin cacheWidth, cada logo se decodifica completo para un hueco de 40px.
    expect((img.image as ResizeImage).width, isNotNull);
  });
}
```

- [ ] **Step 2: Correr y ver fallar** → FAIL.

- [ ] **Step 3: Implementar**

```dart
// mobile/lib/presentation/widgets/channel_row.dart
import 'package:flutter/material.dart';
import '../../domain/models/channel.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_motion.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import 'signal_bars.dart';

const _logoLado = 40.0;

/// Fila de canal. Sustituye al ListTile de Material: separación por hairline,
/// metadatos en mono y hover a carbón, como las celdas de la retícula del sitio.
class ChannelRow extends StatefulWidget {
  const ChannelRow({super.key, required this.channel, required this.onTap});

  final Channel channel;
  final VoidCallback onTap;

  @override
  State<ChannelRow> createState() => _ChannelRowState();
}

class _ChannelRowState extends State<ChannelRow> {
  bool _hover = false;

  /// Une solo las partes que existen, para no dejar separadores huérfanos
  /// cuando al canal le falta país o categoría.
  String get _meta => [
        widget.channel.countryCode,
        widget.channel.categoryId,
      ].where((p) => p.isNotEmpty).join(' · ');

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: AnimatedContainer(
          duration: KorvenMotion.fast,
          curve: KorvenMotion.easeOut,
          padding: const EdgeInsets.symmetric(
              horizontal: KorvenSpacing.s5, vertical: KorvenSpacing.s3),
          decoration: BoxDecoration(
            color: _hover ? KorvenColors.surfaceCard : Colors.transparent,
            border: const Border(
                bottom: BorderSide(color: KorvenColors.borderSubtle)),
          ),
          child: Row(
            children: [
              _Logo(url: widget.channel.logoUrl),
              const SizedBox(width: KorvenSpacing.s4),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      widget.channel.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: KorvenType.body.copyWith(fontSize: 15),
                    ),
                    if (_meta.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(_meta,
                          style: KorvenType.monoLabel
                              .copyWith(color: KorvenColors.textFaint)),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: KorvenSpacing.s4),
              SignalBars(
                  alive: widget.channel.alive,
                  latencyMs: widget.channel.latencyMs),
            ],
          ),
        ),
      ),
    );
  }
}

class _Logo extends StatelessWidget {
  const _Logo({required this.url});
  final String url;

  @override
  Widget build(BuildContext context) {
    final marco = BoxDecoration(
      color: KorvenColors.surfaceInset,
      border: Border.all(color: KorvenColors.borderSubtle),
      borderRadius: BorderRadius.circular(KorvenRadius.sm),
    );

    if (url.isEmpty) {
      return Container(
        width: _logoLado,
        height: _logoLado,
        decoration: marco,
        child: const Icon(Icons.tv, size: 18, color: KorvenColors.textFaint),
      );
    }

    return Container(
      width: _logoLado,
      height: _logoLado,
      decoration: marco,
      clipBehavior: Clip.antiAlias,
      child: Image.network(
        url,
        fit: BoxFit.contain,
        // Sin cacheWidth cada logo se decodifica a resolución completa para un
        // hueco de 40px, y con 12k canales eso es memoria tirada. 2× por
        // densidad de pantalla.
        cacheWidth: (_logoLado * 2).round(),
        cacheHeight: (_logoLado * 2).round(),
        errorBuilder: (_, __, ___) =>
            const Icon(Icons.tv, size: 18, color: KorvenColors.textFaint),
      ),
    );
  }
}
```

Ajustar la firma de `SignalBars` a la que ya tiene el widget existente
(`mobile/lib/presentation/widgets/signal_bars.dart`, 55 líneas) en vez de
asumirla.

- [ ] **Step 4: Correr, ver pasar, y sustituir en `_ChannelList`**

Run: `flutter test && flutter analyze`.

- [ ] **Step 5: Commit**

```bash
git commit -am "mobile: fila de canal con metadatos mono y logos redimensionados"
```

---

### Task 11: Estados vacío, cargando y error

**Files:**
- Create: `mobile/lib/presentation/widgets/state_views.dart`
- Modify: `mobile/lib/presentation/screens/home_screen.dart` (borrar `_EmptyState`)
- Test: `mobile/test/presentation/state_views_test.dart`

**Interfaces:**
- Produces: `KorvenStateView({required String eyebrow, required String message, Widget? action, bool showEmblem})`.

**Contexto:** el estado vacío actual siempre dice que el gateway está sincronizando, incluso cuando el problema es que no hay resultados con esos filtros o que no hay red. Son tres situaciones distintas y merecen tres mensajes.

- [ ] **Step 1: Escribir el test**

```dart
// mobile/test/presentation/state_views_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/presentation/widgets/korven_emblem.dart';
import 'package:iptv_ecosystem/presentation/widgets/state_views.dart';

void main() {
  testWidgets('muestra eyebrow, mensaje y emblema', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: KorvenStateView(
          eyebrow: '// sin resultados',
          message: 'Ningún canal casa con estos filtros.',
        ),
      ),
    ));

    expect(find.text('// sin resultados'), findsOneWidget);
    expect(find.text('Ningún canal casa con estos filtros.'), findsOneWidget);
    expect(find.byType(KorvenEmblem), findsOneWidget);
  });

  testWidgets('la acción es opcional', (tester) async {
    var pulsado = false;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: KorvenStateView(
          eyebrow: '// error',
          message: 'No se pudo contactar con el gateway.',
          action: ElevatedButton(
            onPressed: () => pulsado = true,
            child: const Text('Reintentar'),
          ),
        ),
      ),
    ));

    await tester.tap(find.text('Reintentar'));
    expect(pulsado, isTrue);
  });
}
```

- [ ] **Step 2: Correr y ver fallar** → FAIL.

- [ ] **Step 3: Implementar y cablear los tres casos**

```dart
// mobile/lib/presentation/widgets/state_views.dart
import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';
import 'korven_emblem.dart';

/// Vista de estado para vacío, error y carga. Los tres comparten tratamiento
/// —emblema, eyebrow de consola y mensaje— pero NO comparten texto: distinguir
/// "sin resultados con estos filtros" de "el gateway no responde" es la mitad
/// del valor.
class KorvenStateView extends StatelessWidget {
  const KorvenStateView({
    super.key,
    required this.eyebrow,
    required this.message,
    this.action,
    this.showEmblem = true,
  });

  final String eyebrow;
  final String message;
  final Widget? action;
  final bool showEmblem;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(KorvenSpacing.s6),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (showEmblem) ...[
              Opacity(opacity: 0.5, child: const KorvenEmblem(size: 72)),
              const SizedBox(height: KorvenSpacing.s5),
            ],
            Text(eyebrow,
                style: KorvenType.monoLabel
                    .copyWith(color: KorvenColors.textFaint)),
            const SizedBox(height: KorvenSpacing.s3),
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Text(message,
                  textAlign: TextAlign.center, style: KorvenType.body),
            ),
            if (action != null) ...[
              const SizedBox(height: KorvenSpacing.s5),
              action!,
            ],
          ],
        ),
      ),
    );
  }
}
```

En `home_screen.dart`, distinguir:
- lista vacía **con** filtros activos → `// sin resultados` / "Ningún canal casa con estos filtros." + botón `limpiar filtros`.
- lista vacía **sin** filtros → `// catálogo vacío` / "El gateway todavía no ha sincronizado el catálogo." + `Recargar`.
- `AsyncError` → `// error` / `ApiError.desde(e).mensaje` + `Reintentar`.

- [ ] **Step 4: Correr y ver pasar**, y añadir un test de widget en `home_screen_test.dart` que compruebe que con filtros activos y cero resultados sale el mensaje de filtros y no el de sincronización.

- [ ] **Step 5: Commit**

```bash
git commit -am "mobile: estados de vacío, carga y error diferenciados"
```

---

# FASE 5 — Reproductor

### Task 12: Chrome del reproductor y línea de terminal

**Files:**
- Create: `mobile/lib/presentation/widgets/console_line.dart`
- Modify: `mobile/lib/presentation/screens/player_screen.dart`
- Test: `mobile/test/presentation/console_line_test.dart`

**Interfaces:**
- Produces: `ConsoleLine({required String text, bool blinking})` — línea mono sobre `codeBg` con cursor ámbar parpadeante.

**Contexto:** el overlay de carga muestra hoy un spinner con "Cargando X… / Timeout en 15s". La línea de terminal es on-brand y además mejor UX: nombra lo que está pasando.

- [ ] **Step 1: Escribir el test**

```dart
// mobile/test/presentation/console_line_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:iptv_ecosystem/presentation/widgets/console_line.dart';

void main() {
  testWidgets('muestra el comando y un cursor', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: ConsoleLine(text: r'$ korven tune --channel "BBC One"'),
      ),
    ));

    expect(find.text(r'$ korven tune --channel "BBC One"'), findsOneWidget);
    expect(find.byKey(const Key('console-cursor')), findsOneWidget);

    // El cursor parpadea: dejarlo asentar no debe colgar el test.
    await tester.pump(const Duration(seconds: 2));
  });
}
```

- [ ] **Step 2: Correr y ver fallar** → FAIL.

- [ ] **Step 3: Implementar `ConsoleLine`**

```dart
// mobile/lib/presentation/widgets/console_line.dart
import 'dart:async';
import 'package:flutter/material.dart';
import '../../theme/korven_colors.dart';
import '../../theme/korven_spacing.dart';
import '../../theme/korven_typography.dart';

/// Línea de terminal de la marca, con el cursor ámbar parpadeante. Sustituye al
/// spinner en el reproductor: además de estar en la voz de Korven, nombra lo que
/// está pasando en vez de girar.
class ConsoleLine extends StatefulWidget {
  const ConsoleLine({super.key, required this.text, this.blinking = true});

  final String text;
  final bool blinking;

  @override
  State<ConsoleLine> createState() => _ConsoleLineState();
}

class _ConsoleLineState extends State<ConsoleLine> {
  Timer? _timer;
  bool _visible = true;

  @override
  void initState() {
    super.initState();
    if (widget.blinking) {
      // steps(1): conmuta, no interpola — un cursor no se desvanece.
      _timer = Timer.periodic(const Duration(milliseconds: 550), (_) {
        if (mounted) setState(() => _visible = !_visible);
      });
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
          horizontal: KorvenSpacing.s4, vertical: KorvenSpacing.s3),
      decoration: BoxDecoration(
        color: KorvenColors.codeBg,
        border: Border.all(color: KorvenColors.borderDefault),
        borderRadius: BorderRadius.circular(KorvenRadius.md),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Flexible(
            child: Text(
              widget.text,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: KorvenType.mono.copyWith(color: KorvenColors.textBody),
            ),
          ),
          const SizedBox(width: 6),
          Opacity(
            key: const Key('console-cursor'),
            opacity: _visible ? 1 : 0,
            child: Container(width: 8, height: 16, color: KorvenColors.accent),
          ),
        ],
      ),
    );
  }
}
```

- [ ] **Step 4: Cablear en `player_screen.dart`**

Sustituir el `Column` del overlay de carga (spinner + textos) por el emblema pequeño, `ConsoleLine` con `$ korven tune --channel "${widget.channelName}"`, y el fondo a `surfaceBase`. El estado de error pasa a `KorvenStateView` con el emblema, el mensaje de `ApiError` y el botón ámbar de reintentar.

**No tocar la lógica de `_loadAndPlay`**: el watchdog, el token de generación y el guard se quedan exactamente como están. Esta tarea es solo presentación.

- [ ] **Step 5: Correr todo**

Run: `flutter test && flutter analyze` → verde. Los 7 tests de `playback_guard_test.dart` deben seguir pasando sin tocarlos: si alguno se rompe, es que se tocó lógica que no tocaba.

- [ ] **Step 6: Verificar a mano**

Arrancar gateway y app, abrir un canal y comprobar que sale la línea de terminal; luego matar el gateway y comprobar el estado de error.

- [ ] **Step 7: Commit**

```bash
git commit -am "mobile: reproductor con la línea de terminal de la marca"
```

---

# FASE 6 — Renombrado a Korven Open TV

*Cinco capas, de menor a mayor riesgo. La carpeta va la última por sus efectos fuera del repositorio.*

### Task 13: Capa visible

**Files:** `mobile/lib/main.dart`, `mobile/macos/Runner/Configs/AppInfo.xcconfig`, `mobile/pubspec.yaml` (`description`), `README.md`, `PROMPT_MAESTRO.md`

- [ ] **Step 1: Cambiar los textos visibles**

- `main.dart`: `title: 'IPTV Ecosystem'` → `'Korven Open TV'`.
- `AppInfo.xcconfig`: `PRODUCT_NAME = Korven Open TV`, y
  `PRODUCT_COPYRIGHT = Copyright © 2026 Korven. Todos los derechos reservados.`
- `pubspec.yaml`: `description: "Korven Open TV — televisión abierta, del núcleo a la obra."`
- `README.md` y `PROMPT_MAESTRO.md`: encabezados y menciones del nombre.

- [ ] **Step 2: Verificar**

Run: `flutter analyze && flutter test` → verde. Arrancar la app y comprobar el título de la ventana.

- [ ] **Step 3: Commit**

```bash
git commit -am "chore: renombrar a Korven Open TV en la capa visible"
```

---

### Task 14: Paquete Dart

**Files:** `mobile/pubspec.yaml`, los 9 archivos con `package:iptv_ecosystem/`

- [ ] **Step 1: Renombrar**

```bash
cd /Users/usuario/Dev/ip-tv/mobile
sed -i '' 's/^name: iptv_ecosystem/name: korven_open_tv/' pubspec.yaml
grep -rl "package:iptv_ecosystem/" lib test | xargs sed -i '' 's|package:iptv_ecosystem/|package:korven_open_tv/|g'
flutter pub get
```

- [ ] **Step 2: Verificar**

```bash
grep -rn "iptv_ecosystem" lib test pubspec.yaml || echo "sin referencias al nombre viejo"
flutter analyze && flutter test
```

- [ ] **Step 3: Commit**

```bash
git commit -am "chore: paquete Dart korven_open_tv"
```

---

### Task 15: Módulo Go

**Files:** `gateway/go.mod` y los 22 archivos `.go`

- [ ] **Step 1: Renombrar**

```bash
cd /Users/usuario/Dev/ip-tv/gateway
sed -i '' 's|^module github.com/tu-org/iptv-ecosystem/gateway|module github.com/gdberysan/open-tv/gateway|' go.mod
grep -rl "tu-org/iptv-ecosystem" --include="*.go" . | xargs sed -i '' 's|github.com/tu-org/iptv-ecosystem/gateway|github.com/gdberysan/open-tv/gateway|g'
gofmt -w .
```

- [ ] **Step 2: Verificar**

```bash
grep -rn "tu-org/iptv-ecosystem" . --include="*.go" --include="go.mod" || echo "sin referencias al módulo viejo"
go build ./... && go vet ./... && gofmt -l . && go test -race -count=1 ./...
```

- [ ] **Step 3: Commit**

```bash
git commit -am "chore: módulo Go github.com/gdberysan/open-tv/gateway"
```

---

### Task 16: Bundle de macOS

**Files:** `mobile/macos/Runner/Configs/AppInfo.xcconfig`, `mobile/macos/Runner.xcodeproj/project.pbxproj`

**⚠️ Cambiar el bundle identifier resetea `SharedPreferences`**: se pierde el toggle "mostrar offline". Es una preferencia trivial, recuperable de un clic, y se acepta a cambio de una identidad coherente.

- [ ] **Step 1: Cambiar el identificador**

```bash
cd /Users/usuario/Dev/ip-tv/mobile/macos
sed -i '' 's/com\.example\.iptvEcosystem/dev.korven.opentv/g' Runner/Configs/AppInfo.xcconfig Runner.xcodeproj/project.pbxproj
grep -rn "com.example" Runner/Configs Runner.xcodeproj/project.pbxproj || echo "sin com.example"
```

- [ ] **Step 2: Reconstruir limpio y verificar**

```bash
cd /Users/usuario/Dev/ip-tv/mobile && flutter clean && flutter pub get && flutter build macos --debug 2>&1 | tail -5
```

Arrancar la app: debe abrir con el nombre nuevo. El toggle de offline arranca apagado — es el reseteo esperado.

- [ ] **Step 3: Commit**

```bash
git commit -am "chore: bundle id dev.korven.opentv"
```

---

### Task 17: Repositorio y carpeta

**⚠️ La tarea con efectos fuera del repositorio.** Ejecutar entera y seguida.

- [ ] **Step 1: Renombrar el repositorio en GitHub**

```bash
cd /Users/usuario/Dev/ip-tv
gh repo rename open-tv --yes
git remote -v
```

GitHub mantiene redirecciones desde la URL antigua y `gh` actualiza el remoto local.

- [ ] **Step 2: Cerrar todo lo que tenga la carpeta abierta**

```bash
pkill -f "cmd/server"; lsof -ti:8080 | xargs -r kill -9
pkill -f "iptv_ecosystem.app"; pkill -f "Korven Open TV"
```

- [ ] **Step 3: Renombrar la carpeta**

```bash
cd /Users/usuario/Dev && mv ip-tv open-tv && cd open-tv && pwd && git status --short
```

- [ ] **Step 4: Arreglar las rutas absolutas de `.claude/settings.json`**

Sin esto vuelven los diálogos de permisos que costó trabajo eliminar: el fichero tiene 3 rutas absolutas a la carpeta vieja.

```bash
cd /Users/usuario/Dev/open-tv
sed -i '' 's|/Users/usuario/Dev/ip-tv|/Users/usuario/Dev/open-tv|g' .claude/settings.json
grep -c "Dev/open-tv" .claude/settings.json   # debe dar 3
python3 -c "import json;json.load(open('.claude/settings.json'));print('JSON válido')"
```

- [ ] **Step 5: Mover el directorio de memoria de Claude Code**

Está indexado por ruta; si no se mueve, las memorias del proyecto quedan huérfanas.

```bash
cd ~/.claude/projects
[ -d "-Users-usuario-Dev-open-tv" ] && echo "ya existe, fusionar a mano" || mv "-Users-usuario-Dev-ip-tv" "-Users-usuario-Dev-open-tv"
ls -d "-Users-usuario-Dev-open-tv" && ls "-Users-usuario-Dev-open-tv/memory/"
```

- [ ] **Step 6: Verificar el stack completo desde la ruta nueva**

```bash
cd /Users/usuario/Dev/open-tv/gateway && go build ./... && go test -count=1 ./... 2>&1 | grep -c "^ok"
cd ../mobile && flutter test 2>&1 | tail -1 && flutter analyze 2>&1 | tail -1
cd .. && grep -rniE "iptv.ecosystem|ip-tv|tu-org" --include="*.go" --include="*.dart" --include="*.yaml" --include="*.json" gateway mobile .claude || echo "sin referencias al nombre viejo"
```

- [ ] **Step 7: Commit y push**

```bash
git add -A
git commit -m "chore: renombrar el proyecto a open-tv

Repositorio, carpeta local y rutas de configuración. Incluye el arreglo de
las 3 rutas absolutas de .claude/settings.json y el movimiento del directorio
de memoria de Claude Code, ambos indexados por la ruta antigua."
git push origin main
```

---

## Verificación final

Con todo mergeado, arrancar el stack desde la ruta nueva y comprobar de punta a punta:

1. **Marca:** la ventana se llama *Korven Open TV* y la barra muestra el lockup con la O hexagonal y el nodo ámbar.
2. **Filtros combinados:** aplicar país + categoría + calidad + búsqueda a la vez; las cuatro facetas visibles, cada una removible, y el contador cuadrando con
   `curl -s -D- -o /dev/null "http://127.0.0.1:8080/channels?country=ES&category=news&quality=fhd&q=tv" | grep -i x-total-count`.
3. **Limpiar** devuelve el catálogo completo y la calidad al default.
4. **Reproducción:** abrir un canal y ver la línea de terminal; el vídeo arranca.
5. **Error del reproductor:** matar el gateway y comprobar el estado con emblema y mensaje legible.
6. **Regresión de la Fase 1-5:** el filtro de países sigue devolviendo ~10 835 canales con país y `/epg/now` sigue dando 404.
7. **Permisos:** confirmar que los comandos habituales no vuelven a pedir confirmación tras el renombrado de carpeta.

La verificación visual final es manual: la terminal no tiene permiso de Grabación de Pantalla, así que no hay captura automatizada.

## Fuera de alcance

Pantallas nuevas (ajustes con URL del gateway en runtime, acerca de), tema claro, atajos de teclado, noción de conectividad, y la Fase 8 del roadmap (favoritos, historial, retomar reproducción, PiP). Todo ello sigue identificado en la auditoría y puede ser el siguiente plan.
