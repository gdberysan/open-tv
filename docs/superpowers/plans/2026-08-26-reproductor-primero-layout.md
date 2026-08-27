# Reproductor primero — layout de reproducción persistente · Plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Invertir el modelo del cliente: de catálogo-primero con reproductor modal a **reproductor persistente** (vídeo a la izquierda, catálogo lateral a la derecha, cambio de canal en el sitio), conservando la rejilla rica de P0.6 como modo «ver todo».

**Architecture:** Puro cliente (`web/`): tres componentes nuevos (lista lateral virtualizada, facetas compactas, composición del catálogo lateral), refactor del `Reproductor` de modal a panel (el motor de failover/guard/EPG/PiP se conserva intacto), y reestructuración de `App.svelte` en escenario + modo ver-todo. El backend NO cambia.

**Tech Stack:** Svelte 5 (runas), TypeScript, Vitest + @testing-library/svelte, Playwright. Sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-08-26-reproductor-primero-layout-design.md` — el plan argumenta desde ella; el ejecutor lee ambos.

## Global Constraints

(Del §9 de la spec + CLAUDE.md; TODA tarea las hereda.)

- **`mobile/` CERO diffs** (`git status --porcelain mobile/` vacío al final de cada tarea). Contrato JSON `/channels` de 15 claves intacto — esto es puro cliente, el backend no se toca.
- **Ninguna dependencia Go/JS nueva.** **Bundle propio ≤ 80 KB gzip** (hls.js sigue en su chunk perezoso aparte).
- **Tokens de marca:** ámbar = señal-viva/activo/foco. Nada hardcodeado. Ancho del catálogo lateral = token `--ancho-catalogo: 372px` (nunca un 372 mágico repetido).
- **a11y:** 2 regiones sr-only PERSISTENTES de App como ÚNICOS anunciadores nuevos (NO añadir regiones aria-live; las 2 internas del Reproductor ya existen y se conservan); roving tabindex en la lista lateral; los overlays reales (⌘K) conservan trap/restore + fondo `inert`; **`prefers-reduced-motion` anula toda animación nueva**. Gate VoiceOver manual del autor OBLIGATORIO al final (no automatizable — se deja constancia, no se marca como hecho).
- **i18n:** ES fuente de verdad, EN en paridad (el test `i18n.test.ts` lo asevera) — cada tarea añade SUS claves en `es.ts` Y `en.ts`.
- **Gates al final de CADA tarea:**
  - Go (aunque no se toque): `gofmt -l .` (vacío) · `go vet ./...` · `go build ./...` · `go test -race -count=1 ./...` · `golangci-lint run ./...` · `go run ./tools/scrubcheck` (exit 0). Todo desde la RAÍZ del repo (ojo: `cd web` persiste entre comandos del shell).
  - Web: `cd web && npm run check && npm test && npm run build` (y restaurar `internal/ui/dist/.gitkeep` si el build lo borró).
  - Mobile: `cd mobile && flutter analyze && flutter test` + cero diffs.
- **Identidad de commits:** verificar `git config user.email` == `gdberysan@gmail.com`; trailer `Co-Authored-By:` con el modelo en uso. **NUNCA `git add -A`** — añadir por ruta explícita. Nada a `main` ni push sin autorización del usuario.
- **Rama:** todo el trabajo en `feat/reproductor-primero` (creada en la Tarea 1; aislamiento vía superpowers:using-git-worktrees en tiempo de ejecución).

## Decisiones de diseño de este plan (cerradas aquí, derivadas de la spec)

1. **Modo «ver todo» = REEMPLAZO visual, pero el Reproductor NO se desmonta.** El contenedor del escenario queda `display:none` (clase `.oculto`) mientras ver-todo o una vista-hash (`#ajustes`/`#fuentes`/`#stats`) están encima — el `<video>` sigue montado y el audio continúa (metáfora de televisor encendido). `display:none` ya saca sus controles del orden de foco y del árbol de accesibilidad; no hace falta `inert` adicional. Esto es lo que hace verificable el requisito de la spec «el vídeo no se desmonta/remonta».
2. **Reparto de la barra espaciadora:** en el ESCENARIO, espacio = play/pausa del reproductor (con la guarda `esObjetivoInteractivo` — escribir un espacio en el buscador de la lateral debe escribir un espacio). El gesto «surf» queda SOLO en modo ver-todo (donde vive la rejilla y su pista visual). ⌘K deja de inhibirse «con el reproductor abierto» (§6 de la spec): solo la inhibe otro overlay (`paletaAbierta`).
3. **Responsive estrecho = apilado** (vídeo arriba 16:9, catálogo debajo), la opción simple de §8 — sin cajón nuevo. El cajón de facetas de P0.6 sigue existiendo tal cual dentro del modo ver-todo.
4. **AirPlay** se muda de la barra inferior del modal (que desaparece con el botón Cerrar) a los controles del overlay, junto a PiP.
5. **Refactor en dos fases para no dejar nunca la rama en rojo:** la Tarea 4 da al `Reproductor` un prop `modo: 'modal' | 'panel'` (por defecto `'modal'`, App intacta, todos los tests existentes verdes); la Tarea 5 conmuta App al panel y BORRA el modo modal y sus tests. El andamiaje dual vive exactamente un commit-rango, a cambio de que cada tarea termine verde.
6. **Auto-reanudación** (entrar viendo): NO pasa por `abrirCanal` (no re-registra en el historial — ya es la entrada más reciente); sí anuncia por la región polite («honestidad» de §9).

---

### Task 1: `ListaCanalesLateral.svelte` — lista estrecha virtualizada con salud y roving tabindex

**Files:**
- Create: `web/src/componentes/ListaCanalesLateral.svelte`
- Create: `web/src/componentes/ListaCanalesLateral.test.ts`
- Modify: `web/src/i18n/es.ts`, `web/src/i18n/en.ts` (claves nuevas)

**Interfaces:**
- Consumes: `Canal` (`web/src/datos/catalogo.ts`), `SenalCanal.svelte` (`{vivo, latenciaMs}`), `parsearResolucion(nombre: string): string | null` (`web/src/lib/resolucion.ts`), `t` (`web/src/i18n`).
- Produces: componente con props `{ canales: Canal[]; canalActualId?: string | null; alAbrir: (c: Canal) => void; alPedirMas: () => void }`. La Tarea 3 lo monta con exactamente esa firma.

**Contexto para el implementador:** a diferencia de `RejillaVirtual.svelte` (que virtualiza contra el scroll de la PÁGINA y mide la altura de una tarjeta), esta lista vive dentro de un contenedor con **scroll propio** (la barra lateral tiene altura acotada) y sus filas tienen **altura constante** — la ventana se calcula con `scrollTop`/`clientHeight` del contenedor, sin medir nada. Lee el comentario largo de `RejillaVirtual.svelte` (roving tabindex, coalescencia por rAF, espaciadores): este componente replica esos patrones adaptados a scroll interno.

- [ ] **Step 1: Crear la rama de la feature**

```bash
git checkout -b feat/reproductor-primero
```

- [ ] **Step 2: Añadir las claves i18n (ES y EN)**

En `web/src/i18n/es.ts`:

```ts
'lateral.lista': 'Canales',
'escenario.reproduciendo': 'Reproduciendo',
```

En `web/src/i18n/en.ts`:

```ts
'lateral.lista': 'Channels',
'escenario.reproduciendo': 'Playing',
```

- [ ] **Step 3: Escribir los tests (fallan: el componente no existe)**

`web/src/componentes/ListaCanalesLateral.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { tick } from 'svelte'
import ListaCanalesLateral from './ListaCanalesLateral.svelte'
import type { Canal } from '../datos/catalogo'
import { t } from '../i18n'

// jsdom no implementa IntersectionObserver — mismo doble que
// App.integracion.test.ts, reducido a lo que este fichero usa.
class FalsoIntersectionObserver {
  static instancias: FalsoIntersectionObserver[] = []
  constructor(private cb: IntersectionObserverCallback) {
    FalsoIntersectionObserver.instancias.push(this)
  }
  observe() {}
  disconnect() {}
  unobserve() {}
  takeRecords() { return [] }
  dispararInterseccion() {
    this.cb([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalDePrueba(id: string, extra: Partial<Canal> = {}): Canal {
  return {
    id, nombre: `Canal ${id}`, logoUrl: '', categoriaId: '', idioma: 'es',
    pais: '', vivo: true, latenciaMs: 120, webOk: true, ...extra,
  }
}
const muchos = Array.from({ length: 1000 }, (_, i) => canalDePrueba(String(i)))

describe('ListaCanalesLateral — virtualización con scroll interno', () => {
  it('con 1000 canales solo monta una ventana acotada de filas, nunca las 1000', () => {
    render(ListaCanalesLateral, { canales: muchos, alAbrir: vi.fn(), alPedirMas: vi.fn() })
    const filas = document.querySelectorAll('.lista-lateral article')
    expect(filas.length).toBeGreaterThan(0)
    expect(filas.length).toBeLessThan(100)
  })

  it('la fila muestra la salud (SenalCanal) y la resolución sacada del nombre', () => {
    render(ListaCanalesLateral, {
      canales: [canalDePrueba('a', { nombre: 'Tele Uno 1080p', latenciaMs: 88 })],
      alAbrir: vi.fn(), alPedirMas: vi.fn(),
    })
    expect(screen.getByText('88 ms')).toBeTruthy()
    expect(screen.getByText('1080p')).toBeTruthy()
  })

  it('el canal en curso lleva aria-current y la insignia «Reproduciendo»', () => {
    render(ListaCanalesLateral, {
      canales: [canalDePrueba('a'), canalDePrueba('b')],
      canalActualId: 'b', alAbrir: vi.fn(), alPedirMas: vi.fn(),
    })
    const activo = screen.getByRole('button', { name: /Canal b/ })
    expect(activo.getAttribute('aria-current')).toBe('true')
    expect(screen.getByText(t('escenario.reproduciendo'))).toBeTruthy()
  })

  it('clicar una fila llama a alAbrir con el canal', async () => {
    const alAbrir = vi.fn()
    render(ListaCanalesLateral, { canales: [canalDePrueba('a')], alAbrir, alPedirMas: vi.fn() })
    await fireEvent.click(screen.getByRole('button', { name: /Canal a/ }))
    expect(alAbrir).toHaveBeenCalledWith(expect.objectContaining({ id: 'a' }))
  })

  it('roving tabindex: solo una fila con tabindex=0; las flechas mueven el índice activo', async () => {
    render(ListaCanalesLateral, { canales: muchos.slice(0, 5), alAbrir: vi.fn(), alPedirMas: vi.fn() })
    const botones = () => Array.from(document.querySelectorAll<HTMLButtonElement>('.lista-lateral button.abrir'))
    expect(botones().filter((b) => b.tabIndex === 0)).toHaveLength(1)
    const primero = botones()[0]
    primero.focus()
    await fireEvent.keyDown(primero, { key: 'ArrowDown' })
    await tick()
    expect(botones().filter((b) => b.tabIndex === 0)).toHaveLength(1)
    expect(botones()[1].tabIndex).toBe(0)
  })

  it('el centinela al final dispara alPedirMas al intersectar', async () => {
    const alPedirMas = vi.fn()
    render(ListaCanalesLateral, { canales: muchos.slice(0, 5), alAbrir: vi.fn(), alPedirMas })
    await tick()
    FalsoIntersectionObserver.instancias.at(-1)?.dispararInterseccion()
    expect(alPedirMas).toHaveBeenCalled()
  })
})
```

