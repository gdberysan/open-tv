<script lang="ts">
  import { idioma, t } from '../i18n'
  import type { ClaveMensaje } from '../i18n/es'

  // Extraído de TarjetaCanal (Tarea 8, fix 1): la rejilla pintaba punto+ms
  // inline y la fila de la vista lista seguía con BarrasSenal (el medidor de
  // 3 barras que la spec retira explícitamente — «el medidor de 3 barras es
  // justo el widget que la marca evita»). Dos lenguajes de señal en la misma
  // app. Mismo patrón que LogoCanal.svelte: componente AGNÓSTICO DE LAYOUT
  // — no decide tamaño ni posición, el llamador lo envuelve como quiera
  // (badge sobre miniatura con scrim en la rejilla, inline sin scrim en la
  // fila de lista). Así rejilla y lista hablan el MISMO lenguaje de señal.
  //
  // imagenMs/sinImagen (Tarea 7, tiempo-hasta-la-imagen): opcionales — sin
  // ellos el componente se comporta EXACTAMENTE como antes (los llamadores
  // sin contexto `imagen`, incluidos los tests existentes de este mismo
  // fichero y de los tres llamadores, no pasan nada nuevo). Precedencia:
  // sinImagen pisa a imagenMs (un mirror sin imagen no tiene sentido
  // mostrarlo con un tiempo), e imagenMs>0 pisa a los ms de latencia de hoy.
  let {
    vivo,
    latenciaMs,
    imagenMs,
    sinImagen,
  }: { vivo: boolean | null; latenciaMs: number; imagenMs?: number; sinImagen?: boolean } = $props()

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

  // sinImagen tiene su PROPIA clase de punto (no reutiliza .muerta): un
  // mirror puede estar vivo (responde) y aun así no dar imagen — son dos
  // señales distintas, y confundirlas en el mismo color engañaría sobre
  // cuál es el problema real.
  const clasePunto = $derived(sinImagen ? 'sin-imagen' : estado)

  // Un decimal, coma en es / punto en en — mismo criterio de locale que
  // formatearHoraLocal (lib/hora.ts), pero aquí NO hay Intl que sirva:
  // toFixed(1) da el decimal y solo hace falta cambiar el separador.
  function formatearSegundos(ms: number): string {
    const texto = (ms / 1000).toFixed(1)
    return idioma.actual === 'es' ? texto.replace('.', ',') : texto
  }

  const segundos = $derived(imagenMs && imagenMs > 0 ? formatearSegundos(imagenMs) : null)

  const texto = $derived(
    sinImagen ? t('senal.sinImagen')
    : segundos !== null ? t('senal.imagenEn', { s: segundos })
    : latenciaMs > 0 ? `${latenciaMs} ms`
    : null,
  )

  const etiqueta = $derived(
    sinImagen ? t('senal.sinImagen')
    : segundos !== null ? `${t(clave)}, ${t('senal.imagenEn', { s: segundos }).toLowerCase()}`
    : t(clave),
  )

  // M6 (revisión de rama completa): en las ramas sinImagen/imagenMs el texto
  // VISIBLE ya dice el estado entero (etiqueta lo repite palabra por
  // palabra) — un lector de pantalla lo anunciaría dos veces. El punto pasa
  // a aria-hidden y sin aria-label ahí; en la rama por defecto (solo ms) el
  // texto NO dice el estado de salud, así que el punto sigue siendo el
  // único portador de esa información y conserva role="img"+aria-label.
  const dotAnunciaEstado = $derived(!sinImagen && segundos === null)
</script>

<span class="senal-canal">
  {#if dotAnunciaEstado}
    <i class="punto {clasePunto}" role="img" aria-label={etiqueta}></i>
  {:else}
    <i class="punto {clasePunto}" aria-hidden="true"></i>
  {/if}
  {#if texto}<span class="ms">{texto}</span>{/if}
</span>

<style>
  .senal-canal { display: inline-flex; align-items: center; gap: 4px; }
  .punto { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  /* vivo: pine, NUNCA ámbar — el ámbar queda reservado al favorito activo /
     al foco (mismo principio que IndicadorSenal.svelte en la cabecera). */
  .punto.vivo { background: var(--signal-ok); }
  .punto.muerta { background: var(--signal-error); }
  .punto.desconocido { background: var(--text-faint); }
  .punto.sin-imagen { background: var(--signal-error); }
  .ms {
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
  }
</style>
