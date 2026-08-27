<script lang="ts">
  import { untrack } from 'svelte'
  import type { Canal, Faceta } from '../datos/catalogo'
  import { filtros } from '../estado/filtros'
  import { debounce } from '../lib/debounce'
  import { t } from '../i18n'
  import FacetasCompactas from './FacetasCompactas.svelte'
  import ListaCanalesLateral from './ListaCanalesLateral.svelte'

  // Barra lateral de catálogo del escenario (spec §3): buscador + facetas
  // compactas + lista virtualizada. NO habla con CatalogSource — App sigue
  // siendo el único que consulta; aquí solo llegan datos y salen callbacks.
  let { canales, total, cargando, paises, categorias, canalActualId = null, alAbrir, alPedirMas }: {
    canales: Canal[]
    total: number
    cargando: boolean
    paises: Faceta[]
    categorias: Faceta[]
    canalActualId?: string | null
    alAbrir: (c: Canal) => void
    alPedirMas: () => void
  } = $props()

  // Buscador↔store: mismo patrón (y mismo porqué) que BarraLateralFacetas —
  // debounce para no consultar por tecla; ultimoEmpujado para distinguir el
  // eco de nuestro propio empuje de un cambio llegado de fuera (chips,
  // «Limpiar filtros») sin pisar lo que el usuario está tecleando dentro de
  // la ventana de debounce.
  let busqueda = $state($filtros.q ?? '')
  let ultimoEmpujado = $state($filtros.q ?? '')
  const buscarConRetardo = debounce((valor: string) => {
    $filtros.q = valor
    ultimoEmpujado = valor
  }, 300)
  function alEscribir(evento: Event) {
    busqueda = (evento.target as HTMLInputElement).value
    buscarConRetardo(busqueda)
  }
  $effect(() => {
    const qActual = $filtros.q ?? ''
    if (qActual !== untrack(() => ultimoEmpujado)) {
      busqueda = qActual
      ultimoEmpujado = qActual
    }
  })
</script>

<div class="catalogo-lateral">
  <div class="buscador">
    <!-- Motivo ámbar decorativo (aria-hidden), como en BarraLateralFacetas. -->
    <span class="motivo" aria-hidden="true">&lt;</span>
    <input
      type="search"
      class="buscar"
      placeholder={t('catalogo.buscar')}
      aria-label={t('catalogo.buscar')}
      value={busqueda}
      oninput={alEscribir}
    />
  </div>

  <FacetasCompactas {paises} {categorias} />

  <p class="conteo-total">{t('catalogo.total', { n: total })}</p>

  <ListaCanalesLateral {canales} {canalActualId} {alAbrir} {alPedirMas} />

  {#if cargando}<p class="cargando">{t('catalogo.cargando')}</p>{/if}
  {#if !cargando && canales.length === 0}<p class="vacio">{t('catalogo.vacio')}</p>{/if}
</div>

<style>
  .catalogo-lateral {
    display: flex;
    flex-direction: column;
    gap: var(--space-3, 12px);
    height: 100%;
    min-height: 0;
  }
  /* Mismo tratamiento que el buscador de BarraLateralFacetas. */
  .buscador {
    display: flex;
    align-items: center;
    gap: 6px;
    background: var(--surface-sunken);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    padding: 6px 10px;
  }
  .motivo {
    color: var(--amber-500);
    font: var(--type-mono, inherit);
  }
  .buscar {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-body);
    font: var(--type-body, inherit);
    min-width: 0;
  }
  .buscar:focus {
    outline: none;
  }
  .conteo-total {
    margin: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
  }
  .cargando,
  .vacio {
    margin: 0;
    color: var(--text-muted);
    font: var(--type-body-sm, inherit);
  }
</style>
