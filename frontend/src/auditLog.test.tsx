import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import { type Handler, goTo, json, problem, profile, signedIn, stubApi, applicationsRoute } from './test-utils'

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const ANA = 'aaaaaaaa-1111-4222-8333-444444444444'
const BETO = 'bbbbbbbb-1111-4222-8333-444444444444'
const DAY_MS = 24 * 60 * 60 * 1000

const event = (id: number, overrides: Record<string, unknown> = {}) =>
  ({ id, actorUserId: ANA, action: 'login_failed', resourceType: null, resourceId: null, ip: null, userAgent: null, metadata: {}, createdAt: '2026-10-01T10:00:00Z', ...overrides })
const paramsOf = (url: string) => new URL(url, 'http://localhost').searchParams
const auditCalls = (api: { calls: { key: string; url: string }[] }) => api.calls.filter((call) => call.key === 'GET /api/v1/admin/audit-log')
const lastParams = (api: { calls: { key: string; url: string }[] }) => paramsOf(auditCalls(api).at(-1)?.url ?? '')
const emailLookup = (id: string, email: string): Record<string, Handler> => ({ [`GET /api/v1/admin/users/${id}`]: () => json(200, profile({ id, email })) })

const open = async (path = '/auditoria', extra: Record<string, Handler> = {}) => {
  goTo(path)
  const api = stubApi({
    ...signedIn(adminUser),
    ...applicationsRoute,
    'GET /api/v1/admin/audit-log': () => json(200, { items: [event(1)], nextCursor: null }),
    ...emailLookup(ANA, 'ana@example.test'),
    ...extra,
  })
  render(<App />)
  await screen.findByRole('heading', { name: 'Auditoría' })
  await screen.findByRole('table', { name: 'Registro de auditoría' })
  return api
}
const shortcut = (name: string) => within(screen.getByRole('group', { name: 'Atajos de filtro' })).getByRole('button', { name })

afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  goTo('/')
})

describe('audit log shortcuts', () => {
  it.each([
    ['Inicios fallidos', 'action', 'login_failed', '?accion=login_failed'],
    ['Códigos MFA rechazados', 'action', 'mfa_code_rejected', '?accion=mfa_code_rejected'],
    ['Cambios de roles', 'action', 'role_changed', '?accion=role_changed'],
  ])('TestRF011_ShortcutSendsTheRightQuery %s', async (name, param, value, search) => {
    const api = await open()
    fireEvent.click(shortcut(name))
    await waitFor(() => expect(lastParams(api).get(param)).toBe(value))
    expect(window.location.search).toBe(search)
    expect(shortcut(name)).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(shortcut(name))
    await waitFor(() => expect(lastParams(api).has(param)).toBe(false))
    expect(window.location.search).toBe('')
    expect(shortcut(name)).toHaveAttribute('aria-pressed', 'false')
  })

  it('TestRF011_Last24HoursShortcutSendsASinceOneDayAgoAndCombinesWithAnAction', async () => {
    const api = await open()
    fireEvent.click(shortcut('Inicios fallidos'))
    fireEvent.click(shortcut('Últimas 24 h'))
    await waitFor(() => expect(lastParams(api).has('since')).toBe(true))
    expect(lastParams(api).get('action')).toBe('login_failed')
    expect(Math.abs(Date.now() - DAY_MS - new Date(lastParams(api).get('since') ?? '').getTime())).toBeLessThan(60_000)
    expect(window.location.search).toBe('?accion=login_failed&desde=24h')
    expect(shortcut('Últimas 24 h')).toHaveAttribute('aria-pressed', 'true')
  })
})

