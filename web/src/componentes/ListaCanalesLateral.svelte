<script lang="ts">
  import { onDestroy, tick } from 'svelte'
  import type { Canal } from '../datos/catalogo'
  import SenalCanal from './SenalCanal.svelte'
  import { parsearResolucion } from '../lib/resolucion'
  import { t } from '../i18n'

  // Lista estrecha del escenario reproductor-primero (spec §3). Virtualiza
  // contra su PROPIO scroll (el contenedor tiene overflow-y), no contra el
  // viewport como RejillaVirtual: la lateral tiene altura acotada por CSS y
  // sus filas son de altura CONSTANTE, así que la ventana es aritmética pura
  // sin medir el DOM. El roving tabindex replica el patrón de RejillaVirtual
  // (índice global, no referencia a nodo — ver su comentario largo).
  let { canales, canalActualId = null, alAbrir, alPedirMas }: {
    canales: Canal[]
    canalActualId?: string | null
    alAbrir: (c: Canal) => void
    alPedirMas: () => void
  } = $props()

  // DEBE coincidir con el height de .fila del bloque de estilos (nota: la
  // etiqueta va sin corchetes a propósito — un literal de etiqueta especial
  // dentro de un comentario del script confunde al compilador de Svelte,
  // ver la misma nota en App.svelte): la virtualización
  // entera depende de que cada fila mida exactamente esto.
  const ALTO_FILA = 48
  const FILAS_BUFFER = 8

  let contenedor: HTMLDivElement | undefined = $state()
  let centinela: HTMLDivElement | undefined = $state()
  let scrollTop = $state(0)
  let altoVisor = $state(0)

  // Coalescencia por rAF, mismo motivo que RejillaVirtual (fix ronda 1 de
  // P0.6): decenas de eventos scroll por segundo, una lectura por frame.
  let raf = 0
  function recalcular() {
    raf = 0
    if (!contenedor) return
    scrollTop = contenedor.scrollTop
    altoVisor = contenedor.clientHeight
  }
  function agendar() {
    if (raf) return
    raf = requestAnimationFrame(recalcular)
  }
  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf)
  })

  let innerHeight = $state(0)
  $effect(() => {
    innerHeight
    canales.length
    agendar()
  })

  const filaInicio = $derived(Math.max(0, Math.floor(scrollTop / ALTO_FILA) - FILAS_BUFFER))
  const filaFin = $derived(Math.min(canales.length, Math.ceil((scrollTop + altoVisor) / ALTO_FILA) + FILAS_BUFFER))
  const visibles = $derived(canales.slice(filaInicio, filaFin))
  const altoArriba = $derived(filaInicio * ALTO_FILA)
  const altoAbajo = $derived(Math.max(0, (canales.length - filaFin) * ALTO_FILA))

  // Roving tabindex: índice GLOBAL que sobrevive a que su fila entre o salga
  // de la ventana virtualizada (mismo principio que RejillaVirtual).
  let activeIndex = $state(0)
  $effect(() => {
    if (canales.length === 0) activeIndex = 0
    else if (activeIndex >= canales.length) activeIndex = canales.length - 1
  })

  function enfocarIndice(deseado: number) {
    if (canales.length === 0) return
    const objetivo = Math.max(0, Math.min(canales.length - 1, deseado))
    activeIndex = objetivo
    if (contenedor && (objetivo < filaInicio || objetivo >= filaFin)) {
      contenedor.scrollTop = objetivo * ALTO_FILA
      recalcular()
    }
    tick().then(() => {
      contenedor?.querySelector<HTMLElement>(`[data-indice="${objetivo}"] .abrir`)?.focus()
    })
  }

  function alTeclado(e: KeyboardEvent) {
    if (!(e.target instanceof HTMLElement) || !e.target.classList.contains('abrir')) return
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        enfocarIndice(activeIndex + 1)
        break
      case 'ArrowUp':
        e.preventDefault()
        enfocarIndice(activeIndex - 1)
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

  // Mismo centinela de paginación que RejillaVirtual, con root = el propio
  // contenedor de scroll (el default —viewport— no aplica a un scroll interno).
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

<svelte:window bind:innerHeight />

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
>
  {#if altoArriba > 0}<div class="espaciador" style:height="{altoArriba}px" aria-hidden="true"></div>{/if}
  {#each visibles as canal, i (canal.id)}
    {@const indice = filaInicio + i}
    {@const resolucion = parsearResolucion(canal.nombre)}
    {@const enCurso = canal.id === canalActualId}
    <article class="fila" role="listitem" data-indice={indice}>
      <button
        type="button"
        class="abrir"
        class:en-curso={enCurso}
        tabindex={indice === activeIndex ? 0 : -1}
        aria-current={enCurso ? 'true' : undefined}
        onclick={() => alAbrir(canal)}
      >
        <SenalCanal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
        <span class="nombre">{canal.nombre}</span>
        {#if resolucion}<span class="resolucion">{resolucion}</span>{/if}
        {#if enCurso}<span class="insignia-reproduciendo">{t('escenario.reproduciendo')}</span>{/if}
      </button>
    </article>
  {/each}
  {#if altoAbajo > 0}<div class="espaciador" style:height="{altoAbajo}px" aria-hidden="true"></div>{/if}
  <div class="centinela" bind:this={centinela} aria-hidden="true"></div>
</div>

<style>
  .lista-lateral {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }
  /* height DEBE coincidir con ALTO_FILA del script. */
  .fila {
    height: 48px;
    flex-shrink: 0;
    box-sizing: border-box;
  }
  .fila .abrir {
    all: unset;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: var(--space-2, 8px);
    width: 100%;
    height: 100%;
    padding: 0 var(--space-2, 8px);
    border-radius: var(--radius-sm, 5px);
    cursor: pointer;
    color: var(--text-body);
  }
  .fila .abrir:hover {
    background: var(--surface-raised);
  }
  /* Ámbar = canal activo (señal-viva/activo, token de marca). */
  .fila .abrir.en-curso {
    background: var(--tint-amber-weak);
  }
  .fila .abrir.en-curso .nombre {
    color: var(--amber-500);
  }
  .fila .nombre {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: var(--type-body-sm, inherit);
  }
  .fila .resolucion,
  .fila .insignia-reproduciendo {
    flex-shrink: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
  }
  .fila .insignia-reproduciendo {
    color: var(--amber-500);
  }
  .espaciador {
    flex-shrink: 0;
  }
  .centinela {
    height: 1px;
    flex-shrink: 0;
  }
</style>
