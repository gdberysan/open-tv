<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte'
  import { get } from 'svelte/store'
  import { idioma, t } from './i18n'
  import { crearHttpCatalog } from './datos/http'
  import type { CatalogSource, Canal, ConsultaCatalogo, Faceta, Frescura as InfoFrescura, Fuente } from './datos/catalogo'
  import { filtros } from './estado/filtros'
  import { favoritos } from './estado/favoritos'
  import { preferencias } from './estado/preferencias'
  import { clasificarError, consultarSalud, type ClaseError } from './estado/salud'
  import { reportarDesenlace } from './estado/estadisticas'
  import { historial, type EntradaHistorial } from './estado/historial'
  import { debeHacerSurf, esObjetivoInteractivo } from './lib/surf'
  import BarraLateralFacetas from './componentes/BarraLateralFacetas.svelte'
  import RejillaCanales from './componentes/RejillaCanales.svelte'
  import Reproductor from './componentes/Reproductor.svelte'
  import Sincronizando from './componentes/Sincronizando.svelte'
  import MensajeError from './componentes/MensajeError.svelte'
  import PanelStats from './componentes/PanelStats.svelte'
  import IndicadorSenal from './componentes/IndicadorSenal.svelte'
  import BarraAcciones from './componentes/BarraAcciones.svelte'
  import ContinuarViendo from './componentes/ContinuarViendo.svelte'
  import Onboarding from './componentes/Onboarding.svelte'
  import SincronizandoFuente from './componentes/SincronizandoFuente.svelte'
  import Fuentes from './componentes/Fuentes.svelte'
  import Ajustes from './componentes/Ajustes.svelte'
  import PieDeMarca from './componentes/PieDeMarca.svelte'
  import Paleta from './componentes/Paleta.svelte'
  import {
    borrarFiltrosGuardados,
    borrarVistaGuardada,
    escribirFiltrosGuardados,
    escribirVistaGuardada,
    leerFiltrosGuardados,
    leerVistaGuardada,
  } from './estado/persistenciaSesion'

  // La página son 500 canales, el máximo que acepta el gateway (Tarea 11).
  const PAGINA = 500

  // Fix round 1 (Tarea 6, P0.7): cadencia/tope del sondeo tras añadir una
  // fuente. 1.5s / 60 intentos ≈ 90s — las listas grandes (iptv-org global)
  // pueden tardar decenas de segundos en sincronizar del lado del backend.
  const SONDEO_FUENTE_INTERVALO_MS = 1500
  const SONDEO_FUENTE_INTENTOS_MAX = Math.ceil(90_000 / SONDEO_FUENTE_INTERVALO_MS)

  // fuente es inyectable (Tarea 15): en producción cae en crearHttpCatalog,
  // pero los tests de componente pueden pasar un CatalogSource falso sin
  // tocar la red. Único punto del cliente que habla con CatalogSource: todos
  // los demás componentes leen stores y emiten callbacks.
  //
  // sondeoFuenteIntervaloMs/sondeoFuenteIntentosMax son inyectables SOLO para
  // tests (fix round 1, Tarea 6): sin esto, un test tendría que avanzar
  // timers falsos ~90s reales (60 intentos) para ejercitar el caso de tope.
  let {
    fuente = crearHttpCatalog(''),
    sondeoFuenteIntervaloMs = SONDEO_FUENTE_INTERVALO_MS,
    sondeoFuenteIntentosMax = SONDEO_FUENTE_INTENTOS_MAX,
  }: {
    fuente?: CatalogSource
    sondeoFuenteIntervaloMs?: number
    sondeoFuenteIntentosMax?: number
  } = $props()

  // Puerta de entrada: hasta que /health confirme que el catálogo ya se
  // sincronizó una vez, no tiene sentido pedir /channels — la primera
  // respuesta sería una rejilla vacía indistinguible de un bug. 'error' lleva
  // la clase ya resuelta (gateway/red/servidor), nunca un texto suelto que
  // pueda mezclar los tres.
  type Fase = { tipo: 'comprobando' } | { tipo: 'sincronizando' } | { tipo: 'error'; clase: ClaseError } | { tipo: 'listo' }
  let fase = $state<Fase>({ tipo: 'comprobando' })

  // Tarea 6 (P0.8): versión real para «Acerca de» (Ajustes.svelte) — viene
  // del MISMO /health que ya gobierna `fase`, nunca un segundo fetch.
  let versionApp = $state('dev')

  let canales = $state<Canal[]>([])
  let total = $state(0)
  let cargando = $state(false)
  let errorCatalogo = $state<ClaseError | null>(null)
  let desplazamiento = $state(0)
  let paises = $state<Faceta[]>([])
  let categorias = $state<Faceta[]>([])
  let calidades = $state<Faceta[]>([])
  let frescura = $state<InfoFrescura | null>(null)

  // Tarea 6 (P0.7): fuentes bring-your-own. fuentesCargadas distingue "todavía
  // no sabemos" (arranque, o un fallo de red en la carga inicial) de
  // "confirmado: cero fuentes" — sin esa distinción, sinFuentes (más abajo)
  // se activaría de más en el primer instante de cualquier montaje, antes de
  // que fuente.fuentes() llegue a resolver.
  let fuentes = $state<Fuente[]>([])
  let fuentesCargadas = $state(false)

  // Fix round 1 (Tarea 6, P0.7 — gate real en Chrome del controlador): tras
  // crear una fuente, el backend la sincroniza de forma ASÍNCRONA — un solo
  // refetch inmediato (el código original) casi siempre ve total=0 todavía.
  // sondeandoFuenteNueva/sondeoAgotado gobiernan un tercer estado visible
  // (SincronizandoFuente.svelte) que sondea el catálogo hasta que aparecen
  // canales o se agota el tope. idSondeoActual seguido el mismo patrón que
  // peticionActual: invalida cualquier tick de un sondeo anterior (reintento
  // superpuesto, o componente desmontado) para que nunca dos sondeos
  // escriban intentosSondeo/estado a la vez.
  let sondeandoFuenteNueva = $state(false)
  let sondeoAgotado = $state(false)
  let intentosSondeo = 0
  let idSondeoActual = 0
  let temporizadorSondeo: ReturnType<typeof setTimeout> | undefined

  // Descarta respuestas de peticiones que ya no son la última: cambiar de
  // filtro dos veces seguidas no puede dejar pintada la respuesta de la
  // primera si llega después que la de la segunda.
  let peticionActual = 0

  // Estado del reproductor. Desde la Tarea 5 el propio Reproductor pide sus
  // mirrors y su destino de compatibilidad (vía CatalogSource): App solo
  // necesita saber QUÉ canal está abierto, no resolverle antes una URL.
  let canalAbierto = $state<Canal | null>(null)

  // Paleta de comandos ⌘K/Ctrl+K (Tarea 4, P0.8). RULING del plan: se
  // INHIBE por completo mientras el reproductor está abierto — dos modales
  // con trap de foco propio anidados (¿qué Esc gana? ¿qué Tab atrapa?) es una
  // complejidad que un atajo de conveniencia no justifica; más simple es "un
  // solo modal a la vez" (ver alTeclaVentana, más abajo, y cerrarPaleta).
  let paletaAbierta = $state(false)

  function cerrarPaleta() {
    paletaAbierta = false
  }

  // Shell de dos columnas (Tarea 4 de P0.6): true = barra de facetas visible.
  // En escritorio (>900px) es simplemente la columna izquierda del grid; bajo
  // ~900px la misma bandera gobierna el cajón (aside fijo con translateX).
  // El contenido REAL de las facetas llega en la Tarea 6 — aquí el aside es
  // un hueco mínimo, a propósito (alcance de esta tarea: solo el layout).
  let lateralAbierto = $state(true)

  function alternarLateral() {
    lateralAbierto = !lateralAbierto
  }

  // Fix round 1 (Tarea 15): detectar el viewport estrecho para poder dejar
  // inert el cajón cuando está fuera de pantalla — mismo patrón que
  // RejillaVirtual.svelte (bind:innerWidth vía svelte:window, sin
  // matchMedia). 900 DEBE coincidir con el @media (max-width: 900px) de la
  // hoja de estilos de este componente: si cambia el breakpoint del CSS,
  // este número cambia con él (es el mismo punto en el que el aside pasa de
  // columna fija a cajón fixed+translateX). Nota (hallazgo real de este
  // fix): un literal con corchetes angulares de una etiqueta especial
  // (script/style/svelte:window) escrito DENTRO de un comentario de este
  // bloque <script> confunde al compilador de Svelte ("`<script>` was left
  // open" en svelte-check) aunque el mismo texto en un comentario HTML fuera
  // del bloque es inofensivo — por eso aquí van sin los símbolos < >.
  let innerWidth = $state(0)
  let esEstrecho = $derived(innerWidth > 0 && innerWidth <= 900)

  // vistaStats: vista de depuración local, sin ruta de servidor propia. NO se
  // puede usar el PATH /stats para esto: el router (Tarea 13) ya registra
  // GET /stats como el endpoint JSON de verdad, ANTES del fallback SPA — un
  // F5 en ese path serviría el JSON crudo, no la app. Se usa el hash
  // (#stats), que el navegador nunca manda al servidor, así que no puede
  // chocar con ninguna ruta de la API.
  let vistaStats = $state(typeof window !== 'undefined' && window.location.hash === '#stats')

  // Tarea 7 (P0.7): vista de gestión de fuentes, mismo patrón de hash que
  // vistaStats — #fuentes tampoco choca con ninguna ruta del servidor (el
  // router nunca registra ese path; F5 sirve el SPA de siempre).
  let vistaFuentes = $state(typeof window !== 'undefined' && window.location.hash === '#fuentes')

  // Tarea 6 (P0.8): vista de ajustes, mismo patrón de hash que vistaStats/
  // vistaFuentes — #ajustes tampoco choca con ninguna ruta del servidor.
  let vistaAjustes = $state(typeof window !== 'undefined' && window.location.hash === '#ajustes')

  // irAStats/irAFuentes/irAAjustes son el núcleo sin evento de ratón (Tarea 4,
  // P0.8): la paleta de comandos dispara las mismas vistas sin partir de un
  // clic sobre un <a>, así que no tiene un MouseEvent que prevenir. Los
  // manejadores de clic de abajo (abrirStats/abrirFuentes/abrirAjustes) siguen
  // siendo la única puerta para los enlaces reales de la cabecera/pie — un
  // solo sitio que sabe "qué significa ir a stats/fuentes/ajustes", con o sin
  // evento. Las tres vistas son mutuamente excluyentes: cada irA* apaga las
  // otras dos antes de encender la suya.
  function irAStats() {
    vistaFuentes = false
    vistaAjustes = false
    vistaStats = true
    location.hash = 'stats'
  }

  function abrirStats(e: MouseEvent) {
    e.preventDefault()
    irAStats()
  }

  function volverDelPanel() {
    vistaStats = false
    history.pushState('', document.title, window.location.pathname + window.location.search)
  }

  function irAFuentes() {
    vistaStats = false
    vistaAjustes = false
    vistaFuentes = true
    location.hash = 'fuentes'
  }

  function abrirFuentes(e: MouseEvent) {
    e.preventDefault()
    irAFuentes()
  }

  function volverDeFuentes() {
    vistaFuentes = false
    history.pushState('', document.title, window.location.pathname + window.location.search)
  }

  function irAAjustes() {
    vistaStats = false
    vistaFuentes = false
    vistaAjustes = true
    location.hash = 'ajustes'
  }

  function abrirAjustes(e: MouseEvent) {
    e.preventDefault()
    irAAjustes()
  }

  function volverDeAjustes() {
    vistaAjustes = false
    history.pushState('', document.title, window.location.pathname + window.location.search)
  }

  // Tarea 7 (P0.7): "añadir fuente desde la vista Fuentes" reutiliza EXACTO
  // el mismo camino de sondeo que Onboarding (alFuenteAnadida más abajo) —
  // primero se sale de la vista (la nueva fuente ya no es "gestión", es "el
  // catálogo está sincronizando", el mismo estado visible que el onboarding
  // usa) y LUEGO se dispara el sondeo real. Sin este orden, sondeandoFuenteNueva
  // se activaría por debajo de una vista que sigue tapándolo (vistaFuentes se
  // comprueba antes que el sondeo en el <main> de abajo).
  function alFuenteAnadidaDesdeFuentes(f: Fuente) {
    volverDeFuentes()
    alFuenteAnadida(f)
  }

  // Tarea 7 (P0.7): tras un «Quitar» en la vista Fuentes, App recibe la lista
  // fresca del backend (Fuentes.svelte ya la refetcheó) — se adopta tal cual
  // como la propia copia de `fuentes` (gobierna sinFuentes, Tarea 6) y se
  // refresca el catálogo para que los canales de la fuente quitada
  // desaparezcan de la rejilla sin esperar a que el usuario salga de la
  // vista. `fase.tipo === 'listo'` de guardia: cargarPagina(true) solo tiene
  // sentido con el catálogo ya arrancado (mismo guardián que el resto de
  // llamadas a cargarPagina).
  function alFuentesCambiaron(nuevas: Fuente[]) {
    fuentes = nuevas
    // Fix final-review (F3): con `!vistaFuentes` añadido a `sinFuentes` (ver
    // más abajo), quitar la última fuente DESDE esta misma vista ya no
    // bastaba por sí solo para que el Onboarding "volviera solo" —
    // vistaFuentes seguía en true y bloqueaba esa rama. nuevas.length === 0
    // es el mismo estado que sinFuentes vigila (cero fuentes): se resetea
    // vistaFuentes aquí para preservar el comportamiento ya cubierto por el
    // test "(e) quitar la ÚLTIMA fuente... el Onboarding vuelve a aparecer".
    if (nuevas.length === 0) volverDeFuentes()
    if (fase.tipo === 'listo') cargarPagina(true)
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
    // Tarea 10 (P0.6): se registra al ABRIR, no al confirmar reproducción.
    // "Continuar viendo" tiene que recordar qué se intentó ver aunque el
    // mirror fallara y el Reproductor nunca llegara a alConfirmar() — lo
    // contrario (registrar solo un desenlace 'iniciado') dejaría fuera
    // justo los canales problemáticos que más interesa poder reabrir rápido.
    historial.registrar(canal)
  }

  function cerrarReproductor() {
    canalAbierto = null
  }

  // Puente entre EntradaHistorial (solo id/nombre/logo/cuando — lo mínimo
  // que persiste en localStorage) y el Canal completo que pide abrirCanal.
  // Si el canal sigue en la página ya cargada se reutiliza tal cual (mismos
  // datos de salud que vería una tarjeta de la rejilla); si no —history de
  // otra sesión, canal fuera de la página actual, filtro distinto— se
  // reconstruye un Canal mínimo con lo que el historial sí guarda: basta
  // para que el Reproductor abra por id (pide sus propios mirrors/destino)
  // y muestre nombre/logo, aunque sin badges de salud hasta que reproduzca.
  function abrirDesdeHistorial(entrada: EntradaHistorial) {
    const enCatalogo = canales.find((c) => c.id === entrada.canalId)
    abrirCanal(
      enCatalogo ?? {
        id: entrada.canalId,
        nombre: entrada.nombre,
        logoUrl: entrada.logoUrl ?? '',
        categoriaId: '',
        idioma: '',
        pais: '',
        vivo: null,
        latenciaMs: 0,
        webOk: null,
      },
    )
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

  // Gesto "surf" (Tarea 11, P0.6): la barra espaciadora repite EXACTAMENTE
  // el camino de "Canal al azar" (alAleatorio → fuente.aleatorio con la
  // consulta del filtro actual → abrirCanal) — un solo lugar que sabe saltar
  // a un canal al azar, dos formas de dispararlo. debeHacerSurf ya filtra
  // teclas/objetivos; aquí solo se añaden las condiciones de CUÁNDO tiene
  // sentido saltar: con el catálogo listo (antes no hay qué surfear) y con
  // el reproductor cerrado (abierto, la barra espaciadora ya es su atajo de
  // play/pausa — ver Reproductor.svelte; dejar que ambos oyentes de
  // window compitan por la misma tecla sería confuso e imprevisible).
  // ⌘K (mac) / Ctrl+K (el resto) abre la paleta de comandos (Tarea 4, P0.8).
  // Nunca Alt+K/AltGr — con altKey no cuenta como el atajo real.
  function esAtajoPaleta(e: KeyboardEvent): boolean {
    return e.key.toLowerCase() === 'k' && (e.metaKey || e.ctrlKey) && !e.altKey
  }

  function alTeclaVentana(e: KeyboardEvent) {
    if (esAtajoPaleta(e)) {
      // RULING (ver `paletaAbierta` más arriba): inhibida con el reproductor
      // abierto. paletaAbierta ya true: no hay nada que hacer dos veces (su
      // propio manejador de teclado, dentro de Paleta.svelte, es quien
      // gobierna el teclado mientras está montada).
      if (canalAbierto || paletaAbierta) return
      // No robarle el atajo a un control AJENO ya interactivo (el buscador de
      // facetas, un <select>, un <button> enfocado, contenido
      // contenteditable…): con el foco ya ahí, ⌘K no hace nada — MISMO
      // criterio que debeHacerSurf aplica más abajo a la barra espaciadora
      // (esObjetivoInteractivo, compartido — Tarea 8, P0.8: antes este guard
      // solo excluía INPUT/TEXTAREA, así que ⌘K con el foco en el segmentado
      // de densidad de Ajustes, o en cualquier <button>, sí que la abría).
      if (esObjetivoInteractivo(e.target)) return
      e.preventDefault()
      paletaAbierta = true
      return
    }

    // sinFuentes (Tarea 6, P0.7): sin catálogo que surfear, el mismo espacio
    // debe dejar que el navegador haga lo suyo (p. ej. scroll) en vez de
    // llamar a fuente.aleatorio() sobre un catálogo que se sabe vacío.
    // sondeandoFuenteNueva/sondeoAgotado (fix round 1): mismo motivo mientras
    // se sondea tras añadir una fuente — el catálogo puede seguir en 0.
    // vistaFuentes (Tarea 7): mismo motivo que vistaStats — es otra vista, no
    // el catálogo normal.
    // paletaAbierta: mismo motivo que canalAbierto — con un modal abierto es
    // ese modal quien gobierna el teclado; sin esta guarda, un clic en el
    // chrome no interactivo de la paleta (la pista, un título de grupo) saca
    // el foco del <input> y la barra espaciadora surfea por debajo,
    // violando el invariante "los dos modales nunca coexisten" (fix, P0.8).
    if (
      canalAbierto ||
      paletaAbierta ||
      fase.tipo !== 'listo' ||
      vistaStats ||
      vistaFuentes ||
      vistaAjustes ||
      sinFuentes ||
      sondeandoFuenteNueva ||
      sondeoAgotado
    )
      return
    if (!debeHacerSurf(e)) return
    e.preventDefault()
    alAleatorio()
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
      versionApp = salud.version
      fase = salud.sincronizando ? { tipo: 'sincronizando' } : { tipo: 'listo' }
    } catch (e) {
      fase = { tipo: 'error', clase: clasificarError(e) }
    }
  }

  function alSincronizado() {
    fase = { tipo: 'listo' }
  }

  // Fix round 1 (Tarea 6, P0.7): PARAR cualquier temporizador de sondeo
  // pendiente. Se llama al empezar un sondeo nuevo (evita dos sondeos
  // corriendo a la vez si alguien pulsa "Reintentar" dos veces) y al
  // desmontar App (ver onDestroy más abajo). Bumpear idSondeoActual aquí
  // (no solo en iniciarSondeoFuente) invalida también un tick YA EN VUELO
  // —esperando la respuesta de cargarPagina— para que, al resolver después
  // de este punto, su comprobación `idPropio !== idSondeoActual` lo
  // descarte: sin esto, un desmontaje a mitad de una petición en curso
  // seguiría escribiendo estado (sondeandoFuenteNueva/sondeoAgotado) sobre
  // un componente ya fuera.
  function detenerSondeoFuente() {
    if (temporizadorSondeo !== undefined) clearTimeout(temporizadorSondeo)
    temporizadorSondeo = undefined
    idSondeoActual++
  }

  // Un tick del sondeo: reutiliza cargarPagina(true) tal cual (mismo camino
  // que ya cubre carga inicial/cambio de filtro, con su propio guardián de
  // peticionActual para descartar respuestas fuera de orden) y comprueba
  // `total` DESPUÉS — nunca duplica la lógica de qué hacer con la página
  // recibida. idPropio descarta este tick si, mientras esperaba la
  // respuesta, un "Reintentar" (u onDestroy) ya invalidó este sondeo.
  async function comprobarSondeoFuente(idPropio: number) {
    await cargarPagina(true)
    if (idPropio !== idSondeoActual) return
    if (total > 0) {
      sondeandoFuenteNueva = false
      return
    }
    intentosSondeo++
    if (intentosSondeo >= sondeoFuenteIntentosMax) {
      sondeandoFuenteNueva = false
      sondeoAgotado = true
      return
    }
    temporizadorSondeo = setTimeout(() => comprobarSondeoFuente(idPropio), sondeoFuenteIntervaloMs)
  }

  function iniciarSondeoFuente() {
    detenerSondeoFuente()
    const idPropio = ++idSondeoActual
    intentosSondeo = 0
    sondeandoFuenteNueva = true
    sondeoAgotado = false
    // Primer intento INMEDIATO (sin esperar el primer intervalo): mismo
    // patrón que el comprobar()+setInterval de Sincronizando.svelte — para
    // listas pequeñas, el backend puede haber terminado de sincronizar ya
    // cuando anadirFuente* devolvió.
    comprobarSondeoFuente(idPropio)
  }

  function reintentarSondeoFuente() {
    iniciarSondeoFuente()
  }

  // Fix final-review (F2): salida real del estado "agotado". Sin esto, la
  // rama sondeo/agotado (primera del <main> de abajo) gana PARA SIEMPRE una
  // vez sondeoAgotado queda en true — el botón «Fuentes» de la cabecera
  // seguía "funcionando" (ponía vistaFuentes=true) pero no se notaba: esa
  // rama sigue evaluándose ANTES que vistaFuentes, así que nada cambiaba en
  // pantalla y quien añadió una fuente rota (typo, lista vacía) se quedaba
  // repitiendo "Reintentar" cada ~90s sin poder llegar nunca a borrarla.
  // Limpiar sondeoAgotado aquí es lo que deja que vistaFuentes gane de
  // verdad (fuentes.length ya no es 0 — la fuente rota SÍ se añadió del lado
  // del backend — así que sinFuentes tampoco se interpone).
  function irAGestionarFuentes() {
    sondeoAgotado = false
    vistaStats = false
    vistaFuentes = true
    location.hash = 'fuentes'
  }

  // Tarea 6 (P0.7) — fix round 1: Onboarding ya recibió la Fuente creada
  // como valor de retorno de anadirFuente*/fuentesSugeridas (el propio
  // backend la crea al sincronizar) — no hace falta un fetch adicional a
  // fuente.fuentes() para saber que ya no está vacía. Lo que SÍ falta es el
  // CATÁLOGO: el backend sincroniza la fuente nueva de forma asíncrona (el
  // gate real en Chrome del controlador cazó esto: un solo refetch
  // inmediato casi siempre ve total=0 todavía, y nada volvía a mirar). La
  // fase 'sincronizando' NO sirve aquí — Sincronizando.svelte hace polling
  // de /health, que refleja la sincronización INICIAL del catálogo entero,
  // no si ESTA fuente concreta ya tiene canales — así que se sondea el
  // catálogo directamente (ver iniciarSondeoFuente).
  function alFuenteAnadida(f: Fuente) {
    fuentes = [...fuentes, f]
    iniciarSondeoFuente()
  }

  // Tarea 6 (P0.8): «recordar vista»/«recordar filtros» (Ajustes.svelte,
  // preferencias.recordarVista/recordarFiltros). Restaurar corre UNA VEZ por
  // montaje, en el cuerpo del <script> — no en onMount ni en un $effect —
  // para que se aplique ANTES de que el $effect de guardado de más abajo
  // (que lee `preferencias`/`filtros` reactivamente) tenga ocasión de
  // ejecutarse por primera vez y volver a escribir el valor por defecto del
  // store encima de lo guardado. `get(preferencias)` es una foto suelta
  // adrede: solo hace falta leerla una vez, al arrancar.
  if (get(preferencias).recordarVista) {
    const vistaGuardada = leerVistaGuardada()
    if (vistaGuardada) filtros.update((f) => ({ ...f, vista: vistaGuardada }))
  }
  if (get(preferencias).recordarFiltros) {
    const filtrosGuardados = leerFiltrosGuardados()
    if (filtrosGuardados) filtros.update((f) => ({ ...f, ...filtrosGuardados }))
  }

  // Efectos de guardado: reactivos a CUALQUIER cambio de `filtros` o de la
  // propia preferencia (encender/apagar «recordar vista/filtros» desde
  // Ajustes persiste — o borra— de inmediato, sin esperar al próximo cambio
  // de filtro). Apagada la preferencia, se borra la clave en vez de dejarla
  // como basura: sin esto, volver a encenderla más tarde restauraría un
  // valor viejo que el usuario nunca pidió recordar en esa sesión.
  $effect(() => {
    if ($preferencias.recordarVista) escribirVistaGuardada($filtros.vista)
    else borrarVistaGuardada()
  })
  $effect(() => {
    if ($preferencias.recordarFiltros) {
      escribirFiltrosGuardados({
        q: $filtros.q ?? '',
        pais: $filtros.pais ?? '',
        categoria: $filtros.categoria ?? '',
        calidad: $filtros.calidad ?? '',
        soloFavoritos: $filtros.soloFavoritos,
      })
    } else {
      borrarFiltrosGuardados()
    }
  })

  onDestroy(() => {
    detenerSondeoFuente()
  })

  onMount(() => {
    comprobarSalud()
    ;(async () => {
      try {
        const [p, c, q, f, fu] = await Promise.all([
          fuente.paises(),
          fuente.categorias(),
          fuente.calidades(),
          fuente.frescura(),
          fuente.fuentes(),
        ])
        paises = p
        categorias = c
        calidades = q
        frescura = f
        fuentes = fu
        // fuentesCargadas se marca SOLO en el camino de éxito, dentro del
        // try: si la petición falla no sabemos si hay fuentes o no, y es más
        // seguro no mostrar el onboarding "sin fuentes" por un fallo de red
        // pasajero que mostrarlo de más (ver el comentario de `fuentes`).
        fuentesCargadas = true
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

  // Tarea 15 (P0.6, hallazgo del ledger): la transición al vacío
  // ("Ningún canal casa con el filtro") no la anunciaba ninguna región
  // aria-live — solo sincronizando/error lo hacían. Vacio.svelte NO lleva su
  // propia semántica live a propósito (mismo principio que Sincronizando/
  // MensajeError: evitar el doble anuncio del fix round 2) — la región
  // polite YA PERSISTENTE de más abajo es la única que debe anunciarlo.
  // MISMA condición que RejillaCanales usa para montar <Vacio> (ver
  // RejillaCanales.svelte: `canales.length === 0 && !cargando`), con
  // 'listo'/sin error/fuera del panel de stats añadidos porque esta cadena
  // vive en App, que ve más fases que RejillaCanales.
  // vistaFuentes (Tarea 7): mismo motivo que vistaStats — con esa vista
  // encima, <main> no monta Vacio.svelte, así que la región persistente no
  // debe anunciar su mensaje.
  let catalogoVacio = $derived(
    fase.tipo === 'listo' &&
      !vistaStats &&
      !vistaFuentes &&
      !vistaAjustes &&
      !errorCatalogo &&
      !cargando &&
      canales.length === 0,
  )

  // Tarea 6 (P0.7): onboarding cuando el catálogo está listo pero NO hay
  // ninguna fuente añadida — a diferencia de catalogoVacio (un FILTRO deja el
  // catálogo sin resultados), aquí el catálogo entero está vacío porque no
  // hay de dónde sacarlo. `total === 0` además de `canales.length === 0`:
  // con soloFavoritos ambos podrían discrepar si algún día hay favoritos sin
  // fuente propia, y total es la fuente de verdad de "cuántos hay" en todo
  // el resto de App (ver el comentario de `cargarPagina`).
  // `!vistaFuentes` (fix final-review, F3): sin este guardián, navegar a
  // #fuentes desde el propio onboarding (fuentes.length sigue en 0 hasta que
  // se añade la primera) dejaba `vistaFuentes` en true PERO esta rama seguía
  // ganando (va antes que vistaFuentes en el <main> de abajo) — el clic en
  // «Fuentes» parecía no hacer nada, y al sincronizar la primera fuente
  // añadida desde el propio Onboarding (que sigue montado, vistaFuentes
  // stuck), sinFuentes pasaba a false y la rama vistaFuentes (nunca
  // reseteada) ganaba: quien acababa de añadir su primera fuente aterrizaba
  // en la vista de GESTIÓN en vez de en su catálogo recién sincronizado. Con
  // el guardián, «Fuentes» siempre abre la vista de gestión de verdad
  // (incluida su propia AnadirFuente, aunque la lista esté vacía) en cuanto
  // se pulsa — nunca un clic que no hace nada visible.
  let sinFuentes = $derived(
    fase.tipo === 'listo' &&
      fuentesCargadas &&
      fuentes.length === 0 &&
      !vistaStats &&
      !vistaFuentes &&
      !vistaAjustes &&
      !errorCatalogo &&
      !cargando &&
      canales.length === 0 &&
      total === 0,
  )

  // `catalogoVacio && !sinFuentes`: cuando NO hay fuentes, App renderiza
  // Onboarding en vez de RejillaCanales/Vacio (ver el <main> más abajo) — sin
  // este guardián, esta región seguiría anunciando "Ningún canal casa con el
  // filtro" (el texto de Vacio.svelte, pensado para un FILTRO demasiado
  // estrecho) encima de una pantalla que en realidad pide una fuente, un
  // mensaje falso para quien lo escucha por lector de pantalla.
  //
  // Fix round 1 (Tarea 6, P0.7): sondeandoFuenteNueva/sondeoAgotado se
  // añaden a esta MISMA cadena derivada (nunca una región nueva, mismo
  // invariante que el resto de este bloque) — cubren la transición "fuente
  // añadida, sincronizando…" y, si se agota el tope, el aviso de que
  // todavía no hay canales.
  let mensajePoliteAccesible = $derived(
    fase.tipo === 'sincronizando'
      ? t('estado.sincronizando')
      : sondeandoFuenteNueva
        ? t('onboarding.sondeo.titulo')
        : sondeoAgotado
          ? t('onboarding.sondeo.agotado')
          : catalogoVacio && !sinFuentes
            ? t('catalogo.vacio')
            : '',
  )
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
     duplicadas, y su anuncio no depende de que el fondo esté o no inert.
     La polite (Tarea 15) cubre AHORA dos transiciones —sincronizando y
     vacío— con el mismo nodo, nunca dos regiones compitiendo. -->
<div class="sr-only" aria-live="polite" aria-atomic="true">{mensajePoliteAccesible}</div>
<div class="sr-only" role="alert" aria-live="assertive" aria-atomic="true">{mensajeErrorAccesible}</div>

<!-- Tarea 11 (P0.6): gesto "surf". svelte:window en vez de un listener en un
     nodo concreto porque el espacio puede pulsarse con el foco en cualquier
     parte de la página (o en ningún control) — debeHacerSurf es la guarda
     que decide si ESE objetivo concreto puede robarle el espacio.
     bind:innerWidth (fix round 1, Tarea 15): mismo <svelte:window> que ya
     existía, un solo nodo — alimenta esEstrecho para el inert del cajón. -->
<svelte:window onkeydown={alTeclaVentana} bind:innerWidth />

<div class="fondo" inert={!!canalAbierto || paletaAbierta}>
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
        <!-- Tarea 7 (P0.7): punto de acceso a la gestión de fuentes — en la
             cabecera (siempre visible, en cualquier fase/vista), no escondido
             en el pie como #stats: es una función primaria del producto
             bring-your-own, no una vista de depuración. -->
        <button type="button" class="fuentes-link" onclick={abrirFuentes}>
          {t('fuentes.abrir')}
        </button>
        <!-- Tarea 6 (P0.8): punto de acceso discreto a Ajustes — mismo sitio
             y mismo estilo neutro que «Fuentes» (constraint global: ámbar
             solo para el acento/acción activa; esto es navegación). -->
        <button type="button" class="ajustes-link" onclick={abrirAjustes}>
          {t('ajustes.abrir')}
        </button>
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

      <!-- Fix round 1 (Tarea 15): en viewport estrecho el cajón cerrado se
           saca de pantalla solo con transform (ver <style> .facetas bajo el
           breakpoint) — sin inert, sus controles (buscador, filas de
           facetas) seguían siendo alcanzables por Tab estando invisibles. Se
           gatea con inert SOLO cuando esEstrecho Y está cerrado: en
           escritorio (esEstrecho=false) la barra lateral es la columna fija
           de siempre, nunca inert pase lo que pase lateralAbierto; con el
           cajón abierto (lateralAbierto=true), tampoco. El botón que lo abre
           vive FUERA de este aside (arriba, en .cuerpo), así que sigue
           siendo alcanzable incluso con el aside inert. -->
      <aside
        class="facetas"
        id="panel-facetas"
        data-abierto={lateralAbierto}
        inert={esEstrecho && !lateralAbierto}
      >
        <!-- Tarea 6 (P0.6): contenido real de las facetas. -->
        <h2 class="sr-only">{t('shell.facetas')}</h2>
        <BarraLateralFacetas {paises} {categorias} {calidades} />
      </aside>

      <main inert={!!canalAbierto || paletaAbierta}>
        {#if vistaAjustes}
          <!-- Tarea 6 (P0.8): vista de ajustes, alcanzada por #ajustes (botón
               de la cabecera) — TOP-LEVEL como vistaStats, no anidada bajo
               fase.tipo==='listo': las preferencias tienen sentido aunque el
               catálogo siga sincronizando o haya dado error. -->
          <Ajustes alVolver={volverDeAjustes} version={versionApp} />
        {:else if vistaStats}
          <PanelStats alVolver={volverDelPanel} />
        {:else if fase.tipo === 'sincronizando'}
          <Sincronizando alListo={alSincronizado} />
        {:else if fase.tipo === 'error'}
          <MensajeError clase={fase.clase} />
        {:else if fase.tipo === 'listo'}
          {#if sondeandoFuenteNueva || sondeoAgotado}
            <!-- Fix round 1 (Tarea 6, P0.7): tras añadir una fuente, ANTES de
                 volver al Onboarding o al shell normal — se comprueba
                 primero que sinFuentes (justo abajo), porque fuentes ya deja
                 de estar vacío en cuanto alFuenteAnadida la añade, pero el
                 catálogo (total) todavía puede seguir en 0 mientras el
                 backend sincroniza. Sin esta rama PRIMERO, sinFuentes pasaría
                 a false y se vería la rejilla vacía de siempre — exactamente
                 el bug real que cazó el gate en Chrome del controlador. -->
            <SincronizandoFuente
              agotado={sondeoAgotado}
              alReintentar={reintentarSondeoFuente}
              alGestionarFuentes={irAGestionarFuentes}
            />
          {:else if sinFuentes}
            <!-- Tarea 6 (P0.7): catálogo listo pero sin ninguna fuente
                 bring-your-own — primer arranque en limpio. Sustituye TODO
                 el bloque de abajo (héroe/acciones/pista de surf/rejilla): no
                 tiene sentido ofrecer "Canal al azar" o una rejilla sobre un
                 catálogo que se sabe vacío por falta de fuente, no por un
                 filtro. El aside de facetas (arriba) se deja tal cual —
                 vacío hasta que haya canales, pero sin recablear su propio
                 inert/layout por este caso.

                 Tarea 7 (P0.7): esta rama va ANTES que vistaFuentes (justo
                 abajo) a propósito — si se quita la última fuente desde la
                 vista de gestión, sinFuentes pasa a true y esta rama gana,
                 así que el Onboarding "vuelve solo" sin que Fuentes.svelte
                 tenga que saber nada de eso (ver alFuentesCambiaron). -->
            <Onboarding {fuente} {alFuenteAnadida} />
          {:else if vistaFuentes}
            <!-- Tarea 7 (P0.7): vista de gestión de fuentes, alcanzada por
                 #fuentes (botón de la cabecera). alFuenteAnadida está
                 envuelta (alFuenteAnadidaDesdeFuentes) para que "añadir desde
                 aquí" cierre esta vista y dispare el MISMO sondeo que el
                 onboarding — nunca una segunda lógica de sondeo. -->
            <Fuentes
              {fuente}
              alVolver={volverDeFuentes}
              alFuenteAnadida={alFuenteAnadidaDesdeFuentes}
              {alFuentesCambiaron}
            />
          {:else}
            <!-- Tarea 10 (P0.6): héroe "Continuar viendo", ENCIMA de
                 BarraAcciones — se renderiza compacto o nada, según el
                 historial (ver ContinuarViendo.svelte). -->
            <ContinuarViendo alAbrir={abrirDesdeHistorial} />

            <!-- Tarea 7 (P0.6): buscador/facetas/señal viven en el aside
                 (BarraLateralFacetas, Tarea 6); "Solo favoritos", "Canal al
                 azar", el conmutador de vista, los chips de filtro removibles
                 y el conteo viven aquí, en BarraAcciones. -->
            <BarraAcciones {total} {frescura} {alAleatorio} />

            <!-- Afordancia del gesto "surf" (Tarea 11): sin esto, la barra
                 espaciadora sería un atajo invisible que nadie descubre. El
                 cursor ámbar parpadeante es puramente decorativo (el texto ya
                 dice lo mismo), de ahí aria-hidden en el propio glifo. -->
            <p class="pista-surf">
              <span class="cursor-surf" aria-hidden="true">&lt;_</span>
              {t('accion.surf')}
            </p>

            {#if errorCatalogo}
              <MensajeError clase={errorCatalogo} />
            {:else}
              <!-- Tarea 5 (P0.8): densidad de la rejilla viene de preferencias
                   (localStorage) — la UI para cambiarla es la Tarea 6
                   (#ajustes); aquí solo se lee y se reenvía. -->
              <RejillaCanales
                {canales}
                vista={$filtros.vista}
                {cargando}
                {alPedirMas}
                alAbrir={abrirCanal}
                densidad={$preferencias.densidad}
              />
            {/if}
          {/if}
        {/if}
      </main>
    </div>
  </div>

  <footer class="pie" inert={!!canalAbierto || paletaAbierta}>
    <p>{t('pie.fuente')}</p>
    <p>{t('pie.postura')}</p>
    {#if !vistaStats}
      <p><a class="stats" href="#stats" onclick={abrirStats}>{t('pie.stats')}</a></p>
    {/if}
    <!-- Tarea 10 (P0.7): pie de crédito de marca, INTEGRADO en este mismo
         <footer> (no un segundo <footer> compitiendo) — comparte el mismo
         boundary inert de arriba y el mismo contenedor .fondo. El propio
         componente aporta su borde superior hairline para separarse
         visualmente de las líneas de arriba. -->
    <PieDeMarca />
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

{#if paletaAbierta}
  <!-- Tarea 4 (P0.8): montada FUERA de .fondo, como Reproductor arriba —
       vive su propio ciclo de foco (ver Paleta.svelte) mientras .fondo/
       main/footer quedan inert (arriba). Nunca coexiste con canalAbierto
       (RULING: ⌘K inhibida con el reproductor abierto — ver alTeclaVentana). -->
  <Paleta
    {canales}
    {paises}
    {categorias}
    {calidades}
    alCerrar={cerrarPaleta}
    alAbrirCanal={abrirCanal}
    {alAleatorio}
    alAbrirFuentes={irAFuentes}
    alAbrirStats={irAStats}
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
  /* Mismo estilo neutro que .idioma (constraint global: ámbar solo para el
     acento/acción activa — esto es navegación, no una CTA). */
  .fuentes-link {
    background: none; border: 1px solid var(--border-default); color: var(--text-body);
    border-radius: 6px; padding: 4px 10px; cursor: pointer;
  }
  .ajustes-link {
    background: none; border: 1px solid var(--border-default); color: var(--text-body);
    border-radius: 6px; padding: 4px 10px; cursor: pointer;
  }
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

  /* Afordancia del gesto "surf" (Tarea 11, P0.6). */
  .pista-surf {
    display: flex;
    align-items: center;
    gap: var(--space-2, 8px);
    margin: 0 0 var(--space-3, 12px);
    color: var(--text-muted);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono);
  }
  .cursor-surf {
    color: var(--amber-500);
    animation: parpadeo-surf 1s step-end infinite;
  }
  @keyframes parpadeo-surf {
    50% { opacity: 0; }
  }
  @media (prefers-reduced-motion: reduce) {
    .cursor-surf {
      animation: none;
    }
  }

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
