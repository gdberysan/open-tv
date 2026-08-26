import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'
import { tick } from 'svelte'
import App from '../App.svelte'
import Sincronizando from '../componentes/Sincronizando.svelte'
import MensajeError from '../componentes/MensajeError.svelte'
import RejillaVirtual from '../componentes/RejillaVirtual.svelte'
import RejillaCanales from '../componentes/RejillaCanales.svelte'
import Reproductor from '../componentes/Reproductor.svelte'
import { filtros } from '../estado/filtros'
import { favoritos } from '../estado/favoritos'
import { consultarSalud } from '../estado/salud'
import { t } from '../i18n'
import type { Canal, CatalogSource, PaginaCanales } from '../datos/catalogo'

// Tarea 18 — auditoría de accesibilidad. Estas aserciones son las que
// jsdom + testing-library pueden verificar de verdad (rol, nombre accesible,
// aria-live, tabindex, foco); VoiceOver de verdad es el gate manual del
// autor (ver task-18-brief.md, paso 5 — no se ejecuta aquí).

// App.svelte llama a consultarSalud() (fetch a /health) en onMount; se
// sustituye por una resolución inmediata a "ya sincronizado", igual que
// App.integracion.test.ts — sin esto la App se queda en la fase
// 'comprobando' para siempre y nunca llega a montar la rejilla.
vi.mock('../estado/salud', async (importOriginal) => {
  const real = await importOriginal<typeof import('../estado/salud')>()
  return {
    ...real,
    consultarSalud: vi.fn(async () => ({
      sincronizando: false,
      proxyDisponible: false,
      ultimoSync: new Date('2026-01-01T00:00:00Z'),
      version: 'test',
    })),
  }
})

// Mismo doble que RejillaVirtual.test.ts / App.integracion.test.ts: jsdom no
// implementa IntersectionObserver, y tanto RejillaVirtual como RejillaCanales
// lo usan para el scroll infinito.
class FalsoIntersectionObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return []
  }
}
;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FalsoIntersectionObserver

function canalFalso(i: number): Canal {
  return {
    id: String(i),
    nombre: `Canal ${i}`,
    logoUrl: '',
    categoriaId: 'General',
    idioma: 'en',
    pais: 'GB',
    vivo: true,
    latenciaMs: 100,
    webOk: true,
  }
}

// Igual que RejillaVirtual.test.ts: el cálculo de la ventana virtualizada se
// agenda con requestAnimationFrame (coalescencia por rAF); hay que esperarlo
// explícitamente antes de esperar también un tick de Svelte para que el
// estado se propague al DOM.
async function asentar() {
  await new Promise<number>((resolve) => requestAnimationFrame(resolve))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function fuenteFalsa(overrides: Partial<CatalogSource> = {}): CatalogSource {
  return {
    canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales: [], total: 0 })),
    paises: vi.fn(async () => []),
    categorias: vi.fn(async () => []),
    calidades: vi.fn(async () => []),
    aleatorio: vi.fn(async () => canalFalso(0)),
    destino: vi.fn(async () => ({ url: '', airplayOk: null })),
    mirrors: vi.fn(async () => []),
    frescura: vi.fn(async () => ({ tipo: 'vivo' as const, generadoEn: null })),
    proxyDisponible: vi.fn(async () => false),
    fuentes: vi.fn(async () => []),
    anadirFuente: vi.fn(async () => ({
      id: 'f1', label: 'Fuente falsa', url: '', kind: 'url' as const, ultimoSync: null, canales: 0,
    })),
    anadirFuenteFichero: vi.fn(async () => ({
      id: 'f1', label: 'Fuente falsa', url: '', kind: 'file' as const, ultimoSync: null, canales: 0,
    })),
    quitarFuente: vi.fn(async () => {}),
    resyncFuente: vi.fn(async () => {}),
    fuentesSugeridas: vi.fn(async () => []),
    epgDeCanales: vi.fn(async () => new Map()),
    epgDeCanal: vi.fn(async () => ({ ahora: null, proximos: [] })),
    ...overrides,
  }
}

