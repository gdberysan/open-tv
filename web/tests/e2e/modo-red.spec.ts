import { networkInterfaces } from 'node:os'
import { expect, test } from '@playwright/test'
import { CLAVE_E2E, PUERTO_RED } from './global-setup'

test.skip(
  ({ browserName }) => browserName === 'webkit' && !!process.env.CI,
  'WebKit de Linux no garantiza H.264; el gate de Safari es manual',
)

const BASE = `http://127.0.0.1:${PUERTO_RED}`

/** Primera IPv4 no interna de la máquina. Entrar por 127.0.0.1 es loopback:
 *  el navegador lo trata como origen de confianza y manda Sec-Fetch-Site, lo
 *  que tapaba que desde la LAN por http el login daba 403. */
function ipDeLaLAN(): string | undefined {
  for (const direcciones of Object.values(networkInterfaces())) {
    for (const d of direcciones ?? []) {
      if (d.family === 'IPv4' && !d.internal) return d.address
    }
  }
  return undefined
}

test('en modo red la app pide la clave, rechaza una mala y reproduce tras entrar', async ({ page }) => {
  const ip = ipDeLaLAN()
  if (!ip) {
    test.info().annotations.push({
      type: 'aviso',
      description: 'Sin IPv4 de LAN: se entra por 127.0.0.1 y el caso sin Sec-Fetch-Site no se prueba',
    })
  }
  const BASE = `http://${ip ?? '127.0.0.1'}:${PUERTO_RED}`

  await page.goto(BASE + '/')
  await expect(page).toHaveURL(BASE + '/acceso')

  const campo = page.getByLabel('Clave de acceso')
  await campo.fill('clave-equivocada')
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page.getByRole('alert')).toHaveText('La clave no es correcta.')

  await page.getByLabel('Clave de acceso').fill(CLAVE_E2E)
  await page.getByRole('button', { name: 'Entrar' }).click()
  await expect(page).toHaveURL(BASE + '/')

  // Mismo canal que reproduccion.spec.ts: en Chromium y Firefox solo se ve por
  // el proxy, así que esto prueba sesión + proxy + URLs firmadas juntos.
  await page.locator('.lista-lateral article', { hasText: 'Canal Con CORS' }).locator('button.abrir').click()
  const video = page.locator('video')
  await expect
    .poll(async () => video.evaluate((v: HTMLVideoElement) => v.currentTime), { timeout: 20_000 })
    .toBeGreaterThan(0.5)
})

test('en modo red la API sin sesión responde 401', async ({ request }) => {
  const resp = await request.get(BASE + '/sources')
  expect(resp.status()).toBe(401)
})
