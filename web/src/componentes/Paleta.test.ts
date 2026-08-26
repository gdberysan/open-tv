import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen, within } from '@testing-library/svelte'
import { tick, type ComponentProps } from 'svelte'
import Paleta from './Paleta.svelte'
import type { Canal, Faceta } from '../datos/catalogo'
import { filtros } from '../estado/filtros'
import { idioma, t } from '../i18n'

// Paleta de comandos ⌘K (Tarea 4, P0.8): fuzzy sobre canales/facetas/acciones
// (coincideDifuso, Tarea 3), combobox con aria-activedescendant, foco
// atrapado + restaurado. El atajo ⌘K en sí (abrir/inhibir) vive en App.svelte
// — ver App.integracion.test.ts, describe 'paleta de comandos'.

function canalDePrueba(id: string, nombre: string, pais = ''): Canal {
  return { id, nombre, logoUrl: '', categoriaId: '', idioma: 'es', pais, vivo: null, latenciaMs: 0, webOk: null }
}

function faceta(valor: string, total = 1): Faceta {
  return { valor, total }
}

function propsBase(overrides: Partial<ComponentProps<typeof Paleta>> = {}): ComponentProps<typeof Paleta> {
  return {
    canales: [] as Canal[],
    paises: [] as Faceta[],
    categorias: [] as Faceta[],
    calidades: [] as Faceta[],
    alCerrar: vi.fn(),
    alAbrirCanal: vi.fn(),
    alAleatorio: vi.fn(),
    alAbrirFuentes: vi.fn(),
    alAbrirStats: vi.fn(),
    ...overrides,
  }
}

async function asentar() {
  await new Promise<number>((resolve) => requestAnimationFrame(resolve))
  await tick()
}

function dialogo(): HTMLElement {
  return screen.getByRole('dialog')
}

function entrada(): HTMLInputElement {
  return screen.getByRole('combobox') as HTMLInputElement
}

function opciones(): HTMLElement[] {
  return screen.getAllByRole('option')
}

function opcionActiva(): HTMLElement | null {
  const id = entrada().getAttribute('aria-activedescendant')
  return id ? document.getElementById(id) : null
}

beforeEach(() => {
  idioma.actual = 'es'
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false, soloFavoritos: false, vista: 'rejilla',
  })
})

describe('Paleta — apertura y estructura', () => {
  it('se monta como role=dialog modal, con el input enfocado', async () => {
    render(Paleta, propsBase())
    await tick()
    expect(dialogo().getAttribute('aria-modal')).toBe('true')
    expect(document.activeElement).toBe(entrada())
  })

  it('con la caja vacía, muestra un valor por defecto útil: los canales cargados y las acciones', async () => {
    render(Paleta, propsBase({ canales: [canalDePrueba('1', 'CNN en Español')] }))
    await tick()
    expect(screen.getByText('CNN en Español')).not.toBeNull()
    expect(screen.getByText(t('accion.aleatorio'))).not.toBeNull()
  })

  it('los tres grupos llevan encabezado visible cuando tienen contenido', async () => {
    render(Paleta, propsBase({
      canales: [canalDePrueba('1', 'CNN en Español')],
      paises: [faceta('MX')],
    }))
    await tick()
    expect(screen.getByText(t('paleta.grupo.canales'))).not.toBeNull()
    expect(screen.getByText(t('paleta.grupo.facetas'))).not.toBeNull()
    expect(screen.getByText(t('paleta.grupo.acciones'))).not.toBeNull()
  })
})

