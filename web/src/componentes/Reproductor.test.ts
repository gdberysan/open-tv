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
      alCerrar: () => {},
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

    render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

    // queryAllByText: mismo motivo que arriba — el mensaje aparece por
    // partida doble (visual + región aria-live persistente) desde el fix
    // round 1 de la Tarea 18.
    await vi.waitFor(() => expect(screen.queryAllByText(t('estado.gatewayCaido')).length).toBeGreaterThan(0))
    expect(screen.queryByText(t('reproductor.error.noArranco'))).toBeNull()
  })

  // Tarea 14 (P0.6): el error terminal de agotar el failover, cuando el
  // canal SÍ tenía mirrors, ofrece un CTA que reanuda el MISMO mecanismo
  // (reproducir()) en vez de ser un callejón sin salida.
  it('agotados los mirrors, el error ofrece «Probar el siguiente mirror» y el clic reanuda el failover', async () => {
    hlsState.instancias.length = 0
    const mirrors: Mirror[] = [
      { url: 'https://muerto/x.m3u8', vivo: true, latenciaMs: 100, webOk: true },
      { url: 'https://tambien-muerto/x.m3u8', vivo: true, latenciaMs: 200, webOk: true },
    ]
    const fuente = {
      mirrors: vi.fn(async () => mirrors),
      proxyDisponible: vi.fn(async () => false),
    }

    render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].fallar('manifestLoadError')
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(2))
    hlsState.instancias[1].fallar('manifestLoadError')

    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.error.noArranco')).length).toBeGreaterThan(0))

    // El texto informativo cuenta los mirrors que traía ESTE intento (2).
    // getByText ya lanza si no lo encuentra — no hace falta un matcher aparte.
    screen.getByText(t('reproductor.error.mirrorsDisponibles', { n: 2 }))
    const boton = screen.getByRole('button', { name: t('reproductor.error.probarSiguienteMirror') })

    hlsState.instancias.length = 0
    boton.click()

    // Reanuda el MISMO mecanismo: vuelve a pedir mirrors() (2ª vez) y
    // arranca de nuevo desde el primer intento de la cadena.
    await vi.waitFor(() => expect(fuente.mirrors).toHaveBeenCalledTimes(2))
    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    expect(hlsState.instancias[0].url).toBe('https://muerto/x.m3u8')
  })

  // Sin mirrors (fallback de compatibilidad al destino único): no hay
  // failover que reanudar, así que el CTA no debe aparecer.
  it('sin mirrors, el error NO ofrece el CTA de mirror', async () => {
    hlsState.instancias.length = 0
    const fuente = {
      mirrors: vi.fn(async () => []),
      destino: vi.fn(async () => ({ url: 'https://unico/x.m3u8', airplayOk: null })),
      proxyDisponible: vi.fn(async () => false),
    }

    render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

    await vi.waitFor(() => expect(hlsState.instancias).toHaveLength(1))
    hlsState.instancias[0].fallar('manifestLoadError')

    await vi.waitFor(() => expect(screen.queryAllByText(t('reproductor.error.noArranco')).length).toBeGreaterThan(0))
    expect(screen.queryByRole('button', { name: t('reproductor.error.probarSiguienteMirror') })).toBeNull()
    expect(screen.queryByText(t('reproductor.error.mirrorsDisponibles', { n: 1 }))).toBeNull()
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
    const { container } = render(Reproductor, { canal: canalConMeta, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })

    expect(screen.getByText(t('reproductor.envivo'))).toBeTruthy()
    expect(container.querySelector('.overlay-nombre')?.textContent).toBe('BBC One 1080p')
    expect(container.querySelector('.overlay-meta')?.textContent).toBe('1080p · 150 ms · GB')
  })

  it('sin resolución/latencia/país en el canal, la línea meta no se muestra', () => {
    const canalSinMeta: Canal = { ...canal, nombre: 'X', pais: '', latenciaMs: 0 }
    const { container } = render(Reproductor, { canal: canalSinMeta, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })

    expect(container.querySelector('.overlay-meta')).toBeNull()
  })

  it('el botón Silenciar del overlay togglea video.muted y aria-pressed', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
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
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
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

  it('los controles nuevos del overlay tienen nombre accesible y están dentro del contenedor del diálogo', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
    const dialogo = container.querySelector('[role="dialog"]')
    expect(dialogo).toBeTruthy()

    const silenciar = dialogo!.querySelector('.overlay-controles .silenciar')
    const favorito = dialogo!.querySelector('.overlay-controles .favorito')

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

  it('el botón de pantalla completa llama a requestFullscreen() sobre el CONTENEDOR del diálogo, no sobre el <video>', async () => {
    const fullscreenEnabledDesc = Object.getOwnPropertyDescriptor(Document.prototype, 'fullscreenEnabled')
    Object.defineProperty(document, 'fullscreenEnabled', { value: true, configurable: true })
    const contenedorSpy = vi.fn()
    const videoSpy = vi.fn()
    HTMLDivElement.prototype.requestFullscreen = contenedorSpy
    HTMLVideoElement.prototype.requestFullscreen = videoSpy

    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
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
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
      const contenedor = container.querySelector('[role="dialog"]') as HTMLElement
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

  it('con document.fullscreenElement activo, Escape sale de pantalla completa y NO cierra el modal', async () => {
    const contenedorDeMentira = document.createElement('div')
    Object.defineProperty(document, 'fullscreenElement', { value: contenedorDeMentira, configurable: true })
    document.exitFullscreen = vi.fn().mockResolvedValue(undefined)
    const alCerrar = vi.fn()

    try {
      render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar })

      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

      expect(document.exitFullscreen).toHaveBeenCalledTimes(1)
      expect(alCerrar).not.toHaveBeenCalled()
    } finally {
      // @ts-expect-error limpieza de la propiedad redefinida
      delete document.fullscreenElement
      // @ts-expect-error limpieza del parche de prototipo
      delete document.exitFullscreen
    }
  })

  it('sin document.fullscreenElement, Escape sigue cerrando el modal como antes', async () => {
    const alCerrar = vi.fn()
    render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar })

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(alCerrar).toHaveBeenCalledTimes(1)
  })

  it('el botón de PiP solo se renderiza si document.pictureInPictureEnabled es true', async () => {
    Object.defineProperty(document, 'pictureInPictureEnabled', { value: false, configurable: true })
    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
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
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
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
      const { container, unmount } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, alCerrar: () => {} })
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

    const { container } = render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

    await vi.waitFor(() => expect(fuente.epgDeCanal).toHaveBeenCalledWith('c1'))

    const horaAhora = `${formatearHoraLocal(ahora.inicioSeg)}–${formatearHoraLocal(ahora.finSeg)}`
    await vi.waitFor(() => expect(container.textContent).toContain(`${t('epg.ahora')}: Telediario (${horaAhora})`))
    expect(container.textContent).toContain(`${t('epg.siguiente')}: El Tiempo`)
    expect(container.textContent).not.toContain(t('epg.sinGuia'))
  })

  it('sin guía (ahora=null y sin próximos), el overlay muestra el mensaje explícito «sin guía para esta fuente»', async () => {
    const fuente = fuenteConEpg(async () => ({ ahora: null, proximos: [] }))

    const { container } = render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

    await vi.waitFor(() => expect(container.textContent).toContain(t('epg.sinGuia')))
  })

  it('un fallo de epgDeCanal no rompe el reproductor: sigue montado, sin el mensaje de "sin guía" (eso es para una respuesta confirmada, no un fallo de red)', async () => {
    const fuente = fuenteConEpg(async () => {
      throw new Error('epg inalcanzable')
    })

    const { container } = render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

    await vi.waitFor(() => expect(fuente.epgDeCanal).toHaveBeenCalled())
    // Deja pasar el rechazo de la promesa antes de comprobar que no rompió nada.
    await new Promise((r) => setTimeout(r, 0))

    expect(container.querySelector('[role="dialog"]')).toBeTruthy()
    expect(container.textContent).not.toContain(t('epg.sinGuia'))
  })

  it('al abrir otro canal (no un mirror del mismo), se vuelve a pedir la guía y no se pinta la del canal anterior', async () => {
    const epgPorCanal: Record<string, { ahora: Programa | null; proximos: Programa[] }> = {
      c1: { ahora: { titulo: 'Programa Uno', inicioSeg: 1_700_000_000, finSeg: 1_700_003_000 }, proximos: [] },
      c2: { ahora: { titulo: 'Programa Dos', inicioSeg: 1_700_000_000, finSeg: 1_700_003_000 }, proximos: [] },
    }
    const fuente = fuenteConEpg(async (id) => epgPorCanal[id])

    const { container, rerender } = render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })
    await vi.waitFor(() => expect(container.textContent).toContain('Programa Uno'))

    const canal2 = { ...canal, id: 'c2', nombre: 'Y' } as Canal
    rerender({ canal: canal2, fuente: fuente as any, alCerrar: () => {} })

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
      render(Reproductor, { canal, fuente: fuente as any, alCerrar: () => {} })

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
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, modo: 'panel' })
    expect(container.querySelector('[role="dialog"]')).toBeNull()
    expect(container.querySelector('[aria-modal]')).toBeNull()
    expect(screen.getByRole('region', { name: canal.nombre })).toBeTruthy()
  })

  it('en modo panel, Escape SIN pantalla completa no llama a alCerrar (ya no hay modal que cerrar)', async () => {
    const alCerrar = vi.fn()
    render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, modo: 'panel', alCerrar })
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(alCerrar).not.toHaveBeenCalled()
  })

  it('con activo=false el teclado global del reproductor queda inerte (espacio no reproduce/pausa)', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, modo: 'panel', activo: false })
    const video = container.querySelector('video') as HTMLVideoElement
    const pause = vi.spyOn(video, 'pause').mockImplementation(() => {})
    const play = vi.spyOn(video, 'play').mockResolvedValue()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: ' ' }))
    expect(pause).not.toHaveBeenCalled()
    expect(play).not.toHaveBeenCalled()
  })

  it('en modo panel, una tecla con el foco en un control interactivo AJENO se ignora; sin objetivo interactivo, sí actúa', async () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, modo: 'panel' })
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
      modo: 'panel',
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
      modo: 'panel',
      silenciadoInicial: true,
    })
    const video = container.querySelector('video') as HTMLVideoElement
    expect(video.muted).toBe(true)

    await rerender({ canal: { ...canal, id: 'c2', nombre: 'Otro' } as Canal, fuente, modo: 'panel', silenciadoInicial: true })
    await tick()

    expect(video.muted).toBe(false)
    expect(screen.queryByRole('button', { name: t('reproductor.activarSonido') })).toBeNull()
  })

  it('en modo panel el botón AirPlay vive en el overlay y no hay barra inferior con Cerrar', () => {
    ;(window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent'] = class {}
    try {
      const { container } = render(Reproductor, { canal, fuente: fuenteSinMirrors() as any, modo: 'panel' })
      expect(screen.queryByRole('button', { name: t('reproductor.cerrar') })).toBeNull()
      const airplay = screen.getByRole('button', { name: 'AirPlay' })
      expect(airplay.closest('.overlay-controles')).toBeTruthy()
      expect(container.querySelector('.controles')).toBeNull()
    } finally {
      delete (window as unknown as Record<string, unknown>)['WebKitPlaybackTargetAvailabilityEvent']
    }
  })
})