- [ ] **Step 4: Ejecutar y verificar que falla**

Run: `cd web && npx vitest run src/componentes/ListaCanalesLateral.test.ts`
Expected: FAIL — «Cannot find module './ListaCanalesLateral.svelte'».

- [ ] **Step 5: Implementar el componente**

`web/src/componentes/ListaCanalesLateral.svelte`:

```svelte
<script lang="ts">
  import { onDestroy, tick } from 'svelte'
  import type { Canal } from '../datos/catalogo'
  import SenalCanal from './SenalCanal.svelte'
  import { parsearResolucion } from '../lib/resolucion'
  import { t } from '../i18n'

  // Lista estrecha del escenario reproductor-primero (spec §3). Virtualiza
  // contra su PROPIO scroll (el contenedor tiene overflow-y), no contra el
  // viewport como RejillaVirtual: la lateral tiene altura acotada por CSS y
  // sus filas son de altura CONSTANTE, así que la ventana es aritmética pura
  // sin medir el DOM. El roving tabindex replica el patrón de RejillaVirtual
  // (índice global, no referencia a nodo — ver su comentario largo).
  let { canales, canalActualId = null, alAbrir, alPedirMas }: {
    canales: Canal[]
    canalActualId?: string | null
    alAbrir: (c: Canal) => void
    alPedirMas: () => void
  } = $props()

  // DEBE coincidir con el height de .fila del <style>: la virtualización
  // entera depende de que cada fila mida exactamente esto.
  const ALTO_FILA = 48
  const FILAS_BUFFER = 8

  let contenedor: HTMLDivElement | undefined = $state()
  let centinela: HTMLDivElement | undefined = $state()
  let scrollTop = $state(0)
  let altoVisor = $state(0)

  // Coalescencia por rAF, mismo motivo que RejillaVirtual (fix ronda 1 de
  // P0.6): decenas de eventos scroll por segundo, una lectura por frame.
  let raf = 0
  function recalcular() {
    raf = 0
    if (!contenedor) return
    scrollTop = contenedor.scrollTop
    altoVisor = contenedor.clientHeight
  }
  function agendar() {
    if (raf) return
    raf = requestAnimationFrame(recalcular)
  }
  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf)
  })

  let innerHeight = $state(0)
  $effect(() => {
    innerHeight
    canales.length
    agendar()
  })

  const filaInicio = $derived(Math.max(0, Math.floor(scrollTop / ALTO_FILA) - FILAS_BUFFER))
  const filaFin = $derived(Math.min(canales.length, Math.ceil((scrollTop + altoVisor) / ALTO_FILA) + FILAS_BUFFER))
  const visibles = $derived(canales.slice(filaInicio, filaFin))
  const altoArriba = $derived(filaInicio * ALTO_FILA)
  const altoAbajo = $derived(Math.max(0, (canales.length - filaFin) * ALTO_FILA))

  let activeIndex = $state(0)
  $effect(() => {
    if (canales.length === 0) activeIndex = 0
    else if (activeIndex >= canales.length) activeIndex = canales.length - 1
  })

  function enfocarIndice(deseado: number) {
    if (canales.length === 0) return
    const objetivo = Math.max(0, Math.min(canales.length - 1, deseado))
    activeIndex = objetivo
    if (contenedor && (objetivo < filaInicio || objetivo >= filaFin)) {
      contenedor.scrollTop = objetivo * ALTO_FILA
      recalcular()
    }
    tick().then(() => {
      contenedor?.querySelector<HTMLElement>(`[data-indice="${objetivo}"] .abrir`)?.focus()
    })
  }

  function alTeclado(e: KeyboardEvent) {
    if (!(e.target instanceof HTMLElement) || !e.target.classList.contains('abrir')) return
    switch (e.key) {
      case 'ArrowDown': e.preventDefault(); enfocarIndice(activeIndex + 1); break
      case 'ArrowUp': e.preventDefault(); enfocarIndice(activeIndex - 1); break
      case 'Home': e.preventDefault(); enfocarIndice(0); break
      case 'End': e.preventDefault(); enfocarIndice(canales.length - 1); break
    }
  }

  // Mismo centinela de paginación que RejillaVirtual, con root = el propio
  // contenedor de scroll (el default —viewport— no aplica a un scroll interno).
  $effect(() => {
    if (!centinela || !contenedor) return
    const observador = new IntersectionObserver(
      (entradas) => { if (entradas[0]?.isIntersecting) alPedirMas() },
      { root: contenedor },
    )
    observador.observe(centinela)
    return () => observador.disconnect()
  })
</script>

<svelte:window bind:innerHeight />

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<!-- keydown = delegación del roving tabindex de los <button> hijos, como en
     RejillaVirtual: el contenedor no es interactivo ni recibe foco. -->
<div
  class="lista-lateral"
  bind:this={contenedor}
  role="list"
  aria-label={t('lateral.lista')}
  onscroll={agendar}
  onkeydown={alTeclado}
>
  {#if altoArriba > 0}<div class="espaciador" style:height="{altoArriba}px" aria-hidden="true"></div>{/if}
  {#each visibles as canal, i (canal.id)}
    {@const indice = filaInicio + i}
    {@const resolucion = parsearResolucion(canal.nombre)}
    {@const enCurso = canal.id === canalActualId}
    <article class="fila" role="listitem" data-indice={indice}>
      <button
        type="button"
        class="abrir"
        class:en-curso={enCurso}
        tabindex={indice === activeIndex ? 0 : -1}
        aria-current={enCurso ? 'true' : undefined}
        onclick={() => alAbrir(canal)}
      >
        <SenalCanal vivo={canal.vivo} latenciaMs={canal.latenciaMs} />
        <span class="nombre">{canal.nombre}</span>
        {#if resolucion}<span class="resolucion">{resolucion}</span>{/if}
        {#if enCurso}<span class="insignia-reproduciendo">{t('escenario.reproduciendo')}</span>{/if}
      </button>
    </article>
  {/each}
  {#if altoAbajo > 0}<div class="espaciador" style:height="{altoAbajo}px" aria-hidden="true"></div>{/if}
  <div class="centinela" bind:this={centinela} aria-hidden="true"></div>
</div>

<style>
  .lista-lateral {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }
  /* height DEBE coincidir con ALTO_FILA del script. */
  .fila { height: 48px; flex-shrink: 0; box-sizing: border-box; }
  .fila .abrir {
    all: unset;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: var(--space-2, 8px);
    width: 100%;
    height: 100%;
    padding: 0 var(--space-2, 8px);
    border-radius: var(--radius-sm, 5px);
    cursor: pointer;
    color: var(--text-body);
  }
  .fila .abrir:hover { background: var(--surface-raised); }
  /* Ámbar = canal activo (señal-viva/activo, token de marca). */
  .fila .abrir.en-curso { background: var(--tint-amber-weak); }
  .fila .abrir.en-curso .nombre { color: var(--amber-500); }
  .fila .nombre {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: var(--type-body-sm, inherit);
  }
  .fila .resolucion,
  .fila .insignia-reproduciendo {
    flex-shrink: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
  }
  .fila .insignia-reproduciendo { color: var(--amber-500); }
  .espaciador { flex-shrink: 0; }
  .centinela { height: 1px; flex-shrink: 0; }
</style>
```

- [ ] **Step 6: Verificar que los tests pasan**

Run: `cd web && npx vitest run src/componentes/ListaCanalesLateral.test.ts`
Expected: PASS (6 tests).

- [ ] **Step 7: Gates completos** (ver Global Constraints — Go desde la raíz, web, mobile cero diffs).

- [ ] **Step 8: Commit**

```bash
git add web/src/componentes/ListaCanalesLateral.svelte web/src/componentes/ListaCanalesLateral.test.ts web/src/i18n/es.ts web/src/i18n/en.ts
git commit -m "feat(escenario): lista lateral de canales virtualizada con salud y roving tabindex"
```

---

### Task 2: `FacetasCompactas.svelte` — chips de país/género con conteos

