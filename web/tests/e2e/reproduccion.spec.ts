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
  // Reproductor-primero: el arranque en limpio muestra el escenario (panel de
  // vídeo + catálogo lateral) — el canal se abre desde la fila de la lateral,
  // ya no desde una tarjeta de rejilla.
  await page.goto('/')
  await page.locator('.lista-lateral article', { hasText: nombre }).locator('button.abrir').click()
}

// ClassifyWeb (internal/domain/web.go) exige HTTPS antes de mirar CORS
// siquiera —contenido mixto: una página segura no carga vídeo por HTTP—. Sin
// certificado de confianza para el servidor de fixtures, los dos canales de
// este e2e SIEMPRE salen web_ok=false. Y motorDelNavegador (plan.ts) prefiere
// hls.js sobre cualquier veredicto nativo que no sea definitivo en cuanto hay
// MediaSource disponible, que es el caso de Chromium y Firefox por igual (solo
// Safari da 'probably' y se queda con el nativo). La combinación de las dos
// cosas hace que, con web_ok=false, el ÚNICO intento de Chromium y Firefox
// aquí sea el proxy (planDeReproduccion, rama `webOk === false`): nunca hay
// intento directo que probar por separado.
//
// Que el proxy pueda relayar estas fixtures en absoluto depende de que el
// servidor que este e2e arranca (global-setup.ts) le diga
// OPEN_TV_PERMITIR_DESTINOS_PRIVADOS=1: sin eso, destinoPrivado
// (internal/proxy/handler.go) rechaza con 403 cualquier destino
// loopback/privado por protección SSRF —correcto en producción, donde el
// proxy jamás debe relayar 127.0.0.1— y ningún motor podría reproducir un
// fixture que vive necesariamente en loopback. Con la variable puesta, el
// proxy sí releva el origen real, así que hay algo genuino que probar aquí en
// los dos motores soportados en CI.
test('un canal reproduce vía proxy y los frames avanzan', async ({ page }) => {
  await abrir(page, 'Canal Con CORS')
  const video = page.locator('video')
  await expect(video).toBeVisible()

  // Que la POSICIÓN AVANCE es la única prueba de reproducción. `playing` se
  // emite antes de decodificar un fotograma; es lo que enseñó el guard.
  await expect
    .poll(async () => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000 })
    .toBeGreaterThan(0.5)
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
//
// El mensaje esperado es el de clasificarFallo (diagnostico.ts) para un error
// de red sin status HTTP útil ('caido'), no el genérico "no llegó a
// reproducir" (ese es para 'desconocido', cuando hls.js no da información
// aprovechable): abortar la petición es justo el caso de red que hls.js
// reporta como NETWORK_ERROR, así que el guard sabe distinguirlo y muestra el
// mensaje específico.
test('si el stream muere, el guard lo dice', async ({ page }) => {
  await page.route(
    (url) => url.href.includes('canal'),
    (ruta) => ruta.abort(),
  )
  await abrir(page, 'Canal Con CORS')

  await expect(
    page.getByText(/está caído o su dirección caducó|is down or its address expired/i).first(),
  ).toBeVisible({ timeout: 25_000 })
})
