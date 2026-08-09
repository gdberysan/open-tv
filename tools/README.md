# tools

## `render_icono.swift`

Genera el icono de macOS desde el emblema hexagonal de Korven, en los siete
tamaños que pide `AppIcon.appiconset`. Se dibuja por código y no se guarda un
maestro en Photoshop: la geometría es la misma que `_EmblemPainter`
(`lib/presentation/widgets/korven_emblem.dart`), así que emblema e icono no
pueden divergir sin que alguien lo haga a propósito.

```bash
xcrun swiftc -O -o /tmp/rendericono tools/render_icono.swift -framework AppKit
/tmp/rendericono mobile/macos/Runner/Assets.xcassets/AppIcon.appiconset
cd mobile && flutter build macos --debug
```

Los trazos finos tienen suelo de anchura: sin él, a 16 y 32 px el emblema se
convertía en una mancha.
