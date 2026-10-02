import { psql } from './db'

/**
 * Disables every account this suite created (e2e-*@example.test) so the admins seeded with the
 * password committed in this repo cannot be used to sign in once the run is over. Rows are never
 * deleted (audit_log references them); the pattern is the only thing the statement can touch, and
 * the target database is the one support/db.ts points at.
 */
export default async function globalTeardown(): Promise<void> {
  try {
    psql(
      `UPDATE users SET status = 'disabled'
       WHERE email LIKE :'pattern' AND status <> 'disabled';`,
      { pattern: 'e2e-%@example.test' },
    )
  } catch (error) {
    // A failed cleanup must not turn a green run red; say so loudly instead.
    console.warn(`E2E teardown: could not disable e2e-*@example.test accounts: ${String(error)}`)
  }
}
