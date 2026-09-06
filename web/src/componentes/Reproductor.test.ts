import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import { get } from 'svelte/store'
import { tick } from 'svelte'
import Reproductor from './Reproductor.svelte'
import { t } from '../i18n'
import { urlProxy } from '../reproductor/plan'
import type { DesenlaceReproduccion } from '../reproductor/failover'
import type { Canal, Mirror, Programa } from '../datos/catalogo'
import { favoritos } from '../estado/favoritos'
import { formatearHoraLocal } from '../lib/hora'
import { noCasteaPorFormato, marcarFalloFormato } from '../estado/castFallidos'

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
  instancias: [] as Array<{
    url: string
    /** Error FATAL: el que de verdad termina un intento en hls.js. */
    fallar: (motivo: string) => void
    /** Error NO fatal: hls.js los emite a montones en directos sanos y se
     *  recupera solo. No debe matar el intento. */
    fallarNoFatal: (motivo: string) => void
    /** Error fatal CON status HTTP, que es lo que clasificarFallo necesita para
     *  distinguir un 404 ('caducado') de un fallo sin señal ('desconocido'). */
    fallarConStatus: (motivo: string, status: number) => void
    progreso: () => void
    /** Emite BUFFER_CODECS con las pistas que "vio" el demuxer. */
    pistas: (o: { video: boolean; audio: boolean }) => void
  }>,
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
    static Events = {
      ERROR: 'error',
      MANIFEST_PARSED: 'manifest_parsed',
      FRAG_LOADED: 'frag_loaded',
      BUFFER_APPENDED: 'buffer_appended',
      BUFFER_CODECS: 'buffer_codecs',
    } as const
    static isSupported() {
      return true
    }
    private oyentes: Record<string, Array<(...a: unknown[]) => void>> = {}
    on(evento: string, cb: (...a: unknown[]) => void) {
      ;(this.oyentes[evento] ??= []).push(cb)
    }
    loadSource(url: string) {
      const emitirError = (motivo: string, fatal: boolean, status?: number) => {
        this.oyentes['error']?.forEach((cb) =>
          cb('error', {
            type: 'otherError',
            details: motivo,
            fatal,
            ...(status === undefined ? {} : { response: { code: status } }),
          }),
        )
      }
      hlsState.instancias.push({
        url,
        fallar: (motivo: string) => emitirError(motivo, true),
        fallarNoFatal: (motivo: string) => emitirError(motivo, false),
        fallarConStatus: (motivo: string, status: number) => emitirError(motivo, true, status),
        progreso: () => this.oyentes['frag_loaded']?.forEach((cb) => cb('frag_loaded', {})),
        pistas: (o: { video: boolean; audio: boolean }) =>
          this.oyentes['buffer_codecs']?.forEach((cb) =>
            cb('buffer_codecs', {
              ...(o.video ? { video: { container: 'video/mp4', codec: 'avc1.64001f' } } : {}),
              ...(o.audio ? { audio: { container: 'audio/mpeg', codec: '' } } : {}),
            }),
          ),
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
      alIntentar: (url: string) => intentadas.push(url),
    })

    await vi.waitFor(() => expect(intentadas).toEqual(['https://unico/x.m3u8']))
    expect(fuente.destino).toHaveBeenCalledWith('c1')
  })

  // Fix final, hallazgo 2: el fallback legacy sin mirrors etiquetaba
  // viaProxy por POSICIÓN (i > 0), no por si la url es de verdad la
  // proxeada. Un plan SOLO-proxy (aquí, webOk=false + proxyDisponible=true)
  // tiene su ÚNICA url en i=0 — "i > 0" la etiquetaba como 'directo' en las
  // stats, cuando sí es por proxy. Contra el código viejo, este test
  // fallaría (via sería 'directo').
  it('plan legacy solo-proxy (sin mirrors, webOk=false) reporta via="proxy", no "directo"', async () => {
    hlsState.instancias.length = 0
    const fuente = {
      mirrors: vi.fn(async () => []),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => true),
    }
    const desenlaces: DesenlaceReproduccion[] = []

    render(Reproductor, {
      canal: { ...canal, webOk: false },
      fuente: fuente as any,
      alDesenlace: (d) => desenlaces.push(d),
    })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    // El único intento YA es la url proxeada (planDeReproduccion con
    // webOk=false devuelve solo [urlProxy(url)]).
    expect(hlsState.instancias[0].url).toBe(urlProxy('https://unico/x.m3u8'))

    hlsState.instancias[0].fallar('manifestLoadError')

    await vi.waitFor(() => expect(desenlaces.length).toBeGreaterThan(0))
    expect(desenlaces[0].via).toBe('proxy')
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

    render(Reproductor, { canal, fuente: fuente as any })

    // queryAllByText: mismo motivo que arriba — el mensaje aparece por
    // partida doble (visual + región aria-live persistente) desde el fix
    // round 1 de la Tarea 18.
    await vi.waitFor(() => expect(screen.queryAllByText(t('estado.gatewayCaido')).length).toBeGreaterThan(0))
    expect(screen.queryByText(t('reproductor.error.noArranco'))).toBeNull()
  })

  // Tarea 14 (P0.6): el error terminal de agotar el failover, cuando el
  // canal SÍ tenía mirrors, ofrece un CTA que reanuda el MISMO mecanismo
  // (reproducir()) en vez de ser un callejón sin salida.
  it('agotados los mirrors, el error dice cuántos se probaron y el clic reintenta la cadena', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://muerto/x.m3u8', vivo: true, latenciaMs: 100, webOk: true },
      { url: 'https://tambien-muerto/x.m3u8', vivo: true, latenciaMs: 200, webOk: true },
    ]
    const fuente = {
      mirrors: vi.fn(async () => mirrors),
      proxyDisponible: vi.fn(async () => false),
    }

    render(Reproductor, { canal, fuente: fuente as any })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].fallar('manifestLoadError')
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(2))
    hlsState.instancias[1].fallar('manifestLoadError')

    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.error.noArranco')).length).toBeGreaterThan(0))

    // El texto informativo cuenta los mirrors PROBADOS (2), que es lo que de
    // verdad pasó: la cadena los agotó. getByText ya lanza si no lo encuentra.
    screen.getByText(t('reproductor.error.mirrorsProbados', { n: 2 }))
    const boton = screen.getByRole('button', { name: t('reproductor.error.reintentar') })

    hlsState.instancias.length = 0
    boton.click()

    // Reanuda el MISMO mecanismo: vuelve a pedir mirrors() (2ª vez) y
    // arranca de nuevo desde el primer intento de la cadena.
    await vi.waitFor(() => expect(fuente.mirrors).toHaveBeenCalledTimes(2))
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(hlsState.instancias[0].url).toBe('https://muerto/x.m3u8')
  })

  // Sin mirrors (fallback de compatibilidad al destino único): no hay cadena
  // que anunciar, pero reintentar sigue teniendo sentido.
  it('sin mirrors, no se anuncia recuento pero sí se puede reintentar', async () => {
    hlsState.instancias.length = 0
    const fuente = {
      mirrors: vi.fn(async () => []),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }

    render(Reproductor, { canal, fuente: fuente as any })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].fallar('manifestLoadError')

    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.error.noArranco')).length).toBeGreaterThan(0))
    // El CTA ahora es «Reintentar» y se ofrece SIEMPRE que haya error: un
    // usuario delante de un fallo sin ninguna acción es peor que uno que puede
    // volver a intentarlo. Lo que NO debe salir es el recuento de mirrors,
    // porque aquí no hubo cadena que recorrer.
    expect(screen.getByRole('button', { name: t('reproductor.error.reintentar') })).toBeTruthy()
    expect(screen.queryByText(t('reproductor.error.mirrorsProbados', { n: 1 }))).toBeNull()
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

      const { container } = render(Reproductor, { canal, fuente: fuente as any })
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

  it('salta los mirrors con codecOk === false y prueba solo los demás', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://mpeg2/x.m3u8', vivo: true, latenciaMs: 100, webOk: true, codecOk: false, codecs: 'mpeg2video,mp2' },
      { url: 'https://h264/x.m3u8', vivo: true, latenciaMs: 200, webOk: true, codecOk: null },
    ]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const intentadas: string[] = []
    render(Reproductor, { canal, fuente: fuente as any, alIntentar: (url: string) => intentadas.push(url) })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(intentadas).toEqual(['https://h264/x.m3u8'])
  })

  it('con todos los mirrors indecodificables muestra el mensaje de códec al instante, sin intentar ni botón de reintento', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://mpeg2/x.m3u8', vivo: true, latenciaMs: 100, webOk: false, codecOk: false, codecs: 'mpeg2video,mp2' },
      { url: 'https://hevc/x.m3u8', vivo: true, latenciaMs: 500, webOk: false, codecOk: false, codecs: 'hevc,aac' },
    ]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => true) }
    const desenlaces: DesenlaceReproduccion[] = []
    const intentadas: string[] = []
    render(Reproductor, {
      canal,
      fuente: fuente as any,
      alIntentar: (url: string) => intentadas.push(url),
      alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d),
    })

    const esperado = t('reproductor.error.codec', { codecs: 'mpeg2video,mp2' })
    await vi.waitFor(() => expect(screen.queryAllByText(esperado).length).toBeGreaterThan(0))
    expect(intentadas).toEqual([])
    expect(hlsState.instancias).toHaveLength(0)
    expect(screen.queryByText(t('reproductor.error.reintentar'))).toBeNull()
    expect(screen.queryByText(t('reproductor.error.mirrorsProbados', { n: 2 }))).toBeNull()
    expect(desenlaces).toEqual([
      { canalId: 'c1', resultado: 'fallo', motivo: 'codec', motor: 'hlsjs', via: 'ninguna', mirrorIndex: 0 },
    ])
  })

  it('si el servidor no sabía y hls.js solo vio audio, el fallo se clasifica como codec', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [{ url: 'https://mpeg2/x.m3u8', vivo: true, latenciaMs: 100, webOk: true }]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const desenlaces: DesenlaceReproduccion[] = []
    render(Reproductor, { canal, fuente: fuente as any, alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d) })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].pistas({ video: false, audio: true })
    // El intento muere como muere de verdad: sin vídeo nunca avanza y el
    // guard lo declara fatal. Aquí se fuerza con un fatal cualquiera.
    hlsState.instancias[0].fallar('bufferStalledError')

    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.error.codecGenerico')).length).toBeGreaterThan(0))
    expect(desenlaces.at(-1)?.motivo).toBe('codec')
  })

  it('con pista de vídeo vista, un fallo NO es codec', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [{ url: 'https://h264/x.m3u8', vivo: true, latenciaMs: 100, webOk: true }]
    const fuente = { mirrors: vi.fn(async () => mirrors), proxyDisponible: vi.fn(async () => false) }
    const desenlaces: DesenlaceReproduccion[] = []
    render(Reproductor, { canal, fuente: fuente as any, alDesenlace: (d: DesenlaceReproduccion) => desenlaces.push(d) })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].pistas({ video: true, audio: true })
    hlsState.instancias[0].fallar('bufferStalledError')

    await vi.waitFor(() => expect(desenlaces).toHaveLength(1))
    expect(desenlaces[0].motivo).toBe('inestable')
    expect(screen.queryByText(t('reproductor.error.codecGenerico'))).toBeNull()
  })
})

