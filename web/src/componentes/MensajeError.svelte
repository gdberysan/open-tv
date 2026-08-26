<script lang="ts">
  import { t } from '../i18n'
  import type { ClaseError } from '../estado/salud'

  // Tres problemas que se ven igual en pantalla y se arreglan distinto:
  // Open TV cerrado, sin internet, o el servidor contestando mal. El mensaje
  // nunca los mezcla — mezclarlos fue lo que costó una tarde de diagnóstico.
  let { clase }: { clase: ClaseError } = $props()

  const mensaje = $derived(
    clase === 'gateway' ? t('estado.gatewayCaido')
    : clase === 'red' ? t('estado.sinRed')
    : t('estado.errorServidor'),
  )
</script>

<!-- SIN role="alert"/aria-live (fix round 2, Tarea 18): este componente solo
     se monta desde App.svelte, que YA tiene su propia región
     aria-live="assertive" PERSISTENTE para este mismo texto (ver
     App.svelte). role="alert" se anuncia de forma muy fiable al insertarse
     — con esta semántica AQUÍ TAMBIÉN, un lector de pantalla hablaría el
     error dos veces seguidas. El texto sigue siendo visible; solo se
     retira quién lo anuncia. -->
<p class="error">{mensaje}</p>

<style>
  .error { color: var(--signal-error); }
</style>
