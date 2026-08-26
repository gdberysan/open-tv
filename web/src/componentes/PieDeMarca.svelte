<script lang="ts">
  import { t } from '../i18n'

  // Tarea 10 (P0.7): pie de crédito de marca. Estático a propósito — SIN
  // animación (eso es P0.8) y SIN aria-live nuevo: es contenido de cierre,
  // no un estado que cambie ni que haya que anunciar.
</script>

<div class="pie-marca">
  <img class="emblema" src="/marca/korven-emblema.svg" alt={t('pieMarca.emblemaAlt')} width="20" height="20" />
  <p class="credito">
    {t('pieMarca.desarrolladoPor')}
    <a href="https://korven.dev" target="_blank" rel="noopener">Korven</a>
    &amp;
    <a href="https://claude.com/claude-code" target="_blank" rel="noopener">Claude Code</a>
  </p>
  <a class="repo" href="https://github.com/gdberysan/open-tv" target="_blank" rel="noopener">
    {t('pie.codigo')}
  </a>
</div>

<style>
  /* Mismo criterio de contraste que .pie (ver comentario de la Tarea 18 en
     App.svelte): --text-faint sobre --surface-base da 4.16:1, por debajo del
     4.5:1 que exige AA a 12px. El brief pide "discreto, --text-faint", pero
     este texto es contenido real (no decorativo) — se compone con
     --text-muted, que ya es el tono discreto del resto del pie. */
  .pie-marca {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: var(--space-4, 1rem);
    padding-top: var(--space-4, 1rem);
    border-top: 1px solid var(--border-subtle);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
  }
  .emblema {
    display: block;
    flex-shrink: 0;
    width: 20px;
    height: 20px;
  }
  .credito {
    margin: 0;
    flex: 1 1 auto;
  }
  .pie-marca a {
    color: var(--text-muted);
    text-decoration: underline;
  }
  .pie-marca a:hover {
    color: var(--text-body);
  }
  .repo {
    white-space: nowrap;
  }
</style>
