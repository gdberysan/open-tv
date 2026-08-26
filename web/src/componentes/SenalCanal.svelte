<script lang="ts">
  import { t } from '../i18n'
  import type { ClaveMensaje } from '../i18n/es'

  // Extraído de TarjetaCanal (Tarea 8, fix 1): la rejilla pintaba punto+ms
  // inline y la fila de la vista lista seguía con BarrasSenal (el medidor de
  // 3 barras que la spec retira explícitamente — «el medidor de 3 barras es
  // justo el widget que la marca evita»). Dos lenguajes de señal en la misma
  // app. Mismo patrón que LogoCanal.svelte: componente AGNÓSTICO DE LAYOUT
  // — no decide tamaño ni posición, el llamador lo envuelve como quiera
  // (badge sobre miniatura con scrim en la rejilla, inline sin scrim en la
  // fila de lista). Así rejilla y lista hablan el MISMO lenguaje de señal.
  let { vivo, latenciaMs }: { vivo: boolean | null; latenciaMs: number } = $props()

  // Mapa único vivo→{clase,clave}: la clase gobierna el color del punto vía
  // CSS y la clave el aria-label — el color NUNCA es la única señal (mismo
  // principio que IndicadorSenal.svelte en la cabecera). Reutiliza las
  // MISMAS claves i18n que ya servían a BarrasSenal (senal.viva/muerta/
  // sinDatos): mismo estado, mismo texto en cualquier vista que lo use.
  type EstadoSalud = 'vivo' | 'muerta' | 'desconocido'
  const MAPA_SALUD: Record<EstadoSalud, ClaveMensaje> = {
    vivo: 'senal.viva',
    muerta: 'senal.muerta',
    desconocido: 'senal.sinDatos',
  }
  const estado = $derived<EstadoSalud>(vivo === true ? 'vivo' : vivo === false ? 'muerta' : 'desconocido')
  const clave = $derived(MAPA_SALUD[estado])
</script>

<span class="senal-canal">
  <i class="punto {estado}" role="img" aria-label={t(clave)}></i>
  {#if latenciaMs > 0}<span class="ms">{latenciaMs} ms</span>{/if}
</span>

<style>
  .senal-canal { display: inline-flex; align-items: center; gap: 4px; }
  .punto { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  /* vivo: pine, NUNCA ámbar — el ámbar queda reservado al favorito activo /
     al foco (mismo principio que IndicadorSenal.svelte en la cabecera). */
  .punto.vivo { background: var(--signal-ok); }
  .punto.muerta { background: var(--signal-error); }
  .punto.desconocido { background: var(--text-faint); }
  .ms {
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
  }
</style>
