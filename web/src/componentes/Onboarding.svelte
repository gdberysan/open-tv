<script lang="ts">
  import type { CatalogSource, Fuente } from '../datos/catalogo'
  import { t } from '../i18n'
  import AnadirFuente from './AnadirFuente.svelte'

  // Tarea 6 (P0.7): primer arranque sin fuentes. App decide CUÁNDO mostrar
  // este componente (derivado `sinFuentes`, ver App.svelte); este componente
  // solo sabe pedirle a `fuente` que añada una fuente y avisar hacia arriba
  // con la Fuente ya creada — el mismo patrón que el resto del cliente: un
  // único punto habla con CatalogSource desde fuera (Reproductor es la otra
  // excepción documentada), aquí se reutiliza el que ya recibe App.
  //
  // Tarea 7 (P0.7, DRY): el formulario en sí (URL/fichero/sugeridas) vive
  // ahora en AnadirFuente.svelte, compartido con la vista Fuentes (gestión).
  // Este componente queda como envoltorio: título, copy de bienvenida y
  // disclaimer — el marco que solo tiene sentido en el primer arranque.
  let { fuente, alFuenteAnadida }: {
    fuente: CatalogSource
    alFuenteAnadida: (f: Fuente) => void
  } = $props()
</script>

<!-- Sin aria-live propio (brief, Tarea 6): la transición "fuente añadida →
     sincronizando" la cubre la región polite PERSISTENTE que ya vive en
     App.svelte (su `mensajePoliteAccesible` ya incluye fase.tipo ===
     'sincronizando', que es justo lo que App fuerza en cuanto esta fuente
     llama a `alFuenteAnadida` — ver el comentario de esa función en
     App.svelte). -->
<section class="onboarding" aria-labelledby="onboarding-titulo">
  <h2 id="onboarding-titulo">{t('onboarding.titulo')}</h2>
  <p class="copy">{t('onboarding.copy')}</p>

  <!-- enfocarAlMontar (Tarea 7): SOLO aquí — es el primer control con el que
       alguien que llega sin fuentes va a interactuar. -->
  <AnadirFuente {fuente} {alFuenteAnadida} enfocarAlMontar={true} />

  <p class="disclaimer">{t('onboarding.disclaimer')}</p>
</section>

<style>
  .onboarding {
    display: flex;
    flex-direction: column;
    gap: var(--space-4, 1rem);
    max-width: 34rem;
    margin: var(--space-6, 2rem) auto;
    text-align: center;
  }
  h2 {
    margin: 0;
    color: var(--text-strong);
    font: var(--type-h4, inherit);
  }
  .copy {
    margin: 0;
    color: var(--text-muted);
  }

  .disclaimer {
    margin: 0;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
