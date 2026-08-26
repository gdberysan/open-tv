<script lang="ts">
  import { untrack } from 'svelte'
  import type { Faceta } from '../datos/catalogo'
  import { filtros } from '../estado/filtros'
  import { debounce } from '../lib/debounce'
  import { t } from '../i18n'

  // Los facetas vienen de App (el único que habla con CatalogSource); este
  // componente solo las pinta y escribe en el store de filtros. Reemplaza a
  // BarraFiltros.svelte (Tarea 6, P0.6): el buscador y los tres desplegables
  // migran aquí como la barra lateral etiquetada del aside.
  let { paises, categorias, calidades }: {
    paises: Faceta[]
    categorias: Faceta[]
    calidades: Faceta[]
  } = $props()

  let busqueda = $state($filtros.q ?? '')

  // Último valor de `q` que ESTE componente empujó al store (al disparar el
  // debounce). Distingue el eco de nuestro propio empuje de un cambio que
  // llegó de fuera (chip "q", "Quitar «busqueda»", "Limpiar filtros"): ver el
  // $effect de más abajo.
  let ultimoEmpujado = $state($filtros.q ?? '')

  // Mismo debounce que usaba BarraFiltros: sin él, cada tecla dispararía una
  // consulta al catálogo.
  const buscarConRetardo = debounce((valor: string) => {
    $filtros.q = valor
    ultimoEmpujado = valor
  }, 300)

  function alEscribir(evento: Event) {
    busqueda = (evento.target as HTMLInputElement).value
    buscarConRetardo(busqueda)
  }

  // Re-sincroniza `busqueda` cuando `filtros.q` cambia DESDE FUERA. Un
  // `$effect(() => busqueda = $filtros.q)` a secas borraría lo que el usuario
  // está tecleando dentro de la ventana de debounce (300ms): cada tecla no
  // toca el store todavía, así que un efecto ingenuo pisaría el input con el
  // `q` viejo del store en cada re-render. Comparando contra `ultimoEmpujado`
  // solo reaccionamos cuando el cambio NO fue el eco de nuestro propio
  // empuje. `untrack` evita que la escritura a `ultimoEmpujado` haga que este
  // mismo efecto se re-dispare por leerse a sí mismo.
  $effect(() => {
    const qActual = $filtros.q ?? ''
    if (qActual !== untrack(() => ultimoEmpujado)) {
      busqueda = qActual
      ultimoEmpujado = qActual
    }
  })

  // Alternar: pulsar la faceta ya seleccionada la limpia (vuelve a '').
  function alternarPais(valor: string) {
    $filtros.pais = $filtros.pais === valor ? '' : valor
  }
  function alternarCategoria(valor: string) {
    $filtros.categoria = $filtros.categoria === valor ? '' : valor
  }
  function alternarCalidad(valor: string) {
    $filtros.calidad = $filtros.calidad === valor ? '' : valor
  }

  // Las calidades llegan como valores crudos ('hd'/'fhd'/'4k'/…) desde el
  // catálogo; si hay una etiqueta traducida conocida se usa esa, si no el
  // valor tal cual (nunca deja una fila sin texto por una calidad futura).
  const ETIQUETAS_CALIDAD: Record<string, () => string> = {
    hd: () => t('filtro.calidad.hd'),
    fhd: () => t('filtro.calidad.fhd'),
    '4k': () => t('filtro.calidad.4k'),
  }
  function etiquetaCalidad(valor: string): string {
    return ETIQUETAS_CALIDAD[valor]?.() ?? valor
  }

  // País puede tener decenas de entradas: colapsado por defecto, con un botón
  // real para expandir la lista completa.
  const PAISES_VISIBLES = 8
  let paisesExpandido = $state(false)
  let paisesAMostrar = $derived(paisesExpandido ? paises : paises.slice(0, PAISES_VISIBLES))
  let hayMasPaises = $derived(paises.length > PAISES_VISIBLES)
</script>

