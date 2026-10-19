import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import * as navigation from './navigation'
import { COMPANY_STORAGE_KEY, initialCompany } from './features/admin/company'
import {
  type Handler, applicationsFixture, contabilidadOrganization, empresaBOrganization, goTo, hubOrganization, json, organizationsFixture,
  profile, signedIn, signedOut, stubApi, type,
} from './test-utils'

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const person = (overrides: Record<string, unknown>) => profile({ mfaEnabled: true, lastLoginAt: '2026-10-01T09:30:00Z', ...overrides })
const ana = person({ id: 'ana-id', email: 'ana@example.test', displayName: 'Ana Pérez', roles: ['user', 'contabilidad.analista'] })
const bruno = person({ id: 'bruno-id', email: 'bruno@example.test', displayName: 'Bruno Díaz', roles: ['user', 'inventario.consulta'] })

const inventarioId = '55555555-5555-4555-8555-555555555551'
const inventario = {
  id: inventarioId,
  clientId: 'inventario',
  name: 'Inventario',
  permissions: [{ key: 'productos.ver', description: 'Ver productos' }, { key: 'productos.editar', description: 'Editar productos' }],
  roles: [
    { id: '66666666-6666-4666-8666-666666666661', name: 'inventario.consulta', description: 'Solo lectura', applicationId: inventarioId, permissionKeys: ['productos.ver'], system: false, assignedCount: 1 },
    { id: '66666666-6666-4666-8666-666666666662', name: 'inventario.bodeguero', description: 'Edita productos', applicationId: inventarioId, permissionKeys: ['productos.ver', 'productos.editar'], system: false, assignedCount: 0 },
  ],
}

const organizationOf = (url: string) => new URL(url, 'http://localhost').searchParams.get('organizationId')
// The API filters by company (RF-022): each handler answers only for the company the request names.
const usersByCompany: Handler = (_body, url) => {
  const byCompany: Record<string, unknown[]> = { [contabilidadOrganization.id]: [ana], [empresaBOrganization.id]: [bruno], [hubOrganization.id]: [adminUser] }
  return json(200, { items: byCompany[organizationOf(url) ?? ''] ?? [], nextCursor: null })
}
const applicationsByCompany: Handler = (_body, url) => {
  const byCompany: Record<string, unknown[]> = { [contabilidadOrganization.id]: applicationsFixture(), [empresaBOrganization.id]: [inventario], [hubOrganization.id]: [] }
  return json(200, byCompany[organizationOf(url) ?? ''] ?? [])
}
const adminApi = (extra: Record<string, Handler> = {}) => ({
  ...signedIn(adminUser),
  'GET /api/v1/admin/users': usersByCompany,
  'GET /api/v1/admin/applications': applicationsByCompany,
  ...extra,
})
const open = async (path: string, extra: Record<string, Handler> = {}) => {
  goTo(path)
  const api = stubApi(adminApi(extra))
  render(<App />)
  return api
}
const chooseCompany = (name: string) => fireEvent.change(screen.getByLabelText('Empresa'), { target: { value: organizationsFixture().find((item) => item.name === name)?.id } })
const callsTo = (api: ReturnType<typeof stubApi>, key: string) => api.calls.filter((call) => call.key === key)

beforeEach(() => window.localStorage.clear())
afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  window.localStorage.clear()
  goTo('/')
})