describe('audit log URL filters', () => {
  it('TestRF011_PreselectsTheFiltersFromTheUrl', async () => {
    const api = await open(`/auditoria?accion=login_failed&actor=${ANA}&desde=2026-10-01T08:00`)
    const params = lastParams(api)
    expect(params.get('action')).toBe('login_failed')
    expect(params.get('actorId')).toBe(ANA)
    expect(new Date(params.get('since') ?? '').getTime()).toBe(new Date('2026-10-01T08:00').getTime())
    expect(screen.getByLabelText('Acción')).toHaveValue('login_failed')
    expect(screen.getByLabelText('Actor (ID)')).toHaveValue(ANA)
    expect(screen.getByLabelText('Desde')).toHaveValue('2026-10-01T08:00')
    expect(shortcut('Inicios fallidos')).toHaveAttribute('aria-pressed', 'true')
  })

  it('TestRF011_IgnoresInvalidUrlFilters', async () => {
    const api = await open('/auditoria?actor=no-es-uuid&desde=ayer')
    const params = lastParams(api)
    expect(params.has('actorId')).toBe(false)
    expect(params.has('since')).toBe(false)
  })

  it('TestRF011_TheLast24HoursUrlValueIsResolvedAtQueryTime', async () => {
    const api = await open('/auditoria?accion=login_failed&desde=24h')
    expect(Math.abs(Date.now() - DAY_MS - new Date(lastParams(api).get('since') ?? '').getTime())).toBeLessThan(60_000)
    expect(shortcut('Últimas 24 h')).toHaveAttribute('aria-pressed', 'true')
  })

  it('TestRF011_ChangingAFilterResetsTheCursor', async () => {
    const api = await open('/auditoria', {
      'GET /api/v1/admin/audit-log': (_body, url) => {
        const params = paramsOf(url)
        if (params.get('cursor') === 'old') return json(200, { items: [event(5)], nextCursor: null })
        return json(200, { items: [event(params.has('action') ? 2 : 1)], nextCursor: params.has('action') ? 'new' : 'old' })
      },
    })
    fireEvent.click(await screen.findByRole('button', { name: 'Cargar más' }))
    await waitFor(() => expect(lastParams(api).get('cursor')).toBe('old'))
    fireEvent.click(shortcut('Inicios fallidos'))
    await waitFor(() => expect(lastParams(api).get('action')).toBe('login_failed'))
    expect(lastParams(api).has('cursor')).toBe(false)
    fireEvent.click(await screen.findByRole('button', { name: 'Cargar más' }))
    await waitFor(() => expect(lastParams(api).get('cursor')).toBe('new'))
  })

  it('TestRF011_FilterByThisActorSetsTheActorFilter', async () => {
    const api = await open('/auditoria', {
      'GET /api/v1/admin/audit-log': () => json(200, { items: [event(1), event(2, { actorUserId: null })], nextCursor: null }),
    })
    const buttons = screen.getAllByRole('button', { name: /Filtrar por este actor/ })
    expect(buttons).toHaveLength(1)
    fireEvent.click(buttons[0] as HTMLElement)
    await waitFor(() => expect(lastParams(api).get('actorId')).toBe(ANA))
    expect(window.location.search).toBe(`?actor=${ANA}`)
    expect(screen.getByLabelText('Actor (ID)')).toHaveValue(ANA)
  })
})

