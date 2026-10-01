import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { challengeFor } from './pkce'
import { beginLogin, completeCallback } from './flow'
import * as navigation from '../navigation'
import { AUDIENCE, HUB_ORIGIN, ISSUER } from '../config'
import { makeKey, signToken, validClaims, type TestKey } from '../testing'

const HUB = HUB_ORIGIN
let key: TestKey

beforeAll(async () => { key = await makeKey('hub-key') })

const jsonResponse = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

function stubHub(options: { token?: () => Promise<Response> | Response } = {}) {
  const calls: { url: string; init?: RequestInit }[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    calls.push({ url, init })
    if (url === `${HUB}/oauth/token`) {
      if (options.token) return options.token()
      const access = await signToken(key, validClaims({ iat: Math.floor(Date.now() / 1000), exp: Math.floor(Date.now() / 1000) + 900 }))
      return jsonResponse(200, { access_token: access, token_type: 'Bearer', expires_in: 900 })
    }
    if (url === `${HUB}/.well-known/jwks.json`) return jsonResponse(200, { keys: [key.jwk] })
    throw new Error(`unexpected request ${url}`)
  })
  vi.stubGlobal('fetch', fetchMock)
  return calls
}

beforeEach(() => {
  sessionStorage.clear()
  localStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  window.history.replaceState({}, '', '/')
})

describe('beginLogin: authorization request with PKCE', () => {
  it('TestRF020_BeginLoginRedirectsToHubWithS256ChallengeAndState', async () => {
    const redirect = vi.spyOn(navigation, 'redirectTo').mockImplementation(() => undefined)
    await beginLogin()
    const target = new URL(String(redirect.mock.calls[0]?.[0]))
    expect(`${target.origin}${target.pathname}`).toBe(`${HUB}/oauth/authorize`)
    expect(target.searchParams.get('client_id')).toBe(AUDIENCE)
    expect(target.searchParams.get('redirect_uri')).toBe(`${window.location.origin}/oauth/callback`)
    expect(target.searchParams.get('response_type')).toBe('code')
    expect(target.searchParams.get('code_challenge_method')).toBe('S256')
    const verifier = sessionStorage.getItem('contabilidad.pkce_verifier') ?? ''
    expect(verifier.length).toBeGreaterThanOrEqual(43)
    expect(target.searchParams.get('code_challenge')).toBe(await challengeFor(verifier))
    expect(target.searchParams.get('state')).toBe(sessionStorage.getItem('contabilidad.oauth_state'))
    expect(target.search).not.toContain(verifier)
  })
})

