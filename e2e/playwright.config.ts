import { defineConfig, devices } from '@playwright/test'
import { hubUrl } from './support/config'

const inCI = Boolean(process.env.CI)

export default defineConfig({
  testDir: 'tests',
  globalSetup: './support/global-setup.ts',
  globalTeardown: './support/global-teardown.ts',
  // Serial on purpose: Nginx rate-limits /api/v1/auth/ and the tests share one local database.
  // Flows chain several mail waits (up to 20 s each), so the 30 s default is too tight.
  timeout: 120_000,
  expect: { timeout: 10_000 },
  workers: 1,
  fullyParallel: false,
  retries: inCI ? 1 : 0,
  reporter: inCI ? [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]] : [['list']],
  use: {
    baseURL: hubUrl,
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
