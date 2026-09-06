# Verificación honesta de la reproducción — Plan de implementación

> **Para trabajadores agénticos:** SUB-SKILL OBLIGATORIA: usa
> `superpowers:subagent-driven-development` (recomendada) o
> `superpowers:executing-plans` para implementar tarea a tarea. Los pasos usan
> casillas (`- [ ]`).

**Goal:** Que CI cace por sí solo los fallos de reproducción que ayer solo
encontró un humano mirando la app.

**Architecture:** Dos piezas con contratos distintos. (A) El e2e de Playwright
que YA existe gana **orígenes hostiles** que reproducen byte a byte los fallos
medidos — determinista, en cada commit, es un gate. (B) Un **censo nocturno**
contra el catálogo real que mide y NUNCA tumba el build.

**Tech Stack:** Playwright 1.62 (ya instalado, 3 navegadores en CI), Node
`http` para el servidor de fixtures, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-05-verificacion-honesta-de-reproduccion-design.md`

## Global Constraints

- Código, comentarios y mensajes de commit en **ESPAÑOL**.
- **Ninguna dependencia nueva** (Go ni npm). Playwright y Node bastan.
- **No se toca código de producción.** Este plan solo añade
  `web/tests/e2e/**` y un workflow. Si aparece un bug, se arregla en OTRA rama.
- **Nada bajo `mobile/`.**
- **NUNCA `git add -A`.** Añade por ruta explícita.
- Commit trailer con **el modelo que hace el trabajo** (CLAUDE.md: «o el modelo
  en uso»).
- Gates verdes: `cd web && npm run check && npm test && npm run build`, y
  `npm run test:e2e` para lo que toca este plan.
- **Criterio de aceptación de TODA prueba nueva:** se valida **revirtiendo el
  arreglo que dice cubrir** y viéndola ponerse ROJA. Una prueba que no se ha
  visto fallar no cuenta. Cada tarea dice qué revertir.

---

> **Nota (2026-09-05, tras el spec de salud por segmento):** cuando este
> plan se ejecute, la Tarea 1 debe añadir un origen hostil más,
> `/hostil/mpeg2` — segmentos TS con vídeo MPEG-2 (stream type 0x02) y
> audio MP2 — y una aserción: el reproductor muestra el mensaje de códec
> (`reproductor.error.codecGenerico`), no el genérico ni el de Safari. Ver
> `docs/superpowers/specs/2026-09-05-salud-por-segmento-design.md` §4.3.

## No-objetivos explícitos

- **Causas mixtas entre mirrors en e2e.** El spec (§3.1, aserción 3) lo pedía,
  pero en este arnés no se puede montar honestamente: los fixtures viven en
  loopback sin HTTPS, así que `ClassifyWeb` les da siempre `web_ok=false` y el
  plan de reproducción queda en **un solo intento por el proxy** — no hay
  cadena de dos causas que provocar sin inventarse un certificado. La propiedad
  ya está cubierta por las pruebas unitarias de `claseConsensuada`
  (`diagnostico.test.ts`), que es donde vive la lógica. Se deja fuera a
  propósito y por escrito, en vez de montar un test que finja probarlo.
- **Un no-fatal antes de arrancar no mata el intento** (§3.1, aserción 6): ya
  lo cubre `Reproductor.test.ts` a nivel de componente, con el doble de hls.js
  emitiendo `fallarNoFatal`. Repetirlo en e2e no añade señal.

---

## Estructura de ficheros

**Modificar:**
- `web/tests/e2e/servidor-fixtures.ts` — orígenes hostiles.
- `web/tests/e2e/global-setup.ts` — declarar los canales hostiles en el M3U.

**Crear:**
- `web/tests/e2e/hostiles.spec.ts` — los tests de diagnóstico honesto.
- `web/tests/censo/censo.spec.ts` — el censo nocturno (proyecto aparte).
- `.github/workflows/nightly.yml` — el job programado.

---

### Task 1: Orígenes hostiles en el servidor de fixtures

**Files:**
- Modify: `web/tests/e2e/servidor-fixtures.ts`
- Modify: `web/tests/e2e/global-setup.ts:40-58` (`escribirM3U`)

**Interfaces:**
- Produces: rutas `/hostil/<modo>/canal.m3u8` y `/hostil/<modo>/canalN.ts` con
  `<modo>` ∈ `relleno | lento | prohibido | rota`, y cuatro canales nuevos en
  el catálogo de fixtures.

- [ ] **Paso 1: Añadir los orígenes hostiles**

En `servidor-fixtures.ts`, ANTES del bloque que resuelve ficheros de disco:

```ts
/** Un paquete MPEG-TS nulo: 188 bytes que empiezan por el byte de sync 0x47.
 *  Es EXACTAMENTE lo que devuelve el origen de AXN Latin America South para un
 *  segmento ya caducado — con HTTP 200 y Content-Type video/MP2T, no con un
 *  error. Medido el 2026-09-04. hls.js no puede demuxarlo y emite
 *  fragParsingError, que es MEDIA_ERROR: por eso el clasificador lo llamaba
 *  «tu navegador no puede con el formato». */
