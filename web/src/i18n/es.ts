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
  'filtro.limpiar': 'Limpiar filtros',
  // El nombre de la clave y la etiqueta siguen la semántica de mostrarOffline
  // (http.ts: true → ?alive=all). El gateway YA oculta los muertos por
  // defecto; esta casilla los REVELA, no los oculta — de ahí "Mostrar", no
  // "Ocultar". Coherente con el toggle "Mostrar canales offline" de la app
  // de macOS (mobile/lib/presentation/widgets/console_bar.dart).
  'filtro.mostrarOffline': 'Mostrar los que no responden',

  // Tarea 9 (P0.7): buscador de tipeo del propio grupo de facetas, cuando
  // tiene más de doce entradas — sustituye al viejo «Ver los N países»
  // (Tarea 6, P0.6), que con cientos de países se volvía un muro sin salida.
  'filtro.filtrarPais': 'Filtrar países',
  'filtro.filtrarCategoria': 'Filtrar categorías',

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

  // Reproductor-primero (2026-08-26): escenario con vídeo persistente y
  // catálogo lateral (ver la spec del mismo nombre).
  'lateral.lista': 'Canales',
  'escenario.reproduciendo': 'Reproduciendo',
  'escenario.verTodo': 'Ver todo',
  'escenario.volver': 'Volver al reproductor',
  'escenario.eligeCanal': 'Elige un canal para empezar',
  'escenario.anuncioReproduciendo': 'Reproduciendo {nombre}',

  'canal.favorito.anadir': 'Añadir a favoritos',
  'canal.favorito.quitar': 'Quitar de favoritos',
  'canal.geo': 'Puede estar bloqueado en tu región',
  'canal.soloApp': 'Este canal se ve en la app instalada o en Safari',

  // Tarea 8 (P2, EPG): insignia ahora/después de TarjetaCanal — "Ahora: <X>"
  // y, si hay siguiente programa, "Sig HH:MM · <Y>" (hora local, 24h).
  'epg.ahora': 'Ahora',
  'epg.siguiente': 'Sig',

  // Tarea 9 (P2, EPG): overlay del reproductor — a diferencia de la
  // insignia de TarjetaCanal (que queda limpia sin guía), aquí "sin guía"
  // es EXPLÍCITO: estar viendo un canal sin saber si hay guía o no es
  // distinto de hojear el catálogo.
  'epg.sinGuia': 'Sin guía para esta fuente',

  // Usadas por SenalCanal.svelte (Tarea 8, P0.6, extraído en el fix 1):
  // mismo componente en la insignia de la rejilla y en la fila de lista, así
  // que mismo estado, mismo texto en ambas vistas. 'senal.muerta' pasa de
  // "No responde" a "Sin respuesta" para casar con el texto exacto del brief.
  'senal.viva': 'Señal viva',
  'senal.muerta': 'Sin respuesta',
  'senal.sinDatos': 'Sin comprobar',

  // Tarea 7 (tiempo-hasta-la-imagen): tiempo-hasta-la-imagen por canal
  // (Task 1-4, GET /channels/imagen), leído en SenalCanal a través del
  // store `imagen` de App.svelte. sinImagen pisa a imagenMs (un mirror sin
  // imagen no tiene sentido mostrarlo con un tiempo).
  'senal.sinImagen': 'Sin imagen desde aquí',
  'senal.imagenEn': 'Imagen en {s} s',

  // Tarea 6 (P0.6): bloque «Señal» de la barra lateral de facetas — encabezado
  // del grupo y la fila que resume el estado por defecto (solo canales vivos).
  'senal.titulo': 'Señal',
  'senal.soloViva': 'Solo señal viva',

  'reproductor.cargando': 'Conectando con el canal…',
  'reproductor.silenciar': 'Silenciar',
  // Reproductor-primero (spec §5): CTA sobre el vídeo cuando la entrada
  // auto-reproduce en silencio (los navegadores bloquean autoplay con sonido).
  'reproductor.activarSonido': 'Toca para activar el sonido',
  // Tarea 2 (P0.8): el botón de pantalla completa ahora refleja el estado
  // real (fullscreenchange sobre el CONTENEDOR, no el <video>) — entrar/salir
  // sustituye al rótulo fijo de antes, igual que el patrón silenciar/
  // favorito ya usa aria-pressed + etiqueta que cambia con el estado.
  'reproductor.pantallaCompleta.entrar': 'Entrar en pantalla completa',
  'reproductor.pantallaCompleta.salir': 'Salir de pantalla completa',
  // Picture-in-Picture: botón solo se renderiza si el navegador lo soporta
  // (document.pictureInPictureEnabled) — Firefox/iOS Safari difieren.
  'reproductor.pip.activar': 'Activar Picture-in-Picture',
  'reproductor.pip.desactivar': 'Salir de Picture-in-Picture',
  // Tarea 1 (P0.8): insignia del overlay 1b sobre el vídeo — "en directo",
  // no un estado de salud (ese es senal.viva/muerta/sinDatos, en SenalCanal).
  'reproductor.envivo': 'En vivo',
  'reproductor.error.noArranco': 'El canal no llegó a reproducir. Puede estar caído, geo-bloqueado o su dirección caducó.',
  'reproductor.error.corte': 'Se perdió la señal del canal.',
  'reproductor.error.caido': 'El canal está caído o su dirección caducó.',
  'reproductor.error.geo': 'Puede estar geo-bloqueado en tu región o requerir acceso.',
  'reproductor.error.inestable': 'El canal emite, pero la señal llega rota. Prueba otra vez o elige otro canal.',
  'reproductor.error.formato': 'Tu navegador no puede reproducir este formato. Prueba en Safari o en la app instalada.',
  'reproductor.error.caducado': 'La dirección del canal caducó.',
  // Sonda de códecs (spec 2026-09-05): el vídeo viene en un formato que
  // NINGÚN navegador decodifica (MPEG-2, típicamente). Sin consejo de
  // Safari a propósito: Safari da audio sin imagen.
  'reproductor.error.codec': 'El vídeo de este canal viene en {codecs}, un formato que ningún navegador decodifica. Solo lo puede ver un reproductor de escritorio (VLC o la app instalada).',
  'reproductor.error.codecGenerico': 'El vídeo de este canal viene en un formato que ningún navegador decodifica. Solo lo puede ver un reproductor de escritorio (VLC o la app instalada).',
  // Tarea 6 (tiempo-hasta-la-imagen): todos los mirrors del canal están
  // marcados sinImagen (ningún intento reciente llegó a dar imagen) — a
  // diferencia de codec/codecGenerico (que sí son un callejón sin salida),
  // aquí SÍ hay algo que ofrecer: "Probar de todos modos" (errorProbarIgual)
  // ignora el filtro por si la señal ya se recuperó.
  // {hace} es un valor DESNUDO ("2 min", "3 h": ver tiempo.haceMin/haceH más
  // abajo) — la propia frase pone el "hace" delante, igual que en inglés la
  // frase pone el "ago" detrás.
  'reproductor.error.sinImagen': 'Ningún origen de este canal llega a dar imagen desde aquí (último intento hace {hace}).',
  'reproductor.error.probarIgual': 'Probar de todos modos',

  // Tarea 14 (P0.6): CTA del error de reproducción cuando el canal SÍ tenía
  // mirrors (más allá del destino único de compatibilidad) — reanuda el
  // mismo failover de P0.5 en vez de dejar un callejón sin salida.
  'reproductor.error.mirrorsProbados': 'Se probaron {n} mirrors, ninguno llegó a reproducir.',
  'reproductor.error.reintentar': 'Reintentar',
  // Tarea 6: nota junto a la insignia «En vivo» (y en la región polite
  // persistente) cuando el mirror en curso está confirmado como sin pista de
  // audio (Mirror.audioOk === false) — informativo, no un error: el vídeo sí
  // llega.
  'reproductor.sinAudio': 'Sin audio en este origen',

  // AirPlay (spec 2026-09-03): sin nombre de dispositivo posible — WebKit no
  // expone uno a la página (ver spec §2.6) — así que el texto es genérico.
  'reproductor.airplay': 'AirPlay',
  'reproductor.cast.emitiendo': 'Emitiendo a AirPlay — {canal}',
  'reproductor.cast.parar': 'Dejar de emitir',
  'reproductor.cast.fallo': 'Este canal no se puede emitir por AirPlay. Sigue reproduciéndose aquí.',
  'reproductor.cast.noDisponible': 'Este canal no se pudo emitir por AirPlay antes. Sigue reproduciéndose aquí.',
  'reproductor.cast.terminada': 'Emisión por AirPlay terminada.',

  // Tarea 6 (tiempo-hasta-la-imagen): «hace cuánto» genérico para
  // reproductor.error.sinImagen (formatearHace en Reproductor.svelte).
  // Claves DESNUDAS en los dos idiomas (sin "hace"/"ago"): la preposición
  // vive en la frase que las envuelve, no aquí — así una misma clave sirve
  // para cualquier frase que quiera componerla, sin arriesgarse a un
  // "hace hace 1 min" si la frase ya trae la suya.
  'tiempo.haceMin': '{n} min',
  'tiempo.haceH': '{n} h',

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

  'pie.codigo': 'Ver el código',
  'pie.stats': 'Estadísticas locales',

  // Tarea 10 (P0.7): pie de crédito de marca (PieDeMarca.svelte), integrado
  // en el mismo <footer class="pie"> de arriba. Los nombres propios (Korven,
  // Claude Code) no se traducen — solo el conector "Desarrollado por". El
  // enlace discreto al repo reutiliza pie.codigo, que ya existía sin usar.
  'pieMarca.emblemaAlt': 'Emblema de Korven',
  'pieMarca.desarrolladoPor': 'Desarrollado por',

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

  // Tarea 6 (P0.7): onboarding cuando el catálogo está listo pero no hay
  // ninguna fuente añadida (bring-your-own). Ver Onboarding.svelte.
  'onboarding.titulo': 'Añade una fuente de canales',
  'onboarding.copy': 'Open TV no trae canales de fábrica: añade tu primera lista M3U para empezar a ver algo.',
  'onboarding.url.etiqueta': 'URL de la lista M3U',
  'onboarding.url.placeholder': 'https://ejemplo.com/lista.m3u',
  'onboarding.anadir': 'Añadir',
  'onboarding.anadiendo': 'Añadiendo…',
  'onboarding.fichero.etiqueta': 'O elige un fichero .m3u',
  'onboarding.sugeridas.titulo': 'Sugeridas',
  'onboarding.sugeridas.anadir': 'Añadir {label}',
  'onboarding.error': 'No se pudo añadir la fuente. Comprueba la URL e inténtalo de nuevo.',
  // Versión breve (brief, Tarea 6): el texto legal completo va en la Tarea 8.
  'onboarding.disclaimer': 'Open TV es un reproductor: no aloja contenido. Tú eliges tus fuentes y respondes por ellas.',

  // Fix round 1 (Tarea 6, P0.7): tras añadir una fuente, App sondea el
  // catálogo hasta que aparecen canales — ver SincronizandoFuente.svelte.
  'onboarding.sondeo.titulo': 'Sincronizando la fuente…',
  'onboarding.sondeo.detalle': 'Puede tardar unos segundos, según el tamaño de la lista.',
  'onboarding.sondeo.agotado': 'La fuente se añadió pero todavía no aparecen canales. Puede tardar más, o la lista puede estar vacía.',
  'onboarding.sondeo.reintentar': 'Reintentar',
  // Fix final-review (F2): salida del estado "agotado" — sin esto, quien
  // añadió una fuente con un typo (o una lista vacía) se quedaba atrapado
  // repitiendo "Reintentar" para siempre, sin forma de llegar a la vista de
  // gestión a borrar la fuente rota. Limpia sondeoAgotado y abre #fuentes
  // (ver irAGestionarFuentes en App.svelte).
  'onboarding.sondeo.gestionar': 'Gestionar fuentes',

  // Tarea 7 (P0.7): vista de gestión de fuentes (#fuentes), accesible desde la
  // cabecera. Lista lo ya añadido, permite re-sincronizar o quitar, y
  // reutiliza AnadirFuente.svelte para añadir más.
  'fuentes.abrir': 'Fuentes',
  'fuentes.titulo': 'Fuentes',
  'fuentes.volver': '← Volver',
  'fuentes.etiquetaLista': 'Fuentes añadidas',
  'fuentes.kind.url': 'URL',
  'fuentes.kind.file': 'Fichero',
  'fuentes.canales': '{n} canales',
  'fuentes.nuncaSincronizada': 'Nunca sincronizada',
  'fuentes.relativo.ahora': 'hace un momento',
  'fuentes.relativo.minutos': 'hace {n} min',
  'fuentes.relativo.horas': 'hace {n} h',
  'fuentes.relativo.dias': 'hace {n} d',
  'fuentes.resincronizar': 'Re-sincronizar',
  'fuentes.resincronizar.etiqueta': 'Re-sincronizar {label}',
  'fuentes.resincronizando': 'Sincronizando…',
  'fuentes.quitar': 'Quitar',
  'fuentes.quitar.etiqueta': 'Quitar {label}',
  'fuentes.quitar.confirmar': '¿Seguro? Quitar',
  'fuentes.quitar.confirmar.etiqueta': 'Confirmar: quitar {label}',
  'fuentes.quitar.cancelar': 'Cancelar',
  // F6 (fix final-review): quitar() no tenía catch — un DELETE fallido salía
  // como rechazo sin manejar de un onclick, sin ningún aviso visible. Este
  // mensaje es el único rastro que ve quien usa la app de que la acción no
  // se completó.
  'fuentes.quitar.error': 'No se pudo quitar la fuente. Inténtalo de nuevo.',
  'fuentes.anadirMas.titulo': 'Añadir otra fuente',
  'fuentes.cargando': 'Cargando fuentes…',
  'fuentes.error': 'No se pudieron cargar las fuentes.',
  'fuentes.vacia': 'Todavía no hay fuentes añadidas.',

  // Tarea 8 (P0.7): disclaimer legal completo, al pie de esta vista — la
  // versión breve de onboarding.disclaimer sigue tal cual, esta es la
  // completa. Cuatro líneas cortas, tono Korven: sobrio, declarativo, sin
  // muro de legalese.
  'fuentes.legal.titulo': 'Aviso legal',
  'fuentes.legal.reproductor': 'Open TV es un reproductor: no aloja, no retransmite ni distribuye contenido.',
  'fuentes.legal.responsabilidad': 'Tú eliges tus fuentes y eres responsable de la legalidad del contenido al que accedes con ellas.',
  'fuentes.legal.sinDrm': 'Sin DRM ni elusión de geobloqueo — pensado para televisión abierta (FTA).',
  'fuentes.legal.sugeridas': 'Las fuentes sugeridas son enlaces públicos de la comunidad iptv-org; añadirlas es decisión tuya.',

  // Tarea 4 (P0.8): paleta de comandos ⌘K/Ctrl+K — buscador difuso sobre los
  // canales ya cargados, las facetas y un puñado de acciones. Ver
  // Paleta.svelte.
  'paleta.titulo': 'Paleta de comandos',
  'paleta.placeholder': 'Buscar canales, facetas o acciones…',
  'paleta.grupo.canales': 'Canales',
  'paleta.grupo.facetas': 'Facetas',
  'paleta.grupo.acciones': 'Acciones',
  'paleta.buscarTodos': 'Buscar «{termino}» en todos los canales',
  // "Surf" y "Canal al azar" (accion.aleatorio) son el mismo comando por
  // debajo — dos nombres findables, ver el comentario de accionesBase.
  'paleta.accion.surf': 'Surf: salta a un canal vivo al azar',
  'paleta.accion.quitarSoloFavoritos': 'Quitar «Solo favoritos»',
  'paleta.accion.abrirFuentes': 'Abrir Fuentes',
  'paleta.accion.abrirStats': 'Abrir Estadísticas',
  'paleta.pista': '↑↓ navega · Enter abre · Esc cierra',

  // Tarea 6 (P0.8): vista de ajustes (#ajustes), mismo patrón de hash y
  // punto de acceso en la cabecera que #fuentes. Densidad de la rejilla
  // (ya cableada a RejillaVirtual desde la Tarea 5), «recordar vista»/
  // «recordar filtros» entre sesiones, y «Acerca de» (versión de /health,
  // aviso legal reutilizado de Fuentes, crédito de marca vía PieDeMarca).
  'ajustes.abrir': 'Ajustes',
  'ajustes.titulo': 'Ajustes',
  'ajustes.volver': '← Volver',
  'ajustes.densidad.titulo': 'Densidad de la rejilla',
  'ajustes.densidad.comoda': 'Cómoda',
  'ajustes.densidad.compacta': 'Compacta',
  'ajustes.recordarVista.titulo': 'Recordar vista',
  'ajustes.recordarVista.ayuda': 'Recuerda si prefieres ver los canales en rejilla o en lista entre sesiones.',
  // Default OFF a propósito (brief): un filtro guardado que no encuentra
  // nada al volver es más confuso que útil — quien lo quiera lo activa aquí.
  'ajustes.recordarFiltros.titulo': 'Recordar filtros',
  'ajustes.recordarFiltros.ayuda': 'Recuerda país, categoría, calidad, búsqueda y «solo favoritos» entre sesiones. Desactivado por defecto.',
  'ajustes.acerca.titulo': 'Acerca de',
  'ajustes.acerca.version': 'Versión {version}',
} as const

export type ClaveMensaje = keyof typeof es