beforeEach(() => {
  filtros.set({
    q: '', pais: '', categoria: '', calidad: '', mostrarOffline: false,
    soloFavoritos: false, vista: 'rejilla',
  })
  favoritos.set(new Set())
})

describe('a11y — estados: un único anunciador, sin doble anuncio (fix round 2)', () => {
  // Fix round 1 añadió en App.svelte regiones aria-live PERSISTENTES para
  // sincronizando/error, pero dejó a Sincronizando.svelte/MensajeError.svelte
  // con SU PROPIA semántica live (role="status"/"alert" + aria-live) — ambos
  // se montan a la vez que la región persistente de App, así que un lector
  // de pantalla anunciaba el mismo texto DOS VECES seguidas. El arreglo
  // retira la semántica live de los componentes visuales: App.svelte queda
  // como la única dueña de la región que anuncia (Sincronizando/MensajeError
  // solo se montan desde App — verificado antes de tocar nada).

  // Mismo motivo que en el describe de "regiones aria-live PERSISTENTES" de
  // más abajo: consultarSalud es un mock singleton por módulo, y una
  // respuesta "Once" sin consumir se filtraría al siguiente test/describe.
  afterEach(() => {
    vi.mocked(consultarSalud).mockReset()
    vi.mocked(consultarSalud).mockImplementation(async () => ({
      sincronizando: false,
      proxyDisponible: false,
      ultimoSync: new Date('2026-01-01T00:00:00Z'),
      version: 'test',
    }))
  })

  it('Sincronizando/MensajeError ya NO llevan su propia semántica live (el texto sigue visible)', () => {
    const { container: cSync } = render(Sincronizando, { alListo: () => {} })
    // Antes de este arreglo, [role="status"] existía aquí y esta aserción
    // habría fallado en la primera línea.
    expect(cSync.querySelector('[role="status"]')).toBeNull()
    expect(cSync.querySelector('[aria-live]')).toBeNull()
    expect(cSync.textContent).toContain(t('estado.sincronizando')) // el texto sigue ahí

    const { container: cError } = render(MensajeError, { clase: 'gateway' })
    expect(cError.querySelector('[role="alert"]')).toBeNull()
    expect(cError.querySelector('[aria-live]')).toBeNull()
    expect(cError.textContent).toContain(t('estado.gatewayCaido'))
  })

  it('App en fase "sincronizando": existe EXACTAMENTE UNA región aria-live con ese texto (no dos)', async () => {
    const respuestaSincronizando = {
      sincronizando: true, proxyDisponible: false, ultimoSync: null, version: 'test',
    }
    // Dos respuestas encoladas: ver la nota de la suite de abajo sobre por
    // qué Sincronizando.svelte hace su propia llamada a consultarSalud().
    vi.mocked(consultarSalud).mockResolvedValueOnce(respuestaSincronizando)
    vi.mocked(consultarSalud).mockResolvedValueOnce(respuestaSincronizando)
    const { container } = render(App, { fuente: fuenteFalsa() })

    // Contra el código pre-fix-round-2 (Sincronizando con su propio
    // role="status" aria-live="polite" MONTADO A LA VEZ que la región
    // persistente de App, ambos con el mismo texto), este recuento daría 2,
    // no 1 — es la aserción que habría fallado con el bug del doble anuncio.
    await vi.waitFor(() => {
      const conElTexto = [...container.querySelectorAll('[aria-live]')].filter(
        (el) => el.textContent === t('estado.sincronizando'),
      )
      expect(conElTexto.length).toBe(1)
    })
  })

  it('App en fase "error": existe EXACTAMENTE UNA región aria-live="assertive" con ese texto (no dos)', async () => {
    vi.mocked(consultarSalud).mockRejectedValueOnce(new Error('gateway inalcanzable'))
    const { container } = render(App, { fuente: fuenteFalsa() })

    // Mismo razonamiento que arriba, con MensajeError y role="alert": sin
    // este arreglo el recuento daría 2 (el <p role="alert"> visual + la
    // región persistente), no 1.
    await vi.waitFor(() => {
      const conElTexto = [...container.querySelectorAll('[aria-live="assertive"]')].filter(
        (el) => el.textContent === t('estado.gatewayCaido'),
      )
      expect(conElTexto.length).toBe(1)
    })
  })
})