// Tarea 1 (P0.8): overlay 1b sobre el <video> — insignia, línea meta, nombre
// y controles etiquetados (Silenciar/Favorito). No usa el mock de hls.js de
// arriba: sin mirrors, cae al camino de destino() único (motor nativo en
// jsdom), que no exige que el stream real llegue a confirmar para que el
// overlay ya esté en el DOM (se monta con el resto del marcado del
// reproductor, no tras alConfirmar()).
describe('Reproductor — overlay 1b', () => {
  beforeEach(() => {
    // El store favoritos es un singleton de módulo: sin esto, un canal
    // marcado como favorito en un test anterior seguiría marcado aquí.
    favoritos.set(new Set())
  })

  function fuenteSinMirrors() {
    return {
      mirrors: vi.fn(async () => [] as Mirror[]),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }
  }

  it('muestra la insignia «En vivo», el nombre del canal y la línea meta cuando el canal trae resolución/latencia/país', () => {
    const canalConMeta: Canal = { ...canal, nombre: 'BBC One 1080p', pais: 'GB', latenciaMs: 150 }
    const { container } = render(Reproductor, { canal: canalConMeta, fuente: fuenteSinMirrors() as any })

    expect(screen.getByText(t('reproductor.envivo'))).toBeTruthy()
    expect(container.querySelector('.overlay-nombre')?.textContent).toBe('BBC One 1080p')
    expect(container.querySelector('.overlay-meta')?.textContent).toBe('1080p · 150 ms · GB')
  })

  it('sin resolución/latencia/país en el canal, la línea meta no se muestra', () => {
    const canalSinMeta: Canal = { ...canal, nombre: 'X', pais: '', latenciaMs: 0 }
    const { container } = render(Reproductor, { canal: canalSinMeta, fuente: fuenteSinMirrors() as any })

    expect(container.querySelector('.overlay-meta')).toBeNull()
  })

  it('el botón Silenciar del overlay togglea video.muted y aria-pressed', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
    const video = container.querySelector('video') as HTMLVideoElement
    const boton = container.querySelector('.overlay-controles .silenciar') as HTMLButtonElement

    expect(boton.getAttribute('aria-label')).toBe(t('reproductor.silenciar'))
    expect(boton.getAttribute('aria-pressed')).toBe('false')
    expect(video.muted).toBe(false)

    boton.click()
    await tick()

    expect(boton.getAttribute('aria-pressed')).toBe('true')
    expect(video.muted).toBe(true)
  })

  it('el botón Favorito del overlay togglea el store de favoritos y aria-pressed', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
    const boton = container.querySelector('.overlay-controles .favorito') as HTMLButtonElement

    expect(boton.getAttribute('aria-pressed')).toBe('false')
    expect(boton.getAttribute('aria-label')).toBe(t('canal.favorito.anadir'))
    expect(get(favoritos).has(canal.id)).toBe(false)

    boton.click()
    await tick()

    expect(boton.getAttribute('aria-pressed')).toBe('true')
    expect(boton.getAttribute('aria-label')).toBe(t('canal.favorito.quitar'))
    expect(get(favoritos).has(canal.id)).toBe(true)
  })

  it('los controles nuevos del overlay tienen nombre accesible y están dentro del contenedor del panel', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
    const panel = container.querySelector('.reproductor[role="region"]')
    expect(panel).toBeTruthy()

    const silenciar = panel!.querySelector('.overlay-controles .silenciar')
    const favorito = panel!.querySelector('.overlay-controles .favorito')

    expect(silenciar).toBeTruthy()
    expect(favorito).toBeTruthy()
    expect(silenciar?.getAttribute('aria-label')).toBeTruthy()
    expect(favorito?.getAttribute('aria-label')).toBeTruthy()
  })
})

