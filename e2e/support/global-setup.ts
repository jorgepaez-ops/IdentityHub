import { seedAdmin } from './accounts'

/**
 * Fails fast, with one actionable message, when the API is rejecting logins from this IP
 * (RF-017 per-IP limit). Otherwise every test would fail later with a bare 423.
 */
export default async function globalSetup(): Promise<void> {
  try {
    await seedAdmin()
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    if (message.includes('returned 423')) {
      throw new Error(
        'The API rejects logins from this IP (per-IP failure limit, RF-017). Run the suite with ' +
          '"make e2e", which raises LOGIN_IP_MAX_FAILURES while it runs, or wait for LOGIN_FAILURE_WINDOW to pass.',
      )
    }
    throw error
  }
}
