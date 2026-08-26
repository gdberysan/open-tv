import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen, within } from '@testing-library/svelte'
import { get } from 'svelte/store'
import BarraLateralFacetas from './BarraLateralFacetas.svelte'
import { filtros } from '../estado/filtros'
import { t } from '../i18n'
import type { Faceta } from '../datos/catalogo'

// Patrón establecido en P0.5: el reset vive a nivel de fichero, no anidado en
// cada describe — así ningún test hereda el store mutado por el anterior.
beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
})

const paises: Faceta[] = [{ valor: 'ES', total: 12 }, { valor: 'MX', total: 7 }]
const categorias: Faceta[] = [{ valor: 'Deportes', total: 3 }]
const calidades: Faceta[] = [{ valor: 'hd', total: 20 }, { valor: 'fhd', total: 9 }, { valor: '4k', total: 2 }]

// getByRole con `name` de cadena hace coincidencia EXACTA por defecto (no
// substring), y la etiqueta traducida de "hd" lleva paréntesis que no son
// literales seguros para una RegExp — de ahí este filtro manual en vez de un
// matcher de nombre.
function filaCalidadHd(contenedor: HTMLElement): HTMLElement {
  const etiqueta = t('filtro.calidad.hd')
  const boton = within(contenedor)
    .getAllByRole('button')
    .find((b) => b.textContent?.includes(etiqueta))
  if (!boton) throw new Error(`No se encontró la fila de calidad "${etiqueta}"`)
  return boton
}

describe('BarraLateralFacetas — grupos con conteos', () => {
  it('pinta País/Categoría/Calidad, cada fila con su valor y su conteo', () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    expect(within(grupoPais).getByRole('button', { name: /ES/ }).textContent).toContain('12')
    expect(within(grupoPais).getByRole('button', { name: /MX/ }).textContent).toContain('7')

    const grupoCategoria = screen.getByRole('group', { name: t('filtro.categoria') })
    expect(within(grupoCategoria).getByRole('button', { name: /Deportes/ }).textContent).toContain('3')

    const grupoCalidad = screen.getByRole('group', { name: t('filtro.calidad') })
    expect(filaCalidadHd(grupoCalidad).textContent).toContain('20')
  })
})

describe('BarraLateralFacetas — seleccionar/deseleccionar una faceta', () => {
  it('pulsar un país escribe filtros.pais; pulsarlo otra vez lo limpia', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    const filaEs = within(grupoPais).getByRole('button', { name: /ES/ })

    expect(filaEs.getAttribute('aria-pressed')).toBe('false')

    await fireEvent.click(filaEs)
    expect(get(filtros).pais).toBe('ES')
    expect(filaEs.getAttribute('aria-pressed')).toBe('true')

    await fireEvent.click(filaEs)
    expect(get(filtros).pais).toBe('')
    expect(filaEs.getAttribute('aria-pressed')).toBe('false')
  })

  it('pulsar una categoría escribe filtros.categoria', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoCategoria = screen.getByRole('group', { name: t('filtro.categoria') })
    await fireEvent.click(within(grupoCategoria).getByRole('button', { name: /Deportes/ }))
    expect(get(filtros).categoria).toBe('Deportes')
  })

  it('pulsar una calidad escribe filtros.calidad', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoCalidad = screen.getByRole('group', { name: t('filtro.calidad') })
    await fireEvent.click(filaCalidadHd(grupoCalidad))
    expect(get(filtros).calidad).toBe('hd')
  })
})

describe('BarraLateralFacetas — bloque Señal', () => {
  it('«Mostrar los que no responden» escribe mostrarOffline=true, como promete su etiqueta', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const casilla = screen.getByLabelText(t('filtro.mostrarOffline')) as HTMLInputElement
    expect(casilla.checked).toBe(false)
    expect(get(filtros).mostrarOffline).toBe(false)

    await fireEvent.click(casilla)

    expect(casilla.checked).toBe(true)
    expect(get(filtros).mostrarOffline).toBe(true)
  })

  it('«Solo señal viva» vuelve a poner mostrarOffline=false', async () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    filtros.update((f) => ({ ...f, mostrarOffline: true }))

    const soloViva = screen.getByRole('button', { name: t('senal.soloViva') })
    await fireEvent.click(soloViva)

    expect(get(filtros).mostrarOffline).toBe(false)
  })
})

describe('BarraLateralFacetas — búsqueda con debounce', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('no escribe en el store hasta 300ms después de la última tecla, y solo con el último valor', async () => {
    vi.useFakeTimers()
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const campo = screen.getByLabelText(t('catalogo.buscar')) as HTMLInputElement

    await fireEvent.input(campo, { target: { value: 'b' } })
    vi.advanceTimersByTime(100)
    await fireEvent.input(campo, { target: { value: 'bb' } })
    vi.advanceTimersByTime(100)
    await fireEvent.input(campo, { target: { value: 'bbc' } })

    vi.advanceTimersByTime(299)
    expect(get(filtros).q).toBe('')

    vi.advanceTimersByTime(1)
    expect(get(filtros).q).toBe('bbc')
  })
})

