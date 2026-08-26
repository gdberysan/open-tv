<script lang="ts">
  import { filtros } from '../estado/filtros'
  import { t } from '../i18n'

  // Tarea 7 (P0.6): un chip por dimensión de filtro ACTIVA, derivado del
  // mismo store `filtros` que ya gobierna la barra lateral de facetas
  // (BarraLateralFacetas, Tarea 6) — nunca un estado paralelo. Cada chip es
  // un <button> real que, al pulsarse, limpia SOLO su propia dimensión.

  // Mismo mapa que BarraLateralFacetas (Tarea 6): duplicado a propósito —
  // son tres líneas de texto, no vale la pena acoplar dos componentes de UI
  // a un módulo compartido por esto.
  const ETIQUETAS_CALIDAD: Record<string, () => string> = {
    hd: () => t('filtro.calidad.hd'),
    fhd: () => t('filtro.calidad.fhd'),
    '4k': () => t('filtro.calidad.4k'),
  }
  function etiquetaCalidad(valor: string): string {
    return ETIQUETAS_CALIDAD[valor]?.() ?? valor
  }

  interface Chip {
    clave: string
    etiqueta: string
    quitarEtiqueta: string
    quitar: () => void
  }

  let chips = $derived.by((): Chip[] => {
    const lista: Chip[] = []
    if ($filtros.pais) {
      lista.push({
        clave: 'pais',
        etiqueta: $filtros.pais,
        quitarEtiqueta: t('chip.quitarFiltro', { valor: $filtros.pais }),
        quitar: () => ($filtros.pais = ''),
      })
    }
    if ($filtros.categoria) {
      lista.push({
        clave: 'categoria',
        etiqueta: $filtros.categoria,
        quitarEtiqueta: t('chip.quitarFiltro', { valor: $filtros.categoria }),
        quitar: () => ($filtros.categoria = ''),
      })
    }
    if ($filtros.calidad) {
      const etiqueta = etiquetaCalidad($filtros.calidad)
      lista.push({
        clave: 'calidad',
        etiqueta,
        quitarEtiqueta: t('chip.quitarFiltro', { valor: etiqueta }),
        quitar: () => ($filtros.calidad = ''),
      })
    }
    if ($filtros.q) {
      lista.push({
        clave: 'q',
        etiqueta: $filtros.q,
        quitarEtiqueta: t('chip.quitarBusqueda', { valor: $filtros.q }),
        quitar: () => ($filtros.q = ''),
      })
    }
    if ($filtros.soloFavoritos) {
      const etiqueta = t('accion.favoritos')
      lista.push({
        clave: 'favoritos',
        etiqueta,
        quitarEtiqueta: t('chip.quitarFiltro', { valor: etiqueta }),
        quitar: () => ($filtros.soloFavoritos = false),
      })
    }
    return lista
  })
</script>

{#if chips.length > 0}
  <div class="chips">
    {#each chips as chip (chip.clave)}
      <button type="button" class="chip" aria-label={chip.quitarEtiqueta} onclick={chip.quitar}>
        <span class="etiqueta">{chip.etiqueta}</span>
        <span class="x" aria-hidden="true">✕</span>
      </button>
    {/each}
  </div>
{/if}

<style>
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  /* Chip = faceta activa: mismo tratamiento ámbar que .fila[aria-pressed]
     en BarraLateralFacetas — ámbar es siempre "esto está filtrando ahora". */
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md, 8px);
    padding: 4px 8px;
    color: var(--amber-500);
    font: var(--type-body-sm, inherit);
    cursor: pointer;
  }
  .chip .x {
    font-size: 11px;
  }
</style>
