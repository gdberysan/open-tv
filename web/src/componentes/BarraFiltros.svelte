<script lang="ts">
  import type { Faceta } from '../datos/catalogo'
  import { filtros, type ModoVista } from '../estado/filtros'
  import { debounce } from '../lib/debounce'
  import { t } from '../i18n'

  // Los facetas vienen de App (el único que habla con CatalogSource); este
  // componente solo las pinta y escribe en el store de filtros.
  let { paises, categorias, alAleatorio }: {
    paises: Faceta[]
    categorias: Faceta[]
    alAleatorio: () => void
  } = $props()

  let busqueda = $state($filtros.q ?? '')

  // Sin el debounce, cada tecla dispararía una consulta al catálogo.
  const buscarConRetardo = debounce((valor: string) => {
    $filtros.q = valor
  }, 300)

  function alEscribir(evento: Event) {
    busqueda = (evento.target as HTMLInputElement).value
    buscarConRetardo(busqueda)
  }

  function elegirVista(vista: ModoVista) {
    $filtros.vista = vista
  }
</script>

<div class="barra">
  <input
    type="search"
    class="buscar"
    placeholder={t('catalogo.buscar')}
    aria-label={t('catalogo.buscar')}
    value={busqueda}
    oninput={alEscribir}
  />

  <select aria-label={t('filtro.pais')} bind:value={$filtros.pais}>
    <option value="">{t('filtro.todos')}</option>
    {#each paises as f (f.valor)}
      <option value={f.valor}>{f.valor} ({f.total})</option>
    {/each}
  </select>

  <select aria-label={t('filtro.categoria')} bind:value={$filtros.categoria}>
    <option value="">{t('filtro.todos')}</option>
    {#each categorias as f (f.valor)}
      <option value={f.valor}>{f.valor} ({f.total})</option>
    {/each}
  </select>

  <select aria-label={t('filtro.calidad')} bind:value={$filtros.calidad}>
    <option value="">{t('filtro.todos')}</option>
    <option value="hd">{t('filtro.calidad.hd')}</option>
    <option value="fhd">{t('filtro.calidad.fhd')}</option>
    <option value="4k">{t('filtro.calidad.4k')}</option>
  </select>

  <label class="offline">
    <input type="checkbox" bind:checked={$filtros.mostrarOffline} />
    {t('filtro.mostrarOffline')}
  </label>

  <button
    type="button"
    class="favoritos"
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
      onclick={() => elegirVista('rejilla')}
    >{t('accion.rejilla')}</button>
    <button
      type="button"
      class:activo={$filtros.vista === 'lista'}
      aria-pressed={$filtros.vista === 'lista'}
      onclick={() => elegirVista('lista')}
    >{t('accion.lista')}</button>
  </div>
</div>

<style>
  .barra { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; padding: 8px 0; }
  .buscar {
    flex: 1 1 200px; background: var(--surface-sunken); color: var(--text-body);
    border: 1px solid var(--border-default); border-radius: 6px; padding: 6px 10px;
  }
  select {
    background: var(--surface-sunken); color: var(--text-body);
    border: 1px solid var(--border-default); border-radius: 6px; padding: 6px 8px;
  }
  .offline { display: flex; align-items: center; gap: 4px; font-size: 13px; color: var(--text-muted); }
  button {
    background: var(--surface-raised); color: var(--text-body); border: 1px solid var(--border-default);
    border-radius: 6px; padding: 6px 10px; cursor: pointer;
  }
  /* Ámbar = filtro activo. Un botón neutro no lleva ámbar. */
  button.activo { border-color: var(--amber-500); color: var(--amber-500); }
  .vista { display: flex; gap: 4px; }
</style>
