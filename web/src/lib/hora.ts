// El cable manda inicioSeg/finSeg como epoch UTC en SEGUNDOS (ver Programa,
// datos/catalogo.ts) — la insignia de la tarjeta (TarjetaCanal, Tarea 8) los
// muestra en la hora LOCAL del dispositivo, no en UTC: un "Sig 20:30" tiene
// que coincidir con el reloj de pared de quien mira la pantalla, sea cual
// sea su huso horario. new Date(epochSegundos * 1000) hace esa conversión
// (Date SIEMPRE construye en UTC desde un epoch, pero se LEE en local); el
// formateador (Intl vía toLocaleTimeString) es quien la vuelve a mostrar en
// la hora local del navegador.
//
// hour12: false a propósito, y no un locale explícito: la app ya alterna
// es/en (ver i18n/idioma.svelte.ts) pero un reloj de guía en 12h con AM/PM
// sería más ruido que ayuda en una insignia de una sola línea — HH:MM de 24h
// es el mismo formato en ambos idiomas.
export function formatearHoraLocal(epochSegundos: number): string {
  const fecha = new Date(epochSegundos * 1000)
  return fecha.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hour12: false })
}
