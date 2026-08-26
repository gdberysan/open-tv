import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import Onboarding from './Onboarding.svelte'
import type { CatalogSource, Fuente } from '../datos/catalogo'
import { idioma, t } from '../i18n'

// Doble mínimo de CatalogSource: Onboarding solo llama a
// fuentes/anadirFuente/anadirFuenteFichero/fuentesSugeridas — el resto de la
// interfaz no lo necesita, así que no hace falta implementarla entera aquí
// (a diferencia del doble completo de App.integracion.test.ts).
function fuenteDePrueba(id: string, kind: Fuente['kind'] = 'url'): Fuente {
  return { id, label: `Fuente ${id}`, url: `https://ej.test/${id}.m3u`, kind, ultimoSync: null, canales: 0 }
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
    fuentes: vi.fn(async () => []),
    anadirFuente: vi.fn(async (url: string, label?: string) => fuenteDePrueba(label ?? url)),
    anadirFuenteFichero: vi.fn(async (f: File) => fuenteDePrueba(f.name, 'file')),
    quitarFuente: vi.fn(async () => {}),
    resyncFuente: vi.fn(async () => {}),
    fuentesSugeridas: vi.fn(async () => []),
    epgDeCanales: vi.fn(async () => new Map()),
    epgDeCanal: vi.fn(async () => ({ ahora: null, proximos: [] })),
    ...overrides,
  }
}

beforeEach(() => {
  idioma.actual = 'es'
})

describe('Onboarding — añadir por URL', () => {
  it('pegar una URL y pulsar «Añadir» llama a anadirFuente con esa URL', async () => {
    const anadirFuente = vi.fn(async (url: string) => fuenteDePrueba(url))
    const alFuenteAnadida = vi.fn()
    const fuente = fuenteFalsa({ anadirFuente })
    render(Onboarding, { fuente, alFuenteAnadida })

    const campo = screen.getByLabelText(t('onboarding.url.etiqueta')) as HTMLInputElement
    await fireEvent.input(campo, { target: { value: 'https://ejemplo.com/lista.m3u' } })

    const boton = screen.getByRole('button', { name: t('onboarding.anadir') })
    await fireEvent.click(boton)

    await vi.waitFor(() => expect(anadirFuente).toHaveBeenCalledWith('https://ejemplo.com/lista.m3u'))
    await vi.waitFor(() => expect(alFuenteAnadida).toHaveBeenCalledWith(fuenteDePrueba('https://ejemplo.com/lista.m3u')))
  })

  it('el botón «Añadir» empieza deshabilitado (sin URL) y no llama a nada al no poder pulsarse', () => {
    const anadirFuente = vi.fn()
    const fuente = fuenteFalsa({ anadirFuente })
    render(Onboarding, { fuente, alFuenteAnadida: vi.fn() })

    const boton = screen.getByRole('button', { name: t('onboarding.anadir') }) as HTMLButtonElement
    expect(boton.disabled).toBe(true)
    expect(anadirFuente).not.toHaveBeenCalled()
  })
})

describe('Onboarding — añadir por fichero', () => {
  it('elegir un fichero .m3u llama a anadirFuenteFichero con ese fichero', async () => {
    const anadirFuenteFichero = vi.fn(async (f: File) => fuenteDePrueba(f.name, 'file'))
    const alFuenteAnadida = vi.fn()
    const fuente = fuenteFalsa({ anadirFuenteFichero })
    render(Onboarding, { fuente, alFuenteAnadida })

    const campoFichero = screen.getByLabelText(t('onboarding.fichero.etiqueta')) as HTMLInputElement
    const archivo = new File(['#EXTM3U'], 'mi-lista.m3u', { type: 'audio/x-mpegurl' })
    await fireEvent.change(campoFichero, { target: { files: [archivo] } })

    await vi.waitFor(() => expect(anadirFuenteFichero).toHaveBeenCalledWith(archivo))
    await vi.waitFor(() => expect(alFuenteAnadida).toHaveBeenCalledWith(fuenteDePrueba('mi-lista.m3u', 'file')))
  })
})

describe('Onboarding — sugeridas de un toque', () => {
  it('pinta las sugeridas de fuentesSugeridas() y un tap llama a anadirFuente con esa URL y label', async () => {
    const sugeridas = [{ label: 'iptv-org (deportes)', url: 'https://ej.test/deportes.m3u' }]
    const anadirFuente = vi.fn(async (url: string, label?: string) => fuenteDePrueba(label ?? url))
    const alFuenteAnadida = vi.fn()
    const fuente = fuenteFalsa({ fuentesSugeridas: vi.fn(async () => sugeridas), anadirFuente })
    render(Onboarding, { fuente, alFuenteAnadida })

    const boton = await screen.findByRole('button', { name: /iptv-org \(deportes\)/ })
    await fireEvent.click(boton)

    await vi.waitFor(() =>
      expect(anadirFuente).toHaveBeenCalledWith('https://ej.test/deportes.m3u', 'iptv-org (deportes)'),
    )
    await vi.waitFor(() => expect(alFuenteAnadida).toHaveBeenCalled())
  })

  it('sin sugeridas no pinta el bloque «Sugeridas»', async () => {
    const fuente = fuenteFalsa({ fuentesSugeridas: vi.fn(async () => []) })
    render(Onboarding, { fuente, alFuenteAnadida: vi.fn() })

    await vi.waitFor(() => expect(fuente.fuentesSugeridas).toHaveBeenCalled())
    expect(screen.queryByText(t('onboarding.sugeridas.titulo'))).toBeNull()
  })
})

describe('Onboarding — disclaimer', () => {
  it('muestra la línea de disclaimer', () => {
    render(Onboarding, { fuente: fuenteFalsa(), alFuenteAnadida: vi.fn() })
    expect(screen.getByText(t('onboarding.disclaimer'))).not.toBeNull()
  })
})