**Files:**
- Create: `web/src/componentes/FacetasCompactas.svelte`
- Create: `web/src/componentes/FacetasCompactas.test.ts`

**Interfaces:**
- Consumes: `Faceta` (`{valor: string; total: number}`, de `web/src/datos/catalogo.ts`), store `filtros` (`web/src/estado/filtros.ts`), `banderaDePais`/`nombreDePais` (`web/src/lib/paises.ts`), `iconoDeCategoria` (`web/src/lib/categorias.ts`), `idioma`/`t` (i18n). Claves existentes `filtro.pais` y `filtro.categoria` como títulos de grupo (sr-only).
- Produces: componente con props `{ paises: Faceta[]; categorias: Faceta[] }` que ESCRIBE en `$filtros.pais`/`$filtros.categoria` (alternar: pulsar la activa la limpia — mismo contrato que `BarraLateralFacetas`). La Tarea 3 lo monta con esa firma.

- [ ] **Step 1: Escribir los tests (fallan)**

`web/src/componentes/FacetasCompactas.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import FacetasCompactas from './FacetasCompactas.svelte'
import { filtros } from '../estado/filtros'
import type { Faceta } from '../datos/catalogo'

const paises: Faceta[] = [
  { valor: 'MX', total: 900 }, { valor: 'ES', total: 800 }, { valor: 'AR', total: 700 },
  { valor: 'US', total: 600 }, { valor: 'FR', total: 500 }, { valor: 'DE', total: 400 },
  { valor: 'IT', total: 300 }, { valor: 'JP', total: 200 },
]
const categorias: Faceta[] = [{ valor: 'news', total: 50 }, { valor: 'sports', total: 40 }]

beforeEach(() => {
  filtros.set({ q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false, soloFavoritos: false, vista: 'rejilla' })
})

describe('FacetasCompactas', () => {
  it('muestra como mucho el TOP 6 por conteo de cada grupo, no las 8 facetas', () => {
    render(FacetasCompactas, { paises, categorias })
    expect(screen.queryByRole('button', { name: /MX/ })).toBeTruthy()
    // IT (300) y JP (200) quedan fuera del top 6
    expect(screen.queryByRole('button', { name: /IT/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /JP/ })).toBeNull()
  })

  it('cada chip lleva su conteo y alternar escribe/limpia el filtro (aria-pressed)', async () => {
    render(FacetasCompactas, { paises, categorias })
    const chip = screen.getByRole('button', { name: /MX/ })
    expect(chip.textContent).toContain('900')
    await fireEvent.click(chip)
    expect(get(filtros).pais).toBe('MX')
    expect(chip.getAttribute('aria-pressed')).toBe('true')
    await fireEvent.click(chip)
    expect(get(filtros).pais).toBe('')
  })

  it('una faceta ACTIVA fuera del top 6 aparece igualmente (si no, sería imposible verla/quitarla)', () => {
    filtros.update((f) => ({ ...f, pais: 'JP' }))
    render(FacetasCompactas, { paises, categorias })
    const chip = screen.getByRole('button', { name: /JP/ })
    expect(chip.getAttribute('aria-pressed')).toBe('true')
  })
})
```

- [ ] **Step 2: Verificar que falla**

Run: `cd web && npx vitest run src/componentes/FacetasCompactas.test.ts`
Expected: FAIL — módulo inexistente.

- [ ] **Step 3: Implementar**

`web/src/componentes/FacetasCompactas.svelte`:

```svelte
<script lang="ts">
  import type { Faceta } from '../datos/catalogo'
  import { filtros } from '../estado/filtros'
  import { banderaDePais, nombreDePais } from '../lib/paises'
  import { iconoDeCategoria } from '../lib/categorias'
  import { idioma, t } from '../i18n'

  // Versión CONDENSADA de las facetas para la barra lateral del escenario
  // (spec §3): chips del top-N por conteo, no el panel ancho de
  // BarraLateralFacetas (que sigue vivo en el modo ver-todo). Escribe en el
  // MISMO store de filtros con el MISMO contrato de alternar.
  let { paises, categorias }: { paises: Faceta[]; categorias: Faceta[] } = $props()

  const TOPE_CHIPS = 6

  // La faceta ACTIVA se cuela aunque no esté en el top: sin esto, filtrar por
  // un país minoritario dejaría el chip activo invisible e imposible de quitar
  // desde aquí.
  function top(facetas: Faceta[], activa: string): Faceta[] {
    const orden = [...facetas].sort((a, b) => b.total - a.total).slice(0, TOPE_CHIPS)
    if (activa && !orden.some((f) => f.valor === activa)) {
      const extra = facetas.find((f) => f.valor === activa)
      if (extra) orden.push(extra)
    }
    return orden
  }

  const chipsPais = $derived(top(paises, $filtros.pais ?? ''))
  const chipsCategoria = $derived(top(categorias, $filtros.categoria ?? ''))

  function alternarPais(valor: string) {
    $filtros.pais = $filtros.pais === valor ? '' : valor
  }
  function alternarCategoria(valor: string) {
    $filtros.categoria = $filtros.categoria === valor ? '' : valor
  }
</script>

<div class="facetas-compactas">
  <section class="grupo" role="group" aria-label={t('filtro.pais')}>
    {#each chipsPais as f (f.valor)}
      <button
        type="button"
        class="chip"
        aria-pressed={$filtros.pais === f.valor}
        aria-label={`${nombreDePais(f.valor, idioma.actual)} · ${f.total}`}
        onclick={() => alternarPais(f.valor)}
      >
        <span aria-hidden="true">{banderaDePais(f.valor)}</span>
        {f.valor}
        <span class="conteo">{f.total}</span>
      </button>
    {/each}
  </section>
  <section class="grupo" role="group" aria-label={t('filtro.categoria')}>
    {#each chipsCategoria as f (f.valor)}
      <button
        type="button"
        class="chip"
        aria-pressed={$filtros.categoria === f.valor}
        onclick={() => alternarCategoria(f.valor)}
      >
        <span aria-hidden="true">{iconoDeCategoria(f.valor)}</span>
        {f.valor}
        <span class="conteo">{f.total}</span>
      </button>
    {/each}
  </section>
</div>

<style>
  .facetas-compactas { display: flex; flex-direction: column; gap: var(--space-2, 8px); }
  .grupo { display: flex; flex-wrap: wrap; gap: var(--space-1, 4px); }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: none;
    border: 1px solid var(--border-default);
    border-radius: 999px;
    padding: 2px 8px;
    color: var(--text-body);
    cursor: pointer;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
  }
  .chip:hover { background: var(--surface-raised); }
  .chip .conteo { color: var(--text-muted); font-variant-numeric: tabular-nums; }
  /* Ámbar = faceta activa, misma regla que BarraLateralFacetas. */
  .chip[aria-pressed='true'] {
    background: var(--tint-amber-weak);
    border-color: var(--tint-amber-line);
    color: var(--amber-500);
  }
  .chip[aria-pressed='true'] .conteo { color: var(--amber-500); }
</style>
```

- [ ] **Step 4: Verificar que pasa**

Run: `cd web && npx vitest run src/componentes/FacetasCompactas.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Gates completos.**

- [ ] **Step 6: Commit**

```bash
git add web/src/componentes/FacetasCompactas.svelte web/src/componentes/FacetasCompactas.test.ts
git commit -m "feat(escenario): facetas compactas — chips de país/género con conteos"
```

---

### Task 3: `CatalogoLateral.svelte` — composición: buscador + facetas + lista

**Files:**
- Create: `web/src/componentes/CatalogoLateral.svelte`
- Create: `web/src/componentes/CatalogoLateral.test.ts`

**Interfaces:**
- Consumes: `ListaCanalesLateral` (Tarea 1: `{canales, canalActualId, alAbrir, alPedirMas}`), `FacetasCompactas` (Tarea 2: `{paises, categorias}`), store `filtros`, `debounce` (`web/src/lib/debounce.ts`), claves i18n existentes `catalogo.buscar` («Buscar un canal») y `catalogo.total` (`{n} canales`).
- Produces: componente con props `{ canales: Canal[]; total: number; cargando: boolean; paises: Faceta[]; categorias: Faceta[]; canalActualId?: string | null; alAbrir: (c: Canal) => void; alPedirMas: () => void }`. La Tarea 5 lo monta en App con esa firma.

**Nota:** el buscador replica el patrón buscador↔store de `BarraLateralFacetas.svelte` (debounce 300 ms + `ultimoEmpujado` para no pisar lo tecleado cuando `q` cambia desde fuera) — léelo antes de escribir; copia el patrón, con el mismo comentario de por qué.

- [ ] **Step 1: Escribir los tests (fallan)**

`web/src/componentes/CatalogoLateral.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import CatalogoLateral from './CatalogoLateral.svelte'
import { filtros } from '../estado/filtros'
import type { Canal } from '../datos/catalogo'
import { t } from '../i18n'

