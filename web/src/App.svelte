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
  import BarraFiltros from './componentes/BarraFiltros.svelte'
  import RejillaCanales from './componentes/RejillaCanales.svelte'
  import Reproductor from './componentes/Reproductor.svelte'
  import Sincronizando from './componentes/Sincronizando.svelte'
  import MensajeError from './componentes/MensajeError.svelte'
  import Frescura from './componentes/Frescura.svelte'
  import PanelStats from './componentes/PanelStats.svelte'

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
  let frescura = $state<InfoFrescura | null>(null)

  // Descarta respuestas de peticiones que ya no son la última: cambiar de
  // filtro dos veces seguidas no puede dejar pintada la respuesta de la
  // primera si llega después que la de la segunda.
  let peticionActual = 0

  // Estado del reproductor. Desde la Tarea 5 el propio Reproductor pide sus
  // mirrors y su destino de compatibilidad (vía CatalogSource): App solo
  // necesita saber QUÉ canal está abierto, no resolverle antes una URL.
  let canalAbierto = $state<Canal | null>(null)

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
        const [p, c, f] = await Promise.all([fuente.paises(), fuente.categorias(), fuente.frescura()])
        paises = p
        categorias = c
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
</script>

<!-- inert (Tarea 18, orden de foco): mientras el reproductor está abierto es
     un diálogo modal (role="dialog" aria-modal, ver Reproductor.svelte) que
     vive FUERA de <main> (mismo nivel que <footer>, ver más abajo). Sin
     inert, Tab seguía alcanzando los botones de <main>/<footer> por detrás
     del reproductor — un lector de pantalla o un usuario de teclado podía
     "salirse" del modal sin cerrarlo. inert saca todo <main>/<footer> del
     árbol de accesibilidad y del orden de tabulación de una sola vez, sin
     tener que enumerar a mano cada control de fuera. -->
<!-- Persistentes (fix round 1, Hallazgo 1): ver el comentario largo en el
     <script> sobre por qué NO son los <p aria-live> que se montan dentro de
     Sincronizando/MensajeError. Vacías en catálogo normal; su texto es lo
     único que cambia. -->
<div class="sr-only" aria-live="polite" aria-atomic="true">{mensajeSincronizandoAccesible}</div>
<div class="sr-only" role="alert" aria-live="assertive" aria-atomic="true">{mensajeErrorAccesible}</div>

<main inert={!!canalAbierto}>
  <header>
    <h1>{t('app.titulo')}</h1>
    <p class="lema">{t('app.lema')}</p>
    <button type="button" class="idioma" onclick={alternarIdioma}>
      {idioma.actual === 'es' ? t('idioma.en') : t('idioma.es')}
    </button>
  </header>

  {#if vistaStats}
    <PanelStats alVolver={volverDelPanel} />
  {:else if fase.tipo === 'sincronizando'}
    <Sincronizando alListo={alSincronizado} />
  {:else if fase.tipo === 'error'}
    <MensajeError clase={fase.clase} />
  {:else if fase.tipo === 'listo'}
    <BarraFiltros {paises} {categorias} {alAleatorio} />

    {#if errorCatalogo}
      <MensajeError clase={errorCatalogo} />
    {:else}
      <div class="resumen">
        <p class="total">{t('catalogo.total', { n: total })}</p>
        {#if frescura}<Frescura {frescura} />{/if}
      </div>
      <RejillaCanales {canales} vista={$filtros.vista} {cargando} {alPedirMas} alAbrir={abrirCanal} />
    {/if}
  {/if}
</main>

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

<footer class="pie" inert={!!canalAbierto}>
  <p>{t('pie.fuente')}</p>
  <p>{t('pie.postura')}</p>
  {#if !vistaStats}
    <p><a class="stats" href="#stats" onclick={abrirStats}>{t('pie.stats')}</a></p>
  {/if}
</footer>

<style>
  main {
    max-width: 72rem;
    margin: 0 auto;
    padding: var(--space-6, 2rem);
  }
  header { display: flex; flex-wrap: wrap; align-items: baseline; gap: 12px; margin-bottom: 12px; }
  .lema { color: var(--text-muted); flex: 1; }
  .idioma {
    background: none; border: 1px solid var(--border-default); color: var(--text-body);
    border-radius: 6px; padding: 4px 10px; cursor: pointer;
  }
  .resumen { display: flex; align-items: baseline; gap: 12px; margin: 4px 0 12px; }
  .total { color: var(--text-muted); font-size: 13px; margin: 0; }
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
</style>
