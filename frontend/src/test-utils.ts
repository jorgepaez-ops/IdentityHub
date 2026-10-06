import { fireEvent, screen } from '@testing-library/react'
import { vi } from 'vitest'

export type Handler = (body: unknown, url: string) => Response | Promise<Response>

export const json = (status: number, body?: unknown, type = 'application/json') =>
  new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': type } })
export const problem = (status: number, extra: Record<string, unknown> = {}) =>
  json(status, { type: 'about:blank', title: 'Problem', status, ...extra }, 'application/problem+json')
export const profile = (overrides: Record<string, unknown> = {}) => ({
  id: 'user-id', email: 'person@example.test', displayName: 'Persona', status: 'active',
  roles: ['user', 'contabilidad.senior'], mfaEnabled: true, createdAt: '2026-01-01T00:00:00Z', ...overrides,
})

// Routes fetch by "METHOD path"; unmatched calls fail loudly so a test cannot pass by accident.
export function stubApi(routes: Record<string, Handler>) {
  const calls: { key: string; body: unknown; url: string }[] = []
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    const key = `${init?.method ?? 'GET'} ${url.split('?')[0]}`
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    calls.push({ key, body, url })
    const handler = routes[key]
    if (!handler) throw new Error(`unexpected request ${key}`)
    return handler(body, url)
  })
  vi.stubGlobal('fetch', fetch)
  return { calls, fetch }
}

export const signedOut = { 'POST /api/v1/auth/refresh': () => problem(401) }
export const signedIn = (user = profile()) => ({
  'POST /api/v1/auth/refresh': () => json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }),
  'GET /api/v1/me': () => json(200, user),
})
export const type = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } })
export const goTo = (path: string) => window.history.replaceState({}, '', path)

const appId = '11111111-1111-4111-8111-111111111111'
const role = (id: string, name: string, permissionKeys: string[], assignedCount: number, description = '') =>
  ({ id, name, description, applicationId: appId, permissionKeys, system: false, assignedCount })
// RF-021: one application with its permission catalog and three configurable roles.
export const applicationsFixture = () => [{
  id: appId,
  clientId: 'contabilidad',
  name: 'Contabilidad',
  permissions: [
    { key: 'movimientos.ver_todos', description: 'Ver todos los movimientos contables' },
    { key: 'movimientos.aprobar', description: 'Aprobar movimientos' },
    { key: 'cierre.ejecutar', description: 'Ejecutar el cierre contable' },
    { key: 'reportes.ver', description: 'Ver reportes consolidados' },
    { key: 'movimientos.registrar', description: 'Registrar movimientos' },
  ],
  roles: [
    role('22222222-2222-4222-8222-222222222221', 'contabilidad.senior', ['movimientos.ver_todos', 'movimientos.aprobar', 'cierre.ejecutar', 'reportes.ver', 'movimientos.registrar'], 1, 'Acceso completo'),
    role('22222222-2222-4222-8222-222222222222', 'contabilidad.analista', ['movimientos.registrar'], 2, 'Registra movimientos propios'),
    role('22222222-2222-4222-8222-222222222223', 'contabilidad.auditor', ['movimientos.ver_todos', 'reportes.ver'], 0, 'Solo lectura'),
  ],
}]
export const applicationsRoute = { 'GET /api/v1/admin/applications': () => json(200, applicationsFixture()) }