describe('a11y — regiones aria-live PERSISTENTES (fix round 1, Hallazgo 1)', () => {
  // La diferencia con el describe anterior: allí se comprueba que el
  // ATRIBUTO existe. Aquí se comprueba lo que ese hallazgo señaló que
  // faltaba: que la región ya está montada ANTES del cambio de estado
  // (para que un lector de pantalla que solo anuncia mutaciones de texto
  // dentro de un nodo YA presente, no la inserción del nodo, sí la capte),
  // y que es el MISMO nodo (misma referencia) el que cambia de texto, no
  // uno nuevo que reemplaza al anterior.

  // consultarSalud es un mock SINGLETON por módulo (no se recrea entre
  // tests): si un test deja una respuesta "Once" sin consumir en su cola,
  // el SIGUIENTE test que también llama a consultarSalud() se la comería
  // sin darse cuenta. Restaurar aquí la implementación por defecto tras
  // cada test de este describe evita ese acoplamiento entre tests.
  afterEach(() => {
    vi.mocked(consultarSalud).mockReset()
    vi.mocked(consultarSalud).mockImplementation(async () => ({
      sincronizando: false,
      proxyDisponible: false,
      ultimoSync: new Date('2026-01-01T00:00:00Z'),
      version: 'test',
    }))
  })

  it('App: la región polite ya existe en el montaje inicial y su TEXTO (no el nodo) refleja "sincronizando"', async () => {
    // Fuerza la fase 'sincronizando' (el mock global de consultarSalud
    // resuelve 'listo' por defecto). Se encolan EXACTAMENTE dos respuestas
    // —no solo una— porque Sincronizando.svelte hace su PROPIA llamada a
    // consultarSalud() en su onMount (además de la de App): con una sola
    // respuesta encolada, esa segunda llamada consumiría el valor por
    // defecto (sincronizando:false) y la fase saltaría a 'listo' antes de
    // que la aserción de abajo llegara a leer el texto. Exactamente dos
    // (ni una de más) para no dejar una respuesta sin consumir que
    // contamine el SIGUIENTE test del fichero (el mock es un singleton por
    // módulo, no se resetea entre tests).
    const respuestaSincronizando = {
      sincronizando: true, proxyDisponible: false, ultimoSync: null, version: 'test',
    }
    vi.mocked(consultarSalud).mockResolvedValueOnce(respuestaSincronizando)
    vi.mocked(consultarSalud).mockResolvedValueOnce(respuestaSincronizando)
    const { container } = render(App, { fuente: fuenteFalsa() })

    // Ya existe en el primer render (fase todavía 'comprobando'), vacía.
    const region = container.querySelector('[aria-live="polite"].sr-only')
    expect(region).not.toBeNull()
    expect(region?.textContent).toBe('')

    await vi.waitFor(() => expect(region?.textContent).toBe(t('estado.sincronizando')))

    // MISMO NODO: si el arreglo montara/desmontara un <p aria-live> nuevo
    // (el bug original), esta comprobación de identidad fallaría aunque el
    // texto final fuera el correcto.
    expect(container.querySelector('[aria-live="polite"].sr-only')).toBe(region)
  })

  it('App: la región assertive ya existe en el montaje inicial y su TEXTO refleja el error de salud', async () => {
    vi.mocked(consultarSalud).mockRejectedValueOnce(new Error('gateway inalcanzable'))
    const { container } = render(App, { fuente: fuenteFalsa() })

    const region = container.querySelector('[aria-live="assertive"].sr-only')
    expect(region).not.toBeNull()
    expect(region?.textContent).toBe('')

    await vi.waitFor(() => expect(region?.textContent).toBe(t('estado.gatewayCaido')))
    expect(container.querySelector('[aria-live="assertive"].sr-only')).toBe(region)
  })

  it('Reproductor: las dos regiones existen desde el montaje y solo su texto cambia entre "cargando" y el error', async () => {
    const canal: Canal = { ...canalFalso(0), nombre: 'Canal de prueba' }
    const fuente = fuenteFalsa({
      mirrors: vi.fn(async () => {
        throw new Error('gateway inalcanzable')
      }),
      proxyDisponible: vi.fn(async () => false),
    })
    const { container } = render(Reproductor, { canal, fuente, alCerrar: () => {} })

    const polite = container.querySelector('[aria-live="polite"].sr-only')
    const assertive = container.querySelector('[aria-live="assertive"].sr-only')
    expect(polite).not.toBeNull()
    expect(assertive).not.toBeNull()
    // cargando=true desde el primer render: la región polite ya lo refleja.
    expect(polite?.textContent).toBe(t('reproductor.cargando'))
    expect(assertive?.textContent).toBe('')

    await vi.waitFor(() => expect(assertive?.textContent).toBe(t('estado.gatewayCaido')))
    // La polite vuelve a quedar vacía (ya no está cargando) — mismo nodo,
    // no uno nuevo.
    expect(polite?.textContent).toBe('')
    expect(container.querySelector('[aria-live="polite"].sr-only')).toBe(polite)
    expect(container.querySelector('[aria-live="assertive"].sr-only')).toBe(assertive)
  })
})

