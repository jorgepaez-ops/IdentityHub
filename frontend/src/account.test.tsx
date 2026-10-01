import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'

type Handler = (body: unknown, url: string) => Response | Promise<Response>

const json = (status: number, body?: unknown, type = 'application/json') =>
  new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': type } })
const problem = (status: number, extra: Record<string, unknown> = {}) =>
  json(status, { type: 'about:blank', title: 'Problem', status, ...extra }, 'application/problem+json')
const profile = (overrides: Record<string, unknown> = {}) => ({
  id: 'user-id', email: 'person@example.test', displayName: 'Persona', status: 'active',
  roles: ['user', 'contabilidad.senior'], mfaEnabled: true, createdAt: '2026-01-01T00:00:00Z', ...overrides,
})
const sessions = [
  { id: 's-current', ip: '10.0.0.1', userAgent: 'Chrome en macOS', createdAt: '2026-10-01T10:00:00Z', lastUsedAt: '2026-10-01T11:00:00Z', current: true },
  { id: 's-other', ip: '10.0.0.2', userAgent: 'Firefox en Linux', createdAt: '2026-09-30T10:00:00Z', lastUsedAt: '2026-09-30T11:00:00Z', current: false },
]

// Routes fetch by "METHOD path"; unmatched calls fail loudly so a test cannot pass by accident.
function stubApi(routes: Record<string, Handler>) {
  const calls: { key: string; body: unknown }[] = []
  const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    const key = `${init?.method ?? 'GET'} ${url.split('?')[0]}`
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    calls.push({ key, body })
    const handler = routes[key]
    if (!handler) throw new Error(`unexpected request ${key}`)
    return handler(body, url)
  })
  vi.stubGlobal('fetch', fetch)
  return { calls, fetch }
}

const signedOut = { 'POST /api/v1/auth/refresh': () => problem(401) }
const signedIn = (user = profile()) => ({
  'POST /api/v1/auth/refresh': () => json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }),
  'GET /api/v1/me': () => json(200, user),
})
const type = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } })
const rowAt = (rows: HTMLElement[], index: number): HTMLElement => {
  const row = rows[index]
  if (!row) throw new Error(`missing session row ${index}`)
  return row
}
const goTo = (path: string) => window.history.replaceState({}, '', path)

afterEach(() => {
  resetSessionForTests()
  vi.unstubAllGlobals()
  goTo('/')
})

