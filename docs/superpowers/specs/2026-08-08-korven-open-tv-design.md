# Korven Open TV — rebrand and UX revamp

**Estado:** aprobado 2026-08-08 · **Alcance:** app Flutter (macOS) + renombrado del proyecto

## Contexto

La app funciona bien y no lo parece. Tras cinco fases de fiabilidad el backend es
sólido —catálogo correcto, salud de streams con histéresis, observabilidad real—
pero la interfaz sigue siendo la plantilla por defecto de Flutter: `ThemeData.dark()`
sembrado con `deepPurple`, `ListTile` sin tratar, y filtros que no dicen qué está
activo. La calidad visual no corresponde a la funcional.

Aparte, el proyecto pasa a ser una obra de **Korven** (korven.dev, estudio de
Gerard Bernal) y se renombra a **Korven Open TV**. Existe un design
system completo y de alta fidelidad en `~/Dev/korven/design/`, con
tokens CSS reutilizables tal cual, assets de marca oficiales y capturas de
referencia. No hay que inventar nada: hay que portarlo.

**Resultado buscado:** que la app se vea como lo que es —una herramienta seria,
construida— y que los filtros dejen de ser el punto débil de la experiencia.

## Principio rector

El brand board lo define así: *"un canvas de grafito puntuado por un solo punto de
luz ámbar — el ojo del cuervo, el foco."*

Para una app de televisión ese mapeo es casi literal: **el ámbar es la señal viva.**

> Ámbar significa: este canal está vivo, este filtro está activo, aquí estás
> mirando. Todo lo demás es grafito y acero.

Esa única regla gobierna el rediseño entero y es exactamente lo que le falta hoy:
el morado por defecto no significa nada, así que se reparte sin criterio.

El segundo recurso de marca es la **voz de terminal** (`//` en los eyebrows,
etiquetas mono, `$ korven build --from core`). Encaja sin forzar en una app cuyo
tema son streams, señal y salud.

## Sistema de tokens

Puerto directo de `design_handoff_korven_sitio/tokens/*.css` a Dart, en
`mobile/lib/theme/`. Los valores son finales; no se reinterpretan.

| Rol | Valor |
|---|---|
| Canvas | `#0E131B` grafito · hundido `#0A0E15` · carbón `#171E29` · inset `#121826` |
| Texto | fuerte `#EFF3F8` · cuerpo `#C3CCD7` · mudo `#97A3B2` · tenue `#6B788C` |
| Acento único | `#FF8A2B` (hover `#FFA255`, press `#E06E1C`), glow `rgba(255,138,43,.16)` |
| Semánticos | ok `#4FB286` · error `#E5604D` · info `#5E9BD6` |
| Hairlines | `rgba(151,163,178,.12)` subtle · `.20` default · `.36` strong |
| Radios | 3 / 5 / 8 / 12 / 18 |
| Espaciado | escala de 8px (4, 8, 12, 16, 24, 32, 48, 64, 96) |
| Motion | `Cubic(.16,1,.3,1)` · 120 / 200 / 360 ms |

**Tipografía:** Space Grotesk (display, 500–700), Inter (texto, 400–600),
JetBrains Mono (etiquetas y metadatos, 400–700). Escala de tercera mayor 1.250
con base 16.

**Decisión — fuentes empaquetadas, no `google_fonts`.** El paquete `google_fonts`
descarga en tiempo de ejecución en el primer arranque; para una app de escritorio
eso es una dependencia de red innecesaria y un parpadeo tipográfico. Se descargan
los `.ttf` una vez a `mobile/assets/fonts/` y se declaran en `pubspec.yaml`. Sin
dependencia nueva y funciona sin red. El propio handoff contempla el self-hosting.

**Tema:** solo oscuro. Korven es un canvas oscuro por definición y un reproductor
de vídeo se mira a oscuras. El hallazgo de la auditoría sobre ignorar la apariencia
del sistema se cierra como *no aplica*, no como pendiente.

## Componentes

### Barra de consola (AppBar)

