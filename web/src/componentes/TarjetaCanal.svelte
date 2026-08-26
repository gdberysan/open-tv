<script lang="ts">
  import type { Readable } from 'svelte/store'
  import type { AhoraDespues, Canal } from '../datos/catalogo'
  import MarcaWeb from './MarcaWeb.svelte'
  import LogoCanal from './LogoCanal.svelte'
  import SenalCanal from './SenalCanal.svelte'
  import { pareceGeoBloqueado } from '../lib/geo'
  import { parsearResolucion } from '../lib/resolucion'
  import { formatearHoraLocal } from '../lib/hora'
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
  // densidad (Tarea 5, P0.8): opcional, con 'comoda' por defecto — mismo
  // criterio que focoActivo, para no romper a nadie que use esta tarjeta
  // suelta (tests existentes incluidos). Solo cambia padding/tipografía vía
  // la clase .compacta (más abajo): el punto+ms+resolución (insignias) NO
  // encogen, para que sigan siendo legibles con la tarjeta más pequeña.
  // epg (Tarea 8, P2): opcional, por el mismo motivo que focoActivo/densidad
  // — no romper a nadie que use esta tarjeta suelta (tests existentes
  // incluidos). Solo el store (T7) que App.svelte crea y RejillaVirtual
  // reenvía; la tarjeta NUNCA pide datos (no llama a asegurar/ahoraDespuesDe
  // como comando) — solo se SUSCRIBE para leer, así una misma instancia
  // reciclada por la virtualización se entera sola de que su canal actual
  // ya tiene guía cacheada, sin que nadie tenga que empujarle un nuevo prop
  // por tarjeta.
  let {
    canal,
    alAbrir,
    indice,
    focoActivo = true,
    densidad = 'comoda',
    epg,
  }: {
    canal: Canal
    alAbrir: (c: Canal) => void
    indice?: number
    focoActivo?: boolean
    densidad?: 'comoda' | 'compacta'
    epg?: Readable<Map<string, AhoraDespues>>
  } = $props()
  const esFavorito = $derived($favoritos.has(canal.id))

  // Sin epg (prop ausente): mapaEpg se queda en el Map vacío inicial para
  // siempre — ahoraDespues da undefined y la línea de insignia (más abajo)
  // queda vacía, exactamente el mismo resultado visible que "epg presente
  // pero este canal sin guía". Con epg: el efecto se re-suscribe si la
  // instancia del store cambiara (no pasa hoy — App crea una única
  // instancia — pero es la forma correcta de tratar un prop reactivo).
  let mapaEpg = $state<Map<string, AhoraDespues>>(new Map())
  $effect(() => {
    if (!epg) return
    return epg.subscribe((m) => {
      mapaEpg = m
    })
  })
  const ahoraDespues = $derived(mapaEpg.get(canal.id))
  // El fallback img-o-iniciales (logoRoto/onerror) vive en LogoCanal.svelte
  // (fix final, hallazgo 1) — compartido con la fila de RejillaCanales en
  // modo lista, que antes se quedaba sin él.

  // Resolución parseada del nombre (lib/resolucion.ts): el catálogo no trae
  // un campo propio. Sin match, sin badge — no se inventa un dato que el
  // nombre no dice.
  const resolucion = $derived(parsearResolucion(canal.nombre))

  const metaLinea = $derived([canal.pais, canal.categoriaId].filter(Boolean).join(' · '))
</script>

<article class="tarjeta" class:compacta={densidad === 'compacta'} role="listitem" data-indice={indice}>
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

    <!-- Insignia ahora/después (Tarea 8, P2 EPG). ALTURA FIJA reservada
         SIEMPRE (ver .linea-epg más abajo) — con guía o sin ella: la rejilla
         virtualizada (RejillaVirtual) mide UN alto de fila y asume filas
         uniformes; si unas tarjetas midieran más que otras por tener texto
         de guía y otras no, la ventana virtual se descuadraría. Texto
         plano, sin aria-live (invariante P0.6/P0.8: esta tarjeta no es una
         región live) — el lector de pantalla la lee como cualquier otro
         texto de la tarjeta. Sin guía (ahoraDespues undefined, o
         ahora=null): la tarjeta queda limpia a propósito — la honestidad
         explícita de "sin guía" vive en el overlay del reproductor (Tarea
         9), no aquí. -->
    <p class="linea-epg">
      {#if ahoraDespues?.ahora}
        ● {t('epg.ahora')}: {ahoraDespues.ahora.titulo}{#if ahoraDespues.siguiente} · {t('epg.siguiente')} {formatearHoraLocal(ahoraDespues.siguiente.inicioSeg)} · {ahoraDespues.siguiente.titulo}{/if}
      {/if}
    </p>
  </footer>
</article>

<style>
  /* Hover-lift (Tarea 7, P0.8): UN solo sitio para el "lift" de tarjeta —
     nunca se duplica en RejillaCanales (modo lista) ni en ningún otro
     lugar. `transition` vive en la clase base, pero SOLO se dispara en
     :hover — nunca corre durante el scroll de la rejilla virtualizada,
     porque nada cambia el valor de transform/box-shadow mientras se
     reciclan filas (restricción de rendimiento del brief). transform/
     box-shadow son las dos únicas propiedades compositables en GPU sin
     tocar layout — jamás width/height/top/left aquí. */
  .tarjeta {
    background: var(--surface-card);
    border-radius: 8px;
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    transition: transform var(--dur-fast) var(--ease-out), box-shadow var(--dur-fast) var(--ease-out);
  }
  .tarjeta:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-md);
  }
  @media (prefers-reduced-motion: reduce) {
    /* Anula la ANIMACIÓN, no el efecto: el lift sigue siendo una pista
       visual útil al pasar el ratón, solo que ya no se anima — aparece de
       golpe, como el resto de transiciones ya existentes en el repo
       (Reproductor.svelte .overlay, App.svelte .facetas). */
    .tarjeta { transition: none; }
  }
  /* Densidad compacta (Tarea 5, P0.8): menos padding/gap y nombre más
     pequeño — la miniatura (16:9, ver .logo) ya encoge sola porque la
     tarjeta es más angosta (más columnas, mismo ancho de contenedor). Las
     insignias (punto+ms, resolución) NO se tocan aquí a propósito: son las
     que menos margen tienen para seguir siendo legibles. */
  .tarjeta.compacta { padding: 5px; gap: 4px; }
  .tarjeta.compacta .nombre { font-size: 12px; }
  .tarjeta.compacta .fila-meta { gap: 4px; }
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
  /* Insignia ahora/después (Tarea 8, P2 EPG). height (no min-height) fija:
     esta línea mide EXACTAMENTE lo mismo tenga o no texto — con guía, sin
     guía, o con "ahora" pero sin "siguiente" — para que RejillaVirtual mida
     una única altura de tarjeta uniforme sin importar qué canal tocó a cada
     una. overflow:hidden + ellipsis: un título largo de programa nunca
     empuja la altura ni desborda la tarjeta. */
  .linea-epg {
    margin: 0;
    height: 14px;
    line-height: 14px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    font-size: 11px;
    color: var(--text-muted);
  }
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
