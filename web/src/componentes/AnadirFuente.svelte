<script lang="ts">
  import { onMount } from 'svelte'
  import type { CatalogSource, Fuente } from '../datos/catalogo'
  import { t } from '../i18n'

  // Tarea 7 (P0.7): bloque de "añadir fuente" extraído de Onboarding.svelte
  // (Tarea 6) — el DOM y las clases son IDÉNTICAS a las que Onboarding.test.ts
  // ya ejercita (mismos aria-label, mismo texto de botones), así que ese
  // fichero de test sigue pasando sin tocar sus selectores: Onboarding pasó a
  // ser un envoltorio delgado (título/copy/disclaimer) alrededor de este
  // componente. La vista Fuentes (gestión, este mismo task) reutiliza este
  // mismo componente debajo de su lista — DRY explícito del brief: mismo
  // formulario, mismo flujo de error, en los dos sitios donde se puede añadir
  // una fuente.
  //
  // enfocarAlMontar (default false): Onboarding SÍ quiere el foco automático
  // en el campo URL al montar (es el primer control con el que alguien sin
  // fuentes va a interactuar); la vista Fuentes NO — quien abre esa vista para
  // re-sincronizar o quitar una fuente existente no espera que el foco le
  // salte a un campo de texto sin pedirlo.
  let { fuente, alFuenteAnadida, enfocarAlMontar = false }: {
    fuente: CatalogSource
    alFuenteAnadida: (f: Fuente) => void
    enfocarAlMontar?: boolean
  } = $props()

  let url = $state('')
  let enviando = $state(false)
  let error = $state('')
  let sugeridas = $state<{ label: string; url: string }[]>([])
  let campoUrl = $state<HTMLInputElement | undefined>(undefined)

  onMount(() => {
    if (enfocarAlMontar) campoUrl?.focus()
    ;(async () => {
      try {
        sugeridas = await fuente.fuentesSugeridas()
      } catch {
        // Sin sugeridas no bloquea el resto del formulario: la URL propia y
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

<!-- Sin aria-live propio (mismo motivo que en Onboarding.svelte, ver su
     comentario histórico): la transición "fuente añadida → sincronizando" la
     cubre la región polite PERSISTENTE de App.svelte en los dos sitios donde
     este componente vive. El texto de error de aquí abajo es solo visual a
     propósito. -->
<div class="anadir-fuente">
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
</div>

<style>
  .anadir-fuente {
    display: flex;
    flex-direction: column;
    gap: var(--space-4, 1rem);
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
  /* F6 (fix final-review): .url:focus quita el outline nativo del input,
     pero sin esto el campo se quedaba SIN ningún foco visible — y es el
     primer control con el que se topa quien llega sin fuentes (foco
     automático, ver enfocarAlMontar/Onboarding.svelte:35). Mismo patrón que
     .buscar-grupo:focus en BarraLateralFacetas: ámbar en el borde del
     contenedor, no en el input suelto. */
  .campo-url:focus-within {
    outline: none;
    border-color: var(--tint-amber-line, var(--border-default));
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
  /* Ámbar = la CTA primaria de este bloque (constraint global: ámbar solo
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
</style>
