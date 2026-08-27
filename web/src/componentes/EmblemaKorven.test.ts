/// <reference types="node" />
import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/svelte'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import EmblemaKorven from './EmblemaKorven.svelte'

// `node:fs` con la referencia de tipos LOCAL a este fichero, igual que
// estilos/movimiento.test.ts: jsdom no resuelve variables CSS ni ejecuta
// animaciones, así que lo único falsable sobre el ESTILO es el texto fuente.
const aqui = dirname(fileURLToPath(import.meta.url))
const FUENTE = readFileSync(resolve(aqui, 'EmblemaKorven.svelte'), 'utf-8')

describe('EmblemaKorven', () => {
  it('pinta el SVG del emblema', () => {
    const { container } = render(EmblemaKorven)
    const svg = container.querySelector('svg')
    expect(svg).not.toBeNull()
    expect(svg?.getAttribute('viewBox')).toBe('0 0 128 128')
  })

  it('es decorativo: aria-hidden y fuera del árbol de accesibilidad', () => {
    // El wordmark de la cabecera y el copy del panel de espera ya nombran lo
    // que el emblema acompaña — anunciarlo sería duplicar. Este test es la
    // evidencia falsable de esa decisión: si alguien le pone un aria-label
    // «por mejorar la accesibilidad», salta.
    const { container } = render(EmblemaKorven)
    const raiz = container.querySelector('.emblema')
    expect(raiz?.getAttribute('aria-hidden')).toBe('true')
    expect(raiz?.getAttribute('aria-label')).toBeNull()
    expect(container.querySelector('svg')?.getAttribute('focusable')).toBe('false')
  })

  it('conserva la geometría del emblema: hexágono, seis facetas, cursor y nodo', () => {
    // Los mismos vértices que /marca/korven-emblema.svg (el emblema plano
    // que ya usa PieDeMarca): las dos piezas tienen que seguir siendo el
    // mismo dibujo, no dos marcas parecidas.
    const { container } = render(EmblemaKorven)
    expect(container.querySelector('.canto')?.getAttribute('points'))
      .toBe('64,12 110,38 110,90 64,116 18,90 18,38')
    expect(container.querySelectorAll('.facetas polygon')).toHaveLength(6)
    expect(container.querySelector('.cursor')?.getAttribute('points')).toBe('40,52 28,64 40,76')
    expect(container.querySelector('.nodo')).not.toBeNull()
    expect(container.querySelector('.halo')).not.toBeNull()
  })

  it('ilumina el canto por aristas, no a tono plano', () => {
    // El contorno va en dos capas: el hexágono entero al tono bajo (silueta
    // cerrada) y encima las cinco aristas que la luz clave levanta. Si
    // alguien lo colapsa a un solo trazo uniforme, el emblema deja de leerse
    // como objeto iluminado y pasa a pegatina — que es el bug que este test
    // congela.
    const { container } = render(EmblemaKorven)
    expect(container.querySelectorAll('.canto-luz line')).toHaveLength(5)
    // El eje de la luz clave (60°-240°): la arista superior derecha es la
    // única a plena luz y la inferior izquierda no lleva realce ninguno.
    const luz = container.querySelectorAll('.canto-luz .c-luz')
    expect(luz).toHaveLength(1)
    expect(luz[0].getAttribute('x1')).toBe('64')
    expect(luz[0].getAttribute('y1')).toBe('12')
  })

  it('por defecto es chico y quieto; `animado` y `tamano` lo cambian', () => {
    const quieto = render(EmblemaKorven).container.querySelector('.emblema')
    expect(quieto?.classList.contains('chico')).toBe(true)
    expect(quieto?.classList.contains('animado')).toBe(false)

    const vivo = render(EmblemaKorven, { tamano: 'grande', animado: true })
      .container.querySelector('.emblema')
    expect(vivo?.classList.contains('grande')).toBe(true)
    expect(vivo?.classList.contains('animado')).toBe(true)
  })

  it('el movimiento cuelga SIEMPRE de .animado (nunca por defecto)', () => {
    // Sin esto, un descuido dejaría el emblema de la cabecera basculando en
    // toda la app — justo lo que el brief («discreto») descarta.
    for (const decl of FUENTE.matchAll(/animation:\s*([^;]+);/g)) {
      const regla = FUENTE.slice(0, decl.index).lastIndexOf('{')
      const selector = FUENTE.slice(0, regla).split('\n').pop() ?? ''
      if (decl[1].trim() === 'none') continue
      expect(selector, `"${selector}" anima sin exigir .animado`).toMatch(/\.animado/)
    }
  })

  it('las tres marcas del cliente son EL MISMO dibujo', () => {
    // Hay tres copias del emblema por razones distintas y ninguna se puede
    // fusionar: este componente (inline, para poder usar tokens y animar),
    // /marca/korven-emblema.svg (el <img> del pie) y /favicon.svg (lo pinta
    // el navegador fuera del documento). Tres ficheros sueltos derivan solos
    // en cuanto alguien retoca uno; el hexágono es lo que los ata.
    const hex = '64,12 110,38 110,90 64,116 18,90 18,38'
    const publico = (rel: string) => readFileSync(resolve(aqui, '../../public', rel), 'utf-8')
    expect(FUENTE, 'EmblemaKorven.svelte').toContain(hex)
    expect(publico('marca/korven-emblema.svg'), 'korven-emblema.svg').toContain(hex)
    expect(publico('favicon.svg'), 'favicon.svg').toContain(hex)
    // Y el favicon tiene que seguir siendo el de Korven: el proyecto arrancó
    // con el icono morado de la plantilla y estuvo así hasta P3.
    expect(publico('favicon.svg')).toContain('#FF8A2B')
    expect(publico('favicon.svg')).not.toContain('863bff')
  })

  it('no hardcodea ni un color: todo sale de tokens de marca', () => {
    // Invariante duro del proyecto («Tokens de marca: nada hardcodeado»).
    // El modelo 3D venía con los hex a pelo (0x1D2634, 0xFF8A2B…); al
    // traerlo, cada uno tiene que caer en su token.
    const estilos = FUENTE.slice(FUENTE.indexOf('<style>'))
    expect(estilos).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
    for (const token of ['--amber-500', '--graphite-050', '--graphite-200']) {
      expect(estilos, `falta ${token}`).toContain(`var(${token})`)
    }
  })
})
