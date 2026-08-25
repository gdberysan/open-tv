<script lang="ts">
  import { onMount } from 'svelte'
  import { get } from 'svelte/store'
  import { idioma, t } from './i18n'
  import { crearHttpCatalog } from './datos/http'
  import type { Canal, ConsultaCatalogo, Faceta } from './datos/catalogo'
  import { filtros } from './estado/filtros'
  import { favoritos } from './estado/favoritos'
  import BarraFiltros from './componentes/BarraFiltros.svelte'
  import RejillaCanales from './componentes/RejillaCanales.svelte'

  // La página son 500 canales, el máximo que acepta el gateway (Tarea 11).
  const PAGINA = 500

  // Único punto del cliente que habla con CatalogSource: todos los demás
  // componentes leen stores y emiten callbacks.
  const catalogo = crearHttpCatalog()

  let canales = $state<Canal[]>([])
  let total = $state(0)
  let cargando = $state(false)
  let error = $state<string | null>(null)
  let desplazamiento = $state(0)
  let paises = $state<Faceta[]>([])
  let categorias = $state<Faceta[]>([])

  // Descarta respuestas de peticiones que ya no son la última: cambiar de
  // filtro dos veces seguidas no puede dejar pintada la respuesta de la
  // primera si llega después que la de la segunda.
  let peticionActual = 0

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
    return paginar ? { ...base, limite: PAGINA, desplazamiento } : base
  }

  function mensajeError(e: unknown): string {
    const bruto = e instanceof Error ? e.message : String(e)
    return /red/i.test(bruto) ? t('estado.sinRed') : t('estado.gatewayCaido')
  }

  async function cargarPagina(reiniciar: boolean) {
    const idPeticion = ++peticionActual
    if (reiniciar) {
      desplazamiento = 0
      canales = []
    }
    cargando = true
    try {
      const pagina = await catalogo.canales(construirConsulta(true))
      if (idPeticion !== peticionActual) return // ya hay una consulta más nueva en marcha
      canales = reiniciar ? pagina.canales : [...canales, ...pagina.canales]
      // El total sale de X-Total-Count (vía HttpCatalog), nunca de
      // canales.length: con paginación son números distintos.
      total = pagina.total
      error = null
      if (!get(filtros).soloFavoritos) desplazamiento += pagina.canales.length
    } catch (e) {
      if (idPeticion !== peticionActual) return
      error = mensajeError(e)
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
    // El reproductor llega en la Tarea 13. De momento no hay a dónde abrir.
    console.debug('open-tv: abrir canal', canal.id)
  }

  async function alAleatorio() {
    try {
      abrirCanal(await catalogo.aleatorio(construirConsulta(false)))
    } catch (e) {
      error = mensajeError(e)
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
    // Cambiar cualquier filtro reinicia el desplazamiento y vacía la lista;
    // el scroll infinito solo suma páginas.
    cargarPagina(true)
  })

  onMount(async () => {
    try {
      const [p, c] = await Promise.all([catalogo.paises(), catalogo.categorias()])
      paises = p
      categorias = c
    } catch {
      // Sin facetas los selectores se quedan solo con "Todos"; no es motivo
      // para tumbar el resto de la app.
    }
  })

  function alternarIdioma() {
    idioma.update((v) => (v === 'es' ? 'en' : 'es'))
  }
</script>

<main>
  <header>
    <h1>{t('app.titulo')}</h1>
    <p class="lema">{t('app.lema')}</p>
    <button type="button" class="idioma" onclick={alternarIdioma}>
      {$idioma === 'es' ? t('idioma.en') : t('idioma.es')}
    </button>
  </header>

  <BarraFiltros {paises} {categorias} {alAleatorio} />

  {#if error}
    <p class="error">{error}</p>
  {:else}
    <p class="total">{t('catalogo.total', { n: total })}</p>
    <RejillaCanales {canales} vista={$filtros.vista} {cargando} {alPedirMas} alAbrir={abrirCanal} />
  {/if}
</main>

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
  .total { color: var(--text-muted); font-size: 13px; margin: 4px 0 12px; }
  .error { color: var(--signal-error); }
</style>