describe('audit log actors', () => {
  it('TestRF011_ResolvesEachActorEmailOnlyOnce', async () => {
    const page = (items: unknown[], nextCursor: string | null) => json(200, { items, nextCursor })
    const api = await open('/auditoria', {
      'GET /api/v1/admin/audit-log': (_body, url) => (paramsOf(url).get('cursor')
        ? page([event(4, { actorUserId: BETO }), event(5)], null)
        : page([event(1), event(2), event(3, { actorUserId: BETO })], 'next')),
      ...emailLookup(BETO, 'beto@example.test'),
    })
    const table = screen.getByRole('table', { name: 'Registro de auditoría' })
    expect((await within(table).findAllByText('ana@example.test')).length).toBe(2)
    expect(await within(table).findByText('beto@example.test')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    await waitFor(() => expect(within(table).getAllByText('beto@example.test')).toHaveLength(2))
    const lookups = (id: string) => api.calls.filter((call) => call.key === `GET /api/v1/admin/users/${id}`)
    expect(lookups(ANA)).toHaveLength(1)
    expect(lookups(BETO)).toHaveLength(1)
  })

  it('TestRF011_FallsBackToAShortIdWhenTheLookupFailsAndToSystemWithoutActor', async () => {
    await open('/auditoria', {
      'GET /api/v1/admin/audit-log': () => json(200, { items: [event(1, { actorUserId: BETO }), event(2, { actorUserId: null }), event(3)], nextCursor: null }),
      [`GET /api/v1/admin/users/${BETO}`]: () => problem(404),
    })
    const table = screen.getByRole('table', { name: 'Registro de auditoría' })
    expect(await within(table).findByText('ana@example.test')).toBeInTheDocument()
    expect(within(table).getByText('bbbbbbbb')).toHaveAttribute('title', BETO)
    expect(within(table).getByText('Sistema')).toBeInTheDocument()
  })

  it('TestRF011_DoesNotBlockTheTableWhileEmailsLoad', async () => {
    let release: (response: Response) => void = () => undefined
    const pending = new Promise<Response>((resolve) => { release = resolve })
    await open('/auditoria', { [`GET /api/v1/admin/users/${ANA}`]: () => pending })
    const table = screen.getByRole('table', { name: 'Registro de auditoría' })
    expect(within(table).getByText('aaaaaaaa')).toBeInTheDocument()
    await act(async () => { release(json(200, profile({ id: ANA, email: 'ana@example.test' }))) })
    expect(await within(table).findByText('ana@example.test')).toBeInTheDocument()
  })
})

describe('audit log metadata', () => {
  it('TestRF011_RendersMetadataAsAKeyValueListWithNestedValues', async () => {
    await open('/auditoria', {
      'GET /api/v1/admin/audit-log': () => json(200, {
        items: [event(1, { metadata: { reason: 'bad password', roles: ['user', 'admin'], attempt: 3, flags: { mfa: true, geo: { country: 'MX' } }, note: '<img src=x onerror=alert(1)>' } })],
        nextCursor: null,
      }),
    })
    const list = screen.getByRole('table', { name: 'Registro de auditoría' }).querySelector('dl.metadata-list') as HTMLElement
    const pairs = Object.fromEntries([...list.querySelectorAll('dt')].map((term) => [term.textContent, term.nextElementSibling?.textContent]))
    expect(pairs).toEqual({
      reason: 'bad password',
      roles: 'user, admin',
      attempt: '3',
      'flags.mfa': 'true',
      'flags.geo.country': 'MX',
      note: '<img src=x onerror=alert(1)>',
    })
    expect(document.querySelector('img')).toBeNull()
  })

  it('TestRF011_KeepsBothRowsWhenMetadataKeysFlattenToTheSameName', async () => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    await open('/auditoria', {
      'GET /api/v1/admin/audit-log': () => json(200, { items: [event(1, { metadata: { 'a.b': 'literal', a: { b: 'nested' } } })], nextCursor: null }),
    })
    const list = screen.getByRole('table', { name: 'Registro de auditoría' }).querySelector('dl.metadata-list') as HTMLElement
    expect([...list.querySelectorAll('dd')].map((value) => value.textContent)).toEqual(['literal', 'nested'])
    expect(errors.mock.calls.some((call) => String(call[0]).includes('same key'))).toBe(false)
    errors.mockRestore()
  })

  it('TestRF011_AShortcutClearsAStaleActorError', async () => {
    await open()
    fireEvent.change(screen.getByLabelText('Actor (ID)'), { target: { value: 'not-a-uuid' } })
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    expect(await screen.findByText('El ID del actor debe ser un UUID.')).toBeInTheDocument()
    fireEvent.click(shortcut('Inicios fallidos'))
    await waitFor(() => expect(screen.queryByText('El ID del actor debe ser un UUID.')).not.toBeInTheDocument())
  })

  it('TestRF011_AValidResubmitClearsTheActorErrorEvenWithUnchangedFilters', async () => {
    await open()
    fireEvent.change(screen.getByLabelText('Actor (ID)'), { target: { value: 'not-a-uuid' } })
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    expect(await screen.findByText('El ID del actor debe ser un UUID.')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Actor (ID)'), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(screen.queryByText('El ID del actor debe ser un UUID.')).not.toBeInTheDocument())
  })

  it('TestRF011_ResubmittingUnchangedFiltersReloadsTheLog', async () => {
    const api = await open('/auditoria?accion=login_failed&desde=24h')
    await waitFor(() => expect(auditCalls(api)).toHaveLength(1))
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(auditCalls(api)).toHaveLength(2))
    expect(lastParams(api).get('action')).toBe('login_failed')
    fireEvent.change(screen.getByLabelText('Acción'), { target: { value: 'role_changed' } })
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(lastParams(api).get('action')).toBe('role_changed'))
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(auditCalls(api)).toHaveLength(3)
  })
})
