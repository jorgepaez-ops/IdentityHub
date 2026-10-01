import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { beginLogin } from './auth/flow'
import * as navigation from './navigation'
import { AUDIENCE, HUB_ORIGIN, ISSUER } from './config'
import { makeKey, signToken, validClaims, type TestKey } from './testing'

const HUB = HUB_ORIGIN
let key: TestKey
let redirect: ReturnType<typeof vi.spyOn>

beforeAll(async () => { key = await makeKey('hub-key') })

beforeEach(() => {
  sessionStorage.clear()
  localStorage.clear()
  redirect = vi.spyOn(navigation, 'redirectTo').mockImplementation(() => undefined)
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  window.history.replaceState({}, '', '/')
})

function stubHub(claims: Record<string, unknown>) {
  const nowSeconds = Math.floor(Date.now() / 1000)
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === `${HUB}/oauth/token`) {
      const access = await signToken(key, validClaims({ iat: nowSeconds, exp: nowSeconds + 900, ...claims }))
      return new Response(JSON.stringify({ access_token: access, token_type: 'Bearer', expires_in: 900 }), { status: 200 })
    }
    if (url === `${HUB}/.well-known/jwks.json`) return new Response(JSON.stringify({ keys: [key.jwk] }), { status: 200 })
    throw new Error(`unexpected request ${url}`)
  }))
}

/** Plays the whole round trip: start the flow, then land on the callback with the stored state. */
async function signInAs(roles: string[] | null, landing: 'shell' | 'denied' = 'shell') {
  await beginLogin()
  const state = sessionStorage.getItem('contabilidad.oauth_state')
  stubHub({ roles })
  window.history.replaceState({}, '', `/oauth/callback?code=the-code&state=${state}`)
  render(<App />)
  if (landing === 'shell') await screen.findByRole('navigation', { name: 'Secciones' })
  else await screen.findByRole('heading', { name: 'Sin acceso a Contabilidad' })
}

const rail = () => screen.getByRole('navigation', { name: 'Secciones' })
const goTo = (name: RegExp | string) => fireEvent.click(within(rail()).getByRole('button', { name }))
const rowOf = (folio: string) => screen.getByText(folio).closest('tr') as HTMLElement

describe('login screen', () => {
  it('TestRF020_LoginOffersOnlyContinueWithIdentityHub', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Contabilidad' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continuar con Identity Hub' })).toBeInTheDocument()
    // Contabilidad has no users and must never ask for a password.
    expect(document.querySelector('input')).toBeNull()
  })

  it('TestRF020_ContinueButtonStartsPkceRedirectToHub', async () => {
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: 'Continuar con Identity Hub' }))
    await waitFor(() => expect(redirect).toHaveBeenCalledTimes(1))
    const target = new URL(String(redirect.mock.calls[0]?.[0]))
    expect(target.origin + target.pathname).toBe(`${HUB}/oauth/authorize`)
    expect(target.searchParams.get('code_challenge_method')).toBe('S256')
  })
})

