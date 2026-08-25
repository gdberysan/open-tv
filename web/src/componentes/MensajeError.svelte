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

<p class="error" role="alert">{mensaje}</p>

<style>
  .error { color: var(--signal-error); }
</style>
