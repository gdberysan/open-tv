import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/svelte'
import Onboarding from '../componentes/Onboarding.svelte'
import Fuentes from '../componentes/Fuentes.svelte'
import SincronizandoFuente from '../componentes/SincronizandoFuente.svelte'
import BarraLateralFacetas from '../componentes/BarraLateralFacetas.svelte'
import PieDeMarca from '../componentes/PieDeMarca.svelte'
import type { CatalogSource, Faceta, Fuente } from '../datos/catalogo'
import { filtros } from '../estado/filtros'
import { idioma, t } from '../i18n'

// Tarea 11 (P0.7): pase de accesibilidad sobre TODO lo construido en las
// Tareas 6-10 — Onboarding/Fuentes/AnadirFuente (embebido en ambos)/
// BarraLateralFacetas/PieDeMarca. Este fichero es la evidencia falsable de la
// auditoría: cada test aquí corresponde a un punto concreto del brief de la
// Tarea 11. NO se duplica aquí lo que los ficheros *.test.ts de cada
// componente ya cubren exhaustivamente (comportamiento funcional); estos
// tests se centran en la CAPA DE ACCESIBILIDAD — nombres accesibles, foco,
// y el único bug real cazado en la auditoría (fallback de etiqueta).

function fuenteDePrueba(id: string, overrides: Partial<Fuente> = {}): Fuente {
  return {
    id,
    label: `Fuente ${id}`,
    url: `https://ej.test/${id}.m3u`,
    kind: 'url',
    ultimoSync: null,
    canales: 0,
    ...overrides,
  }
}

function fuenteFalsa(overrides: Partial<CatalogSource> = {}): CatalogSource {
  return {
    canales: vi.fn(async () => ({ canales: [], total: 0 })),
    paises: vi.fn(async () => []),
    categorias: vi.fn(async () => []),
    calidades: vi.fn(async () => []),
    aleatorio: vi.fn(async () => {
      throw new Error('no usado en este test')
    }),
    destino: vi.fn(async () => ({ url: '', airplayOk: null })),
    mirrors: vi.fn(async () => []),
    frescura: vi.fn(async () => ({ tipo: 'vivo' as const, generadoEn: null })),
    proxyDisponible: vi.fn(async () => false),
    fuentes: vi.fn(async () => [fuenteDePrueba('f1')]),
    anadirFuente: vi.fn(async (url: string, label?: string) => fuenteDePrueba(label ?? url)),
    anadirFuenteFichero: vi.fn(async (f: File) => fuenteDePrueba(f.name, { kind: 'file' })),
    quitarFuente: vi.fn(async () => {}),
    resyncFuente: vi.fn(async () => {}),
    fuentesSugeridas: vi.fn(async () => []),
    ...overrides,
  }
}

beforeEach(() => {
  idioma.actual = 'es'
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
})

describe('a11y P0.7 — Onboarding: campos y sugeridas con nombre accesible', () => {
  it('el campo de URL tiene nombre accesible (aria-label) y el de fichero un <label> real', () => {
    render(Onboarding, { fuente: fuenteFalsa(), alFuenteAnadida: vi.fn() })

    expect(screen.getByLabelText(t('onboarding.url.etiqueta'))).toBeTruthy()
    const campoFichero = screen.getByLabelText(t('onboarding.fichero.etiqueta')) as HTMLInputElement
    expect(campoFichero.type).toBe('file')
  })

  it('cada botón de «sugeridas» tiene un nombre accesible que identifica LA FUENTE, no solo «Añadir»', async () => {
    const sugeridas = [
      { label: 'iptv-org (deportes)', url: 'https://ej.test/deportes.m3u' },
      { label: 'iptv-org (noticias)', url: 'https://ej.test/noticias.m3u' },
    ]
    render(Onboarding, {
      fuente: fuenteFalsa({ fuentesSugeridas: vi.fn(async () => sugeridas) }),
      alFuenteAnadida: vi.fn(),
    })

    // Falsable: si el botón cayera a un nombre accesible genérico
    // compartido ("Añadir"), getByRole con el nombre completo por fuente no
    // encontraría dos botones DISTINTOS.
    await screen.findByRole('button', { name: t('onboarding.sugeridas.anadir', { label: 'iptv-org (deportes)' }) })
    await screen.findByRole('button', { name: t('onboarding.sugeridas.anadir', { label: 'iptv-org (noticias)' }) })
  })
})

