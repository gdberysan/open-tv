<script lang="ts">
  import { t } from '../i18n'
  // Solo se pinta cuando el veredicto es un NO explícito. null es "todavía no
  // lo sé" y marcar eso asustaría sin motivo: antes de la primera pasada del
  // health-worker, todo el catálogo es null.
  let { webOk }: { webOk: boolean | null } = $props()
</script>

{#if webOk === false}
  <span class="marca" title={t('canal.soloApp')} aria-label={t('canal.soloApp')}>APP</span>
{/if}

<style>
  .marca {
    font-size: 10px; letter-spacing: .05em; padding: 1px 4px; border-radius: 3px;
    background: var(--graphite-600); color: var(--text-body);
  }
</style>