function paqueteTSNulo(): Buffer {
  const b = Buffer.alloc(188, 0xff)
  b[0] = 0x47
  return b
}

/** Manifiesto HLS de ventana fija que apunta a los segmentos del propio modo
 *  hostil. VOD (no live) para que hls.js no ande recargando la playlist
 *  mientras el test asevera. */
function manifiestoHostil(modo: string): string {
  const seg = (n: number) => `#EXTINF:2.0,\n/hostil/${modo}/canal${n}.ts`
  return ['#EXTM3U', '#EXT-X-VERSION:3', '#EXT-X-TARGETDURATION:2', '#EXT-X-MEDIA-SEQUENCE:0',
    seg(0), seg(1), seg(2), '#EXT-X-ENDLIST', ''].join('\n')
}

/** Cuántas veces se ha pedido cada segmento, por modo. Lo usa 'rota': los dos
 *  primeros segmentos son buenos y a partir de ahí sirve relleno, que es la
 *  forma real en que un cliente se descuelga de la ventana en vivo. */
const pedidos = new Map<string, number>()
```

Y dentro del `createServer`, como PRIMERA rama del router (antes de `/cors/`):

```ts
    // Orígenes que se portan MAL a propósito. Cada uno reproduce un fallo real
    // medido contra un origen de verdad, no uno inventado: sin ellos, el e2e
    // solo sabe probar el camino feliz y un 404.
    if (url.pathname.startsWith('/hostil/')) {
      const [, , modo, recurso] = url.pathname.split('/')
      res.setHeader('Access-Control-Allow-Origin', '*')

      if (modo === 'prohibido') {
        res.writeHead(403).end('prohibido')
        return
      }
      if (recurso === 'canal.m3u8') {
        res.setHeader('Content-Type', TIPOS['.m3u8'])
        res.end(manifiestoHostil(modo))
        return
      }

      const clave = `${modo}/${recurso}`
      const n = (pedidos.get(clave) ?? 0) + 1
      pedidos.set(clave, n)

      const responderRelleno = () => {
        res.setHeader('Content-Type', TIPOS['.ts'])
        res.end(paqueteTSNulo())
      }

      if (modo === 'relleno') return responderRelleno()
      if (modo === 'rota') {
        // Los dos primeros segmentos son de verdad; luego, relleno.
        if (recurso === 'canal0.ts' || recurso === 'canal1.ts') {
          res.setHeader('Content-Type', TIPOS['.ts'])
          createReadStream(join(raiz, 'hls', recurso)).pipe(res)
          return
        }
        return responderRelleno()
      }
      if (modo === 'lento') {
        // MUY por encima del presupuesto sin progreso del guard (7 s): el
        // manifiesto contesta al instante —así el health-check lo llama vivo—
        // y el segmento no llega nunca a tiempo. Es el caso de AMC (720p):
        // segmentos de 4 s servidos en más de 12 s.
        setTimeout(() => {
          res.setHeader('Content-Type', TIPOS['.ts'])
          createReadStream(join(raiz, 'hls', 'canal0.ts')).pipe(res)
        }, 25_000)
        return
      }
      res.writeHead(404).end('modo desconocido')
      return
    }
