import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { MemoryRouter } from 'react-router-dom'
import { UsersPage } from './features/admin/UsersPage'
import { AuditLogPage } from './features/admin/AuditLogPage'
import { resetSessionForTests } from './api/client'
import { type Handler, applicationsRoute, goTo, json, problem, profile, signedIn, stubApi, type } from './test-utils'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

const adminUser = profile({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] })
const person = (overrides: Record<string, unknown>) => profile({ mfaEnabled: true, lastLoginAt: '2026-10-01T09:30:00Z', ...overrides })
const directory = [
  person({ id: 'admin-id', email: 'admin@example.test', displayName: 'Admina Root', roles: ['user', 'admin'] }),
  person({ id: 'ana-id', email: 'ana@example.test', displayName: 'Ana Pérez', roles: ['user', 'contabilidad.senior'] }),
  person({ id: 'beto-id', email: 'beto@example.test', displayName: 'Beto Ruiz', status: 'pending_verification', roles: ['user', 'contabilidad.analista'], lastLoginAt: null }),
  person({ id: 'carla-id', email: 'carla@example.test', displayName: 'Carla Soto', status: 'locked', roles: ['user'] }),
  person({ id: 'dani-id', email: 'dani@example.test', displayName: 'Dani Vega', status: 'disabled', roles: ['user'] }),
]
const listUsers: Handler = (_body, url) => {
  const q = new URL(url, 'http://localhost').searchParams.get('q')?.toLowerCase()
  const items = q ? directory.filter((item) => `${item.email} ${item.displayName}`.toLowerCase().includes(q)) : directory
  return json(200, { items, nextCursor: null })
}

const adminApi = (extra: Record<string, Handler> = {}) => ({
  ...signedIn(adminUser),
  ...applicationsRoute,
  'GET /api/v1/admin/users': listUsers,
  ...extra,
})
const openUsers = async (extra: Record<string, Handler> = {}) => {
  goTo('/usuarios')
  const api = stubApi(adminApi(extra))
  render(<App />)
  await screen.findByText('Ana Pérez')
  return api
}
const rowOf = (name: string) => {
  const row = within(screen.getByRole('table', { name: 'Directorio' })).getByText(name).closest('tr')
  if (!row) throw new Error(`missing row ${name}`)
  return row as HTMLElement
}
const chips = (row: HTMLElement) => within(within(row).getByRole('list', { name: 'Roles' })).getAllByRole('listitem').map((item) => item.textContent)
const drawer = () => screen.getByRole('dialog')
const body = (api: ReturnType<typeof stubApi>, key: string) => api.calls.find((call) => call.key === key)?.body

afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  goTo('/')
})

describe('admin page unmount safety', () => {
  it('TestRNF012_UsersPageIgnoresLateUnauthorizedResponseAfterUnmount', async () => {
    const late = deferred<Response>()
    const onSessionEnded = vi.fn()
    const api = stubApi({
      'GET /api/v1/admin/users': () => late.promise,
      'POST /api/v1/auth/refresh': () => problem(401),
    })
    const view = render(<MemoryRouter><UsersPage currentUserId="admin-id" onSessionEnded={onSessionEnded} /></MemoryRouter>)
    await waitFor(() => expect(api.calls).toHaveLength(1))
    view.unmount()
    await act(async () => { late.resolve(problem(401)) })
    expect(onSessionEnded).not.toHaveBeenCalled()
  })

  it('TestRNF012_AuditLogPageIgnoresLateUnauthorizedResponseAfterUnmount', async () => {
    const late = deferred<Response>()
    const onSessionEnded = vi.fn()
    const api = stubApi({
      'GET /api/v1/admin/audit-log': () => late.promise,
      'POST /api/v1/auth/refresh': () => problem(401),
    })
    const view = render(<MemoryRouter><AuditLogPage onSessionEnded={onSessionEnded} /></MemoryRouter>)
    await waitFor(() => expect(api.calls).toHaveLength(1))
    view.unmount()
    await act(async () => { late.resolve(problem(401)) })
    expect(onSessionEnded).not.toHaveBeenCalled()
  })

  it('TestRNF012_UsersPageIgnoresLateResendFailureAfterUnmount', async () => {
    const late = deferred<Response>()
    const onSessionEnded = vi.fn()
    const api = stubApi({
      'GET /api/v1/admin/users': () => json(200, { items: directory, nextCursor: null }),
      'POST /api/v1/admin/users/beto-id/invitation': () => late.promise,
      'POST /api/v1/auth/refresh': () => problem(401),
    })
    const view = render(<MemoryRouter><UsersPage currentUserId="admin-id" onSessionEnded={onSessionEnded} /></MemoryRouter>)
    const resend = await screen.findByRole('button', { name: 'Reenviar invitación a Beto Ruiz' })
    fireEvent.click(resend)
    await waitFor(() => expect(api.calls.some((call) => call.key === 'POST /api/v1/admin/users/beto-id/invitation')).toBe(true))
    view.unmount()
    await act(async () => { late.resolve(problem(401)) })
    expect(onSessionEnded).not.toHaveBeenCalled()
  })
})