describe('a11y P0.7 — el timeout de sondeo y su «Reintentar» son alcanzables', () => {
  it('«Reintentar» es un <button> nativo, sin tabindex negativo ni disabled — en el orden de tabulación', () => {
    render(SincronizandoFuente, { agotado: true, alReintentar: vi.fn() })

    const boton = screen.getByRole('button', { name: t('onboarding.sondeo.reintentar') }) as HTMLButtonElement
    expect(boton.tagName).toBe('BUTTON')
    expect(boton.disabled).toBe(false)
    expect(boton.getAttribute('tabindex')).toBeNull() // null = tab-stop natural (0 implícito)
  })
})

describe('a11y P0.7 — Fuentes: fallback de etiqueta (fix del ledger)', () => {
  it('una fuente con label Y url vacíos muestra su ID — nunca una fila sin texto identificador', async () => {
    const fuentes = vi.fn(async () => [fuenteDePrueba('src-legado-123', { label: '', url: '' })])
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    // El id aparece como texto visible de la fila...
    await screen.findByText('src-legado-123', { selector: '.label' })
    // ...Y como el nombre accesible de sus dos acciones (mismo fallback,
    // misma función `etiqueta()`) — antes del fix, ambos aria-label habrían
    // quedado como "Re-sincronizar " / "Quitar " (label vacío + url vacía).
    expect(
      screen.getByRole('button', { name: t('fuentes.resincronizar.etiqueta', { label: 'src-legado-123' }) }),
    ).toBeTruthy()
    expect(
      screen.getByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'src-legado-123' }) }),
    ).toBeTruthy()
  })

  it('label vacío pero url presente sigue usando la url (fallback de dos escalones, no roto por el tercero)', async () => {
    const fuentes = vi.fn(async () => [
      fuenteDePrueba('f2', { label: '', url: 'https://ej.test/sin-label.m3u' }),
    ])
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    await screen.findByText('https://ej.test/sin-label.m3u', { selector: '.label' })
  })
})

describe('a11y P0.7 — Fuentes: nombre accesible por fila y por acción', () => {
  it('la fila (li) lleva un aria-label con la etiqueta de SU fuente, no un nombre genérico compartido', async () => {
    const fuentes = vi.fn(async () => [
      fuenteDePrueba('f1', { label: 'IPTV-org · México' }),
      fuenteDePrueba('f2', { label: 'IPTV-org · España' }),
    ])
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const lista = await screen.findByRole('list', { name: t('fuentes.etiquetaLista') })
    const filas = within(lista).getAllByRole('listitem')
    expect(filas.map((f) => f.getAttribute('aria-label')).sort()).toEqual(
      ['IPTV-org · España', 'IPTV-org · México'].sort(),
    )
  })

  it('«Re-sincronizar» y «Quitar» incluyen el label de la fuente en su nombre accesible (ej. "Quitar IPTV-org · México")', async () => {
    const fuentes = vi.fn(async () => [fuenteDePrueba('f1', { label: 'IPTV-org · México' })])
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    expect(
      await screen.findByRole('button', { name: 'Re-sincronizar IPTV-org · México' }),
    ).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Quitar IPTV-org · México' })).toBeTruthy()
  })

  it('el par «¿Seguro? Quitar» / «Cancelar» de la confirmación en dos pasos son <button> nativos alcanzables por teclado', async () => {
    const fuentes = vi.fn(async () => [fuenteDePrueba('f1', { label: 'Mi lista' })])
    const { fireEvent } = await import('@testing-library/svelte')
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const quitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(quitar)

    const confirmar = screen.getByRole('button', { name: t('fuentes.quitar.confirmar.etiqueta', { label: 'Mi lista' }) })
    const cancelar = screen.getByRole('button', { name: t('fuentes.quitar.cancelar') })
    for (const boton of [confirmar, cancelar] as HTMLButtonElement[]) {
      expect(boton.tagName).toBe('BUTTON')
      expect(boton.disabled).toBe(false)
      expect(boton.getAttribute('tabindex')).toBeNull()
    }
  })

  it('el bloque de aviso legal es texto plano, sin aria-hidden, sin ocultarse de un lector de pantalla', async () => {
    render(Fuentes, { fuente: fuenteFalsa(), alVolver: vi.fn(), alFuenteAnadida: vi.fn(), alFuentesCambiaron: vi.fn() })

    const titulo = await screen.findByText(t('fuentes.legal.titulo'))
    const bloque = titulo.closest('.legal') as HTMLElement
    expect(bloque).toBeTruthy()
    expect(bloque.getAttribute('aria-hidden')).toBeNull()
    expect(bloque.textContent).toContain(t('fuentes.legal.responsabilidad'))
  })
})

