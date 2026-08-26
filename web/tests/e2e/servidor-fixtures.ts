import { createReadStream, existsSync, statSync } from 'node:fs'
import { createServer, type Server } from 'node:http'
import { extname, join, normalize } from 'node:path'

const TIPOS: Record<string, string> = {
  '.m3u8': 'application/vnd.apple.mpegurl',
  '.m3u': 'application/vnd.apple.mpegurl',
  '.ts': 'video/mp2t',
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
 */
export function servidorFixtures(raiz: string, puerto: number): Promise<Server> {
  const srv = createServer((req, res) => {
    const url = new URL(req.url ?? '/', 'http://localhost')
    const conCors = url.pathname.startsWith('/cors/')
    const relativa = url.pathname.replace(/^\/(cors|sincors)\//, '')
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