describe('Paleta — (b) filtrado difuso agrupado', () => {
  it('teclear filtra los canales por coincideDifuso: "cnn" casa con CNN, no con BBC', async () => {
    render(Paleta, propsBase({
      canales: [canalDePrueba('1', 'CNN en Español'), canalDePrueba('2', 'BBC News')],
    }))
    const campo = entrada()
    await fireEvent.input(campo, { target: { value: 'cnn' } })
    await tick()

    expect(screen.getByText('CNN en Español')).not.toBeNull()
    expect(screen.queryByText('BBC News')).toBeNull()
  })

  it('teclear filtra las facetas: el nombre de país (no el código) es lo que casa', async () => {
    render(Paleta, propsBase({ paises: [faceta('MX'), faceta('US')] }))
    const campo = entrada()
    await fireEvent.input(campo, { target: { value: 'méxico' } })
    await tick()

    expect(screen.getByText(`${t('filtro.pais')}: México`)).not.toBeNull()
    expect(screen.queryByText(new RegExp(`${t('filtro.pais')}:.*Estados`))).toBeNull()
  })

  it('teclear filtra las acciones: "azar" encuentra "Canal al azar" pero no "Limpiar filtros"', async () => {
    render(Paleta, propsBase())
    const campo = entrada()
    await fireEvent.input(campo, { target: { value: 'azar' } })
    await tick()

    expect(screen.getByText(t('accion.aleatorio'))).not.toBeNull()
    expect(screen.queryByText(t('filtro.limpiar'))).toBeNull()
  })

  it('recorta cada grupo a un tope (20): 30 canales que casan todos solo pintan 20 filas', async () => {
    const canales = Array.from({ length: 30 }, (_, i) => canalDePrueba(String(i), `Canal Uno ${i}`))
    render(Paleta, propsBase({ canales }))
    const campo = entrada()
    await fireEvent.input(campo, { target: { value: 'canal uno' } })
    await tick()

    const grupoCanales = screen.getByText(t('paleta.grupo.canales')).parentElement as HTMLElement
    expect(within(grupoCanales).getAllByRole('option').filter((o) => !o.className.includes('opcion-accion')).length).toBe(20)
  })
})

describe('Paleta — (c) navegación con flechas y activación', () => {
  it('ArrowDown recorre TODAS las opciones (canales + acciones) y envuelve; ArrowUp hace lo mismo al revés', async () => {
    render(Paleta, propsBase({
      canales: [canalDePrueba('1', 'Canal Uno'), canalDePrueba('2', 'Canal Dos')],
    }))
    const campo = entrada()
    await tick()
    const total = opciones().length
    // Con la caja vacía también se listan las Acciones (siempre presentes):
    // más de dos opciones en juego, para que el wrap se pruebe de verdad
    // contra la lista PLANA completa, no solo los dos canales.
    expect(total).toBeGreaterThan(2)
    expect(opcionActiva()).toBe(opciones()[0])

    for (let i = 1; i < total; i++) {
      await fireEvent.keyDown(campo, { key: 'ArrowDown' })
      expect(opcionActiva()).toBe(opciones()[i])
    }
    await fireEvent.keyDown(campo, { key: 'ArrowDown' }) // desde el último, envuelve al primero
    expect(opcionActiva()).toBe(opciones()[0])

    await fireEvent.keyDown(campo, { key: 'ArrowUp' }) // desde el primero, envuelve al último
    expect(opcionActiva()).toBe(opciones()[total - 1])
  })

  it('Enter sobre un canal llama a alAbrirCanal con ese canal y cierra', async () => {
    const alAbrirCanal = vi.fn()
    const alCerrar = vi.fn()
    const canal = canalDePrueba('1', 'Canal Uno')
    render(Paleta, propsBase({ canales: [canal], alAbrirCanal, alCerrar }))
    const campo = entrada()
    // Con la caja vacía, el primer canal cargado es la opción activa por
    // defecto (ver el comentario de opcionesCanal en Paleta.svelte).
    await fireEvent.keyDown(campo, { key: 'Enter' })

    expect(alAbrirCanal).toHaveBeenCalledWith(canal)
    expect(alCerrar).toHaveBeenCalled()
  })

  it('Enter sobre una faceta escribe filtros (el CÓDIGO, no el nombre mostrado) y cierra', async () => {
    const alCerrar = vi.fn()
    render(Paleta, propsBase({ canales: [], paises: [faceta('MX')], alCerrar }))
    const campo = entrada()
    await fireEvent.input(campo, { target: { value: 'méxico' } })
    await tick()
    // La fila "Buscar «méxico» en todos los canales" también aparece con
    // término no vacío — se activa la faceta explícitamente (pasar el ratón
    // por encima), igual que haría alguien navegando con flechas hasta ella.
    const filaFaceta = screen.getByText(`${t('filtro.pais')}: México`)
    await fireEvent.mouseEnter(filaFaceta)
    await fireEvent.keyDown(campo, { key: 'Enter' })

    let valor: string | undefined
    filtros.subscribe((f) => (valor = f.pais))()
    expect(valor).toBe('MX')
    expect(alCerrar).toHaveBeenCalled()
  })

  it('Enter sobre una acción la ejecuta y cierra ("Canal al azar" llama a alAleatorio)', async () => {
    const alAleatorio = vi.fn()
    const alCerrar = vi.fn()
    render(Paleta, propsBase({ alAleatorio, alCerrar }))
    const campo = entrada()
    await fireEvent.input(campo, { target: { value: 'Canal al azar' } })
    await tick()
    // Misma razón que arriba: con término no vacío, la fila "Buscar…" precede
    // a las Acciones en la lista plana — se activa la acción explícitamente.
    const filaAccion = screen.getByText(t('accion.aleatorio'))
    await fireEvent.mouseEnter(filaAccion)
    await fireEvent.keyDown(campo, { key: 'Enter' })

    expect(alAleatorio).toHaveBeenCalled()
    expect(alCerrar).toHaveBeenCalled()
  })

  it('clic con el ratón sobre una opción la activa y la ejecuta igual que Enter', async () => {
    const alAbrirCanal = vi.fn()
    const canal = canalDePrueba('1', 'Canal Uno')
    render(Paleta, propsBase({ canales: [canal], alAbrirCanal }))
    await tick()
    await fireEvent.click(screen.getByText('Canal Uno'))

    expect(alAbrirCanal).toHaveBeenCalledWith(canal)
  })
})

