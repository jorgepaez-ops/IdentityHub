import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import { type Handler, goTo, json, profile, signedIn, stubApi, type, applicationsRoute } from './test-utils'

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const person = (overrides: Record<string, unknown>) => profile({ mfaEnabled: true, lastLoginAt: '2026-10-01T09:30:00Z', ...overrides })
const directory = [
  adminUser,
  person({ id: 'ana-id', email: 'ana@example.test', displayName: 'Ana Pérez' }),
  person({ id: 'carla-id', email: 'carla@example.test', displayName: 'Carla Soto', status: 'locked' }),
  person({ id: 'carlos-id', email: 'carlos@example.test', displayName: 'Carlos Lara', status: 'active' }),
]
const paramsOf = (url: string) => new URL(url, 'http://localhost').searchParams
const listUsers: Handler = (_body, url) => {
  const params = paramsOf(url)
  const q = params.get('q')?.toLowerCase()
  const status = params.get('status')
  const items = directory
    .filter((item) => !status || item.status === status)
    .filter((item) => !q || `${item.email} ${item.displayName}`.toLowerCase().includes(q))
  return json(200, { items, nextCursor: null })
}
const open = async (path = '/usuarios', extra: Record<string, Handler> = {}) => {
  goTo(path)
  const api = stubApi({ ...signedIn(adminUser), ...applicationsRoute, 'GET /api/v1/admin/users': listUsers, ...extra })
  render(<App />)
  await screen.findByRole('heading', { name: 'Usuarios' })
  return api
}
const filter = (name: string) => within(screen.getByRole('group', { name: 'Filtrar por estado' })).getByRole('button', { name })
const lastUrl = (api: Awaited<ReturnType<typeof open>>) => api.calls.filter((call) => call.key === 'GET /api/v1/admin/users').at(-1)?.url ?? ''

afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  goTo('/')
})

describe('user status filter', () => {
  it('TestRF010_OffersAllStatusesWithAllSelectedByDefault', async () => {
    const api = await open()
    await screen.findByText('Ana Pérez')
    for (const name of ['Todos', 'Activos', 'Pendientes', 'Bloqueados', 'Deshabilitados']) expect(filter(name)).toBeInTheDocument()
    expect(filter('Todos')).toHaveAttribute('aria-pressed', 'true')
    expect(filter('Bloqueados')).toHaveAttribute('aria-pressed', 'false')
    expect(paramsOf(lastUrl(api)).has('status')).toBe(false)
  })

  it('TestRF010_ChoosingAStatusSendsItToTheApiAndTheUrl', async () => {
    const api = await open()
    await screen.findByText('Ana Pérez')
    fireEvent.click(filter('Bloqueados'))
    await waitFor(() => expect(paramsOf(lastUrl(api)).get('status')).toBe('locked'))
    expect(await screen.findByText('Carla Soto')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('Ana Pérez')).not.toBeInTheDocument())
    expect(filter('Bloqueados')).toHaveAttribute('aria-pressed', 'true')
    expect(filter('Todos')).toHaveAttribute('aria-pressed', 'false')
    expect(window.location.search).toBe('?estado=locked')
  })

  it('TestRF010_ChoosingAllSendsNoStatusAndClearsTheUrl', async () => {
    const api = await open('/usuarios?estado=locked')
    await screen.findByText('Carla Soto')
    fireEvent.click(filter('Todos'))
    expect(await screen.findByText('Ana Pérez')).toBeInTheDocument()
    expect(paramsOf(lastUrl(api)).has('status')).toBe(false)
    expect(window.location.search).toBe('')
  })

  it('TestRF010_ChangingTheStatusResetsTheCursor', async () => {
    const api = await open('/usuarios', {
      'GET /api/v1/admin/users': (_body, url) => {
        const params = paramsOf(url)
        if (params.get('cursor')) return json(200, { items: [directory[3]], nextCursor: null })
        return json(200, { items: params.get('status') ? [directory[2]] : [directory[1]], nextCursor: params.get('status') ? null : 'ana-id' })
      },
    })
    await screen.findByText('Ana Pérez')
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    await waitFor(() => expect(lastUrl(api)).toContain('cursor=ana-id'))
    fireEvent.click(filter('Bloqueados'))
    await waitFor(() => expect(paramsOf(lastUrl(api)).get('status')).toBe('locked'))
    expect(paramsOf(lastUrl(api)).has('cursor')).toBe(false)
  })

  it('TestRF010_KeepsTheStatusWhenLoadingMore', async () => {
    const api = await open('/usuarios?estado=active', {
      'GET /api/v1/admin/users': (_body, url) => json(200, { items: [directory[1]], nextCursor: paramsOf(url).get('cursor') ? null : 'ana-id' }),
    })
    await screen.findByText('Ana Pérez')
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    await waitFor(() => expect(lastUrl(api)).toContain('cursor=ana-id'))
    expect(paramsOf(lastUrl(api)).get('status')).toBe('active')
  })

  it('TestRF010_CombinesTheStatusFilterWithTheSearch', async () => {
    const api = await open()
    await screen.findByText('Ana Pérez')
    fireEvent.click(filter('Bloqueados'))
    await screen.findByText('Carla Soto')
    type('Buscar por correo o nombre', 'car')
    await waitFor(() => expect(paramsOf(lastUrl(api)).get('q')).toBe('car'))
    expect(paramsOf(lastUrl(api)).get('status')).toBe('locked')
    await waitFor(() => expect(screen.queryByText('Carlos Lara')).not.toBeInTheDocument())
    expect(screen.getByText('Carla Soto')).toBeInTheDocument()
  })

  it('TestRF010_PreselectsTheStatusFromTheUrlOnLoad', async () => {
    const api = await open('/usuarios?estado=locked')
    await screen.findByText('Carla Soto')
    expect(filter('Bloqueados')).toHaveAttribute('aria-pressed', 'true')
    expect(paramsOf(api.calls.find((call) => call.key === 'GET /api/v1/admin/users')?.url ?? '').get('status')).toBe('locked')
  })

  it('TestRF010_IgnoresAnUnknownStatusInTheUrl', async () => {
    const api = await open('/usuarios?estado=bogus')
    await screen.findByText('Ana Pérez')
    expect(filter('Todos')).toHaveAttribute('aria-pressed', 'true')
    expect(paramsOf(lastUrl(api)).has('status')).toBe(false)
  })

  it('TestRF010_ShowsTheEmptyStateWhenNoUserHasTheStatus', async () => {
    await open('/usuarios?estado=disabled')
    expect(await screen.findByText('No hay usuarios con este estado.')).toBeInTheDocument()
    expect(screen.queryByRole('table', { name: 'Directorio' })).not.toBeInTheDocument()
  })

  it('TestRF010_HidesTheLoadedListCountsWhileAStatusFilterIsActive', async () => {
    await open()
    await screen.findByText('Ana Pérez')
    expect(screen.getByRole('group', { name: 'Usuarios activos' })).toBeInTheDocument()
    fireEvent.click(filter('Bloqueados'))
    await waitFor(() => expect(screen.queryByRole('group', { name: 'Usuarios activos' })).not.toBeInTheDocument())
    expect(screen.queryByRole('group', { name: 'Cuentas bloqueadas' })).not.toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Invitaciones pendientes' })).not.toBeInTheDocument()
    fireEvent.click(filter('Todos'))
    expect(await screen.findByRole('group', { name: 'Usuarios activos' })).toBeInTheDocument()
  })
})
