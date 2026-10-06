import { defineConfig, devices } from '@playwright/test'
import { hubUrl } from '../support/config'

// Screenshot generator for docs/manuales/usuario.md. It is deliberately NOT part of "make e2e":
// the main config only looks in tests/ for *.spec.ts, and scripts/traceability.py reads only
// e2e/tests/*.spec.ts, so nothing here counts as a requirement test.
export default defineConfig({
  testDir: '.',
  testMatch: /capturas\.ts$/,
  // Reuse the suite teardown: every account created here is e2e-*@example.test and gets disabled.
  globalTeardown: '../support/global-teardown.ts',
  outputDir: '../test-results/manual',
  timeout: 900_000,
  expect: { timeout: 15_000 },
  workers: 1,
  fullyParallel: false,
  retries: 0,
  reporter: [['list']],
  use: {
    ...devices['Desktop Chrome'],
    baseURL: hubUrl,
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    actionTimeout: 30_000,
    locale: 'es-CO',
    colorScheme: 'light',
  },
  projects: [{ name: 'chromium' }],
})