// Tarea 2 (P0.8): pantalla completa sobre el CONTENEDOR (no el <video>) +
// Picture-in-Picture. jsdom no implementa ninguna de las dos APIs, así que
// cada test mockea a mano los métodos/propiedades que necesita sobre
// document/HTMLVideoElement.prototype/HTMLDivElement.prototype — y los
// restaura en un finally, porque estos son parches sobre PROTOTIPOS
// compartidos entre tests (a diferencia de $state, que es por instancia).
describe('Reproductor — pantalla completa y Picture-in-Picture', () => {
  function fuenteSinMirrors() {
    return {
      mirrors: vi.fn(async () => [] as Mirror[]),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }
  }

  beforeEach(() => {
    favoritos.set(new Set())
  })

  it('el botón de pantalla completa llama a requestFullscreen() sobre el CONTENEDOR del panel, no sobre el <video>', async () => {
    const fullscreenEnabledDesc = Object.getOwnPropertyDescriptor(Document.prototype, 'fullscreenEnabled')
    Object.defineProperty(document, 'fullscreenEnabled', { value: true, configurable: true })
    const contenedorSpy = vi.fn()
    const videoSpy = vi.fn()
    HTMLDivElement.prototype.requestFullscreen = contenedorSpy
    HTMLVideoElement.prototype.requestFullscreen = videoSpy

    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
      const boton = container.querySelector('.overlay-controles .pantalla-completa') as HTMLButtonElement
      expect(boton).toBeTruthy()

      boton.click()

      expect(contenedorSpy).toHaveBeenCalledTimes(1)
      expect(videoSpy).not.toHaveBeenCalled()
    } finally {
      // @ts-expect-error limpieza del parche de prototipo
      delete HTMLDivElement.prototype.requestFullscreen
      // @ts-expect-error limpieza del parche de prototipo
      delete HTMLVideoElement.prototype.requestFullscreen
      if (fullscreenEnabledDesc) Object.defineProperty(document, 'fullscreenEnabled', fullscreenEnabledDesc)
    }
  })

  it('el botón de pantalla completa refleja el estado vía fullscreenchange (aria-pressed + etiqueta)', async () => {
    Object.defineProperty(document, 'fullscreenEnabled', { value: true, configurable: true })
    HTMLDivElement.prototype.requestFullscreen = vi.fn()
    document.exitFullscreen = vi.fn().mockResolvedValue(undefined)

    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
      const contenedor = container.querySelector('.reproductor') as HTMLElement
      const boton = container.querySelector('.overlay-controles .pantalla-completa') as HTMLButtonElement

      expect(boton.getAttribute('aria-pressed')).toBe('false')
      expect(boton.getAttribute('aria-label')).toBe(t('reproductor.pantallaCompleta.entrar'))

      // jsdom no actualiza document.fullscreenElement solo con el click; se
      // simula el navegador entrando en pantalla completa y disparando el
      // evento, que es lo que el componente escucha para reflejar el estado.
      Object.defineProperty(document, 'fullscreenElement', { value: contenedor, configurable: true })
      document.dispatchEvent(new Event('fullscreenchange'))
      await tick()

      expect(boton.getAttribute('aria-pressed')).toBe('true')
      expect(boton.getAttribute('aria-label')).toBe(t('reproductor.pantallaCompleta.salir'))

      Object.defineProperty(document, 'fullscreenElement', { value: null, configurable: true })
      document.dispatchEvent(new Event('fullscreenchange'))
      await tick()

      expect(boton.getAttribute('aria-pressed')).toBe('false')
      expect(boton.getAttribute('aria-label')).toBe(t('reproductor.pantallaCompleta.entrar'))
    } finally {
      // @ts-expect-error limpieza del parche de prototipo
      delete HTMLDivElement.prototype.requestFullscreen
      // @ts-expect-error limpieza del parche de prototipo
      delete document.exitFullscreen
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.fullscreenElement
    }
  })

  it('con document.fullscreenElement activo, Escape sale de pantalla completa (única función de Esc en el panel)', async () => {
    const contenedorDeMentira = document.createElement('div')
    Object.defineProperty(document, 'fullscreenElement', { value: contenedorDeMentira, configurable: true })
    document.exitFullscreen = vi.fn().mockResolvedValue(undefined)

    try {
      render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })

      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

      expect(document.exitFullscreen).toHaveBeenCalledTimes(1)
    } finally {
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.fullscreenElement
      // @ts-expect-error limpieza del parche de prototipo
      delete document.exitFullscreen
    }
  })

  it('el botón de PiP solo se renderiza si document.pictureInPictureEnabled es true', async () => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { value: false, configurable: true })
    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
      expect(container.querySelector('.overlay-controles .pip')).toBeNull()
    } finally {
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.pictureInPictureEnabled
    }
  })

  it('con PiP soportado, el botón se renderiza y al pulsarlo llama a video.requestPictureInPicture()', async () => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { value: true, configurable: true })
    const requestPipSpy = vi.fn().mockResolvedValue(undefined)
    HTMLVideoElement.prototype.requestPictureInPicture = requestPipSpy

    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
      const boton = container.querySelector('.overlay-controles .pip') as HTMLButtonElement
      expect(boton).toBeTruthy()
      expect(boton.getAttribute('aria-label')).toBe(t('reproductor.pip.activar'))

      boton.click()
      await tick()

      expect(requestPipSpy).toHaveBeenCalledTimes(1)
    } finally {
      // @ts-expect-error limpieza del parche de prototipo
      delete HTMLVideoElement.prototype.requestPictureInPicture
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.pictureInPictureEnabled
    }
  })

  it('al destruirse el componente estando en PiP, se llama a document.exitPictureInPicture()', async () => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { value: true, configurable: true })
    const exitPipSpy = vi.fn().mockResolvedValue(undefined)
    document.exitPictureInPicture = exitPipSpy

    try {
      const { container, unmount } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
      const video = container.querySelector('video') as HTMLVideoElement

      // Simula que el navegador entró en PiP: dispara el evento nativo que el
      // componente escucha sobre el propio <video> para reflejar estaEnPiP.
      video.dispatchEvent(new Event('enterpictureinpicture'))
      await tick()

      unmount()

      expect(exitPipSpy).toHaveBeenCalledTimes(1)
    } finally {
      // @ts-expect-error limpieza del parche de prototipo
      delete document.exitPictureInPicture
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.pictureInPictureEnabled
    }
  })
})

