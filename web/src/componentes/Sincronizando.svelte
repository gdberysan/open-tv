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

<!-- SIN role="status"/aria-live (fix round 2, Tarea 18): este componente
     solo se monta desde App.svelte, que YA tiene su propia región
     aria-live="polite" PERSISTENTE para este mismo texto (ver App.svelte).
     Si este <div> tuviera también semántica live, un lector de pantalla
     anunciaría "Sincronizando el catálogo…" DOS VECES seguidas al mismo
     tiempo — la regresión que el fix round 1 evitó en Reproductor.svelte
     pero no aquí. El texto sigue siendo visible para quien ve la pantalla;
     solo se retira quién lo anuncia.

     Barra INDETERMINADA (Tarea 12, P0.6): el gateway no publica ningún
     porcentaje de avance real, así que un aria-valuenow fijo (o animado)
     estaría mintiendo sobre lo que se sabe — de ahí role="progressbar" SIN
     aria-valuenow (el patrón WAI-ARIA correcto para "en curso, sin cifra").
     El barrido es puramente decorativo (aria-hidden) y se desactiva bajo
     prefers-reduced-motion, dejando una barra estática. -->
<div class="sincronizando">
  <h2>{t('estado.sincronizando')}</h2>
  <p>{t('estado.sincronizandoDetalle')}</p>
  <div class="barra" role="progressbar" aria-label={t('estado.sincronizando')}>
    <div class="barra-relleno" aria-hidden="true"></div>
  </div>
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

  /* Pista: graphite. Barrido: ámbar (marca actividad en vivo, uso permitido
     del acento). Sin porcentaje real → indeterminada: el relleno es más
     angosto que la pista y recorre su ancho en bucle, nunca "llega" a un
     valor final. */
  .barra {
    width: 100%;
    max-width: 16rem;
    height: 6px;
    margin-top: var(--space-2, 0.5rem);
    border-radius: 999px;
    background: var(--graphite-600);
    overflow: hidden;
  }
  .barra-relleno {
    width: 40%;
    height: 100%;
    border-radius: inherit;
    background: var(--amber-500);
    animation: barrido 1.4s var(--ease-in-out) infinite;
  }
  @keyframes barrido {
    0%   { transform: translateX(-100%); }
    100% { transform: translateX(350%); }
  }
  /* Sin animación para quien la pidió desactivada: barra estática visible
     (sigue comunicando "hay una barra, hay actividad"), sin barrido. */
  @media (prefers-reduced-motion: reduce) {
    .barra-relleno {
      width: 100%;
      animation: none;
      transform: none;
    }
  }
</style>
