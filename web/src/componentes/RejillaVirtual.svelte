<script lang="ts">
  import { onDestroy, tick } from 'svelte'
  import type { Readable } from 'svelte/store'
  import type { AhoraDespues, Canal } from '../datos/catalogo'
  import TarjetaCanal from './TarjetaCanal.svelte'
  import { t } from '../i18n'

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
  // Teclado — roving tabindex (Tarea 18): el estado de "qué tarjeta tiene el
  // foco" NO puede vivir en el DOM (p.ej. "el <button> que tiene tabindex=0
  // ahora mismo"), porque ese nodo se DESTRUYE en cuanto la fila que le
  // corresponde sale de la ventana virtualizada — perdería el foco sin que
  // nada lo reciba, y un usuario de teclado quedaría literalmente perdido en
  // <body>. Por eso el estado real es `activeIndex`: un índice GLOBAL (no de
  // la ventana visible) que vive en RejillaVirtual y sobrevive a que su
  // tarjeta se monte o desmonte. Cada tarjeta visible recibe `indice` (su
  // posición global) y `focoActivo` (si coincide con activeIndex): SOLO esa
  // tarjeta tiene tabindex=0 en TODOS sus controles (el botón "abrir" y el
  // de favorito — fix round 1: el de favorito se quedó fuera la primera
  // vez, y una tarjeta no-activa seguía aportando un tab stop de más), el
  // resto -1 en ambos — el patrón estándar de roving tabindex, adaptado
  // para que la ÚNICA fuente de verdad sea un número, no una referencia a
  // un nodo.
  //
  // Mover el índice activo (enfocarIndice) a una fila fuera de
  // [filaInicio, filaFin) primero ABRE a mano una ventana nueva y acotada
  // alrededor de esa fila (no todas las filas intermedias — ver el
  // comentario de enfocarIndice) para que la fila destino exista en el DOM,
  // y solo TRAS un tick() —cuando Svelte ya pintó esa tarjeta— se le pide el
  // foco. Pedirlo antes del tick() no encontraría ningún nodo con ese
  // [data-indice]; es la misma clase de bug que un desplazamiento de
  // tabindex al vacío.
  //
  // El Tab nativo del navegador entre <button> sigue funcionando igual que
  // siempre (no se toca): con roving tabindex, un Tab que ENTRA en la
  // rejilla aterriza en el primer control con tabindex=0 de la tarjeta
  // ACTIVA (su botón "abrir"), un segundo Tab pasa al otro control de esa
  // MISMA tarjeta (su botón de favorito — el único otro tabindex=0 que
  // existe), y el siguiente Tab ya sale de la rejilla: ninguna tarjeta
  // no-activa aporta tab stop alguno, ni de abrir ni de favorito. Eso es lo
  // que hace que el roving tabindex reduzca los tab stops de la rejilla de
  // 2·N (dos controles por cada una de las N tarjetas) a 2 (los de la única
  // tarjeta activa) en vez de no cambiar nada — las flechas recorren la
  // colección; Tab la atraviesa.
  // densidad (Tarea 5, P0.8): 'comoda' (por defecto) o 'compacta'. Deriva
  // anchoMin (más abajo) — el resto del cálculo de ventana (columnas,
  // altoFila, filaInicio/filaFin, espaciadores, roving tabindex) no sabe nada
  // de densidad: solo consume anchoMin, así que cambiar de densidad es
  // exactamente el mismo camino que un resize de ventana (agendarRecalculo
  // ya se dispara porque columnas es $derived de anchoMin).
  // epg/alVisiblesCambiar (Tarea 8, P2 EPG): opcionales, mismo criterio que
  // densidad — no romper a nadie que use esta rejilla suelta (tests
  // existentes incluidos). epg se reenvía tal cual a cada <TarjetaCanal>
  // (esta rejilla NUNCA lee el store, solo lo pasa); alVisiblesCambiar es el
  // lado de ESCRITURA: reporta el lote de ids visibles cuando la ventana
  // cambia, para que quien monte la rejilla (App.svelte) decida qué hacer
  // con ellos (epg.asegurar) — RejillaVirtual no conoce epg.asegurar ni
  // debe conocerlo, solo conoce "qué está en pantalla ahora mismo".
  let { canales, alAbrir, alPedirMas, densidad = 'comoda', epg, alVisiblesCambiar }: {
    canales: Canal[]
    alAbrir: (c: Canal) => void
    alPedirMas: () => void
    densidad?: 'comoda' | 'compacta'
    epg?: Readable<Map<string, AhoraDespues>>
    alVisiblesCambiar?: (ids: string[]) => void
  } = $props()

  // Deben coincidir con el grid CSS de abajo (minmax(var(--ancho-min),1fr),
  // gap 12px): es la misma cuenta de columnas que hace el navegador con
  // auto-fill — por eso anchoMin se manda también como custom property
  // inline (ver el <div> de más abajo), en vez de vivir solo en JS.
  const ANCHO_MIN_COMODA = 160
  const ANCHO_MIN_COMPACTA = 128
  const anchoMin = $derived(densidad === 'compacta' ? ANCHO_MIN_COMPACTA : ANCHO_MIN_COMODA)
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

  const columnas = $derived(Math.max(1, Math.floor((anchoContenedor + GAP) / (anchoMin + GAP))))
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

  // Tarea 8 (P2, EPG): reporta el lote de ids visibles cuando la ventana
  // cambia. COALESCENCIA: un solo array por cambio de ventana, nunca una
  // llamada por tarjeta — visibles ya es la ventana entera (recalculada como
  // mucho una vez por rAF, ver recalcularVentana), así que este efecto
  // reacciona al mismo ritmo, no al de cada tarjeta individual.
  $effect(() => {
    if (!alVisiblesCambiar) return
    alVisiblesCambiar(visibles.map((c) => c.id))
  })
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

  // Roving tabindex (Tarea 18): ver el comentario largo de más arriba sobre
  // por qué el estado es un ÍNDICE GLOBAL y no una referencia a un nodo.
  let activeIndex = $state(0)

  // Si el catálogo se reduce (cambia un filtro, o soloFavoritos da menos
  // resultados), activeIndex puede quedar apuntando fuera de rango. Se
  // recorta al último índice válido — o a 0 si la lista quedó vacía —, en
  // vez de dejarlo huérfano hasta el próximo enfocarIndice().
  $effect(() => {
    if (canales.length === 0) {
      activeIndex = 0
    } else if (activeIndex >= canales.length) {
      activeIndex = canales.length - 1
    }
  })

  function enfocarIndice(indiceDeseado: number) {
    if (canales.length === 0) return
    const objetivo = Math.max(0, Math.min(canales.length - 1, indiceDeseado))
    activeIndex = objetivo
    const filaObjetivo = Math.floor(objetivo / columnas)
    if (filaObjetivo < filaInicio || filaObjetivo >= filaFin) {
      // La fila objetivo cae fuera de la ventana actual: se abre una
      // ventana NUEVA y acotada alrededor de ella, en vez de solo estirar
      // el borde más cercano de la ventana vieja hasta alcanzarla. Estirar
      // el borde montaría TODAS las filas intermedias — con Fin desde la
      // primera fila de un catálogo de miles, eso sería montar el
      // catálogo entero de una vez, justo lo que la virtualización existe
      // para evitar.
      filaInicio = Math.max(0, filaObjetivo - FILAS_BUFFER)
      filaFin = Math.min(filas, filaObjetivo + 1 + FILAS_BUFFER)
    }
    tick().then(() => {
      const nodo = contenedor?.querySelector<HTMLElement>(`[data-indice="${objetivo}"] .abrir`)
      // scrollIntoView ANTES de focus(): deja la tarjeta dentro del
      // viewport real, para que el próximo recalcularVentana() (el que
      // dispara el propio evento 'scroll' de este scrollIntoView) la
      // encuentre ya visible por derecho propio y no la vuelva a soltar de
      // la ventana en el siguiente frame.
      // scrollIntoView no existe en jsdom (no implementa layout real); se
      // llama con encadenamiento opcional también en el propio método para
      // no reventar los tests, sin dejar de llamarlo en un navegador real.
      nodo?.scrollIntoView?.({ block: 'nearest' })
      nodo?.focus()
    })
  }

  // Flechas mueven activeIndex; Enter/Espacio abren el canal por la
  // semántica nativa del <button> enfocado — no hace falta reimplementarla.
  // Se filtra por `.abrir` porque las flechas navegan ENTRE tarjetas
  // (mueven qué tarjeta es la activa), no entre los controles DENTRO de
  // una misma tarjeta: con el foco en el botón de favorito (que sí
  // participa del roving tabindex — su tabindex también sigue a
  // focoActivo, ver TarjetaCanal.svelte — pero al que las flechas no
  // apuntan) una flecha debe hacer lo de siempre (nada especial aquí),
  // no saltar de tarjeta.
  function alTecladoRejilla(e: KeyboardEvent) {
    if (!(e.target instanceof HTMLElement) || !e.target.classList.contains('abrir')) return
    switch (e.key) {
      case 'ArrowRight':
        e.preventDefault()
        enfocarIndice(activeIndex + 1)
        break
      case 'ArrowLeft':
        e.preventDefault()
        enfocarIndice(activeIndex - 1)
        break
      case 'ArrowDown':
        e.preventDefault()
        enfocarIndice(activeIndex + columnas)
        break
      case 'ArrowUp':
        e.preventDefault()
        enfocarIndice(activeIndex - columnas)
        break
      case 'Home':
        e.preventDefault()
        enfocarIndice(0)
        break
      case 'End':
        e.preventDefault()
        enfocarIndice(canales.length - 1)
        break
    }
  }

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

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<!-- El keydown aquí NO convierte la lista en un widget interactivo propio:
     es delegación de evento para el roving tabindex de sus <button> hijos
     (que SÍ son interactivos y llevan el tabindex real). El propio
     contenedor no recibe foco ni tabindex. -->
