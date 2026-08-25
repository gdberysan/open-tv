<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { t } from '../i18n'
  import { consultarSalud } from '../estado/salud'

  // alListo se llama en cuanto consultarSalud() deja de reportar
  // sincronizando: quien monta este componente decide qué pintar después
  // (normalmente, dejar de montarlo).
  let { alListo }: { alListo: () => void } = $props()

  let temporizador: ReturnType<typeof setInterval> | undefined

  async function comprobar() {
    try {
      const salud = await consultarSalud()
      if (!salud.sincronizando) alListo()
    } catch {
      // El gateway puede tardar en levantar el puerto justo al arrancar; un
      // fallo de red aquí no es un error que mostrar, es la misma espera.
    }
  }

  onMount(() => {
    comprobar()
    temporizador = setInterval(comprobar, 2000)
  })

  onDestroy(() => {
    if (temporizador) clearInterval(temporizador)
  })
</script>

<!-- Deliberadamente sin barra de progreso: el gateway no publica ningún
     porcentaje de avance y fingir uno sería mentir sobre lo que se sabe. -->
<div class="sincronizando" role="status">
  <h2>{t('estado.sincronizando')}</h2>
  <p>{t('estado.sincronizandoDetalle')}</p>
</div>

<style>
  .sincronizando {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    text-align: center;
    padding: var(--space-6, 3rem) var(--space-4, 1rem);
    color: var(--text-body);
  }
  h2 { margin: 0; color: var(--text-strong); }
  p { margin: 0; color: var(--text-muted); max-width: 32rem; }
</style>