describe('BarraLateralFacetas — resincronización con filtros.q externo', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('un cambio externo de filtros.q (chip/Limpiar filtros) vacía la caja de búsqueda', async () => {
    vi.useFakeTimers()
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const campo = screen.getByLabelText(t('catalogo.buscar')) as HTMLInputElement

    await fireEvent.input(campo, { target: { value: 'sport' } })
    vi.advanceTimersByTime(300)
    expect(get(filtros).q).toBe('sport')
    expect(campo.value).toBe('sport')

    // Simula el chip "q" / "Quitar «sport»" / "Limpiar filtros": ninguno de
    // ellos toca este componente, solo el store.
    filtros.update((f) => ({ ...f, q: '' }))
    await Promise.resolve()

    expect(campo.value).toBe('')
  })

  it('el eco de nuestro propio empuje (mismo valor) no borra texto local sin empujar aún', async () => {
    vi.useFakeTimers()
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const campo = screen.getByLabelText(t('catalogo.buscar')) as HTMLInputElement

    await fireEvent.input(campo, { target: { value: 'bbc' } })
    vi.advanceTimersByTime(300)
    expect(get(filtros).q).toBe('bbc')

    // El usuario sigue tecleando ANTES de que el próximo debounce dispare:
    // el store todavía no sabe nada de esto.
    await fireEvent.input(campo, { target: { value: 'bbc2' } })
    expect(campo.value).toBe('bbc2')

    // Un re-set del store al MISMO valor que ya empujamos (p. ej. otro
    // suscriptor forzando el mismo `q`) es indistinguible de nuestro propio
    // eco: no debe pisar lo que el usuario está tecleando.
    filtros.update((f) => ({ ...f, q: 'bbc' }))
    await Promise.resolve()

    expect(campo.value).toBe('bbc2')
  })
})

describe('BarraLateralFacetas — país buscable (>12 facetas)', () => {
  const paisesLargos: Faceta[] = Array.from({ length: 15 }, (_, i) => ({ valor: `P${i}`, total: i + 1 }))
  const paisesConMexico: Faceta[] = [...paisesLargos, { valor: 'México', total: 4 }]

  it('pinta el buscador del grupo y una lista acotada con TODAS las facetas, sin «Ver los N países»', () => {
    render(BarraLateralFacetas, { paises: paisesLargos, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    expect(within(grupoPais).getByLabelText(t('filtro.filtrarPais'))).toBeTruthy()
    // Sin recorte: las 15 facetas están todas en el DOM (la lista acotada
    // scrollea, no trunca), no las 8 del viejo colapso.
    expect(within(grupoPais).getByRole('button', { name: /P14/ })).toBeTruthy()
    // El botón «Ver los N países» de antes tenía aria-expanded; ya no existe
    // ningún control así en el grupo.
    expect(grupoPais.querySelector('[aria-expanded]')).toBeNull()
    expect(grupoPais.querySelector('.lista-acotada')).not.toBeNull()
  })

  it('teclear "mex" filtra a México, insensible a mayúsculas/acentos', async () => {
    render(BarraLateralFacetas, { paises: paisesConMexico, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    const buscador = within(grupoPais).getByLabelText(t('filtro.filtrarPais'))

    await fireEvent.input(buscador, { target: { value: 'mex' } })

    expect(within(grupoPais).getByRole('button', { name: /México/ })).toBeTruthy()
    expect(within(grupoPais).getAllByRole('button')).toHaveLength(1)
  })

  it('limpiar el filtro restaura todas las facetas', async () => {
    render(BarraLateralFacetas, { paises: paisesConMexico, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    const buscador = within(grupoPais).getByLabelText(t('filtro.filtrarPais'))

    await fireEvent.input(buscador, { target: { value: 'mex' } })
    expect(within(grupoPais).getAllByRole('button')).toHaveLength(1)

    await fireEvent.input(buscador, { target: { value: '' } })
    expect(within(grupoPais).getAllByRole('button')).toHaveLength(paisesConMexico.length)
  })

  it('la selección sigue funcionando sobre la lista filtrada', async () => {
    render(BarraLateralFacetas, { paises: paisesConMexico, categorias, calidades })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    const buscador = within(grupoPais).getByLabelText(t('filtro.filtrarPais'))
    await fireEvent.input(buscador, { target: { value: 'mex' } })

    const filaMexico = within(grupoPais).getByRole('button', { name: /México/ })
    expect(filaMexico.getAttribute('aria-pressed')).toBe('false')

    await fireEvent.click(filaMexico)

    expect(get(filtros).pais).toBe('México')
    expect(filaMexico.getAttribute('aria-pressed')).toBe('true')
  })
})

describe('BarraLateralFacetas — grupo corto sin buscador', () => {
  it('Calidad (3 facetas) no pinta buscador de grupo ni lista acotada', () => {
    render(BarraLateralFacetas, { paises, categorias, calidades })

    const grupoCalidad = screen.getByRole('group', { name: t('filtro.calidad') })
    expect(grupoCalidad.querySelector('input[type="search"]')).toBeNull()
    expect(grupoCalidad.querySelector('.lista-acotada')).toBeNull()
  })
})
