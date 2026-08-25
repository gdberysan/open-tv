<script lang="ts">
  import { t } from '../i18n'
  import type { Frescura as InfoFrescura } from '../datos/catalogo'

  // El componente no sabe si está sobre el gateway local o un snapshot
  // estático (P3): la distinción ya viene resuelta en frescura.tipo, que
  // sale de CatalogSource.frescura().
  let { frescura }: { frescura: InfoFrescura } = $props()

  const horas = $derived(
    frescura.generadoEn
      ? Math.max(0, Math.round((Date.now() - frescura.generadoEn.getTime()) / 3_600_000))
      : 0,
  )
</script>

<span class="frescura">
  {frescura.tipo === 'vivo' ? t('frescura.envivo') : t('frescura.comprobado', { horas })}
</span>

<style>
  .frescura { font-size: 12px; color: var(--text-muted); }
</style>
