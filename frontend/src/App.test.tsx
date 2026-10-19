import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { authenticatedRequest, resetSessionForTests } from './api/client'
import { organizationsRoute, profile, signedIn, stubApi } from './test-utils'

const json = (status: number, body?: unknown) => new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const user = (roles: string[]) => ({ id: 'user-id', email: 'person@example.test', displayName: 'Persona', status: 'active', roles, mfaEnabled: true, createdAt: '2026-01-01T00:00:00Z' })

const accessGrant = { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }
// A signed-out visitor who logs in as an administrator: the console also loads the launcher and the company list.
const adminLoginRoutes = () => {
  let verified = false
  return {
    'POST /api/v1/auth/refresh': () => (verified ? json(200, accessGrant) : json(401, { title: 'Unauthorized', status: 401 })),
    'POST /api/v1/auth/login': () => json(202, { mfaToken: 'challenge', expiresIn: 300 }),
    'POST /api/v1/auth/mfa/verify': () => { verified = true; return json(200, accessGrant) },
    'GET /api/v1/me': () => json(200, user(['admin'])),
    'GET /api/v1/me/applications': () => json(200, []),
    ...organizationsRoute,
  }
}

describe('authentication routes', () => {
  afterEach(() => {
    resetSessionForTests()
    vi.unstubAllGlobals()
    window.history.replaceState({}, '', '/')
  })

  it('TestRF013_RoutesAdminAfterMfaVerification', async () => {
    stubApi({ ...adminLoginRoutes(), 'GET /api/v1/admin/users': () => json(200, { items: [], nextCursor: null }), 'GET /api/v1/admin/audit-log': () => json(200, { items: [], nextCursor: null }) })

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'admin@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.change(screen.getByLabelText('Código de verificación'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))

    expect(await screen.findByRole('heading', { name: 'Inicio' })).toBeInTheDocument()
  })

  it('TestRF013_RoutesMemberAfterMfaVerification', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }))
      .mockResolvedValueOnce(json(202, { mfaToken: 'challenge', expiresIn: 300 }))
      .mockResolvedValueOnce(json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(200, user(['user']))))

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.change(screen.getByLabelText('Código de verificación'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))

    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
  })

  it.each([
    [401, 'Correo o contraseña incorrectos.'],
    [423, 'La cuenta está bloqueada. Inténtalo más tarde.'],
    [429, 'Demasiados intentos. Espera antes de volver a intentarlo.'],
    [503, 'No fue posible enviar el código. Inténtalo de nuevo.'],
  ])('TestRF003_ShowsNeutralMessageForStatus%s', async (status, message) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 })).mockResolvedValueOnce(json(status, { title: 'Failure', status })))
    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    expect(await screen.findByText(message)).toBeInTheDocument()
  })

  it('TestRF003_ShowsNeutralNetworkMessage', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 })).mockRejectedValueOnce(new TypeError('Failed to fetch')))
    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    expect(await screen.findByText('No se pudo conectar con el servicio. Comprueba tu conexión.')).toBeInTheDocument()
  })

  it('TestRF009_RedirectsProtectedRouteWithoutSession', async () => {
    window.history.replaceState({}, '', '/me')
    vi.stubGlobal('fetch', vi.fn(async () => json(401, { title: 'Unauthorized', status: 401 })))
    render(<App />)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument())
  })

  it('TestRF013_ShowsMfaCodeMessageForInvalidCode', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }))
      .mockResolvedValueOnce(json(202, { mfaToken: 'challenge', expiresIn: 300 }))
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 })))

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.change(screen.getByLabelText('Código de verificación'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))

    expect(await screen.findByText('El código no es válido o el desafío expiró.')).toBeInTheDocument()
    expect(screen.queryByText('Correo o contraseña incorrectos.')).not.toBeInTheDocument()
  })

  it('TestRF013_StartsOverAfterInvalidMfaCode', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }))
      .mockResolvedValueOnce(json(202, { mfaToken: 'challenge', expiresIn: 300 }))
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 })))

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.change(screen.getByLabelText('Código de verificación'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))
    await screen.findByText('El código no es válido o el desafío expiró.')
    fireEvent.click(screen.getByRole('button', { name: 'Volver a iniciar sesión' }))

    expect(screen.getByLabelText('Correo electrónico')).toBeInTheDocument()
    expect(screen.queryByLabelText('Código de verificación')).not.toBeInTheDocument()
  })

  it('TestRF013_OffersStartOverWhenResendChallengeExpires', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }))
      .mockResolvedValueOnce(json(202, { mfaToken: 'challenge', expiresIn: 300 }))
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 })))

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.click(screen.getByRole('button', { name: 'Reenviar código' }))

    expect(await screen.findByText('El desafío expiró. Vuelve a iniciar sesión.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Volver a iniciar sesión' })).toBeInTheDocument()
  })

  it.each([
    [429, 'Se solicitaron demasiados códigos. Espera antes de reenviar otro.'],
    [503, 'No se pudo enviar el código. El código anterior sigue siendo válido.'],
  ])('TestRF013_ShowsResendMessageForStatus%s', async (status, message) => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }))
      .mockResolvedValueOnce(json(202, { mfaToken: 'challenge', expiresIn: 300 }))
      .mockResolvedValueOnce(json(status, { title: 'Failure', status })))

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.click(screen.getByRole('button', { name: 'Reenviar código' }))

    expect(await screen.findByText(message)).toBeInTheDocument()
  })

  it('TestRF013_ShowsConfirmationWhenMfaCodeIsResent', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }))
      .mockResolvedValueOnce(json(202, { mfaToken: 'challenge', expiresIn: 300 }))
      .mockResolvedValueOnce(new Response(null, { status: 202 })))

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.click(screen.getByRole('button', { name: 'Reenviar código' }))

    expect(await screen.findByText('Te enviamos un código nuevo.')).toBeInTheDocument()
  })

  it('TestRF013_ReturnsToCredentialsWhenProfileLoadFailsAfterMfa', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }))
      .mockResolvedValueOnce(json(202, { mfaToken: 'challenge', expiresIn: 300 }))
      .mockResolvedValueOnce(json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(503, { title: 'Unavailable', status: 503 }))
      .mockResolvedValueOnce(json(200, user(['user'])))
    vi.stubGlobal('fetch', fetch)

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'person@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.change(screen.getByLabelText('Código de verificación'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))

    expect(await screen.findByText('No se pudo cargar tu sesión. Vuelve a iniciar sesión.')).toBeInTheDocument()
    expect(screen.getByLabelText('Correo electrónico')).toBeInTheDocument()
    expect(screen.queryByLabelText('Código de verificación')).not.toBeInTheDocument()
    await expect(authenticatedRequest('/api/v1/me')).resolves.toEqual(user(['user']))
    const finalRequest = fetch.mock.calls.at(-1)?.[1] as RequestInit | undefined
    expect(new Headers(finalRequest?.headers).get('Authorization')).toBeNull()
  })

  it('TestRF005_RestoresSessionOnMount', async () => {
    stubApi({ ...signedIn(profile({ roles: ['admin'] })), 'GET /api/v1/admin/users': () => json(200, { items: [], nextCursor: null }), 'GET /api/v1/admin/audit-log': () => json(200, { items: [], nextCursor: null }) })

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Inicio' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Iniciar sesión' })).not.toBeInTheDocument()
  })

  it('TestRF009_RedirectsNonAdminUsersFromUsersRoute', async () => {
    window.history.replaceState({}, '', '/usuarios')
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(200, user(['user']))))

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
  })

  it('TestRF009_MarksTheCurrentRailLinkActive', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(200, user(['admin']))))

    render(<App />)

    // The link renders before the redirect to /inicio settles; wait for the route, not the node.
    await waitFor(() => expect(screen.getByRole('link', { name: 'Inicio' })).toHaveClass('active'))
  })

  it('TestRF015_ExponeElDestinoDelEnlaceParaSaltarAlContenidoEnRutasPublicas', async () => {
    window.history.replaceState({}, '', '/forgot-password')

    render(<App />)

    await screen.findByRole('heading', { name: 'Restablecer contraseña' })
    expect(screen.getByRole('link', { name: 'Saltar al contenido' })).toHaveAttribute('href', '#main-content')
    expect(document.querySelector('main#main-content')).toBeInTheDocument()
  })

  it('TestRF015_ElDestinoDelEnlaceParaSaltarPuedeRecibirElFocoEnRutasPublicasYLogin', async () => {
    window.history.replaceState({}, '', '/forgot-password')
    const { unmount } = render(<App />)
    await screen.findByRole('heading', { name: 'Restablecer contraseña' })
    const publicMain = document.querySelector<HTMLElement>('main#main-content')
    expect(publicMain).toHaveAttribute('tabindex', '-1')
    fireEvent.click(screen.getByRole('link', { name: 'Saltar al contenido' }))
    publicMain?.focus()
    expect(publicMain).toHaveFocus()
    unmount()

    window.history.replaceState({}, '', '/login')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json(401, { title: 'Unauthorized', status: 401 })))
    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    const loginMain = document.querySelector<HTMLElement>('main#main-content')
    expect(loginMain).toHaveAttribute('tabindex', '-1')
    loginMain?.focus()
    expect(loginMain).toHaveFocus()
  })

  it('TestRF015_ElpantallaDeRestauracionExponeUnDestinoEnfocableParaElEnlaceDeSalto', () => {
    window.history.replaceState({}, '', '/login')
    vi.stubGlobal('fetch', vi.fn(() => new Promise(() => undefined)))
    render(<App />)
    const restoringMain = document.querySelector<HTMLElement>('main#main-content')
    expect(screen.getByText('Validando tu sesión…')).toBeInTheDocument()
    expect(restoringMain).toHaveAttribute('tabindex', '-1')
  })

  it('TestRF007_LogsOutLocallyWhenRemoteLogoutFails', async () => {
    stubApi({
      ...adminLoginRoutes(),
      'GET /api/v1/admin/users': () => json(200, { items: [], nextCursor: null }),
      'GET /api/v1/admin/audit-log': () => json(200, { items: [], nextCursor: null }),
      'POST /api/v1/auth/logout': () => { throw new TypeError('Failed to fetch') },
    })

    render(<App />)
    await screen.findByLabelText('Correo electrónico')
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'admin@example.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'password' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    fireEvent.change(screen.getByLabelText('Código de verificación'), { target: { value: '123456' } })
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))
    await screen.findByRole('heading', { name: 'Inicio' })
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))

    expect(await screen.findByRole('button', { name: 'Iniciar sesión' })).toBeInTheDocument()
  })
})