describe('callback screen', () => {
  it('TestRF020_ShowsValidatingWhileReturningFromHubThenSignsIn', async () => {
    await beginLogin()
    stubHub({ roles: ['contabilidad.senior'] })
    window.history.replaceState({}, '', `/oauth/callback?code=c&state=${sessionStorage.getItem('contabilidad.oauth_state')}`)
    render(<App />)
    expect(screen.getByText('Validando con Identity Hub…')).toBeInTheDocument()
    await screen.findByRole('navigation', { name: 'Secciones' })
    expect(window.location.search).toBe('')
    expect(window.location.pathname).toBe('/')
  })

  it('TestRF020_StateMismatchShowsErrorWithRetry', async () => {
    await beginLogin()
    stubHub({ roles: ['contabilidad.senior'] })
    window.history.replaceState({}, '', '/oauth/callback?code=c&state=forged')
    render(<App />)
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('No se pudo validar el inicio de sesión')
    expect(screen.queryByRole('navigation', { name: 'Secciones' })).not.toBeInTheDocument()
    redirect.mockClear()
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    await waitFor(() => expect(redirect).toHaveBeenCalledTimes(1))
  })

  it('TestRF020_NothingStoredInBrowserStorageAfterSignIn', async () => {
    await signInAs(['contabilidad.senior'])
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
  })

  it('TestRF020_ExpiredTokenGoesBackToAuthorizeWithoutRefreshToken', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-01T12:00:00Z'))
    await beginLogin()
    const nowSeconds = Math.floor(Date.now() / 1000)
    stubHub({ roles: ['contabilidad.senior'], exp: nowSeconds + 61 })
    window.history.replaceState({}, '', `/oauth/callback?code=c&state=${sessionStorage.getItem('contabilidad.oauth_state')}`)
    redirect.mockClear()
    render(<App />)
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    expect(screen.getByRole('navigation', { name: 'Secciones' })).toBeInTheDocument()
    await act(async () => { await vi.advanceTimersByTimeAsync(61_000) })
    expect(redirect).toHaveBeenCalledTimes(1)
    expect(String(redirect.mock.calls[0]?.[0])).toContain(`${HUB}/oauth/authorize?`)
  })
  it('TestRF020_ShowsHubUnavailableMessageAndRetryWhenJwksFails', async () => {
    await beginLogin()
    const state = sessionStorage.getItem('contabilidad.oauth_state')
    const nowSeconds = Math.floor(Date.now() / 1000)
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === `${HUB}/oauth/token`) {
        const access = await signToken(key, validClaims({ iss: ISSUER, aud: [AUDIENCE], iat: nowSeconds, exp: nowSeconds + 900 }))
        return new Response(JSON.stringify({ access_token: access }), { status: 200 })
      }
      throw new TypeError('Failed to fetch')
    }))
    window.history.replaceState({}, '', `/oauth/callback?code=c&state=${state}`)
    render(<App />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Identity Hub no está disponible')
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument()
  })

  it('TestRF020_ShowsClockProblemInsteadOfRedirectingAgainForNearExpiryToken', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-01T12:00:00Z'))
    await beginLogin()
    const nowSeconds = Math.floor(Date.now() / 1000)
    stubHub({ roles: ['contabilidad.senior'], exp: nowSeconds + 60 })
    window.history.replaceState({}, '', `/oauth/callback?code=c&state=${sessionStorage.getItem('contabilidad.oauth_state')}`)
    redirect.mockClear()
    render(<App />)
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    expect(screen.getByRole('alert')).toHaveTextContent('reloj')
    expect(redirect).not.toHaveBeenCalled()
  })

})

