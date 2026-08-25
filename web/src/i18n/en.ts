import type { ClaveMensaje } from './es'

// Record<ClaveMensaje, string>: si falta una clave, el build falla. No hay
// forma de publicar una UI a medio traducir.
export const en: Record<ClaveMensaje, string> = {
  'app.titulo': 'Korven Open TV',
  'app.lema': 'Free-to-air television, no accounts, no setup',

  'catalogo.buscar': 'Search a channel',
  'catalogo.vacio': 'No channel matches the filter.',
  'catalogo.cargando': 'Loading channels…',
  'catalogo.total': '{n} channels',

  'filtro.pais': 'Country',
  'filtro.categoria': 'Category',
  'filtro.calidad': 'Quality',
  'filtro.calidad.hd': 'HD (720p or higher)',
  'filtro.calidad.fhd': 'Full HD (1080p or higher)',
  'filtro.calidad.4k': '4K',
  'filtro.todos': 'All',
  'filtro.limpiar': 'Clear filters',
  'filtro.mostrarOffline': 'Show the ones not responding',

  'accion.aleatorio': 'Random channel',
  'accion.favoritos': 'Favourites only',
  'accion.rejilla': 'Grid view',
  'accion.lista': 'List view',

  'canal.favorito.anadir': 'Add to favourites',
  'canal.favorito.quitar': 'Remove from favourites',
  'canal.soloApp': 'This channel plays in the installed app or in Safari',

  'senal.viva': 'Live signal',
  'senal.muerta': 'Not responding',
  'senal.sinDatos': 'Not checked yet',

  'reproductor.cargando': 'Connecting to the channel…',
  'reproductor.cerrar': 'Close',
  'reproductor.silenciar': 'Mute',
  'reproductor.pantallaCompleta': 'Full screen',
  'reproductor.error.noArranco': 'The channel never started playing. It may be down, geo-blocked, or its address expired.',
  'reproductor.error.corte': 'The channel stopped broadcasting.',

  'estado.sincronizando': 'Syncing the catalogue…',
  'estado.sincronizandoDetalle': 'The first run takes a few seconds: about 13,000 channels are downloaded.',
  'estado.gatewayCaido': 'Could not reach Open TV. Is it still running?',
  'estado.sinRed': 'No internet connection.',

  'frescura.comprobado': 'Checked {horas} h ago',
  'frescura.envivo': 'Checked live',

  'pie.fuente': 'The source is the public free-to-air list from iptv-org. Korven broadcasts nothing.',
  'pie.postura': 'No premium channels, no VPN, no geo-block circumvention.',
  'pie.codigo': 'View the code',

  'idioma.es': 'Español',
  'idioma.en': 'English',
}
