import { defineConfig, devices } from '@playwright/test'

const inCI = Boolean(process.env.CI)

export default defineConfig({
  testDir: 'tests',
  // Serial on purpose: Nginx rate-limits /api/v1/auth/ and the tests share one local database.
  workers: 1,
  fullyParallel: false,
  retries: inCI ? 1 : 0,
  reporter: inCI ? [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]] : [['list']],
  use: {
    baseURL: process.env.E2E_HUB_URL ?? 'http://identityhub.localhost:8080',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