describe('a11y P0.7 — foco al abrir/cerrar la vista Fuentes (#fuentes)', () => {
  it('al montar, el foco se mueve al título de la vista (h2, tabindex=-1) — no se queda en quien la abrió', async () => {
    const disparador = document.createElement('button')
    document.body.appendChild(disparador)
    disparador.focus()
    expect(document.activeElement).toBe(disparador)

    render(Fuentes, { fuente: fuenteFalsa(), alVolver: vi.fn(), alFuenteAnadida: vi.fn(), alFuentesCambiaron: vi.fn() })

    const titulo = await screen.findByRole('heading', { name: t('fuentes.titulo') })
    await vi.waitFor(() => expect(document.activeElement).toBe(titulo))
    expect(titulo.getAttribute('tabindex')).toBe('-1')

    disparador.remove()
  })

  it('al desmontar (Volver), el foco vuelve a quien abrió la vista — no se pierde en <body>', async () => {
    const disparador = document.createElement('button')
    document.body.appendChild(disparador)
    disparador.focus()

    const { unmount } = render(Fuentes, {
      fuente: fuenteFalsa(),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })
    await screen.findByRole('heading', { name: t('fuentes.titulo') })
    await vi.waitFor(() => expect(document.activeElement).not.toBe(disparador))

    unmount()

    expect(document.activeElement).toBe(disparador)
    disparador.remove()
  })
})

describe('a11y P0.7 — BarraLateralFacetas: el buscador de País tiene nombre accesible real', () => {
  const paisesLargos: Faceta[] = Array.from({ length: 15 }, (_, i) => ({ valor: `P${i}`, total: i + 1 }))

  it('el campo de filtrar país es alcanzable por su etiqueta accesible, no solo por placeholder', () => {
    render(BarraLateralFacetas, { paises: paisesLargos, categorias: [], calidades: [] })

    const campo = screen.getByLabelText(t('filtro.filtrarPais')) as HTMLInputElement
    expect(campo.type).toBe('search')
  })

  it('la lista acotada conserva role="button" en cada fila — el recorte es visual (scroll), no de accesibilidad', () => {
    render(BarraLateralFacetas, { paises: paisesLargos, categorias: [], calidades: [] })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    expect(within(grupoPais).getAllByRole('button')).toHaveLength(paisesLargos.length)
  })

  it('el nombre de país localizado no rompe el nombre accesible de la fila (sigue habiendo botón con ese texto)', () => {
    idioma.actual = 'en'
    render(BarraLateralFacetas, {
      paises: [{ valor: 'MX', total: 3 }],
      categorias: [],
      calidades: [],
    })

    const grupoPais = screen.getByRole('group', { name: t('filtro.pais') })
    expect(within(grupoPais).getByRole('button', { name: /Mexico/ })).toBeTruthy()
  })
})

describe('a11y P0.7 — PieDeMarca: enlaces con nombre claro', () => {
  it('los tres enlaces (Korven, Claude Code, código) tienen nombre accesible propio y no comparten uno genérico', () => {
    render(PieDeMarca)

    const enlaces = screen.getAllByRole('link')
    const nombres = enlaces.map((a) => a.textContent?.trim())
    expect(new Set(nombres).size).toBe(enlaces.length) // ninguno duplicado / vacío
    expect(nombres.every((n) => !!n)).toBe(true)
  })
})
