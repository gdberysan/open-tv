<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte'
  import { t } from '../i18n'
  import { preferencias } from '../estado/preferencias'
  import PieDeMarca from './PieDeMarca.svelte'

  // Tarea 6 (P0.8): vista de ajustes (#ajustes) — mismo patrón de hash y de
  // foco que Fuentes.svelte (Tarea 7, P0.7). Densidad y «recordar vista/
  // filtros» solo LEEN Y ESCRIBEN `preferencias` (mismo store que la Tarea 5
  // ya cablea a RejillaVirtual) — la lógica de QUÉ hacer con recordarVista/
  // recordarFiltros (restaurar al arrancar, persistir vista/filtros) vive en
  // App.svelte, no aquí: este componente no sabe nada de `filtros` ni de
  // localStorage de sesión.
  //
  // version llega como prop en vez de que este componente haga su propio
  // fetch a /health: App ya llama a consultarSalud() al arrancar (gobierna
  // `fase`) — reenviar ese mismo resultado evita una segunda llamada de red
  // solo para pintar un número.
  // alAbrirStats (opcional, con no-op de respaldo): el enlace a «Estadísticas
  // locales» vivía en el pie de página global (App.svelte) — se traslada
  // aquí porque ahí competía por el mismo espacio vertical que la rejilla del
  // escenario. Sigue siendo una vista secundaria/de depuración, no una
  // función primaria como Fuentes (ver el comentario de la cabecera en
  // App.svelte), así que su sitio natural es Ajustes, no la cabecera.
  // Opcional con respaldo, mismo criterio que focoActivo en TarjetaCanal:
  // no romper a nadie que use esta vista suelta (tests existentes incluidos).
  let { alVolver, version, alAbrirStats = () => {} }: {
    alVolver: () => void
    version: string
    alAbrirStats?: () => void
  } = $props()

  // Mismo mecanismo de foco que Fuentes.svelte (Tarea 11, P0.7): al montar,
  // el foco se mueve al título de esta vista (la cabecera que la abre nunca
  // se desmonta, así que sin esto un lector de pantalla no nota el cambio);
  // al desmontar, vuelve a quien la abrió.
  let elementoPrevio: HTMLElement | null = null
  let tituloEl = $state<HTMLHeadingElement | undefined>(undefined)

  onMount(() => {
    elementoPrevio = document.activeElement instanceof HTMLElement ? document.activeElement : null
    tick().then(() => tituloEl?.focus())
  })

  onDestroy(() => {
    if (elementoPrevio && document.body.contains(elementoPrevio)) elementoPrevio.focus()
  })
</script>

<section class="ajustes" aria-labelledby="ajustes-titulo">
  <button type="button" class="volver" onclick={alVolver}>{t('ajustes.volver')}</button>
  <h2 id="ajustes-titulo" bind:this={tituloEl} tabindex="-1">{t('ajustes.titulo')}</h2>

  <div class="bloque">
    <h3 class="subtitulo">{t('ajustes.densidad.titulo')}</h3>
    <!-- Segmentado ámbar=activo, mismo patrón que el conmutador rejilla/lista
         de BarraAcciones.svelte: dos botones aria-pressed, nunca un <select>
         para una elección binaria visible de un vistazo. -->
    <div class="segmentado" role="group" aria-label={t('ajustes.densidad.titulo')}>
      <button
        type="button"
        class:activo={$preferencias.densidad === 'comoda'}
        aria-pressed={$preferencias.densidad === 'comoda'}
        onclick={() => ($preferencias.densidad = 'comoda')}
      >
        {t('ajustes.densidad.comoda')}
      </button>
      <button
        type="button"
        class:activo={$preferencias.densidad === 'compacta'}
        aria-pressed={$preferencias.densidad === 'compacta'}
        onclick={() => ($preferencias.densidad = 'compacta')}
      >
        {t('ajustes.densidad.compacta')}
      </button>
    </div>
  </div>

  <div class="bloque">
    <label class="linea-toggle">
      <input type="checkbox" bind:checked={$preferencias.recordarVista} />
      {t('ajustes.recordarVista.titulo')}
    </label>
    <p class="ayuda">{t('ajustes.recordarVista.ayuda')}</p>
  </div>

  <div class="bloque">
    <label class="linea-toggle">
      <input type="checkbox" bind:checked={$preferencias.recordarFiltros} />
      {t('ajustes.recordarFiltros.titulo')}
    </label>
    <p class="ayuda">{t('ajustes.recordarFiltros.ayuda')}</p>
  </div>

  <div class="bloque">
    <button type="button" class="ver-stats" onclick={alAbrirStats}>{t('pie.stats')}</button>
  </div>

  <!-- Acerca de: versión real (de /health, vía App), el mismo aviso legal
       completo que Fuentes.svelte (Tarea 8, P0.7) y el mismo crédito de
       marca que el pie de App (Tarea 10, P0.7) — nunca un texto nuevo
       inventado para esta vista. Sin aria-live (brief): es contenido
       estático, no un anuncio de estado. -->
  <div class="bloque acerca">
    <h3 class="subtitulo">{t('ajustes.acerca.titulo')}</h3>
    <p class="version">{t('ajustes.acerca.version', { version })}</p>
    <div class="legal">
      <h4 class="titulo-legal">{t('fuentes.legal.titulo')}</h4>
      <p>{t('fuentes.legal.reproductor')}</p>
      <p>{t('fuentes.legal.responsabilidad')}</p>
      <p>{t('fuentes.legal.sinDrm')}</p>
      <p>{t('fuentes.legal.sugeridas')}</p>
    </div>
    <PieDeMarca />
  </div>