describe('a11y — rejilla y lista exponen role=list/listitem', () => {
  it('RejillaVirtual: el contenedor es role=list y cada tarjeta es role=listitem', async () => {
    const canales = Array.from({ length: 4 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const lista = container.querySelector('.rejilla-virtual')
    expect(lista?.getAttribute('role')).toBe('list')
    const items = [...container.querySelectorAll('article')]
    expect(items.length).toBeGreaterThan(0)
    for (const item of items) expect(item.getAttribute('role')).toBe('listitem')
  })

  it('RejillaCanales en modo lista: el contenedor es role=list y cada fila es role=listitem', () => {
    const canales = [canalFalso(0), canalFalso(1)]
    const { container } = render(RejillaCanales, {
      canales, vista: 'lista', cargando: false, alPedirMas: () => {}, alAbrir: () => {},
    })
    const lista = container.querySelector('.canales.lista')
    expect(lista?.getAttribute('role')).toBe('list')
    const filas = [...container.querySelectorAll('article.fila')]
    expect(filas.length).toBe(2)
    for (const fila of filas) expect(fila.getAttribute('role')).toBe('listitem')
  })
})

describe('a11y — roving tabindex en la rejilla virtualizada', () => {
  it('solo la tarjeta activa tiene tabindex=0 en AMBOS controles (abrir y favorito); ArrowRight mueve el foco y el tabindex a la siguiente', async () => {
    const canales = Array.from({ length: 5 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const abrir = () => [...container.querySelectorAll<HTMLElement>('article .abrir')]
    const favorito = () => [...container.querySelectorAll<HTMLElement>('article .favorito')]
    // Antes del arreglo ningún botón "abrir" tenía atributo tabindex, y el
    // de favorito NUNCA lo tuvo (fix round 1, Hallazgo 2): cada tarjeta
    // visible aportaba su estrella como tab stop aunque no fuera la
    // activa. getAttribute devolvía null (abrir) o '0' fijo (favorito) en
    // vez de '0'/'-1' según focoActivo.
    expect(abrir()[0].getAttribute('tabindex')).toBe('0')
    expect(abrir()[1].getAttribute('tabindex')).toBe('-1')
    expect(favorito()[0].getAttribute('tabindex')).toBe('0')
    expect(favorito()[1].getAttribute('tabindex')).toBe('-1')

    abrir()[0].focus()
    await fireEvent.keyDown(abrir()[0], { key: 'ArrowRight' })
    await asentar()

    expect(document.activeElement).toBe(abrir()[1])
    expect(abrir()[0].getAttribute('tabindex')).toBe('-1')
    expect(abrir()[1].getAttribute('tabindex')).toBe('0')
    // El favorito sigue a la MISMA tarjeta activa que el botón "abrir": se
    // gatea igual, no de forma independiente.
    expect(favorito()[0].getAttribute('tabindex')).toBe('-1')
    expect(favorito()[1].getAttribute('tabindex')).toBe('0')
  })

  it('una tarjeta no-activa no aporta ningún tab stop (ni abrir ni favorito) — el roving tabindex reduce 2·N a 2', async () => {
    const canales = Array.from({ length: 4 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const tabuables = container.querySelectorAll('article [tabindex="0"]')
    // Antes del arreglo, cada una de las 4 tarjetas aportaba su favorito
    // como tab stop (tabindex 0 fijo) además del "abrir" de la activa:
    // 5 nodos con tabindex=0 en vez de 2 (los de la única tarjeta activa).
    expect(tabuables.length).toBe(2)
  })

  it('End mueve el foco a un canal que NO estaba montado (sobrevive a la ventana virtualizada)', async () => {
    // 300 canales fuerza una ventana real (con 5 no se llega a virtualizar
    // de verdad: el buffer de filas los cubre todos desde el principio,
    // como ya prueba RejillaVirtual.test.ts). El índice 299 no existe en el
    // DOM al montar — es justo el caso que puede perder el foco si el nodo
    // se desmonta/no existe todavía.
    const canales = Array.from({ length: 300 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    expect(container.querySelector('[data-indice="299"]')).toBeNull() // todavía no montado

    const primerBoton = container.querySelectorAll<HTMLElement>('article .abrir')[0]
    primerBoton.focus()
    await fireEvent.keyDown(primerBoton, { key: 'End' })
    await asentar()

    const ultimo = container.querySelector<HTMLElement>('[data-indice="299"] .abrir')
    expect(ultimo).not.toBeNull() // enfocarIndice lo montó a propósito
    expect(document.activeElement).toBe(ultimo)
    expect(ultimo?.getAttribute('tabindex')).toBe('0')
    // La tarjeta 0 ya no está en el DOM (la ventana se movió, no se
    // "ensanchó" hasta cubrir las 300 filas de por medio).
    expect(container.querySelector('[data-indice="0"]')).toBeNull()
  })

  it('Enter/clic activa la tarjeta enfocada mediante la semántica nativa del <button> (no reimplementada)', async () => {
    const alAbrir = vi.fn()
    const canales = Array.from({ length: 3 }, (_, i) => canalFalso(i))
    const { container } = render(RejillaVirtual, { canales, alAbrir, alPedirMas: () => {} })
    await asentar()

    const boton = container.querySelector<HTMLElement>('article .abrir')!
    await fireEvent.click(boton)
    expect(alAbrir).toHaveBeenCalledOnce()
  })
})

describe('a11y — nombres accesibles', () => {
  it('el botón que abre el canal tiene el nombre del canal como nombre accesible', async () => {
    const canales = [canalFalso(7)]
    const { container } = render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    expect(screen.getByRole('button', { name: 'Canal 7' })).toBe(container.querySelector('.abrir'))
  })

  it('el botón de favorito anuncia añadir/quitar y expone su estado con aria-pressed', async () => {
    const canales = [canalFalso(0)]
    render(RejillaVirtual, { canales, alAbrir: () => {}, alPedirMas: () => {} })
    await asentar()

    const favorito = screen.getByRole('button', { name: t('canal.favorito.anadir') })
    expect(favorito.getAttribute('aria-pressed')).toBe('false')
  })
})

describe('a11y — reproductor: controles etiquetados, aria-live y foco', () => {
  const canal: Canal = { ...canalFalso(0), nombre: 'Canal de prueba' }

  it('todos los botones de control tienen nombre accesible', () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    const botones = screen.getAllByRole('button')
    expect(botones.length).toBeGreaterThan(0)
    for (const boton of botones) {
      const nombre = boton.getAttribute('aria-label') || boton.textContent?.trim()
      expect(nombre).toBeTruthy()
    }
  })

  it('el diálogo es role=dialog, aria-modal y su nombre accesible es el del canal', () => {
    const { container } = render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    const dialogo = container.querySelector('[role="dialog"]')
    expect(dialogo?.getAttribute('aria-modal')).toBe('true')
    expect(dialogo?.getAttribute('aria-label')).toBe('Canal de prueba')
  })

  it('al montar, el foco entra en el reproductor en vez de quedarse fuera', async () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    await tick()
    await tick() // el enfoque real ocurre en un .then() encadenado tras tick(), dentro de onMount

    // Antes del arreglo, document.activeElement se quedaba en <body> (o en
    // lo que tuviera el foco antes de abrir el reproductor): esta
    // comprobación fallaba porque closest('.reproductor') daba null.
    expect(document.activeElement?.closest('.reproductor')).not.toBeNull()
  })

  it('Tab en el último control cicla de vuelta al primero (foco atrapado dentro del diálogo)', async () => {
    render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    await tick()

    // Todos los botones del diálogo, no solo los de `.controles`: desde la
    // Tarea 1 (P0.8) el overlay 1b sobre el vídeo (`.overlay-controles`)
    // también aporta controles focables — el primero del diálogo ya no es
    // el primero de la barra inferior fija.
    const dialogo = document.querySelector('[role="dialog"]')!
    const controles = [...dialogo.querySelectorAll<HTMLElement>('button')]
    expect(controles.length).toBeGreaterThan(1)
    controles.at(-1)!.focus()
    await fireEvent.keyDown(controles.at(-1)!, { key: 'Tab' })

    expect(document.activeElement).toBe(controles[0])
  })

  it('al cerrarse (desmontar), el foco vuelve a quien lo abrió (diferido a un frame — fix round 3)', async () => {
    // Fix round 3 (bug real cazado en el gate manual de teclado en Chrome
    // real, no reproducible por jsdom): la restauración del foco se DIFIERE
    // con requestAnimationFrame en onDestroy —ver Reproductor.svelte—
    // porque App.svelte limpia el `inert` de <main> y desmonta este
    // componente en el MISMO flush, y onDestroy corría ANTES de que ese
    // inert se limpiara; focus() sobre un nodo aún inert es un no-op y el
    // foco caía a <body>. jsdom no aplica el bloqueo de foco de `inert`
    // (esa parte solo la cubre el gate manual en navegador real), pero SÍ
    // permite comprobar lo que es verificable aquí: que el foco vuelve al
    // disparador DESPUÉS de un frame, no que "nunca se pierde" — por eso el
    // `await asentar()` de después de `unmount()`, y no una comprobación
    // síncrona inmediatamente después.
    document.body.innerHTML = '<button id="disparador">abrir</button>'
    const disparador = document.getElementById('disparador') as HTMLButtonElement
    disparador.focus()
    expect(document.activeElement).toBe(disparador)

    const { unmount } = render(Reproductor, { canal, fuente: fuenteFalsa(), alCerrar: () => {} })
    await tick()
    await tick()
    expect(document.activeElement).not.toBe(disparador) // el foco entró al reproductor al abrir

    unmount()
    // Se espera un frame (asentar) antes de comprobar, a propósito: con la
    // restauración diferida por rAF (fix round 3) el foco NO vuelve todavía
    // en el instante justo después de unmount(). Lo falsable aquí: contra
    // una versión que NO restaure el foco en absoluto (el onDestroy sin la
    // llamada a .focus(), regresión que revertiría el fix), esta aserción
    // fallaría incluso después de esperar el frame — document.activeElement
    // se quedaría en <body>, no en disparador.
    await asentar()
    expect(document.activeElement).toBe(disparador)
  })
})

describe('a11y — App: el fondo queda inert mientras el reproductor está abierto', () => {
  // jsdom (a fecha de esta tarea) no implementa la propiedad IDL `inert` del
  // estándar HTML — 'inert' in document.createElement('div') da false — así
  // que Svelte no puede pasar por el mismo camino de reflejo attr↔propiedad
  // que un navegador real. Lo que SÍ hace de forma fiable (comprobado a
  // mano) es asignar la propiedad JS `element.inert = valor`, que jsdom
  // conserva como una propiedad normal aunque no la reflecte a un atributo
  // del DOM. Por eso la aserción lee `.inert` (la propiedad), no
  // `hasAttribute('inert')` (que en jsdom se queda en null pasara lo que
  // pasara, y habría dado un falso verde/rojo sin relación con el fix). En
  // un navegador real ambos caminos —propiedad y atributo— están enlazados.
  it('<main>/<footer> no son inert sin reproductor, y sí en cuanto se abre uno', async () => {
    const canales = [canalFalso(0)]
    const fuente = fuenteFalsa({ canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales, total: 1 })) })
    const { container } = render(App, { fuente })

    const abrirCanal = await screen.findByLabelText('Canal 0')
    const main = container.querySelector('main') as (HTMLElement & { inert?: boolean }) | null
    const pie = container.querySelector('footer.pie') as (HTMLElement & { inert?: boolean }) | null
    // Antes del arreglo, <main>/<footer> no tenían el atributo inert={...}
    // en absoluto: esta propiedad era simplemente undefined, no false.
    expect(main?.inert).toBe(false)
    expect(pie?.inert).toBe(false)

    await fireEvent.click(abrirCanal)
    await tick()

    expect(main?.inert).toBe(true)
    expect(pie?.inert).toBe(true)
  })

  it('fix1 Hallazgo 1: la cabecera (con el toggle de idioma) queda dentro de un ancestro inert al abrir el reproductor', async () => {
    // Antes de este arreglo, <header>/el botón de cajón eran HERMANOS de
    // <main> (Tarea 4 los sacó de dentro al reestructurar el shell) y no
    // llevaban inert propio: con el reproductor abierto, el toggle ES/EN
    // seguía siendo clicable y un lector de pantalla en modo navegación
    // podía entrar en la cabecera por detrás del modal. El arreglo envuelve
    // cabecera + cuerpo + pie en un único contenedor (`div.fondo`) con
    // inert={!!canalAbierto}: basta con comprobar que el ANCESTRO que
    // envuelve la cabecera queda inert (no solo <main>).
    //
    // Igual que en el test de arriba: jsdom no refleja la propiedad IDL
    // `inert` a un atributo del DOM (`hasAttribute('inert')`/`[inert]` se
    // quedan en false pase lo que pase), así que la búsqueda del ancestro
    // recorre `parentElement` a mano comprobando la PROPIEDAD `.inert`, no
    // un selector de atributo.
    function ancestroInert(el: HTMLElement | null): (HTMLElement & { inert?: boolean }) | null {
      for (let n = el; n; n = n.parentElement) {
        if ((n as HTMLElement & { inert?: boolean }).inert !== undefined) return n as HTMLElement & { inert?: boolean }
      }
      return null
    }

    const canales = [canalFalso(0)]
    const fuente = fuenteFalsa({ canales: vi.fn(async (): Promise<PaginaCanales> => ({ canales, total: 1 })) })
    const { container } = render(App, { fuente })

    const cabecera = container.querySelector('header.cabecera') as HTMLElement
    expect(cabecera).not.toBeNull()
    const contenedor = ancestroInert(cabecera)
    // Contra el código pre-fix (cabecera HERMANA de <main>, sin inert
    // propio ni de ningún ancestro), ancestroInert daría null aquí.
    expect(contenedor).not.toBeNull()
    expect(contenedor).not.toBe(cabecera) // el propio <header> no lleva inert; lo hereda de un ancestro
    expect(contenedor?.inert).toBe(false)

    const abrirCanal = await screen.findByLabelText('Canal 0')
    await fireEvent.click(abrirCanal)
    await tick()

    // Mismo nodo, ahora inert: el toggle de idioma (dentro de la cabecera)
    // queda fuera del árbol de accesibilidad y del orden de tabulación.
    expect(ancestroInert(cabecera)).toBe(contenedor)
    expect(contenedor?.inert).toBe(true)
    const botonIdioma = cabecera.querySelector('button.idioma')
    expect(contenedor?.contains(botonIdioma)).toBe(true)
  })
})
