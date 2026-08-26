<script lang="ts">
  import type { Readable } from 'svelte/store'
  import type { AhoraDespues, Canal } from '../datos/catalogo'
  import RejillaVirtual from './RejillaVirtual.svelte'
  import SenalCanal from './SenalCanal.svelte'
  import MarcaWeb from './MarcaWeb.svelte'
  import LogoCanal from './LogoCanal.svelte'
  import Vacio from './Vacio.svelte'
  import { formatearHoraLocal } from '../lib/hora'
  import { favoritos } from '../estado/favoritos'
  import { t } from '../i18n'

  // densidad (Tarea 5, P0.8): solo tiene sentido en modo rejilla (RejillaVirtual);
  // la vista lista no usa TarjetaCanal ni tiene columnas que ensanchar/estrechar,
  // así que este componente solo la reenvía, sin decidir nada sobre ella.
  // epg/alVisiblesCambiar (Tarea 8/paridad de lista, P2 EPG): en modo rejilla
  // son puro paso hacia RejillaVirtual (que las reenvía a cada TarjetaCanal).
  // En modo lista este componente SÍ los consume directamente — mismo patrón
  // de suscripción que TarjetaCanal, ver más abajo — porque la fila <article>
  // de la vista lista es un markup propio, sin TarjetaCanal.
  let { canales, vista, cargando, alPedirMas, alAbrir, densidad = 'comoda', epg, alVisiblesCambiar }: {
    canales: Canal[]
    vista: 'rejilla' | 'lista'
    cargando: boolean
    alPedirMas: () => void
    alAbrir: (c: Canal) => void
    densidad?: 'comoda' | 'compacta'
    epg?: Readable<Map<string, AhoraDespues>>
    alVisiblesCambiar?: (ids: string[]) => void
  } = $props()

  let centinela: HTMLElement | undefined = $state()

  // Suscripción al store epg (mismo patrón que TarjetaCanal.svelte): una
  // sola suscripción a nivel de componente, no una por fila — la lista no
  // virtualiza, así que basta con leer mapaEpg.get(canal.id) dentro del
  // {#each}. Sin epg (prop ausente): mapaEpg se queda en el Map vacío
  // inicial y cada fila queda limpia, igual que TarjetaCanal sin epg.
  let mapaEpg = $state<Map<string, AhoraDespues>>(new Map())
  $effect(() => {
    if (!epg) return
    return epg.subscribe((m) => {
      mapaEpg = m
    })
  })

  // alVisiblesCambiar en modo lista (paridad con RejillaVirtual, que ya lo
  // hace por su cuenta en modo rejilla — por eso esto se limita a
  // vista==='lista', para no pedir la guía dos veces). La lista no
  // virtualiza: "visible" aquí es "cargado", así que se reporta el array
  // completo de ids cada vez que `canales` crece (paginación). El store
  // (epg.asegurar, fuera de este componente) cachea/coalesce por id, así
  // que repetir ids ya conocidos no re-pide nada.
  $effect(() => {
    if (vista !== 'lista') return
    alVisiblesCambiar?.(canales.map((c) => c.id))
  })

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
  <RejillaVirtual {canales} {alAbrir} {alPedirMas} {densidad} {epg} {alVisiblesCambiar} />
{:else}
  <div class="canales lista" role="list" aria-label={t('rejilla.etiquetaLista')}>
    {#each canales as canal (canal.id)}
      {@const ahoraDespues = mapaEpg.get(canal.id)}
      <article class="fila" role="listitem">
        <button
          class="abrir"
          onclick={() => alAbrir(canal)}
          aria-label={ahoraDespues?.ahora
            ? `${canal.nombre}. ${t('epg.ahora')}: ${ahoraDespues.ahora.titulo}`
            : canal.nombre}
        >
          <div class="logo"><LogoCanal logoUrl={canal.logoUrl} nombre={canal.nombre} /></div>
          <span class="info">
            <span class="nombre">{canal.nombre}</span>
            <!-- Insignia ahora/después (paridad con TarjetaCanal, Tarea 8 P2
                 EPG). La lista NO virtualiza, así que no hace falta altura
                 fija: sin guía, sin texto, fila limpia. Es VISUAL: el
                 aria-label del botón (arriba) lleva «<nombre>. Ahora: <título>»
                 cuando hay guía, porque un aria-label explícito anula el texto
                 interno para el nombre accesible — sin él, el lector de
                 pantalla no anunciaría la guía. Sin aria-live nuevo. -->
            {#if ahoraDespues?.ahora}
              <span class="linea-epg" aria-hidden="true">● {t('epg.ahora')}: {ahoraDespues.ahora.titulo}{#if ahoraDespues.siguiente} · {t('epg.siguiente')} {formatearHoraLocal(ahoraDespues.siguiente.inicioSeg)} · {ahoraDespues.siguiente.titulo}{/if}</span>
            {/if}
          </span>
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
  /* .info agrupa nombre + insignia EPG en columna, para que la insignia
     quede debajo del nombre en vez de en la misma fila horizontal (que sí
     tienen logo/nombre) — sin esto, el flex-row de .abrir pondría la
     insignia a la derecha del nombre en vez de debajo. */
  .fila .info { display: flex; flex-direction: column; min-width: 0; overflow: hidden; }
  .fila .nombre { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  /* Insignia ahora/después (paridad de lista, P2 EPG): mismo estilo textual
     que TarjetaCanal.svelte (.linea-epg), pero SIN altura fija reservada —
     la lista no virtualiza (a diferencia de RejillaVirtual/TarjetaCanal),
     así que no hay ventana de altura uniforme que proteger; sin guía, la
     fila simplemente no reserva esa línea. */
  .fila .linea-epg {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    font-size: 11px;
    color: var(--text-muted);
  }
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
