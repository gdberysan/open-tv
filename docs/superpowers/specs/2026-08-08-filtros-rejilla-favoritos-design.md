# Filtros por modal, rejilla, favoritos y canal aleatorio

**Estado:** aprobado 2026-08-08 · **Alcance:** app Flutter + endpoints del gateway

## Contexto

Tras el rebranding a Korven Open TV la app ya se ve como lo que es, pero navegar
9 100 canales sigue siendo torpe. Cuatro cambios, más un bug real que salió al
mirar los datos.

## El bug: las categorías son compuestas y el filtro no lo sabe

Hay 181 valores distintos de `category_id`, pero la mayoría son compuestos
separados por `;`: `Animation;Kids`, `Movies;Series`, incluso
`Culture;Documentary;Entertainment;General;Movies;Music`. La taxonomía real son
**30 categorías atómicas**.

El filtro compara con `=`, así que descarta en silencio todo canal con más de una
etiqueta:

| Filtrando por | Devuelve hoy | Debería |
|---|---|---|
| Movies | 573 | **655** |
| Kids | 253 | **337** |

Una cuarta parte de los canales infantiles es invisible al filtrar por Kids.

**Arreglo:** comparar por pertenencia, no por igualdad —
`';'||category_id||';' LIKE '%;Kids;%'`. Medido: 7 ms de escaneo completo sobre
12 k filas, así que no compensa índice. `LIKE` en SQLite ya es insensible a
mayúsculas para ASCII, lo que además resuelve el caso de `news` vs `News`.

El selector debe ofrecer las 30 atómicas, no los 181 strings compuestos.

## Filtros: tres botones, tres modales

Sustituyen a la fila de chips de calidad y a los chips `+ país` / `+ categoría`.
Tres botones del mismo ancho, alineados en una sola línea:

```
// filtros                                         1 284 canales

[ 🇲🇽  México          × ]  [ 1080p+ ]  [ 🎬 Movies × ]      limpiar
```

Cada botón muestra su valor activo o su etiqueta neutra (`país`, `resolución`,
`categoría`), y en ámbar solo cuando hay selección — la regla del sistema.

- **País.** Los 178 países con canales, cada uno con bandera, **nombre completo**
  y su número de canales, ordenados por volumen. Con barra de búsqueda, que con
  178 opciones deja de ser opcional. La bandera se deriva del código ISO con los
  indicadores regionales; como los lectores de pantalla los leen letra a letra,
  cada fila lleva `semanticLabel` con el nombre real.
- **Resolución.** 4K, 1080p+, HD 720p+, todos.
- **Categoría.** Las 30 atómicas con su recuento real y un icono cada una.
  Se usan iconos vectoriales de Material (`movie`, `sports_soccer`, `newspaper`,
  `child_care`…) en vez de encargar 30 SVG: ya son vectores, ya están en el
  binario y se tiñen con la regla del ámbar sin trabajo extra.

Los nombres de país viven en un mapa `const` en Dart. El gateway no los conoce y
añadir una dependencia de i18n para 250 líneas de datos no se justifica.

## Rejilla de canales

Pasa a ser la vista por defecto, con un botón en la barra para volver a lista.
La preferencia se recuerda en `SharedPreferences`.

**El 15,6 % de los canales no tiene logo** (1 419 de los 9 100 visibles), así que
el marcador de posición no es un detalle: es 1 de cada 6 celdas. Se dibuja con la
inicial del canal sobre el hexágono de la marca, no con un icono genérico.

Tarjeta: logo encuadrado arriba, nombre debajo, y una línea mono con país,
categoría y resolución. Estrella de favorito en la esquina. `SliverGrid` con
`maxCrossAxisExtent` para que el número de columnas se adapte al ancho.

La lista se conserva porque sigue siendo mejor para escanear por nombre y para
los canales sin logo.

## Favoritos

Un `Set<String>` de IDs de canal en `SharedPreferences`. No hace falta `sqflite`
para un conjunto de identificadores.

Se marcan con la estrella desde la tarjeta o la fila, y se filtran con un cuarto
chip `★ favoritos` junto a los tres botones, reusando la maquinaria de filtros
que ya existe. Sin pantalla nueva.

**Detalle no obvio:** los favoritos son un concepto del cliente y el gateway no
los conoce, así que filtrarlos no puede resolverse paginando el catálogo — un
favorito puede estar en la página 20. Cuando el filtro está activo, la app pide
exactamente esos IDs mediante un parámetro `ids` nuevo en `/channels`. Son
decenas de elementos, no miles.

## Canal aleatorio

Botón en la barra superior. Sortea **respetando los filtros activos**: con México
y Deportes puestos, dentro de esa intersección; sin filtros, entre todos los
vivos. Solo canales vivos — un aleatorio muerto arruina la función.

Se resuelve en el gateway (`GET /channels/random`) y no en el cliente: sortear
entre las páginas cargadas sesgaría el resultado hacia el principio del catálogo.

## Endpoints nuevos del gateway

| Endpoint | Para qué |
|---|---|
| `GET /channels/countries` | `[{code, count}]` de los países con canales, para el selector |
| `GET /channels/categories` | `[{name, count}]` de las 30 atómicas, para el selector |
| `GET /channels/random?…` | Un canal vivo al azar respetando los filtros |
| `GET /channels?ids=a,b,c` | Resolver los favoritos sin paginar el catálogo |

Los dos primeros son literalmente los *quick wins* pendientes en
`PROMPT_MAESTRO.md` §9.

Todos reutilizan `buildChannelWhere`, el helper compartido por `FindFiltered` y
`CountFiltered`. Es el mismo principio que hizo honesto al contador: si cada
consulta armara su propio filtro, acabarían discrepando.

## Testing

- **La corrección de categorías es lo primero que hay que probar**, con el caso
  real: un canal `Animation;Kids` tiene que salir al filtrar por `Kids`, y los
  totales de `FindFiltered` y `CountFiltered` deben seguir coincidiendo.
- **Aleatorio:** que nunca devuelva un canal muerto, que respete los filtros, y
  que con muchas llamadas no devuelva siempre el mismo — sortear mal es fácil y
  silencioso.
- **Favoritos:** persisten entre reinicios; el filtro devuelve exactamente los
  marcados aunque estén más allá de la primera página.
- **Selector de país:** la búsqueda filtra por nombre completo y por código, y
  cada fila expone su `semanticLabel`.
- **Rejilla:** el marcador de los canales sin logo se dibuja; el toggle persiste.
- Los 90 tests actuales siguen en verde.

## Verificación end-to-end

Contar con `curl` que `category=Kids` devuelva 337 y no 253; que
`/channels/random?country=MX` dé canales distintos en llamadas sucesivas y todos
vivos; y a mano en la app que los tres modales abran, filtren y se limpien, que
la estrella persista tras reiniciar, y que la rejilla se reflowee al cambiar el
ancho de ventana.

Sigue sin haber captura de pantalla automatizada: la verificación visual es
manual.

## Fuera de alcance

Atajos de teclado, manejo global de errores, accesibilidad más allá de los
`semanticLabel` de este alcance, `autoDispose` de la lista acumulada, pantalla de
ajustes, y el resto de la Fase 8 (historial, retomar reproducción, PiP).
