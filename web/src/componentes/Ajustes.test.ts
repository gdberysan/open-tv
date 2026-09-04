import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import Ajustes from './Ajustes.svelte'
import { preferencias } from '../estado/preferencias'
import { idioma, t } from '../i18n'

beforeEach(() => {
  localStorage.clear()
  preferencias.set({ densidad: 'comoda', recordarVista: true, recordarFiltros: false })
  idioma.actual = 'es'
})

describe('Ajustes — densidad', () => {
  it('(b) pulsar «Compacta» escribe preferencias.densidad y refleja el estado activo', async () => {
    render(Ajustes, { alVolver: () => {}, version: 'dev' })

    const comoda = screen.getByRole('button', { name: t('ajustes.densidad.comoda') })
    const compacta = screen.getByRole('button', { name: t('ajustes.densidad.compacta') })
    expect(comoda.getAttribute('aria-pressed')).toBe('true')
    expect(compacta.getAttribute('aria-pressed')).toBe('false')

    await fireEvent.click(compacta)

    expect(get(preferencias).densidad).toBe('compacta')
    expect(compacta.getAttribute('aria-pressed')).toBe('true')
    expect(comoda.getAttribute('aria-pressed')).toBe('false')
  })

  it('pulsar «Cómoda» de vuelta escribe preferencias.densidad = comoda', async () => {
    preferencias.set({ densidad: 'compacta', recordarVista: true, recordarFiltros: false })
    render(Ajustes, { alVolver: () => {}, version: 'dev' })

    const comoda = screen.getByRole('button', { name: t('ajustes.densidad.comoda') })
    await fireEvent.click(comoda)

    expect(get(preferencias).densidad).toBe('comoda')
  })
})

describe('Ajustes — recordar vista / recordar filtros', () => {
  it('recordarVista por defecto está ON (checkbox marcado) y alternar escribe preferencias', async () => {
    render(Ajustes, { alVolver: () => {}, version: 'dev' })

    const casilla = screen.getByRole('checkbox', { name: t('ajustes.recordarVista.titulo') }) as HTMLInputElement
    expect(casilla.checked).toBe(true)

    await fireEvent.click(casilla)
    expect(get(preferencias).recordarVista).toBe(false)
  })

  it('(d) recordarFiltros por defecto está OFF y activarlo escribe preferencias', async () => {
    render(Ajustes, { alVolver: () => {}, version: 'dev' })

    const casilla = screen.getByRole('checkbox', { name: t('ajustes.recordarFiltros.titulo') }) as HTMLInputElement
    expect(casilla.checked).toBe(false)

    await fireEvent.click(casilla)
    expect(get(preferencias).recordarFiltros).toBe(true)
  })
})

describe('Ajustes — Acerca de', () => {
  it('(e) muestra la versión, el aviso legal completo y el crédito de marca', async () => {
    render(Ajustes, { alVolver: () => {}, version: '1.2.3' })

    await screen.findByText(t('ajustes.acerca.version', { version: '1.2.3' }))
    expect(screen.getByText(t('fuentes.legal.reproductor'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.legal.responsabilidad'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.legal.sinDrm'))).not.toBeNull()
    expect(screen.getByText(t('fuentes.legal.sugeridas'))).not.toBeNull()
    expect(screen.getByText(t('pieMarca.desarrolladoPor'), { exact: false })).not.toBeNull()
  })
})

describe('Ajustes — estadísticas locales', () => {
  it('pulsar «Estadísticas locales» llama a alAbrirStats', async () => {
    const alAbrirStats = vi.fn()
    render(Ajustes, { alVolver: () => {}, version: 'dev', alAbrirStats })

    await fireEvent.click(screen.getByRole('button', { name: t('pie.stats') }))
    expect(alAbrirStats).toHaveBeenCalled()
  })
})

describe('Ajustes — foco (mismo patrón que Fuentes)', () => {
  it('(f) al montar, el foco va al título de la vista', async () => {
    render(Ajustes, { alVolver: () => {}, version: 'dev' })

    await vi.waitFor(() => expect(document.activeElement?.textContent).toBe(t('ajustes.titulo')))
  })

  it('(f) al desmontar, el foco vuelve a quien abrió la vista', async () => {
    const disparador = document.createElement('button')
    document.body.appendChild(disparador)
    disparador.focus()

    const { unmount } = render(Ajustes, { alVolver: () => {}, version: 'dev' })
    await vi.waitFor(() => expect(document.activeElement?.textContent).toBe(t('ajustes.titulo')))

    unmount()
    expect(document.activeElement).toBe(disparador)
    disparador.remove()
  })
})

describe('Ajustes — volver', () => {
  it('el botón «Volver» llama a alVolver', async () => {
    let llamado = false
    render(Ajustes, { alVolver: () => (llamado = true), version: 'dev' })

    await fireEvent.click(screen.getByRole('button', { name: t('ajustes.volver') }))
    expect(llamado).toBe(true)
  })
})
