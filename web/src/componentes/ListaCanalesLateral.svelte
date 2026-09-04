<script lang="ts">
  import { onDestroy, tick } from 'svelte'
  import type { Canal } from '../datos/catalogo'
  import LogoCanal from './LogoCanal.svelte'
  import SenalCanal from './SenalCanal.svelte'
  import { parsearResolucion } from '../lib/resolucion'
  import { t } from '../i18n'

  // Lista estrecha del escenario reproductor-primero (spec §3): cuadrícula de
  // miniaturas CUADRADAS (antes: filas de solo texto — la miniatura da la
  // misma lectura visual en móvil o escritorio, sea cual sea el ancho de la
  // lateral). Virtualiza contra su PROPIO scroll (el contenedor tiene
  // overflow-y), no contra el viewport como RejillaVirtual.
  //
  // El nº de columnas sigue el MISMO cálculo auto-fill/minmax que
  // RejillaVirtual (ver su comentario largo): --ancho-min se manda también
  // como custom property inline para que JS y el grid CSS cuenten siempre
  // las mismas columnas. A diferencia de RejillaVirtual, aquí NO hace falta
  // medir el alto de una tarjeta real: la miniatura es un cuadrado
  // (aspect-ratio 1/1), así que su alto es el mismo ancho ya calculado, y el
  // nombre debajo tiene una altura FIJA de una sola línea (ALTO_NOMBRE) — la
  // ventana entera sigue siendo aritmética pura, sin querySelector ni tick()
  // de medición.
  //
  // Roving tabindex: mismo patrón que RejillaVirtual (índice GLOBAL, no
  // referencia a nodo), con Arriba/Abajo moviendo una fila entera
  // (± columnas) e Izquierda/Derecha moviendo una celda.
  let { canales, canalActualId = null, alAbrir, alPedirMas }: {
    canales: Canal[]
    canalActualId?: string | null
    alAbrir: (c: Canal) => void
    alPedirMas: () => void
  } = $props()

  // DEBEN coincidir con el CSS de abajo: el grid usa minmax(var(--ancho-min),1fr)
  // con este mismo valor, GAP es el `gap` del grid, y ALTO_NOMBRE + MARGEN_NOMBRE
  // son el alto reservado a .nombre bajo la miniatura.
  const ANCHO_MIN = 96
  const GAP = 16
  const MARGEN_NOMBRE = 8
  const ALTO_NOMBRE = 16
  const FILAS_BUFFER = 3

  let contenedor: HTMLDivElement | undefined = $state()
  let centinela: HTMLDivElement | undefined = $state()
  let scrollTop = $state(0)
  let altoVisor = $state(0)
  let anchoContenedor = $state(0)

  // Coalescencia por rAF, mismo motivo que RejillaVirtual (fix ronda 1 de
  // P0.6): decenas de eventos scroll por segundo, una lectura por frame.
  let raf = 0
  function recalcular() {
    raf = 0
    if (!contenedor) return
    scrollTop = contenedor.scrollTop
    altoVisor = contenedor.clientHeight
    anchoContenedor = contenedor.clientWidth
  }
  function agendar() {
    if (raf) return
    raf = requestAnimationFrame(recalcular)
  }
  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf)
  })

  // innerWidth dispara un recálculo cuando la lateral cambia de ancho (p.ej.
  // el breakpoint de 900px que la vuelve de ancho completo) — igual que
  // RejillaVirtual, que solo remide en resize, nunca en cada scroll.
  let innerWidth = $state(0)
  $effect(() => {
    innerWidth
    canales.length
    agendar()
  })

  // Columnas: MISMA fórmula que RejillaVirtual (floor((ancho+GAP)/(anchoMin+GAP))),
  // con el mismo respaldo a 1 columna cuando aún no hay layout real medido
  // (anchoContenedor=0, p.ej. jsdom en los tests o el primer paint).
  const columnas = $derived(Math.max(1, Math.floor((anchoContenedor + GAP) / (ANCHO_MIN + GAP))))
  const anchoCelda = $derived(anchoContenedor > 0 ? (anchoContenedor - (columnas - 1) * GAP) / columnas : ANCHO_MIN)
  // Alto de fila con el separador incluido: miniatura cuadrada (anchoCelda) +
  // el nombre de una línea + su margen, + el gap del grid hasta la fila
  // siguiente.
  const altoFila = $derived(anchoCelda + MARGEN_NOMBRE + ALTO_NOMBRE + GAP)

  const filas = $derived(canales.length === 0 ? 0 : Math.ceil(canales.length / columnas))
  const filaInicio = $derived(Math.max(0, Math.floor(scrollTop / altoFila) - FILAS_BUFFER))
  const filaFin = $derived(Math.min(filas, Math.ceil((scrollTop + altoVisor) / altoFila) + FILAS_BUFFER))
  const indiceInicio = $derived(filaInicio * columnas)
  const indiceFin = $derived(Math.min(canales.length, filaFin * columnas))
  const visibles = $derived(canales.slice(indiceInicio, indiceFin))
  // Fix "doble GAP" (mismo motivo que RejillaVirtual): el espaciador de N
  // filas ocultas no necesita N*altoFila — el propio `gap` del grid ya pone
  // un separador entre el espaciador y la primera fila visible.
  const altoArriba = $derived(Math.max(0, filaInicio * altoFila - GAP))
  const altoAbajo = $derived(Math.max(0, (filas - filaFin) * altoFila - GAP))

  // Roving tabindex: índice GLOBAL que sobrevive a que su celda entre o
  // salga de la ventana virtualizada (mismo principio que RejillaVirtual).
  let activeIndex = $state(0)
  $effect(() => {
    if (canales.length === 0) activeIndex = 0
    else if (activeIndex >= canales.length) activeIndex = canales.length - 1
  })

  function enfocarIndice(deseado: number) {
    if (canales.length === 0) return
    const objetivo = Math.max(0, Math.min(canales.length - 1, deseado))
    activeIndex = objetivo
    const filaObjetivo = Math.floor(objetivo / columnas)
    if (contenedor && (filaObjetivo < filaInicio || filaObjetivo >= filaFin)) {
      contenedor.scrollTop = filaObjetivo * altoFila
      recalcular()
    }
    tick().then(() => {
      contenedor?.querySelector<HTMLElement>(`[data-indice="${objetivo}"] .abrir`)?.focus()
    })
  }

  // Flechas mueven activeIndex; Enter/Espacio abren el canal por la semántica
  // nativa del <button> enfocado. Arriba/Abajo saltan una fila entera
  // (± columnas); Izquierda/Derecha se mueven dentro de la fila (± 1).
  function alTeclado(e: KeyboardEvent) {
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

  // Mismo centinela de paginación que antes, con root = el propio contenedor
  // de scroll (el default —viewport— no aplica a un scroll interno).
  $effect(() => {
    if (!centinela || !contenedor) return
    const observador = new IntersectionObserver(
      (entradas) => {
        if (entradas[0]?.isIntersecting) alPedirMas()
      },
      { root: contenedor },
    )
    observador.observe(centinela)
    return () => observador.disconnect()
  })
</script>

<svelte:window bind:innerWidth />

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<!-- keydown = delegación del roving tabindex de los <button> hijos, como en
     RejillaVirtual: el contenedor no es interactivo ni recibe foco. -->
<div
  class="lista-lateral"
  bind:this={contenedor}
  role="list"
  aria-label={t('lateral.lista')}
  onscroll={agendar}
  onkeydown={alTeclado}
  style:--ancho-min="{ANCHO_MIN}px"
  data-columnas={columnas}
>
  {#if altoArriba > 0}<div class="espaciador" style:height="{altoArriba}px" aria-hidden="true"></div>{/if}
  {#each visibles as canal, i (canal.id)}
    {@const indice = indiceInicio + i}
    {@const resolucion = parsearResolucion(canal.nombre)}
    {@const enCurso = canal.id === canalActualId}
    <article class="fila" role="listitem" data-indice={indice}>
      <!-- onfocus sincroniza activeIndex con la realidad: el foco puede llegar
           por CLIC (no solo por flechas), y sin esto la siguiente flecha
           partiría de un índice rancio — el foco saltaba a una celda lejana o,
           si esa celda ya no estaba montada en la ventana virtual, caía a
           body (bug real cazado en el gate de teclado en Chrome, ver
           RejillaVirtual). Es el patrón roving estándar: quien RECIBE el
           foco es el índice activo. -->
      <button
        type="button"
        class="abrir"
        class:en-curso={enCurso}
        tabindex={indice === activeIndex ? 0 : -1}
        aria-current={enCurso ? 'true' : undefined}
        onfocus={() => (activeIndex = indice)}
        onclick={() => alAbrir(canal)}
      >
        <span class="logo">
          <LogoCanal logoUrl={canal.logoUrl} nombre={canal.nombre} />
          <span class="insignia insignia-salud">
            <SenalCanal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
          </span>
          {#if resolucion}<span class="insignia insignia-resolucion">{resolucion}</span>{/if}
          {#if enCurso}<span class="insignia insignia-reproduciendo">{t('escenario.reproduciendo')}</span>{/if}
        </span>
        <span class="nombre">{canal.nombre}</span>
      </button>
    </article>
  {/each}
  {#if altoAbajo > 0}<div class="espaciador" style:height="{altoAbajo}px" aria-hidden="true"></div>{/if}
  <div class="centinela" bind:this={centinela} aria-hidden="true"></div>
</div>

<style>
  /* --ancho-min (inline, ver arriba): la MISMA ANCHO_MIN que gobierna el
     cálculo de columnas en JS — nunca dos fuentes de verdad. data-columnas
     es solo un gancho de test/depuración, sin estilo propio. */
  .lista-lateral {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(var(--ancho-min, 96px), 1fr));
    gap: 16px;
    align-content: start;
  }
  .fila {
    box-sizing: border-box;
  }
  .fila .abrir {
    all: unset;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    width: 100%;
    cursor: pointer;
    border-radius: var(--radius-sm, 5px);
    color: var(--text-body);
  }
  .fila .abrir:hover .logo {
    outline: 1px solid var(--border-default);
  }
  /* Miniatura cuadrada: mismo primitivo LogoCanal que TarjetaCanal, agnóstico
     de tamaño — este contenedor decide el 1:1. Insignias con el mismo
     lenguaje visual que TarjetaCanal (scrim + esquina), así rejilla y
     lateral hablan el MISMO idioma de señal. */
  .logo {
    position: relative;
    width: 100%;
    aspect-ratio: 1 / 1;
    border-radius: var(--radius-sm, 5px);
    overflow: hidden;
  }
  /* Ámbar = canal activo (señal-viva/activo, token de marca): anillo sobre la
     miniatura en vez de fondo de fila — la miniatura ya llena la celda. */
  .fila .abrir.en-curso .logo {
    box-shadow: 0 0 0 2px var(--amber-500) inset;
  }
  .fila .abrir.en-curso .nombre {
    color: var(--amber-500);
  }
  .insignia {
    position: absolute;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 5px;
    border-radius: 4px;
    background: var(--scrim-strong);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-strong, #fff);
  }
  .insignia-salud {
    top: 4px;
    left: 4px;
  }
  .insignia-resolucion {
    bottom: 4px;
    right: 4px;
  }
  .insignia-reproduciendo {
    bottom: 4px;
    left: 4px;
    color: var(--amber-500);
  }
  .nombre {
    margin-top: 8px;
    height: 16px;
    line-height: 16px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: center;
    font: var(--type-body-sm, inherit);
    font-size: 11px;
  }
  .espaciador {
    grid-column: 1 / -1;
  }
  .centinela {
    grid-column: 1 / -1;
    height: 1px;
  }
</style>
