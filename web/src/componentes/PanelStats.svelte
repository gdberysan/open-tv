<script lang="ts">
  import { onMount } from 'svelte'
  import { t } from '../i18n'

  // Vista de depuración local para GET /stats (Tarea 13): pinta la tasa de
  // reproducción real y el agregado de catálogo. No es producto, es la
  // primera medición viva de "¿de verdad reproducen los canales?" — se
  // mantiene mínima a propósito, sin ámbar (nada aquí es una señal viva de
  // canal en directo).
  let { alVolver }: { alVolver: () => void } = $props()

  interface Reproduccion {
    intentos: number
    iniciados: number
    fallos: number
    cortados: number
    tasa_exito: number
    por_motivo: Record<string, number>
    por_via: Record<string, number>
    por_motor: Record<string, number>
  }
  interface Catalogo {
    streams_totales: number
    vivos: number
    muertos: number
    web_ok: number
    web_no: number
    web_desconocido: number
  }

  type Estado =
    | { tipo: 'cargando' }
    | { tipo: 'error' }
    | { tipo: 'listo'; reproduccion: Reproduccion; catalogo: Catalogo | null }

  let estado = $state<Estado>({ tipo: 'cargando' })

  function entradas(m: Record<string, number> | undefined): [string, number][] {
    return Object.entries(m ?? {}).sort((a, b) => b[1] - a[1])
  }

  onMount(async () => {
    try {
      const resp = await fetch('/stats')
      if (!resp.ok) throw new Error(`stats: ${resp.status}`)
      const cuerpo = (await resp.json()) as { reproduccion: Reproduccion; catalogo?: Catalogo }
      estado = { tipo: 'listo', reproduccion: cuerpo.reproduccion, catalogo: cuerpo.catalogo ?? null }
    } catch {
      estado = { tipo: 'error' }
    }
  })
</script>

<section class="panel-stats">
  <button type="button" class="volver" onclick={alVolver}>{t('stats.volver')}</button>
  <h2>{t('stats.titulo')}</h2>

  {#if estado.tipo === 'cargando'}
    <p class="mensaje">{t('stats.cargando')}</p>
  {:else if estado.tipo === 'error'}
    <p class="mensaje error">{t('stats.error')}</p>
  {:else}
    <div class="bloque">
      <h3>{t('stats.reproduccion')}</h3>
      {#if estado.reproduccion.intentos === 0}
        <p class="mensaje">{t('stats.sinDatos')}</p>
      {:else}
        <div class="cifras">
          <span><strong>{estado.reproduccion.intentos}</strong> {t('stats.intentos')}</span>
          <span><strong>{estado.reproduccion.iniciados}</strong> {t('stats.iniciados')}</span>
          <span><strong>{estado.reproduccion.fallos}</strong> {t('stats.fallos')}</span>
          <span><strong>{estado.reproduccion.cortados}</strong> {t('stats.cortados')}</span>
          <span><strong>{Math.round(estado.reproduccion.tasa_exito * 100)}%</strong> {t('stats.tasaExito')}</span>
        </div>

        <div class="columnas">
          <div>
            <h4>{t('stats.porMotivo')}</h4>
            <ul>
              {#each entradas(estado.reproduccion.por_motivo) as [motivo, n] (motivo)}
                <li><span class="clave">{motivo}</span><span class="valor">{n}</span></li>
              {/each}
            </ul>
          </div>
          <div>
            <h4>{t('stats.porVia')}</h4>
            <ul>
              {#each entradas(estado.reproduccion.por_via) as [via, n] (via)}
                <li><span class="clave">{via}</span><span class="valor">{n}</span></li>
              {/each}
            </ul>
          </div>
          <div>
            <h4>{t('stats.porMotor')}</h4>
            <ul>
              {#each entradas(estado.reproduccion.por_motor) as [motor, n] (motor)}
                <li><span class="clave">{motor}</span><span class="valor">{n}</span></li>
              {/each}
            </ul>
          </div>
        </div>
      {/if}
    </div>

    {#if estado.catalogo}
      <div class="bloque">
        <h3>{t('stats.catalogo')}</h3>
        <div class="cifras">
          <span><strong>{estado.catalogo.streams_totales}</strong> {t('stats.streamsTotales')}</span>
          <span><strong>{estado.catalogo.vivos}</strong> {t('stats.vivos')}</span>
          <span><strong>{estado.catalogo.muertos}</strong> {t('stats.muertos')}</span>
          <span><strong>{estado.catalogo.web_ok}</strong> {t('stats.webOk')}</span>
          <span><strong>{estado.catalogo.web_no}</strong> {t('stats.webNo')}</span>
          <span><strong>{estado.catalogo.web_desconocido}</strong> {t('stats.webDesconocido')}</span>
        </div>
      </div>
    {/if}
  {/if}
</section>

<style>
  .panel-stats {
    display: flex;
    flex-direction: column;
    gap: var(--space-4, 1rem);
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
  h3 { margin: 0 0 var(--space-2, 0.5rem); color: var(--text-body); font-size: 15px; }
  h4 { margin: 0 0 4px; color: var(--text-muted); font-size: 12px; font-weight: 600; }
  .mensaje { color: var(--text-muted); }
  .mensaje.error { color: var(--signal-error); }
  .bloque {
    background: var(--surface-card);
    border-radius: var(--radius-md);
    padding: var(--space-4, 1rem);
  }
  .cifras {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4, 1rem);
    color: var(--text-body);
    font-size: 13px;
    margin-bottom: var(--space-3, 0.75rem);
  }
  .cifras strong { color: var(--text-strong); font-size: 16px; margin-right: 4px; }
  .columnas {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
    gap: var(--space-4, 1rem);
  }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  li { display: flex; justify-content: space-between; gap: 8px; font-size: 13px; color: var(--text-body); }
  .clave { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .valor { color: var(--text-muted); }
</style>
