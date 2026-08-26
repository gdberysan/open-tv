<script lang="ts">
  import { t } from '../i18n'

  // Fix round 1 (Tarea 6, P0.7): estado visible mientras App SONDEA el
  // catálogo tras añadir una fuente — a diferencia de Sincronizando.svelte
  // (que hace polling de /health, la sincronización INICIAL del catálogo
  // entero), aquí el catálogo ya está "listo" en general; lo que falta es
  // que el backend termine de descargar y volcar ESTA fuente concreta. Dos
  // variantes en un solo componente, sin togglear el nodo raíz entero (mismo
  // principio que Sincronizando/MensajeError): `agotado` cambia el texto y
  // añade "Reintentar" sin desmontar/remontar nada.
  //
  // Fix final-review (F2): "Gestionar fuentes" es la SEGUNDA acción del
  // estado agotado — sin ella, "Reintentar" era la única salida y una fuente
  // realmente rota (typo en la URL, lista vacía) dejaba a quien la añadió
  // repitiendo el mismo timeout de ~90s para siempre, sin forma de llegar a
  // la vista de gestión a borrarla. alGestionarFuentes (ver App.svelte,
  // irAGestionarFuentes) limpia sondeoAgotado Y abre #fuentes en el mismo
  // gesto.
  let { agotado, alReintentar, alGestionarFuentes }: {
    agotado: boolean
    alReintentar: () => void
    alGestionarFuentes: () => void
  } = $props()
</script>

<!-- Sin aria-live propio: la transición la anuncia la región polite
     PERSISTENTE de App (ver el comentario de mensajePoliteAccesible en
     App.svelte) — una región aquí duplicaría el anuncio. -->
<div class="sincronizando-fuente">
  {#if agotado}
    <h2>{t('onboarding.sondeo.titulo')}</h2>
    <p>{t('onboarding.sondeo.agotado')}</p>
    <div class="acciones-agotado">
      <button type="button" class="reintentar" onclick={alReintentar}>
        {t('onboarding.sondeo.reintentar')}
      </button>
      <button type="button" class="gestionar" onclick={alGestionarFuentes}>
        {t('onboarding.sondeo.gestionar')}
      </button>
    </div>
  {:else}
    <h2>{t('onboarding.sondeo.titulo')}</h2>
    <p>{t('onboarding.sondeo.detalle')}</p>
    <!-- Misma barra indeterminada de Sincronizando.svelte: role="progressbar"
         SIN aria-valuenow (no hay ningún porcentaje real que reportar), el
         barrido es decorativo (aria-hidden) y se para bajo
         prefers-reduced-motion. -->
    <div class="barra" role="progressbar" aria-label={t('onboarding.sondeo.titulo')}>
      <div class="barra-relleno" aria-hidden="true"></div>
    </div>
  {/if}
</div>

<style>
  .sincronizando-fuente {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    text-align: center;
    padding: var(--space-6, 3rem) var(--space-4, 1rem);
    color: var(--text-body);
  }
  h2 { margin: 0; color: var(--text-strong); }
  p { margin: 0; color: var(--text-muted); max-width: 32rem; }

  .barra {
    width: 100%;
    max-width: 16rem;
    height: 6px;
    margin-top: var(--space-2, 0.5rem);
    border-radius: 999px;
    background: var(--graphite-600);
    overflow: hidden;
  }
  .barra-relleno {
    width: 40%;
    height: 100%;
    border-radius: inherit;
    background: var(--amber-500);
    animation: barrido-fuente 1.4s var(--ease-in-out) infinite;
  }
  @keyframes barrido-fuente {
    0%   { transform: translateX(-100%); }
    100% { transform: translateX(350%); }
  }
  @media (prefers-reduced-motion: reduce) {
    .barra-relleno {
      width: 100%;
      animation: none;
      transform: none;
    }
  }

  /* Fix final-review (F2): dos acciones en fila — "Reintentar" sigue siendo
     la única CTA ámbar (constraint global); "Gestionar fuentes" es
     secundaria, mismo estilo neutro que .fuentes-link/.idioma en App.svelte. */
  .acciones-agotado {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: var(--space-2, 8px);
  }
  .reintentar {
    margin-top: var(--space-2, 0.5rem);
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md, 8px);
    padding: 6px 16px;
    color: var(--amber-500);
    cursor: pointer;
    font: inherit;
  }
  .gestionar {
    margin-top: var(--space-2, 0.5rem);
    background: none;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    padding: 6px 16px;
    color: var(--text-body);
    cursor: pointer;
    font: inherit;
  }
</style>
