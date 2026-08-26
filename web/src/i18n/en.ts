import type { ClaveMensaje } from './es'

// Record<ClaveMensaje, string>: si falta una clave, el build falla. No hay
// forma de publicar una UI a medio traducir.
export const en: Record<ClaveMensaje, string> = {
  'app.lema': 'Free-to-air television, no accounts, no setup',

  'catalogo.buscar': 'Search a channel',
  'catalogo.vacio': 'No channel matches the filter.',
  'catalogo.cargando': 'Loading channels…',
  'catalogo.total': '{n} channels',

  'vacio.sugerencia.quitar': 'Remove {valor}',
  'vacio.sugerencia.quitarBusqueda': 'Remove the search “{valor}”',

  'filtro.pais': 'Country',
  'filtro.categoria': 'Category',
  'filtro.calidad': 'Quality',
  'filtro.calidad.hd': 'HD (720p or higher)',
  'filtro.calidad.fhd': 'Full HD (1080p or higher)',
  'filtro.calidad.4k': '4K',
  'filtro.limpiar': 'Clear filters',
  'filtro.mostrarOffline': 'Show the ones not responding',
  'filtro.verPaises': 'See all {n} countries',
  'filtro.verMenosPaises': 'Show fewer countries',
  'shell.facetas': 'Facets',

  'accion.aleatorio': 'Random channel',
  'accion.favoritos': 'Favourites only',
  'accion.rejilla': 'Grid view',
  'accion.lista': 'List view',
  'accion.surf': 'Spacebar: jump to a random live channel',

  'chip.quitarFiltro': 'Remove {valor} filter',
  'chip.quitarBusqueda': 'Remove search “{valor}”',

  'rejilla.etiquetaLista': 'Channel list',

  'canal.favorito.anadir': 'Add to favourites',
  'canal.favorito.quitar': 'Remove from favourites',
  'canal.geo': 'May be geo-blocked in your region',
  'canal.soloApp': 'This channel plays in the installed app or in Safari',

  'senal.viva': 'Live signal',
  'senal.muerta': 'Not responding',
  'senal.sinDatos': 'Not checked yet',

  'senal.titulo': 'Signal',
  'senal.soloViva': 'Live signal only',

  'reproductor.cargando': 'Connecting to the channel…',
  'reproductor.cerrar': 'Close',
  'reproductor.silenciar': 'Mute',
  'reproductor.pantallaCompleta': 'Full screen',
  'reproductor.error.noArranco': 'The channel never started playing. It may be down, geo-blocked, or its address expired.',
  'reproductor.error.corte': 'The channel stopped broadcasting.',
  'reproductor.error.caido': 'The channel is down or its address expired.',
  'reproductor.error.geo': 'It may be geo-blocked in your region or require access.',
  'reproductor.error.formato': 'Your browser cannot play this format. Try Safari or the installed app.',
  'reproductor.error.caducado': "The channel's address expired.",

  'reproductor.error.mirrorsDisponibles': 'There are {n} mirrors with better health.',
  'reproductor.error.probarSiguienteMirror': 'Try the next mirror',

  'estado.sincronizando': 'Syncing the catalogue…',
  'estado.sincronizandoDetalle': 'The first run takes a few seconds: about 13,000 channels are downloaded.',
  'estado.gatewayCaido': 'Could not reach Open TV. Is it still running?',
  'estado.sinRed': 'No internet connection.',
  'estado.errorServidor': 'Open TV replied with an error. Please try again in a moment.',

  'frescura.comprobado': 'Checked {horas} h ago',
  'frescura.envivo': 'Checked live',

  'indicador.vivo': 'Checked live',
  'indicador.sincronizando': 'Syncing…',
  'indicador.sinGateway': 'No connection to Open TV',

  'historial.titulo': 'Continue watching',
  'historial.seguir': 'Keep watching',
  'historial.borrar': 'Clear history',

  'pie.fuente': 'The source is the public free-to-air list from iptv-org. Korven broadcasts nothing.',
  'pie.postura': 'No premium channels, no VPN, no geo-block circumvention.',
  'pie.codigo': 'View the code',
  'pie.stats': 'Local stats',

  'idioma.es': 'Español',
  'idioma.en': 'English',

  'stats.titulo': 'Local stats',
  'stats.volver': '← Back',
  'stats.cargando': 'Loading…',
  'stats.error': 'Could not read the stats.',
  'stats.reproduccion': 'Playback',
  'stats.intentos': 'Attempts',
  'stats.iniciados': 'Started',
  'stats.fallos': 'Failures',
  'stats.cortados': 'Cut off',
  'stats.tasaExito': 'Success rate',
  'stats.porMotivo': 'Failures by reason',
  'stats.porVia': 'Direct vs proxy',
  'stats.porMotor': 'By engine',
  'stats.catalogo': 'Catalogue',
  'stats.streamsTotales': 'Total streams',
  'stats.vivos': 'Alive',
  'stats.muertos': 'Dead',
  'stats.webOk': 'web_ok',
  'stats.webNo': 'web_no',
  'stats.webDesconocido': 'Not checked',
  'stats.sinDatos': 'No data yet.',

  'onboarding.titulo': 'Add a channel source',
  'onboarding.copy': "Open TV doesn't ship with channels: add your first M3U list to start watching something.",
  'onboarding.url.etiqueta': 'M3U list URL',
  'onboarding.url.placeholder': 'https://example.com/list.m3u',
  'onboarding.anadir': 'Add',
  'onboarding.anadiendo': 'Adding…',
  'onboarding.fichero.etiqueta': 'Or choose a .m3u file',
  'onboarding.sugeridas.titulo': 'Suggested',
  'onboarding.sugeridas.anadir': 'Add {label}',
  'onboarding.error': 'Could not add the source. Check the URL and try again.',
  'onboarding.disclaimer': 'Open TV is a player: it hosts no content. You choose your sources and you are responsible for them.',

  'onboarding.sondeo.titulo': 'Syncing the source…',
  'onboarding.sondeo.detalle': "It can take a few seconds, depending on the list's size.",
  'onboarding.sondeo.agotado': 'The source was added but no channels have shown up yet. It may take longer, or the list could be empty.',
  'onboarding.sondeo.reintentar': 'Retry',
}