class FalsoIntersectionObserver {
  constructor(private cb: IntersectionObserverCallback) {}
  observe() {}
  disconnect() {}
  unobserve() {}
  takeRecords() { return [] }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalDePrueba(id: string): Canal {
  return { id, nombre: `Canal ${id}`, logoUrl: '', categoriaId: '', idioma: 'es', pais: '', vivo: true, latenciaMs: 10, webOk: true }
}

const props = {
  canales: [canalDePrueba('a')],
  total: 1234,
  cargando: false,
  paises: [{ valor: 'MX', total: 9 }],
  categorias: [{ valor: 'news', total: 5 }],
  alAbrir: vi.fn(),
  alPedirMas: vi.fn(),
}

beforeEach(() => {
  vi.useRealTimers()
  filtros.set({ q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false, soloFavoritos: false, vista: 'rejilla' })
})

describe('CatalogoLateral', () => {
  it('monta buscador, chips de facetas, conteo total y la lista de canales', () => {
    render(CatalogoLateral, props)
    expect(screen.getByLabelText(t('catalogo.buscar'))).toBeTruthy()
    expect(screen.getByRole('button', { name: /MX/ })).toBeTruthy()
    expect(screen.getByText(t('catalogo.total', { n: 1234 }))).toBeTruthy()
    expect(screen.getByRole('button', { name: /Canal a/ })).toBeTruthy()
  })

  it('teclear en el buscador escribe filtros.q tras el debounce', async () => {
    vi.useFakeTimers()
    render(CatalogoLateral, props)
    const input = screen.getByLabelText(t('catalogo.buscar'))
    await fireEvent.input(input, { target: { value: 'tele' } })
    expect(get(filtros).q).toBe('')
    vi.advanceTimersByTime(350)
    expect(get(filtros).q).toBe('tele')
  })
})
```

- [ ] **Step 2: Verificar que falla**

Run: `cd web && npx vitest run src/componentes/CatalogoLateral.test.ts`
Expected: FAIL — módulo inexistente.

- [ ] **Step 3: Implementar**

`web/src/componentes/CatalogoLateral.svelte`:

```svelte
<script lang="ts">
  import { untrack } from 'svelte'
  import type { Canal, Faceta } from '../datos/catalogo'
  import { filtros } from '../estado/filtros'
  import { debounce } from '../lib/debounce'
  import { t } from '../i18n'
  import FacetasCompactas from './FacetasCompactas.svelte'
  import ListaCanalesLateral from './ListaCanalesLateral.svelte'

  // Barra lateral de catálogo del escenario (spec §3): buscador + facetas
  // compactas + lista virtualizada. NO habla con CatalogSource — App sigue
  // siendo el único que consulta; aquí solo llegan datos y salen callbacks.
  let { canales, total, cargando, paises, categorias, canalActualId = null, alAbrir, alPedirMas }: {
    canales: Canal[]
    total: number
    cargando: boolean
    paises: Faceta[]
    categorias: Faceta[]
    canalActualId?: string | null
    alAbrir: (c: Canal) => void
    alPedirMas: () => void
  } = $props()

  // Buscador↔store: mismo patrón (y mismo porqué) que BarraLateralFacetas —
  // debounce para no consultar por tecla; ultimoEmpujado para distinguir el
  // eco de nuestro propio empuje de un cambio llegado de fuera (chips,
  // «Limpiar filtros») sin pisar lo que el usuario está tecleando.
  let busqueda = $state($filtros.q ?? '')
  let ultimoEmpujado = $state($filtros.q ?? '')
  const buscarConRetardo = debounce((valor: string) => {
    $filtros.q = valor
    ultimoEmpujado = valor
  }, 300)
  function alEscribir(evento: Event) {
    busqueda = (evento.target as HTMLInputElement).value
    buscarConRetardo(busqueda)
  }
  $effect(() => {
    const qActual = $filtros.q ?? ''
    if (qActual !== untrack(() => ultimoEmpujado)) {
      busqueda = qActual
      ultimoEmpujado = qActual
    }
  })
</script>

<div class="catalogo-lateral">
  <div class="buscador">
    <span class="motivo" aria-hidden="true">&lt;</span>
    <input
      type="search"
      class="buscar"
      placeholder={t('catalogo.buscar')}
      aria-label={t('catalogo.buscar')}
      value={busqueda}
      oninput={alEscribir}
    />
  </div>

  <FacetasCompactas {paises} {categorias} />

  <p class="conteo-total">{t('catalogo.total', { n: total })}</p>

  <ListaCanalesLateral {canales} {canalActualId} {alAbrir} {alPedirMas} />

