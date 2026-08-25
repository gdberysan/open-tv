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

<!-- aria-live="assertive" explícito (Tarea 18): role="alert" ya implica
     assertive, pero se deja explícito por la misma razón que en
     Sincronizando.svelte — comprobable sin depender de la interpretación
     implícita de cada lector de pantalla. -->
<p class="error" role="alert" aria-live="assertive">{mensaje}</p>

<style>
  .error { color: var(--signal-error); }
</style>