```

- [ ] **Paso 2: Declarar los canales hostiles en el catálogo**

En `global-setup.ts`, dentro de `escribirM3U`, **APPEND** al final del array (antes
del `''`) — nunca al principio: el primer canal del M3U es el que la app abre
sola al arrancar, y moverlo cambiaría lo que ven los specs que ya existen.

```ts
    // Canales hostiles (Tarea 1). Van AL FINAL a propósito: el primero del M3U
    // es el que el escenario abre solo al arrancar, y las specs que ya existen
    // cuentan con que ese sea "Canal Con CORS".
    `#EXTINF:-1 tvg-id="Relleno.xx" tvg-logo="" group-title="News",Canal Relleno`,
    `${base}/hostil/relleno/canal.m3u8`,
    `#EXTINF:-1 tvg-id="Lento.xx" tvg-logo="" group-title="News",Canal Lento`,
    `${base}/hostil/lento/canal.m3u8`,
    `#EXTINF:-1 tvg-id="Prohibido.xx" tvg-logo="" group-title="News",Canal Prohibido`,
    `${base}/hostil/prohibido/canal.m3u8`,
    `#EXTINF:-1 tvg-id="Rota.xx" tvg-logo="" group-title="News",Canal Rota`,
    `${base}/hostil/rota/canal.m3u8`,
```

- [ ] **Paso 3: Comprobar que los orígenes responden lo prometido**

Run:
```bash
cd web && npx playwright test --project=chromium tests/e2e/catalogo.spec.ts
```
Expected: PASS — las specs existentes siguen verdes con el catálogo ampliado.

Y a mano, con el servidor de fixtures levantado por el global-setup:
```bash
curl -s -o /dev/null -w '%{http_code} %{size_download}\n' http://127.0.0.1:8099/hostil/relleno/canal0.ts
```
Expected: `200 188`.

- [ ] **Paso 4: Commit**

```bash
git add web/tests/e2e/servidor-fixtures.ts web/tests/e2e/global-setup.ts
git commit -m "test(e2e): orígenes hostiles que reproducen fallos reales

El servidor de fixtures solo sabía portarse bien o devolver 404, así que
el e2e nunca podía ver los fallos que de verdad rompen la app. Ahora
reproduce, byte a byte, los medidos el 2026-09-04: 188 bytes de relleno
con HTTP 200 (AXN), segmentos imposiblemente lentos (AMC), 403, y un
stream que arranca bien y se rompe a mitad.

Co-Authored-By: Claude <modelo> <noreply@anthropic.com>"
```

---

### Task 2: Un atasco no se anuncia como problema de formato

**Files:**
- Create: `web/tests/e2e/hostiles.spec.ts`

**Interfaces:**
- Consumes de Task 1: los canales `Canal Relleno` y `Canal Prohibido`.

- [ ] **Paso 1: Escribir el test**

```ts
import { expect, test } from '@playwright/test'

// Los bugs que estos tests fijan son todos de Chromium y de la capa de medios;
// correrlos en tres navegadores triplica el tiempo sin añadir cobertura, y el
// WebKit de Linux ni siquiera garantiza H.264 (ver reproduccion.spec.ts).
test.describe.configure({ mode: 'serial' })
test.skip(({ browserName }) => browserName !== 'chromium', 'diagnóstico: solo chromium')

async function abrir(page: import('@playwright/test').Page, nombre: string) {
  await page.goto('/')
  await page.locator('.lista-lateral article', { hasText: nombre }).locator('button.abrir').click()
}

