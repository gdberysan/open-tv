import { expect, test } from '@playwright/test'

// e2e del escenario reproductor-primero (spec §10) contra el binario real:
// entrada, cambio de canal EN EL SITIO, modo ver-todo sin desmontar el vídeo,
// auto-reanudación muted con CTA de sonido, y el apilado responsive.

test.skip(
  ({ browserName }) => browserName === 'webkit' && !!process.env.CI,
  'WebKit de Linux no garantiza H.264; el gate de Safari es manual',
)

function filaLateral(page: import('@playwright/test').Page, nombre: string) {
  return page.locator('.lista-lateral article', { hasText: nombre }).locator('button.abrir')
}

test('arranque en limpio: tarjeta «elige un canal», lateral con canales, sin modal', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Elige un canal para empezar')).toBeVisible()
  await expect(page.locator('.lista-lateral article').first()).toBeVisible()
  await expect(page.locator('[role="dialog"]')).toHaveCount(0)
})

test('cambiar de canal en la lateral intercambia el vídeo EN EL SITIO (mismo elemento)', async ({ page }) => {
  await page.goto('/')
  await filaLateral(page, 'Canal Con CORS').click()
  await expect(page.locator('video')).toBeVisible()
  // Marca el nodo: si el swap desmontara/remontara el <video>, la marca se
  // perdería con el nodo viejo.
  await page.evaluate(() => {
    const v = document.querySelector('video') as HTMLVideoElement & { __marca?: string }
    v.__marca = 'persistente'
  })
  await filaLateral(page, 'Canal Sin CORS').click()
  // La fila activa refleja el cambio antes de que el stream llegue a nada.
  await expect(filaLateral(page, 'Canal Sin CORS')).toHaveAttribute('aria-current', 'true')
  const marca = await page.evaluate(
    () => (document.querySelector('video') as HTMLVideoElement & { __marca?: string }).__marca,
  )
  expect(marca).toBe('persistente')
})

test('ver-todo abre la rejilla sin desmontar el vídeo, y seleccionar vuelve reproduciendo', async ({ page }) => {
  await page.goto('/')
  await filaLateral(page, 'Canal Con CORS').click()
  await expect(page.locator('video')).toBeVisible()

  await page.getByRole('button', { name: 'Ver todo' }).click()
  // El escenario queda CUBIERTO (hidden), nunca desmontado: el <video> sigue
  // en el DOM debajo del modo ver-todo (spec §10, «sin desmontar/remontar»).
  await expect(page.locator('.escenario[hidden] video')).toHaveCount(1)

  await page
    .locator('article:visible', { hasText: 'Canal Sin CORS' })
    .getByRole('button', { name: 'Canal Sin CORS (720p)' })
    .click()
  await expect(page.locator('.escenario[hidden]')).toHaveCount(0)
  await expect(page.getByRole('region', { name: 'Canal Sin CORS (720p)' })).toBeVisible()
})

test('tras ver un canal, recargar auto-reproduce EN SILENCIO con la CTA de sonido', async ({ page }) => {
  await page.goto('/')
  await filaLateral(page, 'Canal Con CORS').click()
  await expect(page.locator('video')).toBeVisible()

  await page.reload()
  await expect(page.getByRole('button', { name: 'Toca para activar el sonido' })).toBeVisible()
  await expect(page.locator('video')).toHaveJSProperty('muted', true)
  await page.getByRole('button', { name: 'Toca para activar el sonido' }).click()
  await expect(page.locator('video')).toHaveJSProperty('muted', false)
  await expect(page.getByRole('button', { name: 'Toca para activar el sonido' })).toHaveCount(0)
})

test('viewport estrecho: vídeo arriba, catálogo debajo — el vídeo no se pierde', async ({ page }) => {
  await page.setViewportSize({ width: 700, height: 900 })
  await page.goto('/')
  await filaLateral(page, 'Canal Con CORS').click()
  await expect(page.locator('video')).toBeVisible()
  const video = await page.locator('.panel-video').boundingBox()
  const lateral = await page.locator('.lateral').boundingBox()
  expect(video && lateral && video.y < lateral.y).toBe(true)
})
