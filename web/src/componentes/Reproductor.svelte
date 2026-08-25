<script lang="ts">
  import { onDestroy } from 'svelte'
  import type { Canal, CatalogSource } from '../datos/catalogo'
  import { PlaybackGuard } from '../reproductor/guard'
  import { planDeReproduccion, motorDelNavegador, type Motor } from '../reproductor/plan'
  import { planDeFailover, type Intento, type DesenlaceReproduccion } from '../reproductor/failover'
  import { t } from '../i18n'

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
    guardActual?.alError(String(video?.error?.code ?? 'error-nativo'))
  }

  function via(intento: Intento): 'directo' | 'proxy' {
    return intento.viaProxy ? 'proxy' : 'directo'
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
      const inicio = performance.now()

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
          reject(new Error(mensaje))
        },
        alConfirmar: () => {
          confirmado = true
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
        video.play().catch(() => {
          // Un rechazo de play() no es necesariamente fatal (autoplay
          // bloqueado sin gesto); el guard decide con la posición real.
        })
        return
      }

      // hls.js SOLO se importa aquí, dentro del camino hlsjs: Safari nunca
      // pasa por esta rama, así que Safari nunca lo descarga.
      import('hls.js').then(({ default: Hls }) => {
        if (destruido || guardActual !== guard) return
        if (!Hls.isSupported()) {
          guard.alError('hls.js no soportado en este navegador')
          return
        }
        const hls = new Hls()
        hlsActual = hls
        hls.on(Hls.Events.ERROR, (_evt, data) => {
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
    } catch {
      if (destruido || miId !== intentoId) return
      cargando = false
      mensajeError = t('reproductor.error.noArranco')
      return
    }

    if (intentos.length === 0) {
      cargando = false
      mensajeError = t('canal.soloApp')
      return
    }

    for (const intento of intentos) {
      if (destruido || miId !== intentoId) return
      limpiarIntento()
      alIntentar(intento.url)
      try {
        await intentar(intento, motor)
        return // confirmado
      } catch (e) {
        alDesenlace({
          canalId: canal.id,
          resultado: 'fallo',
          motivo: e instanceof Error ? e.message : String(e),
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
    mensajeError = t('reproductor.error.noArranco')
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
    }
  }

  onDestroy(() => {
    destruido = true
    limpiarIntento()
  })
</script>

<svelte:window onkeydown={alTeclado} />

<div class="reproductor" role="dialog" aria-modal="true" aria-label={canal.nombre}>
  <div class="lienzo">
    <!-- svelte-ignore a11y_media_has_caption -->
    <video bind:this={video} {...{ 'x-webkit-airplay': 'allow' }} playsinline muted={silenciado}></video>

    {#if cargando}
      <p class="estado">{t('reproductor.cargando')}</p>
    {:else if mensajeError}
      <p class="estado error">{mensajeError}</p>
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
    <button type="button" class="cerrar" onclick={alCerrar} aria-label={t('reproductor.cerrar')}>✕</button>
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
