import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import { type Handler, goTo, json, problem, profile, signedIn, stubApi } from './test-utils'

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const DAY_MS = 24 * 60 * 60 * 1000

const event = (id: number, action: string, createdAt: string, extra: Record<string, unknown> = {}) =>
  ({ id, actorUserId: null, action, resourceType: null, resourceId: null, ip: null, userAgent: null, metadata: {}, createdAt, ...extra })
const users = (count: number) => Array.from({ length: count }, (_, index) => profile({ id: `u-${index}`, email: `u${index}@example.test` }))

const byStatus = (counts: Record<string, { count: number; more?: boolean }>): Handler => (_body, url) => {
  const status = new URL(url, 'http://localhost').searchParams.get('status') ?? ''
  const entry = counts[status] ?? { count: 0 }
  return json(200, { items: users(entry.count), nextCursor: entry.more ? 'next' : null })
}
const audit = (failed: ReturnType<typeof event>[], recent: ReturnType<typeof event>[]): Handler => (_body, url) => {
  const params = new URL(url, 'http://localhost').searchParams
  const action = params.get('action')
  // mfa_code_rejected has no events unless a test overrides the whole handler.
  return json(200, { items: action === 'login_failed' ? failed : action === 'mfa_code_rejected' ? [] : recent, nextCursor: null })
}

const homeApi = (extra: Record<string, Handler> = {}, user = adminUser) => stubApi({
  ...signedIn(user),
  'GET /api/v1/admin/users': byStatus({ active: { count: 7 }, locked: { count: 2 }, disabled: { count: 1 }, pending_verification: { count: 3 } }),
  'GET /api/v1/admin/audit-log': audit([event(1, 'login_failed', '2026-10-06T08:00:00Z')], [event(2, 'role_assigned', '2026-10-06T09:00:00Z')]),
  ...extra,
})
const tile = (name: string) => screen.getByRole('group', { name })

afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  goTo('/')
})

