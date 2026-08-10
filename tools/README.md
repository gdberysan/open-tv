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

## `dev.korven.opentv.gateway.plist`

LaunchAgent para no tener que arrancar el gateway a mano. Es **plantilla**, no
un fichero instalable: launchd no expande `~` ni variables de entorno en las
rutas, así que hay que sustituirlas antes de copiarlo.

```bash
sed -e "s|__RUTA_AL_REPO__|$PWD|g" -e "s|__RUTA_HOME__|$HOME|g" \
  tools/dev.korven.opentv.gateway.plist \
  > ~/Library/LaunchAgents/dev.korven.opentv.gateway.plist   # desde la raíz del repo

cd gateway && go build -o server ./cmd/server && cd ..
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/dev.korven.opentv.gateway.plist
```

`RunAtLoad` lo levanta al iniciar sesión y `KeepAlive` lo revive si se cae
—comprobado con `kill -9`: vuelve en un segundo con PID nuevo—. Los logs van a
`~/Library/Logs/korven-gateway.log`.

Sirve el binario compilado, así que **tras tocar código Go hay que recompilar**;
si no, launchd sigue sirviendo la versión vieja sin quejarse:

```bash
cd gateway && go build -o server ./cmd/server
launchctl kickstart -k gui/$(id -u)/dev.korven.opentv.gateway
```

Para desinstalarlo:

```bash
launchctl bootout gui/$(id -u)/dev.korven.opentv.gateway
rm ~/Library/LaunchAgents/dev.korven.opentv.gateway.plist
```

`WorkingDirectory` no es decorativo: `DB_PATH` es relativo al directorio de
trabajo y launchd arranca en `/`. Sin fijarlo, el gateway se crea una `iptv.db`
vacía en la raíz y sirve cero canales con un `200` — un fallo bastante peor de
diagnosticar que un puerto cerrado.
