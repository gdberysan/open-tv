<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte'
  import type { CatalogSource, Fuente } from '../datos/catalogo'
  import { t } from '../i18n'
  import AnadirFuente from './AnadirFuente.svelte'

  // Tarea 7 (P0.7): vista de gestión de fuentes (#fuentes) — lista lo ya
  // añadido (label/url/kind/ultimoSync/canales), permite re-sincronizar o
  // quitar, y reutiliza AnadirFuente.svelte para añadir más desde aquí mismo.
  //
  // Este componente habla con CatalogSource directamente (fuentes/
  // resyncFuente/quitarFuente), igual que Onboarding ya hace con
  // fuentesSugeridas/anadirFuente — el mismo patrón que la Tarea 6 estableció:
  // App sigue siendo el único punto que decide QUÉ VISTA se muestra, pero los
  // componentes que gestionan fuentes hablan con `fuente` ellos mismos.
  //
  // alFuenteAnadida: se reenvía TAL CUAL a AnadirFuente — es la misma función
  // que App le pasa a Onboarding (envuelta por App para además cerrar esta
  // vista y arrancar su sondeo de sincronización, ver App.svelte). Así "añadir
  // desde Fuentes" dispara EXACTAMENTE la misma UX de sondeo que el onboarding
  // — gratis, por construcción, sin duplicar esa lógica aquí.
  //
  // alFuentesCambiaron: se llama con la lista fresca tras un quitar (siempre
  // via refetch, nunca un filter local — ver quitar()) para que App
  // mantenga su propia copia de `fuentes` (gobierna `sinFuentes`, Tarea 6) y
  // refresque el catálogo — los canales de la fuente quitada tienen que
  // desaparecer de la rejilla aunque el usuario nunca salga de esta vista.
  let { fuente, alVolver, alFuenteAnadida, alFuentesCambiaron }: {
    fuente: CatalogSource
    alVolver: () => void
    alFuenteAnadida: (f: Fuente) => void
    alFuentesCambiaron: (fuentes: Fuente[]) => void
  } = $props()

  type Estado = { tipo: 'cargando' } | { tipo: 'error' } | { tipo: 'listo' }
  let estado = $state<Estado>({ tipo: 'cargando' })
  let fuentes = $state<Fuente[]>([])

  // Fila en curso de re-sincronizar (brief: indicación breve, NO el sondeo
  // completo del onboarding — un "sincronizando…" optimista seguido de un
  // refetch de fuentes() unos segundos después basta aquí, documentado en el
  // report de esta tarea).
  let resincronizandoId = $state<string | null>(null)
  const RESYNC_ESPERA_MS = 3000

  // Confirmación en dos pasos de "Quitar" (brief): un solo id a la vez, nunca
  // dos filas confirmando simultáneamente — pulsar Quitar en otra fila
  // cancela implícitamente cualquier confirmación pendiente.
  let confirmandoId = $state<string | null>(null)

  // F6 (fix final-review): quitar() tenía try/finally SIN catch — un DELETE
  // fallido (o el refetch de fuentes() que le sigue) salía como un rechazo
  // sin manejar de un onclick async, sin ningún aviso visible, y
  // alFuentesCambiaron nunca llegaba a llamarse (la copia de `fuentes` de
  // App se quedaba desincronizada de la de esta vista). errorQuitar es el
  // único rastro visible de que la acción no se completó.
  let errorQuitar = $state('')

  // Tarea 11 (P0.7, pase de accesibilidad — fix del ledger): label→url→id.
  // Filas legacy de bases reales pueden tener label Y url vacíos (fuentes
  // creadas antes de que ambos campos fueran obligatorios); sin este tercer
  // escalón la fila entera se queda sin ningún texto identificador — ni en
  // pantalla (.label) ni en los aria-label de Re-sincronizar/Quitar, que
  // reutilizan esta misma función. `id` SIEMPRE existe (es la clave primaria
  // de la fuente), así que es el único fallback que no puede fallar.
  function etiqueta(f: Fuente): string {
    return f.label || f.url || f.id
  }

  function relativo(ultimoSync: number | null): string {
    if (ultimoSync === null) return t('fuentes.nuncaSincronizada')
    const segundos = Math.max(0, Math.round(Date.now() / 1000) - ultimoSync)
    if (segundos < 60) return t('fuentes.relativo.ahora')
    const minutos = Math.round(segundos / 60)
    if (minutos < 60) return t('fuentes.relativo.minutos', { n: minutos })
    const horas = Math.round(minutos / 60)
    if (horas < 24) return t('fuentes.relativo.horas', { n: horas })
    const dias = Math.round(horas / 24)
    return t('fuentes.relativo.dias', { n: dias })
  }

  async function cargar() {
    estado = { tipo: 'cargando' }
    try {
      fuentes = await fuente.fuentes()
      estado = { tipo: 'listo' }
    } catch {
      estado = { tipo: 'error' }
    }
  }

  // Tarea 11 (P0.7, pase de accesibilidad): esta vista se abre/cierra
  // reemplazando lo que hay dentro de <main> (ver App.svelte, `vistaFuentes`)
  // — la cabecera con el botón «Fuentes» que la abre NUNCA se desmonta, así
  // que sin esto el foco se quedaba quieto ahí: quien navega con lector de
  // pantalla no recibía ningún indicio de que el contenido había cambiado
  // por debajo. Al montar, se mueve el foco al título de esta vista
  // (tabindex="-1" en el h2 de más abajo: programáticamente focable, nunca
  // parada de Tab) — mismo principio que el diálogo modal de
  // Reproductor.svelte al abrirse, pero sin su cepo de foco: esto no es un
  // diálogo, `<main>` no queda inert mientras esta vista está montada.
  // Al desmontar (Volver, o quitar la última fuente y que el Onboarding la
  // reemplace) se devuelve el foco a quien la abrió — sin esto, pulsar
  // «Volver» destruye el propio botón que tenía el foco (este componente se
  // desmonta con él) y el foco cae a <body>, dejando a quien navega con
  // teclado sin ningún indicador visible de dónde está. No hace falta el
  // rAF que sí usa Reproductor (ahí compite con el `inert` de <main>
  // limpiándose en el mismo flush; aquí nada vuelve inert al cerrar esta
  // vista).
  let elementoPrevio: HTMLElement | null = null
  let tituloEl = $state<HTMLHeadingElement | undefined>(undefined)

  // F6 (fix final-review): handle del setTimeout de resync() — sin
  // trackearlo, cerrar esta vista (o quitar App entero) a mitad de la
  // espera de RESYNC_ESPERA_MS dejaba el timer suelto, escribiendo sobre
  // `fuentes`/`resincronizandoId` de un componente ya desmontado. Mismo
  // principio de "cancelar al desmontar" que App.svelte aplica a su propio
  // sondeo (ver detenerSondeoFuente/onDestroy allí).
  let temporizadorResync: ReturnType<typeof setTimeout> | undefined

  onMount(() => {
    elementoPrevio = document.activeElement instanceof HTMLElement ? document.activeElement : null
    cargar()
    tick().then(() => tituloEl?.focus())
  })

  onDestroy(() => {
    if (temporizadorResync !== undefined) clearTimeout(temporizadorResync)
    if (elementoPrevio && document.body.contains(elementoPrevio)) elementoPrevio.focus()
  })

  async function resync(id: string) {
    if (resincronizandoId) return
    resincronizandoId = id
    try {
      await fuente.resyncFuente(id)
    } catch {
      // Fallo silencioso a propósito (brief: indicación breve, no un sondeo
      // completo con reintentar) — el refetch de más abajo, programado pase
      // lo que pase, deja la fila con los datos reales que tenga el backend.
    }
    temporizadorResync = setTimeout(async () => {
      temporizadorResync = undefined
      try {
        fuentes = await fuente.fuentes()
      } catch {
        // Un refetch fallido deja la lista tal cual estaba — no es motivo
        // para tumbar la vista.
      } finally {
        if (resincronizandoId === id) resincronizandoId = null
      }
    }, RESYNC_ESPERA_MS)
  }

  function pedirConfirmacion(id: string) {
    confirmandoId = id
  }

  function cancelarConfirmacion() {
    confirmandoId = null
  }

  async function quitar(id: string) {
    errorQuitar = ''
    try {
      await fuente.quitarFuente(id)
    } catch {
      errorQuitar = t('fuentes.quitar.error')
      return
    } finally {
      confirmandoId = null
    }
    // Refetch real (no un filter local): la fuente de verdad tras un quitar
    // es el backend, y App necesita la lista fresca para su propia copia de
    // `fuentes` (gobierna sinFuentes) y para refrescar el catálogo.
    try {
      const fuentesFrescas = await fuente.fuentes()
      fuentes = fuentesFrescas
      alFuentesCambiaron(fuentesFrescas)
    } catch {
      errorQuitar = t('fuentes.error')
    }
  }