describe('user directory', () => {
  it('TestRF010_ListsUsersWithStatusRolesAndLastAccess', async () => {
    const api = await openUsers()
    expect(screen.getByRole('heading', { name: 'Usuarios' })).toBeInTheDocument()
    expect(api.calls.find((c) => c.key === 'GET /api/v1/admin/users')?.url).toContain('limit=100')
    const ana = rowOf('Ana Pérez')
    expect(within(ana).getByText('ana@example.test')).toBeInTheDocument()
    expect(within(ana).getByText('AP')).toBeInTheDocument()
    expect(within(ana).getByText('Activo')).toBeInTheDocument()
    expect(chips(ana)).toEqual(['user', 'contabilidad.senior'])
    expect(within(ana).getByText(/2026/)).toBeInTheDocument()
    expect(within(rowOf('Beto Ruiz')).getByText('Pendiente')).toBeInTheDocument()
    expect(within(rowOf('Beto Ruiz')).getByText('Sin accesos')).toBeInTheDocument()
    expect(within(rowOf('Carla Soto')).getByText('Bloqueado')).toBeInTheDocument()
    expect(within(rowOf('Dani Vega')).getByText('Deshabilitado')).toBeInTheDocument()
  })

  it('TestRF010_ShowsStatTilesComputedFromTheList', async () => {
    await openUsers()
    const tile = (name: string) => within(screen.getByRole('group', { name }))
    expect(tile('Usuarios activos').getByText('2')).toBeInTheDocument()
    expect(tile('Cuentas bloqueadas').getByText('1')).toBeInTheDocument()
    expect(tile('Invitaciones pendientes').getByText('1')).toBeInTheDocument()
  })

  it('TestRF010_SearchesByEmailOrNameThroughTheApiQuery', async () => {
    const api = await openUsers()
    type('Buscar por correo o nombre', 'carla')
    await waitFor(() => expect(screen.queryByText('Ana Pérez')).not.toBeInTheDocument())
    expect(screen.getByText('Carla Soto')).toBeInTheDocument()
    expect(api.calls.at(-1)?.url).toContain('q=carla')
    expect(within(screen.getByRole('group', { name: 'Cuentas bloqueadas' })).getByText('1')).toBeInTheDocument()
  })

  it('TestRF010_ShowsAnEmptyMessageWhenTheSearchMatchesNobody', async () => {
    await openUsers()
    type('Buscar por correo o nombre', 'zzz')
    expect(await screen.findByText('No hay usuarios que coincidan con la búsqueda.')).toBeInTheDocument()
  })

  it('TestRF010_LoadsMoreUsersWithTheCursor', async () => {
    const first = [directory[0], directory[1]]
    const second = [directory[2]]
    const api = await openUsers({
      'GET /api/v1/admin/users': (_body, url) => (url.includes('cursor=ana-id') ? json(200, { items: second, nextCursor: null }) : json(200, { items: first, nextCursor: 'ana-id' })),
    })
    expect(screen.queryByText('Beto Ruiz')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    expect(await screen.findByText('Beto Ruiz')).toBeInTheDocument()
    expect(screen.getByText('Ana Pérez')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Cargar más' })).not.toBeInTheDocument()
    expect(api.calls.at(-1)?.url).toContain('cursor=ana-id')
  })

  it('TestRF010_DiscardsALateLoadMoreFromAnOlderSearchAndKeepsTheNewCursor', async () => {
    const late = deferred<Response>()
    const api = await openUsers({
      'GET /api/v1/admin/users': (_body, url) => {
        const params = new URL(url, 'http://localhost').searchParams
        if (params.get('cursor') === 'ana-id') return late.promise
        if (params.get('q') === 'carla') return json(200, { items: [directory[3]], nextCursor: 'carla-cursor' })
        return json(200, { items: [directory[0], directory[1]], nextCursor: 'ana-id' })
      },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    type('Buscar por correo o nombre', 'carla')
    await waitFor(() => expect(screen.queryByText('Ana Pérez')).not.toBeInTheDocument())
    expect(await screen.findByText('Carla Soto')).toBeInTheDocument()
    await act(async () => { late.resolve(json(200, { items: [directory[2]], nextCursor: 'stale-cursor' })) })
    expect(screen.queryByText('Beto Ruiz')).not.toBeInTheDocument()
    expect(screen.getByText('Carla Soto')).toBeInTheDocument()
    const more = screen.getByRole('button', { name: 'Cargar más' })
    expect(more).toBeEnabled()
    fireEvent.click(more)
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('cursor=carla-cursor'))
    expect(api.calls.at(-1)?.url).toContain('q=carla')
    expect(api.calls.some((c) => c.url.includes('stale-cursor'))).toBe(false)
  })

  it('TestRF010_HidesLoadMoreUntilTheFirstPageOfTheNewSearchArrives', async () => {
    const search = deferred<Response>()
    const api = await openUsers({
      'GET /api/v1/admin/users': (_body, url) => (url.includes('q=carla') ? search.promise : json(200, { items: [directory[0], directory[1]], nextCursor: 'ana-id' })),
    })
    expect(screen.getByRole('button', { name: 'Cargar más' })).toBeInTheDocument()
    type('Buscar por correo o nombre', 'carla')
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('q=carla'))
    expect(screen.queryByRole('button', { name: 'Cargar más' })).not.toBeInTheDocument()
    await act(async () => { search.resolve(json(200, { items: [directory[3]], nextCursor: null })) })
    expect(await screen.findByText('Carla Soto')).toBeInTheDocument()
  })

  it('TestRF010_ShowsAnErrorWhenTheListCannotBeLoaded', async () => {
    goTo('/usuarios')
    stubApi(adminApi({ 'GET /api/v1/admin/users': () => problem(500) }))
    render(<App />)
    expect(await screen.findByText('No fue posible cargar el directorio. Inténtalo de nuevo.')).toBeInTheDocument()
  })

  it('TestRF010_EndsTheSessionWhenAuthenticationKeepsFailingOnTheList', async () => {
    goTo('/usuarios')
    stubApi(adminApi({ 'GET /api/v1/admin/users': () => problem(401) }))
    render(<App />)
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
  })
})

describe('admin route protection', () => {
  it.each(['/usuarios', '/auditoria'])('TestRF009_MembersCannotReachAdminRoute %s', async (path) => {
    goTo(path)
    const api = stubApi({ ...signedIn(profile()), 'GET /api/v1/me/sessions': () => json(200, []) })
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Usuarios' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Auditoría' })).not.toBeInTheDocument()
    expect(api.calls.some((c) => c.key.includes('/admin/'))).toBe(false)
  })

  it('TestRF009_AdminsSeeUsersAndAuditInTheRail', async () => {
    await openUsers()
    expect(screen.getByRole('link', { name: 'Usuarios' })).toHaveAttribute('href', '/usuarios')
    expect(screen.getByRole('link', { name: 'Auditoría' })).toHaveAttribute('href', '/auditoria')
  })
})

