import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // En desarrollo Vite hace de proxy hacia la API. En producción ese papel lo
    // cumple Nginx (frontend/nginx/default.conf), que además es donde viven las
    // cabeceras de seguridad y el rate limiting.
    proxy: {
      '/api': { target: 'http://localhost:8081', changeOrigin: true },
      '/.well-known': { target: 'http://localhost:8081', changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    // Sin sourcemaps en producción: publicarlos entrega el código original y
    // facilita encontrar lógica que no debería ser evidente desde el cliente.
    sourcemap: false,
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    coverage: { reporter: ['text', 'lcov'], include: ['src/**/*.{ts,tsx}'] },
  },
})
