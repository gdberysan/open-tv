/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// El cliente se construye DENTRO del paquete Go que lo embebe: go:embed no
// admite "..", así que web/dist no serviría. El binario y el cliente son un
// solo artefacto y esto es lo que lo hace literal.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/ui/dist',
    emptyOutDir: true,
    // El presupuesto es 80 KB gzip de JS propio (spec §3.1). hls.js va en su
    // propio trozo y solo se descarga al reproducir.
    chunkSizeWarningLimit: 100,
  },
  server: {
    // En desarrollo el cliente corre en 5173 y la API en 8080. Sin este proxy
    // el navegador haría peticiones cruzadas — y el CORS se retiró a
    // propósito, así que fallarían. En producción todo es el mismo origen.
    proxy: {
      '/channels': 'http://127.0.0.1:8080',
      '/health': 'http://127.0.0.1:8080',
      '/proxy': 'http://127.0.0.1:8080',
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
  },
})