describe('create user drawer', () => {
  const openCreate = async (extra: Record<string, Handler> = {}) => {
    const api = await openUsers(extra)
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo usuario' }))
    await screen.findByRole('dialog', { name: 'Nuevo usuario' })
    await within(drawer()).findByLabelText('contabilidad.auditor')
    return api
  }
  const created = person({ id: 'eva-id', email: 'eva@example.test', displayName: 'Eva Mora', status: 'pending_verification', roles: ['user', 'contabilidad.senior'], lastLoginAt: null })

  it('TestRF001_CreatesAUserWithTheCatalogRolesAndShowsAToast', async () => {
    const api = await openCreate({ 'POST /api/v1/admin/users': () => json(201, created) })
    const roles = within(drawer()).getByRole('group', { name: 'Roles' })
    expect(within(roles).getAllByRole('checkbox').map((box) => (box as HTMLInputElement).value)).toEqual(['admin', 'user', 'contabilidad.senior', 'contabilidad.analista', 'contabilidad.auditor'])
    expect(within(roles).getByLabelText('user')).toBeChecked()
    expect(within(roles).getByLabelText('user')).toBeDisabled()
    expect(within(roles).getByText('Control total del Hub y de todas las aplicaciones conectadas.')).toBeInTheDocument()
    type('Nombre para mostrar', 'Eva Mora')
    type('Correo electrónico', 'eva@example.test')
    fireEvent.click(within(roles).getByLabelText('contabilidad.senior'))
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Crear y enviar invitación' }))
    expect(await screen.findByText('Invitación enviada a eva@example.test.')).toBeInTheDocument()
    expect(body(api, 'POST /api/v1/admin/users')).toEqual({ email: 'eva@example.test', displayName: 'Eva Mora', roles: ['user', 'contabilidad.senior'] })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(within(rowOf('Eva Mora')).getByText('Pendiente')).toBeInTheDocument()
  })

  it('TestRF021_CajonListaRolesDeAplicacionDeLaApiAgrupadosConSusPermisos', async () => {
    await openCreate()
    const group = within(drawer()).getByRole('group', { name: 'Contabilidad' })
    expect(within(group).getAllByRole('checkbox').map((item) => (item as HTMLInputElement).value)).toEqual(['contabilidad.senior', 'contabilidad.analista', 'contabilidad.auditor'])
    expect(within(group).getByText('Permisos: movimientos.ver_todos, reportes.ver')).toBeInTheDocument()
    expect(within(group).getByText('Permisos: movimientos.registrar')).toBeInTheDocument()
  })

  it('TestRF021_CajonMuestraElErrorSiNoCargaElCatalogoDeRoles', async () => {
    goTo('/usuarios')
    stubApi({ ...adminApi(), 'GET /api/v1/admin/applications': () => problem(500) })
    render(<App />)
    await screen.findByText('Ana Pérez')
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo usuario' }))
    expect(await within(await screen.findByRole('dialog')).findByText('No fue posible cargar los roles de las aplicaciones. Los roles del Hub siguen disponibles.')).toBeInTheDocument()
  })

  it('TestRF001_ShowsFieldErrorsFromTheApiAndKeepsTheDrawerOpen', async () => {
    await openCreate({ 'POST /api/v1/admin/users': () => problem(400, { errors: [{ field: 'email', message: 'must be a valid email address' }] }) })
    type('Nombre para mostrar', 'Eva Mora')
    type('Correo electrónico', 'eva@example.test')
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Crear y enviar invitación' }))
    expect(await within(drawer()).findByText('Correo electrónico: must be a valid email address')).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('TestRF001_ExplainsADuplicateEmailFor409', async () => {
    await openCreate({ 'POST /api/v1/admin/users': () => problem(409) })
    type('Nombre para mostrar', 'Eva Mora')
    type('Correo electrónico', 'ana@example.test')
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Crear y enviar invitación' }))
    expect(await within(drawer()).findByText('No se pudo crear la cuenta: ese correo ya está registrado o no está disponible.')).toBeInTheDocument()
  })

  it('TestRF001_RejectsAnEmptyNameBeforeCallingTheApi', async () => {
    const api = await openCreate()
    type('Nombre para mostrar', '   ')
    type('Correo electrónico', 'eva@example.test')
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Crear y enviar invitación' }))
    expect(await within(drawer()).findByText('El nombre para mostrar no puede estar vacío.')).toBeInTheDocument()
    expect(api.calls.some((c) => c.key === 'POST /api/v1/admin/users')).toBe(false)
  })

  it('TestRF001_ShowsAGenericMessageWhenCreationFailsWithoutDetails', async () => {
    await openCreate({ 'POST /api/v1/admin/users': () => problem(503) })
    type('Nombre para mostrar', 'Eva Mora')
    type('Correo electrónico', 'eva@example.test')
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Crear y enviar invitación' }))
    expect(await within(drawer()).findByText('No fue posible crear la cuenta. Inténtalo de nuevo.')).toBeInTheDocument()
  })

  it('TestRF001_EndsTheSessionWhenAuthenticationKeepsFailingOnCreate', async () => {
    await openCreate({ 'POST /api/v1/admin/users': () => problem(401) })
    type('Nombre para mostrar', 'Eva Mora')
    type('Correo electrónico', 'eva@example.test')
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Crear y enviar invitación' }))
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
  })
})

describe('drawer focus and keyboard', () => {
  it('TestRF010_FocusesTheFirstFieldClosesWithEscapeAndReturnsFocus', async () => {
    await openUsers()
    const trigger = screen.getByRole('button', { name: 'Nuevo usuario' })
    trigger.focus()
    fireEvent.click(trigger)
    const dialog = await screen.findByRole('dialog', { name: 'Nuevo usuario' })
    await waitFor(() => expect(screen.getByLabelText('Nombre para mostrar')).toHaveFocus())
    fireEvent.keyDown(dialog, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(trigger).toHaveFocus()
  })

  it('TestRF010_KeepsTabInsideTheDrawer', async () => {
    await openUsers()
    fireEvent.click(screen.getByRole('button', { name: 'Nuevo usuario' }))
    const dialog = await screen.findByRole('dialog', { name: 'Nuevo usuario' })
    const buttons = within(dialog).getAllByRole('button')
    const last = buttons[buttons.length - 1] as HTMLElement
    last.focus()
    fireEvent.keyDown(dialog, { key: 'Tab' })
    expect(dialog.contains(document.activeElement)).toBe(true)
    expect(document.activeElement).not.toBe(last)
  })
})

describe('edit user drawer', () => {
  const openEdit = async (name: string, extra: Record<string, Handler> = {}) => {
    const api = await openUsers(extra)
    fireEvent.click(within(rowOf(name)).getByRole('button', { name: `Editar a ${name}` }))
    await screen.findByRole('dialog', { name: 'Editar usuario' })
    await within(drawer()).findByLabelText('contabilidad.auditor')
    return api
  }

  it('TestRF010_ShowsTheAccountWithReadOnlyEmailAndStatusAndRolesPrefilled', async () => {
    await openEdit('Ana Pérez')
    expect(screen.getByLabelText('Correo electrónico')).toHaveAttribute('readonly')
    expect(screen.getByLabelText('Correo electrónico')).toHaveValue('ana@example.test')
    expect(screen.getByLabelText('Nombre para mostrar')).toHaveAttribute('readonly')
    expect(screen.getByLabelText('Estado')).toHaveValue('active')
    expect(screen.getByLabelText('contabilidad.senior')).toBeChecked()
    expect(screen.getByLabelText('contabilidad.analista')).not.toBeChecked()
  })

  it('TestRF010_EditsRolesAndSendsOnlyTheChangedField', async () => {
    const api = await openEdit('Ana Pérez', {
      'PATCH /api/v1/admin/users/ana-id': (payload) => json(200, { ...directory[1], roles: (payload as { roles: string[] }).roles }),
    })
    fireEvent.click(screen.getByLabelText('contabilidad.senior'))
    fireEvent.click(screen.getByLabelText('contabilidad.analista'))
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText('Cambios guardados.')).toBeInTheDocument()
    expect(body(api, 'PATCH /api/v1/admin/users/ana-id')).toEqual({ roles: ['user', 'contabilidad.analista'] })
    expect(chips(rowOf('Ana Pérez'))).toEqual(['user', 'contabilidad.analista'])
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('TestRF010_DisablesAnAccountThroughTheStatusControl', async () => {
    const api = await openEdit('Ana Pérez', {
      'PATCH /api/v1/admin/users/ana-id': (payload) => json(200, { ...directory[1], status: (payload as { status: string }).status }),
    })
    fireEvent.change(screen.getByLabelText('Estado'), { target: { value: 'disabled' } })
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText('Cambios guardados.')).toBeInTheDocument()
    expect(body(api, 'PATCH /api/v1/admin/users/ana-id')).toEqual({ status: 'disabled' })
    expect(within(rowOf('Ana Pérez')).getByText('Deshabilitado')).toBeInTheDocument()
    expect(within(screen.getByRole('group', { name: 'Usuarios activos' })).getByText('1')).toBeInTheDocument()
  })

  it('TestRF010_BlocksTheAdminFromDisablingThemselfAndNeverCallsTheApi', async () => {
    const api = await openEdit('Admina Root')
    expect(screen.getByLabelText('Estado')).toBeDisabled()
    expect(within(drawer()).getByText('Un admin no puede deshabilitarse a sí mismo (RF-010).')).toBeInTheDocument()
    expect(screen.getByLabelText('admin')).toBeDisabled()
    expect(within(drawer()).getByText('Un admin no puede asignarse roles a sí mismo; otro admin debe hacerlo (RF-010).')).toBeInTheDocument()
    expect(within(drawer()).getByRole('button', { name: 'Guardar cambios' })).toBeDisabled()
    expect(api.calls.some((c) => c.key.startsWith('PATCH'))).toBe(false)
  })

  it('TestRF010_ExplainsABadRequestWithoutFieldErrors', async () => {
    await openEdit('Ana Pérez', { 'PATCH /api/v1/admin/users/ana-id': () => problem(400, { detail: 'The requested user update is not allowed.' }) })
    fireEvent.change(screen.getByLabelText('Estado'), { target: { value: 'locked' } })
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Guardar cambios' }))
    expect(await within(drawer()).findByText('El cambio no está permitido: un admin no puede deshabilitarse ni asignarse roles a sí mismo, y siempre debe quedar un admin activo.')).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('TestRF010_ShowsFieldErrorsFromTheApiOnEdit', async () => {
    await openEdit('Ana Pérez', { 'PATCH /api/v1/admin/users/ana-id': () => problem(400, { errors: [{ field: 'roles', message: 'unknown role "x"' }] }) })
    fireEvent.click(screen.getByLabelText('contabilidad.analista'))
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Guardar cambios' }))
    expect(await within(drawer()).findByText('Roles: unknown role "x"')).toBeInTheDocument()
  })

  it('TestRF010_ClosesWithoutCallingTheApiWhenNothingChanged', async () => {
    const api = await openEdit('Ana Pérez')
    fireEvent.click(within(drawer()).getByRole('button', { name: 'Guardar cambios' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(api.calls.some((c) => c.key.startsWith('PATCH'))).toBe(false)
  })
})

describe('resend invitation', () => {
  const resend = (name: string) => fireEvent.click(within(rowOf(name)).getByRole('button', { name: `Reenviar invitación a ${name}` }))

  it('TestRF001_OffersResendOnlyForPendingAccounts', async () => {
    await openUsers()
    expect(within(rowOf('Beto Ruiz')).getByRole('button', { name: /Reenviar invitación/ })).toBeInTheDocument()
    for (const name of ['Ana Pérez', 'Carla Soto', 'Dani Vega']) {
      expect(within(rowOf(name)).queryByRole('button', { name: /Reenviar invitación/ })).not.toBeInTheDocument()
    }
  })

  it('TestRF001_ResendsTheInvitationAndShowsAToast', async () => {
    const api = await openUsers({ 'POST /api/v1/admin/users/beto-id/invitation': () => json(204) })
    resend('Beto Ruiz')
    expect(await screen.findByText('Invitación reenviada a beto@example.test.')).toBeInTheDocument()
    expect(api.calls.some((c) => c.key === 'POST /api/v1/admin/users/beto-id/invitation')).toBe(true)
  })

  it('TestRF001_ResendingTwiceShowsTheSameToastAndRestartsItsTimer', async () => {
    await openUsers({ 'POST /api/v1/admin/users/beto-id/invitation': () => json(204) })
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    try {
      const message = 'Invitación reenviada a beto@example.test.'
      // Testing Library's waitFor/findBy drain with setTimeout, which would hang under fake timers.
      const flush = () => act(async () => { await vi.advanceTimersByTimeAsync(0) })
      resend('Beto Ruiz')
      await flush()
      expect(screen.getByRole('status')).toHaveTextContent(message)
      act(() => { vi.advanceTimersByTime(3000) })
      expect(screen.getByRole('button', { name: 'Reenviar invitación a Beto Ruiz' })).toBeEnabled()
      resend('Beto Ruiz')
      await flush()
      act(() => { vi.advanceTimersByTime(3000) })
      expect(screen.getByRole('status')).toHaveTextContent(message)
      act(() => { vi.advanceTimersByTime(2000) })
      expect(screen.queryByRole('status')).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('TestRF001_KeepsResendCompletionAfterReloadingTheDirectory', async () => {
    const invitation = deferred<Response>()
    const api = await openUsers({ 'POST /api/v1/admin/users/beto-id/invitation': () => invitation.promise })
    resend('Beto Ruiz')
    await waitFor(() => expect(api.calls.some((call) => call.key === 'POST /api/v1/admin/users/beto-id/invitation')).toBe(true))
    type('Buscar por correo o nombre', 'beto')
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('q=beto'))
    await act(async () => { invitation.resolve(json(204)) })
    expect(await screen.findByText('Invitación reenviada a beto@example.test.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reenviar invitación a Beto Ruiz' })).toBeEnabled()
  })

  it.each([
    [409, 'La cuenta ya no está pendiente: no hace falta reenviar la invitación.'],
    [404, 'La cuenta ya no existe.'],
    [403, 'No tienes permiso para reenviar invitaciones.'],
    [503, 'No fue posible reenviar la invitación. Inténtalo de nuevo.'],
  ])('TestRF001_ExplainsResendFailureFor%s', async (status, message) => {
    await openUsers({ 'POST /api/v1/admin/users/beto-id/invitation': () => problem(status) })
    resend('Beto Ruiz')
    expect(await screen.findByText(message)).toBeInTheDocument()
  })

  it('TestRF001_EndsTheSessionWhenAuthenticationKeepsFailingOnResend', async () => {
    await openUsers({ 'POST /api/v1/admin/users/beto-id/invitation': () => problem(401) })
    resend('Beto Ruiz')
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
  })
})

describe('audit log', () => {
  const events = [
    { id: 12, actorUserId: 'aaaaaaaa-1111-2222-3333-444444444444', action: 'employee_created', resourceType: 'user', resourceId: 'bbbbbbbb-1111-2222-3333-444444444444', ip: '10.0.0.5', userAgent: 'curl/8', metadata: { roles: ['user', 'contabilidad.senior'] }, createdAt: '2026-10-01T10:00:00Z' },
    { id: 11, actorUserId: null, action: 'login_failed', resourceType: null, resourceId: null, ip: null, userAgent: null, metadata: { note: '<img src=x onerror=alert(1)>' }, createdAt: '2026-10-01T09:00:00Z' },
  ]
  const openAudit = async (extra: Record<string, Handler> = {}) => {
    goTo('/auditoria')
    const api = stubApi({ ...adminApi(), 'GET /api/v1/admin/audit-log': () => json(200, { items: events, nextCursor: null }), ...extra })
    render(<App />)
    await screen.findByRole('heading', { name: 'Auditoría' })
    return api
  }
  const table = () => screen.getByRole('table', { name: 'Registro de auditoría' })

  it('TestRF011_ListsEventsWithTimeActorActionAndResource', async () => {
    const api = await openAudit()
    await within(await screen.findByRole('table', { name: 'Registro de auditoría' })).findByText('employee_created')
    expect(api.calls.find((c) => c.key === 'GET /api/v1/admin/audit-log')?.url).toContain('limit=50')
    const rows = within(table()).getAllByRole('row').slice(1)
    expect(within(rows[0] as HTMLElement).getByText('aaaaaaaa')).toHaveAttribute('title', 'aaaaaaaa-1111-2222-3333-444444444444')
    expect(within(rows[0] as HTMLElement).getByText('user · bbbbbbbb')).toHaveAttribute('title', 'bbbbbbbb-1111-2222-3333-444444444444')
    expect(within(rows[0] as HTMLElement).getByText('10.0.0.5')).toBeInTheDocument()
    expect(within(rows[0] as HTMLElement).getByText(/2026/)).toBeInTheDocument()
    expect(within(rows[1] as HTMLElement).getByText('Sistema')).toBeInTheDocument()
    expect(within(rows[1] as HTMLElement).getByText('login_failed')).toBeInTheDocument()
  })

  it('TestRF011_RendersMetadataAsInertText', async () => {
    await openAudit()
    await screen.findByText('login_failed')
    expect(screen.getByText(/onerror=alert\(1\)/)).toBeInTheDocument()
    expect(document.querySelector('img')).toBeNull()
    expect(screen.getByText('user, contabilidad.senior')).toBeInTheDocument()
  })

  it('TestRF011_FiltersByActionActorAndSince', async () => {
    const api = await openAudit()
    await screen.findByText('login_failed')
    type('Acción', 'login_failed')
    type('Actor (ID)', 'aaaaaaaa-1111-2222-3333-444444444444')
    type('Desde', '2026-10-01T08:00')
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('action=login_failed'))
    const url = new URL(api.calls.at(-1)?.url ?? '', 'http://localhost')
    expect(url.searchParams.get('actorId')).toBe('aaaaaaaa-1111-2222-3333-444444444444')
    expect(new Date(url.searchParams.get('since') ?? '').getTime()).toBe(new Date('2026-10-01T08:00').getTime())
  })

  it('TestRF011_RejectsAnActorThatIsNotAUuid', async () => {
    const api = await openAudit()
    await screen.findByText('login_failed')
    const before = api.calls.length
    type('Actor (ID)', 'no-es-uuid')
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    expect(await screen.findByText('El ID del actor debe ser un UUID.')).toBeInTheDocument()
    expect(api.calls).toHaveLength(before)
  })

  it('TestRF011_ClearsTheFilters', async () => {
    const api = await openAudit()
    await screen.findByText('login_failed')
    type('Acción', 'login_failed')
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('action=login_failed'))
    fireEvent.click(screen.getByRole('button', { name: 'Limpiar' }))
    await waitFor(() => expect(api.calls.at(-1)?.url).not.toContain('action='))
    expect(screen.getByLabelText('Acción')).toHaveValue('')
  })

  it('TestRF011_LoadsOlderEventsWithTheCursor', async () => {
    const older = { ...events[1], id: 10, action: 'login_succeeded', metadata: {} }
    const api = await openAudit({
      'GET /api/v1/admin/audit-log': (_body, url) => (url.includes('cursor=11') ? json(200, { items: [older], nextCursor: null }) : json(200, { items: events, nextCursor: '11' })),
    })
    await screen.findByText('employee_created')
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    expect(await screen.findByText('login_succeeded')).toBeInTheDocument()
    expect(screen.getByText('employee_created')).toBeInTheDocument()
    expect(api.calls.at(-1)?.url).toContain('cursor=11')
  })

  it('TestRF011_DiscardsALateLoadMoreAfterTheFilterChangesAndNeverSendsTheOldCursor', async () => {
    const late = deferred<Response>()
    const older = { ...events[1], id: 10, action: 'stale_event', metadata: {} }
    const filtered = { ...events[1], id: 9, action: 'login_failed', metadata: {} }
    const api = await openAudit({
      'GET /api/v1/admin/audit-log': (_body, url) => {
        const params = new URL(url, 'http://localhost').searchParams
        if (params.get('cursor') === '11') return late.promise
        if (params.get('action') === 'login_failed') return json(200, { items: [filtered], nextCursor: 'filtered-cursor' })
        return json(200, { items: events, nextCursor: '11' })
      },
    })
    await screen.findByText('employee_created')
    fireEvent.click(screen.getByRole('button', { name: 'Cargar más' }))
    type('Acción', 'login_failed')
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(within(table()).queryByText('employee_created')).not.toBeInTheDocument())
    await act(async () => { late.resolve(json(200, { items: [older], nextCursor: 'stale-cursor' })) })
    expect(screen.queryByText('stale_event')).not.toBeInTheDocument()
    const more = await screen.findByRole('button', { name: 'Cargar más' })
    expect(more).toBeEnabled()
    fireEvent.click(more)
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('cursor=filtered-cursor'))
    expect(api.calls.at(-1)?.url).toContain('action=login_failed')
    expect(api.calls.some((c) => c.url.includes('stale-cursor'))).toBe(false)
  })

  it('TestRF011_HidesLoadMoreUntilTheFirstPageOfTheNewFilterArrives', async () => {
    const filtered = deferred<Response>()
    const api = await openAudit({
      'GET /api/v1/admin/audit-log': (_body, url) => (url.includes('action=login_failed') ? filtered.promise : json(200, { items: events, nextCursor: '11' })),
    })
    await screen.findByText('employee_created')
    type('Acción', 'login_failed')
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('action=login_failed'))
    expect(screen.queryByRole('button', { name: 'Cargar más' })).not.toBeInTheDocument()
    await act(async () => { filtered.resolve(json(200, { items: [], nextCursor: null })) })
  })

  it('TestRF011_DiscardsALateLoadMoreAfterClearingTheFilters', async () => {
    const late = deferred<Response>()
    const older = { ...events[1], id: 10, action: 'stale_event', metadata: {} }
    const api = await openAudit({
      'GET /api/v1/admin/audit-log': (_body, url) => {
        const params = new URL(url, 'http://localhost').searchParams
        if (params.get('cursor') === '11-filtered') return late.promise
        if (params.get('action') === 'login_failed') return json(200, { items: [events[1]], nextCursor: '11-filtered' })
        return json(200, { items: events, nextCursor: '11' })
      },
    })
    await screen.findByText('employee_created')
    type('Acción', 'login_failed')
    fireEvent.click(screen.getByRole('button', { name: 'Filtrar' }))
    await waitFor(() => expect(within(table()).queryByText('employee_created')).not.toBeInTheDocument())
    fireEvent.click(await screen.findByRole('button', { name: 'Cargar más' }))
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('cursor=11-filtered'))
    fireEvent.click(screen.getByRole('button', { name: 'Limpiar' }))
    expect(await screen.findByText('employee_created')).toBeInTheDocument()
    await act(async () => { late.resolve(json(200, { items: [older], nextCursor: 'stale-cursor' })) })
    expect(screen.queryByText('stale_event')).not.toBeInTheDocument()
    fireEvent.click(await screen.findByRole('button', { name: 'Cargar más' }))
    await waitFor(() => expect(api.calls.at(-1)?.url).toContain('cursor=11'))
    expect(api.calls.at(-1)?.url).not.toContain('action=')
  })

  it('TestRF011_ShowsAnEmptyMessage', async () => {
    await openAudit({ 'GET /api/v1/admin/audit-log': () => json(200, { items: [], nextCursor: null }) })
    expect(await screen.findByText('No hay eventos para los filtros elegidos.')).toBeInTheDocument()
  })

  it('TestRF011_ShowsAnErrorWhenTheLogCannotBeLoaded', async () => {
    await openAudit({ 'GET /api/v1/admin/audit-log': () => problem(500) })
    expect(await screen.findByText('No fue posible cargar el registro de auditoría. Inténtalo de nuevo.')).toBeInTheDocument()
  })

  it('TestRF011_EndsTheSessionWhenAuthenticationKeepsFailing', async () => {
    goTo('/auditoria')
    stubApi({ ...adminApi(), 'GET /api/v1/admin/audit-log': () => problem(401) })
    render(<App />)
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
  })
})