// Tarea 9 (P2, EPG): now/next en el overlay del reproductor. A diferencia de
// TarjetaCanal (Tarea 8), aquí "sin guía" es EXPLÍCITO — el overlay muestra
// el texto de i18n en vez de quedarse limpio, porque estar viendo un canal
// sin saber si hay guía o no es una situación distinta a hojear el catálogo.
describe('Reproductor — EPG ahora/después en el overlay', () => {
  beforeEach(() => {
    favoritos.set(new Set())
  })

  function fuenteConEpg(epgDeCanal: (id: string) => Promise<{ ahora: Programa | null; proximos: Programa[] }>) {
    return {
      mirrors: vi.fn(async () => [] as Mirror[]),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
      epgDeCanal: vi.fn(epgDeCanal),
    }
  }

  it('con guía (ahora + próximo), el overlay muestra "Ahora: <título> (HH:MM–HH:MM)" y "Sig: <título>"', async () => {
    const ahora: Programa = { titulo: 'Telediario', inicioSeg: 1_700_000_000, finSeg: 1_700_003_000 }
    const siguiente: Programa = { titulo: 'El Tiempo', inicioSeg: 1_700_003_000, finSeg: 1_700_007_000 }
    const fuente = fuenteConEpg(async () => ({ ahora, proximos: [siguiente] }))

    const { container } = render(Reproductor, { canal, fuente: fuente as any })

    await vi.waitFor(() => expect(fuente.epgDeCanal).toHaveBeenCalledWith('c1'))

    const horaAhora = `${formatearHoraLocal(ahora.inicioSeg)}–${formatearHoraLocal(ahora.finSeg)}`
    await vi.waitFor(() => expect(container.textContent).toContain(`${t('epg.ahora')}: Telediario (${horaAhora})`))
    expect(container.textContent).toContain(`${t('epg.siguiente')}: El Tiempo`)
    expect(container.textContent).not.toContain(t('epg.sinGuia'))
  })

  it('sin guía (ahora=null y sin próximos), el overlay muestra el mensaje explícito «sin guía para esta fuente»', async () => {
    const fuente = fuenteConEpg(async () => ({ ahora: null, proximos: [] }))

    const { container } = render(Reproductor, { canal, fuente: fuente as any })

    await vi.waitFor(() => expect(container.textContent).toContain(t('epg.sinGuia')))
  })

  it('un fallo de epgDeCanal no rompe el reproductor: sigue montado, sin el mensaje de "sin guía" (eso es para una respuesta confirmada, no un fallo de red)', async () => {
    const fuente = fuenteConEpg(async () => {
      throw new Error('epg inalcanzable')
    })

    const { container } = render(Reproductor, { canal, fuente: fuente as any })

    await vi.waitFor(() => expect(fuente.epgDeCanal).toHaveBeenCalled())
    // Deja pasar el rechazo de la promesa antes de comprobar que no rompió nada.
    await new Promise((r) => setTimeout(r, 0))

    expect(container.querySelector('.reproductor')).toBeTruthy()
    expect(container.textContent).not.toContain(t('epg.sinGuia'))
  })

  it('al abrir otro canal (no un mirror del mismo), se vuelve a pedir la guía y no se pinta la del canal anterior', async () => {
    const epgPorCanal: Record<string, { ahora: Programa | null; proximos: Programa[] }> = {
      c1: { ahora: { titulo: 'Programa Uno', inicioSeg: 1_700_000_000, finSeg: 1_700_003_000 }, proximos: [] },
      c2: { ahora: { titulo: 'Programa Dos', inicioSeg: 1_700_000_000, finSeg: 1_700_003_000 }, proximos: [] },
    }
    const fuente = fuenteConEpg(async (id) => epgPorCanal[id])

    const { container, rerender } = render(Reproductor, { canal, fuente: fuente as any })
    await vi.waitFor(() => expect(container.textContent).toContain('Programa Uno'))

    const canal2 = { ...canal, id: 'c2', nombre: 'Y' } as Canal
    rerender({ canal: canal2, fuente: fuente as any })

    await vi.waitFor(() => expect(container.textContent).toContain('Programa Dos'))
    expect(container.textContent).not.toContain('Programa Uno')
  })

  it('mientras el overlay sigue en el mismo canal, refresca la guía por intervalo (Ahora no envejece en una sesión larga)', async () => {
    vi.useFakeTimers()
    try {
      const fuente = fuenteConEpg(async () => ({
        ahora: { titulo: 'En directo', inicioSeg: 1_700_000_000, finSeg: 1_700_003_000 },
        proximos: [],
      }))
      render(Reproductor, { canal, fuente: fuente as any })

      // Deja resolver la petición inicial (al abrir el canal).
      await vi.advanceTimersByTimeAsync(0)
      const inicial = fuente.epgDeCanal.mock.calls.length
      expect(inicial).toBeGreaterThanOrEqual(1)

      // Avanza un intervalo de refresco (2 min): se vuelve a pedir la guía del
      // MISMO canal, sin que el usuario haga nada.
      await vi.advanceTimersByTimeAsync(2 * 60_000)
      expect(fuente.epgDeCanal.mock.calls.length).toBeGreaterThan(inicial)
    } finally {
      vi.useRealTimers()
    }
  })
})

// Reproductor-primero (spec §4): modo panel — el mismo motor, sin envoltorio
// modal. El modo 'modal' por defecto conserva el comportamiento de siempre
// (todos los describes de arriba); este describe ejercita SOLO lo que cambia
// con modo='panel'.
describe('Reproductor — modo panel (reproductor-primero)', () => {
  beforeEach(() => {
    favoritos.set(new Set())
  })

  function fuenteSinMirrors() {
    return {
      mirrors: vi.fn(async () => [] as Mirror[]),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }
  }

  it('en modo panel no hay role=dialog ni aria-modal: es una region etiquetada con el canal', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
    expect(container.querySelector('[role="dialog"]')).toBeNull()
    expect(container.querySelector('[aria-modal]')).toBeNull()
    expect(screen.getByRole('region', { name: canal.nombre })).toBeTruthy()
  })

  it('Escape SIN pantalla completa no hace nada: el panel sigue montado (ya no hay modal que cerrar)', async () => {
    render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(screen.getByRole('region', { name: canal.nombre })).toBeTruthy()
  })

  it('con activo=false el teclado global del reproductor queda inerte (espacio no reproduce/pausa)', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, activo: false })
    const video = container.querySelector('video') as HTMLVideoElement
    const pause = vi.spyOn(video, 'pause').mockImplementation(() => {})
    const play = vi.spyOn(video, 'play').mockResolvedValue()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }))
    expect(pause).not.toHaveBeenCalled()
    expect(play).not.toHaveBeenCalled()
  })

  it('en modo panel, una tecla con el foco en un control interactivo AJENO se ignora; sin objetivo interactivo, sí actúa', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
    const video = container.querySelector('video') as HTMLVideoElement
    const pause = vi.spyOn(video, 'pause').mockImplementation(() => {})
    const play = vi.spyOn(video, 'play').mockResolvedValue()

    // Espacio con el foco en un <input> ajeno (el buscador de la lateral):
    // el evento burbujea hasta window con target=input → se ignora.
    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()
    input.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))
    expect(pause).not.toHaveBeenCalled()
    expect(play).not.toHaveBeenCalled()
    input.remove()

    // El mismo espacio despachado sin objetivo interactivo sí llega al
    // reproductor (video.paused=true en jsdom → play).
    window.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }))
    expect(play).toHaveBeenCalledTimes(1)
  })

  it('silenciadoInicial arranca muted y muestra la CTA; pulsarla activa el sonido y la retira', async () => {
    const { container } = render(Reproductor, {
      canal,
      fuente: fuenteSinMirrors() as any,
      silenciadoInicial: true,
    })
    const video = container.querySelector('video') as HTMLVideoElement
    expect(video.muted).toBe(true)
    const cta = screen.getByRole('button', { name: t('reproductor.activarSonido') })
    cta.click()
    await tick()
    expect(video.muted).toBe(false)
    expect(screen.queryByRole('button', { name: t('reproductor.activarSonido') })).toBeNull()
  })

  it('cambiar de canal con la CTA visible activa el sonido solo (elegir canal ES el primer gesto)', async () => {
    const fuente = fuenteSinMirrors() as any
    const { container, rerender } = render(Reproductor, {
      canal,
      fuente,
      silenciadoInicial: true,
    })
    const video = container.querySelector('video') as HTMLVideoElement
    expect(video.muted).toBe(true)

    await rerender({ canal: { ...canal, id: 'c2', nombre: 'Otro' } as Canal, fuente, silenciadoInicial: true })
    await tick()

    expect(video.muted).toBe(false)
    expect(screen.queryByRole('button', { name: t('reproductor.activarSonido') })).toBeNull()
  })

  it('en modo panel el botón AirPlay vive en el overlay y no hay barra inferior con Cerrar', () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any })
      const airplay = screen.getByRole('button', { name: 'AirPlay' })
      expect(airplay.closest('.overlay-controles')).toBeTruthy()
      expect(container.querySelector('.controles')).toBeNull()
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
    }
  })
})

