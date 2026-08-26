<script lang="ts">
  import { filtros } from '../estado/filtros'
  import type { Frescura as InfoFrescura } from '../datos/catalogo'
  import { t } from '../i18n'
  import ChipsFiltro from './ChipsFiltro.svelte'
  import Frescura from './Frescura.svelte'

  // Tarea 7 (P0.6): barra de acciones del área principal. Reúne el conteo,
  // los chips de filtro removibles (ChipsFiltro) y los tres controles que la
  // Tarea 6 dejó viviendo temporalmente en App (.acciones-temporales):
  // "Solo favoritos", "Canal al azar" y el conmutador Rejilla/Lista. App
  // sigue siendo el único que habla con CatalogSource: aquí solo se lee/
  // escribe el store `filtros` y se reenvía `alAleatorio`.
  let { total, frescura, alAleatorio }: {
    total: number
    frescura: InfoFrescura | null
    alAleatorio: () => void
  } = $props()

  // "Limpiar filtros" resetea las CUATRO dimensiones de la consulta más
  // soloFavoritos de una sola vez — mostrarOffline se deja fuera a
  // propósito: no es un filtro que un chip represente (Tarea 6 ya lo trata
  // como una faceta de señal, no de "qué estoy buscando").
  function limpiarFiltros() {
    $filtros.q = ''
    $filtros.pais = ''
    $filtros.categoria = ''
    $filtros.calidad = ''
    $filtros.soloFavoritos = false
  }

  let hayFiltrosActivos = $derived(
    !!($filtros.q || $filtros.pais || $filtros.categoria || $filtros.calidad || $filtros.soloFavoritos),
  )
</script>

<div class="barra-acciones">
  <div class="fila-controles">
    <button
      type="button"
      class:activo={$filtros.soloFavoritos}
      aria-pressed={$filtros.soloFavoritos}
      onclick={() => ($filtros.soloFavoritos = !$filtros.soloFavoritos)}
    >{t('accion.favoritos')}</button>
    <button type="button" onclick={alAleatorio}>{t('accion.aleatorio')}</button>
    <div class="vista">
      <button
        type="button"
        class:activo={$filtros.vista === 'rejilla'}
        aria-pressed={$filtros.vista === 'rejilla'}
        onclick={() => ($filtros.vista = 'rejilla')}
      >{t('accion.rejilla')}</button>
      <button
        type="button"
        class:activo={$filtros.vista === 'lista'}
        aria-pressed={$filtros.vista === 'lista'}
        onclick={() => ($filtros.vista = 'lista')}
      >{t('accion.lista')}</button>
    </div>
  </div>

  {#if hayFiltrosActivos}
    <div class="fila-chips">
      <ChipsFiltro />
      <button type="button" class="limpiar" onclick={limpiarFiltros}>{t('filtro.limpiar')}</button>
    </div>
  {/if}

  <div class="resumen">
    <p class="total">{t('catalogo.total', { n: total })}</p>
    <!-- Dedup (Tarea 7, limpieza menor): 'vivo' repetiría literalmente el
         texto que IndicadorSenal ya dice en la cabecera ("Comprobado en
         vivo") — decirlo dos veces no añade información. 'instantanea'
         ("Comprobado hace N h") sí aporta algo que la cabecera no dice, así
         que ese caso se conserva. -->
    {#if frescura && frescura.tipo !== 'vivo'}<Frescura {frescura} />{/if}
  </div>
</div>

<style>
  .barra-acciones {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px 0;
  }
  .fila-controles {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .fila-controles button {
    background: var(--surface-raised);
    color: var(--text-body);
    border: 1px solid var(--border-default);
    border-radius: 6px;
    padding: 6px 10px;
    cursor: pointer;
  }
  /* Ámbar = filtro/vista activo — misma regla que el resto de la app
     (BarraLateralFacetas: .fila[aria-pressed='true']). */
  .fila-controles button.activo {
    border-color: var(--amber-500);
    color: var(--amber-500);
  }
  .fila-controles .vista {
    display: flex;
    gap: 4px;
  }

  .fila-chips {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  .limpiar {
    background: none;
    border: none;
    color: var(--text-accent);
    cursor: pointer;
    font: var(--type-mono-label, inherit);
    padding: 4px 8px;
  }

  .resumen {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  .total {
    color: var(--text-muted);
    font-size: 13px;
    margin: 0;
  }
</style>
