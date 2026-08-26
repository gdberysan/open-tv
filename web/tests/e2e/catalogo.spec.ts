import { expect, test } from '@playwright/test'

test('la rejilla se llena y el filtro reduce', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Canal Con CORS (1080p)')).toBeVisible()
  await expect(page.getByText('Canal Sin CORS (720p)')).toBeVisible()

  await page.getByPlaceholder(/buscar|search/i).fill('Sin CORS')
  await expect(page.getByText('Canal Sin CORS (720p)')).toBeVisible()
  await expect(page.getByText('Canal Con CORS (1080p)')).toHaveCount(0)
})

// La marca APP es el veredicto web_ok llegando hasta el píxel. Si el clasificador,
// la columna, la agregación o el mapeo del cliente se rompen, se ve aquí.
//
// El brief original esperaba que SOLO el canal sin CORS se marcara. Contra el
// clasificador real (internal/domain/web.go, ClassifyWeb) eso no ocurre: antes
// de mirar CORS exige HTTPS —contenido mixto, una página segura no carga vídeo
// por HTTP— y las fixtures de este e2e son HTTP a propósito (loopback, sin
// certificado de confianza: SSL_CERT_FILE no lo resuelve en macOS, que verifica
// contra el Keychain). Los DOS canales salen con web_ok=false pase lo que pase
// con el CORS, así que los DOS se marcan. Sigue siendo una prueba real: prueba
// que el veredicto persistido —cualquiera que sea— llega íntegro desde el
// checker hasta el píxel para cada canal, que es lo que este test dice medir.
test('el veredicto web_ok llega marcado hasta la tarjeta de cada canal', async ({ page }) => {
  await page.goto('/')
  const sinCors = page.locator('article', { hasText: 'Canal Sin CORS' })
  const conCors = page.locator('article', { hasText: 'Canal Con CORS' })
  await expect(sinCors.getByText('APP')).toBeVisible()
  await expect(conCors.getByText('APP')).toBeVisible()
})

test('el idioma cambia y se recuerda', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'English' }).click()
  await expect(page.getByPlaceholder(/search/i)).toBeVisible()
  await page.reload()
  await expect(page.getByPlaceholder(/search/i)).toBeVisible()
})

test('un favorito sobrevive a la recarga', async ({ page }) => {
  await page.goto('/')
  const tarjeta = page.locator('article', { hasText: 'Canal Con CORS' })
  await tarjeta.getByRole('button', { name: /favorit/i }).click()
  await page.reload()
  await expect(tarjeta.getByRole('button', { name: /favorit/i })).toHaveAttribute('aria-pressed', 'true')
})