Sustituye a `AppBar(title: Text('IPTV'))`. Izquierda: el lockup de marca —
**KORVEN** en Space Grotesk 700 con la "O" hexagonal característica rellena de
acero y el nodo ámbar de 6px al centro, seguido de `open tv` en mono 12px mudo.
Es el mismo patrón que `KORVEN.dev` del sitio.

La "O" hexagonal se dibuja con `ClipPath` y un `CustomClipper<Path>` replicando el
`clip-path: polygon(50% 0, 100% 27%, 100% 73%, 50% 100%, 0 73%, 0 27%)` del CSS.
Sin assets ni dependencias, y conserva la webfont.

Acciones (buscar, offline, recargar): mudo → hueso al hover, **ámbar solo cuando
están activas** (p. ej. el toggle de offline encendido).

### Superficie de filtros — la reestructuración

**Hoy:** cuatro chips de calidad siempre visibles, país y categoría escondidos tras
diálogos, ningún indicador de qué está activo y ninguna forma de limpiar.

**Nuevo**, tomado de la consola de casos del sitio:

```
// filtros                                         1 284 canales

[ país: MX × ]  [ categoría: noticias × ]                 limpiar

[ 4K ]  [ 1080p+ ]  [ HD 720p+ ]  [ todos ]
```

- Eyebrow mono `// filtros` con el `//` en tenue.
- **Facetas activas como chips ámbar removibles.** País y categoría siguen
  eligiéndose en un selector, pero al aplicarse se materializan aquí con su `×`.
  Es lo que hoy no existe: estado visible y reversible de un toque.
- **Calidad como grupo segmentado**, uno activo a la vez. Ámbar cuando no es el
  default (`1080p+`), acero cuando lo es — el acento solo aparece si hay decisión
  del usuario.
- **`limpiar`** solo cuando hay algo que limpiar.
- **Contador de resultados** en mono tenue. Con 12 000 canales es información
  genuina, y es la voz de terminal ganándose el sitio en vez de decorar.

Chips según el spec de marca: mono 13px, padding 9×16, radio 5, borde default,
texto mudo; hover borde ámbar; activo borde y texto ámbar sobre
`rgba(255,138,43,.10)`.

### Filas de canal

Fuera el `ListTile` de Material. Filas separadas por hairline sobre grafito:

- Logo en cuadrado con borde hairline y radio 5.
- Nombre en Inter 15px, cuerpo.
- Línea de metadatos en **mono 12px tenue**: `MX · noticias · 1080p`.
- Barras de señal (ya existen) con la rampa ámbar.
- Hover: fondo carbón, igual que las celdas de la retícula de "Sistemas" en el sitio.

Aquí aterriza además el hallazgo de la auditoría sobre imágenes: los logos se
decodifican hoy a resolución completa para un hueco de 40×40. Se añade
`cacheWidth`/`cacheHeight` al reescribir la fila, porque es el momento natural.

### Reproductor

El overlay de carga cambia el spinner por la línea de terminal de la marca:

```
$ korven tune --channel "BBC One (1080p)"█
```

Mono, sobre `--code-bg`, con el cursor ámbar parpadeando en `steps(1)` a 1.1s. Es
on-brand y además mejor UX que un spinner: nombra lo que está pasando. Se conserva
la promesa de timeout, ya correcta tras la Fase 2.

Estado de error: emblema del cuervo, el mensaje legible que ya produce `ApiError`,
y botón ámbar de reintentar.

### Estados vacío / cargando / error

Los tres comparten tratamiento: emblema o marca de agua, eyebrow mono con `//`, y
mensaje en cuerpo. Se elimina el texto actual que siempre asume que el gateway está
sincronizando, y se distingue "sin resultados con estos filtros" de "no se pudo
contactar con el gateway".

### Emblema

`korven-emblem.svg` son 949 bytes de polígonos, polilíneas y círculos — se replica
con un `CustomPainter` de unas 40 líneas. **Sin `flutter_svg`**: en una app de
escritorio cada dependencia es pasivo, y el painter da control total.

El ojo se queda **estático**. El seguimiento del cursor es encantador en una
one-page y molesto en una herramienta.

## Lo que deliberadamente no se hace

- **Sin sistema de scroll-reveal.** Es un recurso de landing y pelea con una lista
  que se recorre rápido.
