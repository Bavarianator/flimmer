import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'
import legacy from '@vitejs/plugin-legacy'

export default defineConfig({
  plugins: [
    preact(),
    // webOS 4 = Chromium 53, Tizen 4 = Chromium 56: bekommen ein transpiliertes Bundle mit Polyfills.
    legacy({ targets: ['chrome >= 53', 'safari >= 12', 'defaults'] }),
  ],
  build: { outDir: 'dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://localhost:8096' } },
})