describe('invitation acceptance', () => {
  const fill = (password: string, confirm = password) => {
    type('Contraseña nueva', password)
    type('Confirma la contraseña', confirm)
    fireEvent.click(screen.getByRole('button', { name: 'Definir contraseña' }))
  }

  it('TestRF002_AcceptsInvitationAndLinksToLogin', async () => {
    goTo('/invitations/accept?token=tok-123')
    const api = stubApi({ ...signedOut, 'POST /api/v1/auth/invitations/accept': () => json(204) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Define tu contraseña' })
    fill('correct horse battery')
    expect(await screen.findByText('Tu cuenta quedó activada. Ya puedes iniciar sesión.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Ir a iniciar sesión' })).toHaveAttribute('href', '/login')
    expect(api.calls.find((c) => c.key === 'POST /api/v1/auth/invitations/accept')?.body).toEqual({ token: 'tok-123', password: 'correct horse battery' })
  })

  it('TestRF002_RejectsShortOrMismatchedPasswordBeforeCallingTheApi', async () => {
    goTo('/invitations/accept?token=tok-123')
    const api = stubApi(signedOut)
    render(<App />)
    await screen.findByRole('heading', { name: 'Define tu contraseña' })
    fill('short')
    expect(await screen.findByText('La contraseña debe tener entre 12 y 128 caracteres.')).toBeInTheDocument()
    fill('correct horse battery', 'correct horse batterz')
    expect(await screen.findByText('Las contraseñas no coinciden.')).toBeInTheDocument()
    expect(api.calls.some((c) => c.key.includes('invitations/accept'))).toBe(false)
  })

  it('TestRF002_ShowsFieldProblemsFromA400', async () => {
    goTo('/invitations/accept?token=tok-123')
    stubApi({ ...signedOut, 'POST /api/v1/auth/invitations/accept': () => problem(400, { errors: [{ field: 'password', message: 'La contraseña es demasiado común.' }] }) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Define tu contraseña' })
    fill('correct horse battery')
    expect(await screen.findByText('La contraseña es demasiado común.')).toBeInTheDocument()
  })

  it('TestRF002_ExplainsAnExpiredOrUsedInvitationFor410', async () => {
    goTo('/invitations/accept?token=tok-123')
    stubApi({ ...signedOut, 'POST /api/v1/auth/invitations/accept': () => problem(410) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Define tu contraseña' })
    fill('correct horse battery')
    expect(await screen.findByText(/venció o ya se usó/)).toBeInTheDocument()
    expect(screen.getByText(/administrador/)).toBeInTheDocument()
  })

  it('TestRF002_ShowsInvalidLinkWithoutToken', async () => {
    goTo('/invitations/accept')
    stubApi(signedOut)
    render(<App />)
    expect(await screen.findByText(/enlace no es válido/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Contraseña nueva')).not.toBeInTheDocument()
  })
})

describe('password reset', () => {
  const neutral = 'Si la cuenta existe, enviamos un enlace para restablecer la contraseña. Revisa tu correo.'
  const request = async () => {
    await screen.findByRole('heading', { name: 'Restablecer contraseña' })
    type('Correo electrónico', 'person@example.test')
    fireEvent.click(screen.getByRole('button', { name: 'Enviar enlace' }))
  }

  it('TestRF015_LoginLinksToPasswordResetRequest', async () => {
    goTo('/login')
    stubApi(signedOut)
    render(<App />)
    expect(await screen.findByRole('link', { name: '¿Olvidaste tu contraseña?' })).toHaveAttribute('href', '/forgot-password')
  })

  it('TestRF015_ResetRequestShowsNeutralMessageFor202', async () => {
    goTo('/forgot-password')
    const api = stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/request': () => json(202) })
    render(<App />)
    await request()
    expect(await screen.findByText(neutral)).toBeInTheDocument()
    expect(api.calls.find((c) => c.key.endsWith('password-reset/request'))?.body).toEqual({ email: 'person@example.test' })
  })

  it.each([400, 429, 503, 500])('TestRF015_ResetRequestShowsTheSameNeutralMessageForStatus%s', async (status) => {
    goTo('/forgot-password')
    stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/request': () => problem(status) })
    render(<App />)
    await request()
    expect(await screen.findByText(neutral)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('TestRF015_ResetRequestShowsConnectionErrorOnNetworkFailure', async () => {
    goTo('/forgot-password')
    stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/request': () => { throw new TypeError('Failed to fetch') } })
    render(<App />)
    await request()
    expect(await screen.findByText('No se pudo conectar con el servicio. Comprueba tu conexión.')).toBeInTheDocument()
    expect(screen.queryByText(neutral)).not.toBeInTheDocument()
  })

  const confirm = (password: string) => {
    type('Contraseña nueva', password)
    type('Confirma la contraseña', password)
    fireEvent.click(screen.getByRole('button', { name: 'Restablecer contraseña' }))
  }

  it('TestRF015_ResetConfirmSetsPasswordAndLinksToLogin', async () => {
    goTo('/password-reset?token=reset-tok')
    const api = stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/confirm': () => json(204) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Elige una contraseña nueva' })
    confirm('another long password')
    expect(await screen.findByText(/Tu contraseña se actualizó/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Ir a iniciar sesión' })).toHaveAttribute('href', '/login')
    expect(api.calls.find((c) => c.key.endsWith('password-reset/confirm'))?.body).toEqual({ token: 'reset-tok', password: 'another long password' })
  })

  it('TestRF015_ResetConfirmExplainsExpiredLinkFor410', async () => {
    goTo('/password-reset?token=reset-tok')
    stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/confirm': () => problem(410) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Elige una contraseña nueva' })
    confirm('another long password')
    expect(await screen.findByText(/venció o ya se usó/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Solicitar un enlace nuevo' })).toHaveAttribute('href', '/forgot-password')
  })

  it('TestRF015_ResetConfirmShowsFieldProblemsFrom400', async () => {
    goTo('/password-reset?token=reset-tok')
    stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/confirm': () => problem(400, { errors: [{ field: 'password', message: 'Contraseña rechazada.' }] }) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Elige una contraseña nueva' })
    confirm('another long password')
    expect(await screen.findByText('Contraseña rechazada.')).toBeInTheDocument()
  })
})

describe('my account', () => {
  const open = async (user = profile(), extra: Record<string, Handler> = {}) => {
    goTo('/me')
    const api = stubApi({ ...signedIn(user), 'GET /api/v1/me/sessions': () => json(200, sessions), ...extra })
    render(<App />)
    await screen.findByRole('heading', { name: 'Mi cuenta' })
    return api
  }

  it('TestRF008_ShowsProfileWithReadOnlyEmailAndRoleChips', async () => {
    await open()
    expect(await screen.findByDisplayValue('Persona')).toBeInTheDocument()
    expect(screen.getByLabelText('Correo electrónico')).toHaveAttribute('readonly')
    expect(screen.getByLabelText('Correo electrónico')).toHaveValue('person@example.test')
    const roles = screen.getByRole('list', { name: 'Roles' })
    expect(within(roles).getAllByRole('listitem').map((item) => item.textContent)).toEqual(['user', 'contabilidad.senior'])
  })

  it('TestRF008_EditsDisplayNameAndKeepsTheSavedValue', async () => {
    const api = await open(profile(), { 'PATCH /api/v1/me': (body) => json(200, profile({ displayName: (body as { displayName: string }).displayName })) })
    await screen.findByDisplayValue('Persona')
    type('Nombre para mostrar', 'Persona Nueva')
    fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText('Perfil actualizado.')).toBeInTheDocument()
    expect(api.calls.find((c) => c.key === 'PATCH /api/v1/me')?.body).toEqual({ displayName: 'Persona Nueva' })
    expect(screen.getByLabelText('Nombre para mostrar')).toHaveValue('Persona Nueva')
  })

  it('TestRF008_ShowsFieldProblemsWhenTheProfileUpdateIsRejected', async () => {
    await open(profile(), { 'PATCH /api/v1/me': () => problem(400, { errors: [{ field: 'displayName', message: 'El nombre es demasiado largo.' }] }) })
    await screen.findByDisplayValue('Persona')
    type('Nombre para mostrar', 'x')
    fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText('El nombre es demasiado largo.')).toBeInTheDocument()
  })

  it('TestRF008_RefreshesTheTokenWhenTheProfileUpdateGets401', async () => {
    let attempts = 0
    const api = await open(profile(), {
      'PATCH /api/v1/me': (body) => (++attempts === 1 ? problem(401) : json(200, profile({ displayName: (body as { displayName: string }).displayName }))),
    })
    await screen.findByDisplayValue('Persona')
    type('Nombre para mostrar', 'Otra')
    fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText('Perfil actualizado.')).toBeInTheDocument()
    expect(api.calls.filter((c) => c.key === 'POST /api/v1/auth/refresh')).toHaveLength(2)
  })

  it('TestRF016_ListsActiveSessionsAndMarksTheCurrentOne', async () => {
    await open()
    const rows = await screen.findAllByRole('listitem', { name: /Sesión/ })
    expect(rows).toHaveLength(2)
    expect(within(rowAt(rows, 0)).getByText('Chrome en macOS')).toBeInTheDocument()
    expect(within(rowAt(rows, 0)).getByText('10.0.0.1')).toBeInTheDocument()
    expect(within(rowAt(rows, 0)).getByText('Esta sesión')).toBeInTheDocument()
    expect(within(rowAt(rows, 1)).queryByText('Esta sesión')).not.toBeInTheDocument()
  })

  it('TestRF016_RevokesAnotherSessionAndKeepsTheRest', async () => {
    const api = await open(profile(), { 'DELETE /api/v1/me/sessions/s-other': () => json(204) })
    const rows = await screen.findAllByRole('listitem', { name: /Sesión/ })
    fireEvent.click(within(rowAt(rows, 1)).getByRole('button', { name: 'Revocar' }))
    await waitFor(() => expect(screen.getAllByRole('listitem', { name: /Sesión/ })).toHaveLength(1))
    expect(screen.getByText('Chrome en macOS')).toBeInTheDocument()
    expect(api.calls.some((c) => c.key === 'DELETE /api/v1/me/sessions/s-other')).toBe(true)
    expect(screen.getByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
  })

  it('TestRF016_RevokingTheCurrentSessionLogsOutLocally', async () => {
    const api = await open(profile(), { 'DELETE /api/v1/me/sessions/s-current': () => json(204) })
    const rows = await screen.findAllByRole('listitem', { name: /Sesión/ })
    fireEvent.click(within(rowAt(rows, 0)).getByRole('button', { name: 'Revocar' }))
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
    expect(api.calls.some((c) => c.key === 'DELETE /api/v1/me/sessions/s-current')).toBe(true)
    expect(api.calls.some((c) => c.key === 'POST /api/v1/auth/logout')).toBe(false)
  })

  it('TestRF016_ShowsAnErrorWhenSessionsCannotBeLoaded', async () => {
    await open(profile(), { 'GET /api/v1/me/sessions': () => problem(503) })
    expect(await screen.findByText('No fue posible cargar tus sesiones. Inténtalo de nuevo.')).toBeInTheDocument()
  })

  it('TestRF016_ShowsAnErrorWhenRevokingFails', async () => {
    await open(profile(), { 'DELETE /api/v1/me/sessions/s-other': () => problem(404) })
    const rows = await screen.findAllByRole('listitem', { name: /Sesión/ })
    fireEvent.click(within(rowAt(rows, 1)).getByRole('button', { name: 'Revocar' }))
    expect(await screen.findByText('No fue posible revocar la sesión. Inténtalo de nuevo.')).toBeInTheDocument()
    expect(screen.getAllByRole('listitem', { name: /Sesión/ })).toHaveLength(2)
  })
})

describe('route protection', () => {
  it.each([
    ['/invitations/accept?token=t', 'Define tu contraseña'],
    ['/forgot-password', 'Restablecer contraseña'],
    ['/password-reset?token=t', 'Elige una contraseña nueva'],
  ])('TestRF009_PublicRouteIsReachableWithoutSession %s', async (path, heading) => {
    goTo(path)
    stubApi(signedOut)
    render(<App />)
    expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Iniciar sesión' })).not.toBeInTheDocument()
  })

  it.each(['/me', '/usuarios'])('TestRF009_ConsoleRouteStaysProtected %s', async (path) => {
    goTo(path)
    stubApi(signedOut)
    render(<App />)
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
  })
})
