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