// Tarea 3 (spec 2026-09-03): sesión de cast AirPlay — el evento REAL de
// WebKit (webkitcurrentplaybacktargetiswirelesschanged) engancha a
// iniciarCast()/pararCast(), que fuerzan motorForzado='nativo' en
// reproducir() en vez de dejar que motorDelNavegador() decida (siempre
// 'hlsjs' en jsdom, sin canPlayType real). Reusa reproducir()/intentar()/
// PlaybackGuard/clasificarFallo tal cual — no se duplica su lógica aquí.
describe('Reproductor — sesión de cast AirPlay (Tarea 3)', () => {
  beforeEach(() => {
    favoritos.set(new Set())
    // castFallidos usa localStorage: sin limpiarlo, un canal marcado "sin
    // formato" por un test de este describe (mismo canal.id 'c1' en todo el
    // fichero) seguiría marcado en el siguiente.
    localStorage.clear()
  })

  function fuenteSinMirrors() {
    return {
      mirrors: vi.fn(async () => [] as Mirror[]),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }
  }

  it('el evento webkitcurrentplaybacktargetiswirelesschanged fuerza el motor nativo, sin hls.js', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    // El intento inicial de montaje (sin cast) va por 'hlsjs' en jsdom
    // (canPlayType sin mockear), que SÍ llama a load()/play() reales del
    // <video> vía hls.js — no hace falta espiarlos para ese camino. El
    // camino nativo que fuerza el cast sí los llama de verdad (rama
    // motor==='nativo' de intentar()); jsdom no implementa
    // HTMLMediaElement.play() (rechaza con "not implemented"), así que se
    // espía aquí igual que ya hace el test del camino nativo del failover
    // más arriba en este fichero.
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })

      // Deja asentar el intento inicial (motor 'hlsjs' sin forzar nada
      // todavía) para no contar SU import('hls.js') como parte de la
      // sesión de cast que este test quiere aislar.
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))

      // Motor nativo: video.src se asigna directo (rama motor==='nativo' de
      // intentar()), NUNCA pasa por el import('hls.js') mockeado de este
      // fichero — si hls.js se hubiera instanciado, hlsState tendría una
      // entrada.
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))
      expect(hlsState.instancias.length).toBe(0)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('agotar los intentos en modo cast NO muestra mensajeError y reanuda hls.js', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })

      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))

      // Motor nativo: el fallo se señala con el evento 'error' nativo del
      // <video>, código 4 = MEDIA_ERR_SRC_NOT_SUPPORTED (jsdom no reproduce
      // vídeo de verdad, así que se dispara a mano).
      Object.defineProperty(video, 'error', { value: { code: 4 }, configurable: true })
      video.dispatchEvent(new Event('error'))

      // Sin más mirrors tras el fallo nativo: reproducir() cae al bloque de
      // cast y llama reproducir() de nuevo — esta vez SIN motorForzado, así
      // que motorDelNavegador(video) decide, y en jsdom (sin canPlayType
      // real) eso es 'hlsjs'. hls.js mockeado registra una instancia nueva.
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      // No hay tarjeta de error a pantalla completa (.estado.error): un
      // fallo del motor nativo forzado por cast reanuda en local sin
      // mostrarla.
      expect(container.querySelector('.estado.error')).toBeNull()

      // clasificarFallo({ mediaErrorCode: 4 }) === 'formato' (código
      // SRC_NOT_SUPPORTED): la Task 3 solo recuerda "este canal no castea"
      // para fallos de formato/códec, nunca para uno de red/timeout
      // transitorio — este es el lado POSITIVO de esa distinción.
      expect(noCasteaPorFormato(canal.id)).toBe(true)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  // Lado NEGATIVO de la misma distinción: un fallo de RED (código 2 =
  // MEDIA_ERR_NETWORK, clasificarFallo lo mapea a 'caido', no 'formato') es
  // transitorio — el mirror pudo estar caído un instante, no es que el
  // canal sea incompatible con AirPlay. marcarFalloFormato() NO debe
  // llamarse, así que noCasteaPorFormato() sigue en false tras agotar el
  // intento nativo forzado.
  it('agotar los intentos en modo cast por un fallo de RED (no formato) no marca el canal como sin-cast', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })

      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))

      // Código 2 = MEDIA_ERR_NETWORK — clasificarFallo() lo mapea a 'caido',
      // NO a 'formato' (ver diagnostico.ts).
      Object.defineProperty(video, 'error', { value: { code: 2 }, configurable: true })
      video.dispatchEvent(new Event('error'))

      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      expect(container.querySelector('.estado.error')).toBeNull()
      expect(noCasteaPorFormato(canal.id)).toBe(false)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })
})

// Tarea 4 (spec 2026-09-03): superficie visible de la sesión de cast — el
// panel que reemplaza la vista de vídeo, y las dos guardas de teclado
// (estadoCast !== 'idle' apaga 'm'/'f', porque el MISMO <video> alimenta al
// TV mientras dura). No se duplica la lógica de iniciarCast()/pararCast()
// (Tarea 3): estos tests solo verifican lo que la Tarea 4 añadió.
describe('Reproductor — interfaz de cast (Tarea 4)', () => {
  beforeEach(() => {
    favoritos.set(new Set())
    localStorage.clear()
  })

  function fuenteSinMirrors() {
    return {
      mirrors: vi.fn(async () => [] as Mirror[]),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }
  }

  it('cambiar de canal durante un cast activo sigue emitiendo, sin volver a elegir dispositivo', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      // destino varía por id (a diferencia de fuenteSinMirrors de arriba):
      // hace falta distinguir la url del canal A de la del canal B para
      // comprobar que el efecto de cambio de canal (canal.id) reanuda con
      // el destino nuevo sin pasar por abrirSelectorAirplay() de nuevo.
      const destino = vi.fn(async (id: string) => ({ url: `https://unico/${id}.m3u8`, airplayOk: null }))
      const fuente = {
        mirrors: vi.fn(async () => [] as Mirror[]),
        destino,
        proxyDisponible: vi.fn(async () => false),
      }
      const canalA = { ...canal, id: 'a' } as Canal
      const canalB = { ...canal, id: 'b' } as Canal
      const { container, rerender } = render(Reproductor, { canal: canalA, fuente: fuente as any })

      // Deja asentar el intento inicial (motor 'hlsjs', sin cast todavía)
      // antes de forzar el cast — mismo patrón que los tests de la Tarea 3.
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.waitFor(() => expect(video.src).toContain('a.m3u8'))

      await rerender({ canal: canalB, fuente: fuente as any })

      // El $effect existente de cambio de canal (canal.id) llama
      // limpiarIntento()+reproducir() igual que siempre — motorForzado NO se
      // tocó, así que el canal nuevo también arranca en motor nativo, sin
      // que el test dispare el evento de WebKit de nuevo (no hay un segundo
      // paso por el selector).
      await vi.waitFor(() => expect(video.src).toContain('b.m3u8'))
      expect(hlsState.instancias.length).toBe(0)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it("'m' no silencia mientras hay una sesión de AirPlay activa", async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })

      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))

      const mutedAntes = video.muted
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'm' }))
      await tick()
      expect(video.muted).toBe(mutedAntes)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it("'f' no expande a pantalla completa mientras hay una sesión de AirPlay activa", async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    // Mismo patrón que el describe de "pantalla completa y Picture-in-Picture"
    // más arriba en este fichero: soportaFullscreen se lee de
    // document.fullscreenEnabled al montar, y alternarPantallaCompleta()
    // llama requestFullscreen() sobre el CONTENEDOR (HTMLDivElement), no
    // sobre el <video>.
    Object.defineProperty(document, 'fullscreenEnabled', { value: true, configurable: true })
    const requestFullscreenSpy = vi.fn()
    HTMLDivElement.prototype.requestFullscreen = requestFullscreenSpy
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })

      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))

      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f' }))
      await tick()
      expect(requestFullscreenSpy).not.toHaveBeenCalled()
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      // @ts-expect-error limpieza del parche de prototipo
      delete HTMLDivElement.prototype.requestFullscreen
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('el botón AirPlay se marca activo (ámbar, aria-pressed) mientras se emite', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })

      const airplay = screen.getByRole('button', { name: t('reproductor.airplay') })
      expect(airplay.getAttribute('aria-pressed')).toBe('false')
      expect(airplay.classList.contains('activo')).toBe(false)

      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))
      await tick()

      expect(airplay.getAttribute('aria-pressed')).toBe('true')
      expect(airplay.classList.contains('activo')).toBe(true)
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('pararCast() muestra el panel "Emitiendo…" mientras estadoCast === \'emitiendo\' y el botón "Dejar de emitir" lo cierra', async () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })

      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitCurrentPlaybackTargetIsWireless?: boolean
      }
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        value: true,
        configurable: true,
      })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))

      // Deja que el guard nativo confirme para llegar a estadoCast ===
      // 'emitiendo' — 'conectando' se pinta como "cargando", no como el
      // panel de cast (Step 3 de esta tarea). El guard confirma cuando la
      // POSICIÓN AVANZA entre dos timeupdate (guard.ts: alPosicion), no con
      // un único evento — jsdom no avanza currentTime solo, así que se
      // mockea a mano igual que webkitCurrentPlaybackTargetIsWireless arriba.
      let posicion = 0
      Object.defineProperty(video, 'currentTime', { get: () => posicion, configurable: true })
      video.dispatchEvent(new Event('timeupdate'))
      posicion = 1
      video.dispatchEvent(new Event('timeupdate'))
      // getByText: hay DOS nodos con este texto a propósito (el panel
      // visible Y la región sr-only que lo reusa, Step 4) — se comprueba el
      // panel visible directamente por selector en vez de por texto.
      await vi.waitFor(() => expect(container.querySelector('.estado.cast')).toBeTruthy())
      expect(container.querySelector('.estado.cast .mensaje')?.textContent).toBe(
        t('reproductor.cast.emitiendo', { canal: canal.nombre }),
      )

      const parar = screen.getByRole('button', { name: t('reproductor.cast.parar') })
      hlsState.instancias.length = 0
      parar.click()
      await tick()

      // El panel de "Emitiendo…" se fue (ya no hay botón de parar) y en su
      // sitio queda el aviso transitorio VISIBLE de fin de emisión — fix de
      // revisión final: antes ese aviso solo existía en la región sr-only.
      expect(screen.queryByRole('button', { name: t('reproductor.cast.parar') })).toBeNull()
      expect(container.querySelector('.estado.cast.aviso .mensaje')?.textContent).toBe(t('reproductor.cast.terminada'))
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })
})

