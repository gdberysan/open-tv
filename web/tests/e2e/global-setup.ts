import { execFileSync, spawn, type ChildProcess } from 'node:child_process'
import { existsSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import type { Server } from 'node:http'
import { servidorFixtures } from './servidor-fixtures'

// web/package.json tiene "type": "module": Playwright ejecuta este fichero
// como ESM y __dirname no existe ahí, así que hay que reconstruirlo desde
// import.meta.url.
const AQUI = dirname(fileURLToPath(import.meta.url))
const RAIZ = resolve(AQUI, '../../..') // raíz del repo
const FIXTURES = resolve(AQUI, '../fixtures')
const DIST_UI = resolve(RAIZ, 'internal/ui/dist')
const PUERTO_FIXTURES = 8099
export const PUERTO_APP = 8090

let servidor: Server | undefined
let binario: ChildProcess | undefined

/** Genera un HLS real de 10 s con ffmpeg. H.264 + AAC: lo que decodifica
 *  cualquier navegador. Se cachea entre ejecuciones. */
function generarHLS(): void {
  const salida = join(FIXTURES, 'hls')
  if (existsSync(join(salida, 'canal.m3u8'))) return
  mkdirSync(salida, { recursive: true })

  execFileSync('ffmpeg', [
    '-y',
    '-f', 'lavfi', '-i', 'testsrc=size=640x360:rate=25:duration=10',
    '-f', 'lavfi', '-i', 'sine=frequency=440:duration=10',
    '-c:v', 'libx264', '-profile:v', 'main', '-pix_fmt', 'yuv420p', '-g', '50',
    '-c:a', 'aac', '-b:a', '64k',
    '-f', 'hls', '-hls_time', '2', '-hls_list_size', '0', '-hls_playlist_type', 'vod',
    join(salida, 'canal.m3u8'),
  ], { stdio: 'inherit' })
}

function escribirM3U(): void {
  const base = `http://127.0.0.1:${PUERTO_FIXTURES}`
  writeFileSync(join(FIXTURES, 'catalogo.m3u'), [
    // url-tvg en la cabecera (Tarea 10, e2e de la guía): apunta al XMLTV
    // dinámico que servidor-fixtures.ts genera en /epg.xml. Es lo que
    // ejercita el camino real de punta a punta — captura en el parser M3U
    // (T1) → Syncer (T5) → join (T3) → endpoint (T6) → tarjeta (T7/T8) —
    // sin tocar nada del catálogo que ya usan catalogo.spec.ts/
    // reproduccion.spec.ts: la guía solo declara <channel id="ConCors.xx">
    // (ver CHANNEL_ID_CON_GUIA en servidor-fixtures.ts), así que "Canal Con
    // CORS" adquiere un ahora/después y "Canal Sin CORS" queda sin guía, tal
    // cual pide el brief de la Tarea 10.
    `#EXTM3U url-tvg="${base}/cors/epg.xml"`,
    `#EXTINF:-1 tvg-id="ConCors.xx" tvg-logo="" group-title="News",Canal Con CORS (1080p)`,
    `${base}/cors/hls/canal.m3u8`,
    `#EXTINF:-1 tvg-id="SinCors.xx" tvg-logo="" group-title="Movies",Canal Sin CORS (720p)`,
    `${base}/sincors/hls/canal.m3u8`,
    '',
  ].join('\n'))
}

async function esperarSync(url: string): Promise<void> {
  for (let i = 0; i < 120; i++) {
    try {
      const r = await fetch(`${url}/health`)
      const c = (await r.json()) as { last_sync?: string | null }
      if (c.last_sync) return
    } catch {
      // Todavía no escucha.
    }
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error('open-tv no completó el primer sync en 60 s')
}

export default async function globalSetup(): Promise<() => Promise<void>> {
  generarHLS()
  escribirM3U()

  // El cliente y el binario se construyen aquí: el e2e prueba el ARTEFACTO,
  // no el servidor de desarrollo de Vite.
  execFileSync('npm', ['run', 'build'], { cwd: join(RAIZ, 'web'), stdio: 'inherit' })
  // `vite build` limpia internal/ui/dist (emptyOutDir) y con él se lleva el
  // .gitkeep versionado. go:embed no lo necesita para compilar —le basta con
  // que el directorio no esté vacío, y el build ya deja index.html— pero sin
  // restaurarlo `git status` queda con un .gitkeep "borrado" después de cada
  // corrida del e2e.
  writeFileSync(join(DIST_UI, '.gitkeep'), '')
  execFileSync('go', ['build', '-o', 'open-tv', './cmd/open-tv'], { cwd: RAIZ, stdio: 'inherit' })

  servidor = await servidorFixtures(FIXTURES, PUERTO_FIXTURES)

  const datos = mkdtempSync(join(tmpdir(), 'opentv-e2e-'))
  binario = spawn(join(RAIZ, 'open-tv'), ['serve', '--no-browser'], {
    cwd: RAIZ,
    env: {
      ...process.env,
      DB_PATH: join(datos, 'e2e.db'),
      LISTEN_ADDR: `127.0.0.1:${PUERTO_APP}`,
      IPTV_ORG_URL: `http://127.0.0.1:${PUERTO_FIXTURES}/cors/catalogo.m3u`,
      HEALTH_INTERVAL: '10s',
      // Las fixtures HLS de este e2e viven en 127.0.0.1: sin esto, el proxy
      // las rechazaría con 403 por protección SSRF (destino privado), igual
      // que rechazaría cualquier loopback en producción. Solo este script
      // fija esta variable — el default de main.go es bloquear, que es lo
      // que corre en cualquier instalación real.
      OPEN_TV_PERMITIR_DESTINOS_PRIVADOS: '1',
    },
    stdio: 'inherit',
  })

  await esperarSync(`http://127.0.0.1:${PUERTO_APP}`)
  // Un margen para que la primera pasada de salud clasifique web_ok: sin ella
  // los dos canales salen con veredicto null y el caso del proxy no se prueba.
  // El health-worker arranca en cuanto el sync termina y su primera pasada es
  // inmediata (Worker.Start hace un checkOnce antes del ticker), así que con
  // solo dos streams locales este margen sobra de sobra.
  await new Promise((r) => setTimeout(r, 3000))

  // Playwright invoca como teardown lo que este default export DEVUELVE: es
  // el mecanismo estándar, y evita depender de un globalTeardown que apunte
  // al mismo fichero (eso re-ejecutaría este setup en vez de cerrar nada).
  return async () => {
    binario?.kill('SIGTERM')
    servidor?.close()
  }
}
