<script lang="ts">
  import type { Canal } from '../datos/catalogo'
  import BarrasSenal from './BarrasSenal.svelte'
  import MarcaWeb from './MarcaWeb.svelte'
  import { pareceGeoBloqueado } from '../lib/geo'
  import { favoritos } from '../estado/favoritos'
  import { t } from '../i18n'

  let { canal, alAbrir }: { canal: Canal; alAbrir: (c: Canal) => void } = $props()
  const esFavorito = $derived($favoritos.has(canal.id))
  // La logoUrl del catálogo suele estar muerta/404/bloqueada; si la imagen no
  // carga, caemos a las iniciales en vez de dejar el icono de imagen rota.
  let logoRoto = $state(false)
  // En virtualización (Tarea 17), la instancia se reutiliza con otro canal.
  // Resetear logoRoto al cambiar para que el logo nuevo intente cargar.
  $effect(() => {
    void canal.id
    logoRoto = false
  })
</script>

<article class="tarjeta">
  <button class="abrir" onclick={() => alAbrir(canal)} aria-label={canal.nombre}>
    {#if canal.logoUrl && !logoRoto}
      <img src={canal.logoUrl} alt="" loading="lazy" onerror={() => (logoRoto = true)} />
    {:else}
      <span class="sinlogo" aria-hidden="true">{canal.nombre.slice(0, 2)}</span>
    {/if}
    <span class="nombre">{canal.nombre}</span>
  </button>

  <footer>
    <BarrasSenal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
    {#if canal.pais}<span class="pais">{canal.pais}</span>{/if}
    {#if pareceGeoBloqueado(canal)}
      <span class="geo" title={t('canal.geo')} aria-label={t('canal.geo')}>GEO</span>
    {/if}
    <MarcaWeb webOk={canal.webOk} />
    <button
      class="favorito"
      class:activo={esFavorito}
      onclick={() => favoritos.alternar(canal.id)}
      aria-pressed={esFavorito}
      aria-label={esFavorito ? t('canal.favorito.quitar') : t('canal.favorito.anadir')}
    >★</button>
  </footer>
</article>

<style>
  .tarjeta { background: var(--surface-card); border-radius: 8px; padding: 8px; display: flex; flex-direction: column; gap: 6px; }
  .abrir { all: unset; cursor: pointer; display: flex; flex-direction: column; gap: 6px; align-items: center; }
  img, .sinlogo { width: 100%; aspect-ratio: 16/9; object-fit: contain; }
  .sinlogo { display: grid; place-items: center; background: var(--surface-sunken); color: var(--text-muted, var(--graphite-300)); }
  .nombre { font-size: 13px; text-align: center; }
  footer { display: flex; align-items: center; gap: 6px; }
  .pais { font-size: 11px; color: var(--graphite-300); }
  .geo { font-size: 10px; letter-spacing: .05em; padding: 1px 4px; border-radius: 3px; background: var(--graphite-600); color: var(--text-muted, var(--graphite-300)); }
  .favorito { all: unset; cursor: pointer; margin-left: auto; color: var(--graphite-500); }
  /* Ámbar = activo. Un favorito apagado es neutro. */
  .favorito.activo { color: var(--amber-500); }
</style>