// Bug real del 2026-09-04: el origen de AXN devuelve 188 bytes (un paquete TS
// nulo) con HTTP 200 para segmentos caducados. hls.js emite fragParsingError,
// que es MEDIA_ERROR, y el clasificador mandaba a TODO mediaError a 'formato'.
// El usuario leía «tu navegador no puede reproducir este formato, prueba en
// Safari» — falso, y un consejo inútil: Safari recibiría los mismos 188 bytes.
test('relleno del origen NO se anuncia como problema de formato', async ({ page }) => {
  await abrir(page, 'Canal Relleno')

  const error = page.locator('.estado.error')
  await expect(error).toBeVisible({ timeout: 40_000 })

  // Lo que el usuario LEE es la aserción; el estado interno no.
  await expect(error).toContainText('la señal llega rota')
  await expect(error).not.toContainText('formato')
  await expect(error).not.toContainText('Safari')
})

// Un 403 sí tiene una causa concreta que afirmar.
test('un 403 se anuncia como geo-bloqueo, no como algo genérico', async ({ page }) => {
  await abrir(page, 'Canal Prohibido')

  const error = page.locator('.estado.error')
  await expect(error).toBeVisible({ timeout: 40_000 })
  await expect(error).not.toContainText('formato')
})

// Bug real del 2026-09-04 con AMC (720p): su manifiesto contesta al instante
// —por eso el health-check lo llama vivo— pero sirve segmentos de 4 s en más
// de 12 s. Se anunciaba como «la dirección del canal caducó», que era la clase
// del ÚLTIMO intento y no tenía nada que ver con lo que pasaba.
test('un origen imposiblemente lento no se anuncia como caducado', async ({ page }) => {
  await abrir(page, 'Canal Lento')

  const error = page.locator('.estado.error')
  await expect(error).toBeVisible({ timeout: 40_000 })
  await expect(error).not.toContainText('caduc')
})
```

- [ ] **Paso 2: Ejecutar y ver que pasa**

Run: `cd web && npx playwright test --project=chromium tests/e2e/hostiles.spec.ts`
Expected: PASS (el arreglo ya está en `main`).

- [ ] **Paso 3: VALIDAR LA PRUEBA revirtiendo el arreglo**

Esto es obligatorio. En `web/src/reproductor/diagnostico.ts`, sustituir
temporalmente la guarda de códecs por la regla vieja:

```ts
  if (tipo.includes('mediaerror')) return 'formato'
```
(colocada ANTES del bloque `CODEC_INCOMPATIBLE`).

Run: `cd web && npx playwright test --project=chromium tests/e2e/hostiles.spec.ts`
Expected: **FAIL** en «relleno del origen NO se anuncia como problema de
formato», con el texto «formato»/«Safari» presente.

**Revertir el cambio** (`git checkout web/src/reproductor/diagnostico.ts`) y
volver a ejecutar: PASS. Pegar ambas salidas en el informe.

- [ ] **Paso 4: Commit**

```bash
git add web/tests/e2e/hostiles.spec.ts
git commit -m "test(e2e): un atasco no puede anunciarse como fallo de formato

Validado revirtiendo el clasificador: con la regla vieja (todo mediaError
-> formato) el test se pone rojo, que es la prueba de que cubre el bug.

