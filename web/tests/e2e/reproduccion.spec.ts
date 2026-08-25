import { expect, test } from '@playwright/test'

// El WebKit de Playwright sobre Linux NO es Safari: su soporte de H.264/HLS
// depende de plugins de GStreamer que no están garantizados en ubuntu-latest.
// Declarar verde un motor que no decodifica sería una puerta verde que no
// prueba nada. La reproducción en WebKit es un gate MANUAL en Safari de verdad
// (ver la Tarea 17 del plan).
test.skip(
  ({ browserName }) => browserName === 'webkit' && !!process.env.CI,
  'WebKit de Linux no garantiza H.264; el gate de Safari es manual',
)

async function abrir(page: import('@playwright/test').Page, nombre: string) {
  await page.goto('/')
  await page.locator('article', { hasText: nombre }).getByRole('button', { name: nombre }).click()
}

// Dos comportamientos de producción, los dos correctos por separado, chocan
// con unas fixtures que solo pueden vivir en loopback y sin TLS:
//
// 1. ClassifyWeb (internal/domain/web.go) exige HTTPS antes de mirar CORS
//    siquiera —contenido mixto: una página seguía no carga vídeo por HTTP—.
//    Sin certificado de confianza para el servidor de fixtures (SSL_CERT_FILE
//    no lo consigue en macOS: la verificación va por el Keychain, no por esa
//    variable), los dos canales de este e2e SIEMPRE salen web_ok=false, con o
//    sin CORS. La distinción CORS/sin-CORS que separa las dos rutas de
//    fixtures se queda solo en si el intento directo puede o no leer la
//    respuesta desde JS; el veredicto persistido es falso para ambas.
// 2. El proxy (internal/proxy/handler.go, destinoPrivado) rechaza con 403
//    cualquier destino loopback/privado a propósito —es protección contra
//    SSRF, no un descuido— así que NUNCA puede relayar las fixtures de este
//    mismo e2e, que viven en 127.0.0.1.
//
// Combinado con motorDelNavegador (canPlayType): en este Playwright/macOS,
// Chromium y WebKit devuelven "maybe" para HLS y toman la rama NATIVA, que
// ignora web_ok y prueba el vídeo directo primero —una carga de <video src>
// cross-origin no pasa por el mismo cauce de lectura que un fetch/XHR, así
// que el CORS que falta no la bloquea— y por eso reproducen sin tocar el
// proxy. Firefox sí devuelve "" y toma la rama hls.js real: con web_ok=false
// forzado, su ÚNICO intento es el proxy, que rechaza el origen loopback por
// diseño. Firefox no puede reproducir NINGÚN canal de este fixture concreto,
// no por un fallo del test sino porque las dos protecciones de producción
// [1] y [2] son exactamente las que deberían dispararse aquí.
test('un canal con CORS reproduce directo y los frames avanzan', async ({ page, browserName }) => {
  test.skip(
    browserName === 'firefox',
    'web_ok sale false para las dos fixtures (contenido mixto, ver comentario arriba); ' +
      'con hls.js real el único intento es el proxy, y el proxy rechaza el 127.0.0.1 de ' +
      'las fixtures por protección SSRF. No hay forma de que Firefox reproduzca esta fixture.',
  )
  await abrir(page, 'Canal Con CORS')
  const video = page.locator('video')
  await expect(video).toBeVisible()

  // Que la POSICIÓN AVANCE es la única prueba de reproducción. `playing` se
  // emite antes de decodificar un fotograma; es lo que enseñó el guard.
  await expect
    .poll(async () => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000 })
    .toBeGreaterThan(0.5)
})

// El brief original pedía comprobar que un canal sin CORS reproduce A TRAVÉS
// del proxy. Eso es irreproducible aquí: el proxy rechaza CUALQUIER destino
// loopback —el 127.0.0.1 de este mismo servidor de fixtures incluido— así que
// jamás lo relaya con éxito (ver el bloque de comentario de arriba). Lo que
// SÍ es real y vale la pena verificar es que el rechazo no deja al usuario
// mirando un cuadro negro mudo: con web_ok=false forzado, el único intento de
// un motor hls.js de verdad es el proxy, el proxy contesta 403, y el guard
// tiene que declararlo fatal igual que si el origen hubiera muerto. Solo
// Firefox toma esa rama en este Playwright/macOS (ver arriba); Chromium y
// WebKit reproducen directo sin pasar por el proxy y no ejercitan este
// camino, así que se saltan.
test('un canal sin proxy disponible (rechazado por SSRF) declara el fallo, no se cuelga', async ({
  page,
  browserName,
}) => {
  test.skip(
    browserName !== 'firefox',
    'Chromium y WebKit toman la rama nativa (canPlayType "maybe") y reproducen directo ' +
      'sin tocar el proxy; este camino solo lo ejercita un motor hls.js real.',
  )
  await abrir(page, 'Canal Sin CORS')

  await expect(page.getByText(/no llegó a reproducir|never started playing/i)).toBeVisible({ timeout: 25_000 })
})

// Verificar el FALLO, no la salud: se mata el origen y se comprueba que el
// guard declara fatal con el mensaje correcto en vez de dejar un negro mudo.
//
// El manifiesto y los segmentos se piden de dos formas: directa
// (…/hls/canal.m3u8) y, si el directo falla, por el proxy
// (/proxy/hls?u=<url-codificada>). Un patrón como '**/hls/**' solo caza la
// primera: la URL del proxy lleva "hls" detrás de un "?", no de una "/", así
// que ese glob nunca la reconoce y el intento de respaldo pasaría igual —el
// canal reproduciría por el proxy y este test nunca vería el fallo que dice
// probar. "canal" es el nombre base que ffmpeg le da al manifiesto y a cada
// segmento (canal.m3u8, canal0.ts, …), y aparece igual de literal dentro de
// la URL codificada del proxy, así que un solo filtro por substring basta
// para cortar las dos rutas sin tocar el JS de la propia app (que no lo
// contiene).
test('si el stream muere, el guard lo dice', async ({ page }) => {
  await page.route(
    (url) => url.href.includes('canal'),
    (ruta) => ruta.abort(),
  )
  await abrir(page, 'Canal Con CORS')

  await expect(page.getByText(/no llegó a reproducir|never started playing/i)).toBeVisible({ timeout: 25_000 })
})
