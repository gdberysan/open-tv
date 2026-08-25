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

// Ronda 1 de revisión: un import('hls.js') que resuelve DESPUÉS de que su
// intento ya se declaró fatal no puede pisar hlsActual. Para hacerlo
// observable hace falta retener a propósito la resolución del módulo — sin
// esto el hueco es de un solo microtask y ninguna espera basada en
// temporizadores (vi.waitFor incluido) puede aterrizar dentro: los
// microtasks siempre drenan del todo antes de que corra cualquier timer. La
// fábrica del mock no resuelve hasta que se libera esta puerta a mano; una
// vez liberada queda así para el resto de los tests del fichero (el módulo
// se resuelve una sola vez y se cachea), así que el test de la carrera va
// PRIMERO.
const hlsGate = vi.hoisted(() => {
  let liberar: () => void = () => {}
  const promesa = new Promise<void>((resolver) => {
    liberar = resolver
  })
  return { promesa, liberar: () => liberar() }
})

vi.mock('hls.js', async () => {
  await hlsGate.promesa
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
  // Ver la nota de hlsGate arriba: este test tiene que ir primero.
  it('un import de hls.js tardío de un mirror ya abandonado no pisa el intento vigente', async () => {
    const mirrors: Mirror[] = [
      { url: 'https://muerto/x.m3u8', vivo: true, latenciaMs: 100, webOk: true },
      { url: 'https://vivo/x.m3u8', vivo: true, latenciaMs: 200, webOk: true },
    ]
    const fuente = {
      mirrors: vi.fn(async () => mirrors),
      proxyDisponible: vi.fn(async () => false),
    }
    const intentadas: string[] = []

    const { container } = render(Reproductor, {
      canal,
      fuente: fuente as any,
      alCerrar: () => {},
      alIntentar: (url: string) => intentadas.push(url),
    })

    // El primer intento llega a registrar su listener de error del <video> y
    // su import('hls.js') (todavía pendiente: la puerta sigue cerrada) antes
    // de que el test pueda observar nada más — alIntentar() se llama justo
    // antes, en el mismo tramo síncrono.
    await vi.waitFor(() => expect(intentadas).toEqual(['https://muerto/x.m3u8']))

    // Fatal ANTES de que su propio import('hls.js') resuelva: un evento de
    // error real sobre el <video>, el mismo camino que un fallo de decode
    // real dispararía (onVideoError → guard.alError → alFallar), sin esperar
    // a que hls.js llegue a existir para este intento.
    const video = container.querySelector('video')!
    video.dispatchEvent(new Event('error'))

    // Se libera la puerta: el import pendiente del primer intento resuelve
    // DESPUÉS de que ese intento ya se rechazó.
    hlsGate.liberar()

    // El failover avanza solo: el segundo intento sí construye su hls.
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(hlsState.instancias[0].url).toBe('https://vivo/x.m3u8')
    expect(intentadas).toEqual(['https://muerto/x.m3u8', 'https://vivo/x.m3u8'])

    // El import tardío del mirror muerto no añadió una segunda instancia ni
    // reemplazó la del mirror vivo: sigue habiendo exactamente una, y es la
    // correcta.
    await new Promise((r) => setTimeout(r, 0))
    expect(hlsState.instancias).toHaveLength(1)
    expect(hlsState.instancias[0].url).toBe('https://vivo/x.m3u8')
  })

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
    // queryAllByText (no queryByText): desde el fix round 1 de la Tarea 18
    // el mensaje aparece DOS veces a propósito — el <p class="estado error">
    // visual y la región aria-live persistente y oculta que lo anuncia de
    // forma fiable (ver Reproductor.svelte).
    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.error.noArranco')).length).toBeGreaterThan(0))
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

  // Ronda 1 de revisión: un fetch que falla (mirrors()/destino()/
  // proxyDisponible()) no es "el canal no arrancó" — es exactamente el
  // mismo problema de tres estados que MensajeError.svelte separa para el
  // catálogo (gateway caído / sin red / servidor), y mezclarlos ya costó una
  // tarde de diagnóstico una vez.
  it('un fallo al pedir los mirrors se clasifica (gateway), no el genérico "no arrancó"', async () => {
    const fuente = {
      mirrors: vi.fn(async () => {
        throw new Error('gateway inalcanzable')
      }),
      proxyDisponible: vi.fn(async () => false),
    }

    render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

    // queryAllByText: mismo motivo que arriba — el mensaje aparece por
    // partida doble (visual + región aria-live persistente) desde el fix
    // round 1 de la Tarea 18.
    await vi.waitFor(() => expect(screen.queryAllByText(t('estado.gatewayCaido')).length).toBeGreaterThan(0))
    expect(screen.queryByText(t('reproductor.error.noArranco'))).toBeNull()
  })

  // Ronda 2 de revisión (gate manual en Chrome real): el <video> de la app se
  // quedaba en readyState 0 para siempre en motor nativo — un <video> suelto
  // con la MISMA url y un load() explícito sí cargaba. jsdom no decodifica
  // (canPlayType siempre '' y play() no devuelve Promise), así que aquí se
  // fuerza el motor nativo con spies y se comprueba el CONTRATO de llamadas
  // (load() y play() por intento, con el src ya puesto), no la decodificación
  // — eso es exactamente lo que el gate manual verifica de verdad.
  it('el camino nativo llama a load() y play() por cada intento del failover', async () => {
    const canPlayTypeSpy = vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockReturnValue('maybe')
    // limpiarIntento() TAMBIÉN llama a video.load() (al limpiar, con el src ya
    // vacío) antes de que el intento nativo asigne el nuevo src y llame a SU
    // propio load(). Para no confundir esas dos llamadas, la del fix se
    // distingue registrando qué src tenía el <video> en cada llamada a
    // load(): la de limpiarIntento() ocurre con el src vacío; la del camino
    // nativo ocurre con el src YA puesto al del intento.
    const srcAlLlamarLoad: string[] = []
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(function (this: HTMLVideoElement) {
      srcAlLlamarLoad.push(this.src)
    })
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)

    try {
      const mirrors: Mirror[] = [
        { url: 'https://muerto/x.m3u8', vivo: true, latenciaMs: 100, webOk: true },
        { url: 'https://vivo/x.m3u8', vivo: true, latenciaMs: 200, webOk: true },
      ]
      const fuente = {
        mirrors: vi.fn(async () => mirrors),
        proxyDisponible: vi.fn(async () => false),
      }

      const { container } = render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })
      const video = container.querySelector('video')! as HTMLVideoElement

      // Primer intento: play() confirma que se llegó al final del camino
      // nativo; el ÚLTIMO load() registrado hasta ahora tiene que haber
      // ocurrido con el src YA puesto al mirror muerto (la llamada del fix,
      // no la de limpiarIntento()).
      await vi.waitFor(() => expect(playSpy).toHaveBeenCalledTimes(1))
      expect(video.src).toContain('muerto')
      expect(srcAlLlamarLoad.at(-1)).toContain('muerto')

      // Fatal el primero (mismo camino que un fallo de decode real): el
      // failover repite load()/play() para el segundo intento.
      video.dispatchEvent(new Event('error'))
      await vi.waitFor(() => expect(playSpy).toHaveBeenCalledTimes(2))
      expect(video.src).toContain('vivo')
      expect(srcAlLlamarLoad.at(-1)).toContain('vivo')
    } finally {
      canPlayTypeSpy.mockRestore()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })
})
