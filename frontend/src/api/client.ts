/**
 * Cliente HTTP.
 *
 * A partir de la semana 2 los tipos de este archivo vienen de
 * `src/api/schema.d.ts`, generado con `npm run gen:api` desde
 * specs/03-api/openapi.yaml. Hasta entonces se declaran a mano solo los dos
 * que necesita el esqueleto.
 */

export interface Health {
  status: 'ok'
  version: string
}

export interface Readiness {
  status: 'ready' | 'degraded'
  checks: Record<string, { status: 'up' | 'down'; error?: string }>
}

async function request<T>(path: string): Promise<T> {
  const res = await fetch(path, {
    headers: { Accept: 'application/json' },
    // Los tokens de sesión viajarán en cookie HttpOnly, no en localStorage,
    // para que un XSS no pueda leerlos (AM-015).
    credentials: 'same-origin',
  })

  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }
  return (await res.json()) as T
}

// Nginx proxea /healthz y /readyz hacia la API igual que /api/v1.
export const getHealth = () => request<Health>('/healthz')
export const getReadiness = () => request<Readiness>('/readyz')
