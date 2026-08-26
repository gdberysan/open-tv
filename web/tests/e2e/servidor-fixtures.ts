import { createReadStream, existsSync, statSync } from 'node:fs'
import { createServer, type Server } from 'node:http'
import { extname, join, normalize } from 'node:path'

const TIPOS: Record<string, string> = {
  '.m3u8': 'application/vnd.apple.mpegurl',
  '.m3u': 'application/vnd.apple.mpegurl',
  '.ts': 'video/mp2t',
}

// ID del <channel> XMLTV que casa con el tvg-id="ConCors.xx" del canal "Con
// CORS" en catalogo.m3u (ver global-setup.ts, escribirM3U). Deliberadamente
// el ÚNICO channel que la guía declara: "Canal Sin CORS" (tvg-id=SinCors.xx)
// queda sin entrada en el XMLTV a propósito, para que la Tarea 10 (e2e de la
// guía) tenga sin más cambios su caso "canal sin guía → tarjeta limpia" con
// las dos fuentes que este servidor ya expone a las demás specs.
const CHANNEL_ID_CON_GUIA = 'ConCors.xx'

/** "AñoMesDíaHoraMinSeg +0000", el formato XMLTV (ver xmltvTimeLayout en
 * internal/adapters/epg/xmltv.go). Siempre en UTC: la +0000 al final es
 * literal, no depende de la zona del proceso que ejecuta el test. */
function formatearXMLTV(fecha: Date): string {
  const p = (n: number) => String(n).padStart(2, '0')
  return (
    `${fecha.getUTCFullYear()}${p(fecha.getUTCMonth() + 1)}${p(fecha.getUTCDate())}` +
    `${p(fecha.getUTCHours())}${p(fecha.getUTCMinutes())}${p(fecha.getUTCSeconds())} +0000`
  )
}

/**
 * XMLTV de fixture generado EN CADA PETICIÓN (nunca un fichero estático):
 * el <programme> tiene que cubrir el instante en que el test hace la
 * aserción, y ese instante es "ahora mismo, cuando Playwright ejecuta",
 * nunca una hora fija de cuando se escribió el fixture — el mismo problema
 * que ffmpeg resuelve para el VOD de HLS generándolo bajo demanda. Ventana
 * ±10 min/+50 min: de sobra para que el margen de arranque del binario real
 * (build, sync, health) del global-setup nunca lo deje fuera de horario.
 */
function xmltvDinamico(): string {
  const ahora = Date.now()
  const inicio = formatearXMLTV(new Date(ahora - 10 * 60_000))
  const fin = formatearXMLTV(new Date(ahora + 50 * 60_000))
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<tv>',
    `  <channel id="${CHANNEL_ID_CON_GUIA}"><display-name>Canal Con CORS</display-name></channel>`,
    `  <programme start="${inicio}" stop="${fin}" channel="${CHANNEL_ID_CON_GUIA}">`,
    '    <title>Noticias en directo</title>',
    '  </programme>',
    '</tv>',
    '',
  ].join('\n')
}

/**
 * Sirve las fixtures en dos rutas deliberadamente distintas:
 *
 * - /cors/…  manda Access-Control-Allow-Origin: * → web_ok = true → el cliente
 *   reproduce DIRECTO.
 * - /sincors/… no manda nada → web_ok = false → el cliente tiene que pasar por
 *   el proxy de loopback.
 *
 * Con un solo caso, la mitad del camino que P0 construye no se probaría nunca.
 *
 * `/cors/epg.xml` es la única ruta servida SIN tocar disco (ver
 * xmltvDinamico arriba): url-tvg de catalogo.m3u apunta ahí (global-setup.ts).
 */
export function servidorFixtures(raiz: string, puerto: number): Promise<Server> {
  const srv = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://localhost')
    const conCors = url.pathname.startsWith('/cors/')
    const relativa = url.pathname.replace(/^\/(cors|sincors)\//, '')

    if (relativa === 'epg.xml') {
      if (conCors) res.setHeader('Access-Control-Allow-Origin', '*')
      res.setHeader('Content-Type', 'application/xml')
      res.end(xmltvDinamico())
      return
    }

    const fichero = join(raiz, normalize(relativa).replace(/^(\.\.[/\\])+/, ''))

    if (!existsSync(fichero) || !statSync(fichero).isFile()) {
      res.writeHead(404).end('no está')
      return
    }
    if (conCors) res.setHeader('Access-Control-Allow-Origin', '*')
    res.setHeader('Content-Type', TIPOS[extname(fichero)] ?? 'application/octet-stream')
    createReadStream(fichero).pipe(res)
  })

  return new Promise((resolve) => srv.listen(puerto, '127.0.0.1', () => resolve(srv)))
}
