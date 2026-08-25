import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import Reproductor from './Reproductor.svelte'
import { t } from '../i18n'
import type { Canal, Mirror } from '../datos/catalogo'

// jsdom no decodifica HLS de verdad: canPlayType() no está implementado (así
// que motorDelNavegador siempre elige 'hlsjs' aquí) y no hay MediaSource, así
// que hls.js real se declararía "no soportado" al instante en TODOS los
// intentos por igual — no deja distinguir un mirror muerto de uno vivo, y
// esconde el momento exacto en el que el failover cruza de uno a otro tras
// resolverse en microtasks, antes de que el test pueda observar nada
// intermedio. Por eso se mockea 'hls.js' con un doble mínimo: control total
// de CUÁNDO cada intento falla, igual que los tests del guard controlan el
// tiempo con temporizadores falsos. Es la SECUENCIA que produce
// planDeFailover + el avance de índice lo que se prueba aquí, no la
// decodificación real (eso es el e2e/gate manual con ffmpeg).
const hlsState = vi.hoisted(() => ({
  instancias: [] as Array<{ url: string; fallar: (motivo: string) => void }>,
}))

vi.mock('hls.js', () => {
  class FakeHls {
    static Events = { ERROR: 'error', MANIFEST_PARSED: 'manifest_parsed' } as const
    static isSupported() {
      return true
    }
    private oyentes: Record<string, Array<(...a: unknown[]) => void>> = {}
    on(evento: string, cb: (...a: unknown[]) => void) {
      ;(this.oyentes[evento] ??= []).push(cb)
    }
    loadSource(url: string) {
      hlsState.instancias.push({
        url,
        fallar: (motivo: string) => {
          this.oyentes['error']?.forEach((cb) => cb('error', { type: 'otherError', details: motivo }))
        },
      })
    }
    attachMedia() {}
    destroy() {}
  }
  return { default: FakeHls }
})

const canal = { id: 'c1', nombre: 'X', webOk: true } as Canal

describe('Reproductor — failover entre mirrors', () => {
  it('cae al siguiente mirror cuando el primero falla, y solo muestra error al agotar todos', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://muerto/x.m3u8', vivo: true, latenciaMs: 100, webOk: true },
      { url: 'https://vivo/x.m3u8', vivo: true, latenciaMs: 200, webOk: true },
    ]
    const fuente = {
      mirrors: vi.fn(async () => mirrors),
      proxyDisponible: vi.fn(async () => false),
    }
    const intentadas: string[] = []

    render(Reproductor, {
      canal,
      fuente: fuente as any,
      alCerrar: () => {},
      alIntentar: (url: string) => intentadas.push(url),
    })

    // Primer intento: el mirror muerto. Nada se ha declarado fatal todavía.
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(intentadas).toEqual(['https://muerto/x.m3u8'])
    expect(screen.queryByText(t('reproductor.error.noArranco'))).toBeNull()

    hlsState.instancias[0].fallar('manifestLoadError')

    // El failover cruza al segundo mirror ANTES de mostrar cualquier error.
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(2))
    expect(intentadas).toEqual(['https://muerto/x.m3u8', 'https://vivo/x.m3u8'])
    expect(screen.queryByText(t('reproductor.error.noArranco'))).toBeNull()

    hlsState.instancias[1].fallar('manifestLoadError')

    // Los dos mirrors se agotaron: ahora sí se muestra el error, y no antes.
    await vi.waitFor(() => expect(screen.queryByText(t('reproductor.error.noArranco'))).not.toBeNull())
    expect(intentadas).toEqual(['https://muerto/x.m3u8', 'https://vivo/x.m3u8'])
    expect(fuente.mirrors).toHaveBeenCalledWith('c1')
  })

  it('sin mirrors, cae al destino() único como compatibilidad', async () => {
    hlsState.instancias.length = 0
    const fuente = {
      mirrors: vi.fn(async () => []),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }
    const intentadas: string[] = []

    render(Reproductor, {
      canal,
      fuente: fuente as any,
      alCerrar: () => {},
      alIntentar: (url: string) => intentadas.push(url),
    })

    await vi.waitFor(() => expect(intentadas).toEqual(['https://unico/x.m3u8']))
    expect(fuente.destino).toHaveBeenCalledWith('c1')
  })
})
