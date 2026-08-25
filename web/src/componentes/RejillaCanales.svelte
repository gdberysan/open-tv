<script lang="ts">
  import type { Canal } from '../datos/catalogo'
  import RejillaVirtual from './RejillaVirtual.svelte'
  import BarrasSenal from './BarrasSenal.svelte'
  import MarcaWeb from './MarcaWeb.svelte'
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
  <p class="vacio">{t('catalogo.vacio')}</p>
{:else if vista === 'rejilla'}
  <RejillaVirtual {canales} {alAbrir} {alPedirMas} />
{:else}
  <div class="canales lista">
    {#each canales as canal (canal.id)}
      <article class="fila">
        <button class="abrir" onclick={() => alAbrir(canal)} aria-label={canal.nombre}>
          {#if canal.logoUrl}
            <img src={canal.logoUrl} alt="" loading="lazy" />
          {:else}
            <span class="sinlogo" aria-hidden="true">{canal.nombre.slice(0, 2)}</span>
          {/if}
          <span class="nombre">{canal.nombre}</span>
        </button>
        {#if canal.pais}<span class="pais">{canal.pais}</span>{/if}
        <BarrasSenal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
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
  .fila img, .fila .sinlogo { width: 32px; height: 32px; object-fit: contain; flex-shrink: 0; }
  .fila .sinlogo { display: grid; place-items: center; background: var(--surface-sunken); color: var(--text-muted); border-radius: 4px; }
  .fila .nombre { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .fila .pais { font-size: 11px; color: var(--graphite-300); flex-shrink: 0; }
  .fila .favorito { all: unset; cursor: pointer; color: var(--graphite-500); margin-left: auto; }
  /* Ámbar = activo. Un favorito apagado es neutro. */
  .fila .favorito.activo { color: var(--amber-500); }
  .vacio, .cargando { text-align: center; color: var(--text-muted); padding: 24px 0; }
  .centinela { height: 1px; }
</style>
