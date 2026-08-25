<script lang="ts">
  import { nivelSenal, type NivelSenal } from '../lib/senal'
  import { t } from '../i18n'

  let { vivo, latenciaMs }: { vivo: boolean | null; latenciaMs: number } = $props()

  const nivel = $derived(nivelSenal(vivo, latenciaMs))
  const barras = $derived({ buena: 3, media: 2, pobre: 1, muerta: 0, desconocido: 0 }[nivel])
  const etiqueta = $derived(
    nivel === 'desconocido' ? t('senal.sinDatos') : nivel === 'muerta' ? t('senal.muerta') : t('senal.viva'),
  )
</script>

<!-- Un solo container con Semantics: tres barras sueltas serían tres nodos sin
     sentido para un lector de pantalla. -->
<span class="senal" role="img" aria-label={etiqueta} data-nivel={nivel}>
  {#each [1, 2, 3] as n}
    <i class:encendida={n <= barras}></i>
  {/each}
</span>

<style>
  .senal { display: inline-flex; gap: 2px; align-items: flex-end; height: 12px; }
  i { width: 3px; background: var(--graphite-500); border-radius: 1px; }
  i:nth-child(1) { height: 5px; }
  i:nth-child(2) { height: 8px; }
  i:nth-child(3) { height: 12px; }
  /* Ámbar SOLO cuando hay señal viva. Es la regla del sistema de diseño. */
  [data-nivel='buena'] i.encendida { background: var(--amber-500); }
  [data-nivel='media'] i.encendida { background: var(--amber-400); }
  [data-nivel='pobre'] i.encendida { background: var(--signal-error); }
</style>
