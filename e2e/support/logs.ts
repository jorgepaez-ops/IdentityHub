import { execFileSync } from 'node:child_process'

const docker = process.env.E2E_DOCKER ?? 'docker'
const project = process.env.E2E_COMPOSE_PROJECT ?? 'identity-hub'

/** Full stdout+stderr of a compose service container (`identity-hub-<service>-1`), one entry per line. */
export function serviceLogs(service: 'api' | 'worker'): string[] {
  const name = process.env[`E2E_${service.toUpperCase()}_CONTAINER`] ?? `${project}-${service}-1`
  // docker logs writes the container's stderr to its own stderr; the services log JSON to stderr or stdout.
  const output = execFileSync(docker, ['logs', name], { encoding: 'utf8', timeout: 30_000, stdio: ['ignore', 'pipe', 'pipe'] })
  return output.split('\n').filter((line) => line.trim() !== '')
}

/** Lines of `logs` that contain any of the given secrets (empty values are ignored). */
export function linesLeaking(logs: string[], secrets: string[]): string[] {
  const needles = secrets.filter((secret) => secret.length > 0)
  return logs.filter((line) => needles.some((secret) => line.includes(secret)))
}
