import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { resetSessionForTests } from './api/client'
import * as navigation from './navigation'
import { safeContinueTarget } from './features/account/continueTarget'
import { goTo, json, profile, signedIn, signedOut, stubApi, type } from './test-utils'

const AUTHORIZE = '/oauth/authorize?client_id=contabilidad&state=abc&response_type=code'

describe('safeContinueTarget (RF-020: sin redireccion abierta)', () => {
  it.each([
    [AUTHORIZE, AUTHORIZE],
    ['/oauth/authorize', '/oauth/authorize'],
  ])('TestRF020_AcceptsRelativeAuthorizePath %s', (value, expected) => {
    expect(safeContinueTarget(value)).toBe(expected)
  })

  it.each([
    [null], [''], ['https://evil.example/oauth/authorize'], ['//evil.example/oauth/authorize'], ['/\\evil.example'],
    ['/\\/evil.example/oauth/authorize'], ['/oauth/authorize/../../evil'], ['/oauth/authorizer'], ['/oauth/authorize/extra'],
    ['/usuarios'], ['oauth/authorize'], ['javascript:alert(1)'], ['/oauth/authorize\n/evil'], ['/oauth/authorize?x=1#//evil'],
    [' /oauth/authorize'], ['/OAUTH/authorize'],
  ])('TestRF020_RejectsUnsafeContinue %j', (value) => {
    expect(safeContinueTarget(value)).toBeNull()
  })
})

describe('login with continue', () => {
  let redirect: ReturnType<typeof vi.spyOn>

  beforeEach(() => { redirect = vi.spyOn(navigation, 'redirectTo').mockImplementation(() => undefined) })
  afterEach(() => {
    resetSessionForTests()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    goTo('/')
  })

  const completeMfa = async () => {
    await screen.findByLabelText('Correo electrónico')
    type('Correo electrónico', 'person@example.test')
    type('Contraseña', 'password')
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    await screen.findByLabelText('Código de verificación')
    type('Código de verificación', '123456')
    fireEvent.click(screen.getByRole('button', { name: 'Verificar' }))
  }

  const loginApi = {
    ...signedOut,
    'POST /api/v1/auth/login': () => json(202, { mfaToken: 'challenge', expiresIn: 300 }),
    'POST /api/v1/auth/mfa/verify': () => json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }),
    'GET /api/v1/me': () => json(200, profile({ roles: ['user'] })),
  }

  it('TestRF020_ReturnsToAuthorizeAfterMfaWhenContinueIsSafe', async () => {
    goTo(`/login?continue=${encodeURIComponent(AUTHORIZE)}`)
    stubApi(loginApi)
    render(<App />)
    await completeMfa()
    await waitFor(() => expect(redirect).toHaveBeenCalledWith(AUTHORIZE))
  })

  it('TestRF020_IgnoresUnsafeContinueAndStaysInConsole', async () => {
    goTo(`/login?continue=${encodeURIComponent('https://evil.example/oauth/authorize')}`)
    stubApi(loginApi)
    render(<App />)
    await completeMfa()
    expect(await screen.findByRole('heading', { name: 'Mi cuenta' })).toBeInTheDocument()
    expect(redirect).not.toHaveBeenCalled()
  })

  it('TestRF020_SignedInConsoleUserWithoutHubSessionCanStillAuthenticateForContinue', async () => {
    // /oauth/authorize sent them to /login because hub_session is missing: bouncing them to
    // the console home would strand the SSO flow, so the login form is shown instead.
    goTo(`/login?continue=${encodeURIComponent(AUTHORIZE)}`)
    stubApi({ ...loginApi, ...signedIn() })
    render(<App />)
    expect(await screen.findByLabelText('Correo electrónico')).toBeInTheDocument()
  })
})