</script>

<section class="fuentes" aria-labelledby="fuentes-titulo">
  <button type="button" class="volver" onclick={alVolver}>{t('fuentes.volver')}</button>
  <h2 id="fuentes-titulo" bind:this={tituloEl} tabindex="-1">{t('fuentes.titulo')}</h2>

  <!-- F6 (fix final-review): único rastro visible de un "Quitar" fallido —
       fuera del {#if estado.tipo} de abajo a propósito, para que se vea sin
       importar en qué rama de esa lista esté la fila afectada. -->
  {#if errorQuitar}
    <p class="mensaje error">{errorQuitar}</p>
  {/if}

  {#if estado.tipo === 'cargando'}
    <p class="mensaje">{t('fuentes.cargando')}</p>
  {:else if estado.tipo === 'error'}
    <p class="mensaje error">{t('fuentes.error')}</p>
  {:else if fuentes.length === 0}
    <p class="mensaje">{t('fuentes.vacia')}</p>
  {:else}
    <ul class="lista" aria-label={t('fuentes.etiquetaLista')}>
      {#each fuentes as f (f.id)}
        <li class="fila" aria-label={etiqueta(f)}>
          <div class="info">
            <span class="label">{etiqueta(f)}</span>
            <span class="url">{f.url}</span>
            <div class="meta">
              <span class="chip">{f.kind === 'file' ? t('fuentes.kind.file') : t('fuentes.kind.url')}</span>
              <span class="dato">{relativo(f.ultimoSync)}</span>
              <span class="dato">{t('fuentes.canales', { n: f.canales })}</span>
            </div>
          </div>

          <div class="acciones">
            <button
              type="button"
              aria-label={t('fuentes.resincronizar.etiqueta', { label: etiqueta(f) })}
              disabled={resincronizandoId === f.id}
              onclick={() => resync(f.id)}
            >
              {resincronizandoId === f.id ? t('fuentes.resincronizando') : t('fuentes.resincronizar')}
            </button>

            {#if confirmandoId === f.id}
              <button
                type="button"
                class="peligro"
                aria-label={t('fuentes.quitar.confirmar.etiqueta', { label: etiqueta(f) })}
                onclick={() => quitar(f.id)}
              >
                {t('fuentes.quitar.confirmar')}
              </button>
              <button type="button" onclick={cancelarConfirmacion}>
                {t('fuentes.quitar.cancelar')}
              </button>
            {:else}
              <button
                type="button"
                aria-label={t('fuentes.quitar.etiqueta', { label: etiqueta(f) })}
                onclick={() => pedirConfirmacion(f.id)}
              >
                {t('fuentes.quitar')}
              </button>
            {/if}
          </div>
        </li>
      {/each}
    </ul>
  {/if}

  <div class="anadir-mas">
    <h3 class="titulo-anadir">{t('fuentes.anadirMas.titulo')}</h3>
    <AnadirFuente {fuente} {alFuenteAnadida} />
  </div>

  <!-- Tarea 8 (P0.7): disclaimer legal completo. Sin aria-live (brief) — es
       texto estático, no un anuncio de estado. Bloque sobrio al pie, mismo
       patrón mono-eyebrow que .titulo-anadir/.titulo-sugeridas para el
       encabezado, --text-faint (no --text-muted) para marcar que es la
       letra pequeña, no contenido de la vista. -->
  <div class="legal">
    <h3 class="titulo-legal">{t('fuentes.legal.titulo')}</h3>
    <p>{t('fuentes.legal.reproductor')}</p>
    <p>{t('fuentes.legal.responsabilidad')}</p>
    <p>{t('fuentes.legal.sinDrm')}</p>
    <p>{t('fuentes.legal.sugeridas')}</p>
  </div>
</section>

<style>
  .fuentes {
    display: flex;
    flex-direction: column;
    gap: var(--space-4, 1rem);
    max-width: 44rem;
    margin: 0 auto;
  }
  .volver {
    all: unset;
    cursor: pointer;
    align-self: flex-start;
    color: var(--text-muted);
    font-size: 13px;
  }
  .volver:hover { color: var(--text-body); }
  h2 { margin: 0; color: var(--text-strong); }
  .mensaje { color: var(--text-muted); }
  .mensaje.error { color: var(--signal-error); }

  .lista {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .fila {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    background: var(--surface-card);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    padding: var(--space-3, 0.75rem) var(--space-4, 1rem);
  }
  .info {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .label {
    color: var(--text-strong);
    font: var(--type-label, inherit);
  }
  .url {
    color: var(--text-muted);
    font: var(--type-mono, inherit);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 28rem;
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  .chip {
    border: 1px solid var(--border-default);
    border-radius: 999px;
    padding: 1px 8px;
    color: var(--text-muted);
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    text-transform: uppercase;
    font-size: 11px;
  }
  .dato {
    color: var(--text-muted);
    font-size: 12px;
  }

  .acciones {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  .acciones button {
    background: var(--surface-raised);
    color: var(--text-body);
    border: 1px solid var(--border-default);
    border-radius: 6px;
    padding: 6px 10px;
    cursor: pointer;
    font: inherit;
  }
  .acciones button:disabled {
    opacity: 0.6;
    cursor: default;
  }
  /* Rojo = confirmación de una acción destructiva — el único sitio de esta
     vista donde el color no es ámbar (constraint global: ámbar solo para el
     acento/acción activa; el signal-error ya es el color de fallo en todo el
     resto de la app). */
  .acciones button.peligro {
    border-color: var(--signal-error);
    color: var(--signal-error);
    background: none;
  }

  .anadir-mas {
    border-top: 1px solid var(--border-default);
    padding-top: var(--space-4, 1rem);
    display: flex;
    flex-direction: column;
    gap: var(--space-3, 0.75rem);
  }
  .titulo-anadir {
    margin: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    text-transform: uppercase;
    color: var(--text-muted);
  }

  .legal {
    border-top: 1px solid var(--border-default);
    padding-top: var(--space-4, 1rem);
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .titulo-legal {
    margin: 0 0 4px;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    text-transform: uppercase;
    color: var(--text-faint);
  }
  .legal p {
    margin: 0;
    color: var(--text-faint);
    font-size: 12px;
    line-height: 1.5;
  }
</style>