// Revisión final de rama (fix wave, spec 2026-09-03): cuatro agujeros que las
// revisiones por tarea no vieron.
//   1. Terminar un cast revertía el MOTOR pero nunca soltaba la RUTA AirPlay:
//      el <video> seguía remitiendo al TV con una fuente MSE que AirPlay no
//      sabe reproducir (spec §2.1) — negro en el TV y en local a la vez.
//   2. El aviso de fallo/fin de cast solo existía en la región sr-only: un
//      usuario que ve no se enteraba de nada.
//   3. Las guardas de 'm'/'f' solo cubrían el teclado; los botones seguían
//      clicables con el ratón.
//   4. iniciarCast() no tenía guarda de reentrancia: un segundo evento de
//      ruta inalámbrica reiniciaba el stream en plena emisión.
describe('Reproductor — fixes de la revisión final del cast', () => {
  beforeEach(() => {
    favoritos.set(new Set())
    localStorage.clear()
  })

  function fuenteSinMirrors() {
    return {
      mirrors: vi.fn(async () => [] as Mirror[]),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }
  }

  /** disableRemotePlayback no lo implementa jsdom, y aunque lo hiciera no
   *  habría televisor que desconectar: lo que se puede verificar aquí es el
   *  CONTRATO (se pone a true y se devuelve a false en la misma pasada), que
   *  es justo lo que la Remote Playback API define como "terminar la sesión
   *  remota sin suprimir la función entera". Se instala un descriptor propio
   *  sobre la instancia para registrar la SECUENCIA de asignaciones. */
  function espiarRutaRemota(video: HTMLVideoElement) {
    const cambios: boolean[] = []
    let valor = false
    Object.defineProperty(video, 'disableRemotePlayback', {
      configurable: true,
      get: () => valor,
      set: (v: boolean) => {
        valor = v
        cambios.push(v)
      },
    })
    return cambios
  }

  function conAirplayDisponible() {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    return () => delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
  }

  /** Monta el reproductor, deja asentar el intento local inicial y arranca
   *  una sesión de cast disparando el evento REAL de WebKit — el mismo
   *  preámbulo que ya usan los tests de las Tareas 3 y 4. */
  async function montarYCastear(fuente: unknown, canalUsado: Canal = canal) {
    const vista = render(Reproductor, { canal: canalUsado, fuente: fuente as any })
    await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
    hlsState.instancias.length = 0
    const video = vista.container.querySelector('video') as HTMLVideoElement
    Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', { value: true, configurable: true })
    video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
    await vi.waitFor(() => expect(video.src).toContain('.m3u8'))
    return { ...vista, video }
  }

  // --- 1. La ruta AirPlay se suelta de verdad, en los TRES caminos de salida ---

  it('«Dejar de emitir» suelta la ruta AirPlay (disableRemotePlayback true→false), no solo el motor', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const { container, video } = await montarYCastear(fuenteSinMirrors())
      const cambios = espiarRutaRemota(video)

      // Confirmar la reproducción nativa para llegar a 'emitiendo' (el guard
      // confirma cuando la POSICIÓN AVANZA entre dos timeupdate).
      let posicion = 0
      Object.defineProperty(video, 'currentTime', { get: () => posicion, configurable: true })
      video.dispatchEvent(new Event('timeupdate'))
      posicion = 1
      video.dispatchEvent(new Event('timeupdate'))
      await vi.waitFor(() => expect(container.querySelector('.estado.cast')).toBeTruthy())

      screen.getByRole('button', { name: t('reproductor.cast.parar') }).click()
      await tick()

      expect(cambios).toEqual([true, false])
      // Vuelve a false SIEMPRE: dejarlo en true no desconectaría "esta"
      // sesión, suprimiría la función entera y el botón 📺 dejaría de servir.
      expect(video.disableRemotePlayback).toBe(false)
    } finally {
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('agotar los intentos en modo cast suelta la ruta AirPlay antes de reanudar en local', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const { video } = await montarYCastear(fuenteSinMirrors())
      const cambios = espiarRutaRemota(video)

      Object.defineProperty(video, 'error', { value: { code: 4 }, configurable: true })
      video.dispatchEvent(new Event('error'))

      // Reanuda en local (hls.js) — y por el camino soltó la ruta: sin esto,
      // el TV se quedaba enganchado a una fuente MSE que no puede reproducir.
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      expect(cambios).toEqual([true, false])
      expect(video.disableRemotePlayback).toBe(false)
    } finally {
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('un canal ya marcado «no castea por formato» suelta la ruta en vez de dejar el TV colgado', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      marcarFalloFormato(canal.id)
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement
      const cambios = espiarRutaRemota(video)
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', { value: true, configurable: true })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()

      // Salida temprana: ni motor nativo ni sesión de cast — pero la ruta ya
      // estaba activa (por eso llegó el evento), así que hay que soltarla.
      expect(cambios).toEqual([true, false])
      expect(container.querySelector('.estado.cast.aviso .mensaje')?.textContent).toBe(
        t('reproductor.cast.noDisponible'),
      )
      expect(screen.getByRole('button', { name: t('reproductor.airplay') }).getAttribute('aria-pressed')).toBe('false')
    } finally {
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  // --- 2. El aviso de cast se VE, y no se queda pegado ---

  it('el aviso de un cast fallido se pinta VISIBLE (gana a «cargando») y se borra solo a los pocos segundos', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.useFakeTimers()
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })
      await vi.advanceTimersByTimeAsync(0)

      const video = container.querySelector('video') as HTMLVideoElement
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', { value: true, configurable: true })
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await vi.advanceTimersByTimeAsync(0)
      expect(video.src).toContain('x.m3u8')

      Object.defineProperty(video, 'error', { value: { code: 4 }, configurable: true })
      video.dispatchEvent(new Event('error'))
      await vi.advanceTimersByTimeAsync(0)

      // Visible de verdad, no solo en la región sr-only.
      expect(container.querySelector('.estado.cast.aviso .mensaje')?.textContent).toBe(t('reproductor.cast.fallo'))
      // Y gana a la rama `cargando`, que reproducir() acaba de poner a true al
      // reanudar en local: con el orden natural del {#if} el aviso no se
      // llegaría a ver NUNCA (<p class="estado"> es el marcado de esa rama).
      expect(container.querySelector('p.estado')).toBeNull()

      // No se queda pegado: pasado su plazo desaparece y vuelve la UI normal.
      await vi.advanceTimersByTimeAsync(5_000)
      expect(container.querySelector('.estado.cast.aviso')).toBeNull()
      expect(container.querySelector('p.estado')).toBeTruthy()
    } finally {
      vi.useRealTimers()
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('abrir otro canal se lleva el aviso de cast del canal anterior (nada de mensajes zombis)', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container, video, rerender } = await montarYCastear(fuente)

      Object.defineProperty(video, 'error', { value: { code: 4 }, configurable: true })
      video.dispatchEvent(new Event('error'))
      await vi.waitFor(() => expect(container.querySelector('.estado.cast.aviso')).toBeTruthy())

      await rerender({ canal: { ...canal, id: 'c2', nombre: 'Otro' } as Canal, fuente: fuente as any })
      await tick()

      expect(container.querySelector('.estado.cast.aviso')).toBeNull()
      // La región assertive tampoco arrastra el texto viejo: era justo la vía
      // por la que un aviso caducado volvía a alertar más tarde.
      expect(container.textContent).not.toContain(t('reproductor.cast.fallo'))
    } finally {
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  // --- 3. Los BOTONES de silenciar y pantalla completa, no solo las teclas ---

  it('los botones Silenciar y Pantalla completa quedan inertes (disabled) mientras hay una sesión de cast', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    Object.defineProperty(document, 'fullscreenEnabled', { value: true, configurable: true })
    const requestFullscreenSpy = vi.fn()
    HTMLDivElement.prototype.requestFullscreen = requestFullscreenSpy
    try {
      const { container, video } = await montarYCastear(fuenteSinMirrors())

      const silenciar = container.querySelector('.overlay-controles .silenciar') as HTMLButtonElement
      const pantallaCompleta = container.querySelector('.overlay-controles .pantalla-completa') as HTMLButtonElement
      expect(silenciar.disabled).toBe(true)
      expect(pantallaCompleta.disabled).toBe(true)

      // Y el clic del RATÓN tampoco hace nada: la guarda vive dentro de
      // alternarSilencio()/alternarPantallaCompleta(), así que cubre a
      // cualquier llamador, no solo al atajo de teclado.
      const mutedAntes = video.muted
      silenciar.click()
      pantallaCompleta.click()
      await tick()
      expect(video.muted).toBe(mutedAntes)
      expect(requestFullscreenSpy).not.toHaveBeenCalled()

      // Al terminar la emisión vuelven a estar operativos: la guarda es
      // temporal, no una jubilación. (Hasta aquí la sesión estaba en
      // 'conectando' — ya bloqueaba; se confirma la reproducción nativa para
      // llegar a 'emitiendo', que es cuando existe el botón de parar.)
      let posicion = 0
      Object.defineProperty(video, 'currentTime', { get: () => posicion, configurable: true })
      video.dispatchEvent(new Event('timeupdate'))
      posicion = 1
      video.dispatchEvent(new Event('timeupdate'))
      await vi.waitFor(() => expect(container.querySelector('.estado.cast')).toBeTruthy())
      expect(silenciar.disabled).toBe(true)

      screen.getByRole('button', { name: t('reproductor.cast.parar') }).click()
      await tick()
      expect((container.querySelector('.overlay-controles .silenciar') as HTMLButtonElement).disabled).toBe(false)
      expect((container.querySelector('.overlay-controles .pantalla-completa') as HTMLButtonElement).disabled).toBe(
        false,
      )
    } finally {
      limpiarAirplay()
      // @ts-expect-error limpieza del parche de prototipo
      delete HTMLDivElement.prototype.requestFullscreen
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.fullscreenEnabled
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  // --- 4. Reentrancia de iniciarCast() ---

  it('un segundo evento de ruta inalámbrica con la sesión ya en marcha no reinicia el stream', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { video } = await montarYCastear(fuente)

      // Cada reproducir() pide destino() una vez y cada intento pasa por
      // limpiarIntento()+load(): si iniciarCast() volviera a entrar, ambos
      // contadores subirían y el TV vería el stream reiniciarse.
      const destinosAntes = fuente.destino.mock.calls.length
      const loadsAntes = loadSpy.mock.calls.length

      // Cambio de dispositivo a mitad de sesión / re-disparo espurio: el
      // evento vuelve con la ruta todavía inalámbrica.
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()
      await new Promise((r) => setTimeout(r, 0))

      expect(fuente.destino.mock.calls.length).toBe(destinosAntes)
      expect(loadSpy.mock.calls.length).toBe(loadsAntes)
      expect(hlsState.instancias.length).toBe(0)
    } finally {
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  // --- 5. Bucle real en hardware (2026-09-04): confirmado con trace, no con
  //     sospecha — 5213 ciclos en 23 s sin llegar NUNCA a 'emitiendo'. Dos
  //     causas, dos tests. ---

  it('pulsar el botón AirPlay arranca el motor nativo antes de que llegue ningún evento de ruta', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitShowPlaybackTargetPicker?: () => void
      }
      video.webkitShowPlaybackTargetPicker = vi.fn()

      screen.getByRole('button', { name: t('reproductor.airplay') }).click()

      // Sin haber disparado NINGÚN webkitcurrentplaybacktargetiswirelesschanged
      // todavía: el motor nativo ya debería estar cargando — es la condición
      // que el spike original tenía y el flujo reactivo viejo no reproducía
      // (preparaba el nativo DESPUÉS de que la ruta se activase, no antes).
      await vi.waitFor(() => expect(video.src).toContain('x.m3u8'))
      expect(hlsState.instancias.length).toBe(0)
      expect(video.webkitShowPlaybackTargetPicker).toHaveBeenCalledTimes(1)
    } finally {
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('cancelar el selector (ningún evento de ruta llega nunca) reanuda en local pasado el plazo', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.useFakeTimers()
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })
      await vi.advanceTimersByTimeAsync(0)
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement & {
        webkitShowPlaybackTargetPicker?: () => void
      }
      video.webkitShowPlaybackTargetPicker = vi.fn()

      screen.getByRole('button', { name: t('reproductor.airplay') }).click()
      await vi.advanceTimersByTimeAsync(0)
      expect(video.src).toContain('x.m3u8')

      // El usuario canceló el selector: NINGÚN
      // webkitcurrentplaybacktargetiswirelesschanged llega nunca. Sin el
      // timeout, la app se quedaría pensando que emite para siempre en
      // cuanto la reproducción local confirmara.
      await vi.advanceTimersByTimeAsync(45_000)

      // Reanuda en local (hls.js) — la señal de que se abandonó el intento.
      expect(hlsState.instancias.length).toBeGreaterThan(0)
      expect(container.querySelector('.estado.cast')).toBeNull()
      expect(screen.getByRole('button', { name: t('reproductor.airplay') }).getAttribute('aria-pressed')).toBe('false')
    } finally {
      vi.useRealTimers()
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })

  it('una ruta que WebKit ya revocó (activa=false) NO reactiva disableRemotePlayback — corta el bucle', async () => {
    const limpiarAirplay = conAirplayDisponible()
    const loadSpy = vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
    const playSpy = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    try {
      const fuente = fuenteSinMirrors()
      const { container } = render(Reproductor, { canal, fuente: fuente as any })
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(0))
      hlsState.instancias.length = 0

      const video = container.querySelector('video') as HTMLVideoElement
      // El bug real: para cuando el evento de "ruta perdida" llega, el flag
      // YA lee false (WebKit la revocó casi al instante, confirmado con
      // trace). El getter modela justo eso.
      let inalambrica = true
      Object.defineProperty(video, 'webkitCurrentPlaybackTargetIsWireless', {
        configurable: true,
        get: () => inalambrica,
      })
      const cambios = espiarRutaRemota(video)

      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()

      inalambrica = false
      video.dispatchEvent(new Event('webkitcurrentplaybacktargetiswirelesschanged'))
      await tick()

      // El bug real: esto disparaba el toggle igual, lo que hacía que WebKit
      // volviera a ofrecer la ruta casi al instante — bucle sin fin (5213
      // ciclos en 23 s, confirmado con trace real).
      expect(cambios).toEqual([])
      expect(screen.getByRole('button', { name: t('reproductor.airplay') }).getAttribute('aria-pressed')).toBe('false')
    } finally {
      limpiarAirplay()
      loadSpy.mockRestore()
      playSpy.mockRestore()
    }
  })
})

// Regresión medida en Chrome real el 2026-09-04 sobre una muestra de 30
// canales del catálogo: el arranque real tiene p50 2,2 s y p90 6,6 s, así que
// el corte fijo de 7 s del guard caía justo sobre la cola SANA. Dos canales
// que reproducen perfectamente (111 TV, A Spor) tardaron 13,3 s y 7,9 s en dar
// la segunda posición bajo la contención del arranque de la propia app —el
// catálogo y cientos de logos cargando a la vez que el primer canal— y se
// anunciaron como "no llegó a reproducir". En solitario arrancaban en 5,0 s.
// Además hls.js emitía no-fatales ('aborted', 'fragLoadError') que el
// Reproductor pasaba al guard SIN el flag fatal, matando el intento al vuelo
// pese a que el comentario del propio código decía ignorarlos.
describe('Reproductor — un canal lento pero vivo no se declara caído', () => {
  it('un error NO fatal de hls.js no cruza de mirror ni muestra error', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://lento/x.m3u8', vivo: true, latenciaMs: 100, webOk: true },
      { url: 'https://otro/x.m3u8', vivo: true, latenciaMs: 200, webOk: true },
    ]
    const fuente = {
      mirrors: vi.fn(async () => mirrors),
      proxyDisponible: vi.fn(async () => false),
    }
    const intentadas: string[] = []

    render(Reproductor, {
      canal,
      fuente: fuente as any,
      alIntentar: (url: string) => intentadas.push(url),
    })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].fallarNoFatal('fragLoadError')

    // Nada de failover ni de cartel de error: hls.js se recupera de esto solo.
    await new Promise((r) => setTimeout(r, 0))
    expect(hlsState.instancias).toHaveLength(1)
    expect(intentadas).toEqual(['https://lento/x.m3u8'])
    expect(screen.queryAllByText(t('reproductor.error.noArranco'))).toHaveLength(0)
  })
})

