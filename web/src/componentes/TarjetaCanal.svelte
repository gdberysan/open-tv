<script lang="ts">
  import type { Canal } from '../datos/catalogo'
  import BarrasSenal from './BarrasSenal.svelte'
  import MarcaWeb from './MarcaWeb.svelte'
  import LogoCanal from './LogoCanal.svelte'
  import { pareceGeoBloqueado } from '../lib/geo'
  import { favoritos } from '../estado/favoritos'
  import { t } from '../i18n'

  // indice/focoActivo (Tarea 18, roving tabindex): opcionales y con
  // focoActivo por defecto en true para no romper a nadie que use esta
  // tarjeta suelta (tests existentes incluidos) — solo RejillaVirtual, que
  // SÍ gestiona un único índice activo por rejilla, pasa focoActivo=false a
  // las tarjetas que no son la activa. indice es el índice GLOBAL dentro de
  // canales (no el de la ventana visible): permite a RejillaVirtual
  // localizar el nodo DOM por [data-indice] tras reajustar la ventana,
  // porque la instancia de esta tarjeta puede no existir todavía cuando se
  // decide a qué índice mover el foco.
  let {
    canal,
    alAbrir,
    indice,
    focoActivo = true,
  }: { canal: Canal; alAbrir: (c: Canal) => void; indice?: number; focoActivo?: boolean } = $props()
  const esFavorito = $derived($favoritos.has(canal.id))
  // El fallback img-o-iniciales (logoRoto/onerror) vive en LogoCanal.svelte
  // (fix final, hallazgo 1) — compartido con la fila de RejillaCanales en
  // modo lista, que antes se quedaba sin él.
</script>

<article class="tarjeta" role="listitem" data-indice={indice}>
  <button
    class="abrir"
    onclick={() => alAbrir(canal)}
    aria-label={canal.nombre}
    tabindex={focoActivo ? 0 : -1}
  >
    <div class="logo"><LogoCanal logoUrl={canal.logoUrl} nombre={canal.nombre} /></div>
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
      tabindex={focoActivo ? 0 : -1}
    >★</button>
  </footer>
</article>

<style>
  .tarjeta { background: var(--surface-card); border-radius: 8px; padding: 8px; display: flex; flex-direction: column; gap: 6px; }
  .abrir { all: unset; cursor: pointer; display: flex; flex-direction: column; gap: 6px; align-items: center; }
  /* LogoCanal (fix final, hallazgo 1) no decide tamaño: este contenedor es
     el mismo width:100%/aspect-ratio:16:9 que antes tenían img/.sinlogo
     directamente — layout visual sin cambios. */
  .logo { width: 100%; aspect-ratio: 16/9; }
  .nombre { font-size: 13px; text-align: center; }
  footer { display: flex; align-items: center; gap: 6px; }
  /* Contraste (Tarea 18): --graphite-300 sobre --surface-card da 3.74:1,
     por debajo del 4.5:1 que exige AA para texto normal. Se compone con
     --text-muted (6.54:1 sobre la misma tarjeta), ya en los tokens. */
  .pais { font-size: 11px; color: var(--text-muted); }
  .geo { font-size: 10px; letter-spacing: .05em; padding: 1px 4px; border-radius: 3px; background: var(--graphite-600); color: var(--text-muted, var(--graphite-300)); }
  /* Contraste (Tarea 18): --graphite-500 sobre --surface-card da 1.71:1 —
     el icono de favorito inactivo era casi invisible. --text-muted da
     6.54:1, con margen de sobra tanto si se trata como texto (glifo ★,
     AA exige 4.5:1) como si se trata como grafismo de control (1.4.11,
     3:1). El activo sigue en ámbar — "activo" es la única semántica que
     usa ámbar aquí, y ya existía antes de esta tarea. */
  .favorito { all: unset; cursor: pointer; margin-left: auto; color: var(--text-muted); }
  /* Ámbar = activo. Un favorito apagado es neutro. */
  .favorito.activo { color: var(--amber-500); }
</style>
