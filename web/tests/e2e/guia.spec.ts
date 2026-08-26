import { expect, test } from '@playwright/test'

// Prueba de punta a punta del camino real de la guía EPG (P2): captura de
// url-tvg en la cabecera del M3U (T1) → refresco dentro del ciclo de sync del
// Syncer (T5, que descarga y parsea el XMLTV con T4) → join por
// (provider_id, tvg_id) (T3) → GET /channels/epg en lote (T6) → store con TTL
// (T7) → insignia de TarjetaCanal (T8). Ningún tramo de esta cadena se
// mockea: el binario real (global-setup.ts) sirve el catálogo desde
// catalogo.m3u, cuya cabecera declara url-tvg apuntando al XMLTV dinámico que
// servidor-fixtures.ts genera en /epg.xml (ver xmltvDinamico ahí — el
// <programme> se genera con horas relativas al instante de la petición, para
// que siempre cubra "ahora" sin importar cuándo corra este test).
//
// El XMLTV de fixture solo declara <channel id="ConCors.xx">: es el tvg-id
// del canal "Canal Con CORS" en catalogo.m3u. "Canal Sin CORS" (tvg-id
// "SinCors.xx") no tiene entrada en la guía a propósito — es el caso "sin
// guía" del brief, sin necesitar un tercer canal ni tocar el catálogo que ya
// usan catalogo.spec.ts/reproduccion.spec.ts.
test('la tarjeta con guía real enseña ahora/después y la que no la tiene queda limpia', async ({
  page,
}) => {
  await page.goto('/')

  const conGuia = page.locator('article', { hasText: 'Canal Con CORS' })
  const sinGuia = page.locator('article', { hasText: 'Canal Sin CORS' })
  await expect(conGuia).toBeVisible()
  await expect(sinGuia).toBeVisible()

  // La insignia depende de que el cliente pida /channels/epg (T7, con su TTL
  // y su intervalo propio) DESPUÉS de que el catálogo ya esté pintado y de
  // que el Syncer real haya terminado de descargar+parsear+guardar la guía
  // (T5) — un timeout generoso cubre ambos, sin acoplarse a los tiempos
  // exactos de ninguno de los dos.
  await expect(conGuia.getByText(/Ahora:\s*Noticias en directo/)).toBeVisible({ timeout: 20_000 })

  // "Queda limpia" es literal: la línea de insignia (T8) reserva su alto
  // siempre, pero sin guía posible para ese canal se queda sin texto — nunca
  // un "Ahora"/"Sig" vacío o un placeholder.
  await expect(sinGuia.locator('p.linea-epg')).toHaveText('')
})
