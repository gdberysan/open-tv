import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import Fuentes from './Fuentes.svelte'
import type { CatalogSource, Fuente } from '../datos/catalogo'
import { idioma, t } from '../i18n'

// Doble mínimo de CatalogSource: Fuentes solo llama a
// fuentes/resyncFuente/quitarFuente (más lo que AnadirFuente necesita) — el
// resto de la interfaz no lo usa este componente.
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
})

afterEach(() => {
  vi.useRealTimers()
})

describe('Fuentes — lista', () => {
  it('(a) pinta una fila por fuente con label, url, kind, sincronía relativa y nº de canales', async () => {
    const ahora = Math.floor(Date.now() / 1000)
    const fuentes = vi.fn(async () => [
      fuenteDePrueba('f1', { label: 'IPTV-org · México', url: 'https://ej.test/mx.m3u', kind: 'url', ultimoSync: ahora - 300, canales: 42 }),
      fuenteDePrueba('f2', { label: '', url: 'https://ej.test/sin-label.m3u', kind: 'file', ultimoSync: null, canales: 0 }),
    ])
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    await screen.findByText('IPTV-org · México')
    expect(screen.getByText('https://ej.test/mx.m3u')).not.toBeNull()
    expect(screen.getByText(t('fuentes.kind.url'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.kind.file'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.relativo.minutos', { n: 5 }))).not.toBeNull()
    expect(screen.getByText(t('fuentes.canales', { n: 42 }))).not.toBeNull()

    // Fallback de label: la URL cuando el label viene vacío.
    expect(screen.getByText('https://ej.test/sin-label.m3u', { selector: '.label' })).not.toBeNull()
    expect(screen.getByText(t('fuentes.nuncaSincronizada'))).not.toBeNull()
  })

  it('lista vacía muestra el mensaje honesto, sin filas', async () => {
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes: vi.fn(async () => []) }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    await screen.findByText(t('fuentes.vacia'))
    expect(screen.queryByRole('list', { name: t('fuentes.etiquetaLista') })).toBeNull()
  })

  it('un fallo al cargar fuentes() muestra el mensaje de error', async () => {
    render(Fuentes, {
      fuente: fuenteFalsa({
        fuentes: vi.fn(async () => {
          throw new Error('boom')
        }),
      }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    await screen.findByText(t('fuentes.error'))
  })
})

describe('Fuentes — re-sincronizar', () => {
  it('(b) pulsar «Re-sincronizar» llama a resyncFuente(id) con el nombre accesible correcto', async () => {
    const resyncFuente = vi.fn(async () => {})
    render(Fuentes, {
      fuente: fuenteFalsa({
        fuentes: vi.fn(async () => [fuenteDePrueba('f1', { label: 'IPTV-org · México' })]),
        resyncFuente,
      }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const boton = await screen.findByRole('button', { name: t('fuentes.resincronizar.etiqueta', { label: 'IPTV-org · México' }) })
    await fireEvent.click(boton)

    await vi.waitFor(() => expect(resyncFuente).toHaveBeenCalledWith('f1'))
  })

  it('tras re-sincronizar, muestra un estado breve y luego refetcha fuentes()', async () => {
    vi.useFakeTimers()
    const fuentes = vi.fn(async () => [fuenteDePrueba('f1', { label: 'Mi lista' })])
    const resyncFuente = vi.fn(async () => {})
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes, resyncFuente }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    await vi.advanceTimersByTimeAsync(0) // carga inicial
    const boton = screen.getByRole('button', { name: t('fuentes.resincronizar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(boton)
    await vi.advanceTimersByTimeAsync(0) // resyncFuente resuelve

    expect(screen.getByText(t('fuentes.resincronizando'))).not.toBeNull()
    expect(fuentes).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(3000) // espera documentada antes del refetch
    expect(fuentes).toHaveBeenCalledTimes(2)
  })

  // F6 (fix final-review): el setTimeout de resync() no se trackeaba, así
  // que desmontar la vista a mitad de la espera de 3s no lo cancelaba — el
  // timer seguía vivo y, al disparar, escribía sobre un componente ya fuera.
  // Falsable: sin clearTimeout en onDestroy, fuentes() se seguiría llamando
  // una segunda vez pasados los 3s aunque el componente ya no exista.
  it('F6: desmontar durante la espera de resync() cancela el temporizador — no refetchea fuentes() tras desmontar', async () => {
    vi.useFakeTimers()
    const fuentes = vi.fn(async () => [fuenteDePrueba('f1', { label: 'Mi lista' })])
    const resyncFuente = vi.fn(async () => {})
    const { unmount } = render(Fuentes, {
      fuente: fuenteFalsa({ fuentes, resyncFuente }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    await vi.advanceTimersByTimeAsync(0) // carga inicial
    const boton = screen.getByRole('button', { name: t('fuentes.resincronizar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(boton)
    await vi.advanceTimersByTimeAsync(0) // resyncFuente resuelve, arranca el timer de 3s

    const llamadasAntesDeDesmontar = fuentes.mock.calls.length
    unmount()

    await vi.advanceTimersByTimeAsync(5000) // de sobra para los 3s documentados
    expect(fuentes.mock.calls.length).toBe(llamadasAntesDeDesmontar)
  })
})

describe('Fuentes — quitar (confirmación en dos pasos)', () => {
  it('(c) el primer clic NO llama a quitarFuente — solo pide confirmación', async () => {
    const quitarFuente = vi.fn(async () => {})
    render(Fuentes, {
      fuente: fuenteFalsa({
        fuentes: vi.fn(async () => [fuenteDePrueba('f1', { label: 'IPTV-org · México' })]),
        quitarFuente,
      }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const botonQuitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'IPTV-org · México' }) })
    await fireEvent.click(botonQuitar)

    expect(quitarFuente).not.toHaveBeenCalled()
    await screen.findByRole('button', { name: t('fuentes.quitar.confirmar.etiqueta', { label: 'IPTV-org · México' }) })
  })

  it('confirmar SÍ llama a quitarFuente(id)', async () => {
    const quitarFuente = vi.fn(async () => {})
    render(Fuentes, {
      fuente: fuenteFalsa({
        fuentes: vi.fn(async () => [fuenteDePrueba('f1', { label: 'Mi lista' })]),
        quitarFuente,
      }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const botonQuitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(botonQuitar)
    const confirmar = await screen.findByRole('button', { name: t('fuentes.quitar.confirmar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(confirmar)

    await vi.waitFor(() => expect(quitarFuente).toHaveBeenCalledWith('f1'))
  })

  it('«Cancelar» vuelve al botón normal sin llamar a quitarFuente', async () => {
    const quitarFuente = vi.fn(async () => {})
    render(Fuentes, {
      fuente: fuenteFalsa({
        fuentes: vi.fn(async () => [fuenteDePrueba('f1', { label: 'Mi lista' })]),
        quitarFuente,
      }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const botonQuitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(botonQuitar)
    const cancelar = await screen.findByRole('button', { name: t('fuentes.quitar.cancelar') })
    await fireEvent.click(cancelar)

    expect(quitarFuente).not.toHaveBeenCalled()
    await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Mi lista' }) })
  })

  it('(d) tras quitar con éxito, refetcha fuentes() y notifica alFuentesCambiaron con la lista fresca', async () => {
    const fuentes = vi.fn()
    fuentes
      .mockResolvedValueOnce([fuenteDePrueba('f1', { label: 'Mi lista' })]) // carga inicial
      .mockResolvedValueOnce([]) // tras quitar, cero fuentes
    const quitarFuente = vi.fn(async () => {})
    const alFuentesCambiaron = vi.fn()
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes, quitarFuente }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron,
    })

    const botonQuitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(botonQuitar)
    const confirmar = await screen.findByRole('button', { name: t('fuentes.quitar.confirmar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(confirmar)

    await vi.waitFor(() => expect(fuentes).toHaveBeenCalledTimes(2))
    await vi.waitFor(() => expect(alFuentesCambiaron).toHaveBeenCalledWith([]))
    await screen.findByText(t('fuentes.vacia'))
  })

  // F6 (fix final-review): antes, quitar() tenía un try/finally SIN catch —
  // un quitarFuente() rechazado salía como un rechazo sin manejar de un
  // onclick async (vitest lo reporta como "unhandled rejection" si no se
  // captura aquí) y no dejaba NINGÚN rastro visible.
  it('F6: un quitarFuente() fallido muestra un error visible (no un rechazo sin manejar) y NO notifica alFuentesCambiaron', async () => {
    const quitarFuente = vi.fn(async () => {
      throw new Error('boom')
    })
    const alFuentesCambiaron = vi.fn()
    render(Fuentes, {
      fuente: fuenteFalsa({
        fuentes: vi.fn(async () => [fuenteDePrueba('f1', { label: 'Mi lista' })]),
        quitarFuente,
      }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron,
    })

    const botonQuitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(botonQuitar)
    const confirmar = await screen.findByRole('button', { name: t('fuentes.quitar.confirmar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(confirmar)

    await screen.findByText(t('fuentes.quitar.error'))
    expect(alFuentesCambiaron).not.toHaveBeenCalled()
    // La fila sigue ahí: un quitar fallido no la hace desaparecer de la lista.
    expect(screen.getByText('Mi lista')).not.toBeNull()
  })

  it('F6: un fuentes() de refetch fallido tras un quitar exitoso también muestra un error visible', async () => {
    const fuentes = vi.fn()
    fuentes
      .mockResolvedValueOnce([fuenteDePrueba('f1', { label: 'Mi lista' })]) // carga inicial
      .mockRejectedValueOnce(new Error('boom')) // refetch tras quitar
    const quitarFuente = vi.fn(async () => {})
    render(Fuentes, {
      fuente: fuenteFalsa({ fuentes, quitarFuente }),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const botonQuitar = await screen.findByRole('button', { name: t('fuentes.quitar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(botonQuitar)
    const confirmar = await screen.findByRole('button', { name: t('fuentes.quitar.confirmar.etiqueta', { label: 'Mi lista' }) })
    await fireEvent.click(confirmar)

    await vi.waitFor(() => expect(quitarFuente).toHaveBeenCalledWith('f1'))
    await screen.findByText(t('fuentes.error'))
  })
})

describe('Fuentes — añadir más (bloque compartido)', () => {
  it('el bloque de añadir (AnadirFuente) está presente y llama a anadirFuente con la URL', async () => {
    const anadirFuente = vi.fn(async (url: string) => fuenteDePrueba('nueva', { url, label: url }))
    const alFuenteAnadida = vi.fn()
    render(Fuentes, {
      fuente: fuenteFalsa({ anadirFuente }),
      alVolver: vi.fn(),
      alFuenteAnadida,
      alFuentesCambiaron: vi.fn(),
    })

    const campo = await screen.findByLabelText(t('onboarding.url.etiqueta'))
    await fireEvent.input(campo, { target: { value: 'https://ejemplo.com/otra.m3u' } })
    const boton = screen.getByRole('button', { name: t('onboarding.anadir') })
    await fireEvent.click(boton)

    await vi.waitFor(() => expect(anadirFuente).toHaveBeenCalledWith('https://ejemplo.com/otra.m3u'))
    await vi.waitFor(() => expect(alFuenteAnadida).toHaveBeenCalled())
  })

  it('el campo de URL de AnadirFuente NO recibe el foco automático en esta vista', async () => {
    render(Fuentes, {
      fuente: fuenteFalsa(),
      alVolver: vi.fn(),
      alFuenteAnadida: vi.fn(),
      alFuentesCambiaron: vi.fn(),
    })

    const campo = await screen.findByLabelText(t('onboarding.url.etiqueta'))
    expect(document.activeElement).not.toBe(campo)
  })
})

describe('Fuentes — disclaimer legal', () => {
  it('(e) pinta el bloque legal completo, incluida la línea de responsabilidad del usuario', async () => {
    render(Fuentes, { fuente: fuenteFalsa(), alVolver: vi.fn(), alFuenteAnadida: vi.fn(), alFuentesCambiaron: vi.fn() })

    await screen.findByText(t('fuentes.legal.titulo'))
    expect(screen.getByText(t('fuentes.legal.reproductor'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.legal.responsabilidad'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.legal.sinDrm'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.legal.sugeridas'))).not.toBeNull()
  })
})

describe('Fuentes — volver', () => {
  it('el botón «Volver» llama a alVolver', async () => {
    const alVolver = vi.fn()
    render(Fuentes, { fuente: fuenteFalsa(), alVolver, alFuenteAnadida: vi.fn(), alFuentesCambiaron: vi.fn() })

    await fireEvent.click(screen.getByRole('button', { name: t('fuentes.volver') }))
    expect(alVolver).toHaveBeenCalled()
  })
})