</section>

<style>
  .ajustes {
    display: flex;
    flex-direction: column;
    gap: var(--space-4, 1rem);
    max-width: 34rem;
    margin: 0 auto;
    /* Entrada de panel (Tarea 7, P0.8): monta una vez (App.svelte la mete/
       saca con un {#if vistaAjustes}), nunca en un bucle — no es la rejilla
       virtualizada, así que un @keyframes aquí es barato. opacity+
       translateY(pequeño) son compositables en GPU. */
    animation: entrada-panel var(--dur-base) var(--ease-out);
  }
  @keyframes entrada-panel {
    from { opacity: 0; transform: translateY(var(--space-1, 4px)); }
    to { opacity: 1; transform: translateY(0); }
  }
  .volver {
    all: unset;
    cursor: pointer;
    align-self: flex-start;
    color: var(--text-muted);
    font-size: 13px;
  }
  .volver:hover { color: var(--text-body); }
  h2 { margin: 0; color: var(--text-strong); }

  .bloque {
    display: flex;
    flex-direction: column;
    gap: 8px;
    background: var(--surface-card);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    padding: var(--space-3, 0.75rem) var(--space-4, 1rem);
  }
  .subtitulo {
    margin: 0;
    color: var(--text-body);
    font-size: 15px;
  }

  /* Ámbar = elección activa — misma regla que el resto de la app (constraint
     global: ámbar solo para el acento/acción activa). */
  .segmentado {
    display: inline-flex;
    gap: 6px;
  }
  .segmentado button {
    background: var(--surface-raised);
    color: var(--text-body);
    border: 1px solid var(--border-default);
    border-radius: 6px;
    padding: 6px 12px;
    cursor: pointer;
    font: inherit;
  }
  .segmentado button.activo {
    border-color: var(--amber-500);
    color: var(--amber-500);
  }

  .ver-stats {
    all: unset;
    cursor: pointer;
    align-self: flex-start;
    color: var(--text-body);
    text-decoration: underline;
  }
  .ver-stats:hover {
    color: var(--amber-500);
  }

  .linea-toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text-body);
    cursor: pointer;
  }
  .ayuda {
    margin: 0;
    color: var(--text-muted);
    font-size: 12px;
    line-height: 1.5;
  }

  .acerca .version {
    margin: 0;
    color: var(--text-muted);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
  }
  .legal {
    display: flex;
    flex-direction: column;
    gap: 4px;
    border-top: 1px solid var(--border-default);
    padding-top: var(--space-3, 0.75rem);
  }
  .titulo-legal {
    margin: 0 0 4px;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    text-transform: uppercase;
    color: var(--text-faint);
  }
  .legal p {
    margin: 0;
    color: var(--text-faint);
    font-size: 12px;
    line-height: 1.5;
  }

  @media (prefers-reduced-motion: reduce) {
    .ajustes { animation: none; }
  }
</style>
