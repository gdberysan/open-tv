<script lang="ts">
  import { onMount } from 'svelte'
  import type { CatalogSource, Fuente } from '../datos/catalogo'
  import { t } from '../i18n'

  // Tarea 6 (P0.7): primer arranque sin fuentes. App decide CUÁNDO mostrar
  // este componente (derivado `sinFuentes`, ver App.svelte); este componente
  // solo sabe pedirle a `fuente` que añada una fuente y avisar hacia arriba
  // con la Fuente ya creada — el mismo patrón que el resto del cliente: un
  // único punto habla con CatalogSource desde fuera (Reproductor es la otra
  // excepción documentada), aquí se reutiliza el que ya recibe App.
  let { fuente, alFuenteAnadida }: {
    fuente: CatalogSource
    alFuenteAnadida: (f: Fuente) => void
  } = $props()

  let url = $state('')
  let enviando = $state(false)
  let error = $state('')
  let sugeridas = $state<{ label: string; url: string }[]>([])
  let campoUrl = $state<HTMLInputElement | undefined>(undefined)

  onMount(() => {
    // Foco al input al montar (brief): es el primer control con el que
    // alguien que llega sin fuentes va a interactuar.
    campoUrl?.focus()
    ;(async () => {
      try {
        sugeridas = await fuente.fuentesSugeridas()
      } catch {
        // Sin sugeridas no bloquea el resto del onboarding: la URL propia y
        // el fichero siguen disponibles.
      }
    })()
  })

  async function anadirPorUrl() {
    const valor = url.trim()
    if (!valor || enviando) return
    enviando = true
    error = ''
    try {
      const f = await fuente.anadirFuente(valor)
      url = ''
      alFuenteAnadida(f)
    } catch {
      error = t('onboarding.error')
    } finally {
      enviando = false
    }
  }

  function alTeclaUrl(e: KeyboardEvent) {
    if (e.key === 'Enter') anadirPorUrl()
  }

  async function alElegirFichero(e: Event) {
    const input = e.target as HTMLInputElement
    const f = input.files?.[0]
    // Se limpia siempre, aunque falle: permite volver a elegir el MISMO
    // fichero tras un error sin que el navegador lo trate como "sin cambios".
    input.value = ''
    if (!f || enviando) return
    enviando = true
    error = ''
    try {
      const fuenteCreada = await fuente.anadirFuenteFichero(f)
      alFuenteAnadida(fuenteCreada)
    } catch {
      error = t('onboarding.error')
    } finally {
      enviando = false
    }
  }

  async function anadirSugerida(s: { label: string; url: string }) {
    if (enviando) return
    enviando = true
    error = ''
    try {
      const f = await fuente.anadirFuente(s.url, s.label)
      alFuenteAnadida(f)
    } catch {
      error = t('onboarding.error')
    } finally {
      enviando = false
    }
  }
</script>

<!-- Sin aria-live propio (brief, Tarea 6): la transición "fuente añadida →
     sincronizando" la cubre la región polite PERSISTENTE que ya vive en
     App.svelte (su `mensajePoliteAccesible` ya incluye fase.tipo ===
     'sincronizando', que es justo lo que App fuerza en cuanto esta fuente
     llama a `alFuenteAnadida` — ver el comentario de esa función en
     App.svelte). El texto de error de aquí abajo es solo visual a
     propósito: una región nueva duplicaría el anuncio, el mismo bug que el
     fix round 2 evitó en Sincronizando/MensajeError. -->
<section class="onboarding" aria-labelledby="onboarding-titulo">
  <h2 id="onboarding-titulo">{t('onboarding.titulo')}</h2>
  <p class="copy">{t('onboarding.copy')}</p>

  <div class="fila-url">
    <!-- Mismo motivo ámbar "<" del buscador de BarraLateralFacetas: decorativo
         (aria-hidden), evoca la misma sala de control. -->
    <div class="campo-url">
      <span class="motivo" aria-hidden="true">&lt;</span>
      <input
        type="url"
        class="url"
        bind:value={url}
        bind:this={campoUrl}
        placeholder={t('onboarding.url.placeholder')}
        aria-label={t('onboarding.url.etiqueta')}
        disabled={enviando}
        onkeydown={alTeclaUrl}
      />
    </div>
    <button
      type="button"
      class="anadir"
      onclick={anadirPorUrl}
      disabled={enviando || !url.trim()}
    >
      {enviando ? t('onboarding.anadiendo') : t('onboarding.anadir')}
    </button>
  </div>

  <label class="fichero">
    <span>{t('onboarding.fichero.etiqueta')}</span>
    <input type="file" accept=".m3u,.m3u8" disabled={enviando} onchange={alElegirFichero} />
  </label>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  {#if sugeridas.length}
    <div class="sugeridas">
      <h3 class="titulo-sugeridas">{t('onboarding.sugeridas.titulo')}</h3>
      <div class="lista-sugeridas">
        {#each sugeridas as s (s.url)}
          <button
            type="button"
            class="sugerida"
            aria-label={t('onboarding.sugeridas.anadir', { label: s.label })}
            disabled={enviando}
            onclick={() => anadirSugerida(s)}
          >
            <span aria-hidden="true">+</span>
            {s.label}
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <p class="disclaimer">{t('onboarding.disclaimer')}</p>
</section>

<style>
  .onboarding {
    display: flex;
    flex-direction: column;
    gap: var(--space-4, 1rem);
    max-width: 34rem;
    margin: var(--space-6, 2rem) auto;
    text-align: center;
  }
  h2 {
    margin: 0;
    color: var(--text-strong);
    font: var(--type-h4, inherit);
  }
  .copy {
    margin: 0;
    color: var(--text-muted);
  }

  .fila-url {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  .campo-url {
    flex: 1;
    min-width: 12rem;
    display: flex;
    align-items: center;
    gap: 6px;
    background: var(--surface-sunken);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    padding: 8px 12px;
    text-align: left;
  }
  .motivo {
    color: var(--amber-500);
    font: var(--type-mono, inherit);
  }
  .url {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-body);
    font: var(--type-body, inherit);
    min-width: 0;
  }
  .url:focus {
    outline: none;
  }
  /* Ámbar = la CTA primaria de este estado (constraint global: ámbar solo
     para el acento/acción activa). */
  .anadir {
    background: var(--tint-amber-weak);
    border: 1px solid var(--tint-amber-line);
    border-radius: var(--radius-md, 8px);
    padding: 8px 16px;
    color: var(--amber-500);
    cursor: pointer;
    font: inherit;
  }
  .anadir:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .fichero {
    display: flex;
    flex-direction: column;
    gap: 4px;
    align-items: center;
    color: var(--text-muted);
    font: var(--type-mono-label, inherit);
  }

  .error {
    margin: 0;
    color: var(--signal-error);
  }

  .sugeridas {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .titulo-sugeridas {
    margin: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    text-transform: uppercase;
    color: var(--text-muted);
  }
  .lista-sugeridas {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }
  .sugerida {
    background: none;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    padding: 6px 12px;
    color: var(--text-body);
    cursor: pointer;
    font: var(--type-body-sm, inherit);
  }
  .sugerida:hover {
    background: var(--surface-raised);
  }
  .sugerida:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .disclaimer {
    margin: 0;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
