<script lang="ts">
  import type { Canal } from '../datos/catalogo'
  import MarcaWeb from './MarcaWeb.svelte'
  import LogoCanal from './LogoCanal.svelte'
  import SenalCanal from './SenalCanal.svelte'
  import { pareceGeoBloqueado } from '../lib/geo'
  import { parsearResolucion } from '../lib/resolucion'
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

  // Resolución parseada del nombre (lib/resolucion.ts): el catálogo no trae
  // un campo propio. Sin match, sin badge — no se inventa un dato que el
  // nombre no dice.
  const resolucion = $derived(parsearResolucion(canal.nombre))

  const metaLinea = $derived([canal.pais, canal.categoriaId].filter(Boolean).join(' · '))
</script>

<article class="tarjeta" role="listitem" data-indice={indice}>
  <button
    class="abrir"
    onclick={() => alAbrir(canal)}
    aria-label={canal.nombre}
    tabindex={focoActivo ? 0 : -1}
  >
    <div class="logo">
      <LogoCanal logoUrl={canal.logoUrl} nombre={canal.nombre} />

      <!-- Arriba-izq: punto de salud + ms, sobre --scrim-strong. SenalCanal
           (fix 1, compartido con la fila de lista en RejillaCanales) pone el
           punto + el ms; esta insignia solo aporta el badge/scrim, que es
           cosa del llamador, no de SenalCanal. -->
      <span class="insignia insignia-salud">
        <SenalCanal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
      </span>

      <!-- Abajo-der: resolución, si el nombre la trae. -->
      {#if resolucion}
        <span class="insignia insignia-resolucion">{resolucion}</span>
      {/if}
    </div>
  </button>

  <footer>
    <div class="fila-nombre">
      <span class="nombre">{canal.nombre}</span>
      <button
        class="favorito"
        class:activo={esFavorito}
        onclick={() => favoritos.alternar(canal.id)}
        aria-pressed={esFavorito}
        aria-label={esFavorito ? t('canal.favorito.quitar') : t('canal.favorito.anadir')}
        tabindex={focoActivo ? 0 : -1}
      >★</button>
    </div>
    <div class="fila-meta">
      {#if metaLinea}<span class="meta">{metaLinea}</span>{/if}
      {#if pareceGeoBloqueado(canal)}
        <span class="geo" title={t('canal.geo')} aria-label={t('canal.geo')}>GEO</span>
      {/if}
      <MarcaWeb webOk={canal.webOk} />
    </div>
  </footer>
</article>

<style>
  .tarjeta { background: var(--surface-card); border-radius: 8px; padding: 8px; display: flex; flex-direction: column; gap: 6px; }
  .abrir { all: unset; cursor: pointer; display: block; }
  /* LogoCanal (fix final, hallazgo 1) no decide tamaño: este contenedor es
     el mismo width:100%/aspect-ratio:16:9 que antes tenían img/.sinlogo
     directamente — layout visual sin cambios. position:relative para poder
     anclar las insignias (Tarea 8) sobre la miniatura. */
  .logo { position: relative; width: 100%; aspect-ratio: 16/9; border-radius: 6px; overflow: hidden; }

  /* Insignias sobre --scrim-strong (Tarea 8, P0.6): velo oscuro para que el
     texto/punto se lean encima de cualquier logo, claro u oscuro. */
  .insignia {
    position: absolute;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 5px;
    border-radius: 4px;
    background: var(--scrim-strong);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-strong, #fff);
  }
  .insignia-salud { top: 4px; left: 4px; }
  .insignia-resolucion { bottom: 4px; right: 4px; }

  .fila-nombre { display: flex; align-items: center; gap: 6px; }
  .nombre { font-size: 13px; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .fila-meta { display: flex; align-items: center; gap: 6px; }
  /* Contraste (Tarea 18): --graphite-300 sobre --surface-card da 3.74:1,
     por debajo del 4.5:1 que exige AA para texto normal. Se compone con
     --text-muted (6.54:1 sobre la misma tarjeta), ya en los tokens. */
  .meta {
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    font-size: 11px;
    color: var(--text-muted);
  }
  .geo { font-size: 10px; letter-spacing: .05em; padding: 1px 4px; border-radius: 3px; background: var(--graphite-600); color: var(--text-muted, var(--graphite-300)); }
  /* Contraste (Tarea 18): --graphite-500 sobre --surface-card da 1.71:1 —
     el icono de favorito inactivo era casi invisible. --text-muted da
     6.54:1, con margen de sobra tanto si se trata como texto (glifo ★,
     AA exige 4.5:1) como si se trata como grafismo de control (1.4.11,
     3:1). El activo sigue en ámbar — "activo" es la única semántica que
     usa ámbar aquí, y ya existía antes de esta tarea. */
  .favorito { all: unset; cursor: pointer; color: var(--text-muted); flex-shrink: 0; }
  /* Ámbar = activo. Un favorito apagado es neutro. */
  .favorito.activo { color: var(--amber-500); }
</style>