- **Sin typewriter fuera del reproductor.** Solo donde la espera es real.
- **Sin ojo que sigue el cursor** en la UI principal.
- **Sin tema claro.** Ver arriba.
- **Sin pantallas nuevas** (ajustes, acerca de). Quedan fuera de este alcance.

## Renombrado a Korven Open TV

Aprobado el alcance completo, incluido el repositorio. Cinco capas, de menor a
mayor riesgo:

1. **Visible** — `MaterialApp.title`, `PRODUCT_NAME` y `PRODUCT_COPYRIGHT` en
   `AppInfo.xcconfig`, `pubspec.yaml` `description`, README y `PROMPT_MAESTRO.md`.
2. **Paquete Dart** — `pubspec.yaml` `name: iptv_ecosystem` → `korven_open_tv`, y
   los imports `package:iptv_ecosystem/` en **9 archivos**.
3. **Módulo Go** — `github.com/tu-org/iptv-ecosystem/gateway` →
   `github.com/gdberysan/open-tv/gateway`, en **22 archivos** (9 rutas distintas).
   Se usa el owner real del repositorio, no un `korven-dev` inventado: la ruta de
   módulo debe seguir siendo resoluble si algún día se hace `go get`.
4. **Bundle macOS** — `com.example.iptvEcosystem` → `dev.korven.opentv`.
   ⚠️ **Resetea `SharedPreferences`**: se pierde el toggle "mostrar offline". Es
   una preferencia trivial y recuperable de un clic; se acepta y se documenta.
5. **Repositorio y carpeta** — nombres concretos, decididos aquí para que no haya
   ambigüedad al ejecutar: repositorio `gdberysan/open-tv` (vía `gh repo rename
   open-tv`) y carpeta `~/Dev/open-tv`.

### ⚠️ El renombrado de carpeta tiene efectos fuera del repositorio

Descubierto al medir, no es obvio:

- **`.claude/settings.json` contiene 3 rutas absolutas** a `~/Dev/ip-tv`.
  Al renombrar la carpeta dejan de casar y **vuelven los diálogos de permisos** que
  costó trabajo eliminar.
- **El directorio de memoria de Claude Code está indexado por ruta**
  (`~/.claude/projects/-Users-usuario-Dev-ip-tv`). Con la carpeta nueva se crea
  otro directorio y las memorias del proyecto quedan huérfanas.

Por eso el renombrado de carpeta va **el último**, en su propio paso, con los dos
arreglos inmediatamente después: reescribir las rutas de `settings.json` y mover el
directorio de memoria.

`gh repo rename` mantiene redirecciones desde la URL antigua y actualiza el remoto
local, así que esa parte es segura.

## Testing

- **Tokens y tema:** un test que verifica que los valores del `ThemeData` salen de
  los tokens y no de literales sueltos, para que nadie vuelva a colar un
  `Colors.grey`.
- **Filtros:** tests de widget sobre la conducta nueva — aplicar una faceta la
  muestra como chip, la `×` la quita, `limpiar` las quita todas, y el contador
  refleja los resultados. Es la parte con lógica real.
- **Filas y estados:** tests de render de los tres estados (vacío por filtros,
  vacío por gateway, error) verificando que muestran mensajes distintos.
- **Renombrado:** la garantía es mecánica — `go build`, `go vet`, `flutter analyze`
  y las dos suites completas tras cada capa. Además un `grep` de verificación de
  que no queda ninguna referencia al nombre viejo.
- El suite existente (58 tests) debe seguir en verde en todo momento; los tests que
  referencian textos de UI que cambian se actualizan en el mismo commit.

## Verificación end-to-end

Más allá de los tests: arrancar gateway y app, y comprobar a mano que los filtros
combinan bien (país + categoría + calidad + búsqueda a la vez), que el contador
cuadra con lo que devuelve `/channels`, que un canal reproduce, y que el estado de
error del reproductor se ve correcto con el gateway caído.

Nota conocida: no hay captura de pantalla automatizada disponible —la terminal no
tiene permiso de Grabación de Pantalla—, así que la verificación visual final es
manual.