// Chrome no abre un MediaSource en una pestaña oculta (comprobado aislando
// userActivation de visibilityState en Chrome real, 2026-09-04): hls.js sigue
// sondeando la playlist pero no pide un segmento y readyState no pasa de 0.
// La app gastaba ahí su presupuesto de 7 s y anunciaba "no llegó a reproducir"
// sobre un canal sano, sin reintentar nunca al volver la pestaña. Como la app
// reanuda «continuar viendo» al cargar, abrirla en segundo plano daba ese
// error SIEMPRE.
describe('Reproductor — pestaña oculta', () => {
  const ponerVisibilidad = (estado: 'hidden' | 'visible') => {
    Object.defineProperty(document, 'visibilityState', { value: estado, configurable: true })
    Object.defineProperty(document, 'hidden', { value: estado === 'hidden', configurable: true })
    document.dispatchEvent(new Event('visibilitychange'))
  }

  it('oculta no muestra error, y al volver a verse reintenta el canal', async () => {
    hlsState.instancias.length = 0
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
    Object.defineProperty(document, 'hidden', { value: true, configurable: true })

    const mirrors: Mirror[] = [{ url: 'https://uno/x.m3u8', vivo: true, latenciaMs: 100, webOk: true }]
    const fuente = {
      mirrors: vi.fn(async () => mirrors),
      proxyDisponible: vi.fn(async () => false),
    }

    try {
      render(Reproductor, { canal, fuente: fuente as any, alIntentar: () => {} })

      await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))

      // Oculta: el presupuesto está congelado, así que no se declara caído.
      await new Promise((r) => setTimeout(r, 50))
      expect(screen.queryAllByText(t('reproductor.error.noArranco'))).toHaveLength(0)

      // Al volver a verse, se reintenta desde cero con un MediaSource utilizable.
      ponerVisibilidad('visible')
      await vi.waitFor(() => expect(hlsState.instancias.length).toBeGreaterThan(1))
    } finally {
      ponerVisibilidad('visible')
    }
  })
})

