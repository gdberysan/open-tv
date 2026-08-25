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
  'filtro.todos': 'Todos',
  'filtro.limpiar': 'Limpiar filtros',
  'filtro.ocultarOffline': 'Ocultar los que no responden',

  'accion.aleatorio': 'Canal al azar',
  'accion.favoritos': 'Solo favoritos',
  'accion.rejilla': 'Ver en rejilla',
  'accion.lista': 'Ver en lista',

  'canal.favorito.anadir': 'Añadir a favoritos',
  'canal.favorito.quitar': 'Quitar de favoritos',
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

  'frescura.comprobado': 'Comprobado hace {horas} h',
  'frescura.envivo': 'Comprobado en vivo',

  'pie.fuente': 'La fuente es la lista pública de televisión abierta de iptv-org. Korven no retransmite nada.',
  'pie.postura': 'Sin canales premium, sin VPN, sin elusión de geobloqueo.',
  'pie.codigo': 'Ver el código',

  'idioma.es': 'Español',
  'idioma.en': 'English',
} as const

export type ClaveMensaje = keyof typeof es
