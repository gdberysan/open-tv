<script lang="ts">
  import type { Faceta } from '../datos/catalogo'
  import { filtros } from '../estado/filtros'
  import { banderaDePais, nombreDePais } from '../lib/paises'
  import { iconoDeCategoria } from '../lib/categorias'
  import { idioma, t } from '../i18n'

  // Versión CONDENSADA de las facetas para la barra lateral del escenario
  // (spec §3): chips del top-N por conteo, no el panel ancho de
  // BarraLateralFacetas (que sigue vivo en el modo ver-todo). Escribe en el
  // MISMO store de filtros con el MISMO contrato de alternar.
  let { paises, categorias }: { paises: Faceta[]; categorias: Faceta[] } = $props()

  const TOPE_CHIPS = 6

  // La faceta ACTIVA se cuela aunque no esté en el top: sin esto, filtrar por
  // un país minoritario dejaría el chip activo invisible e imposible de quitar
  // desde aquí.
  function top(facetas: Faceta[], activa: string): Faceta[] {
    const orden = [...facetas].sort((a, b) => b.total - a.total).slice(0, TOPE_CHIPS)
    if (activa && !orden.some((f) => f.valor === activa)) {
      const extra = facetas.find((f) => f.valor === activa)
      if (extra) orden.push(extra)
    }
    return orden
  }

  const chipsPais = $derived(top(paises, $filtros.pais ?? ''))
  const chipsCategoria = $derived(top(categorias, $filtros.categoria ?? ''))

  function alternarPais(valor: string) {
    $filtros.pais = $filtros.pais === valor ? '' : valor
  }
  function alternarCategoria(valor: string) {
    $filtros.categoria = $filtros.categoria === valor ? '' : valor
  }
</script>

<div class="facetas-compactas">
  <section class="grupo" role="group" aria-label={t('filtro.pais')}>
    {#each chipsPais as f (f.valor)}
      <!-- El aria-label EMPIEZA por el texto visible («US 1837») y añade el
           nombre localizado detrás: quien navega con lector de pantalla oye
           el país de verdad (no un ISO críptico) y quien dicta comandos por
           voz puede decir lo que VE — WCAG 2.5.3 Label in Name, cazado por
           el audit de Lighthouse del gate de reproductor-primero. -->
      <button
        type="button"
        class="chip"
        aria-pressed={$filtros.pais === f.valor}
        aria-label={`${f.valor} ${f.total}, ${nombreDePais(f.valor, idioma.actual)}`}
        onclick={() => alternarPais(f.valor)}
      >
        <span aria-hidden="true">{banderaDePais(f.valor)}</span>
        {f.valor}
        <span class="conteo">{f.total}</span>
      </button>
    {/each}
  </section>
  <section class="grupo" role="group" aria-label={t('filtro.categoria')}>
    {#each chipsCategoria as f (f.valor)}
      <button
        type="button"
        class="chip"
        aria-pressed={$filtros.categoria === f.valor}
        onclick={() => alternarCategoria(f.valor)}
      >
        <span aria-hidden="true">{iconoDeCategoria(f.valor)}</span>
        {f.valor}
        <span class="conteo">{f.total}</span>
      </button>
    {/each}
  </section>
</div>

<style>
  .facetas-compactas {
    display: flex;
    flex-direction: column;
    gap: var(--space-3, 12px);
  }
  .grupo {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 26px;
    box-sizing: border-box;
    background: var(--surface-sunken);
    border: 1px solid var(--border-default);
    border-radius: 999px;
    padding: 0 10px;
    color: var(--text-body);
    cursor: pointer;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    transition: background-color var(--dur-fast, .15s) var(--ease-out, ease),
      border-color var(--dur-fast, .15s) var(--ease-out, ease),
      color var(--dur-fast, .15s) var(--ease-out, ease);
  }
  @media (prefers-reduced-motion: reduce) {
    .chip {
      transition: none;
    }
  }
  .chip:hover {
    background: var(--surface-raised);
    border-color: var(--border-strong, var(--border-default));
  }
  .chip .conteo {
    display: inline-flex;
    align-items: center;
    height: 16px;
    padding: 0 5px;
    border-radius: 999px;
    background: var(--surface-raised);
    color: var(--text-muted);
    font-size: 10px;
    font-variant-numeric: tabular-nums;
  }
  /* Ámbar = faceta activa, misma regla que BarraLateralFacetas. */
  .chip[aria-pressed='true'] {
    background: var(--tint-amber-weak);
    border-color: var(--tint-amber-line);
    color: var(--amber-500);
  }
  .chip[aria-pressed='true'] .conteo {
    background: transparent;
    color: var(--amber-500);
  }
</style>