Co-Authored-By: Claude <modelo> <noreply@anthropic.com>"
```

---

### Task 3: La tarjeta de error es utilizable

**Files:**
- Modify: `web/tests/e2e/hostiles.spec.ts`

**Interfaces:**
- Consumes de Task 1: `Canal Prohibido` (falla rápido y determinista).

- [ ] **Paso 1: Escribir el test**

Añadir a `hostiles.spec.ts`:

```ts
// Bug real del 2026-09-04: el botón «Reintentar» era VISIBLE pero inclicable.
// `.overlay` es un hermano POSTERIOR en el DOM, absoluto con inset:0 y
// pointer-events auto, así que se pintaba encima y se comía el clic — y como
// opacity:0 NO desactiva el hit-testing, se lo comía también estando oculto.
// El clic tiene que ser REAL (Playwright hace hit-testing de verdad): un
// dispatchEvent sintético habría pasado por encima del bug sin verlo.
test('el botón de la tarjeta de error se puede pulsar de verdad', async ({ page }) => {
  const intentos: string[] = []
  page.on('request', (r) => {
    if (r.url().includes('/proxy/hls') || r.url().includes('/hostil/')) intentos.push(r.url())
  })

  await abrir(page, 'Canal Prohibido')
  const boton = page.getByRole('button', { name: 'Reintentar' })
  await expect(boton).toBeVisible({ timeout: 40_000 })

  const antes = intentos.length
  await boton.click() // clic real: falla si algo lo tapa
  await expect.poll(() => intentos.length, { timeout: 15_000 }).toBeGreaterThan(antes)
})
```

- [ ] **Paso 2: Ejecutar y ver que pasa**

Run: `cd web && npx playwright test --project=chromium tests/e2e/hostiles.spec.ts -g "pulsar de verdad"`
Expected: PASS.

- [ ] **Paso 3: VALIDAR revirtiendo el arreglo**

En `web/src/componentes/Reproductor.svelte`, quitar `z-index: 2;` de la regla
`.estado`.

Run: el mismo comando.
Expected: **FAIL** — Playwright aborta el clic con un mensaje del tipo
`<div class="overlay"> intercepts pointer events`.

Revertir y volver a ejecutar: PASS. Pegar ambas salidas en el informe.

- [ ] **Paso 4: Commit**

```bash
git add web/tests/e2e/hostiles.spec.ts
git commit -m "test(e2e): la tarjeta de error tiene que ser pulsable

Validado quitando el z-index: Playwright falla con «overlay intercepts
pointer events», exactamente el bug reportado. El clic es real a
propósito: un dispatchEvent sintético no lo habría visto.

Co-Authored-By: Claude <modelo> <noreply@anthropic.com>"
```

---

### Task 4: Con la pestaña oculta no se inventa un error

**Files:**
- Modify: `web/tests/e2e/hostiles.spec.ts`

- [ ] **Paso 1: Escribir el test**

```ts
// Bug real del 2026-09-04: Chrome NO abre un MediaSource en una pestaña
// oculta. hls.js baja manifiesto y playlist —el canal está VIVO— pero no pide
// un segmento y readyState no pasa de 0. La app gastaba ahí su presupuesto de
// carga y anunciaba «no llegó a reproducir» sobre un canal sano. Como la app
// reanuda «continuar viendo» al arrancar, abrirla en segundo plano daba ese
// error SIEMPRE.
//
// Solo chromium: la visibilidad se fuerza por CDP y es específica de Chrome.
test('con la pestaña oculta no se declara caído un canal sano', async ({ page, context }) => {
  const cdp = await context.newCDPSession(page)
  await cdp.send('Emulation.setPageVisibilityOverride', { visibility: 'hidden' })

  await abrir(page, 'Canal Con CORS')

  // Tiempo de sobra para que el presupuesto de 7 s habría expirado.
  await page.waitForTimeout(15_000)
  await expect(page.locator('.estado.error')).toHaveCount(0)
  await expect(page.locator('.estado')).toContainText('Conectando')

  // Al volver a verse, arranca.
  await cdp.send('Emulation.setPageVisibilityOverride', { visibility: 'visible' })
  await expect
    .poll(async () => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime), {
      timeout: 30_000,
    })
    .toBeGreaterThan(0.5)
})
```

Si `Emulation.setPageVisibilityOverride` no está disponible en la versión de
Chromium de Playwright, marcar el test con `test.fixme()` y **decirlo en el
informe** — no borrarlo ni fingir que pasa.

- [ ] **Paso 2: Ejecutar**

Run: `cd web && npx playwright test --project=chromium tests/e2e/hostiles.spec.ts -g "pestaña oculta"`
Expected: PASS.

- [ ] **Paso 3: VALIDAR revirtiendo el arreglo**

En `web/src/reproductor/guard.ts`, hacer que `pausar()` sea un no-op:
```ts
  pausar(): void { return }