describe('company selection', () => {
  it('TestRF022_ChoosesTheFirstNonOperatorCompanyByDefault', () => {
    const organizations = organizationsFixture()
    expect(initialCompany([hubOrganization, ...organizations.slice(0, 2)], null)?.slug).toBe('contabilidad-sa')
    expect(initialCompany(organizations, empresaBOrganization.id)?.slug).toBe('empresa-b')
    expect(initialCompany(organizations, 'gone')?.slug).toBe('contabilidad-sa')
    expect(initialCompany([hubOrganization], null)?.slug).toBe('hub')
    expect(initialCompany([], null)).toBeNull()
  })

  it('TestRF022_SelectorListsEveryCompanyAndLabelsTheOperator', async () => {
    await open('/usuarios')
    await screen.findByText('Ana Pérez')
    const select = screen.getByLabelText('Empresa')
    expect(within(select).getAllByRole('option').map((option) => option.textContent)).toEqual(['Contabilidad S.A.', 'Empresa B', 'Identity Hub (operador)'])
    expect(select).toHaveValue(contabilidadOrganization.id)
  })

  it('TestRF022_UsersPageListsOnlyTheSelectedCompanyAndSwitchingReplacesThem', async () => {
    const api = await open('/usuarios')
    await screen.findByText('Ana Pérez')
    expect(organizationOf(callsTo(api, 'GET /api/v1/admin/users')[0]?.url ?? '')).toBe(contabilidadOrganization.id)
    expect(screen.queryByText('Bruno Díaz')).not.toBeInTheDocument()

    chooseCompany('Empresa B')

    await screen.findByText('Bruno Díaz')
    expect(screen.queryByText('Ana Pérez')).not.toBeInTheDocument()
    expect(organizationOf(callsTo(api, 'GET /api/v1/admin/users').at(-1)?.url ?? '')).toBe(empresaBOrganization.id)
  })

  it('TestRF022_RolesPageShowsOnlyTheApplicationsOfTheSelectedCompany', async () => {
    const api = await open('/roles')
    await screen.findByRole('table', { name: 'Permisos de roles de Contabilidad' })
    expect(screen.queryByRole('table', { name: 'Permisos de roles de Inventario' })).not.toBeInTheDocument()
    expect(organizationOf(callsTo(api, 'GET /api/v1/admin/applications')[0]?.url ?? '')).toBe(contabilidadOrganization.id)

    chooseCompany('Empresa B')

    await screen.findByRole('table', { name: 'Permisos de roles de Inventario' })
    expect(screen.queryByRole('table', { name: 'Permisos de roles de Contabilidad' })).not.toBeInTheDocument()
    expect(screen.queryByText('contabilidad.senior')).not.toBeInTheDocument()
    // Creating a role only offers the applications of the company on screen.
    expect(screen.getAllByRole('button', { name: /^Nuevo rol de / }).map((button) => button.textContent)).toEqual(['Nuevo rol de Inventario'])
  })

  it('TestRF022_HomeCountsTheUsersOfTheSelectedCompany', async () => {
    const api = await open('/inicio', { 'GET /api/v1/admin/audit-log': () => json(200, { items: [], nextCursor: null }) })
    await screen.findByText(/Los conteos de usuarios son de Contabilidad S\.A\./)
    await waitFor(() => expect(callsTo(api, 'GET /api/v1/admin/users').length).toBe(4))
    expect(callsTo(api, 'GET /api/v1/admin/users').every((call) => organizationOf(call.url) === contabilidadOrganization.id)).toBe(true)
  })

  it('TestRF022_RemembersTheCompanyPerViewerInLocalStorage', async () => {
    await open('/usuarios')
    await screen.findByText('Ana Pérez')
    chooseCompany('Empresa B')
    await screen.findByText('Bruno Díaz')
    expect(window.localStorage.getItem(COMPANY_STORAGE_KEY)).toBe(empresaBOrganization.id)
  })

  it('TestRF022_StartsOnTheStoredCompany', async () => {
    window.localStorage.setItem(COMPANY_STORAGE_KEY, empresaBOrganization.id)
    await open('/usuarios')
    await screen.findByText('Bruno Díaz')
    expect(screen.getByLabelText('Empresa')).toHaveValue(empresaBOrganization.id)
  })

  it('TestRF022_WorksWhenStorageIsBlocked', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked') })
    await open('/usuarios')
    await screen.findByText('Ana Pérez')
    chooseCompany('Empresa B')
    expect(await screen.findByText('Bruno Díaz')).toBeInTheDocument()
  })

  it('TestRF022_ShowsAnErrorAndRetriesWhenCompaniesDoNotLoad', async () => {
    let healthy = false
    await open('/usuarios', { 'GET /api/v1/admin/organizations': () => (healthy ? json(200, organizationsFixture()) : json(500, { title: 'Error', status: 500 })) })
    expect(await screen.findByText('No fue posible cargar las empresas. Inténtalo de nuevo.')).toBeInTheDocument()
    healthy = true
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(await screen.findByText('Ana Pérez')).toBeInTheDocument()
  })
})

