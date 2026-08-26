<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { get } from 'svelte/store'
  import { idioma, t } from './i18n'
  import { crearHttpCatalog } from './datos/http'
  import type { CatalogSource, Canal, ConsultaCatalogo, Faceta, Frescura as InfoFrescura } from './datos/catalogo'
  import { filtros } from './estado/filtros'
  import { favoritos } from './estado/favoritos'
  import { clasificarError, consultarSalud, type ClaseError } from './estado/salud'
  import { reportarDesenlace } from './estado/estadisticas'
  import BarraLateralFacetas from './componentes/BarraLateralFacetas.svelte'
  import RejillaCanales from './componentes/RejillaCanales.svelte'
  import Reproductor from './componentes/Reproductor.svelte'
  import Sincronizando from './componentes/Sincronizando.svelte'
  import MensajeError from './componentes/MensajeError.svelte'
  import PanelStats from './componentes/PanelStats.svelte'
  import IndicadorSenal from './componentes/IndicadorSenal.svelte'
  import BarraAcciones from './componentes/BarraAcciones.svelte'

  // La página son 500 canales, el máximo que acepta el gateway (Tarea 11).
  const PAGINA = 500

  // fuente es inyectable (Tarea 15): en producción cae en crearHttpCatalog,
  // pero los tests de componente pueden pasar un CatalogSource falso sin
  // tocar la red. Único punto del cliente que habla con CatalogSource: todos
  // los demás componentes leen stores y emiten callbacks.
  let { fuente = crearHttpCatalog('') }: { fuente?: CatalogSource } = $props()

  // Puerta de entrada: hasta que /health confirme que el catálogo ya se
  // sincronizó una vez, no tiene sentido pedir /channels — la primera
  // respuesta sería una rejilla vacía indistinguible de un bug. 'error' lleva
  // la clase ya resuelta (gateway/red/servidor), nunca un texto suelto que
  // pueda mezclar los tres.
  type Fase = { tipo: 'comprobando' } | { tipo: 'sincronizando' } | { tipo: 'error'; clase: ClaseError } | { tipo: 'listo' }
  let fase = $state<Fase>({ tipo: 'comprobando' })

  let canales = $state<Canal[]>([])
  let total = $state(0)
  let cargando = $state(false)
  let errorCatalogo = $state<ClaseError | null>(null)
  let desplazamiento = $state(0)
  let paises = $state<Faceta[]>([])
  let categorias = $state<Faceta[]>([])
  let calidades = $state<Faceta[]>([])
  let frescura = $state<InfoFrescura | null>(null)

  // Descarta respuestas de peticiones que ya no son la última: cambiar de
  // filtro dos veces seguidas no puede dejar pintada la respuesta de la
  // primera si llega después que la de la segunda.
  let peticionActual = 0

  // Estado del reproductor. Desde la Tarea 5 el propio Reproductor pide sus
  // mirrors y su destino de compatibilidad (vía CatalogSource): App solo
  // necesita saber QUÉ canal está abierto, no resolverle antes una URL.
  let canalAbierto = $state<Canal | null>(null)

  // Shell de dos columnas (Tarea 4 de P0.6): true = barra de facetas visible.
  // En escritorio (>900px) es simplemente la columna izquierda del grid; bajo
  // ~900px la misma bandera gobierna el cajón (aside fijo con translateX).
  // El contenido REAL de las facetas llega en la Tarea 6 — aquí el aside es
  // un hueco mínimo, a propósito (alcance de esta tarea: solo el layout).
  let lateralAbierto = $state(true)

  function alternarLateral() {
    lateralAbierto = !lateralAbierto
  }

  // vistaStats: vista de depuración local, sin ruta de servidor propia. NO se
  // puede usar el PATH /stats para esto: el router (Tarea 13) ya registra
  // GET /stats como el endpoint JSON de verdad, ANTES del fallback SPA — un
  // F5 en ese path serviría el JSON crudo, no la app. Se usa el hash
  // (#stats), que el navegador nunca manda al servidor, así que no puede
  // chocar con ninguna ruta de la API.
  let vistaStats = $state(typeof window !== 'undefined' && window.location.hash === '#stats')

  function abrirStats(e: MouseEvent) {
    e.preventDefault()
    vistaStats = true
    location.hash = 'stats'
  }

  function volverDelPanel() {
    vistaStats = false
    history.pushState('', document.title, window.location.pathname + window.location.search)
  }

  function construirConsulta(paginar: boolean): ConsultaCatalogo {
    const f = get(filtros)
    // soloFavoritos no pagina: un favorito puede estar en la página 20, así
    // que se manda la lista completa de ids (el centinela del conjunto
    // vacío ya está resuelto en HttpCatalog).
    if (f.soloFavoritos) return { ids: [...get(favoritos)] }

    const base: ConsultaCatalogo = {
      q: f.q,
      pais: f.pais,
      categoria: f.categoria,
      calidad: f.calidad,
      mostrarOffline: f.mostrarOffline,
    }
    // untrack: desplazamiento se lee aquí pero adrede NO forma parte de "qué
    // cambia el resultado del catálogo" (ver claveConsulta más abajo, que lo
    // excluye a propósito). Sin untrack, el efecto de claveConsulta —que
    // llama a esta función a través de cargarPagina(true)— se suscribe de
    // rebote a desplazamiento por leerlo en su misma pasada síncrona; como
    // cargarPagina ESCRIBE desplazamiento al terminar con éxito, el efecto se
    // disparaba a sí mismo sin fin: reset→consulta→éxito→reset→… sin que la
    // rejilla llegara nunca a pintar nada. Bug real que este mismo e2e
    // (Tarea 15) fue el primero en poder ver, porque los tests de componente
    // con mocks nunca dejan correr el ciclo de efectos lo bastante para que
    // ocurra.
    return paginar ? { ...base, limite: PAGINA, desplazamiento: untrack(() => desplazamiento) } : base
  }

  async function cargarPagina(reiniciar: boolean) {
    const idPeticion = ++peticionActual
    if (reiniciar) {
      desplazamiento = 0
      canales = []
    }
    cargando = true
    try {
      const pagina = await fuente.canales(construirConsulta(true))
      if (idPeticion !== peticionActual) return // ya hay una consulta más nueva en marcha
      canales = reiniciar ? pagina.canales : [...canales, ...pagina.canales]
      // El total sale de X-Total-Count (vía HttpCatalog), nunca de
      // canales.length: con paginación son números distintos.
      total = pagina.total
      errorCatalogo = null
      if (!get(filtros).soloFavoritos) desplazamiento += pagina.canales.length
    } catch (e) {
      if (idPeticion !== peticionActual) return
      errorCatalogo = clasificarError(e)
    } finally {
      if (idPeticion === peticionActual) cargando = false
    }
  }

  function alPedirMas() {
    if (cargando) return
    if (get(filtros).soloFavoritos) return // ya vino todo de una vez
    if (canales.length >= total) return
    cargarPagina(false)
  }

  function abrirCanal(canal: Canal) {
    canalAbierto = canal
  }

  function cerrarReproductor() {
    canalAbierto = null
  }

  // ←/→ del reproductor se mueven dentro de la lista ya cargada en pantalla,
  // no piden más canales: son "el siguiente que ya veo", no paginación.
  function indiceAbierto(): number {
    return canalAbierto ? canales.findIndex((c) => c.id === canalAbierto!.id) : -1
  }

  function canalAnterior() {
    const i = indiceAbierto()
    if (i > 0) abrirCanal(canales[i - 1])
  }

  function canalSiguiente() {
    const i = indiceAbierto()
    if (i >= 0 && i < canales.length - 1) abrirCanal(canales[i + 1])
  }

  async function alAleatorio() {
    try {
      abrirCanal(await fuente.aleatorio(construirConsulta(false)))
    } catch (e) {
      errorCatalogo = clasificarError(e)
    }
  }

  // Todo lo que cambia el resultado del catálogo, con "vista" excluida a
  // propósito: es un modo de pintar la lista, no un filtro de la consulta.
  let claveConsulta = $derived(
    JSON.stringify({
      q: $filtros.q,
      pais: $filtros.pais,
      categoria: $filtros.categoria,
      calidad: $filtros.calidad,
      mostrarOffline: $filtros.mostrarOffline,
      soloFavoritos: $filtros.soloFavoritos,
      favIds: $filtros.soloFavoritos ? [...$favoritos].sort().join(',') : '',
    }),
  )

  $effect(() => {
    claveConsulta
    // Antes de 'listo' no hay catálogo que pedir: pedirlo igual sería la
    // rejilla vacía que parece rota que este estado existe para evitar.
    if (fase.tipo !== 'listo') return
    // Cambiar cualquier filtro reinicia el desplazamiento y vacía la lista;
    // el scroll infinito solo suma páginas.
    cargarPagina(true)
  })

  // consultarSalud() es la única fuente de verdad sobre si el catálogo ya
  // está listo. Si falla, clasificarError separa "Open TV cerrado" de "sin
  // red" de "el servidor contesta mal" — confundirlos costó una tarde de
  // diagnóstico el 2026-08-07.
  async function comprobarSalud() {
    try {
      const salud = await consultarSalud()
      fase = salud.sincronizando ? { tipo: 'sincronizando' } : { tipo: 'listo' }
    } catch (e) {
      fase = { tipo: 'error', clase: clasificarError(e) }
    }
  }

  function alSincronizado() {
    fase = { tipo: 'listo' }
  }

  onMount(() => {
    comprobarSalud()
    ;(async () => {
      try {
        const [p, c, q, f] = await Promise.all([
          fuente.paises(),
          fuente.categorias(),
          fuente.calidades(),
          fuente.frescura(),
        ])
        paises = p
        categorias = c
        calidades = q
        frescura = f
      } catch {
        // Sin facetas los selectores se quedan solo con "Todos"; no es motivo
        // para tumbar el resto de la app.
      }
    })()
  })

  function alternarIdioma() {
    idioma.actual = idioma.actual === 'es' ? 'en' : 'es'
  }

  // Regiones aria-live PERSISTENTES (fix round 1, Hallazgo 1): Sincronizando
  // y MensajeError se montan/desmontan con {#if}/{:else if} — un lector de
  // pantalla que solo escucha MUTACIONES DE TEXTO dentro de una región ya
  // presente (NVDA, y VoiceOver de forma inconsistente) no anuncia la
  // inserción de un nodo aria-live nuevo. Estas dos cadenas derivadas
  // alimentan un par de <div class="sr-only" aria-live> que existen SIEMPRE
  // (vacíos en catálogo normal) en vez de togglear el nodo entero; los
  // componentes visuales Sincronizando/MensajeError no cambian — siguen
  // siendo lo que ve quien SÍ ve la pantalla.
  const mensajeDeClaseAccesible = (clase: ClaseError) =>
    clase === 'gateway' ? t('estado.gatewayCaido') : clase === 'red' ? t('estado.sinRed') : t('estado.errorServidor')

  let mensajeSincronizandoAccesible = $derived(fase.tipo === 'sincronizando' ? t('estado.sincronizando') : '')
  let mensajeErrorAccesible = $derived(
    fase.tipo === 'error'
      ? mensajeDeClaseAccesible(fase.clase)
      : fase.tipo === 'listo' && errorCatalogo
        ? mensajeDeClaseAccesible(errorCatalogo)
        : '',
  )

  // Tarea 5 (P0.6): estado del IndicadorSenal de la cabecera, derivado de la
  // MISMA fase que ya gobierna qué se pinta en <main> — nunca un estado
  // paralelo inventado. 'comprobando' (el instante antes de que /health
  // conteste por primera vez) cuenta como 'sincronizando': todavía no hay
  // nada que confirmar como vivo. 'error' (fase.tipo, cualquier clase:
  // gateway/red/servidor) y un errorCatalogo posterior a un 'listo' cuentan
  // los dos como 'sin-gateway' — en ambos casos el cliente dejó de poder
  // contactar con Open TV, que es justo lo que ese estado comunica.
  let estadoSenal = $derived<'vivo' | 'sincronizando' | 'sin-gateway'>(
    fase.tipo === 'sincronizando' || fase.tipo === 'comprobando'
      ? 'sincronizando'
      : fase.tipo === 'error' || (fase.tipo === 'listo' && !!errorCatalogo)
        ? 'sin-gateway'
        : 'vivo',
  )
