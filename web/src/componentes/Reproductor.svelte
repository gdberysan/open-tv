<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import type { Canal, CatalogSource } from '../datos/catalogo'
  import { PlaybackGuard } from '../reproductor/guard'
  import { planDeReproduccion, motorDelNavegador, urlProxy, type Motor } from '../reproductor/plan'
  import { planDeFailover, type Intento, type DesenlaceReproduccion } from '../reproductor/failover'
  import { clasificarError, type ClaseError } from '../estado/salud'
  import { clasificarFallo, claseConsensuada, type ClaseFallo, type InfoFallo } from '../reproductor/diagnostico'
  import { t } from '../i18n'
  import type { ClaveMensaje } from '../i18n/es'
  import { parsearResolucion } from '../lib/resolucion'
  import { formatearHoraLocal } from '../lib/hora'
  import { favoritos } from '../estado/favoritos'
  import { esObjetivoInteractivo } from '../lib/surf'
  import { noCasteaPorFormato, marcarFalloFormato } from '../estado/castFallidos'

  // alAnterior/alSiguiente son opcionales: App los da cuando hay una lista de
  // canales de la que moverse (flechas ← →). fuente es el CatalogSource: el
  // Reproductor ya no recibe una URL resuelta, pide sus propios mirrors
  // (Tarea 5) para poder recorrer planDeFailover. alDesenlace y alIntentar son
  // opcionales — no-op en producción salvo que la Tarea 12/el test los den.
  //
  // Reproductor-primero (spec §4): PANEL persistente del escenario, ya no un
  // modal — sin role=dialog, sin focus-trap, sin «cerrar» (siempre hay un
  // canal en curso una vez hay catálogo). Esc solo sale de pantalla completa,
  // y el teclado global respeta esObjetivoInteractivo (el buscador de la
  // lateral coexiste en el orden de tabulación). `activo` apaga el teclado
  // global cuando otra vista cubre el escenario (ver-todo, vistas-hash,
  // paleta). El MOTOR (failover/guard/EPG/PiP) es el de siempre: solo cambió
  // el envoltorio y el modelo de foco.
  let {
    canal,
    fuente,
    activo = true,
    silenciadoInicial = false,
    alAnterior,
    alSiguiente,
    alDesenlace = () => {},
    alIntentar = () => {},
  }: {
    canal: Canal
    fuente: CatalogSource
    activo?: boolean
    silenciadoInicial?: boolean
    alAnterior?: () => void
    alSiguiente?: () => void
    alDesenlace?: (o: DesenlaceReproduccion) => void
    alIntentar?: (url: string) => void
  } = $props()

  let video: HTMLVideoElement | undefined = $state()
  let cargando = $state(true)
  let mensajeError = $state<string | null>(null)
  // >0 SOLO junto al mensajeError de agotar el failover (claveDeClase más
  // abajo): cuántos mirrors tenía este canal cuando se pidieron. Sirve para
  // ofrecer el CTA "Probar el siguiente mirror" — 0 en cualquier otro
  // mensajeError (canal.soloApp, el catch de mirrors()/destino(), un corte
  // tras confirmar) porque ninguno de esos tiene un failover que reanudar.
  let numMirrorsProbados = $state(0)
  // Error sin salida: reintentar no cambia los códecs de un origen, y un
  // botón que no puede arreglar nada es una promesa falsa (misma lección
  // que el contador de mirrors «con mejor salud»).
  let errorSinReintento = $state(false)
  // untrack: silenciadoInicial es a propósito SOLO el valor inicial (es la
  // condición de ENTRADA del escenario, no un estado vivo que seguir) — mismo
  // criterio que los untrack de App.svelte para lecturas iniciales; sin él,
  // svelte-check lo marca como warning de referencia local.
  let silenciado = $state(untrack(() => silenciadoInicial))

  // Spec §5: la entrada auto-reproduce EN SILENCIO (los navegadores bloquean
  // autoplay con sonido) con una affordance clara para activarlo. La CTA vive
  // mientras el silencio siga siendo el "de entrada": cualquier gesto que
  // active el sonido (la propia CTA, Silenciar manual, o elegir otro canal —
  // ver el $effect de canal.id) la retira para siempre.
  let mostrarActivarSonido = $state(untrack(() => silenciadoInicial))

  function activarSonido() {
    silenciado = false
    if (video) video.muted = false
    mostrarActivarSonido = false
  }

  // Overlay 1b (Tarea 1, P0.8): insignia + línea meta + nombre + controles
  // etiquetados, sobre el <video>. esFavorito es derivado del store
  // compartido (mismo patrón que TarjetaCanal.svelte); resolucion, la misma
  // heurística sobre el nombre que ya usa TarjetaCanal (lib/resolucion.ts) —
  // el catálogo no trae un campo propio, así que sin match no hay insignia
  // de resolución, no se inventa un dato que el nombre no dice.
  const esFavorito = $derived($favoritos.has(canal.id))
  const resolucion = $derived(parsearResolucion(canal.nombre))
  const metaLinea = $derived(
    [resolucion, canal.latenciaMs > 0 ? `${canal.latenciaMs} ms` : null, canal.pais || null]
      .filter((parte): parte is string => Boolean(parte))
      .join(' · '),
  )

  // EPG ahora/después del overlay (Tarea 9, P2): null = todavía sin
  // resolver (estado neutro, no se pinta nada roto mientras llega) O un
  // fallo de epgDeCanal (try/catch más abajo) — un fallo de red no es lo
  // mismo que una fuente que YA confirmó que no tiene guía para este canal,
  // así que no cae en el mensaje explícito de "sin guía". A diferencia de
  // TarjetaCanal (Tarea 8, que queda limpia sin guía), aquí SÍ hay un texto
  // explícito («sin guía para esta fuente», epg.sinGuia) para "ahora=null y
  // sin próximos": el usuario ya tiene un canal abierto y merece saber que
  // de verdad no hay guía, no solo que aún no llegó.
  let epgLinea = $state<string | null>(null)
  // Mismo patrón que intentoId más arriba: si el usuario abre OTRO canal
  // mientras esta petición sigue en el aire, la respuesta tardía se descarta
  // en vez de pintar la guía del canal anterior sobre el nuevo.
  let epgPeticionId = 0

  // Cada cuánto se re-consulta la guía del canal abierto para que "Ahora" no
  // envejezca en una sesión larga (ver un canal >1h): el programa en curso
  // cambia en los bordes (típico 30-60 min), así que refrescar cada par de
  // minutos basta. La tarjeta ya hace lo propio vía el store; el overlay del
  // reproductor lo replica para su superficie.
  const REFRESCO_EPG_MS = 2 * 60_000

  // esRefresco=true (el tick del intervalo) NO limpia epgLinea al empezar:
  // mantiene el texto vigente hasta que llega el nuevo, sin parpadeo. Solo el
  // cambio de canal (esRefresco=false) vuelve al estado neutro mientras carga.
  async function cargarEpg(idCanal: string, esRefresco = false) {
    const miId = ++epgPeticionId
    if (!esRefresco) epgLinea = null
    try {
      const r = await fuente.epgDeCanal(idCanal)
      if (destruido || miId !== epgPeticionId) return
      if (r.ahora) {
        const rango = `${formatearHoraLocal(r.ahora.inicioSeg)}–${formatearHoraLocal(r.ahora.finSeg)}`
        let texto = `${t('epg.ahora')}: ${r.ahora.titulo} (${rango})`
        if (r.proximos[0]) texto += ` · ${t('epg.siguiente')}: ${r.proximos[0].titulo}`
        epgLinea = texto
      } else if (r.proximos[0]) {
        // Sin "ahora" pero con algo próximo: no es "sin guía" (el catálogo
        // sí tiene datos para este canal), pero tampoco hay nada "ahora"
        // que anunciar.
        epgLinea = `${t('epg.siguiente')}: ${r.proximos[0].titulo}`
      } else {
        epgLinea = t('epg.sinGuia')
      }
    } catch {
      // Un fallo al pedir la guía nunca rompe el reproductor: se queda en
      // el estado neutro (sin texto), igual que mientras la petición está
      // en curso.
      if (destruido || miId !== epgPeticionId) return
      epgLinea = null
    }
  }

  // Se re-suscribe solo cuando canal.id cambia de verdad (abrir OTRO canal):
  // el CTA de "probar el siguiente mirror" y el failover automático reusan
  // el MISMO canal.id, así que no disparan una nueva petición de guía.
  $effect(() => {
    const id = canal.id
    void cargarEpg(id)
    // Mientras el overlay siga en ESTE canal, refresca la guía por intervalo
    // (mantiene "Ahora" fresco). El cleanup del $effect corre al cambiar de
    // canal o al desmontar → sin fuga de intervalo ni guía obsoleta.
    const intervalo = setInterval(() => void cargarEpg(id, true), REFRESCO_EPG_MS)
    return () => clearInterval(intervalo)
  })

  // El listener se engancha una sola vez: video es el MISMO elemento
  // persistente durante toda la vida del panel (spec §3) — no hay que
  // re-enganchar al cambiar de canal.
  $effect(() => {
    if (!video || !soportaAirplay) return
    const el = video
    el.addEventListener('webkitcurrentplaybacktargetiswirelesschanged', alCambioRutaAirplay)
    return () => el.removeEventListener('webkitcurrentplaybacktargetiswirelesschanged', alCambioRutaAirplay)
  })

  // Auto-ocultar del overlay: visible por defecto (también durante
  // cargando/error, que tienen su propio estado centrado y no chocan con la
  // insignia/controles de los bordes). mostrar() se llama al mousemove/
  // keydown Y cada vez que se confirma la reproducción (el $effect de más
  // abajo) — solo arma el temporizador de ~3s mientras reproduce de verdad;
  // en cargando/error se queda visible sin temporizador.
  let ocultarOverlay = $state(false)
  let overlayEl: HTMLDivElement | undefined = $state()
  let temporizadorOverlay: ReturnType<typeof setTimeout> | undefined

  function mostrar() {
    ocultarOverlay = false
    if (temporizadorOverlay !== undefined) clearTimeout(temporizadorOverlay)
    if (cargando || mensajeError) return
    temporizadorOverlay = setTimeout(ocultarSiProcede, 3000)
  }

  function ocultarSiProcede() {
    // NUNCA se oculta si el foco de teclado sigue dentro del overlay: un
    // usuario de teclado se quedaría con el foco en un control que ya no ve.
    // Se reprograma el mismo plazo en vez de ocultar.
    if (overlayEl && document.activeElement && overlayEl.contains(document.activeElement)) {
      temporizadorOverlay = setTimeout(ocultarSiProcede, 3000)
      return
    }
    ocultarOverlay = true
  }

  // Si el foco llega al overlay por Tab (no por ratón), tiene que reaparecer
  // — sin esto, tabular hacia un overlay ya oculto dejaría el foco en un
  // control invisible.
  function alRecibirFocoOverlay() {
    mostrar()
  }

  // Arranca el ciclo de auto-ocultar en cuanto se confirma la reproducción
  // (cargando pasa a false sin error) — no hace falta esperar al primer
  // mousemove/keydown del usuario para que el overlay empiece a poder
  // ocultarse.
  $effect(() => {
    if (!cargando && !mensajeError) mostrar()
  })

  // Misma idea que el $effect de arriba (mostrar()): cargando=false Y
  // mensajeError=null significa "este intento confirmó reproducción",
  // cualquiera sea el motor. Con motorForzado='nativo' eso es "la sesión de
  // AirPlay está reproduciendo de verdad" — PlaybackGuard.alConfirmar() ya
  // hizo cargando=false, aquí solo se traduce a estadoCast.
  $effect(() => {
    if (motorForzado === 'nativo' && estadoCast === 'conectando' && !cargando && !mensajeError) {
      estadoCast = 'emitiendo'
    }
  })

  // Contenedor raíz del panel: destino de requestFullscreen (así el overlay
  // sigue visible en pantalla completa, mismo patrón que P0.8). El panel NO
  // captura ni restaura el foco: vive en el orden natural de la página
  // (spec §6) — la maquinaria de foco del modal se fue con el modal.
  let contenedorRaiz: HTMLDivElement | undefined = $state()

  // AirPlay de Safari: es una llamada y un evento, sin capa nativa. Si el
  // navegador no lo expone, el botón no existe. La app de macOS es la que
  // hace sesiones de emisión de verdad; aquí solo se abre el selector del
  // sistema.
  const soportaAirplay = typeof window !== 'undefined' && 'WebKitPlaybackTargetAvailabilityEvent' in window

  // Tarea 2 (P0.8): pantalla completa sobre el CONTENEDOR del diálogo (no el
  // <video>) — así el overlay y la barra de controles siguen visibles en
  // pantalla completa, que antes desaparecían porque solo el <video> se
  // expandía. soportaFullscreen/soportaPiP se leen una vez al montar (mismo
  // patrón que soportaAirplay); document.fullscreenEnabled/
  // pictureInPictureEnabled son el feature-detect — Firefox/iOS difieren, y
  // jsdom no los define en absoluto (los tests los mockean a mano).
  const soportaFullscreen = typeof document !== 'undefined' && document.fullscreenEnabled === true
  const soportaPiP =
    typeof document !== 'undefined' && 'pictureInPictureEnabled' in document && document.pictureInPictureEnabled === true
  let estaEnPantallaCompleta = $state(false)
  let estaEnPiP = $state(false)

  let guardActual: PlaybackGuard | undefined

  // Eventos del <video> que demuestran avance real de la carga (valen para el
  // motor nativo y para hls.js). 'progress' se emite mientras el buffer crece;
  // los otros dos marcan que ya hay medio decodificable.
  const EVENTOS_PROGRESO = ['progress', 'loadedmetadata', 'canplay'] as const

  // Sesión de AirPlay (spec 2026-09-03): motorForzado fuerza 'nativo' en
  // reproducir() en vez de dejar que motorDelNavegador() decida — hoy en
  // Safari real SIEMPRE elige hls.js (canPlayType devuelve 'maybe', no
  // 'probably'; ver plan.ts), y AirPlay no reproduce fuentes MSE/blob
  // (confirmado con hardware real, spec §2.1). null = sin cast en curso.
  // El tipo NO incluye 'fallido' (spec §4): ese estado es una transición
  // instantánea de un solo tick (Step 4 más abajo hace el aviso + vuelve a
  // 'idle' + reproducir() en la misma pasada), nunca algo que la UI necesite
  // pintar de forma sostenida — por eso no hay una cuarta rama visual en la
  // Task 4.
  let motorForzado: Motor | null = $state(null)
  let estadoCast = $state<'idle' | 'conectando' | 'emitiendo'>('idle')
  // Mensaje transitorio de cast (fallo o fin de sesión) — se pinta como panel
  // visible (ver el marcado) Y se anuncia por la región aria-live assertive
  // que ya existía, NUNCA por una tercera región.
  let avisoCast = $state<string | null>(null)
  // Fix de revisión final: el aviso se AUTO-BORRA. Sin esto se quedaba
  // pegado hasta el siguiente iniciarCast() — y como la región assertive
  // pinta `mensajeError ?? avisoCast`, un aviso viejo resucitaba (y volvía a
  // alertar al lector de pantalla) en cuanto un mensajeError ajeno se
  // limpiaba, días después y sin venir a cuento. 5 s es el mismo orden de
  // magnitud que el auto-ocultar del overlay: suficiente para leerlo, corto
  // para no tapar la UI normal de carga/error.
  const AVISO_CAST_MS = 5_000
  let temporizadorAvisoCast: ReturnType<typeof setTimeout> | undefined

  function mostrarAvisoCast(texto: string) {
    avisoCast = texto
    if (temporizadorAvisoCast !== undefined) clearTimeout(temporizadorAvisoCast)
    temporizadorAvisoCast = setTimeout(() => {
      temporizadorAvisoCast = undefined
      avisoCast = null
    }, AVISO_CAST_MS)
  }

  function limpiarAvisoCast() {
    if (temporizadorAvisoCast !== undefined) {
      clearTimeout(temporizadorAvisoCast)
      temporizadorAvisoCast = undefined
    }
    avisoCast = null
  }

  // Cancelar el selector nativo NO dispara ningún evento (la API no lo
  // avisa): sin esto, abrirSelectorAirplay() ya había preparado el motor
  // nativo (spec del fix de 2026-09-04) y la app se quedaba pensando que
  // emitía —el efecto de más abajo pasa a 'emitiendo' con solo confirmar
  // reproducción LOCAL, sin saber si AirPlay llegó a recibir nada— sin
  // ninguna señal de que el usuario canceló. rutaConfirmadaEnEsteIntento
  // distingue "todavía no ha llegado la confirmación real" de "nunca va a
  // llegar"; el trace real del fix mostró ~11,4 s entre el clic y la
  // confirmación de ruta en un intento que SÍ funcionó, así que el margen
  // tiene que ser bastante mayor que eso.
  const TIMEOUT_SELECTOR_CANCELADO_MS = 45_000
  let temporizadorSelectorCancelado: ReturnType<typeof setTimeout> | undefined
  let rutaConfirmadaEnEsteIntento = false

  // any: el tipo real de Hls solo existe tras el import() perezoso.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let hlsActual: any
  let destruido = false

  // Info del error MÁS reciente del intento en curso: la rellenan el handler
  // de Hls.Events.ERROR (solo si data.fatal — un no-fatal no dice nada del
  // desenlace) y onVideoError. Se reinicia al empezar cada intentar(); si
  // este intento falla, clasificarFallo() la lee para dar un motivo granular
  // en vez del "timeout-de-carga"/"tipo:detalles" crudo de antes.
  let infoUltimoError: InfoFallo = {}

  // Se incrementa en cada llamada a reproducir(): si el canal cambia (o el
  // componente se destruye) mientras una petición de mirrors()/destino() aún
  // está en el aire, la respuesta tardía queda descartada en vez de pisar el
  // intento del canal nuevo. Mismo patrón que peticionActual en App.svelte.
  let intentoId = 0

  function limpiarIntento() {
    // abortar(), no destruir(): este guard no falló, es el failover
    // decidiendo por su cuenta pasar al siguiente mirror (o el componente
    // cerrándose). destruir() queda para el vocabulario del guard en sí.
    guardActual?.abortar()
    guardActual = undefined
    if (hlsActual) {
      hlsActual.destroy()
      hlsActual = undefined
    }
    if (video) {
      video.removeEventListener('timeupdate', onTimeUpdate)
      video.removeEventListener('error', onVideoError)
      for (const ev of EVENTOS_PROGRESO) video.removeEventListener(ev, onProgreso)
      video.removeAttribute('src')
      video.load()
    }
  }

  function onTimeUpdate() {
    guardActual?.alPosicion(video?.currentTime ?? 0)
  }

  // Prueba de que la tubería AVANZA aunque todavía no se vea nada. Sin esto la
  // única señal de vida era timeupdate, que no llega hasta que el elemento
  // reproduce de verdad: un canal sano pero lento moría a los 7 s. Ver
  // PlaybackGuard.alProgreso().
  function onProgreso() {
    guardActual?.alProgreso()
  }

  /**
   * Chrome NO abre un MediaSource en una pestaña oculta: el <video> se queda
   * con un blob que nunca llega a 'sourceopen', hls.js sigue sondeando la
   * playlist —el canal está VIVO— pero no pide un solo segmento. Gastar ahí el
   * presupuesto de carga era declarar caído un canal sano, y como la app
   * reanuda «continuar viendo» al arrancar, abrirla en segundo plano daba ese
   * error siempre.
   *
   * Oculta: se congela el presupuesto. Al volver a verse, se reintenta desde
   * cero en vez de confiar en que el navegador reanime un MediaSource que
   * nació muerto — un intento nuevo es barato y sí tiene garantía de arrancar.
   */
  function onVisibilidad() {
    if (destruido) return
    if (document.hidden) {
      guardActual?.pausar()
      return
    }
    // Solo si el intento en curso no llegó a confirmar: un canal que ya se ve
    // no se reinicia por cambiar de pestaña.
    if (cargando) {
      limpiarIntento()
      reproducir()
      return
    }
    guardActual?.reanudar()
  }

  function onVideoError() {
    const codigo = video?.error?.code
    if (codigo !== undefined) infoUltimoError = { mediaErrorCode: codigo }
    guardActual?.alError(String(codigo ?? 'error-nativo'))
  }

  // 'desconocido' reutiliza el mensaje genérico de siempre (noArranco): sin
  // señal específica, sigue siendo la triple-adivinanza honesta que ya había.
  function claveDeClase(clase: ClaseFallo): ClaveMensaje {
    switch (clase) {
      case 'caido':
        return 'reproductor.error.caido'
      case 'geo':
        return 'reproductor.error.geo'
      case 'formato':
        return 'reproductor.error.formato'
      case 'inestable':
        return 'reproductor.error.inestable'
      case 'caducado':
        return 'reproductor.error.caducado'
      case 'codec':
        return 'reproductor.error.codecGenerico'
      default:
        return 'reproductor.error.noArranco'
    }
  }

  function via(intento: Intento): 'directo' | 'proxy' {
    return intento.viaProxy ? 'proxy' : 'directo'
  }

  // Mismo reparto de tres estados que MensajeError.svelte usa para el
  // catálogo: gateway caído / sin red / servidor no se pueden mezclar sin
  // repetir el diagnóstico de una tarde entera del 2026-08-07. Se usa aquí
  // para el fetch de mirrors()/destino()/proxyDisponible() — antes de tener
  // siquiera una lista de intentos que probar, no es que "no arrancó".
  function mensajeDeClase(clase: ClaseError): string {
    return clase === 'gateway' ? t('estado.gatewayCaido') : clase === 'red' ? t('estado.sinRed') : t('estado.errorServidor')
  }

  /** Un intento: arma el guard, ENTONCES asigna la fuente. Se resuelve al
   *  confirmar reproducción; se rechaza si el guard lo declara fatal antes de
   *  confirmar. Un fallo DESPUÉS de confirmar no rechaza: el canal ya se vio,
   *  así que es un corte, no un intento fallido — y se reporta como tal. */
  function intentar(intento: Intento, motor: Motor): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      if (!video) {
        reject(new Error('sin elemento de vídeo'))
        return
      }
      let confirmado = false
      // zanjado se cierra al resolver O rechazar. Existe porque guardActual
      // NO alcanza para detectar un import('hls.js') tardío: cuando el guard
      // declara fatal, reject() solo REANUDA el await del bucle en un
      // microtask posterior — durante esa ventana guardActual todavía
      // apunta a ESTE guard (el bucle aún no llegó a limpiarIntento() del
      // siguiente intento). Sin zanjado, un import que resuelve justo en esa
      // ventana pasaría el check de identidad y pisaría hlsActual con un
      // stream que ya se decidió fatal.
      let zanjado = false
      const inicio = performance.now()
      // Info del intento ANTERIOR no vale para clasificar este: cada intentar()
      // arranca en blanco.
      infoUltimoError = {}
      // Lo que el demuxer de hls.js VIO: si al fallar solo había audio, el
      // vídeo iba en un formato que tira en silencio (MPEG-2). Se anota en
      // infoUltimoError al fallar, no antes: un canal de solo audio que sí
      // reproduce nunca pasa por aquí.
      const pistas = { video: false, audio: false }

      const guard = new PlaybackGuard({
        alFallar: (mensaje) => {
          if (confirmado) {
            alDesenlace({
              canalId: canal.id,
              resultado: 'cortado',
              motivo: mensaje,
              motor,
              via: via(intento),
              mirrorIndex: intento.mirrorIndex,
            })
            mensajeError = t('reproductor.error.corte')
            cargando = false
            return
          }
          zanjado = true
          if (pistas.audio && !pistas.video) infoUltimoError = { ...infoUltimoError, sinVideo: true }
          reject(new Error(mensaje))
        },
        alConfirmar: () => {
          confirmado = true
          zanjado = true
          cargando = false
          alDesenlace({
            canalId: canal.id,
            resultado: 'iniciado',
            motor,
            via: via(intento),
            mirrorIndex: intento.mirrorIndex,
            msPrimerFrame: performance.now() - inicio,
          })
          resolve()
        },
      })
      guardActual = guard

      // El guard se arma ANTES de tocar la fuente: si la carga se cuelga, el
      // timeout tiene que saltar igual. Armarlo después fue el bug original.
      guard.armarTimeoutDeCarga()
      // Ya oculta al empezar: no se gasta presupuesto que el navegador no deja
      // usar (ver onVisibilidad).
      if (document.hidden) guard.pausar()

      video.addEventListener('timeupdate', onTimeUpdate)
      video.addEventListener('error', onVideoError)
      for (const ev of EVENTOS_PROGRESO) video.addEventListener(ev, onProgreso)

      if (motor === 'nativo') {
        video.src = intento.url
        // load() explícito: asignar .src debería bastar por espec (dispara el
        // algoritmo de selección de recurso solo), pero verificado a mano en
        // Chrome (Ronda 2 de revisión) el <video> del reproductor se quedaba
        // en readyState 0 para SIEMPRE — ni error ni datos — mientras que un
        // <video> suelto con la MISMA url y un load() explícito sí llegaba a
        // readyState 4 en segundos. Forzar el algoritmo de selección de
        // recurso a mano, en vez de confiar en que la asignación implícita lo
        // dispare, es lo que de verdad hace arrancar la carga en este camino.
        video.load()
        video.play().catch(() => {
          // Un rechazo de play() no es necesariamente fatal (autoplay
          // bloqueado sin gesto); el guard decide con la posición real.
        })
        return
      }

      // hls.js SOLO se importa aquí, dentro del camino hlsjs: Safari nunca
      // pasa por esta rama, así que Safari nunca lo descarga.
      import('hls.js').then(({ default: Hls }) => {
        // destruido: el componente se desmontó. zanjado: ESTE intento ya se
        // resolvió o rechazó (aunque guardActual todavía no se haya
        // reasignado al siguiente). guardActual !== guard: el failover ya
        // avanzó a otro intento. Cualquiera de los tres significa que este
        // import ya no puede tocar hlsActual ni el <video>.
        if (destruido || zanjado || guardActual !== guard) return
        if (!Hls.isSupported()) {
          guard.alError('hls.js no soportado en este navegador')
          return
        }
        const hls = new Hls()
        hlsActual = hls
        hls.on(Hls.Events.ERROR, (_evt, data) => {
          // Solo el fatal describe el DESENLACE del intento — hls.js emite
          // errores no-fatales constantemente en directos que se ven
          // perfectamente (bufferStalledError, fragParsingError…); guardarlos
          // ensuciaría la clasificación con ruido que el guard ya ignora.
          if (data.fatal) {
            infoUltimoError = { tipoHls: data.type, detallesHls: data.details, httpStatus: data.response?.code }
          }
          // El flag fatal decide: un no-fatal antes de arrancar ya NO mata el
          // intento (hls.js se recupera solo de fragLoadError/bufferStalled/
          // aborted). Antes se pasaban todos como fatales, en contra de lo que
          // dice el comentario de arriba, y eso tiraba canales sanos.
          guard.alError(`${data.type}:${data.details}`, !!data.fatal)
        })
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          video?.play().catch(() => {})
        })
        hls.on(Hls.Events.BUFFER_CODECS, (_evt, data) => {
          if (data.video || data.audiovideo) pistas.video = true
          if (data.audio) pistas.audio = true
        })
        // Segmentos que llegan y buffer que se anexa: la carga avanza aunque
        // el <video> todavía no emita nada.
        hls.on(Hls.Events.FRAG_LOADED, () => guard.alProgreso())
        hls.on(Hls.Events.BUFFER_APPENDED, () => guard.alProgreso())
        hls.loadSource(intento.url)
        hls.attachMedia(video!)
      })
    })
  }

  async function reproducir() {
    if (!video) return
    const miId = ++intentoId
    cargando = true
    mensajeError = null
    errorSinReintento = false
    numMirrorsProbados = 0

    const motor = motorForzado ?? motorDelNavegador(video)
    let intentos: Intento[]
    // Cuántos mirrors traía ESTE intento de reproducir(): 0 en el camino de
    // compatibilidad sin mirrors (más abajo). Se lee solo si el bucle acaba
    // agotando todos los intentos (ver el mensajeError final).
    let totalMirrors = 0

    try {
      const mirrors = await fuente.mirrors(canal.id)
      if (destruido || miId !== intentoId) return

      if (mirrors.length > 0) {
        // Los mirrors cuyo vídeo ningún navegador decodifica (sonda del
        // servidor) no se intentan: cada uno costaría el presupuesto entero
        // del guard para acabar en el mismo sitio.
        const reproducibles = mirrors.filter((m) => m.codecOk !== false)
        if (reproducibles.length === 0) {
          if (motorForzado === 'nativo') {
            // Igual que cuando el failover se agota en pleno cast: soltar la
            // ruta AirPlay y volver al motor local; la siguiente pasada de
            // reproducir() ya mostrará el mensaje de códec sin cast a medias.
            motorForzado = null
            estadoCast = 'idle'
            terminarRutaAirplay()
            mostrarAvisoCast(t('reproductor.cast.fallo'))
            reproducir()
            return
          }
          cargando = false
          errorSinReintento = true
          const codecs = mirrors.find((m) => m.codecs)?.codecs ?? ''
          mensajeError = codecs
            ? t('reproductor.error.codec', { codecs })
            : t('reproductor.error.codecGenerico')
          alDesenlace({ canalId: canal.id, resultado: 'fallo', motivo: 'codec', motor, via: 'ninguna', mirrorIndex: 0 })
          return
        }
        totalMirrors = reproducibles.length
        const proxyDisp = await fuente.proxyDisponible()
        if (destruido || miId !== intentoId) return
        intentos = planDeFailover(reproducibles, motor, proxyDisp)
      } else {
        // Sin mirrors (fuente vieja o canal sin entrada en /channels/streams):
        // compatibilidad con el destino único de siempre, como un solo mirror.
        const destinoUnico = await fuente.destino(canal.id)
        if (destruido || miId !== intentoId) return
        const proxyDisp = await fuente.proxyDisponible()
        if (destruido || miId !== intentoId) return
        const plan = planDeReproduccion({
          motor,
          url: destinoUnico.url,
          webOk: canal.webOk,
          proxyDisponible: proxyDisp,
        })
        if (plan.aviso === 'solo-app-o-safari') {
          cargando = false
          mensajeError = t('canal.soloApp')
          return
        }
        // viaProxy se deriva comparando la url del intento contra la
        // proxeada (fix final, hallazgo 2), no por posición (i > 0): un plan
        // SOLO-proxy (mixed-content o webOk=false, ver planDeReproduccion)
        // tiene su única url en i=0 y ES por proxy — "i > 0" la etiquetaba
        // como 'directo' en las stats. Misma derivación que planDeFailover
        // usa en failover.ts, para no duplicar la lógica de "¿esta url va
        // por proxy?" en dos sitios.
        intentos = plan.intentos.map((url) => ({
          url,
          viaProxy: url === urlProxy(destinoUnico.url),
          mirrorIndex: 0,
        }))
      }
    } catch (e) {
      if (destruido || miId !== intentoId) return
      cargando = false
      // Esto es un fallo al PEDIR los datos (mirrors/destino/proxyDisponible),
      // no un fallo al reproducir: "no arrancó" se reserva para cuando SÍ
      // hubo una lista de intentos y ninguno reprodujo (más abajo).
      mensajeError = mensajeDeClase(clasificarError(e))
      return
    }

    if (intentos.length === 0) {
      cargando = false
      mensajeError = t('canal.soloApp')
      return
    }

    // Se guardan las clases de TODOS los intentos, no solo la del último.
    // Mostrar la del último escondía la razón real: con AMC (720p) el primer
    // mirror estaba vivo pero servía segmentos de 4 s en más de 12 s
    // ('desconocido') y el segundo daba 404 ('caducado'), así que el usuario
    // leía «la dirección caducó» sobre un canal cuyo problema era la lentitud.
    // Ver claseConsensuada().
    const clasesVistas: ClaseFallo[] = []

    for (const intento of intentos) {
      if (destruido || miId !== intentoId) return
      limpiarIntento()
      alIntentar(intento.url)
      try {
        await intentar(intento, motor)
        return // confirmado
      } catch {
        // clasificarFallo lee la info que dejó ESTE intento (hls.js ERROR
        // fatal o video.error nativo); "timeout-de-carga" sin más señal cae
        // en 'desconocido', no en un motivo crudo que /stats no puede agrupar.
        const clase = clasificarFallo(infoUltimoError)
        clasesVistas.push(clase)
        alDesenlace({
          canalId: canal.id,
          resultado: 'fallo',
          motivo: clase,
          motor,
          via: via(intento),
          mirrorIndex: intento.mirrorIndex,
        })
        // se agota este intento; se prueba el siguiente mirror
      }
    }
    if (destruido || miId !== intentoId) return
    // El último intento del bucle NO se limpió al entrar en él (limpiarIntento
    // se llama al EMPEZAR cada intento, no al fallar el último): sin esto, su
    // guard queda vivo con arrancado===false, hls sigue reintentando fetches
    // en segundo plano, y timeupdate/error del <video> siguen atados a un
    // guard muerto. Si ese intento residual llegara a avanzar, dispararía
    // alConfirmar() y pisaría el error que se muestra a continuación.
    limpiarIntento()
    if (motorForzado === 'nativo') {
      // El motor nativo falló — pero hls.js (motorDelNavegador de verdad)
      // probablemente SÍ reproduce este canal, así que no se muestra la
      // tarjeta de error a pantalla completa: solo un aviso transitorio,
      // y se reanuda la reproducción local normal.
      if (claseConsensuada(clasesVistas) === 'formato') marcarFalloFormato(canal.id)
      motorForzado = null
      estadoCast = 'idle'
      // Fix de revisión final: revertir el motor NO soltaba la ruta AirPlay —
      // el <video> seguía remitiendo al TV (que no puede con MSE), así que el
      // resultado real era pantalla negra en el TV Y en local mientras la app
      // creía haber vuelto a la normalidad. Ver terminarRutaAirplay().
      terminarRutaAirplay()
      mostrarAvisoCast(t('reproductor.cast.fallo'))
      reproducir()
      return
    }
    cargando = false
    mensajeError = t(claveDeClase(claseConsensuada(clasesVistas)))
    // Mirrors PROBADOS, no "disponibles": al llegar aquí el bucle los agotó
    // todos, así que no queda ninguno por intentar. Decir "hay N con mejor
    // salud" era falso —era el total, sin filtrar por salud— y prometía una
    // opción que no existe.
    numMirrorsProbados = totalMirrors
  }

  /** CTA "Reintentar": reanuda EXACTAMENTE el mismo mecanismo
   *  que el failover automático (P0.5) usó para llegar hasta aquí — la misma
   *  reproducir(), no una ruta de reintento paralela. Vuelve a pedir
   *  mirrors() (la salud pudo cambiar desde el último intento) y recorre la
   *  cadena de nuevo desde el principio; limpiarIntento() es el mismo
   *  defensivo que ya usa el $effect de cambio de canal, aunque el bucle ya
   *  se limpió a sí mismo al agotarse. */
  function reintentar() {
    limpiarIntento()
    reproducir()
  }

  // Reacciona a cambiar de canal (flechas ← →) igual que a la apertura
  // inicial: canal.id es la clave de "hay que reconectar" (el destino/los
  // mirrors los pide el propio Reproductor, ya no llegan por prop).
  let idCanalPrevio: string | null = null
  $effect(() => {
    void canal.id
    if (!video) return
    // Cambio REAL de canal con la CTA de sonido todavía visible: elegir un
    // canal ES un gesto del usuario (spec §5, «primer gesto → sonido»), así
    // que el silencio de entrada se levanta solo. untrack: leer el estado de
    // la CTA aquí no debe suscribir este efecto a él (activar el sonido con
    // la CTA re-dispararía limpiarIntento()+reproducir() sin venir a cuento).
    untrack(() => {
      if (idCanalPrevio !== null && idCanalPrevio !== canal.id) {
        if (mostrarActivarSonido) activarSonido()
        // Un aviso de cast pertenece al canal que lo produjo: abrir OTRO canal
        // es una acción nueva y ajena, así que el aviso viejo se va con él en
        // vez de esperar a su temporizador (fix de revisión final).
        limpiarAvisoCast()
      }
      idCanalPrevio = canal.id
    })
    limpiarIntento()
    reproducir()
  })

  function alternarSilencio() {
    // Guarda dentro de la función, no solo en el atajo de teclado (fix de
    // revisión final): el botón 🔊/🔇 del overlay seguía siendo clicable con
    // el ratón durante una emisión, y el MISMO <video> alimenta al TV (spec
    // §3/§5) — silenciar aquí muy probablemente silencia el televisor. Aquí
    // cubre a TODOS los llamadores (botón y tecla 'm') de una vez; el botón
    // además va `disabled` para que se vea que no aplica.
    if (estadoCast !== 'idle') return
    silenciado = !silenciado
    if (video) video.muted = silenciado
    // Activar el sonido a mano cuenta como "primer gesto" (spec §5): la CTA
    // de entrada ya no pinta nada que ofrecer.
    if (!silenciado) mostrarActivarSonido = false
  }

  // Sobre contenedorRaiz (el div role="dialog" que envuelve TANTO el
  // <video> como el overlay y la barra de controles), no sobre el <video>:
  // pedir fullscreen solo del <video> saca al overlay/controles de la
  // presentación en pantalla completa (el navegador solo muestra el árbol
  // del elemento fullscreen-eado). El estado real se refleja vía
  // fullscreenchange (alCambioFullscreen), no aquí — esta función solo pide
  // el cambio, no lo asume.
  function alternarPantallaCompleta() {
    // Misma guarda "para todos los llamadores" que alternarSilencio(): durante
    // una emisión no hay vídeo local que expandir (lo cubre el panel
    // «Emitiendo…»), y el botón ⛶ era clicable con el ratón aunque la tecla
    // 'f' ya estuviera bloqueada. Esc sigue saliendo de pantalla completa si
    // se entró antes de empezar a emitir (ver alTeclado).
    if (estadoCast !== 'idle') return
    if (document.fullscreenElement) {
      document.exitFullscreen()
    } else if (soportaFullscreen) {
      contenedorRaiz?.requestFullscreen()
    }
  }

  function alCambioFullscreen() {
    estaEnPantallaCompleta = document.fullscreenElement === contenedorRaiz
  }

  // Picture-in-Picture: botón feature-detectado (soportaPiP, arriba) — si no
  // hay soporte, el botón ni se renderiza. requestPictureInPicture() puede
  // rechazar sin un gesto de usuario reciente en algunos navegadores; es
  // best-effort, se registra el error y no se propaga (no hay nada que
  // "fallar" de cara al reproductor en sí).
  function alternarPiP() {
    if (!video) return
    if (document.pictureInPictureElement) {
      document.exitPictureInPicture().catch((e: unknown) => {
        console.error('No se pudo salir de Picture-in-Picture', e)
      })
    } else {
      video.requestPictureInPicture().catch((e: unknown) => {
        console.error('No se pudo activar Picture-in-Picture', e)
      })
    }
  }

  function alEntrarPiP() {
    estaEnPiP = true
  }

  function alSalirPiP() {
    estaEnPiP = false
  }

  /** Arranca la preparación del motor nativo, SIN esperar a que la ruta
   *  AirPlay se active. La llaman tanto abrirSelectorAirplay() (el camino
   *  normal) como iniciarCast() (red de seguridad reactiva, ver más abajo). */
  function prepararCastNativo() {
    limpiarAvisoCast()
    motorForzado = 'nativo'
    estadoCast = 'conectando'
    limpiarIntento()
    reproducir()
  }

  function abrirSelectorAirplay() {
    if (!video) return
    // Empezar a preparar el motor nativo AQUÍ, antes de abrir el selector —
    // no al reaccionar al evento de ruta, que es como estaba hasta el
    // 2026-09-04. Confirmado con trace real contra hardware (5213 ciclos en
    // 23 s sin llegar NUNCA a 'emitiendo'): AirPlay comprueba el <video> a
    // 0-2 ms de que la ruta se active, y si limpiarIntento() lo había dejado
    // sin src (porque reproducir() aún no había resuelto la URL de forma
    // asíncrona) simplemente soltaba la ruta al instante. El spike original
    // que SÍ funcionó tenía el vídeo YA reproduciendo nativo antes de tocar
    // el selector — esto reproduce esa misma condición: el hueco entre este
    // clic y que el usuario elija de verdad un dispositivo en el selector
    // nativo (siempre cientos de ms, hay un humano de por medio) le da
    // tiempo de sobra al fetch asíncrono de reproducir().
    if (estadoCast === 'idle' && !noCasteaPorFormato(canal.id)) {
      prepararCastNativo()
      armarTimeoutSelectorCancelado()
    }
    // @ts-expect-error API solo de WebKit
    video.webkitShowPlaybackTargetPicker()
  }

  /** Red de seguridad para el selector cancelado (ver el comentario de
   *  rutaConfirmadaEnEsteIntento más arriba): si pasa el plazo sin que
   *  alCambioRutaAirplay() confirme una ruta de verdad, se asume cancelado
   *  y se revierte a reproducción local — sin tocar disableRemotePlayback,
   *  porque la ruta nunca llegó a estar activa (nada que soltar). */
  function armarTimeoutSelectorCancelado() {
    rutaConfirmadaEnEsteIntento = false
    if (temporizadorSelectorCancelado !== undefined) clearTimeout(temporizadorSelectorCancelado)
    temporizadorSelectorCancelado = setTimeout(() => {
      temporizadorSelectorCancelado = undefined
      if (!rutaConfirmadaEnEsteIntento && motorForzado === 'nativo') {
        motorForzado = null
        estadoCast = 'idle'
        limpiarIntento()
        reproducir()
      }
    }, TIMEOUT_SELECTOR_CANCELADO_MS)
  }

  /** Corta la sesión de reproducción remota (AirPlay) EN EL NAVEGADOR, no
   *  solo en el estado de la app. Poner disableRemotePlayback a true en un
   *  <video> que está remitiendo termina esa sesión remota (Remote Playback
   *  API); volverlo a false inmediatamente es obligatorio, porque dejarlo en
   *  true no desconecta "esta" sesión sino que suprime la función entera —
   *  el propio selector nativo (webkitShowPlaybackTargetPicker) dejaría de
   *  ofrecerse y el usuario no podría volver a emitir nunca.
   *
   *  Bug real en hardware (2026-09-04, confirmado con trace, no solo
   *  sospecha): alternar disableRemotePlayback CUANDO LA RUTA YA NO ESTABA
   *  inalámbrica (p.ej. reaccionando al propio evento que acaba de
   *  reportarla como perdida) hacía que WebKit la volviera a ofrecer casi al
   *  instante — bucle sin salida (5213 ciclos en 23 s, "Emisión terminada"
   *  parpadeando, el 429 al origin de antes). Por eso solo se toca
   *  disableRemotePlayback si la ruta SIGUE activa en este momento: soltar
   *  algo que ya no se tiene no libera nada, solo reactiva la detección. */
  function terminarRutaAirplay() {
    if (!video) return
    // @ts-expect-error API solo de WebKit
    if (!video.webkitCurrentPlaybackTargetIsWireless) return
    video.disableRemotePlayback = true
    video.disableRemotePlayback = false
  }

  function iniciarCast() {
    // Red de seguridad reactiva: en el camino normal, abrirSelectorAirplay()
    // YA preparó el motor nativo antes de que este evento llegara, así que
    // estadoCast ya no es 'idle' aquí y esta función no tiene nada que
    // hacer. Esta rama cubre que la ruta se active sin pasar por nuestro
    // botón (selector de sistema fuera de la app) y sigue evitando la
    // reentrancia si ya hay una sesión en marcha.
    if (estadoCast !== 'idle') return
    if (noCasteaPorFormato(canal.id)) {
      // La ruta AirPlay YA está activa (por eso llegó el evento): si solo se
      // avisa y se vuelve, el TV se queda conectado a un <video> que va a
      // seguir en hls.js/MSE — o sea, negro. Se suelta la ruta.
      terminarRutaAirplay()
      mostrarAvisoCast(t('reproductor.cast.noDisponible'))
      return
    }
    prepararCastNativo()
  }

  function pararCast() {
    // Higiene: si el usuario para ANTES de que llegara la confirmación real
    // de ruta (posible — 'emitiendo' puede llegar solo con reproducción
    // LOCAL confirmada, antes de que alCambioRutaAirplay() la confirme de
    // verdad, ver el trace del fix), no dejar el timeout colgado.
    if (temporizadorSelectorCancelado !== undefined) {
      clearTimeout(temporizadorSelectorCancelado)
      temporizadorSelectorCancelado = undefined
    }
    motorForzado = null
    estadoCast = 'idle'
    terminarRutaAirplay()
    mostrarAvisoCast(t('reproductor.cast.terminada'))
    limpiarIntento()
    reproducir()
  }

  // Traduce el evento REAL de WebKit (se dispara cuando el usuario elige o
  // suelta una ruta en el picker nativo que abre abrirSelectorAirplay(), y
  // también si el TV se apaga a mitad de emisión) al arranque/parada del
  // cast. video.webkitCurrentPlaybackTargetIsWireless no está en el tipo
  // HTMLVideoElement — API solo de WebKit, igual que
  // webkitShowPlaybackTargetPicker más arriba.
  function alCambioRutaAirplay() {
    // @ts-expect-error API solo de WebKit
    const activa = Boolean(video?.webkitCurrentPlaybackTargetIsWireless)
    if (activa) {
      // Confirmación REAL de ruta: desarma el timeout del selector
      // cancelado (armarTimeoutSelectorCancelado) — esta señal es
      // justo la que ese timeout esperaba y nunca llegó en un cancelado.
      rutaConfirmadaEnEsteIntento = true
      if (temporizadorSelectorCancelado !== undefined) {
        clearTimeout(temporizadorSelectorCancelado)
        temporizadorSelectorCancelado = undefined
      }
      iniciarCast()
    } else if (estadoCast !== 'idle') {
      pararCast()
    }
  }

  function alTeclado(e: KeyboardEvent) {
    // Con otra vista encima del escenario (ver-todo, vistas-hash, paleta) el
    // teclado global del reproductor se apaga entero: es esa vista quien
    // gobierna el teclado, no un panel que ni siquiera se ve.
    if (!activo) return
    // El panel COEXISTE con controles ajenos (el buscador de la lateral, sus
    // filas, la cabecera): con el foco en uno de ellos, el espacio/las
    // flechas son suyos — misma guarda compartida que el surf y ⌘K
    // (lib/surf.ts).
    if (esObjetivoInteractivo(e.target)) return
    // Cualquier tecla reprograma el auto-ocultar del overlay (mismo trato que
    // el mousemove del contenedor) — teclear para navegar/pausar no debe
    // dejar los controles desaparecer a mitad de gesto.
    mostrar()
    switch (e.key) {
      case ' ':
        e.preventDefault()
        if (video) {
          if (video.paused) video.play().catch(() => {})
          else video.pause()
        }
        break
      case 'Escape':
        // Precedencia (Tarea 2, P0.8): si estamos en pantalla completa,
        // Escape SOLO sale de ella — sin este check, salir de fullscreen con
        // Escape también cerraba el reproductor entero (el navegador ya sale
        // de fullscreen por su cuenta en algunos casos sin llegar a disparar
        // este handler; aquí se cubre el caso en que sí llega).
        if (document.fullscreenElement) {
          document.exitFullscreen()
          return
        }
        // Panel (spec §4): sin pantalla completa que abandonar, Esc no hace
        // nada — ya no existe un modal que cerrar.
        break
      // La guarda de cast de ambas ('idle' o nada) vive DENTRO de las dos
      // funciones desde el fix de revisión final — así cubre también sus
      // botones del overlay, que antes seguían clicables con el ratón. Aquí
      // se llaman sin condición a propósito: una sola fuente de verdad.
      case 'f':
        alternarPantallaCompleta()
        break
      case 'm':
        alternarSilencio()
        break
      case 'ArrowLeft':
        alAnterior?.()
        break
      case 'ArrowRight':
        alSiguiente?.()
        break
    }
  }

  document.addEventListener('visibilitychange', onVisibilidad)

  onDestroy(() => {
    destruido = true
    document.removeEventListener('visibilitychange', onVisibilidad)
    limpiarIntento()
    if (temporizadorOverlay !== undefined) clearTimeout(temporizadorOverlay)
    if (temporizadorAvisoCast !== undefined) clearTimeout(temporizadorAvisoCast)
    if (temporizadorSelectorCancelado !== undefined) clearTimeout(temporizadorSelectorCancelado)

    // Un reproductor cerrado no debe dejar un PiP flotante de un <video> que
    // ya se está desmontando — se cierra explícitamente, no se confía en que
    // el navegador lo haga solo al quitar el nodo del DOM.
    if (estaEnPiP && typeof document.exitPictureInPicture === 'function') {
      document.exitPictureInPicture().catch(() => {})
    }
  })
