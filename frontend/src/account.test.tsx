import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import { type Handler, goTo, json, problem, profile, signedIn, signedOut, stubApi, type } from './test-utils'

const chromeMac = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36'
const firefoxLinux = 'Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0'
const sessions = [
  { id: 's-current', ip: '10.0.0.1', userAgent: chromeMac, createdAt: '2026-10-01T10:00:00Z', lastUsedAt: '2026-10-01T11:00:00Z', current: true },
  { id: 's-other', ip: '10.0.0.2', userAgent: firefoxLinux, createdAt: '2026-09-30T10:00:00Z', lastUsedAt: '2026-09-30T11:00:00Z', current: false },
]

const rowAt = (rows: HTMLElement[], index: number): HTMLElement => {
  const row = rows[index]
  if (!row) throw new Error(`missing session row ${index}`)
  return row
}

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

  it('TestRF002_RejectsAPasswordLongerThan128BeforeCallingTheApi', async () => {
    goTo('/invitations/accept?token=tok-123')
    const api = stubApi(signedOut)
    render(<App />)
    await screen.findByRole('heading', { name: 'Define tu contraseña' })
    fill('a'.repeat(129))
    expect(await screen.findByText('La contraseña debe tener entre 12 y 128 caracteres.')).toBeInTheDocument()
    expect(api.calls.some((c) => c.key.includes('invitations/accept'))).toBe(false)
    fill('a'.repeat(128))
    await waitFor(() => expect(api.calls.some((c) => c.key.includes('invitations/accept'))).toBe(true))
  })

  it('TestRF002_ShowsAFallbackMessageForA400WithoutFieldErrors', async () => {
    goTo('/invitations/accept?token=tok-123')
    stubApi({ ...signedOut, 'POST /api/v1/auth/invitations/accept': () => problem(400) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Define tu contraseña' })
    fill('correct horse battery')
    expect(await screen.findByText('La contraseña no cumple los requisitos.')).toBeInTheDocument()
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

  it('TestRF015_ResetConfirmRejectsAPasswordLongerThan128', async () => {
    goTo('/password-reset?token=reset-tok')
    const api = stubApi(signedOut)
    render(<App />)
    await screen.findByRole('heading', { name: 'Elige una contraseña nueva' })
    confirm('a'.repeat(129))
    expect(await screen.findByText('La contraseña debe tener entre 12 y 128 caracteres.')).toBeInTheDocument()
    expect(api.calls.some((c) => c.key.endsWith('password-reset/confirm'))).toBe(false)
  })

  it('TestRF015_ResetConfirmShowsAFallbackMessageForA400WithoutFieldErrors', async () => {
    goTo('/password-reset?token=reset-tok')
    stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/confirm': () => problem(400) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Elige una contraseña nueva' })
    confirm('another long password')
    expect(await screen.findByText('La contraseña no cumple los requisitos.')).toBeInTheDocument()
  })

  it('TestRF015_ResetConfirmShowsAGenericMessageForA500AndKeepsTheForm', async () => {
    goTo('/password-reset?token=reset-tok')
    stubApi({ ...signedOut, 'POST /api/v1/auth/password-reset/confirm': () => problem(500) })
    render(<App />)
    await screen.findByRole('heading', { name: 'Elige una contraseña nueva' })
    confirm('another long password')
    expect(await screen.findByText('No fue posible completar la solicitud. Inténtalo de nuevo.')).toBeInTheDocument()
    expect(screen.getByLabelText('Contraseña nueva')).toBeInTheDocument()
    expect(screen.queryByText(/Tu contraseña se actualizó/)).not.toBeInTheDocument()
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

  it('TestRF008_RejectsAnEmptyOrWhitespaceDisplayNameWithoutCallingTheApi', async () => {
    const api = await open()
    await screen.findByDisplayValue('Persona')
    for (const value of ['', '   ']) {
      type('Nombre para mostrar', value)
      fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))
      expect(await screen.findByText('El nombre para mostrar no puede estar vacío.')).toBeInTheDocument()
    }
    expect(api.calls.some((c) => c.key === 'PATCH /api/v1/me')).toBe(false)
  })

  it('TestRF008_RendersTwoIdenticalProblemMessagesWithoutDuplicateKeys', async () => {
    const errors = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    try {
      await open(profile(), {
        'PATCH /api/v1/me': () => problem(400, { errors: [{ field: 'displayName', message: 'Valor no válido.' }, { field: 'displayName', message: 'Valor no válido.' }] }),
      })
      await screen.findByDisplayValue('Persona')
      type('Nombre para mostrar', 'x')
      fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))
      expect(await screen.findAllByText('Valor no válido.')).toHaveLength(2)
      expect(errors.mock.calls.some((call) => String(call[0]).includes('same key'))).toBe(false)
    } finally {
      errors.mockRestore()
    }
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
    expect(within(rowAt(rows, 1)).getByText('Firefox en Linux')).toBeInTheDocument()
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

  it('TestRF016_ShowsAShortBrowserAndOsLabelWithTheFullUserAgentInTheTitle', async () => {
    await open()
    const rows = await screen.findAllByRole('listitem', { name: /Sesión/ })
    expect(within(rowAt(rows, 0)).getByText('Chrome en macOS')).toHaveAttribute('title', chromeMac)
    expect(within(rowAt(rows, 1)).getByText('Firefox en Linux')).toHaveAttribute('title', firefoxLinux)
  })

  it.each([
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0', 'Edge en Windows'],
    ['Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1', 'Safari en iOS'],
    ['Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36', 'Chrome en Android'],
    ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15', 'Safari en macOS'],
    ['curl/8.7.1', 'curl'],
    ['', 'Dispositivo desconocido'],
    ['SomethingElse/1.0', 'Dispositivo desconocido'],
  ])('TestRF016_LabelsTheUserAgentSimply %#', async (userAgent, label) => {
    await open(profile(), { 'GET /api/v1/me/sessions': () => json(200, [{ ...sessions[0], userAgent }]) })
    const row = (await screen.findAllByRole('listitem', { name: /Sesión/ }))[0] as HTMLElement
    expect(within(row).getByText(label)).toBeInTheDocument()
  })

  it('TestRF016_EndsTheSessionWhenAuthenticationKeepsFailingWhileRevoking', async () => {
    const api = await open(profile(), { 'DELETE /api/v1/me/sessions/s-other': () => problem(401) })
    const rows = await screen.findAllByRole('listitem', { name: /Sesión/ })
    fireEvent.click(within(rowAt(rows, 1)).getByRole('button', { name: 'Revocar' }))
    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
    expect(api.calls.filter((c) => c.key === 'DELETE /api/v1/me/sessions/s-other')).toHaveLength(2)
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
