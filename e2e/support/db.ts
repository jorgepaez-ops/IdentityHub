import { execFileSync } from 'node:child_process'

const container = process.env.E2E_DB_CONTAINER ?? 'identity-hub-db-1'
const dbUser = process.env.E2E_DB_USER ?? 'identity'
const dbName = process.env.E2E_DB_NAME ?? 'identity'
// Optional override, e.g. "docker compose exec -T db psql" is not supported; keep it a single binary.
const docker = process.env.E2E_DOCKER ?? 'docker'

const EMAIL = /^[a-z0-9][a-z0-9._+-]*@example\.test$/i
const HEX = /^[0-9a-f]+$/i

export function assertEmail(value: string): string {
  if (!EMAIL.test(value)) throw new Error(`Refusing non-test email: ${value}`)
  return value
}

export function assertHex(value: string): string {
  if (!HEX.test(value)) throw new Error('Expected a hex string')
  return value
}

/**
 * Runs a SQL script through psql inside the database container. Untrusted values travel as psql
 * variables (-v) and are referenced as :'name' in the script, so nothing is interpolated in a shell.
 */
export function psql(script: string, vars: Record<string, string> = {}): string {
  const args = ['exec', '-i', container, 'psql', '-U', dbUser, '-d', dbName, '-X', '-q', '-t', '-A', '-v', 'ON_ERROR_STOP=1']
  for (const [key, value] of Object.entries(vars)) {
    if (!/^[a-z_][a-z0-9_]*$/i.test(key)) throw new Error(`Invalid psql variable name: ${key}`)
    args.push('-v', `${key}=${value}`)
  }
  // execFileSync blocks the worker, so Playwright's own test timeout cannot interrupt a hung
  // docker exec; bound it here instead.
  return execFileSync(docker, args, { input: script, encoding: 'utf8', timeout: 30_000 }).trim()
}