```

Run: el mismo comando.
Expected: **FAIL** — aparece `.estado.error`.

Revertir y volver a ejecutar: PASS. Pegar ambas salidas.

- [ ] **Paso 4: Añadir el test del stream que se rompe a mitad**

```ts
// Bug real del 2026-09-04 con AXN: el canal arrancaba, iba a tirones y moría
// en «El canal dejó de emitir». El watchdog de atasco miraba SOLO la posición,
// así que mataba a hls.js en plena recuperación. Y el mensaje mentía: el canal
// seguía emitiendo; nos habíamos caído de su ventana.
test('un stream que se rompe a mitad no se anuncia como que dejó de emitir', async ({ page }) => {
  await abrir(page, 'Canal Rota')

  // Primero reproduce de verdad (los dos primeros segmentos son buenos).
  await expect
    .poll(async () => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime), {
      timeout: 30_000,
    })
    .toBeGreaterThan(0.5)

  // Y cuando se rompe, lo que se dice es honesto.
  const error = page.locator('.estado.error')
  await expect(error).toBeVisible({ timeout: 60_000 })
  await expect(error).not.toContainText('dejó de emitir')
})
```

**Validar revirtiendo:** en `web/src/i18n/es.ts`, devolver
`'reproductor.error.corte'` a `'El canal dejó de emitir.'`. El test tiene que
ponerse ROJO. Revertir y volver a verde. Pegar ambas salidas.

- [ ] **Paso 5: Commit** (mismo patrón que las anteriores)

---

### Task 5: Censo nocturno contra el catálogo real

**Files:**
- Create: `web/tests/censo/censo.spec.ts`
- Create: `.github/workflows/nightly.yml`
- Modify: `web/playwright.config.ts` (excluir `tests/censo` del `testDir` normal)

- [ ] **Paso 1: Excluir el censo del e2e normal**

En `playwright.config.ts`, añadir a `defineConfig`:
```ts
  // El censo vive fuera del gate: usa red de terceros y no puede tumbar el
  // build (ver §3.2 del spec). Se ejecuta con su propia config en el nightly.
  testIgnore: '**/censo/**',
```

- [ ] **Paso 2: Escribir el censo**

`web/tests/censo/censo.spec.ts`:

```ts
import { expect, test } from '@playwright/test'
import { writeFileSync } from 'node:fs'

// NO es un gate. Mide contra el catálogo REAL, cuyos orígenes cambian solos:
// convertir eso en rojo/verde enseña al equipo a ignorar los rojos. Su salida
// es un artefacto con historial y la señal que importa es la TENDENCIA.
test('censo de reproducibilidad', async ({ page }) => {
  test.setTimeout(30 * 60_000)

  const canales: { ID: string; Name: string }[] = await (
    await fetch('http://127.0.0.1:8090/channels?limit=2000')
  ).json()

  // Muestra ESTABLE (cada 37) para poder comparar entre noches; aleatoria no
  // serviría: la tendencia sería ruido de muestreo.
  const muestra = canales.filter((_, i) => i % 37 === 0).slice(0, 50)

  const filas: { canal: string; reproduce: boolean; ms: number | null }[] = []
  for (const c of muestra) {
    await page.goto('/')
    const inicio = Date.now()
    let ms: number | null = null
    try {
      await page.locator('.lista-lateral article', { hasText: c.Name }).locator('button.abrir').click({ timeout: 5_000 })
      await expect
        .poll(async () => page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000 })
        .toBeGreaterThan(0.5)
      ms = Date.now() - inicio
    } catch {
      ms = null
    }
    filas.push({ canal: c.Name, reproduce: ms !== null, ms })
  }

  const reproducen = filas.filter((f) => f.reproduce)
  const tiempos = reproducen.map((f) => f.ms!).sort((a, b) => a - b)
  const pct = (p: number) => tiempos[Math.min(tiempos.length - 1, Math.floor(tiempos.length * p))] ?? null
  const resumen = {
    fecha: new Date().toISOString(),
    probados: filas.length,
    reproducen: reproducen.length,
    tasa: filas.length ? Math.round((reproducen.length / filas.length) * 1000) / 10 : 0,
    p50: pct(0.5),
    p90: pct(0.9),
    filas,
  }
  writeFileSync('censo.json', JSON.stringify(resumen, null, 2))
  console.log(`tasa reproducible: ${resumen.tasa}%  p50=${resumen.p50}ms  p90=${resumen.p90}ms`)

  // Única aserción: que el censo se haya podido ejecutar. La tasa NO es un
  // gate — si cae, lo dice la tendencia del artefacto, no un build rojo.
  expect(filas.length).toBeGreaterThan(0)
})
```

- [ ] **Paso 3: El workflow nocturno**

`.github/workflows/nightly.yml`:

```yaml
name: Nightly