<div class="barra-lateral">
  <div class="buscador">
    <!-- Motivo ámbar, puramente decorativo (aria-hidden): el "<" evoca el
         prompt de una sala de control, no un dato. -->
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

  <section class="grupo" role="group" aria-labelledby="titulo-pais">
    <h3 id="titulo-pais" class="titulo-grupo">{t('filtro.pais')}</h3>
    <div class="lista">
      {#each paisesAMostrar as f (f.valor)}
        <button
          type="button"
          class="fila"
          aria-pressed={$filtros.pais === f.valor}
          onclick={() => alternarPais(f.valor)}
        >
          <span class="valor">{f.valor}</span>
          <span class="conteo">{f.total}</span>
        </button>
      {/each}
    </div>
    {#if hayMasPaises}
      <button
        type="button"
        class="expandir"
        aria-expanded={paisesExpandido}
        onclick={() => (paisesExpandido = !paisesExpandido)}
      >
        {paisesExpandido ? t('filtro.verMenosPaises') : t('filtro.verPaises', { n: paises.length })}
      </button>
    {/if}
  </section>

  <section class="grupo" role="group" aria-labelledby="titulo-categoria">
    <h3 id="titulo-categoria" class="titulo-grupo">{t('filtro.categoria')}</h3>
    <div class="lista">
      {#each categorias as f (f.valor)}
        <button
          type="button"
          class="fila"
          aria-pressed={$filtros.categoria === f.valor}
          onclick={() => alternarCategoria(f.valor)}
        >
          <span class="valor">{f.valor}</span>
          <span class="conteo">{f.total}</span>
        </button>
      {/each}
    </div>
  </section>

  <section class="grupo" role="group" aria-labelledby="titulo-calidad">
    <h3 id="titulo-calidad" class="titulo-grupo">{t('filtro.calidad')}</h3>
    <div class="lista">
      {#each calidades as f (f.valor)}
        <button
          type="button"
          class="fila"
          aria-pressed={$filtros.calidad === f.valor}
          onclick={() => alternarCalidad(f.valor)}
        >
          <span class="valor">{etiquetaCalidad(f.valor)}</span>
          <span class="conteo">{f.total}</span>
        </button>
      {/each}
    </div>
  </section>

  <section class="grupo" role="group" aria-labelledby="titulo-senal">
    <h3 id="titulo-senal" class="titulo-grupo">{t('senal.titulo')}</h3>
    <div class="lista">
      <!-- "Solo señal viva" resume el estado por defecto (mostrarOffline
           false): ámbar cuando es el estado activo, como cualquier faceta
           seleccionada. Pulsarla vuelve a ese estado. -->
      <button
        type="button"
        class="fila fila-senal"
        aria-pressed={!$filtros.mostrarOffline}
        onclick={() => ($filtros.mostrarOffline = false)}
      >
        {t('senal.soloViva')}
      </button>
      <label class="fila fila-senal offline">
        <input type="checkbox" bind:checked={$filtros.mostrarOffline} />
        {t('filtro.mostrarOffline')}
      </label>
    </div>
  </section>
</div>

<style>
  .barra-lateral {
    display: flex;
    flex-direction: column;
    gap: var(--space-5, 24px);
  }

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

  .grupo {
    display: flex;
    flex-direction: column;
    gap: var(--space-2, 8px);
  }
  .titulo-grupo {
    margin: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
    text-transform: uppercase;
  }
  .lista {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .fila {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius-sm, 5px);
    padding: 4px 8px;
    color: var(--text-body);
    cursor: pointer;
    font: var(--type-body-sm, inherit);
    text-align: left;
  }
  .fila:hover {
    background: var(--surface-raised);
  }
  .fila .valor {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .fila .conteo {
    flex-shrink: 0;
    font: var(--type-mono-label, inherit);
    color: var(--text-muted);
  }
  /* Ámbar = faceta activa (única regla de color de todo el componente). */
  .fila[aria-pressed='true'] {
    background: var(--tint-amber-weak);
    border-color: var(--tint-amber-line);
  }
  .fila[aria-pressed='true'] .valor,
  .fila[aria-pressed='true'] .conteo {
    color: var(--amber-500);
  }

  .expandir {
    align-self: flex-start;
    background: none;
    border: none;
    color: var(--text-accent);
    cursor: pointer;
    font: var(--type-mono-label, inherit);
    padding: 4px 8px;
  }

  .offline {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
  }
</style>