</script>

<!-- inert (Tarea 18, orden de foco; fix round 1, Hallazgo 1): mientras el
     reproductor está abierto es un diálogo modal (role="dialog" aria-modal,
     ver Reproductor.svelte) que vive FUERA de <div class="fondo">, como
     hermano. Sin inert, Tab seguía alcanzando los botones de fuera del
     modal — un lector de pantalla o un usuario de teclado podía "salirse"
     del diálogo sin cerrarlo. La reestructuración de la Tarea 4 sacó
     <header> (con el toggle de idioma, interactivo) y el botón de cajón de
     dentro de <main>, así que un inert puesto solo en <main>/<footer> ya NO
     bastaba: la cabecera quedaba clicable/focable por detrás del modal.
     El arreglo envuelve TODO el fondo —cabecera, cuerpo (aside + main) y
     pie— en <div class="fondo" inert={...}>, un contenedor nuevo sin estilo
     propio (no toca el fondo visual de .sala ni el del pie: solo aporta el
     boundary de inert). inert se hereda por todo el subárbol, así que
     cabecera, botón de cajón, aside, main y pie quedan inert de una sola
     vez, y cuando la Tarea 6 meta controles reales en el aside quedarán
     cubiertos automáticamente sin tocar este boundary. <main>/<footer>
     conservan además su inert propio (redundante pero inocuo) porque un
     test anterior ya lo comprueba nodo a nodo. -->