</script>

<svelte:window onkeydown={alTeclado} />
<!-- fullscreenchange es un evento de document, no de window — svelte:document
     es el mismo mecanismo que svelte:window de arriba, para el objeto global
     que sí lo dispara. -->
<svelte:document onfullscreenchange={alCambioFullscreen} />

<!-- Panel persistente (spec §4): una region con el nombre del canal —
     coexiste en el árbol de accesibilidad con la lateral del escenario, sin
     role=dialog ni aria-modal que finjan un modal que ya no existe. -->
<div class="reproductor" role="region" aria-label={canal.nombre} bind:this={contenedorRaiz}>
  <!-- onmousemove aquí, no en el <div role="dialog"> de fuera: mover el
       ratón sobre el vídeo es lo que reprograma el auto-ocultar del overlay
       (los botones de la barra fija de abajo ya se auto-muestran solos, al
       ser siempre visibles). Puesto en el div del diálogo, el linter de a11y
       exige tabindex por convertirlo en "interactivo" — aquí, sobre un div
       sin rol, no aplica. -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="lienzo" onmousemove={mostrar}>
    <!-- svelte-ignore a11y_media_has_caption -->
    <!-- onenterpictureinpicture/onleavepictureinpicture no están en los tipos
         de atributos de <video> de Svelte (igual que x-webkit-airplay arriba)
         — el spread evita el chequeo de propiedades conocidas del literal. -->
    <video
      bind:this={video}
      {...{ 'x-webkit-airplay': 'allow', onenterpictureinpicture: alEntrarPiP, onleavepictureinpicture: alSalirPiP }}
      playsinline
      muted={silenciado}
    ></video>

    <!-- El aviso de cast va PRIMERO en la cadena a propósito (fix de revisión
         final): antes solo vivía en la región sr-only, así que un usuario que
         ve no se enteraba de que su emisión había fallado o terminado. Y no
         puede ir al final: la ruta de agotamiento del cast llama a
         reproducir() en la misma pasada (cargando pasa a true de inmediato),
         así que la rama `cargando` ganaría siempre y el aviso no llegaría a
         pintarse nunca. No bloquea la UI normal más allá de AVISO_CAST_MS: se
         auto-borra (mostrarAvisoCast) y también al cambiar de canal. -->
    {#if avisoCast}
      <div class="estado cast aviso">
        <p class="mensaje">{avisoCast}</p>
      </div>
    {:else if cargando}
      <p class="estado">{t('reproductor.cargando')}</p>
    {:else if mensajeError}
      <div class="estado error">
        <p class="mensaje">{mensajeError}</p>
        {#if numMirrorsProbados > 0}
          <p class="mirrors">{t('reproductor.error.mirrorsProbados', { n: numMirrorsProbados })}</p>
        {/if}
        {#if !errorSinReintento}
          <button type="button" class="probar-mirror" onclick={reintentar}>
            {t('reproductor.error.reintentar')}
          </button>
        {/if}
      </div>
    {:else if estadoCast === 'emitiendo'}
      <!-- estadoCast === 'conectando' ya se ve como "cargando" arriba (sigue
           en true hasta que PlaybackGuard.alConfirmar() dispara) — sin
           tercer texto de carga redundante para ese estado. -->
      <div class="estado cast">
        <p class="mensaje">{t('reproductor.cast.emitiendo', { canal: canal.nombre })}</p>
        <button type="button" class="parar-cast" onclick={pararCast}>
          {t('reproductor.cast.parar')}
        </button>
      </div>
    {/if}

    <!-- Regiones aria-live PERSISTENTES (fix round 1, Hallazgo 1): los <p>
         de arriba son SOLO visuales — se montan/desmontan con {#if}/
         {:else if}, y un lector de pantalla que solo anuncia mutaciones de
         texto dentro de una región ya presente (no la inserción del nodo
         entero) no anunciaría nada al abrirse. Estas dos existen SIEMPRE
         mientras el reproductor existe (vacías cuando no aplican); es su
         textContent el que cambia. -->
    <!-- Cast (spec 2026-09-03): reusa estas DOS regiones ya existentes, no
         añade una tercera — invariante duro del proyecto (CLAUDE.md). -->
    <p class="sr-only" aria-live="polite" aria-atomic="true">
      {cargando ? t('reproductor.cargando') : estadoCast === 'emitiendo' ? t('reproductor.cast.emitiendo', { canal: canal.nombre }) : ''}
    </p>
    <p class="sr-only" role="alert" aria-live="assertive" aria-atomic="true">{mensajeError ?? avisoCast ?? ''}</p>

    <!-- CTA de sonido (spec §5): la entrada auto-reproduce muted; esta es la
         affordance «bien visible» para activar el sonido. Desaparece con el
         primer gesto (pulsar aquí, Silenciar, o elegir otro canal). Sin
         animación propia: nada nuevo que prefers-reduced-motion deba anular. -->
    {#if mostrarActivarSonido && !mensajeError}
      <button type="button" class="activar-sonido" onclick={activarSonido}>
        <span aria-hidden="true">🔊</span>
        {t('reproductor.activarSonido')}
      </button>
    {/if}

    <!-- Overlay 1b (Tarea 1, P0.8): barra de controles SOBRE el vídeo, con
         gradiente inferior. Sustituye al título y al botón Silenciar que
         antes vivían en la barra fija de abajo (ahora ya sin ellos) —
         Pantalla completa/AirPlay/Cerrar siguen en esa barra; la Tarea 2 los
         de Pantalla completa/PiP se unen aquí (contenedor .overlay-controles,
         mismo focus-trap del diálogo). -->
    <div class="overlay" class:oculto={ocultarOverlay} bind:this={overlayEl} onfocusin={alRecibirFocoOverlay}>
      <div class="overlay-arriba">
        <div class="overlay-arriba-fila">
          <span class="insignia-vivo">
            <!-- Punto ROJO (--signal-error), no ámbar: excepción deliberada a
                 la regla "ámbar = activo/foco/señal" — aquí el rojo del
                 mockup 1b marca "en directo" (como en un plató de TV), no un
                 error de reproducción. -->
            <i class="punto-vivo" aria-hidden="true"></i>
            {t('reproductor.envivo')}
          </span>
          {#if metaLinea}
            <span class="overlay-meta">{metaLinea}</span>
          {/if}
        </div>
        <!-- Guía ahora/después (Tarea 9, P2 EPG): línea propia DEBAJO de la
             fila insignia+meta, dentro del mismo overlay-arriba — así no
             toca el justify-content:space-between de .overlay (que solo
             espera dos hijos, arriba/abajo) ni empuja overlay-abajo. Texto
             plano; SIN aria-live nuevo (las regiones live existentes son
             para cargando/error, no para esto); sin animación, así que no
             hay nada que prefers-reduced-motion deba anular aquí. -->
        {#if epgLinea}
          <p class="overlay-epg">{epgLinea}</p>
        {/if}
      </div>
      <div class="overlay-abajo">
        <span class="overlay-nombre">{canal.nombre}</span>
        <div class="overlay-controles">
          <!-- disabled durante una emisión (fix de revisión final): el mismo
               <video> alimenta al TV, así que silenciar aquí lo silencia
               allí. La guarda de verdad vive en alternarSilencio() (cubre
               teclado y cualquier llamador futuro); `disabled` es la señal
               VISIBLE de que ahora mismo no aplica — un botón que se puede
               pulsar y no hace nada es peor que uno apagado. -->
          <button
            type="button"
            class="silenciar"
            class:activo={silenciado}
            onclick={alternarSilencio}
            disabled={estadoCast !== 'idle'}
            aria-pressed={silenciado}
            aria-label={t('reproductor.silenciar')}
          >
            {silenciado ? '🔇' : '🔊'}
          </button>
          <button
            type="button"
            class="favorito"
            class:activo={esFavorito}
            onclick={() => favoritos.alternar(canal.id)}
            aria-pressed={esFavorito}
            aria-label={esFavorito ? t('canal.favorito.quitar') : t('canal.favorito.anadir')}
          >★</button>
          <!-- Tarea 2 (P0.8): Pantalla completa se traslada aquí desde la
               barra fija de abajo — sobre el CONTENEDOR del diálogo (ver
               alternarPantallaCompleta), para que ella misma y el resto del
               overlay sigan visibles en pantalla completa. PiP se une junto a
               ella, feature-detectado (soportaPiP): si el navegador no lo
               soporta (Firefox/iOS difieren), el botón ni se renderiza. -->
          <button
            type="button"
            class="pantalla-completa"
            class:activo={estaEnPantallaCompleta}
            onclick={alternarPantallaCompleta}
            disabled={estadoCast !== 'idle'}
            aria-pressed={estaEnPantallaCompleta}
            aria-label={estaEnPantallaCompleta ? t('reproductor.pantallaCompleta.salir') : t('reproductor.pantallaCompleta.entrar')}
          >⛶</button>
          {#if soportaPiP}
            <button
              type="button"
              class="pip"
              class:activo={estaEnPiP}
              onclick={alternarPiP}
              aria-pressed={estaEnPiP}
              aria-label={estaEnPiP ? t('reproductor.pip.desactivar') : t('reproductor.pip.activar')}
            >🗗</button>
          {/if}
          {#if soportaAirplay}
            <!-- La barra inferior del modal se fue con el modal: AirPlay vive
                 aquí, junto a PiP (decisión 4 del plan reproductor-primero).
                 class:activo reusa la regla de marca ya existente (ámbar =
                 señal-viva/activo, .overlay-controles button.activo más
                 abajo) — sin CSS nuevo. -->
            <button
              type="button"
              class="airplay"
              class:activo={estadoCast !== 'idle'}
              onclick={abrirSelectorAirplay}
              aria-pressed={estadoCast !== 'idle'}
              aria-label={t('reproductor.airplay')}
            >📺</button>
          {/if}
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  /* Panel persistente del escenario (spec §3/§4): 16:9 dentro de su columna;
     en pantalla completa manda el navegador y el aspecto se libera. Sin
     animación de entrada: el panel persiste, no «entra». */
  .reproductor {
    background: var(--graphite-900);
    display: flex;
    flex-direction: column;
    position: relative;
    width: 100%;
    aspect-ratio: 16 / 9;
    border-radius: var(--radius-md);
    overflow: hidden;
  }
  .reproductor:fullscreen {
    aspect-ratio: auto;
    border-radius: 0;
    height: 100%;
  }
  /* CTA de sonido (spec §5): única acción primaria sobre el vídeo durante la
     entrada silenciosa — mismo relleno ámbar que .probar-mirror (familia de
     estados con una acción concreta que ofrecer). */
  .activar-sonido {
    position: absolute;
    bottom: 18%;
    left: 50%;
    transform: translateX(-50%);
    /* Por ENCIMA de .overlay (hermano posterior en el DOM, también absoluto
       con inset:0): sin esto, el overlay interceptaba el puntero y la CTA
       era visible pero inclicable — bug real cazado por el e2e del escenario
       (Playwright: «.overlay intercepts pointer events»). */
    z-index: 1;
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-4);
    color: var(--amber-500);
    cursor: pointer;
    font: inherit;
  }
  .activar-sonido:hover { background: var(--tint-amber-line); }
  .lienzo {
    position: relative;
    flex: 1;
    display: grid;
    place-items: center;
  }
  video { width: 100%; height: 100%; object-fit: contain; background: #000; }
  .estado {
    position: absolute;
    /* Por ENCIMA de .overlay, por la MISMA razón que .activar-sonido más
       abajo: el overlay es un hermano POSTERIOR en el DOM, también absoluto
       con inset:0 y pointer-events auto, así que se pinta encima y se come el
       puntero. Sin esto, el botón «Reintentar» de la tarjeta de error era
       visible pero inclicable (reportado por el dueño el 2026-09-04), y el
       scrim del overlay atenuaba la tarjeta al aparecer. opacity:0 en el
       overlay NO desactiva el hit-testing, así que no bastaba con que el
       overlay estuviera oculto: el clic se perdía igual.
       z-index 2 para quedar también por encima de .activar-sonido (1). */
    z-index: 2;
    color: var(--text-body);
    background: rgba(14, 19, 27, 0.85);
    padding: var(--space-3) var(--space-5);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    max-width: 32rem;
    text-align: center;
  }
  .estado.error .mensaje { color: var(--signal-error); margin: 0; }
  .estado.error .mirrors { color: var(--text-muted); margin: 0; }
  /* Sin color de error (var(--signal-error)) a propósito: fallar el cast no
     es un error de reproducción — hls.js sigue funcionando en local. Mismo
     margin:0 que .estado.error .mensaje, por la misma razón (el <p> suelto
     traería el margen por defecto del user-agent y desalinearía el gap del
     flex de .estado). */
  .estado.cast .mensaje { color: var(--text-body); margin: 0; }
  /* Ámbar = CTA primaria de un estado, mismo tratamiento "relleno" que
     .sugerida en Vacio.svelte (misma familia de estados con una acción
     concreta que sacar de un error). */
  .probar-mirror {
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-4);
    color: var(--amber-500);
    cursor: pointer;
    font: inherit;
  }
  .probar-mirror:hover { background: var(--tint-amber-line); }
  /* Mismo tratamiento visual que .probar-mirror, clase separada porque es
     una acción distinta (parar un cast, no reintentar un mirror) — mismo
     criterio que ya usa este fichero de una clase por acción en vez de
     compartir selector. */
  .parar-cast {
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-4);
    color: var(--amber-500);
    cursor: pointer;
    font: inherit;
  }
  .parar-cast:hover { background: var(--tint-amber-line); }

  /* Overlay 1b (Tarea 1, P0.8): barra de controles sobre el vídeo, con
     gradiente inferior — insignia arriba, nombre+controles abajo. La
     opacidad (no display:none) es lo que se anima: display:none no
     transiciona y además sacaría los botones del foco/tab de golpe, algo
     que el propio auto-ocultar ya evita devolviendo el foco con
     onfocusin/mostrar(). */
  .overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    padding: var(--space-4);
    background: var(--scrim-gradient);
    opacity: 1;
    transition: opacity var(--dur-base) var(--ease-out);
  }
  .overlay.oculto { opacity: 0; }
  @media (prefers-reduced-motion: reduce) {
    .overlay { transition: none; }
  }
  /* Columna: la fila insignia+meta (.overlay-arriba-fila) y, debajo, la
     línea de guía EPG (Tarea 9, P2) cuando la hay. align-items:flex-start
     para que ninguna de las dos se estire a lo ancho del overlay. */
  .overlay-arriba { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-1); }
  .overlay-arriba-fila { display: flex; align-items: center; gap: var(--space-3); }
  .insignia-vivo {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    font: var(--type-label);
    color: var(--text-strong);
  }
  /* Punto ROJO (--signal-error): ver el comentario junto al marcado — aquí
     marca "en directo" (mockup 1b), no una señal de error/salud. */
  .punto-vivo { width: 8px; height: 8px; border-radius: 50%; background: var(--signal-error); flex-shrink: 0; }
  .overlay-meta {
    font: var(--type-mono-label);
    letter-spacing: var(--tracking-mono);
    color: var(--text-muted);
  }
  /* Guía ahora/después (Tarea 9, P2 EPG): mismo tratamiento tipográfico que
     .overlay-meta (línea secundaria del overlay), truncada en vez de
     desbordar u obligar al overlay a crecer con un título largo. */
  .overlay-epg {
    margin: 0;
    max-width: 100%;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font: var(--type-mono-label);
    letter-spacing: var(--tracking-mono);
    color: var(--text-muted);
  }
  .overlay-abajo {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: var(--space-4);
  }
  .overlay-nombre {
    font: var(--type-h3);
    color: var(--text-strong);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .overlay-controles { display: flex; align-items: center; gap: var(--space-2); flex-shrink: 0; }
  .overlay-controles button {
    all: unset;
    cursor: pointer;
    color: var(--text-strong);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }
  .overlay-controles button:hover { background: rgba(255, 255, 255, 0.08); }
  /* `all: unset` borra también el aspecto apagado que el navegador da por su
     cuenta a un <button disabled>: sin esto, los controles bloqueados
     mientras se emite (silenciar / pantalla completa) se verían idénticos a
     los activos. Solo opacidad, cursor y el hover — ninguna propiedad de caja,
     así que la geometría de la barra no se mueve. Mismos valores que el
     :disabled que ya usan Fuentes.svelte/AnadirFuente.svelte, para que un
     control apagado se vea igual en toda la app. */
  .overlay-controles button:disabled { opacity: 0.6; cursor: default; }
  .overlay-controles button:disabled:hover { background: none; }
  /* Ámbar solo mientras el botón representa una señal activa (silenciado/favorito). */
  .overlay-controles button.activo { color: var(--amber-500); }
</style>
