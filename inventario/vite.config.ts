import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// One config for the dev server, the production build and Vitest.
export default defineConfig({
  plugins: [react()],
  server: { port: 5175 },
  // No source maps in production, like the Hub console.
  build: { outDir: 'dist', sourcemap: false },
  test: {
    environment: 'jsdom',
    globals: true,
    // A zone behind UTC makes "late in the day" differ between local time and UTC, so date bugs show up.
    env: { TZ: 'America/Bogota' },
    setupFiles: ['./src/vitest.setup.ts'],
    coverage: { include: ['src/**/*.{ts,tsx}'], reporter: ['text', 'lcov'] },
  },
})
