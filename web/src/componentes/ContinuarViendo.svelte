<script lang="ts">
  import { historial, type EntradaHistorial } from '../estado/historial'
  import { t } from '../i18n'
  import LogoCanal from './LogoCanal.svelte'

  // Tarea 10 (P0.6): héroe compacto de "continuar viendo", encima de
  // BarraAcciones. Consume el store `historial` (Tarea 9) directamente — no
  // recibe canales por prop porque el historial vive fuera del ciclo de
  // consulta del catálogo (sobrevive a un cambio de filtro, a un F5, a un
  // canal que ya no está en la página cargada). alAbrir recibe la
  // EntradaHistorial tal cual: el store solo guarda id/nombre/logo/cuando,
  // no el Canal completo (eso obligaría a guardar todo el catálogo en
  // localStorage); quien llama (App) decide cómo reconstruir un Canal
  // reproducible a partir del id.
  let { alAbrir }: { alAbrir: (entrada: EntradaHistorial) => void } = $props()

  // 3–4 miniaturas recientes, SIN contar la más reciente (que ya se muestra
  // en grande arriba) — mostrarla dos veces sería ruido, no información.
  const TOPE_MINIATURAS = 4
  let miniaturas = $derived($historial.slice(1, 1 + TOPE_MINIATURAS))
</script>

{#if $historial.length}
  <section class="continuar-viendo" aria-label={t('historial.titulo')}>
    <p class="eyebrow"><span aria-hidden="true">// </span>{t('historial.titulo')}</p>

    <div class="fila">
      <button type="button" class="principal" onclick={() => alAbrir($historial[0])}>
        <span class="logo-principal">
          <LogoCanal logoUrl={$historial[0].logoUrl ?? ''} nombre={$historial[0].nombre} />
        </span>
        <span class="texto-principal">
          <span class="nombre-principal">{$historial[0].nombre}</span>
          <span class="cta">▶ {t('historial.seguir')}</span>
        </span>
      </button>

      {#if miniaturas.length}
        <div class="miniaturas">
          {#each miniaturas as entrada (entrada.canalId)}
            <button
              type="button"
              class="miniatura"
              onclick={() => alAbrir(entrada)}
              aria-label={entrada.nombre}
              title={entrada.nombre}
            >
              <LogoCanal logoUrl={entrada.logoUrl ?? ''} nombre={entrada.nombre} />
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <button type="button" class="borrar" onclick={() => historial.borrar()}>
      {t('historial.borrar')}
    </button>
  </section>
{/if}

<style>
  /* Compacto a propósito (brief): la fila principal más las miniaturas no
     puede empujar la rejilla más de ~1 fila de tarjetas. Alturas fijas
     pequeñas (44px/40px) en vez de aspect-ratio como en TarjetaCanal, que
     ahí sí puede crecer con el ancho de la rejilla. */
  .continuar-viendo {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px 12px;
    margin-bottom: 8px;
    background: var(--surface-card);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
  }

  .eyebrow {
    margin: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    text-transform: uppercase;
    color: var(--text-muted);
  }

  .fila {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }

  .principal {
    all: unset;
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    min-width: 0;
  }
  .logo-principal {
    width: 44px;
    height: 44px;
    flex-shrink: 0;
    border-radius: 6px;
    overflow: hidden;
  }
  .texto-principal {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .nombre-principal {
    color: var(--text-strong);
    font: var(--type-label, inherit);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Ámbar = única CTA activa de este componente (constraint global: ámbar
     solo para el CTA/estado activo). */
  .cta {
    color: var(--amber-500);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
  }

  .miniaturas {
    display: flex;
    gap: 10px;
  }
  .miniatura {
    all: unset;
    cursor: pointer;
    width: 64px;
    height: 64px;
    border-radius: 6px;
    overflow: hidden;
    border: 1px solid var(--border-default);
    flex-shrink: 0;
  }
  .miniatura:hover {
    border-color: var(--border-strong);
  }

  /* Discreto (brief): ni ámbar ni negrita — una acción destructiva secundaria,
     no la CTA de la fila. */
  .borrar {
    all: unset;
    align-self: flex-start;
    cursor: pointer;
    color: var(--text-muted);
    font-size: 11px;
    text-decoration: underline;
  }
  .borrar:hover {
    color: var(--text-body);
  }
</style>
