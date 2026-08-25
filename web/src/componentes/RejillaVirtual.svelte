<script lang="ts">
  import { onDestroy, tick } from 'svelte'
  import type { Canal } from '../datos/catalogo'
  import TarjetaCanal from './TarjetaCanal.svelte'

  // Virtualización mínima a medida (Tarea 17): con 8871 canales, pintar una
  // <TarjetaCanal> por cada uno son miles de nodos DOM. Se descartó una
  // librería de virtual-list (p.ej. @tanstack/svelte-virtual): el presupuesto
  // es 80 KB gzip de JS propio y la rejilla es una cuadrícula CSS uniforme
  // (auto-fill + aspect-ratio fijo en la tarjeta), así que el cálculo de
  // ventana es simple y no compensa arrastrar una dependencia para esto.
  //
  // No hay contenedor con overflow propio: es la página entera la que hace
  // scroll (ver App.svelte, <main> no fija alto). Por eso la ventana se mide
  // contra el viewport (getBoundingClientRect) y no contra un scrollTop
  // local.
  //
  // Teclado (Tarea 18): esta rejilla NO tenía navegación por flechas antes
  // de esta tarea (el único manejo de ArrowLeft/Right existente está en
  // Reproductor.svelte y es "canal anterior/siguiente" con el reproductor
  // abierto, sin relación con esto). El Tab nativo del navegador entre los
  // <button> de las tarjetas sigue funcionando igual que antes: es
  // estructuralmente imposible que el foco "salga" de la ventana visible
  // solo con Tab, porque las tarjetas fuera de la ventana no existen en el
  // DOM (son los espaciadores de abajo). Cuando la Tarea 18 añada
  // navegación por flechas, moverla a un índice fuera de
  // [filaInicio*columnas, filaFin*columnas) debe hacer scrollIntoView (o
  // ajustar el scroll) ANTES de enfocar, para que el nodo ya exista en el
  // DOM en el momento de pedirle el foco.
  let { canales, alAbrir, alPedirMas }: {
    canales: Canal[]
    alAbrir: (c: Canal) => void
    alPedirMas: () => void
  } = $props()

  // Deben coincidir con el grid CSS de abajo (minmax(160px,1fr), gap 12px):
  // es la misma cuenta de columnas que hace el navegador con auto-fill.
  const ANCHO_MIN = 160
  const GAP = 12
  // Alto de fila antes de medir una tarjeta real (primer pintado, y en
  // jsdom -sin layout real- durante los tests): evita dividir por 0.
  const ALTO_RESPALDO = 220
  // Filas de margen por encima/por debajo de lo estrictamente visible:
  // absorbe el error de estimar el alto de fila con una sola tarjeta medida
  // y evita huecos en blanco durante un scroll rápido.
  const FILAS_BUFFER = 3

  let contenedor: HTMLDivElement | undefined = $state()
  let centinela: HTMLDivElement | undefined = $state()
  let anchoContenedor = $state(0)
  let altoMedido = $state(0)

  let innerHeight = $state(0)
  let innerWidth = $state(0)

  const columnas = $derived(Math.max(1, Math.floor((anchoContenedor + GAP) / (ANCHO_MIN + GAP))))
  // Alto de fila con el separador incluido (el "paso" entre una fila y la
  // siguiente): altura real de la tarjeta medida, o el respaldo si aún no
  // se ha medido nada.
  const altoFila = $derived((altoMedido > 0 ? altoMedido : ALTO_RESPALDO) + GAP)
  const filas = $derived(canales.length === 0 ? 0 : Math.ceil(canales.length / columnas))

  let filaInicio = $state(0)
  let filaFin = $state(0)

  // Fix ronda 1 (controlador, rendimiento): con bind:scrollY el efecto de
  // la ventana leía getBoundingClientRect() de forma SÍNCRONA en cada
  // evento 'scroll' — un flick de trackpad dispara decenas de esos eventos
  // por segundo, y cada lectura de layout forzada así es "layout thrash".
  // Se sustituye por un listener manual pasivo + coalescencia por rAF: cada
  // 'scroll' solo agenda un requestAnimationFrame (si no hay ya uno
  // pendiente), y la única lectura de layout ocurre dentro de ese rAF —
  // como mucho una vez por frame, sea cual sea el nº de eventos de scroll
  // que hayan llegado en ese frame.
  let rafVentana = 0

  function recalcularVentana() {
    rafVentana = 0
    if (!contenedor) return
    const top = contenedor.getBoundingClientRect().top
    const inicioPx = Math.max(0, -top - FILAS_BUFFER * altoFila)
    const finPx = -top + innerHeight + FILAS_BUFFER * altoFila
    filaInicio = Math.min(filas, Math.max(0, Math.floor(inicioPx / altoFila)))
    filaFin = Math.max(filaInicio, Math.min(filas, Math.ceil(finPx / altoFila)))
  }

  function agendarRecalculo() {
    if (rafVentana) return // ya hay un rAF pendiente: no agendar otro
    rafVentana = requestAnimationFrame(recalcularVentana)
  }

  onDestroy(() => {
    if (rafVentana) cancelAnimationFrame(rafVentana)
  })

  // Listener pasivo: no bloquea el scroll del navegador a la espera de que
  // termine el handler (no hay preventDefault posible ni falta que hace).
  $effect(() => {
    window.addEventListener('scroll', agendarRecalculo, { passive: true })
    return () => window.removeEventListener('scroll', agendarRecalculo)
  })

  // Recalcula también cuando cambia algo que NO viene de un evento de
  // scroll: alto/ancho de ventana, nº de columnas, alto de fila medido, o
  // el nº de filas (cambia el catálogo). Estos son mucho menos frecuentes
  // que el scroll, pero pasan por el mismo agendarRecalculo() para no leer
  // layout dos veces en el mismo frame si coinciden con un scroll, y para
  // sembrar filaInicio/filaFin en el primer render.
  $effect(() => {
    innerHeight
    innerWidth
    columnas
    altoFila
    filas
    agendarRecalculo()
  })

  // Ancho real del contenedor y alto real de una tarjeta: se miden tras
  // pintar, con al menos una tarjeta ya en el DOM. Disparado solo por
  // montaje/resize (innerWidth), nunca por scroll, para no realimentar el
  // efecto de arriba en cada frame.
  $effect(() => {
    innerWidth
    // canales.length también: al montar con la rejilla vacía (primera carga
    // en curso, ver RejillaCanales) no hay ninguna tarjeta que medir todavía;
    // en cuanto llega la primera página hay que reintentar la medición, si
    // no altoMedido se queda en 0 para siempre y solo un resize la dispara.
    canales.length
    if (!contenedor) return
    anchoContenedor = contenedor.clientWidth
    tick().then(medir)
  })

  function medir() {
    const muestra = contenedor?.querySelector('article')
    if (!muestra) return
    const alto = muestra.getBoundingClientRect().height
    // Umbral de 1px: sin él, una medición que oscila por redondeo
    // realimentaría el efecto de la ventana en un bucle sin fin.
    if (alto > 0 && Math.abs(alto - altoMedido) > 1) altoMedido = alto
  }

  const indiceInicio = $derived(filaInicio * columnas)
  const indiceFin = $derived(Math.min(canales.length, filaFin * columnas))
  const visibles = $derived(canales.slice(indiceInicio, indiceFin))
  // Fix ronda 1 (controlador, minor — doble GAP): un espaciador de N filas
  // ocultas NO necesita N*altoFila. El `gap:12px` de la propia grid YA pone
  // un separador entre el espaciador y la primera tarjeta visible (ese es
  // exactamente el gap real entre la última fila oculta y la primera
  // visible) — sumar N*altoFila = N*(alto+GAP) además de ese gap cuenta el
  // último GAP dos veces. El espaciador solo debe cubrir las N alturas de
  // fila MÁS los (N-1) gaps INTERNOS entre ellas, y dejar que el grid ponga
  // el último: N*alto + (N-1)*GAP = N*altoFila - GAP.
  const altoArriba = $derived(Math.max(0, filaInicio * altoFila - GAP))
  const altoAbajo = $derived(Math.max(0, (filas - filaFin) * altoFila - GAP))

  // Mismo centinela que en modo lista (ver RejillaCanales): al entrar en el
  // viewport pide la página siguiente. Vive fuera del contenedor
  // virtualizado, después del espaciador inferior, para quedar en la
  // posición del final REAL del documento — solo se cruza cuando de verdad
  // se ha llegado al final de los canales ya cargados, sin que la
  // virtualización adelante ni retrase el disparo.
  $effect(() => {
    if (!centinela) return
    const observador = new IntersectionObserver((entradas) => {
      if (entradas[0]?.isIntersecting) alPedirMas()
    })
    observador.observe(centinela)
    return () => observador.disconnect()
  })
</script>

<svelte:window bind:innerHeight bind:innerWidth />

<div class="rejilla-virtual" bind:this={contenedor}>
  {#if altoArriba > 0}
    <div class="espaciador" style:height="{altoArriba}px"></div>
  {/if}
  {#each visibles as canal (canal.id)}
    <TarjetaCanal {canal} {alAbrir} />
  {/each}
  {#if altoAbajo > 0}
    <div class="espaciador" style:height="{altoAbajo}px"></div>
  {/if}
</div>
<div class="centinela" bind:this={centinela} aria-hidden="true"></div>

<style>
  .rejilla-virtual { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 12px; }
  /* Ocupa toda la fila para no intercalarse como si fuera una tarjeta más:
     obliga a que lo siguiente empiece en una fila nueva. */
  .espaciador { grid-column: 1 / -1; }
  .centinela { height: 1px; }
</style>