describe('Paleta — (d) Esc cierra y restaura el foco', () => {
  it('Esc llama a alCerrar', async () => {
    const alCerrar = vi.fn()
    render(Paleta, propsBase({ alCerrar }))
    await fireEvent.keyDown(entrada(), { key: 'Escape' })
    expect(alCerrar).toHaveBeenCalled()
  })

  it('al desmontar, el foco vuelve a quien lo tenía antes de abrir (diferido a un frame)', async () => {
    document.body.innerHTML = '<button id="disparador">abrir</button>'
    const disparador = document.getElementById('disparador') as HTMLButtonElement
    disparador.focus()
    expect(document.activeElement).toBe(disparador)

    const { unmount } = render(Paleta, propsBase())
    await tick()
    await tick()
    expect(document.activeElement).not.toBe(disparador) // el foco entró en la paleta

    unmount()
    await asentar()
    expect(document.activeElement).toBe(disparador)
  })
})

describe('Paleta — (e) "Buscar «X» en todos los canales"', () => {
  it('aparece con término no vacío y, al elegirla, fija filtros.q y cierra', async () => {
    const alCerrar = vi.fn()
    render(Paleta, propsBase({ alCerrar }))
    const campo = entrada()
    await fireEvent.input(campo, { target: { value: 'motogp' } })
    await tick()

    const fila = screen.getByText(t('paleta.buscarTodos', { termino: 'motogp' }))
    expect(fila).not.toBeNull()
    await fireEvent.click(fila)

    let q: string | undefined
    filtros.subscribe((f) => (q = f.q))()
    expect(q).toBe('motogp')
    expect(alCerrar).toHaveBeenCalled()
  })

  it('NO aparece con la caja vacía', async () => {
    render(Paleta, propsBase())
    await tick()
    expect(screen.queryByText(/Buscar «.*» en todos los canales/)).toBeNull()
  })
})