on:
  schedule:
    # 04:15 UTC. No es un gate: mide contra orígenes de terceros.
    - cron: '15 4 * * *'
  workflow_dispatch:

jobs:
  censo:
    name: Censo de reproducibilidad
    runs-on: ubuntu-latest
    # Nunca tumba nada: su valor es la tendencia del artefacto.
    continue-on-error: true
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-node@v7
        with:
          node-version: 24
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: construir cliente y binario
        run: |
          cd web && npm ci && npm run build && cd ..
          go build -o open-tv ./cmd/open-tv

      - name: navegador
        working-directory: web
        run: npx playwright install --with-deps chromium

      - name: arrancar el gateway con el catálogo real
        run: |
          DB_PATH=$RUNNER_TEMP/censo.db LISTEN_ADDR=127.0.0.1:8090 \
./open-tv serve --no-browser &
          for i in $(seq 1 60); do
            curl -sf http://127.0.0.1:8090/channels?limit=1 >/dev/null && break
            sleep 10
          done

      - name: censo
        working-directory: web
        run: npx playwright test --config=playwright.config.ts --project=chromium tests/censo/censo.spec.ts
        env:
          PLAYWRIGHT_TEST_IGNORE: ''

      - uses: actions/upload-artifact@v7
        with:
          name: censo
          path: web/censo.json
          retention-days: 90
```

**Por defecto, crea `web/playwright.censo.config.ts`** con su propio
`testDir: './tests/censo'`, sin `testIgnore`, sin `retries` (un reintento
falsearía la tasa) y `timeout: 30 * 60_000`; y en el workflow usa
`npx playwright test --config=playwright.censo.config.ts --project=chromium`,
quitando la variable `PLAYWRIGHT_TEST_IGNORE` del paso. Si al ejecutarlo algo
no encaja con la versión instalada, ajústalo a lo que de verdad funcione y
dilo en el informe — **no dejes un workflow que no hayas visto ejecutar**.

- [ ] **Paso 4: Ejecutar el censo a mano una vez**

Run:
```bash
./open-tv serve --no-browser &   # desde la raíz del repo
cd web && npx playwright test --config=<la config del censo> --project=chromium
```
Expected: termina, escribe `censo.json` y registra la tasa. **Pegar la tasa
real en el informe** — es el primer número honesto de cuántos canales se ven de
verdad, frente a los 9.304 que el health-check llama vivos.

- [ ] **Paso 5: Commit**

```bash
git add web/tests/censo/ web/playwright.config.ts .github/workflows/nightly.yml
git commit -m "test(censo): medición nocturna de reproducibilidad real

No es un gate y no puede serlo: mide contra orígenes de terceros que
cambian solos, y un rojo que no depende de nosotros enseña a ignorar los
rojos. Su salida es un artefacto con 90 días de historial; la señal es la
tendencia.

Da por fin el número que falta: cuántos canales se VEN, frente a los que
el health-check llama vivos porque su manifiesto contestó.

Co-Authored-By: Claude <modelo> <noreply@anthropic.com>"
```

---

## Verificación final

- [ ] `cd web && npm run check && npm test && npm run build` verde.
- [ ] `cd web && npm run test:e2e` verde en los tres navegadores (los hostiles
      se saltan fuera de chromium).
- [ ] `git status --porcelain mobile/` vacío.
- [ ] **Ninguna línea de `web/src/` ni de `internal/` en el diff de la rama.**
      Este plan no toca producción; si algo se coló, sacarlo.
- [ ] Los tres tests de diagnóstico (Tareas 2-4) tienen en su informe la salida
      ROJA de su reversión y la VERDE de después.
- [ ] El censo se ha ejecutado al menos una vez a mano y su tasa está anotada.