  {#if cargando}<p class="cargando">{t('catalogo.cargando')}</p>{/if}
  {#if !cargando && canales.length === 0}<p class="vacio">{t('catalogo.vacio')}</p>{/if}
</div>

<style>
  .catalogo-lateral {
    display: flex;
    flex-direction: column;
    gap: var(--space-3, 12px);
    height: 100%;
    min-height: 0;
  }
  /* Mismo tratamiento que el buscador de BarraLateralFacetas. */
  .buscador {
    display: flex;
    align-items: center;
    gap: 6px;
    background: var(--surface-sunken);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md, 8px);
    padding: 6px 10px;
  }
  .motivo { color: var(--amber-500); font: var(--type-mono, inherit); }
  .buscar { flex: 1; background: none; border: none; color: var(--text-body); font: var(--type-body, inherit); min-width: 0; }
  .buscar:focus { outline: none; }
  .conteo-total {
    margin: 0;
    font: var(--type-mono-label, inherit);
    letter-spacing: var(--tracking-mono, normal);
    color: var(--text-muted);
  }
  .cargando, .vacio { margin: 0; color: var(--text-muted); font: var(--type-body-sm, inherit); }
</style>
```

- [ ] **Step 4: Verificar que pasa**

Run: `cd web && npx vitest run src/componentes/CatalogoLateral.test.ts`
Expected: PASS (2 tests).

- [ ] **Step 5: Gates completos.**

- [ ] **Step 6: Commit**

```bash
git add web/src/componentes/CatalogoLateral.svelte web/src/componentes/CatalogoLateral.test.ts
git commit -m "feat(escenario): catálogo lateral — buscador + facetas compactas + lista"
```

---

### Task 4: `Reproductor` — modo panel (dual transitorio: `modo: 'modal' | 'panel'`)

**Files:**
- Modify: `web/src/componentes/Reproductor.svelte`
- Modify: `web/src/componentes/Reproductor.test.ts` (añadir describe de modo panel; los tests modales existentes se quedan verdes)
- Modify: `web/src/i18n/es.ts`, `web/src/i18n/en.ts`

**Interfaces:**
- Consumes: `esObjetivoInteractivo(target: EventTarget | null): boolean` de `web/src/lib/surf.ts` (la misma guarda que App usa para el surf y ⌘K).
- Produces: props NUEVAS del Reproductor — la Tarea 5 monta con `modo="panel"`:
  - `modo?: 'modal' | 'panel'` (por defecto `'modal'` → comportamiento actual byte a byte; App no se toca en esta tarea).
  - `activo?: boolean` (por defecto `true`): con `false`, el manejador de teclado global del reproductor se ignora entero (App lo apagará cuando ver-todo/una vista-hash/la paleta estén encima).
  - `alCerrar?: () => void` pasa a OPCIONAL (el panel no cierra; el modal lo sigue usando).
  - `silenciadoInicial?: boolean` (por defecto `false`): arranca `muted` y muestra la CTA grande «Toca para activar el sonido».
- **Esta pieza es la delicada de la spec (§4): el MOTOR no se toca** — `planDeFailover`, `PlaybackGuard`, CTA «Probar el siguiente mirror», EPG + refresco, PiP, auto-ocultar del overlay quedan idénticos; cambian el envoltorio y el modelo de foco. Los tests existentes de failover/EPG/PiP DEBEN seguir verdes sin editarlos.

Comportamiento del modo panel (todo condicionado a `modo === 'panel'`):

1. Sin `role="dialog"`/`aria-modal`: el contenedor raíz es `role="region"` con `aria-label={canal.nombre}`.
2. Sin focus-trap: el case `'Tab'` de `alTeclado` solo corre en modal. Sin foco al montar ni restauración al desmontar (`onMount`/`onDestroy` de foco solo en modal).
3. `Escape`: si `document.fullscreenElement`, salir; si no, NADA (no hay modal que cerrar) — precedencia exacta de §4.
4. Teclado global: primera línea de `alTeclado`: `if (!activo) return`; segunda, solo en panel: `if (esObjetivoInteractivo(e.target)) return` — espacio en el buscador de la lateral escribe un espacio, nunca pausa.
5. Barra inferior `.controles` (AirPlay + Cerrar) solo en modal. En panel, el botón AirPlay (mismo `abrirSelectorAirplay`, mismo feature-detect `soportaAirplay`) se añade a `.overlay-controles`, junto a PiP.
6. Pantalla completa: sigue pidiéndose sobre el contenedor raíz (renombrar `contenedorDialogo` → `contenedorRaiz` en todo el fichero — en panel ya no es un diálogo); el overlay sigue visible dentro (mismo patrón P0.8).
7. Silencio de entrada: `silenciado` se inicializa a `silenciadoInicial`; estado nuevo `mostrarActivarSonido` (inicial = `silenciadoInicial`). Mientras es `true` y no hay error, se pinta una CTA grande centrada-baja sobre el vídeo: `t('reproductor.activarSonido')`. Pulsar la CTA (o Silenciar manualmente hacia no-silenciado) → `silenciado = false`, `video.muted = false`, `mostrarActivarSonido = false`. Además, **cambiar de canal mientras la CTA sigue visible desactiva el silencio solo** (elegir canal ES el primer gesto del usuario, spec §5): en el `$effect` de `canal.id`, guarda `idCanalPrevio` y, si hay cambio real y `mostrarActivarSonido`, llama a `activarSonido()`.
8. Estilo: `data-modo={modo}` en el raíz. `.reproductor[data-modo='modal']` conserva `position:fixed; inset:0; z-index; animation`. `.reproductor[data-modo='panel']` = `position:relative; width:100%; aspect-ratio:16/9; background:#000; border-radius:var(--radius-md); overflow:hidden;` sin animación de entrada (persiste, no «entra»), y `.reproductor[data-modo='panel']:fullscreen { aspect-ratio:auto; border-radius:0; height:100%; }`. La CTA nueva no lleva animación → nada nuevo que anular en `prefers-reduced-motion` (déjalo dicho en un comentario).

- [ ] **Step 1: Añadir claves i18n**

`es.ts`: `'reproductor.activarSonido': 'Toca para activar el sonido',` — `en.ts`: `'reproductor.activarSonido': 'Tap to turn on sound',`

- [ ] **Step 2: Escribir el describe nuevo (falla)**

Añadir a `web/src/componentes/Reproductor.test.ts` (reutiliza los helpers/mocks existentes del fichero — `canal`, la fábrica de `fuente` falsa, el mock de hls.js):

```ts
describe('Reproductor — modo panel (reproductor-primero)', () => {
  it('en modo panel no hay role=dialog ni aria-modal: es una region etiquetada con el canal', () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), modo: 'panel' })
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(screen.getByRole('region', { name: canal.nombre })).toBeTruthy()
  })

  it('en modo panel, Escape SIN pantalla completa no llama a alCerrar (ya no hay modal que cerrar)', async () => {
    const alCerrar = vi.fn()
    render(Reproductor, { canal, fuente: fuenteFalsa(), modo: 'panel', alCerrar })
    await fireEvent.keyDown(window, { key: 'Escape' })
    expect(alCerrar).not.toHaveBeenCalled()
  })

  it('con activo=false el teclado global del reproductor queda inerte (espacio no pausa)', async () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), modo: 'panel', activo: false })
    const video = document.querySelector('video') as HTMLVideoElement
    const pause = vi.spyOn(video, 'pause')
    const play = vi.spyOn(video, 'play').mockResolvedValue()
    await fireEvent.keyDown(window, { key: ' ' })
    expect(pause).not.toHaveBeenCalled()
    expect(play).not.toHaveBeenCalled()
  })

  it('en modo panel, una tecla con el foco en un control interactivo AJENO se ignora (esObjetivoInteractivo)', async () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), modo: 'panel' })
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    const video = document.querySelector('video') as HTMLVideoElement
    const pause = vi.spyOn(video, 'pause')
    await fireEvent.keyDown(window, { key: ' ' })
    expect(pause).not.toHaveBeenCalled()
    input.remove()
  })

  it('silenciadoInicial arranca muted y muestra la CTA; pulsarla activa el sonido y la retira', async () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), modo: 'panel', silenciadoInicial: true })
    const video = document.querySelector('video') as HTMLVideoElement
    expect(video.muted).toBe(true)
    const cta = screen.getByRole('button', { name: t('reproductor.activarSonido') })
    await fireEvent.click(cta)
    expect(video.muted).toBe(false)
    expect(screen.queryByRole('button', { name: t('reproductor.activarSonido') })).toBeNull()
  })

  it('cambiar de canal con la CTA visible activa el sonido solo (elegir canal ES el gesto)', async () => {
    const { rerender } = render(Reproductor, { canal, fuente: fuenteFalsa(), modo: 'panel', silenciadoInicial: true })
    await rerender({ canal: { ...canal, id: 'c2', nombre: 'Otro' } })
    await tick()
    const video = document.querySelector('video') as HTMLVideoElement
    expect(video.muted).toBe(false)
    expect(screen.queryByRole('button', { name: t('reproductor.activarSonido') })).toBeNull()
  })

  it('en modo panel el botón AirPlay vive en el overlay (no hay barra inferior con Cerrar)', () => {
    // Simular soporte AirPlay ANTES de montar, como hace el describe modal
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    render(Reproductor, { canal, fuente: fuenteFalsa(), modo: 'panel' })
    expect(screen.queryByRole('button', { name: t('reproductor.cerrar') })).toBeNull()
    const airplay = screen.getByRole('button', { name: 'AirPlay' })
    expect(airplay.closest('.overlay-controles')).toBeTruthy()
    delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
  })
})
```

Ajusta los nombres de helper a los REALES del fichero (p. ej. si la fuente falsa se construye inline, replica ese estilo). Si `rerender` no re-ejecuta el `$effect` de `canal.id` en @testing-library/svelte 5, monta con un prop `$state` externo como hacen los tests de failover existentes.

- [ ] **Step 3: Verificar que el describe nuevo falla**

Run: `cd web && npx vitest run src/componentes/Reproductor.test.ts`
Expected: FAIL solo el describe nuevo; los describes existentes (failover, overlay 1b, fullscreen/PiP, EPG) PASS.

- [ ] **Step 4: Implementar el modo panel en `Reproductor.svelte`** según la lista numerada de arriba. Puntos exactos:
  - Props: añadir `modo = 'modal'`, `activo = true`, `silenciadoInicial = false`; `alCerrar` → `alCerrar?: () => void` (los call-sites del modal ya comprueban existencia con `alCerrar()` directo — cámbialo a `alCerrar?.()`).
  - `let silenciado = $state(silenciadoInicial)` y `let mostrarActivarSonido = $state(silenciadoInicial)`; función `activarSonido()` central.
  - `alTeclado`: `if (!activo) return` primero; `if (modo === 'panel' && esObjetivoInteractivo(e.target)) return` después; el case `'Escape'` en panel solo sale de fullscreen; el case `'Tab'` con `if (modo !== 'panel')`.
  - `onMount`/`onDestroy`: envolver la captura/restauración de foco en `if (modo === 'modal')` (la limpieza de PiP y `limpiarIntento` siguen incondicionales).
  - `$effect` de `canal.id`: añadir `idCanalPrevio` y el auto-`activarSonido()` en cambio real con CTA visible.
  - Markup: atributos `role`/`aria-modal` condicionales; CTA `<button class="activar-sonido">` dentro de `.lienzo` con `{#if mostrarActivarSonido && !mensajeError}`; `{#if soportaAirplay && modo === 'panel'}` botón AirPlay en `.overlay-controles`; `{#if modo === 'modal'}` alrededor de `.controles`.
  - CSS: mover lo fijo/modal a `[data-modo='modal']`, añadir el bloque `[data-modo='panel']` (punto 8 de arriba) y el estilo de `.activar-sonido` (relleno ámbar, mismo tratamiento que `.probar-mirror`; posicionado `position:absolute; bottom:20%; left:50%; transform:translateX(-50%)`).

- [ ] **Step 5: Verificar TODO el fichero verde**

Run: `cd web && npx vitest run src/componentes/Reproductor.test.ts`
Expected: PASS — modal y panel a la vez.

- [ ] **Step 6: Gates completos** (incluye `npm run check`: los tipos de los props nuevos).

- [ ] **Step 7: Commit**

```bash
git add web/src/componentes/Reproductor.svelte web/src/componentes/Reproductor.test.ts web/src/i18n/es.ts web/src/i18n/en.ts
git commit -m "feat(reproductor): modo panel — sin dialog/trap, Esc solo fullscreen, CTA de sonido, AirPlay al overlay"
```

---

### Task 5: App — escenario reproductor-primero + modo «ver todo» (la reestructuración)

**Files:**
- Modify: `web/src/App.svelte` (la pieza central)
- Modify: `web/src/componentes/Reproductor.svelte` + `Reproductor.test.ts` (BORRAR el modo modal y sus tests — fin del andamiaje dual de la Tarea 4)
- Modify: `web/src/App.integracion.test.ts`, `web/src/lib/a11y-p06.test.ts`, `web/src/lib/a11y-p08.test.ts`
- Modify: `web/src/estilos/tokens/spacing.css` (token `--ancho-catalogo`)
- Modify: `web/src/i18n/es.ts`, `web/src/i18n/en.ts`

**Interfaces:**
- Consumes: `CatalogoLateral` (Tarea 3), `Reproductor` en modo panel (Tarea 4: `{canal, fuente, activo, silenciadoInicial, alDesenlace, alAnterior, alSiguiente}` — tras esta tarea `modo` y `alCerrar` YA NO EXISTEN), `historial`/`EntradaHistorial`, `tick` de svelte.
- Produces: los tres estados de nivel superior de la spec §2. Estado nuevo de App que la Tarea 6 extiende: `canalActual: Canal | null` (renombrado desde `canalAbierto`), `modoVerTodo: boolean`, función `canalDesdeHistorial(entrada: EntradaHistorial): Canal`, derivadas `enCatalogo`/`escenarioVisible`/`reproductorActivo`.

- [ ] **Step 1: Claves i18n**

`es.ts`:
```ts
'escenario.verTodo': 'Ver todo',
'escenario.volver': 'Volver al reproductor',
'escenario.eligeCanal': 'Elige un canal para empezar',
'escenario.anuncioReproduciendo': 'Reproduciendo {nombre}',
```
`en.ts`:
```ts
'escenario.verTodo': 'See all',
'escenario.volver': 'Back to the player',
'escenario.eligeCanal': 'Pick a channel to start',
'escenario.anuncioReproduciendo': 'Playing {nombre}',
```

Y en `web/src/estilos/tokens/spacing.css`, junto al resto de tokens: `--ancho-catalogo: 372px;` (el ancho del 1b como token, spec §3).

- [ ] **Step 2: Escribir los tests de integración nuevos (fallan)**

Añadir a `web/src/App.integracion.test.ts` (reutiliza `fuenteFalsa`, `canalDePrueba`, el mock de salud — ya existen). Helper compartido nuevo:

```ts
async function abrirVerTodo() {
  await fireEvent.click(screen.getByRole('button', { name: t('escenario.verTodo') }))
  await tick()
}
```

Tests:

```ts
describe('escenario reproductor-primero', () => {
  function fuenteConCanales(n = 3) {
    const lista = Array.from({ length: n }, (_, i) => canalDePrueba(`c${i}`))
    return fuenteFalsa({
      canales: vi.fn(async () => ({ canales: lista, total: n })),
    })
  }

  it('con catálogo y sin canal en curso, el panel de vídeo muestra «elige un canal» y la lateral lista los canales', async () => {
    render(App, { fuente: fuenteConCanales() })
    await vi.waitFor(() => expect(screen.getByText(t('escenario.eligeCanal'))).toBeTruthy())
    expect(screen.getByRole('button', { name: /c1/ })).toBeTruthy()
    // No hay modal: nada con role=dialog, y el fondo NO está inert
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(document.querySelector('.fondo')?.hasAttribute('inert')).toBe(false)
  })

  it('clicar un canal en la lateral reproduce EN EL SITIO: aparece el vídeo, sin modal, y cambiar de canal NO desmonta el <video>', async () => {
    render(App, { fuente: fuenteConCanales() })
    await vi.waitFor(() => expect(screen.getByRole('button', { name: /c0/ })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: /c0/ }))
    await tick()
    const video = document.querySelector('video')
    expect(video).toBeTruthy()
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: /c1/ }))
    await tick()
    expect(document.querySelector('video')).toBe(video) // mismo nodo: swap in place
  })

  it('«Ver todo» abre la rejilla rica (P0.6 conservada), oculta el escenario SIN desmontar el vídeo, y el foco aterriza en «Volver»', async () => {
    render(App, { fuente: fuenteConCanales() })
    await vi.waitFor(() => expect(screen.getByRole('button', { name: /c0/ })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: /c0/ }))
    await tick()
    await abrirVerTodo()
    // La rejilla/acciones de siempre están de vuelta
    expect(screen.getByRole('button', { name: t('accion.favoritos') })).toBeTruthy()
    // El escenario sigue MONTADO (el video existe) pero oculto
    expect(document.querySelector('.escenario.oculto video')).toBeTruthy()
    expect(document.activeElement?.textContent).toContain(t('escenario.volver'))
  })

  it('seleccionar un canal en ver-todo vuelve al escenario reproduciéndolo', async () => {
    render(App, { fuente: fuenteConCanales() })
    await vi.waitFor(() => expect(screen.getByRole('button', { name: /c0/ })).toBeTruthy())
    await abrirVerTodo()
    // La tarjeta c2 de la rejilla (markup de TarjetaCanal: botón con el nombre)
    await fireEvent.click(screen.getAllByRole('button', { name: /c2/ })[0])
    await tick()
    expect(document.querySelector('.escenario.oculto')).toBeNull()
    expect(screen.getByRole('region', { name: /c2/ })).toBeTruthy()
  })

  it('la región polite anuncia «Reproduciendo <nombre>» al cambiar de canal (sin regiones live nuevas)', async () => {
    render(App, { fuente: fuenteConCanales() })
    await vi.waitFor(() => expect(screen.getByRole('button', { name: /c0/ })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: /c0/ }))
    await tick()
    const politeRegions = document.querySelectorAll('[aria-live="polite"]')
    const deApp = politeRegions[0] // la persistente de App va primera en el DOM
    expect(deApp.textContent).toContain('c0')
  })

  it('el surf por barra espaciadora funciona en ver-todo y NO en el escenario (ahí el espacio es del reproductor)', async () => {
    const fuente = fuenteConCanales()
    render(App, { fuente })
    await vi.waitFor(() => expect(screen.getByRole('button', { name: /c0/ })).toBeTruthy())
    await fireEvent.keyDown(window, { key: ' ', code: 'Space' })
    expect(fuente.aleatorio).not.toHaveBeenCalled()
    await abrirVerTodo()
    await fireEvent.keyDown(window, { key: ' ', code: 'Space' })
    expect(fuente.aleatorio).toHaveBeenCalled()
  })

  it('⌘K abre la paleta también con un canal reproduciéndose (ya no hay modal que la inhiba) y el fondo queda inert', async () => {
    render(App, { fuente: fuenteConCanales() })
    await vi.waitFor(() => expect(screen.getByRole('button', { name: /c0/ })).toBeTruthy())
    await fireEvent.click(screen.getByRole('button', { name: /c0/ }))
    await tick()
    await fireEvent.keyDown(window, { key: 'k', metaKey: true })
    await tick()
    expect(document.querySelector('.fondo')?.hasAttribute('inert')).toBe(true)
  })
})
```

- [ ] **Step 3: Verificar que fallan**

Run: `cd web && npx vitest run src/App.integracion.test.ts -t 'escenario reproductor-primero'`
Expected: FAIL — «escenario.verTodo» sin traducir aún es imposible (ya añadida en Step 1), fallará por markup inexistente.

- [ ] **Step 4: Reestructurar `App.svelte`**

Cambios de `<script>`:

1. Renombrar `canalAbierto` → `canalActual` en todo el fichero. Borrar `cerrarReproductor`.
2. Estado nuevo:
```ts
let modoVerTodo = $state(false)
let anuncioCanal = $state('')
let botonVerTodo = $state<HTMLButtonElement | undefined>()
let botonVolverEscenario = $state<HTMLButtonElement | undefined>()
```
3. Extraer el puente historial→Canal (lo reutiliza la Tarea 6):
```ts
function canalDesdeHistorial(entrada: EntradaHistorial): Canal {
  const enCatalogo = canales.find((c) => c.id === entrada.canalId)
  return (
    enCatalogo ?? {
      id: entrada.canalId, nombre: entrada.nombre, logoUrl: entrada.logoUrl ?? '',
      categoriaId: '', idioma: '', pais: '', vivo: null, latenciaMs: 0, webOk: null,
    }
  )
}
function abrirDesdeHistorial(entrada: EntradaHistorial) {
  abrirCanal(canalDesdeHistorial(entrada))
}
```
4. `abrirCanal` — cambio en el sitio + anuncio + retorno desde ver-todo:
```ts
function abrirCanal(canal: Canal) {
  canalActual = canal
  historial.registrar(canal)
  anuncioCanal = t('escenario.anuncioReproduciendo', { nombre: canal.nombre })
  if (modoVerTodo) volverAlEscenario()
}
```
5. Alternar ver-todo con gestión de foco (el botón pulsado desaparece con el modo — sin mover el foco a mano, caería a `<body>`):
```ts
async function abrirVerTodo() {
  modoVerTodo = true
  await tick()
  botonVolverEscenario?.focus()
}
async function volverAlEscenario() {
  modoVerTodo = false
  await tick()
  botonVerTodo?.focus()
}
```
6. Derivadas nuevas (después de `sinFuentes`):
```ts
// Los tres estados de nivel superior (spec §2): sinFuentes → Onboarding;
// modoVerTodo → rejilla completa; por defecto → escenario.
let enCatalogo = $derived(
  fase.tipo === 'listo' && !sinFuentes && !sondeandoFuenteNueva && !sondeoAgotado &&
  !vistaFuentes && !vistaAjustes && !vistaStats,
)
let escenarioVisible = $derived(enCatalogo && !modoVerTodo)
let reproductorActivo = $derived(escenarioVisible && !paletaAbierta)
```
7. Guardas de teclado en `alTeclaVentana`:
   - ⌘K: la condición pasa de `if (canalAbierto || paletaAbierta) return` a `if (paletaAbierta) return` (§6: ya no hay modal reproductor).
   - Surf: en la condición larga, sustituir `canalAbierto ||` por `!modoVerTodo ||` — el surf vive SOLO en ver-todo (decisión 2 del plan; en el escenario el espacio es play/pausa del panel).
8. `mensajePoliteAccesible`: añadir la rama de MENOR prioridad al final de la cadena — donde hoy termina en `''`, termina en `anuncioCanal`.
9. `inert`: `.fondo`, `<main>` y `<footer>` pasan de `!!canalAbierto || paletaAbierta` a `paletaAbierta` a secas.

Cambios de markup (dentro de `<div class="fondo">`, tras `<header>`):

```svelte
<!-- ESCENARIO (spec §3): montado siempre que hay catálogo con fuentes — las
     otras vistas (ver-todo, #ajustes/#fuentes/#stats) lo CUBREN con
     display:none pero NUNCA lo desmontan: el <video> persiste y el audio
     continúa (decisión 1 del plan; es lo que hace verificable el
     «sin desmontar/remontar» de la spec §10). -->
{#if fase.tipo === 'listo' && fuentesCargadas && !sinFuentes}
  <div class="escenario" class:oculto={!escenarioVisible}>
    <div class="panel-video">
      {#if canalActual}
        <Reproductor
          canal={canalActual}
          {fuente}
          activo={reproductorActivo}
          alDesenlace={reportarDesenlace}
          alAnterior={canalAnterior}
          alSiguiente={canalSiguiente}
        />
      {:else}
        <div class="panel-espera"><p>{t('escenario.eligeCanal')}</p></div>
      {/if}
      <ContinuarViendo alAbrir={abrirDesdeHistorial} />
    </div>
    <aside class="lateral">
      <button type="button" class="ver-todo" bind:this={botonVerTodo} onclick={abrirVerTodo}>
        {t('escenario.verTodo')}
      </button>
      <CatalogoLateral
        {canales} {total} {cargando} {paises} {categorias}
        canalActualId={canalActual?.id ?? null}
        alAbrir={abrirCanal}
        {alPedirMas}
      />
    </aside>
  </div>
{/if}

{#if !escenarioVisible}
  <div class="cuerpo">
    <!-- …el .cuerpo actual ÍNTEGRO (botón-cajón + aside facetas + main con
         todas sus ramas), con UN cambio: en la rama final (catálogo normal,
         que ahora solo se alcanza en modoVerTodo), añadir ANTES de
         ContinuarViendo: -->
    <button type="button" class="volver-escenario" bind:this={botonVolverEscenario} onclick={volverAlEscenario}>
      ← {t('escenario.volver')}
    </button>
    <!-- …ContinuarViendo, BarraAcciones, pista-surf, RejillaCanales como hoy -->
  </div>
{/if}
```

Y borrar el bloque `{#if canalAbierto}<Reproductor …/>{/if}` del final (el de Paleta se queda; `alAbrirCanal={abrirCanal}` ya vuelve al escenario gracias al punto 4).

CSS nuevo del `<style>` de App:

```css
/* Escenario reproductor-primero (spec §3): vídeo 1fr | catálogo lateral con
   el ancho del 1b como token (--ancho-catalogo, tokens/spacing.css). */
.escenario {
  display: grid;
  grid-template-columns: minmax(0, 1fr) var(--ancho-catalogo, 372px);
  gap: var(--space-4, 16px);
  align-items: start;
  padding: var(--space-4, 1rem) var(--space-6, 2rem);
  background: var(--surface-abyss);
}
.escenario.oculto { display: none; }
.panel-video { min-width: 0; display: flex; flex-direction: column; gap: var(--space-3, 12px); }
.panel-espera {
  aspect-ratio: 16 / 9;
  display: grid;
  place-items: center;
  background: var(--graphite-900);
  border-radius: var(--radius-md, 8px);
  color: var(--text-muted);
}
.lateral {
  display: flex;
  flex-direction: column;
  gap: var(--space-3, 12px);
  position: sticky;
  top: var(--space-4, 1rem);
  height: calc(100dvh - 2 * var(--space-4, 1rem));
  min-height: 0;
}
.ver-todo, .volver-escenario {
  background: none; border: 1px solid var(--border-default); color: var(--text-body);
  border-radius: 6px; padding: 4px 10px; cursor: pointer; align-self: flex-start;
}
.volver-escenario { margin-bottom: var(--space-3, 12px); }

/* Responsive estrecho (spec §8, decisión 3 del plan): apilado — vídeo arriba
   16:9, catálogo debajo. El vídeo no se pierde al navegar la lista. */
@media (max-width: 900px) {
  .escenario { grid-template-columns: 1fr; }
  .lateral { position: static; height: auto; max-height: 60dvh; }
}
```

- [ ] **Step 5: Borrar el modo modal del Reproductor**

En `Reproductor.svelte`: eliminar el prop `modo` (y `alCerrar`), y todo lo condicionado a modal: `role="dialog"`/`aria-modal` (queda `role="region"`), el case `'Tab'`, la rama Escape→`alCerrar`, la captura/restauración de foco de `onMount`/`onDestroy`, la barra `.controles` entera con el botón Cerrar (el AirPlay del overlay pierde su `modo === 'panel'` y queda solo con `soportaAirplay`), el CSS `[data-modo='modal']` y la animación de entrada + su `prefers-reduced-motion` asociado, y el atributo `data-modo` (el panel es el único modo). En `Reproductor.test.ts`: borrar los tests que ejercitaban modal (Escape cierra, Tab envuelve, botón Cerrar, foco al montar/restaurar) y quitar `modo: 'panel'`/`alCerrar` de los props de TODOS los render — el resto de tests (failover, EPG, PiP, fullscreen, overlay, panel) se conservan; en los de fullscreen, el contenedor es ahora el panel (`.reproductor`), mismo `contenedorRaiz`.

- [ ] **Step 6: Adaptar los tests existentes de App y a11y**

Regla general: los tests que ejercitan la REJILLA/facetas/acciones/surf ahora viven detrás de «Ver todo» — se les antepone `await abrirVerTodo()` (helper del Step 2). Los que ejercitan abrir/cerrar el reproductor modal se reescriben al contrato nuevo. Concretamente:

- `App.integracion.test.ts`: (a) todo test que espere `BarraAcciones`/`RejillaCanales`/chips/pista-surf montados por defecto → `abrirVerTodo()` primero; (b) tests de «abrir canal → dialog / cerrar reproductor» → sustituidos por los del Step 2 (borrar los obsoletos); (c) tests de Onboarding/fuentes/sondeo/stats/ajustes NO cambian (esas ramas viven igual); (d) el test del surf existente → ver-todo.
- `a11y-p06.test.ts`: los describes del cajón responsive y del vacío en la región polite renderizan App → añadir `abrirVerTodo()` tras el waitFor inicial (el aside de facetas ya no está montado en el escenario). Los describes de componentes sueltos (BarraLateralFacetas, BarraAcciones, ContinuarViendo, IndicadorSenal, chips) no cambian.
- `a11y-p08.test.ts`: el describe «focus-trap completo» del Reproductor se BORRA y se sustituye por uno nuevo «Reproductor panel: sin trap, en el orden natural» con dos tests: (1) el contenedor no tiene `role=dialog` y ningún control lleva tabindex>0; (2) los controles del overlay conservan nombre accesible + `aria-pressed` (mover aquí las aserciones aún válidas del describe borrado). El describe del guard focus-within del auto-ocultar se CONSERVA (sigue aplicando al panel) — solo ajusta los props del render.

- [ ] **Step 7: Verificar todo el suite web**

Run: `cd web && npm run check && npm test`
Expected: PASS completo (integración + a11y + Reproductor + componentes nuevos).

- [ ] **Step 8: Gates completos + gate visual en Chrome**

Además de los gates: reconstruir y mirar con ojos (CLAUDE.md «Entorno de desarrollo»): `cd web && npm run build`, desde la RAÍZ `go build -o open-tv ./cmd/open-tv`, restaurar `internal/ui/dist/.gitkeep` si falta, matar `open-tv serve` (launchd respawnea) y verificar en Chrome con cache-buster (`?fresh=xyz`) que: el escenario aparece, el cambio de canal es en el sitio, ver-todo va y vuelve, ⌘K funciona. OJO a la trampa del bundle cacheado (`curl -s :8080/ | grep index-`).

- [ ] **Step 9: Commit**

```bash
git add web/src/App.svelte web/src/App.integracion.test.ts web/src/componentes/Reproductor.svelte web/src/componentes/Reproductor.test.ts web/src/lib/a11y-p06.test.ts web/src/lib/a11y-p08.test.ts web/src/estilos/tokens/spacing.css web/src/i18n/es.ts web/src/i18n/en.ts
git commit -m "feat(escenario): layout reproductor-primero — panel persistente + catálogo lateral + modo ver-todo"
```

---

### Task 6: Entrada — auto-reanudación muted del último canal (spec §5)

**Files:**
- Modify: `web/src/App.svelte`
- Modify: `web/src/App.integracion.test.ts`

**Interfaces:**
- Consumes: `canalDesdeHistorial` y `canalActual` (Tarea 5), store `historial` (`get(historial)[0]` — `EntradaHistorial` más reciente), prop `silenciadoInicial` del Reproductor (Tarea 4).
- Produces: estado `entradaSilenciada: boolean` que App pasa como `silenciadoInicial` al Reproductor.

- [ ] **Step 1: Tests (fallan)**

Añadir a `App.integracion.test.ts` (en el describe del escenario):

```ts
it('con historial, el arranque auto-reproduce el último canal EN SILENCIO con la CTA de sonido visible', async () => {
  historial.borrar()
  historial.registrar(canalDePrueba('visto-ayer'))
  render(App, { fuente: fuenteConCanales() })
  await vi.waitFor(() => expect(screen.getByRole('region', { name: /visto-ayer/ })).toBeTruthy())
  const video = document.querySelector('video') as HTMLVideoElement
  expect(video.muted).toBe(true)
  expect(screen.getByRole('button', { name: t('reproductor.activarSonido') })).toBeTruthy()
})

it('la auto-reanudación NO re-registra en el historial (sigue habiendo una sola entrada)', async () => {
  historial.borrar()
  historial.registrar(canalDePrueba('visto-ayer'))
  render(App, { fuente: fuenteConCanales() })
  await vi.waitFor(() => expect(screen.getByRole('region', { name: /visto-ayer/ })).toBeTruthy())
  expect(get(historial)).toHaveLength(1)
})

it('con fuentes pero SIN historial no se auto-reproduce nada: tarjeta «elige un canal»', async () => {
  historial.borrar()
  render(App, { fuente: fuenteConCanales() })
  await vi.waitFor(() => expect(screen.getByText(t('escenario.eligeCanal'))).toBeTruthy())
  expect(document.querySelector('video')).toBeNull()
})
```

(`historial` es el singleton exportado de `web/src/estado/historial.ts`; impórtalo. Añadir `historial.borrar()` también al `beforeEach`/`afterEach` general del fichero para que ningún test viejo herede historial.)

- [ ] **Step 2: Verificar que fallan**

Run: `cd web && npx vitest run src/App.integracion.test.ts -t 'auto-reproduce'`
Expected: FAIL — no existe la auto-reanudación.

- [ ] **Step 3: Implementar en `App.svelte`**

```ts
// Entrada del escenario (spec §5): con historial, se entra VIENDO — el último
// canal, EN SILENCIO (los navegadores bloquean autoplay con sonido; la CTA
// del Reproductor ofrece activarlo). Corre UNA sola vez, cuando el catálogo
// confirma que hay fuentes; sin historial no se auto-reproduce nada (la
// tarjeta «elige un canal» manda). NO pasa por abrirCanal: reanudar no debe
// re-registrar la entrada que ya es la más reciente. Sí anuncia por la región
// polite (honestidad §9: quien escucha debe saber que hay vídeo en marcha).
let entradaSilenciada = $state(false)
let entradaResuelta = false
$effect(() => {
  if (entradaResuelta) return
  if (fase.tipo !== 'listo' || !fuentesCargadas) return
  entradaResuelta = true
  if (fuentes.length === 0 || canalActual) return
  const ultimo = get(historial)[0]
  if (!ultimo) return
  entradaSilenciada = true
  canalActual = canalDesdeHistorial(ultimo)
  anuncioCanal = t('escenario.anuncioReproduciendo', { nombre: ultimo.nombre })
})
```

Y en el montaje del Reproductor: `silenciadoInicial={entradaSilenciada}`.

- [ ] **Step 4: Verificar que pasan** — `cd web && npx vitest run src/App.integracion.test.ts`. Expected: PASS completo.

- [ ] **Step 5: Gates completos.**

- [ ] **Step 6: Commit**

```bash
git add web/src/App.svelte web/src/App.integracion.test.ts
git commit -m "feat(escenario): entrada viendo — auto-reanudación muted del último canal con CTA de sonido"
```

---

### Task 7: e2e Playwright — escenario nuevo + adaptación de los specs existentes

**Files:**
- Create: `web/tests/e2e/escenario.spec.ts`
- Modify: `web/tests/e2e/reproduccion.spec.ts`, `web/tests/e2e/catalogo.spec.ts`, `web/tests/e2e/guia.spec.ts`

**Interfaces:**
- Consumes: la infraestructura e2e existente (global-setup arranca el binario real con fixtures en `:8090`; NO se toca), los nombres de canal de `tests/fixtures/catalogo.m3u` («Canal Con CORS», etc.), y el DOM nuevo: filas de la lateral = `.lista-lateral article button.abrir`, botón «Ver todo», clase `.escenario`.
- Produces: cobertura e2e de la spec §10 contra el binario real, 3 motores (webkit conserva sus `test.skip` condicionales existentes).

- [ ] **Step 1: Adaptar el helper `abrir` de `reproduccion.spec.ts`**

El flujo de apertura ya no pasa por la rejilla: el arranque en limpio (sin localStorage) muestra el escenario con la lateral. Sustituir:

```ts
async function abrir(page: import('@playwright/test').Page, nombre: string) {
  await page.goto('/')
  await page.locator('.lista-lateral article', { hasText: nombre }).locator('button.abrir').click()
}
```

El resto del spec (frames avanzan, guard de corte) queda igual — el `<video>` sigue siendo `page.locator('video')`.

- [ ] **Step 2: Adaptar `catalogo.spec.ts` y `guia.spec.ts`**

Los cuatro tests de catálogo y el de guía ejercitan la REJILLA/tarjetas → tras `page.goto('/')`, añadir:

```ts
await page.getByRole('button', { name: 'Ver todo' }).click()
```

(el texto es la clave ES `escenario.verTodo`; el locale del config ya fija es-ES). El test del idioma («English») no necesita ver-todo — el toggle vive en la cabecera, visible en el escenario.

- [ ] **Step 3: Escribir `escenario.spec.ts`**

```ts
import { expect, test } from '@playwright/test'

test.skip(
  ({ browserName }) => browserName === 'webkit' && !!process.env.CI,
  'WebKit de Linux no garantiza H.264; el gate de Safari es manual',
)

test('arranque en limpio: tarjeta «elige un canal», lateral con canales, sin modal', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Elige un canal para empezar')).toBeVisible()
  await expect(page.locator('.lista-lateral article').first()).toBeVisible()
  await expect(page.locator('[role="dialog"]')).toHaveCount(0)
})

test('cambiar de canal en la lateral intercambia el vídeo EN EL SITIO (mismo elemento)', async ({ page }) => {
  await page.goto('/')
  await page.locator('.lista-lateral article', { hasText: 'Canal Con CORS' }).locator('button.abrir').click()
  await expect(page.locator('video')).toBeVisible()
  // Marca el nodo: si el swap desmontara/remontara, la marca se perdería.
  await page.evaluate(() => {
    const v = document.querySelector('video') as HTMLVideoElement & { __marca?: string }
    v.__marca = 'persistente'
  })
  await page.locator('.lista-lateral article', { hasText: 'Canal Sin CORS' }).locator('button.abrir').click()
  const marca = await page.evaluate(
    () => (document.querySelector('video') as HTMLVideoElement & { __marca?: string }).__marca,
  )
  expect(marca).toBe('persistente')
})

test('ver-todo abre la rejilla sin desmontar el vídeo, y seleccionar vuelve reproduciendo', async ({ page }) => {
  await page.goto('/')
  await page.locator('.lista-lateral article', { hasText: 'Canal Con CORS' }).locator('button.abrir').click()
  await expect(page.locator('video')).toBeVisible()
  await page.getByRole('button', { name: 'Ver todo' }).click()
  await expect(page.locator('.escenario.oculto video')).toHaveCount(1) // montado, oculto
  await page.locator('article', { hasText: 'Canal Sin CORS' }).getByRole('button', { name: 'Canal Sin CORS' }).click()
  await expect(page.locator('.escenario:not(.oculto)')).toBeVisible()
})

test('tras ver un canal, recargar auto-reproduce EN SILENCIO con la CTA de sonido', async ({ page }) => {
  await page.goto('/')
  await page.locator('.lista-lateral article', { hasText: 'Canal Con CORS' }).locator('button.abrir').click()
  await expect(page.locator('video')).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Toca para activar el sonido' })).toBeVisible()
  await expect(page.locator('video')).toHaveJSProperty('muted', true)
  await page.getByRole('button', { name: 'Toca para activar el sonido' }).click()
  await expect(page.locator('video')).toHaveJSProperty('muted', false)
})

test('viewport estrecho: vídeo arriba, catálogo debajo — el vídeo no se pierde', async ({ page }) => {
  await page.setViewportSize({ width: 700, height: 900 })
  await page.goto('/')
  await page.locator('.lista-lateral article', { hasText: 'Canal Con CORS' }).locator('button.abrir').click()
  const video = await page.locator('.panel-video').boundingBox()
  const lateral = await page.locator('.lateral').boundingBox()
  expect(video && lateral && video.y < lateral.y).toBe(true)
})
```

Ajusta los nombres de canal a los REALES de `tests/fixtures/catalogo.m3u` (léelo primero; si solo existe «Canal Con CORS»/«Canal Sin CORS», los de arriba ya valen — verifica).

- [ ] **Step 4: Ejecutar el e2e completo**

Run: `cd web && npx playwright test`
Expected: PASS en chromium y firefox; webkit según sus skips condicionales de siempre. Los specs adaptados (reproduccion/catalogo/guia) también verdes.

- [ ] **Step 5: Gates completos.**

- [ ] **Step 6: Commit**

```bash
git add web/tests/e2e/escenario.spec.ts web/tests/e2e/reproduccion.spec.ts web/tests/e2e/catalogo.spec.ts web/tests/e2e/guia.spec.ts
git commit -m "test(e2e): escenario reproductor-primero — swap in place, ver-todo, autoplay muted, responsive"
```

---

### Task 8: Cierre — gates finales, presupuesto de bundle y entrega al gate manual

**Files:**
- Modify: ninguno esperado (solo verificación; arreglos menores si un gate falla).

- [ ] **Step 1: Gates completos de los tres lenguajes** (Global Constraints) — TODOS desde estado limpio (`git status` sin sorpresas fuera de la rama).

- [ ] **Step 2: Presupuesto de bundle (≤ 80 KB gzip el chunk propio)**

```bash
cd web && npm run build
ls -la ../internal/ui/dist/assets/
for f in ../internal/ui/dist/assets/index-*.js; do echo "$f: $(gzip -c "$f" | wc -c) bytes gzip"; done
```
Expected: el chunk `index-*.js` ≤ 81920 bytes gzip (el chunk de hls.js va aparte y NO cuenta). Si se pasa, buscar imports que hayan dejado de ser perezosos antes de tocar nada más.

- [ ] **Step 3: `mobile/` cero diffs**

Run: `git status --porcelain mobile/`
Expected: salida vacía. Y `cd mobile && flutter analyze && flutter test` verdes.

- [ ] **Step 4: Binario + gateway + gate visual**

Desde la raíz: `go build -o open-tv ./cmd/open-tv`, restaurar `internal/ui/dist/.gitkeep` si el build web lo borró, matar `open-tv serve` (launchd respawnea) y verificar `curl -s :8080/ | grep index-` sirve el hash nuevo. Recorrido en Chrome con `?fresh=xyz`: entrada con historial (muted+CTA), swap de canal, ver-todo ida/vuelta, ⌘K, #ajustes/#fuentes, viewport estrecho.

- [ ] **Step 5: Entrega**

Reportar al controlador de la sesión (NO marcar como hecho lo manual):
- **Gate VoiceOver + Safari/Firefox del autor: OBLIGATORIO y PENDIENTE** (spec §9: cambia el foco de toda la app). Recorrido sugerido: orden de tabulación vídeo→lateral, roving de la lista, ver-todo ida/vuelta con foco anunciado, CTA de sonido, ⌘K trap.
- El merge a `main` y cualquier push son decisión del usuario (invariante del repo).
- Review final de rama en el modelo más capaz (flujo SDD) antes del merge.

---

## Autorevisión del plan (hecha al escribirlo)

- **Cobertura de la spec:** §2 estados (T5/T6, Onboarding intacto) · §3 escenario+lateral (T1–T3, T5) · §4 refactor Reproductor (T4/T5) · §5 entrada (T4 CTA + T6) · §6 foco/inert/⌘K (T5) · §7 ver-todo (T5) · §8 responsive (T5 CSS + T7 e2e) · §9 restricciones (Global Constraints) · §10 pruebas (tests por tarea + T7) · §11 no-objetivos respetados (cero backend, cero DVR).
- **Tipos consistentes:** `canalDesdeHistorial(entrada: EntradaHistorial): Canal` (T5) es lo que consume T6; props de `CatalogoLateral` (T3) = lo que monta T5; props panel del Reproductor (T4) = lo que monta T5 tras borrar `modo`/`alCerrar`.
- **Riesgo conocido:** T5 es la tarea grande inevitable (la reestructuración es atómica); T1–T4 le quitan todo lo extraíble. El andamiaje dual de T4 existe solo para que T4 y T5 terminen verdes por separado.