describe('role assignment drawer', () => {
  it('TestRF022_EditDrawerOffersOnlyTheRolesOfTheUsersCompany', async () => {
    window.localStorage.setItem(COMPANY_STORAGE_KEY, empresaBOrganization.id)
    const api = await open('/usuarios')
    await screen.findByText('Bruno Díaz')
    fireEvent.click(screen.getByRole('button', { name: 'Editar a Bruno Díaz' }))
    const dialog = await screen.findByRole('dialog')
    await within(dialog).findByLabelText('inventario.bodeguero')

    const offered = within(dialog).getAllByRole('checkbox').map((box) => (box as HTMLInputElement).value)
    expect(offered).toEqual(['admin', 'user', 'inventario.consulta', 'inventario.bodeguero'])
    expect(offered.some((role) => role.startsWith('contabilidad.'))).toBe(false)
    expect(organizationOf(callsTo(api, 'GET /api/v1/admin/applications').at(-1)?.url ?? '')).toBe(empresaBOrganization.id)
  })

  it('TestRF022_InviteFormAsksForTheCompanyPrefilledAndSendsIt', async () => {
    const created = person({ id: 'eva-id', email: 'eva@example.test', displayName: 'Eva Mora', status: 'pending_verification', roles: ['user', 'contabilidad.analista'], lastLoginAt: null })
    const api = await open('/usuarios', { 'POST /api/v1/admin/users': () => json(201, created) })
    await screen.findByText('Ana Pérez')
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo usuario' }))
    const dialog = await screen.findByRole('dialog', { name: 'Nuevo usuario' })
    await within(dialog).findByLabelText('contabilidad.analista')
    expect(within(dialog).getByLabelText('Empresa')).toHaveValue(contabilidadOrganization.id)

    fireEvent.click(within(dialog).getByLabelText('contabilidad.analista'))
    type('Nombre para mostrar', 'Eva Mora')
    type('Correo electrónico', 'eva@example.test')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Crear y enviar invitación' }))

    await screen.findByText('Invitación enviada a eva@example.test.')
    expect(callsTo(api, 'POST /api/v1/admin/users')[0]?.body).toEqual({
      email: 'eva@example.test', displayName: 'Eva Mora', organizationId: contabilidadOrganization.id, roles: ['user', 'contabilidad.analista'],
    })
  })

  it('TestRF022_ChangingTheInviteCompanySwapsTheOfferedRolesAndDropsTheChosenOnes', async () => {
    const created = person({ id: 'eva-id', email: 'eva@example.test', displayName: 'Eva Mora', status: 'pending_verification', roles: ['user'], lastLoginAt: null })
    const api = await open('/usuarios', { 'POST /api/v1/admin/users': () => json(201, created) })
    await screen.findByText('Ana Pérez')
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo usuario' }))
    const dialog = await screen.findByRole('dialog', { name: 'Nuevo usuario' })
    await within(dialog).findByLabelText('contabilidad.analista')
    fireEvent.click(within(dialog).getByLabelText('contabilidad.analista'))

    fireEvent.change(within(dialog).getByLabelText('Empresa'), { target: { value: empresaBOrganization.id } })

    await within(dialog).findByLabelText('inventario.bodeguero')
    expect(within(dialog).queryByLabelText('contabilidad.analista')).not.toBeInTheDocument()
    type('Nombre para mostrar', 'Eva Mora')
    type('Correo electrónico', 'eva@example.test')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Crear y enviar invitación' }))
    await screen.findByText('Invitación enviada a eva@example.test.')
    expect(callsTo(api, 'POST /api/v1/admin/users')[0]?.body).toEqual({
      email: 'eva@example.test', displayName: 'Eva Mora', organizationId: empresaBOrganization.id, roles: ['user'],
    })
  })
})