describe('access denied', () => {
  it.each([[['admin']], [['user']], [[]], [null]])('TestRF009_TokenWithoutContabilidadRoleIsDenied %j', async (roles) => {
    await signInAs(roles, 'denied')
    // The access-denied screen replaces the whole shell: no rail, no ledger.
    expect(screen.queryByRole('navigation', { name: 'Secciones' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Sin acceso a Contabilidad' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Identity Hub/ })).toHaveAttribute('href', HUB)
    expect(screen.queryByText('M-2041')).not.toBeInTheDocument()
  })
})

describe('analyst role', () => {
  beforeEach(async () => { await signInAs(['contabilidad.analista']) })

  it('TestRF009_AnalystRailLocksClosingAndHidesAdministration', () => {
    expect(within(rail()).getByRole('button', { name: /Cierre contable/ })).toBeDisabled()
    expect(within(rail()).getByLabelText('Bloqueado')).toBeInTheDocument()
    expect(within(rail()).queryByRole('button', { name: /Administración/ })).not.toBeInTheDocument()
    expect(screen.getByText('Este rol solo ve y crea sus propios movimientos.')).toBeInTheDocument()
  })

  it('TestRF009_AnalystCannotOpenClosingContent', () => {
    goTo(/Cierre contable/)
    expect(screen.queryByRole('button', { name: /Cerrar mes de/ })).not.toBeInTheDocument()
    expect(screen.queryByText('Último cierre')).not.toBeInTheDocument()
  })

  it('TestRF009_AnalystSeesOnlyOwnTransactionsWithoutApprovalActions', () => {
    goTo('Transacciones')
    for (const folio of ['M-2043', 'M-2044', 'M-2046']) expect(screen.getByText(folio)).toBeInTheDocument()
    for (const folio of ['M-2041', 'M-2042', 'M-2045']) expect(screen.queryByText(folio)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Aprobar/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Rechazar/ })).not.toBeInTheDocument()
    expect(within(rowOf('M-2043')).getByText('esperando aprobación')).toBeInTheDocument()
  })

  it('TestRF009_AnalystSummaryOnlyCountsOwnMovements', () => {
    expect(screen.getByText('Mis aprobados')).toBeInTheDocument()
    expect(screen.getByText('Mis pendientes')).toBeInTheDocument()
    expect(screen.getByText('Mis registros del mes')).toBeInTheDocument()
    expect(screen.queryByText('Usuarios con acceso')).not.toBeInTheDocument()
  })

  it('TestRF009_AnalystRegistersAPendingMovementOfTheirOwn', () => {
    goTo('Transacciones')
    fireEvent.click(screen.getByRole('button', { name: '+ Registrar movimiento' }))
    fireEvent.change(screen.getByLabelText('Descripción'), { target: { value: 'Taxi aeropuerto' } })
    fireEvent.change(screen.getByLabelText('Monto'), { target: { value: '85000' } })
    fireEvent.click(screen.getByRole('button', { name: 'Registrar' }))
    const row = rowOf('M-2047')
    expect(within(row).getByText('Taxi aeropuerto')).toBeInTheDocument()
    expect(within(row).getByText('Pendiente')).toBeInTheDocument()
  })
})

describe('senior role', () => {
  beforeEach(async () => { await signInAs(['contabilidad.senior']) })

  it('TestRF009_SeniorSeesEveryTransactionAndApprovesAPendingRow', () => {
    goTo('Transacciones')
    for (const folio of ['M-2041', 'M-2042', 'M-2043', 'M-2044', 'M-2045', 'M-2046']) expect(screen.getByText(folio)).toBeInTheDocument()
    expect(within(rowOf('M-2043')).getByText('Pendiente')).toBeInTheDocument()
    fireEvent.click(within(rowOf('M-2043')).getByRole('button', { name: /Aprobar/ }))
    expect(within(rowOf('M-2043')).getByText('Aprobado')).toBeInTheDocument()
    expect(within(rowOf('M-2043')).queryByRole('button', { name: /Aprobar/ })).not.toBeInTheDocument()
    fireEvent.click(within(rowOf('M-2044')).getByRole('button', { name: /Rechazar/ }))
    expect(within(rowOf('M-2044')).getByText('Rechazado')).toBeInTheDocument()
  })

  it('TestRF009_SeniorCanOpenClosingAndCloseTheMonth', () => {
    expect(within(rail()).getByRole('button', { name: /Cierre contable/ })).toBeEnabled()
    goTo(/Cierre contable/)
    fireEvent.click(screen.getByRole('button', { name: /Cerrar mes de/ }))
    expect(screen.getByRole('status')).toHaveTextContent(/cierre/i)
  })

  it('TestRF009_SeniorHasNoAdministrationInTheRail', () => {
    expect(within(rail()).queryByRole('button', { name: /Administración/ })).not.toBeInTheDocument()
    expect(screen.getByText('Acceso completo dentro de Contabilidad.')).toBeInTheDocument()
    expect(screen.getByText('Usuarios con acceso')).toBeInTheDocument()
  })
})


describe('logout', () => {
  it('TestRF020_LogoutClearsMemoryAndReturnsToLogin', async () => {
    await signInAs(['contabilidad.senior'])
    fireEvent.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    expect(screen.getByRole('button', { name: 'Continuar con Identity Hub' })).toBeInTheDocument()
    expect(screen.queryByText('M-2041')).not.toBeInTheDocument()
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
    // A user that signed out does not reach the ledger by going back: a new login is required.
    await act(async () => { await Promise.resolve() })
    expect(screen.queryByRole('navigation', { name: 'Secciones' })).not.toBeInTheDocument()
  })
})
