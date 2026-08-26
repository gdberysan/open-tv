/// <reference types="node" />
import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

// Tarea 7 (P0.8): pase de micro-interacciones. jsdom no ejecuta animaciones
// de verdad — el gate real de "se ve fluido" lo hace el controlador en
// Chrome (visual). Lo único atrapable aquí, en el propio texto fuente, es
// que (a) ninguna duración/curva de easing quede hardcodeada — todo sale de
// los tokens de estilos/tokens/effects.css (--dur-*/--ease-*) — y (b) que
// cada regla de movimiento que este pase añade tenga su contrapartida en un
// bloque `@media (prefers-reduced-motion: reduce)` que la anule. Solo se
// listan aquí los ficheros que este pase TOCA: exigir esto retroactivamente
// a movimiento preexistente fuera de alcance (p.ej. el parpadeo de
// .cursor-surf en App.svelte, ya con su propio guard) no es esta tarea.
//
// `node:fs` en vez de un import `?raw` de Vite (que en este runtime
// devuelve '' para .css — comprobado): la referencia de tipos de arriba es
// LOCAL a este fichero (no toca tsconfig.app.json, que a propósito no trae
// los tipos de `node` para el resto del paquete cliente); @types/node ya es
// devDependency del repo.
const aqui = dirname(fileURLToPath(import.meta.url))

function leer(rutaRelativa: string): string {
  return readFileSync(resolve(aqui, rutaRelativa), 'utf-8')
}

// Extrae el VALOR de cada declaración `transition:`/`animation:` (una por
// punto y coma) del texto fuente — en estos ficheros solo aparecen dentro
// de <style>, así que basta con buscar en el texto completo.
function declaraciones(fuente: string, propiedad: 'transition' | 'animation'): string[] {
  const patron = new RegExp(`${propiedad}:\\s*([^;]+);`, 'g')
  return [...fuente.matchAll(patron)].map((m) => m[1].trim())
}

const FICHEROS = [
  '../componentes/TarjetaCanal.svelte',
  './global.css',
  '../componentes/ChipsFiltro.svelte',
  '../componentes/BarraLateralFacetas.svelte',
  '../componentes/Ajustes.svelte',
  '../componentes/Fuentes.svelte',
  '../componentes/PanelStats.svelte',
  '../componentes/Paleta.svelte',
  '../componentes/Reproductor.svelte',
]

describe('micro-interacciones: tokens, nunca valores mágicos', () => {
  it.each(FICHEROS)('%s: toda transition/animation usa var(--dur-…) (ninguna duración en crudo)', (ruta) => {
    const fuente = leer(ruta)
    const todas = [...declaraciones(fuente, 'transition'), ...declaraciones(fuente, 'animation')]
      .filter((valor) => valor !== 'none')
    // Un fichero tocado por este pase debe aportar AL MENOS una declaración
    // de movimiento — si no hay ninguna, el fichero no necesitaba estar en
    // esta lista.
    expect(todas.length).toBeGreaterThan(0)
    for (const valor of todas) {
      expect(valor, `"${valor}" no referencia var(--dur-…)`).toMatch(/var\(--dur-/)
    }
  })

  it.each(FICHEROS)('%s: toda transition/animation usa var(--ease-…) (ninguna curva en crudo)', (ruta) => {
    const fuente = leer(ruta)
    const todas = [...declaraciones(fuente, 'transition'), ...declaraciones(fuente, 'animation')]
      .filter((valor) => valor !== 'none')
    for (const valor of todas) {
      expect(valor, `"${valor}" no referencia var(--ease-…)`).toMatch(/var\(--ease-/)
    }
  })

  it.each(FICHEROS)('%s: define un @media (prefers-reduced-motion: reduce) que anula lo añadido', (ruta) => {
    const fuente = leer(ruta)
    expect(fuente).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)/)
  })
})
