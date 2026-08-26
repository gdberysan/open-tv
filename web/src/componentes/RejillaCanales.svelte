<script lang="ts">
  import type { Canal } from '../datos/catalogo'
  import RejillaVirtual from './RejillaVirtual.svelte'
  import SenalCanal from './SenalCanal.svelte'
  import MarcaWeb from './MarcaWeb.svelte'
  import LogoCanal from './LogoCanal.svelte'
  import Vacio from './Vacio.svelte'
  import { favoritos } from '../estado/favoritos'
  import { t } from '../i18n'

  let { canales, vista, cargando, alPedirMas, alAbrir }: {
    canales: Canal[]
    vista: 'rejilla' | 'lista'
    cargando: boolean
    alPedirMas: () => void
    alAbrir: (c: Canal) => void
  } = $props()

  let centinela: HTMLElement | undefined = $state()

  // Un solo IntersectionObserver por instancia sobre el centinela al final
  // de la lista EN MODO LISTA: cuando entra en el viewport, pide la página
  // siguiente. App decide si hay más que pedir (soloFavoritos no pagina, ver
  // Tarea 12). En modo rejilla el centinela equivalente vive DENTRO de
  // RejillaVirtual (Tarea 17): la ventana virtualizada necesita controlar
  // ella misma la posición de su propio centinela en el documento.
  $effect(() => {
    if (vista !== 'lista') return
    if (!centinela) return
    const observador = new IntersectionObserver((entradas) => {
      if (entradas[0]?.isIntersecting) alPedirMas()
    })
    observador.observe(centinela)
    return () => observador.disconnect()
  })
</script>

{#if canales.length === 0 && !cargando}
  <!-- Tarea 13 (P0.6): el vacío estático se sustituye por Vacio.svelte, que
       deriva una sugerencia concreta del filtro activo en vez de solo
       constatar "no hay nada". -->
  <Vacio />
{:else if vista === 'rejilla'}
  <RejillaVirtual {canales} {alAbrir} {alPedirMas} />
{:else}
  <div class="canales lista" role="list" aria-label={t('rejilla.etiquetaLista')}>
    {#each canales as canal (canal.id)}
      <article class="fila" role="listitem">
        <button class="abrir" onclick={() => alAbrir(canal)} aria-label={canal.nombre}>
          <div class="logo"><LogoCanal logoUrl={canal.logoUrl} nombre={canal.nombre} /></div>
          <span class="nombre">{canal.nombre}</span>
        </button>
        {#if canal.pais}<span class="pais">{canal.pais}</span>{/if}
        <!-- Fix 1 (Tarea 8): mismo componente que la insignia de la rejilla
             (TarjetaCanal), sin scrim/badge — aquí es una fila con su propio
             fondo (--surface-card), no una miniatura. Rejilla y lista ya
             hablan el mismo lenguaje de señal. -->
        <SenalCanal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
        <MarcaWeb webOk={canal.webOk} />
        <button
          class="favorito"
          class:activo={$favoritos.has(canal.id)}
          aria-pressed={$favoritos.has(canal.id)}
          aria-label={$favoritos.has(canal.id) ? t('canal.favorito.quitar') : t('canal.favorito.anadir')}
          onclick={() => favoritos.alternar(canal.id)}
        >★</button>
      </article>
    {/each}
  </div>
  <div class="centinela" bind:this={centinela} aria-hidden="true"></div>
{/if}

{#if cargando}<p class="cargando">{t('catalogo.cargando')}</p>{/if}

<style>
  .canales.lista { display: flex; flex-direction: column; gap: 4px; }
  .fila { display: flex; align-items: center; gap: 8px; background: var(--surface-card); border-radius: 6px; padding: 6px 10px; }
  .fila .abrir { all: unset; cursor: pointer; display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
  /* LogoCanal (fix final, hallazgo 1) no decide tamaño ni redondeo: este
     contenedor reproduce el mismo 32×32 que antes tenían img/.sinlogo
     directamente, y overflow:hidden+border-radius le da el borde redondeado
     que .sinlogo tenía en la lista (y la rejilla NO tiene) sin que
     LogoCanal necesite saber que existe esa diferencia entre vistas. */
  .fila .logo { width: 32px; height: 32px; flex-shrink: 0; border-radius: 4px; overflow: hidden; }
  .fila .nombre { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  /* Contraste (Tarea 18): mismo arreglo que TarjetaCanal.svelte — se compone
     con --text-muted en vez de --graphite-300/--graphite-500 (ver ahí el
     cálculo). */
  .fila .pais { font-size: 11px; color: var(--text-muted); flex-shrink: 0; }
  .fila .favorito { all: unset; cursor: pointer; color: var(--text-muted); margin-left: auto; }
  /* Ámbar = activo. Un favorito apagado es neutro. */
  .fila .favorito.activo { color: var(--amber-500); }
  .cargando { text-align: center; color: var(--text-muted); padding: 24px 0; }
  .centinela { height: 1px; }
</style>
