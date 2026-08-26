<script lang="ts">
  // Extraído de TarjetaCanal.svelte (fix final, hallazgo 1): la rejilla ya
  // tenía el fallback img-o-iniciales desde la Tarea 7, pero la vista lista
  // (RejillaCanales.svelte) se quedó con un <img> suelto sin onerror — una
  // logoUrl muerta (frecuente en el catálogo) mostraba el icono de imagen
  // rota del navegador en vez de las iniciales. Componente compartido para
  // que ambas vistas usen la MISMA lógica en vez de duplicarla (y del
  // bug de la lista quedarse desincronizado otra vez si algo cambia aquí).
  //
  // Deliberadamente NO decide tamaño ni redondeo: el llamador envuelve
  // <LogoCanal> en su propio contenedor dimensionado (aspect-ratio 16/9 y
  // ancho completo en la rejilla; 32×32 en la lista) — este componente solo
  // rellena ese contenedor al 100% con object-fit:contain. Así la rejilla y
  // la lista pueden mantener layouts distintos (incluido el borde
  // redondeado del icono de iniciales en la lista, vía overflow:hidden +
  // border-radius del contenedor del llamador) sin que este componente
  // necesite saber nada de eso.
  let { logoUrl, nombre }: { logoUrl: string; nombre: string } = $props()

  // La logoUrl del catálogo suele estar muerta/404/bloqueada; si la imagen no
  // carga, caemos a las iniciales en vez de dejar el icono de imagen rota.
  let logoRoto = $state(false)
  // En virtualización (Tarea 17), la instancia se reutiliza con otro canal.
  // Resetear logoRoto al cambiar para que el logo nuevo intente cargar.
  $effect(() => {
    void logoUrl
    void nombre
    logoRoto = false
  })
</script>

{#if logoUrl && !logoRoto}
  <img src={logoUrl} alt="" loading="lazy" onerror={() => (logoRoto = true)} />
{:else}
  <span class="sinlogo" aria-hidden="true">{nombre.slice(0, 2)}</span>
{/if}

<style>
  img, .sinlogo { display: block; width: 100%; height: 100%; object-fit: contain; }
  .sinlogo { display: grid; place-items: center; background: var(--surface-sunken); color: var(--text-muted, var(--graphite-300)); }
</style>
