<script lang="ts">
  import { filtros } from '../estado/filtros'
  import { t } from '../i18n'

  // Tarea 13 (P0.6): el vacío estático («Ningún canal casa con el filtro.»)
  // no dice qué hacer — el usuario tiene que adivinar cuál de sus filtros lo
  // dejó sin nada. Este componente deriva UNA sugerencia concreta a partir
  // del mismo store `filtros` que ya gobierna facetas (BarraLateralFacetas),
  // chips (ChipsFiltro) y "Limpiar filtros" (BarraAcciones): nunca un
  // estado paralelo.
  //
  // Heurística de "dimensión más restrictiva", en este orden:
  //   calidad > categoría > país > q
  // - calidad suele ser el filtro más estrecho: "4K" descarta la inmensa
  //   mayoría del catálogo de una sola vez.
  // - categoría y país acotan por facetas más anchas (decenas de valores
  //   posibles, no un puñado).
  // - q (texto libre) va última a propósito: aunque puede ser muy
  //   restrictiva, quitarla primero sería descartar a ciegas justo lo que
  //   el usuario escribió que quería buscar — se ofrece, pero solo si
  //   ninguna faceta más "candidata a error" explica ya el vacío.
  const ETIQUETAS_CALIDAD: Record<string, () => string> = {
    hd: () => t('filtro.calidad.hd'),
    fhd: () => t('filtro.calidad.fhd'),
    '4k': () => t('filtro.calidad.4k'),
  }
  function etiquetaCalidad(valor: string): string {
    return ETIQUETAS_CALIDAD[valor]?.() ?? valor
  }

  interface Sugerencia {
    texto: string
    quitar: () => void
  }

  let sugerencia = $derived.by((): Sugerencia | null => {
    if ($filtros.calidad) {
      const valor = etiquetaCalidad($filtros.calidad)
      return { texto: t('vacio.sugerencia.quitar', { valor }), quitar: () => ($filtros.calidad = '') }
    }
    if ($filtros.categoria) {
      const valor = $filtros.categoria
      return { texto: t('vacio.sugerencia.quitar', { valor }), quitar: () => ($filtros.categoria = '') }
    }
    if ($filtros.pais) {
      const valor = $filtros.pais
      return { texto: t('vacio.sugerencia.quitar', { valor }), quitar: () => ($filtros.pais = '') }
    }
    if ($filtros.q) {
      const valor = $filtros.q
      return { texto: t('vacio.sugerencia.quitarBusqueda', { valor }), quitar: () => ($filtros.q = '') }
    }
    return null
  })

  // Caso límite (brief, Tarea 13): ninguna de las cuatro dimensiones de
  // arriba está activa y aun así no hay resultados — típicamente porque
  // mostrarOffline=false (el valor por defecto) oculta canales muertos y
  // TODOS los que sobreviven al resto de filtros (o al catálogo entero)
  // están muertos. soloFavoritos con cero favoritos cae también aquí: no
  // tiene una dimensión propia en la heurística, pero "Limpiar filtros" (ver
  // abajo) sí lo resetea.
  let ofrecerMostrarOffline = $derived(!sugerencia && !$filtros.mostrarOffline)

  // Mismo reseteo que BarraAcciones.limpiarFiltros (duplicado a propósito,
  // como ya hace ChipsFiltro con sus etiquetas: tres líneas, no vale la pena
  // acoplar dos componentes de UI a un módulo compartido por esto). Las
  // CUATRO dimensiones de consulta más soloFavoritos; mostrarOffline queda
  // fuera porque aquí ya tiene su propio botón dedicado arriba.
  function limpiarFiltros() {
    $filtros.q = ''
    $filtros.pais = ''
    $filtros.categoria = ''
    $filtros.calidad = ''
    $filtros.soloFavoritos = false
  }
</script>

<!-- Sin aria-live/role="status" propio (mismo patrón que Sincronizando.svelte
     y MensajeError.svelte): es un bloque visual, no una región que se
     anuncia a sí misma. Ver el informe de la Tarea 13 sobre si esta
     transición concreta queda cubierta por las regiones aria-live
     persistentes de App.svelte. -->
<div class="vacio">
  <p class="mensaje">{t('catalogo.vacio')}</p>
  <div class="acciones">
    {#if sugerencia}
      <button type="button" class="sugerida" onclick={sugerencia.quitar}>{sugerencia.texto}</button>
    {:else if ofrecerMostrarOffline}
      <button type="button" class="sugerida" onclick={() => ($filtros.mostrarOffline = true)}>
        {t('filtro.mostrarOffline')}
      </button>
    {/if}
    <button type="button" class="limpiar" onclick={limpiarFiltros}>{t('filtro.limpiar')}</button>
  </div>
</div>

<style>
  .vacio {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    text-align: center;
    padding: 24px 0;
  }
  .mensaje {
    color: var(--text-muted);
    margin: 0;
  }
  .acciones {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }
  /* Ámbar = la sugerencia derivada, la CTA primaria de este estado — mismo
     tratamiento "relleno" que .chip en ChipsFiltro (faceta activa). */
  .sugerida {
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md, 8px);
    padding: 6px 12px;
    color: var(--amber-500);
    cursor: pointer;
    font: inherit;
  }
  /* "Limpiar filtros" es la acción secundaria: mismo texto-enlace que el
     botón homónimo de BarraAcciones, sin relleno. */
  .limpiar {
    background: none;
    border: none;
    color: var(--text-accent);
    cursor: pointer;
    font: var(--type-mono-label, inherit);
    padding: 6px 12px;
  }
</style>
