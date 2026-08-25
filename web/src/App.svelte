<script lang="ts">
  // Andamiaje mínimo: el catálogo, los filtros y el reproductor llegan en
  // tareas posteriores. Esto solo demuestra que los tokens y el i18n están
  // cableados de punta a punta.
  import { idioma, t } from './i18n'

  function alternarIdioma() {
    idioma.update((v) => (v === 'es' ? 'en' : 'es'))
  }
</script>

<main>
  <h1>{t('app.titulo')}</h1>
  <p>{t('app.lema')}</p>
  <button type="button" on:click={alternarIdioma}>
    {$idioma === 'es' ? t('idioma.en') : t('idioma.es')}
  </button>
</main>

<style>
  main {
    max-width: 40rem;
    margin: 0 auto;
    padding: var(--space-6, 2rem);
  }
</style>
