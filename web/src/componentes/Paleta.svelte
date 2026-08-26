<script lang="ts">
  // Paleta de comandos ⌘K/Ctrl+K (Tarea 4, P0.8). App.svelte es quien decide
  // CUÁNDO montarla (el atajo global y la exclusión mientras el reproductor
  // está abierto viven allí — ver el comentario largo junto a `paletaAbierta`
  // en App.svelte); este componente solo sabe DIBUJARSE y responder al
  // teclado/ratón mientras existe. Igual que BarraLateralFacetas/
  // BarraAcciones/ChipsFiltro, escribe directamente en el store compartido
  // `filtros` — eso no es "meterse en App", es el mismo store que ya usan
  // esos tres componentes; lo que NO se hace aquí es leer estado interno de
  // App (canalAbierto, vistaFuentes…), que llega vía props/callbacks.
  import { onDestroy, onMount, tick } from 'svelte'
  import type { Canal, Faceta } from '../datos/catalogo'
  import { filtros } from '../estado/filtros'
  import { coincideDifuso } from '../lib/fuzzy'
  import { nombreDePais } from '../lib/paises'
  import { idioma, t } from '../i18n'

  let {
    canales,
    paises,
    categorias,
    calidades,
    alCerrar,
    alAbrirCanal,
    alAleatorio,
    alAbrirFuentes,
    alAbrirStats,
  }: {
    canales: Canal[]
    paises: Faceta[]
    categorias: Faceta[]
    calidades: Faceta[]
    alCerrar: () => void
    alAbrirCanal: (canal: Canal) => void
    alAleatorio: () => void
    alAbrirFuentes: () => void
    alAbrirStats: () => void
  } = $props()

  // Tope por categoría (brief, Tarea 4): con 8k+ canales cargados, pintar más
  // de un puñado de filas por grupo no aporta nada (nadie lee 200 resultados)
  // y es la parte cara de cada tecla — se recorta ANTES de tocar el DOM.
  const LIMITE_POR_GRUPO = 20

  interface Opcion {
    id: string
    etiqueta: string
    puntaje: number
    ejecutar: () => void
  }

  let termino = $state('')
  let indiceActivo = $state(0)
  let inputEl = $state<HTMLInputElement>()
  let elementoPrevio: HTMLElement | null = null

  // Duplicado a propósito de BarraLateralFacetas/ChipsFiltro: mismo mapa de
  // tres líneas, mismo motivo (no vale acoplar componentes de UI a un módulo
  // compartido por esto).
  const ETIQUETAS_CALIDAD: Record<string, () => string> = {
    hd: () => t('filtro.calidad.hd'),
    fhd: () => t('filtro.calidad.fhd'),
    '4k': () => t('filtro.calidad.4k'),
  }
  function etiquetaCalidad(valor: string): string {
    return ETIQUETAS_CALIDAD[valor]?.() ?? valor
  }

  // Filtra+ordena+recorta un candidato genérico contra `termino` con el
  // matcher barato de la Tarea 3 (coincideDifuso) — mismo patrón en las tres
  // categorías, para no repetir la lógica de puntuar/ordenar/cortar tres
  // veces con matices distintos.
  function filtrarYPuntuar<T>(items: T[], etiquetaDe: (item: T) => string): { item: T; puntaje: number }[] {
    const puntuados: { item: T; puntaje: number }[] = []
    for (const item of items) {
      const puntaje = coincideDifuso(termino, etiquetaDe(item))
      if (puntaje !== null) puntuados.push({ item, puntaje })
    }
    // sort() es estable (ES2019+): a igual puntaje, se conserva el orden de
    // llegada (el orden del catálogo/facetas ya cargado).
    puntuados.sort((a, b) => b.puntaje - a.puntaje)
    return puntuados.slice(0, LIMITE_POR_GRUPO)
  }

  // Grupo 1: Canales — fuzzy sobre los YA CARGADOS en memoria (instantáneo,
  // sin red). Con `termino` vacío, coincideDifuso devuelve el puntaje neutro
  // para todos (Tarea 3): el resultado es "los primeros N canales cargados",
  // un valor por defecto razonable antes de teclear nada.
  let opcionesCanal = $derived.by((): Opcion[] =>
    filtrarYPuntuar(canales, (c) => c.nombre).map(({ item: canal, puntaje }) => ({
      id: `canal:${canal.id}`,
      etiqueta: canal.nombre,
      puntaje,
      ejecutar: () => {
        alAbrirCanal(canal)
        cerrar()
      },
    })),
  )

  // Fila de acción "Buscar «X» en todos los canales" (brief): reutiliza la
  // búsqueda de servidor ya existente (filtros.q) para lo que la lista
  // cargada en memoria no puede responder — no compite por el tope de
  // arriba, vive fuera de él y solo aparece con término no vacío.
  let opcionBuscarTodos = $derived.by((): Opcion | null => {
    const q = termino.trim()
    if (!q) return null
    return {
      id: 'buscar-todos',
      etiqueta: t('paleta.buscarTodos', { termino: q }),
      puntaje: 0,
      ejecutar: () => {
        $filtros.q = q
        cerrar()
      },
    }
  })

  // Grupo 2: Facetas — país (nombreDePais para mostrar, el CÓDIGO es lo que
  // escribe en filtros — mismo contrato que BarraLateralFacetas), categoría,
  // calidad. Las tres dimensiones se funden en una sola lista candidata para
  // que el tope de 20 sea del GRUPO, no de cada dimensión por separado.
  interface CandidatoFaceta {
    dimension: 'pais' | 'categoria' | 'calidad'
    valor: string
    etiqueta: string
  }
  let candidatosFaceta = $derived.by((): CandidatoFaceta[] => [
    ...paises.map((f) => ({ dimension: 'pais' as const, valor: f.valor, etiqueta: nombreDePais(f.valor, idioma.actual) })),
    ...categorias.map((f) => ({ dimension: 'categoria' as const, valor: f.valor, etiqueta: f.valor })),
    ...calidades.map((f) => ({ dimension: 'calidad' as const, valor: f.valor, etiqueta: etiquetaCalidad(f.valor) })),
  ])
  function tituloDimension(d: CandidatoFaceta['dimension']): string {
    return d === 'pais' ? t('filtro.pais') : d === 'categoria' ? t('filtro.categoria') : t('filtro.calidad')
  }
  let opcionesFaceta = $derived.by((): Opcion[] =>
    filtrarYPuntuar(candidatosFaceta, (f) => f.etiqueta).map(({ item: f, puntaje }) => ({
      id: `faceta:${f.dimension}:${f.valor}`,
      etiqueta: `${tituloDimension(f.dimension)}: ${f.etiqueta}`,
      puntaje,
      ejecutar: () => {
        $filtros[f.dimension] = f.valor
        cerrar()
      },
    })),
  )

  // Grupo 3: Acciones — lista estática de comandos; se filtran/ordenan con el
  // mismo matcher, para que teclear "fav" o "azar" también los encuentre.
  // "Surf" y "Canal al azar" son, a propósito, EL MISMO camino (alAleatorio):
  // el gesto "surf" (barra espaciadora) no es más que ese mismo salto — dos
  // filas findables por dos nombres distintos, una sola acción real.
  function limpiarFiltros() {
    $filtros.q = ''
    $filtros.pais = ''
    $filtros.categoria = ''
    $filtros.calidad = ''
    $filtros.soloFavoritos = false
  }
  let accionesBase = $derived.by((): Omit<Opcion, 'puntaje'>[] => [
    { id: 'accion:aleatorio', etiqueta: t('accion.aleatorio'), ejecutar: () => { alAleatorio(); cerrar() } },
    { id: 'accion:surf', etiqueta: t('paleta.accion.surf'), ejecutar: () => { alAleatorio(); cerrar() } },
    {
      id: 'accion:vista',
      etiqueta: $filtros.vista === 'rejilla' ? t('accion.lista') : t('accion.rejilla'),
      ejecutar: () => { $filtros.vista = $filtros.vista === 'rejilla' ? 'lista' : 'rejilla'; cerrar() },
    },
    {
      id: 'accion:favoritos',
      etiqueta: $filtros.soloFavoritos ? t('paleta.accion.quitarSoloFavoritos') : t('accion.favoritos'),
      ejecutar: () => { $filtros.soloFavoritos = !$filtros.soloFavoritos; cerrar() },
    },
    { id: 'accion:limpiar', etiqueta: t('filtro.limpiar'), ejecutar: () => { limpiarFiltros(); cerrar() } },
    { id: 'accion:fuentes', etiqueta: t('paleta.accion.abrirFuentes'), ejecutar: () => { alAbrirFuentes(); cerrar() } },
    { id: 'accion:stats', etiqueta: t('paleta.accion.abrirStats'), ejecutar: () => { alAbrirStats(); cerrar() } },
  ])
  let opcionesAccion = $derived.by((): Opcion[] =>
    filtrarYPuntuar(accionesBase, (a) => a.etiqueta).map(({ item, puntaje }) => ({ ...item, puntaje })),
  )

  // Lista plana en el mismo orden visual (Canales → Buscar todos → Facetas →
  // Acciones): es sobre ESTA lista que se mueven las flechas y se resuelve
  // aria-activedescendant — el combobox no distingue grupos al navegar,
  // solo al agrupar visualmente.
  let opcionesPlanas = $derived.by((): Opcion[] => [
    ...opcionesCanal,
    ...(opcionBuscarTodos ? [opcionBuscarTodos] : []),
    ...opcionesFaceta,
    ...opcionesAccion,
  ])

  // Cambiar de término reinicia la selección a la primera opción: sin esto,
  // el índice activo podría apuntar a una fila que ya no existe (o a una
  // completamente distinta) tras cada tecla.
  $effect(() => {
    termino
    indiceActivo = 0
  })

  function idDeOpcion(o: Opcion): string {
    return `paleta-opcion-${o.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`
  }
  let idActivo = $derived(opcionesPlanas[indiceActivo] ? idDeOpcion(opcionesPlanas[indiceActivo]) : undefined)

  function activar(opcion: Opcion) {
    const i = opcionesPlanas.findIndex((o) => o.id === opcion.id)
    if (i >= 0) indiceActivo = i
  }

  function elegir(opcion: Opcion | undefined) {
    opcion?.ejecutar()
  }

  function cerrar() {
    alCerrar()
  }

  function alTeclado(e: KeyboardEvent) {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        if (opcionesPlanas.length > 0) indiceActivo = (indiceActivo + 1) % opcionesPlanas.length
        break
      case 'ArrowUp':
        e.preventDefault()
        if (opcionesPlanas.length > 0) indiceActivo = (indiceActivo - 1 + opcionesPlanas.length) % opcionesPlanas.length
        break
      case 'Enter':
        e.preventDefault()
        elegir(opcionesPlanas[indiceActivo])
        break
      case 'Escape':
        e.preventDefault()
        cerrar()
        break
      case 'Tab':
        // Atrapa el foco (brief): el input es el ÚNICO elemento de la paleta
        // en el orden de tabulación — las opciones llevan tabindex="-1" (solo
        // para que el linter de a11y no se queje del rol "option" sin foco
        // propio) y nunca reciben Tab: patrón combobox de foco VIRTUAL vía
        // aria-activedescendant, no Tab entre filas. Así que Tab no tiene a
        // dónde ir salvo quedarse en el input.
        e.preventDefault()
        break
    }
  }

  // Foco al abrir / restaurarlo al cerrar — mismo patrón que Reproductor.svelte
  // (el otro único modal de la app): recordar qué tenía el foco ANTES de
  // montar, y devolvérselo al desmontar. El rAF en onDestroy es necesario por
  // el mismo motivo documentado allí: App quita el `inert` del fondo Y
  // desmonta este componente en el MISMO flush de Svelte (paletaAbierta pasa
  // a false), así que sin diferir un frame el nodo previo podría seguir
  // dentro de un contenedor todavía inert en el instante de este onDestroy.
  onMount(() => {
    elementoPrevio = document.activeElement instanceof HTMLElement ? document.activeElement : null
    tick().then(() => inputEl?.focus())
  })

  onDestroy(() => {
    const previo = elementoPrevio
    requestAnimationFrame(() => {
      if (previo && document.body.contains(previo)) previo.focus()
    })
  })
