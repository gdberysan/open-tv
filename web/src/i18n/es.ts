// El español es la fuente: sus claves definen el tipo, así que una clave que
// falte en inglés rompe el typecheck y no llega a producción.
export const es = {
  'app.lema': 'Televisión abierta, sin cuentas y sin configuración',

  'catalogo.buscar': 'Buscar un canal',
  'catalogo.vacio': 'Ningún canal casa con el filtro.',
  'catalogo.cargando': 'Cargando canales…',
  'catalogo.total': '{n} canales',

  // Tarea 13 (P0.6): CTA del estado vacío (Vacio.svelte) que sugiere quitar
  // la dimensión de filtro más restrictiva. quitarBusqueda distingue la
  // búsqueda de texto libre (comillas) del resto de dimensiones, igual que
  // chip.quitarFiltro/chip.quitarBusqueda en ChipsFiltro.
  'vacio.sugerencia.quitar': 'Quitar {valor}',
  'vacio.sugerencia.quitarBusqueda': 'Quitar la búsqueda «{valor}»',

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

  // Tarea 6 (P0.6): «Ver los N países» expande la lista de país, colapsada
  // por defecto porque puede tener decenas de entradas.
  'filtro.verPaises': 'Ver los {n} países',
  'filtro.verMenosPaises': 'Ver menos países',

  // Shell de dos columnas (Tarea 4): el botón de cajón alterna la barra
  // lateral de facetas en pantallas estrechas; el mismo texto etiqueta el
  // encabezado (accesible, oculto visualmente) del propio aside mientras su
  // contenido real (Tarea 6) todavía no existe.
  'shell.facetas': 'Facetas',

  'accion.aleatorio': 'Canal al azar',
  'accion.favoritos': 'Solo favoritos',
  'accion.rejilla': 'Ver en rejilla',
  'accion.lista': 'Ver en lista',

  // Tarea 11 (P0.6): afordancia del gesto "surf" — la barra espaciadora hace
  // el mismo salto que el botón "Canal al azar" cuando el foco no está en un
  // control (ver debeHacerSurf en lib/surf.ts).
  'accion.surf': 'Barra espaciadora: salta a un canal vivo al azar',

  // Tarea 7 (P0.6): chips removibles de la barra de acciones — uno por
  // dimensión de filtro activa (país/categoría/calidad/favoritos comparten la
  // misma redacción; la búsqueda de texto lleva la suya, entre comillas).
  'chip.quitarFiltro': 'Quitar filtro {valor}',
  'chip.quitarBusqueda': 'Quitar búsqueda «{valor}»',

  // Tarea 18 (auditoría de accesibilidad): nombre del rol de lista de la
  // rejilla/lista de canales, para quien navega con lector de pantalla.
  'rejilla.etiquetaLista': 'Lista de canales',

  'canal.favorito.anadir': 'Añadir a favoritos',
  'canal.favorito.quitar': 'Quitar de favoritos',
  'canal.geo': 'Puede estar bloqueado en tu región',
  'canal.soloApp': 'Este canal se ve en la app instalada o en Safari',

  // Usadas por SenalCanal.svelte (Tarea 8, P0.6, extraído en el fix 1):
  // mismo componente en la insignia de la rejilla y en la fila de lista, así
  // que mismo estado, mismo texto en ambas vistas. 'senal.muerta' pasa de
  // "No responde" a "Sin respuesta" para casar con el texto exacto del brief.
  'senal.viva': 'Señal viva',
  'senal.muerta': 'Sin respuesta',
  'senal.sinDatos': 'Sin comprobar',

  // Tarea 6 (P0.6): bloque «Señal» de la barra lateral de facetas — encabezado
  // del grupo y la fila que resume el estado por defecto (solo canales vivos).
  'senal.titulo': 'Señal',
  'senal.soloViva': 'Solo señal viva',

  'reproductor.cargando': 'Conectando con el canal…',
  'reproductor.cerrar': 'Cerrar',
  'reproductor.silenciar': 'Silenciar',
  'reproductor.pantallaCompleta': 'Pantalla completa',
  'reproductor.error.noArranco': 'El canal no llegó a reproducir. Puede estar caído, geo-bloqueado o su dirección caducó.',
  'reproductor.error.corte': 'El canal dejó de emitir.',
  'reproductor.error.caido': 'El canal está caído o su dirección caducó.',
  'reproductor.error.geo': 'Puede estar geo-bloqueado en tu región o requerir acceso.',
  'reproductor.error.formato': 'Tu navegador no puede reproducir este formato. Prueba en Safari o en la app instalada.',
  'reproductor.error.caducado': 'La dirección del canal caducó.',

  'estado.sincronizando': 'Sincronizando el catálogo…',
  'estado.sincronizandoDetalle': 'La primera vez tarda unos segundos: se descargan unos 13 000 canales.',
  'estado.gatewayCaido': 'No se pudo contactar con Open TV. ¿Sigue abierto?',
  'estado.sinRed': 'Sin conexión a internet.',
  'estado.errorServidor': 'Open TV respondió con un error. Vuelve a intentarlo en un momento.',

  'frescura.comprobado': 'Comprobado hace {horas} h',
  'frescura.envivo': 'Comprobado en vivo',

  // Tarea 5 (P0.6): IndicadorSenal en la cabecera. El texto es la fuente de
  // verdad del estado, no el color del punto (accesibilidad: el color solo
  // refuerza lo que el texto ya dice).
  'indicador.vivo': 'Comprobado en vivo',
  'indicador.sincronizando': 'Sincronizando…',
  'indicador.sinGateway': 'Sin conexión con Open TV',

  // Tarea 10 (P0.6): héroe compacto "Continuar viendo", encima de
  // BarraAcciones, alimentado por el store `historial` (Tarea 9).
  'historial.titulo': 'Continuar viendo',
  'historial.seguir': 'Seguir viendo',
  'historial.borrar': 'Borrar historial',

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
