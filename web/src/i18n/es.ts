// El español es la fuente: sus claves definen el tipo, así que una clave que
// falte en inglés rompe el typecheck y no llega a producción.
export const es = {
  'app.titulo': 'Korven Open TV',
  'app.lema': 'Televisión abierta, sin cuentas y sin configuración',

  'catalogo.buscar': 'Buscar un canal',
  'catalogo.vacio': 'Ningún canal casa con el filtro.',
  'catalogo.cargando': 'Cargando canales…',
  'catalogo.total': '{n} canales',

  'filtro.pais': 'País',
  'filtro.categoria': 'Categoría',
  'filtro.calidad': 'Calidad',
  'filtro.calidad.hd': 'HD (720p o más)',
  'filtro.calidad.fhd': 'Full HD (1080p o más)',
  'filtro.calidad.4k': '4K',
  'filtro.todos': 'Todos',
  'filtro.limpiar': 'Limpiar filtros',
  // El nombre de la clave y la etiqueta siguen la semántica de mostrarOffline
  // (http.ts: true → ?alive=all). El gateway YA oculta los muertos por
  // defecto; esta casilla los REVELA, no los oculta — de ahí "Mostrar", no
  // "Ocultar". Coherente con el toggle "Mostrar canales offline" de la app
  // de macOS (mobile/lib/presentation/widgets/console_bar.dart).
  'filtro.mostrarOffline': 'Mostrar los que no responden',

  'accion.aleatorio': 'Canal al azar',
  'accion.favoritos': 'Solo favoritos',
  'accion.rejilla': 'Ver en rejilla',
  'accion.lista': 'Ver en lista',

  'canal.favorito.anadir': 'Añadir a favoritos',
  'canal.favorito.quitar': 'Quitar de favoritos',
  'canal.geo': 'Puede estar bloqueado en tu región',
  'canal.soloApp': 'Este canal se ve en la app instalada o en Safari',

  'senal.viva': 'Señal viva',
  'senal.muerta': 'No responde',
  'senal.sinDatos': 'Sin comprobar',

  'reproductor.cargando': 'Conectando con el canal…',
  'reproductor.cerrar': 'Cerrar',
  'reproductor.silenciar': 'Silenciar',
  'reproductor.pantallaCompleta': 'Pantalla completa',
  'reproductor.error.noArranco': 'El canal no llegó a reproducir. Puede estar caído, geo-bloqueado o su dirección caducó.',
  'reproductor.error.corte': 'El canal dejó de emitir.',

  'estado.sincronizando': 'Sincronizando el catálogo…',
  'estado.sincronizandoDetalle': 'La primera vez tarda unos segundos: se descargan unos 13 000 canales.',
  'estado.gatewayCaido': 'No se pudo contactar con Open TV. ¿Sigue abierto?',
  'estado.sinRed': 'Sin conexión a internet.',
  'estado.errorServidor': 'Open TV respondió con un error. Vuelve a intentarlo en un momento.',

  'frescura.comprobado': 'Comprobado hace {horas} h',
  'frescura.envivo': 'Comprobado en vivo',

  'pie.fuente': 'La fuente es la lista pública de televisión abierta de iptv-org. Korven no retransmite nada.',
  'pie.postura': 'Sin canales premium, sin VPN, sin elusión de geobloqueo.',
  'pie.codigo': 'Ver el código',
  'pie.stats': 'Estadísticas locales',

  'idioma.es': 'Español',
  'idioma.en': 'English',

  'stats.titulo': 'Estadísticas locales',
  'stats.volver': '← Volver',
  'stats.cargando': 'Cargando…',
  'stats.error': 'No se pudieron leer las estadísticas.',
  'stats.reproduccion': 'Reproducción',
  'stats.intentos': 'Intentos',
  'stats.iniciados': 'Iniciados',
  'stats.fallos': 'Fallos',
  'stats.cortados': 'Cortados',
  'stats.tasaExito': 'Tasa de éxito',
  'stats.porMotivo': 'Fallos por motivo',
  'stats.porVia': 'Directo vs proxy',
  'stats.porMotor': 'Por motor',
  'stats.catalogo': 'Catálogo',
  'stats.streamsTotales': 'Streams totales',
  'stats.vivos': 'Vivos',
  'stats.muertos': 'Muertos',
  'stats.webOk': 'web_ok',
  'stats.webNo': 'web_no',
  'stats.webDesconocido': 'Sin comprobar',
  'stats.sinDatos': 'Sin datos todavía.',
} as const

export type ClaveMensaje = keyof typeof es