</script>

<!-- Capa de fondo: puramente un gesto de ratón ("clic fuera cierra"), Escape
     ya cubre el cierre por teclado. role="presentation" porque no aporta
     semántica propia — es el propio diálogo, más abajo, el que lleva
     role="dialog". -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="capa" role="presentation" onclick={cerrar}>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="paleta"
    role="dialog"
    aria-modal="true"
    aria-label={t('paleta.titulo')}
    tabindex="-1"
    onclick={(e) => e.stopPropagation()}
  >
    <div class="campo">
      <span class="motivo" aria-hidden="true">&gt;</span>
      <input
        type="text"
        class="entrada"
        role="combobox"
        aria-expanded="true"
        aria-controls="paleta-listbox"
        aria-autocomplete="list"
        aria-activedescendant={idActivo}
        aria-label={t('paleta.placeholder')}
        placeholder={t('paleta.placeholder')}
        bind:value={termino}
        bind:this={inputEl}
        onkeydown={alTeclado}
      />
    </div>

    <div id="paleta-listbox" role="listbox" aria-label={t('paleta.titulo')} class="lista">
      {#if opcionesCanal.length > 0 || opcionBuscarTodos}
        <div class="grupo" role="group" aria-label={t('paleta.grupo.canales')}>
          <h3 class="titulo-grupo">{t('paleta.grupo.canales')}</h3>
          {#each opcionesCanal as opcion (opcion.id)}
            <div
              id={idDeOpcion(opcion)}
              role="option"
              tabindex="-1"
              aria-selected={opcion.id === opcionesPlanas[indiceActivo]?.id}
              class="opcion"
              class:activa={opcion.id === opcionesPlanas[indiceActivo]?.id}
              onmouseenter={() => activar(opcion)}
              onclick={() => elegir(opcion)}
            >
              {opcion.etiqueta}
            </div>
          {/each}
          {#if opcionBuscarTodos}
            <div
              id={idDeOpcion(opcionBuscarTodos)}
              role="option"
              tabindex="-1"
              aria-selected={opcionBuscarTodos.id === opcionesPlanas[indiceActivo]?.id}
              class="opcion opcion-accion"
              class:activa={opcionBuscarTodos.id === opcionesPlanas[indiceActivo]?.id}
              onmouseenter={() => activar(opcionBuscarTodos as Opcion)}
              onclick={() => elegir(opcionBuscarTodos as Opcion)}
            >
              {opcionBuscarTodos.etiqueta}
            </div>
          {/if}
        </div>
      {/if}

      {#if opcionesFaceta.length > 0}
        <div class="grupo" role="group" aria-label={t('paleta.grupo.facetas')}>
          <h3 class="titulo-grupo">{t('paleta.grupo.facetas')}</h3>
          {#each opcionesFaceta as opcion (opcion.id)}
            <div
              id={idDeOpcion(opcion)}
              role="option"
              tabindex="-1"
              aria-selected={opcion.id === opcionesPlanas[indiceActivo]?.id}
              class="opcion"
              class:activa={opcion.id === opcionesPlanas[indiceActivo]?.id}
              onmouseenter={() => activar(opcion)}
              onclick={() => elegir(opcion)}
            >
              {opcion.etiqueta}
            </div>
          {/each}
        </div>
      {/if}

      {#if opcionesAccion.length > 0}
        <div class="grupo" role="group" aria-label={t('paleta.grupo.acciones')}>
          <h3 class="titulo-grupo">{t('paleta.grupo.acciones')}</h3>
          {#each opcionesAccion as opcion (opcion.id)}
            <div
              id={idDeOpcion(opcion)}
              role="option"
              tabindex="-1"
              aria-selected={opcion.id === opcionesPlanas[indiceActivo]?.id}
              class="opcion opcion-accion"
              class:activa={opcion.id === opcionesPlanas[indiceActivo]?.id}
              onmouseenter={() => activar(opcion)}
              onclick={() => elegir(opcion)}
            >
              {opcion.etiqueta}
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <p class="pista">{t('paleta.pista')}</p>
  </div>
</div>

<style>
  .capa {
    position: fixed;
    inset: 0;
    z-index: var(--z-modal);
    background: var(--backdrop);
    -webkit-backdrop-filter: var(--blur-panel);
    backdrop-filter: var(--blur-panel);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 12vh;
    /* Entrada (Tarea 7, P0.8): monta/desmonta una vez con {#if paletaAbierta}
       en App.svelte, nunca en bucle — un fundido corto del velo. */
    animation: entrada-capa var(--dur-fast) var(--ease-out);
  }
  @keyframes entrada-capa {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .paleta {
    width: min(560px, 92vw);
    max-height: 70vh;
    display: flex;
    flex-direction: column;
    background: var(--surface-card);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    box-shadow: var(--shadow-panel);
    overflow: hidden;
    /* El propio diálogo entra con un pequeño "settle" además del fundido
       del velo — opacity+translateY, compositables en GPU. */
    animation: entrada-paleta var(--dur-base) var(--ease-out);
  }
  @keyframes entrada-paleta {
    from { opacity: 0; transform: translateY(calc(-1 * var(--space-2, 8px))); }
    to { opacity: 1; transform: translateY(0); }
  }

  .campo {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-default);
    flex-shrink: 0;
  }
  .motivo {
    color: var(--amber-500);
    font: var(--type-mono, inherit);
  }
  .entrada {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-body);
    font: var(--type-body, inherit);
    min-width: 0;
  }
  .entrada:focus {
    outline: none;
  }

  .lista {
    overflow-y: auto;
    padding: 8px;
  }

  .grupo + .grupo {
    margin-top: 8px;
  }
  .titulo-grupo {
    margin: 4px 8px;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
    text-transform: uppercase;
  }

  .opcion {
    padding: 8px 10px;
    border-radius: var(--radius-sm, 5px);
    color: var(--text-body);
    cursor: pointer;
  }
  .opcion-accion {
    color: var(--text-accent);
  }
  /* Ámbar = opción activa (única regla de color de selección/foco de este
     componente): el foco DOM real se queda en el input; esto es el foco
     "virtual" del combobox (aria-activedescendant). */
  .opcion.activa {
    background: var(--tint-amber-weak);
    color: var(--amber-500);
  }

  .pista {
    margin: 0;
    padding: 8px 16px;
    border-top: 1px solid var(--border-default);
    color: var(--text-muted);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono);
    flex-shrink: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .capa, .paleta { animation: none; }
  }
</style>