// Caso AMC (720p), reportado por el dueño el 2026-09-04. El failover SÍ probó
// los dos mirrors automáticamente —eso funcionaba—, pero la tarjeta de error
// mentía tres veces: decía «La dirección del canal caducó» (la clase del ÚLTIMO
// intento, cuando el primero falló por lentitud), anunciaba «Hay 2 mirrors con
// mejor salud» (era el TOTAL, ya agotado, y sin filtrar por salud) y ofrecía
// «Probar el siguiente mirror» cuando no quedaba ninguno.
describe('Reproductor — la tarjeta de error no miente sobre los mirrors', () => {
  it('con dos mirrors que fallan por causas distintas, no afirma ninguna', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://lento/x.m3u8', vivo: true, latenciaMs: 100, webOk: true },
      { url: 'https://ido/x.m3u8', vivo: false, latenciaMs: 900, webOk: true },
    ]
    const fuente = {
      mirrors: vi.fn(async () => mirrors),
      proxyDisponible: vi.fn(async () => false),
    }

    render(Reproductor, { canal, fuente: fuente as any })

    // Mirror 1: sin señal concreta -> 'desconocido' (el lento de AMC).
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].fallar('bufferStalledError')

    // Mirror 2: 404 -> 'caducado' (el que ya no existe).
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(2))
    hlsState.instancias[1].fallarConStatus('manifestLoadError', 404)

    // Con causas contradictorias se muestra la triple adivinanza declarada, NO
    // «la dirección caducó», que solo era verdad de uno de los dos.
    await vi.waitFor(() =>
      expect(screen.queryAllByText(t('reproductor.error.noArranco')).length).toBeGreaterThan(0),
    )
    expect(screen.queryAllByText(t('reproductor.error.caducado'))).toHaveLength(0)

    // Y el recuento dice lo que de verdad pasó: se probaron los dos.
    expect(screen.getByText(t('reproductor.error.mirrorsProbados', { n: 2 }))).toBeTruthy()
  })
})