describe('completeCallback: code exchange and token handling', () => {
  async function startedFlow() {
    vi.spyOn(navigation, 'redirectTo').mockImplementation(() => undefined)
    await beginLogin()
    return { state: sessionStorage.getItem('contabilidad.oauth_state') ?? '', verifier: sessionStorage.getItem('contabilidad.pkce_verifier') ?? '' }
  }

  it('TestRF020_CallbackExchangesCodeAndKeepsTokenOutOfBrowserStorage', async () => {
    const { state, verifier } = await startedFlow()
    const calls = stubHub()
    window.history.replaceState({}, '', `/oauth/callback?code=the-code&state=${state}`)

    const session = await completeCallback(window.location.search)

    expect(session.verified.roles).toEqual(['contabilidad.senior'])
    expect(session.accessToken.split('.')).toHaveLength(3)
    const exchange = calls.find((call) => call.url === `${HUB}/oauth/token`)
    const form = new URLSearchParams(String(exchange?.init?.body))
    expect(Object.fromEntries(form)).toEqual({
      grant_type: 'authorization_code', code: 'the-code', code_verifier: verifier, client_id: AUDIENCE,
      redirect_uri: `${window.location.origin}/oauth/callback`,
    })
    expect(exchange?.init?.method).toBe('POST')
    expect(exchange?.init?.credentials).toBe('omit')
    // The token lives only in memory: nothing may remain in either storage.
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
    expect(JSON.stringify({ ...localStorage, ...sessionStorage })).not.toContain(session.accessToken)
    // code and state are gone from the address bar (and so from history and Referer).
    expect(window.location.search).toBe('')
    expect(window.location.href).not.toContain('the-code')
  })

  it('TestRF020_CallbackRejectsStateMismatchWithoutCallingTokenEndpoint', async () => {
    await startedFlow()
    const calls = stubHub()
    window.history.replaceState({}, '', '/oauth/callback?code=the-code&state=forged')
    await expect(completeCallback(window.location.search)).rejects.toMatchObject({ kind: 'state' })
    expect(calls).toHaveLength(0)
    expect(window.location.search).toBe('')
    expect(sessionStorage.length).toBe(0)
  })

  it('TestRF020_CallbackRejectsWhenNoFlowWasStarted', async () => {
    const calls = stubHub()
    window.history.replaceState({}, '', '/oauth/callback?code=the-code&state=anything')
    await expect(completeCallback(window.location.search)).rejects.toMatchObject({ kind: 'state' })
    expect(calls).toHaveLength(0)
  })

  it('TestRF020_StateIsSingleUse', async () => {
    const { state } = await startedFlow()
    stubHub()
    await completeCallback(`?code=one&state=${state}`)
    await expect(completeCallback(`?code=two&state=${state}`)).rejects.toMatchObject({ kind: 'state' })
  })

  it('TestRF020_CallbackReportsAuthorizationErrorFromHub', async () => {
    const { state } = await startedFlow()
    const calls = stubHub()
    await expect(completeCallback(`?error=invalid_request&state=${state}`)).rejects.toMatchObject({ kind: 'denied' })
    expect(calls).toHaveLength(0)
  })

  it('TestRF020_CallbackReportsTokenExchangeFailure', async () => {
    const { state } = await startedFlow()
    stubHub({ token: () => jsonResponse(400, { title: 'Bad Request' }) })
    await expect(completeCallback(`?code=bad&state=${state}`)).rejects.toMatchObject({ kind: 'exchange' })
    expect(sessionStorage.length).toBe(0)
  })

  it('TestRF020_CallbackRejectsTokenWithWrongIssuerFromTokenEndpoint', async () => {
    const { state } = await startedFlow()
    const forged = await signToken(key, validClaims({ iss: 'http://evil.localhost', exp: Math.floor(Date.now() / 1000) + 900 }))
    stubHub({ token: () => jsonResponse(200, { access_token: forged, token_type: 'Bearer', expires_in: 900 }) })
    await expect(completeCallback(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'token' })
    expect(ISSUER).toBe(HUB)
  })

  it('TestRF020_CallbackReportsJwksNetworkFailureAsHubUnavailable', async () => {
    const { state } = await startedFlow()
    const access = await signToken(key, validClaims({ exp: Math.floor(Date.now() / 1000) + 900 }))
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === `${HUB}/oauth/token`) return jsonResponse(200, { access_token: access })
      throw new TypeError('Failed to fetch')
    }))
    await expect(completeCallback(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'hub_unavailable' })
  })

  it('TestRF020_CallbackReportsJwksServerFailureAsHubUnavailable', async () => {
    const { state } = await startedFlow()
    const access = await signToken(key, validClaims({ exp: Math.floor(Date.now() / 1000) + 900 }))
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === `${HUB}/oauth/token`) return jsonResponse(200, { access_token: access })
      return jsonResponse(503, { title: 'Unavailable' })
    }))
    await expect(completeCallback(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'hub_unavailable' })
  })

  it.each([
    ['404 response', () => jsonResponse(404, { title: 'Not found' })],
    ['invalid JSON', () => new Response('{', { status: 200, headers: { 'Content-Type': 'application/json' } })],
    ['missing keys array', () => jsonResponse(200, {})],
  ])('TestRF020_CallbackReportsJwks%sAsHubUnavailable', async (_case, jwksResponse) => {
    const { state } = await startedFlow()
    const access = await signToken(key, validClaims({ exp: Math.floor(Date.now() / 1000) + 900 }))
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === `${HUB}/oauth/token`) return jsonResponse(200, { access_token: access })
      return jwksResponse()
    }))
    await expect(completeCallback(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'hub_unavailable' })
  })

  it('TestRF020_CallbackReportsNetworkFailureAsExchangeError', async () => {
    const { state } = await startedFlow()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    await expect(completeCallback(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'exchange' })
  })
})