describe('console home', () => {
  it('TestRF010_AdminLandsOnTheHomePageFromTheCatchAllRoute', async () => {
    goTo('/')
    homeApi()
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Inicio' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/inicio')
    expect(screen.getByRole('link', { name: 'Inicio' })).toHaveAttribute('href', '/inicio')
  })

  it('TestRF010_AdminLandsOnTheHomePageFromTheLoginRoute', async () => {
    goTo('/login')
    homeApi()
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Inicio' })).toBeInTheDocument()
    expect(window.location.pathname).toBe('/inicio')
  })

  it('TestRF010_ShowsTheUserCountPerStatusUsingTheStatusFilter', async () => {
    goTo('/inicio')
    const api = homeApi()
    render(<App />)
    await screen.findByRole('heading', { name: 'Inicio' })
    expect(await within(await screen.findByRole('group', { name: 'Activos' })).findByText('7')).toBeInTheDocument()
    expect(within(tile('Bloqueados')).getByText('2')).toBeInTheDocument()
    expect(within(tile('Deshabilitados')).getByText('1')).toBeInTheDocument()
    expect(within(tile('Pendientes')).getByText('3')).toBeInTheDocument()
    const statuses = api.calls.filter((call) => call.key === 'GET /api/v1/admin/users').map((call) => new URL(call.url, 'http://localhost').searchParams.get('status')).sort()
    expect(statuses).toEqual(['active', 'disabled', 'locked', 'pending_verification'])
  })

  it('TestRF010_ShowsAPlusSignWhenMorePagesExist', async () => {
    goTo('/inicio')
    homeApi({ 'GET /api/v1/admin/users': byStatus({ active: { count: 100, more: true } }) })
    render(<App />)
    expect(await within(await screen.findByRole('group', { name: 'Activos' })).findByText('100+')).toBeInTheDocument()
    expect(within(tile('Bloqueados')).getByText('0')).toBeInTheDocument()
  })

  it('TestRF011_CountsFailedSignInsFromTheLast24HoursOnly', async () => {
    goTo('/inicio')
    const before = Date.now()
    const api = homeApi({ 'GET /api/v1/admin/audit-log': audit([event(1, 'login_failed', '2026-10-06T08:00:00Z'), event(3, 'login_failed', '2026-10-06T08:30:00Z')], []) })
    render(<App />)
    expect(await within(await screen.findByRole('group', { name: 'Inicios de sesión fallidos (24 h)' })).findByText('2')).toBeInTheDocument()
    const call = api.calls.find((item) => new URL(item.url, 'http://localhost').searchParams.get('action') === 'login_failed')
    const params = new URL(call?.url ?? '', 'http://localhost').searchParams
    expect(params.get('limit')).toBe('100')
    const since = Date.parse(params.get('since') ?? '')
    expect(since).toBeGreaterThanOrEqual(before - DAY_MS - 1000)
    expect(since).toBeLessThanOrEqual(Date.now() - DAY_MS + 1000)
  })

  it('TestRF011_ShowsAPlusSignForManyFailedSignIns', async () => {
    goTo('/inicio')
    homeApi({ 'GET /api/v1/admin/audit-log': (_body, url) => {
      const failed = new URL(url, 'http://localhost').searchParams.get('action') === 'login_failed'
      const items = failed ? Array.from({ length: 100 }, (_, index) => event(index + 10, 'login_failed', '2026-10-06T08:00:00Z')) : []
      return json(200, { items, nextCursor: failed ? 'more' : null })
    } })
    render(<App />)
    expect(await within(await screen.findByRole('group', { name: 'Inicios de sesión fallidos (24 h)' })).findByText('100+')).toBeInTheDocument()
  })

  it('TestRF011_CountsRejectedMfaCodesAsFailedSignIns', async () => {
    goTo('/inicio')
    const api = homeApi({ 'GET /api/v1/admin/audit-log': (_body, url) => {
      const action = new URL(url, 'http://localhost').searchParams.get('action')
      if (action === 'login_failed') return json(200, { items: [event(1, 'login_failed', '2026-10-06T08:00:00Z')], nextCursor: null })
      if (action === 'mfa_code_rejected') return json(200, { items: [event(2, 'mfa_code_rejected', '2026-10-06T08:10:00Z'), event(3, 'mfa_code_rejected', '2026-10-06T08:20:00Z')], nextCursor: 'more' })
      return json(200, { items: [], nextCursor: null })
    } })
    render(<App />)
    expect(await within(await screen.findByRole('group', { name: 'Inicios de sesión fallidos (24 h)' })).findByText('3+')).toBeInTheDocument()
    const rejected = api.calls.find((item) => new URL(item.url, 'http://localhost').searchParams.get('action') === 'mfa_code_rejected')
    expect(new URL(rejected?.url ?? '', 'http://localhost').searchParams.has('since')).toBe(true)
  })

  it('TestRF011_ListsTheRecentActivity', async () => {
    goTo('/inicio')
    const api = homeApi({ 'GET /api/v1/admin/audit-log': audit([], [event(2, 'role_assigned', '2026-10-06T09:00:00Z'), event(5, 'user_created', '2026-10-06T08:00:00Z')]) })
    render(<App />)
    const list = await screen.findByRole('list', { name: 'Actividad reciente' })
    expect(within(list).getAllByRole('listitem')).toHaveLength(2)
    expect(within(list).getByText('role_assigned')).toBeInTheDocument()
    expect(within(list).getByText('user_created')).toBeInTheDocument()
    const recent = api.calls.find((item) => item.key === 'GET /api/v1/admin/audit-log' && !new URL(item.url, 'http://localhost').searchParams.has('action'))
    expect(new URL(recent?.url ?? '', 'http://localhost').searchParams.get('limit')).toBe('10')
  })

  it('TestRF011_ShowsAnEmptyMessageWithoutActivity', async () => {
    goTo('/inicio')
    homeApi({ 'GET /api/v1/admin/audit-log': audit([], []) })
    render(<App />)
    expect(await screen.findByText('Todavía no hay actividad registrada.')).toBeInTheDocument()
  })

  it('TestRF010_LinksEachUserStatusTileToTheDirectoryFilteredByThatStatus', async () => {
    goTo('/inicio')
    homeApi()
    render(<App />)
    await within(await screen.findByRole('group', { name: 'Activos' })).findByText('7')
    const hrefs = {
      Activos: '/usuarios?estado=active',
      Pendientes: '/usuarios?estado=pending_verification',
      Bloqueados: '/usuarios?estado=locked',
      Deshabilitados: '/usuarios?estado=disabled',
    }
    for (const [name, href] of Object.entries(hrefs)) {
      expect(within(tile(name)).getByRole('link', { name })).toHaveAttribute('href', href)
    }
  })

  it('TestRF010_LeavesTheFailedSignInsTileUnlinkedUntilTheAuditPageReadsFilters', async () => {
    goTo('/inicio')
    homeApi()
    render(<App />)
    await within(await screen.findByRole('group', { name: 'Activos' })).findByText('7')
    expect(within(tile('Inicios de sesión fallidos (24 h)')).queryByRole('link')).not.toBeInTheDocument()
  })

  it('TestRF010_OpensTheFilteredDirectoryFromAHomeTile', async () => {
    goTo('/inicio')
    const api = homeApi()
    render(<App />)
    fireEvent.click(await within(await screen.findByRole('group', { name: 'Bloqueados' })).findByRole('link', { name: 'Bloqueados' }))
    expect(await screen.findByRole('heading', { name: 'Usuarios' })).toBeInTheDocument()
    expect(window.location.pathname + window.location.search).toBe('/usuarios?estado=locked')
    expect(screen.getByRole('button', { name: 'Bloqueados' })).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(api.calls.filter((call) => call.key === 'GET /api/v1/admin/users').at(-1)?.url).toContain('status=locked'))
  })

  it('TestRF010_OffersShortcutsToUsersRolesAndAudit', async () => {
    goTo('/inicio')
    homeApi()
    render(<App />)
    const shortcuts = await screen.findByRole('region', { name: 'Accesos directos' })
    expect(within(shortcuts).getByRole('link', { name: /Usuarios/ })).toHaveAttribute('href', '/usuarios')
    expect(within(shortcuts).getByRole('link', { name: /Roles/ })).toHaveAttribute('href', '/roles')
    expect(within(shortcuts).getByRole('link', { name: /Auditoría/ })).toHaveAttribute('href', '/auditoria')
  })

  it('TestRF009_RedirectsNonAdminsFromTheHomeRoute', async () => {
    goTo('/inicio')
    const api = stubApi({ ...signedIn(profile()), 'GET /api/v1/me/sessions': () => json(200, []) })
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Inicio' })).not.toBeInTheDocument()
    expect(api.calls.some((call) => call.key.includes('/admin/'))).toBe(false)
  })

  it('TestRF010_ShowsTheErrorNoticeWhenAnIndicatorCannotBeLoaded', async () => {
    goTo('/inicio')
    homeApi({ 'GET /api/v1/admin/audit-log': () => problem(500) })
    render(<App />)
    expect(await screen.findByText('No fue posible cargar los indicadores. Inténtalo de nuevo.')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('TestRF010_KeepsASingleStatusRegionWhileLoadingAndAfter', async () => {
    goTo('/inicio')
    homeApi()
    render(<App />)
    await screen.findByRole('heading', { name: 'Inicio' })
    expect(screen.queryAllByRole('status').length).toBeLessThanOrEqual(1)
    await screen.findByRole('list', { name: 'Actividad reciente' })
    expect(screen.queryAllByRole('status')).toHaveLength(0)
  })

  it('TestRF010_EndsTheSessionWhenAuthenticationKeepsFailing', async () => {
    goTo('/inicio')
    let refreshes = 0
    homeApi({
      'GET /api/v1/admin/users': () => problem(401),
      'POST /api/v1/auth/refresh': () => (++refreshes === 1 ? json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }) : problem(401)),
    })
    render(<App />)
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
  })
})
