<script lang="ts">
  import { onMount } from 'svelte'
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

  function etiqueta(f: Fuente): string {
    return f.label || f.url
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

  onMount(cargar)

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
    setTimeout(async () => {
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
    try {
      await fuente.quitarFuente(id)
    } finally {
      confirmandoId = null
    }
    // Refetch real (no un filter local): la fuente de verdad tras un quitar
    // es el backend, y App necesita la lista fresca para su propia copia de
    // `fuentes` (gobierna sinFuentes) y para refrescar el catálogo.
    const fuentesFrescas = await fuente.fuentes()
    fuentes = fuentesFrescas
    alFuentesCambiaron(fuentesFrescas)
  }
</script>

<section class="fuentes" aria-labelledby="fuentes-titulo">
  <button type="button" class="volver" onclick={alVolver}>{t('fuentes.volver')}</button>
  <h2 id="fuentes-titulo">{t('fuentes.titulo')}</h2>

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
</style>