<!-- Persistentes (fix round 1, Hallazgo 1): ver el comentario largo en el
     <script> sobre por qué NO son los <p aria-live> que se montan dentro de
     Sincronizando/MensajeError. Vacías en catálogo normal; su texto es lo
     único que cambia. Se quedan aquí, en la raíz, FUERA del contenedor
     inert y ANTES del shell — un solo nodo persistente cada una, nunca
     duplicadas, y su anuncio no depende de que el fondo esté o no inert. -->
<div class="sr-only" aria-live="polite" aria-atomic="true">{mensajeSincronizandoAccesible}</div>
<div class="sr-only" role="alert" aria-live="assertive" aria-atomic="true">{mensajeErrorAccesible}</div>

<div class="fondo" inert={!!canalAbierto}>
  <div class="sala">
    <header class="cabecera">
      <div class="marca">
        <h1 class="wordmark">KORVEN <span class="acento">OPEN TV</span></h1>
        <p class="subtitulo">{t('app.lema')}</p>
      </div>
      <div class="cabecera-derecha">
        <!-- Tarea 5 (P0.6): sustituye el hueco de layout de la Tarea 4 por el
             indicador honesto de señal, con el estado REAL derivado más
             arriba de la misma fase que gobierna <main> — nunca decorativo. -->
        <IndicadorSenal estado={estadoSenal} />
        <button type="button" class="idioma" onclick={alternarIdioma}>
          {idioma.actual === 'es' ? t('idioma.en') : t('idioma.es')}
        </button>
      </div>
    </header>

    <div class="cuerpo">
      <!-- Botón de cajón: solo tiene sentido visualmente bajo el breakpoint de
           ~900px (CSS lo oculta en escritorio, donde el aside ya es la columna
           fija de siempre); se deja siempre montado para que aria-controls /
           aria-expanded describan un control real y estable. -->
      <button
        type="button"
        class="boton-cajon"
        aria-controls="panel-facetas"
        aria-expanded={lateralAbierto}
        onclick={alternarLateral}
      >
        {t('shell.facetas')}
      </button>

      <aside class="facetas" id="panel-facetas" data-abierto={lateralAbierto}>
        <!-- Tarea 6 (P0.6): contenido real de las facetas. -->
        <h2 class="sr-only">{t('shell.facetas')}</h2>
        <BarraLateralFacetas {paises} {categorias} {calidades} />
      </aside>

      <main inert={!!canalAbierto}>
        {#if vistaStats}
          <PanelStats alVolver={volverDelPanel} />
        {:else if fase.tipo === 'sincronizando'}
          <Sincronizando alListo={alSincronizado} />
        {:else if fase.tipo === 'error'}
          <MensajeError clase={fase.clase} />
        {:else if fase.tipo === 'listo'}
          <!-- Tarea 7 (P0.6): buscador/facetas/señal viven en el aside
               (BarraLateralFacetas, Tarea 6); "Solo favoritos", "Canal al
               azar", el conmutador de vista, los chips de filtro removibles
               y el conteo viven aquí, en BarraAcciones. -->
          <BarraAcciones {total} {frescura} {alAleatorio} />

          {#if errorCatalogo}
            <MensajeError clase={errorCatalogo} />
          {:else}
            <RejillaCanales {canales} vista={$filtros.vista} {cargando} {alPedirMas} alAbrir={abrirCanal} />
          {/if}
        {/if}
      </main>
    </div>
  </div>

  <footer class="pie" inert={!!canalAbierto}>
    <p>{t('pie.fuente')}</p>
    <p>{t('pie.postura')}</p>
    {#if !vistaStats}
      <p><a class="stats" href="#stats" onclick={abrirStats}>{t('pie.stats')}</a></p>
    {/if}
  </footer>
</div>

{#if canalAbierto}
  <Reproductor
    canal={canalAbierto}
    {fuente}
    alDesenlace={reportarDesenlace}
    alCerrar={cerrarReproductor}
    alAnterior={canalAnterior}
    alSiguiente={canalSiguiente}
  />
{/if}

<style>
  /* Shell de dos columnas (Tarea 4 de P0.6). El fondo --surface-abyss es más
     profundo que --surface-base (body): así se nota dónde empieza la "sala
     de control" frente al resto de la página (footer incluido). */
  .sala {
    background: var(--surface-abyss);
    /* overflow-x nunca desborda (constraint global): el cajón fijo de
       abajo de 900px se posiciona respecto a este contenedor, y ningún hijo
       (rejilla, tarjetas) puede forzar scroll horizontal de la página. */
    overflow-x: hidden;
  }

  .cabecera {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3, 12px);
    padding: var(--space-4, 1rem) var(--space-6, 2rem);
    border-bottom: 1px solid var(--border-default);
  }
  .marca { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
  .wordmark {
    margin: 0;
    font: var(--type-h4);
    letter-spacing: var(--tracking-snug);
    color: var(--text-strong);
  }
  .wordmark .acento { color: var(--text-accent); }
  .subtitulo {
    margin: 0;
    color: var(--text-muted);
    font: var(--type-mono-label);
    letter-spacing: var(--tracking-mono);
  }
  .cabecera-derecha { display: flex; align-items: center; gap: var(--space-3, 12px); }
  .idioma {
    background: none; border: 1px solid var(--border-default); color: var(--text-body);
    border-radius: 6px; padding: 4px 10px; cursor: pointer;
  }

  /* Cuerpo: dos columnas — aside de facetas (248px) + main (resto). minmax(0,
     1fr), no 1fr a secas: sin el mínimo explícito, un hijo ancho (rejilla,
     tabla) puede forzar la columna a crecer más allá del hueco disponible y
     desbordar horizontalmente toda la página. */
  .cuerpo {
    display: grid;
    grid-template-columns: 248px minmax(0, 1fr);
    align-items: start;
  }

  .boton-cajon {
    /* Solo tiene sentido bajo el breakpoint responsive: en escritorio el
       aside ya es la columna fija, visible siempre. */
    display: none;
  }

  .facetas {
    padding: var(--space-4, 1rem);
    border-right: 1px solid var(--border-default);
    min-height: 100%;
  }

  main {
    padding: var(--space-6, 2rem);
    min-width: 0;
  }

  .pie {
    max-width: 72rem;
    margin: 0 auto;
    padding: var(--space-4, 1rem) var(--space-6, 2rem) var(--space-6, 2rem);
    /* Contraste (Tarea 18): --text-faint sobre --surface-base da 4.16:1,
       por debajo del 4.5:1 que exige AA para texto normal a 12px. Se
       compone con --text-muted (7.27:1), ya definido en los tokens de
       Korven — no se toca colors.css. */
    color: var(--text-muted);
    font-size: 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .pie a.stats { color: var(--text-muted); text-decoration: underline; }
  .pie a.stats:hover { color: var(--text-body); }
  .pie p { margin: 0; }

  /* Responsive: bajo ~900px el aside se convierte en un cajón (fixed +
     translate) gobernado por data-abierto, y main pasa a ocupar todo el
     ancho. El botón de cajón solo aparece en este breakpoint: en escritorio
     el aside ya está siempre visible como columna, así que el botón sería
     redundante. */
  @media (max-width: 900px) {
    .cuerpo {
      grid-template-columns: 1fr;
    }
    .boton-cajon {
      display: inline-flex;
      align-self: flex-start;
      margin: var(--space-3, 12px) var(--space-3, 12px) 0;
      background: none;
      border: 1px solid var(--border-default);
      color: var(--text-body);
      border-radius: 6px;
      padding: 4px 10px;
      cursor: pointer;
    }
    .facetas {
      position: fixed;
      top: 0;
      left: 0;
      bottom: 0;
      width: 248px;
      max-width: 80vw;
      background: var(--surface-abyss);
      box-shadow: var(--shadow-panel);
      transform: translateX(-100%);
      transition: transform var(--dur-base, 200ms) var(--ease-out, ease-out);
      z-index: 20;
      overflow-y: auto;
    }
    .facetas[data-abierto='true'] {
      transform: translateX(0);
    }
    @media (prefers-reduced-motion: reduce) {
      .facetas {
        transition: none;
      }
    }
  }
</style>