<div
  class="rejilla-virtual"
  bind:this={contenedor}
  role="list"
  aria-label={t('rejilla.etiquetaLista')}
  onkeydown={alTecladoRejilla}
  style:--ancho-min="{anchoMin}px"
  data-columnas={columnas}
>
  {#if altoArriba > 0}
    <div class="espaciador" style:height="{altoArriba}px" aria-hidden="true"></div>
  {/if}
  {#each visibles as canal, i (canal.id)}
    <TarjetaCanal
      {canal}
      {alAbrir}
      {densidad}
      {epg}
      indice={indiceInicio + i}
      focoActivo={indiceInicio + i === activeIndex}
    />
  {/each}
  {#if altoAbajo > 0}
    <div class="espaciador" style:height="{altoAbajo}px" aria-hidden="true"></div>
  {/if}
</div>
<div class="centinela" bind:this={centinela} aria-hidden="true"></div>

<style>
  /* --ancho-min (inline, ver arriba): la MISMA anchoMin que gobierna el
     cálculo de columnas en JS — nunca dos fuentes de verdad del ancho
     mínimo de columna. data-columnas es solo un gancho de test (Tarea 5,
     P0.8): no tiene estilo propio ni afecta el layout. */
  .rejilla-virtual { display: grid; grid-template-columns: repeat(auto-fill, minmax(var(--ancho-min, 160px), 1fr)); gap: 12px; }
  /* Ocupa toda la fila para no intercalarse como si fuera una tarjeta más:
     obliga a que lo siguiente empiece en una fila nueva. */
  .espaciador { grid-column: 1 / -1; }
  .centinela { height: 1px; }
</style>
