import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  // El teardown NO se declara aquí: globalSetup devuelve una función y
  // Playwright la llama sola al terminar (mecanismo estándar). Apuntar
  // `globalTeardown` al mismo fichero que `globalSetup` —como hacía un
  // borrador anterior— volvería a ejecutar el setup completo (build, ffmpeg,
  // arrancar el binario) en vez de cerrarlo.
  globalSetup: './tests/e2e/global-setup.ts',
  timeout: 60_000,
  // Solo en CI: el VOD de 10 s de la fixture (5 segmentos de 2 s) se sirve
  // por loopback tan rápido que hls.js a veces adjunta el buffer a
  // MediaSource antes de que Firefox headless termine de inicializarlo —
  // confirmado en local ejecutando el mismo test en bucle: la red siempre
  // entrega los 5 segmentos con 200, pero el <video> se queda a veces en
  // readyState 0 sin ningún error que el guard pueda capturar. Es
  // flakiness real del entorno (streams reales en producción no llegan tan
  // rápido), no un fallo del player ni de este proxy; reintentar es la
  // mitigación estándar de Playwright para esto, no un "skip" disfrazado.
  // La misma carrera se dispara con muchísima más frecuencia cuando varios
  // proyectos (chromium+firefox) decodifican vídeo real a la vez y compiten
  // por CPU — confirmado también en local: en serie, la tasa de fallo baja a
  // la de arriba (~30%, la que sí cubren los reintentos); en paralelo, casi
  // nunca se recupera ni con reintentos. `workers: 1` en CI es la
  // configuración que la propia plantilla oficial de Playwright recomienda
  // para runners compartidos, y aquí además es lo que hace que los
  // reintentos sirvan de algo.
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  use: {
    baseURL: 'http://127.0.0.1:8090',
    trace: 'retain-on-failure',
    // El toggle de idioma y el aria-label del favorito solo se comprueban de
    // forma determinista si el arranque es en español: sin fijar el locale
    // del navegador, `navigator.language` depende del entorno (en CI suele
    // salir "en-US") y el catálogo arrancaría ya en inglés.
    locale: 'es-ES',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    { name: 'webkit', use: { ...devices['Desktop Safari'] } },
  ],
})
