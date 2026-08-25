<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte'
  import type { Canal, CatalogSource } from '../datos/catalogo'
  import { PlaybackGuard } from '../reproductor/guard'
  import { planDeReproduccion, motorDelNavegador, type Motor } from '../reproductor/plan'
  import { planDeFailover, type Intento, type DesenlaceReproduccion } from '../reproductor/failover'
  import { clasificarError, type ClaseError } from '../estado/salud'
  import { clasificarFallo, type ClaseFallo, type InfoFallo } from '../reproductor/diagnostico'
  import { t } from '../i18n'
  import type { ClaveMensaje } from '../i18n/es'

  // alAnterior/alSiguiente son opcionales: App los da cuando hay una lista de
  // canales de la que moverse (flechas ← →). fuente es el CatalogSource: el
  // Reproductor ya no recibe una URL resuelta, pide sus propios mirrors
  // (Tarea 5) para poder recorrer planDeFailover. alDesenlace y alIntentar son
  // opcionales — no-op en producción salvo que la Tarea 12/el test los den.
  let {
    canal,
    fuente,
    alCerrar,
    alAnterior,
    alSiguiente,
    alDesenlace = () => {},
    alIntentar = () => {},
  }: {
    canal: Canal
    fuente: CatalogSource
    alCerrar: () => void
    alAnterior?: () => void
    alSiguiente?: () => void
    alDesenlace?: (o: DesenlaceReproduccion) => void
    alIntentar?: (url: string) => void
  } = $props()

  let video: HTMLVideoElement | undefined = $state()
  let cargando = $state(true)
  let mensajeError = $state<string | null>(null)
  let silenciado = $state(false)

  // Foco del diálogo modal (Tarea 18, orden de foco): este componente se
  // monta FUERA de <main> (ver App.svelte), como el único overlay de pantalla
  // completa — App marca <main>/<footer> como inert mientras está abierto,
  // así que aquí basta con (1) llevar el foco DENTRO al montar, y (2)
  // devolverlo a quien lo abrió al desmontar. Sin esto, abrir el reproductor
  // dejaba el foco donde estaba (la tarjeta de detrás, ahora inert) o lo
  // perdía en <body> — ninguna de las dos deja a un usuario de teclado/lector
  // de pantalla saber dónde está.
  let contenedorDialogo: HTMLDivElement | undefined = $state()
  let botonCerrar: HTMLButtonElement | undefined = $state()
  let elementoPrevio: HTMLElement | null = null

  // AirPlay de Safari: es una llamada y un evento, sin capa nativa. Si el
  // navegador no lo expone, el botón no existe. La app de macOS es la que
  // hace sesiones de emisión de verdad; aquí solo se abre el selector del
  // sistema.
  const soportaAirplay = typeof window !== 'undefined' && 'WebKitPlaybackTargetAvailabilityEvent' in window

  let guardActual: PlaybackGuard | undefined
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
      video.removeAttribute('src')
      video.load()
    }
  }

  function onTimeUpdate() {
    guardActual?.alPosicion(video?.currentTime ?? 0)
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
      case 'caducado':
        return 'reproductor.error.caducado'
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

      video.addEventListener('timeupdate', onTimeUpdate)
      video.addEventListener('error', onVideoError)

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
          guard.alError(`${data.type}:${data.details}`)
        })
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          video?.play().catch(() => {})
        })
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

    const motor = motorDelNavegador(video)
    let intentos: Intento[]

    try {
      const mirrors = await fuente.mirrors(canal.id)
      if (destruido || miId !== intentoId) return

      if (mirrors.length > 0) {
        const proxyDisp = await fuente.proxyDisponible()
        if (destruido || miId !== intentoId) return
        intentos = planDeFailover(mirrors, motor, proxyDisp)
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
        intentos = plan.intentos.map((url, i) => ({ url, viaProxy: i > 0, mirrorIndex: 0 }))
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

    // La clase del ÚLTIMO intento agotado es la que se muestra: es el error
    // más informativo, el más cercano a "por qué el canal no llegó a verse"
    // (el failover ya cruzó los mirrors anteriores, así que sus fallos
    // importan menos que el del intento final).
    let ultimaClase: ClaseFallo = 'desconocido'

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
        ultimaClase = clasificarFallo(infoUltimoError)
        alDesenlace({
          canalId: canal.id,
          resultado: 'fallo',
          motivo: ultimaClase,
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
    cargando = false
    mensajeError = t(claveDeClase(ultimaClase))
  }

  // Reacciona a cambiar de canal (flechas ← →) igual que a la apertura
  // inicial: canal.id es la clave de "hay que reconectar" (el destino/los
  // mirrors los pide el propio Reproductor, ya no llegan por prop).
  $effect(() => {
    void canal.id
    if (!video) return
    limpiarIntento()
    reproducir()
  })

  function alternarSilencio() {
    silenciado = !silenciado
    if (video) video.muted = silenciado
  }

  function alternarPantallaCompleta() {
    if (document.fullscreenElement) document.exitFullscreen()
    else video?.requestFullscreen()
  }

  function abrirSelectorAirplay() {
    // @ts-expect-error API solo de WebKit
    video?.webkitShowPlaybackTargetPicker()
  }

  // Elementos focables DENTRO del diálogo, en orden de documento: con <main>/
  // <footer> inert (App.svelte) son los ÚNICOS focables de toda la página,
  // así que basta con ciclar entre ellos — no hace falta un centinela ni un
  // "focus sentinel" aparte.
  function elementosFocables(): HTMLElement[] {
    if (!contenedorDialogo) return []
    return Array.from(
      contenedorDialogo.querySelectorAll<HTMLElement>('button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])'),
    )
  }

  function alTeclado(e: KeyboardEvent) {
    switch (e.key) {
      case ' ':
        e.preventDefault()
        if (video) {
          if (video.paused) video.play().catch(() => {})
          else video.pause()
        }
        break
      case 'Escape':
        alCerrar()
        break
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
      case 'Tab': {
        // Atrapa el foco dentro del diálogo: sin esto, Tab desde el último
        // control saldría del documento (con el resto de la página inert,
        // ya no hay a dónde ir) en vez de volver al primero — un usuario de
        // teclado se quedaría sin poder volver a los controles sin Shift+Tab
        // de vuelta manualmente.
        const focables = elementosFocables()
        if (focables.length === 0) break
        const primero = focables[0]
        const ultimo = focables[focables.length - 1]
        if (e.shiftKey && document.activeElement === primero) {
          e.preventDefault()
          ultimo.focus()
        } else if (!e.shiftKey && document.activeElement === ultimo) {
          e.preventDefault()
          primero.focus()
        }
        break
      }
    }
  }

  onMount(() => {
    // Recuerda qué tenía el foco antes de abrir el reproductor (normalmente,
    // el botón "abrir" de la tarjeta pulsada) para devolvérselo al cerrar.
    elementoPrevio = document.activeElement instanceof HTMLElement ? document.activeElement : null
    // tick(): botonCerrar ya está bind:this-eado tras el primer render, pero
    // se espera igualmente por disciplina — no cuesta nada y evita depender
    // de que Svelte enlace bind:this antes de que onMount corra en todas las
    // versiones.
    tick().then(() => botonCerrar?.focus())
  })

  onDestroy(() => {
    destruido = true
    limpiarIntento()
    // Restaura el foco a quien abrió el reproductor — pero solo si ese nodo
    // sigue en el documento: la tarjeta que lo abrió pudo haber salido de la
    // ventana virtualizada (Tarea 17) mientras el reproductor estaba abierto,
    // y focus() sobre un nodo desconectado no hace nada ni avisa.
    if (elementoPrevio && document.body.contains(elementoPrevio)) elementoPrevio.focus()
  })
</script>

<svelte:window onkeydown={alTeclado} />

<div class="reproductor" role="dialog" aria-modal="true" aria-label={canal.nombre} bind:this={contenedorDialogo}>
  <div class="lienzo">
    <!-- svelte-ignore a11y_media_has_caption -->
    <video bind:this={video} {...{ 'x-webkit-airplay': 'allow' }} playsinline muted={silenciado}></video>

    {#if cargando}
      <p class="estado" aria-live="polite">{t('reproductor.cargando')}</p>
    {:else if mensajeError}
      <p class="estado error" role="alert" aria-live="assertive">{mensajeError}</p>
    {/if}
  </div>

  <div class="controles">
    <span class="titulo">{canal.nombre}</span>
    <button type="button" class:activo={silenciado} onclick={alternarSilencio} aria-pressed={silenciado} aria-label={t('reproductor.silenciar')}>
      {silenciado ? '🔇' : '🔊'}
    </button>
    <button type="button" onclick={alternarPantallaCompleta} aria-label={t('reproductor.pantallaCompleta')}>⛶</button>
    {#if soportaAirplay}
      <button type="button" onclick={abrirSelectorAirplay} aria-label="AirPlay">📺</button>
    {/if}
    <button type="button" class="cerrar" bind:this={botonCerrar} onclick={alCerrar} aria-label={t('reproductor.cerrar')}>✕</button>
  </div>
</div>

<style>
  .reproductor {
    position: fixed;
    inset: 0;
    z-index: var(--z-modal);
    background: var(--graphite-900);
    display: flex;
    flex-direction: column;
  }
  .lienzo {
    position: relative;
    flex: 1;
    display: grid;
    place-items: center;
  }
  video { width: 100%; height: 100%; object-fit: contain; background: #000; }
  .estado {
    position: absolute;
    color: var(--text-body);
    background: rgba(14, 19, 27, 0.85);
    padding: var(--space-3) var(--space-5);
    border-radius: var(--radius-md);
  }
  .estado.error { color: var(--signal-error); }
  .controles {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    background: var(--surface-card);
  }
  .titulo { color: var(--text-strong); font-weight: 600; margin-right: auto; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .controles button {
    all: unset;
    cursor: pointer;
    color: var(--text-muted);
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }
  .controles button:hover { color: var(--text-body); background: var(--surface-raised); }
  /* Ámbar solo mientras el botón representa una señal activa (silenciado). */
  .controles button.activo { color: var(--amber-500); }
  .controles button.cerrar:hover { color: var(--signal-error); }
</style>