describe('application launcher', () => {
  const memberApi = (applications: unknown[], extra: Record<string, Handler> = {}) => ({
    ...signedIn(profile({ roles: ['user', 'contabilidad.analista'] })),
    'GET /api/v1/me/applications': () => json(200, applications),
    'GET /api/v1/me/sessions': () => json(200, []),
    ...extra,
  })

  it('TestRF022_RailListsTheApplicationsOfTheUser', async () => {
    goTo('/me')
    stubApi(memberApi([{ clientId: 'contabilidad', name: 'Contabilidad', launchUrl: 'http://contabilidad.localhost:8080' }]))
    render(<App />)
    const link = await screen.findByRole('link', { name: 'Contabilidad' })
    expect(link).toHaveAttribute('href', 'http://contabilidad.localhost:8080')
    expect(screen.getByText('Aplicaciones conectadas')).toBeInTheDocument()
  })

  it('TestRF022_RailShowsOnlyTheAppsReturnedByTheApi', async () => {
    goTo('/me')
    stubApi(memberApi([{ clientId: 'inventario', name: 'Inventario', launchUrl: 'http://inventario.localhost:8080' }]))
    render(<App />)
    expect(await screen.findByRole('link', { name: 'Inventario' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Contabilidad' })).not.toBeInTheDocument()
  })

  it('TestRF022_RailHidesTheBlockWhenTheUserHasNoApplications', async () => {
    goTo('/me')
    const api = stubApi(memberApi([]))
    render(<App />)
    await screen.findByRole('heading', { name: 'Mi cuenta' })
    await waitFor(() => expect(callsTo(api, 'GET /api/v1/me/applications').length).toBe(1))
    expect(screen.queryByText('Aplicaciones conectadas')).not.toBeInTheDocument()
  })

  it('TestRF022_RailHidesTheBlockWhenTheApplicationsDoNotLoad', async () => {
    goTo('/me')
    stubApi(memberApi([], { 'GET /api/v1/me/applications': () => json(500, { title: 'Error', status: 500 }) }))
    render(<App />)
    await screen.findByRole('heading', { name: 'Mi cuenta' })
    expect(screen.queryByText('Aplicaciones conectadas')).not.toBeInTheDocument()
  })
})

describe('post-login routing', () => {
  let redirect: ReturnType<typeof vi.spyOn>
  beforeEach(() => { redirect = vi.spyOn(navigation, 'redirectTo').mockImplementation(() => undefined) })

  const loginAs = async (user: ReturnType<typeof profile>, applications: unknown[], extra: Record<string, Handler> = {}) => {
    goTo('/login')
    let verified = false
    const grant = { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }
    const api = stubApi({
      ...signedOut,
      'POST /api/v1/auth/refresh': () => (verified ? json(200, grant) : signedOut['POST /api/v1/auth/refresh']()),
      'POST /api/v1/auth/login': () => json(202, { mfaToken: 'challenge', expiresIn: 300 }),
      'POST /api/v1/auth/mfa/verify': () => { verified = true; return json(200, grant) },
      'GET /api/v1/me': () => json(200, user),
      'GET /api/v1/me/applications': () => json(200, applications),
      'GET /api/v1/me/sessions': () => json(200, []),
      'GET /api/v1/admin/organizations': () => json(200, organizationsFixture()),
      'GET /api/v1/admin/users': usersByCompany,
      'GET /api/v1/admin/audit-log': () => json(200, { items: [], nextCursor: null }),
      ...extra,
    })
    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    type('Correo electrónico', user.email)
    type('Contraseña', 'password')
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    type('Código de verificación', '123456')
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))
    return api
  }
  const contabilidad = { clientId: 'contabilidad', name: 'Contabilidad', launchUrl: 'http://contabilidad.localhost:8080' }
  const inventarioApp = { clientId: 'inventario', name: 'Inventario', launchUrl: 'http://inventario.localhost:8080' }

  it('TestRF022_ASingleApplicationSendsTheUserStraightToIt', async () => {
    await loginAs(profile({ roles: ['user', 'contabilidad.analista'] }), [contabilidad])
    await waitFor(() => expect(redirect).toHaveBeenCalledWith('http://contabilidad.localhost:8080'))
  })

  it('TestRF022_SeveralApplicationsLeaveTheUserOnMyAccountWithTheList', async () => {
    await loginAs(profile({ roles: ['user', 'contabilidad.analista', 'inventario.consulta'] }), [contabilidad, inventarioApp])
    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
    expect(await screen.findByRole('link', { name: 'Inventario' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Contabilidad' })).toBeInTheDocument()
    expect(redirect).not.toHaveBeenCalled()
  })

  it('TestRF022_NoApplicationsLeaveTheUserOnMyAccount', async () => {
    await loginAs(profile({ roles: ['user'] }), [])
    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
    expect(redirect).not.toHaveBeenCalled()
  })

  it('TestRF022_IfTheApplicationsFailTheUserStillLandsOnMyAccount', async () => {
    await loginAs(profile({ roles: ['user', 'contabilidad.analista'] }), [], { 'GET /api/v1/me/applications': () => json(500, { title: 'Error', status: 500 }) })
    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
    expect(redirect).not.toHaveBeenCalled()
  })

  it('TestRF022_AdministratorsKeepGoingToTheConsoleEvenWithOneApplication', async () => {
    const api = await loginAs(adminUser, [contabilidad])
    expect(await screen.findByRole('heading', { name: 'Inicio' })).toBeInTheDocument()
    expect(redirect).not.toHaveBeenCalled()
    expect(callsTo(api, 'GET /api/v1/admin/organizations').length).toBeGreaterThan(0)
  })
})
