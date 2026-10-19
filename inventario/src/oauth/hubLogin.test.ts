import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import * as browser from '../browser'
import { CLIENT_ID, HUB_ORIGIN } from '../hub'
import { FakeHub, reply, silentUntilAborted } from '../testkit/fakeHub'
import { DEADLINE_MS, finishAuthorization, startAuthorization } from './hubLogin'
import { s256 } from './pkce'

/**
 * `FakeHub` (src/testkit/fakeHub.ts) stands in for the Hub in these tests. What the tests rely on:
 *  - `hub.serveToken(overrides)` replaces `fetch`: the token endpoint answers with a freshly signed
 *    token for the claims `hub.claims(now, overrides)` (issuer, audience `inventario`, a 15 minute
 *    life, role `inventario.bodeguero`), the key set endpoint serves the matching public key, and it
 *    returns the list of requests seen so far.
 *  - `hub.serve(answer)` does the same but lets the test decide what the token endpoint answers.
 *  - `hub.mint(claims)` signs an arbitrary claim set.
 */
let hub: FakeHub
beforeAll(async () => { hub = await FakeHub.create() })

const nowSeconds = () => Math.floor(Date.now() / 1000)

beforeEach(() => {
  sessionStorage.clear()
  localStorage.clear()
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  window.history.replaceState(null, '', '/')
})

/** Starts a login and hands back what the Hub would see and what the browser kept for the return. */
async function leaveForTheHub() {
  const navigate = vi.spyOn(browser, 'goTo').mockImplementation(() => undefined)
  await startAuthorization()
  const target = new URL(String(navigate.mock.calls[0]?.[0]))
  return {
    target,
    state: sessionStorage.getItem('inventario.login.state') ?? '',
    verifier: sessionStorage.getItem('inventario.login.verifier') ?? '',
  }
}

describe('startAuthorization', () => {
  it('TestRF020_TheAuthorizationRequestCarriesAnS256ChallengeAndAState', async () => {
    const { target, state, verifier } = await leaveForTheHub()
    expect(`${target.origin}${target.pathname}`).toBe(`${HUB_ORIGIN}/oauth/authorize`)
    expect(Object.fromEntries(target.searchParams)).toEqual({
      response_type: 'code',
      client_id: CLIENT_ID,
      redirect_uri: `${window.location.origin}/oauth/callback`,
      code_challenge: await s256(verifier),
      code_challenge_method: 'S256',
      state,
    })
    expect(verifier.length).toBeGreaterThanOrEqual(43)
    expect(target.search).not.toContain(verifier)
  })
})

describe('finishAuthorization', () => {
  it('TestRF020_TradesTheCodeForATokenAndKeepsItOutOfEveryStorage', async () => {
    const { state, verifier } = await leaveForTheHub()
    const seen = hub.serveToken()
    const result = await finishAuthorization(`?code=the-code&state=${state}`)

    expect(result.accessToken.split('.')).toHaveLength(3)
    expect(result.identity.roles).toEqual(['inventario.bodeguero'])
    const exchange = seen.find((call) => call.url === `${HUB_ORIGIN}/oauth/token`)
    expect(exchange?.init?.method).toBe('POST')
    expect(exchange?.init?.credentials).toBe('omit')
    expect(Object.fromEntries(new URLSearchParams(String(exchange?.init?.body)))).toEqual({
      grant_type: 'authorization_code',
      code: 'the-code',
      code_verifier: verifier,
      client_id: CLIENT_ID,
      redirect_uri: `${window.location.origin}/oauth/callback`,
    })
    expect(localStorage).toHaveLength(0)
    expect(sessionStorage).toHaveLength(0)
    expect(window.location.search).toBe('')
  })

  it('TestRF020_AfterASuccessfulReturnTheCallbackUrlIsReplacedByTheAppRoot', async () => {
    const { state } = await leaveForTheHub()
    hub.serveToken()
    window.history.replaceState(null, '', `/oauth/callback?code=the-code&state=${state}`)
    await finishAuthorization(window.location.search)
    expect(window.location.pathname).toBe('/')
    expect(window.location.search).toBe('')
    expect(window.location.href).not.toContain('the-code')
  })

  it('TestRF020_AfterAFailedReturnTheCallbackUrlIsReplacedByTheAppRootToo', async () => {
    window.history.replaceState(null, '', '/oauth/callback?code=the-code&state=forged')
    await expect(finishAuthorization(window.location.search)).rejects.toMatchObject({ kind: 'state' })
    expect(window.location.pathname).toBe('/')
    expect(window.location.href).not.toContain('the-code')
  })

  it('TestRF020_AForgedStateIsRefusedBeforeAnyRequestAndEmptiesTheSession', async () => {
    await leaveForTheHub()
    const seen = hub.serveToken()
    window.history.replaceState(null, '', '/oauth/callback?code=c&state=forged')
    await expect(finishAuthorization(window.location.search)).rejects.toMatchObject({ kind: 'state' })
    expect(seen).toHaveLength(0)
    expect(window.location.search).toBe('')
    expect(sessionStorage).toHaveLength(0)
  })

  it('TestRF020_AReturnWithoutAStartedLoginIsRefused', async () => {
    const seen = hub.serveToken()
    await expect(finishAuthorization('?code=c&state=anything')).rejects.toMatchObject({ kind: 'state' })
    expect(seen).toHaveLength(0)
  })

  it('TestRF020_TheStateWorksOnlyOnce', async () => {
    const { state } = await leaveForTheHub()
    hub.serveToken()
    await finishAuthorization(`?code=one&state=${state}`)
    await expect(finishAuthorization(`?code=two&state=${state}`)).rejects.toMatchObject({ kind: 'state' })
  })

  it('TestRF020_AHubErrorIsReportedWithoutRequestingAToken', async () => {
    const { state } = await leaveForTheHub()
    const seen = hub.serveToken()
    await expect(finishAuthorization(`?error=invalid_request&state=${state}`)).rejects.toMatchObject({ kind: 'refused' })
    expect(seen).toHaveLength(0)
  })

  it('TestRF022_AccessDeniedMeansNoAccessAndRequestsNothing', async () => {
    const { state } = await leaveForTheHub()
    const seen = hub.serveToken()
    await expect(finishAuthorization(`?error=access_denied&state=${state}`)).rejects.toMatchObject({ kind: 'no_access' })
    expect(seen).toHaveLength(0)
  })

  it('TestRF020_AReturnWithNeitherCodeNorErrorIsRefused', async () => {
    const { state } = await leaveForTheHub()
    await expect(finishAuthorization(`?state=${state}`)).rejects.toMatchObject({ kind: 'refused' })
  })

  it.each([
    ['an error status', () => reply(400, { title: 'Bad Request' })],
    ['a body that is not JSON', () => new Response('{', { status: 200 })],
    ['a body without access_token', () => reply(200, { token_type: 'Bearer' })],
  ])('TestRF020_TokenEndpointAnswering%sIsAnExchangeFailure', async (_label, answer) => {
    const { state } = await leaveForTheHub()
    hub.serve(answer)
    await expect(finishAuthorization(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'exchange' })
  })

  it('TestRF020_ANetworkFailureAtTheTokenEndpointIsAnExchangeFailure', async () => {
    const { state } = await leaveForTheHub()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    await expect(finishAuthorization(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'exchange' })
  })

  it('TestRF020_ATokenFromAnotherIssuerIsRejected', async () => {
    const { state } = await leaveForTheHub()
    hub.serveToken({ iss: 'http://evil.localhost' })
    await expect(finishAuthorization(`?code=c&state=${state}`)).rejects.toMatchObject({ kind: 'token' })
  })

  describe('when the key set cannot be had', () => {
    async function withKeySet(answer: (init?: RequestInit) => Response | Promise<Response>) {
      const { state } = await leaveForTheHub()
      const access = await hub.mint(hub.claims(nowSeconds()))
      vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) =>
        String(input).endsWith('/oauth/token') ? reply(200, { access_token: access }) : answer(init)))
      return finishAuthorization(`?code=c&state=${state}`)
    }

    it.each([
      ['not found', () => reply(404, { title: 'Not found' })],
      ['unavailable', () => reply(503, { title: 'Unavailable' })],
      ['not JSON', () => new Response('{', { status: 200 })],
      ['without keys', () => reply(200, {})],
    ])('TestRF020_AKeySetThatIs%sMeansTheHubIsDown', async (_label, answer) => {
      await expect(withKeySet(answer)).rejects.toMatchObject({ kind: 'hub_down' })
    })

    it('TestRF020_ANetworkFailureAtTheKeySetMeansTheHubIsDown', async () => {
      await expect(withKeySet(() => { throw new TypeError('Failed to fetch') })).rejects.toMatchObject({ kind: 'hub_down' })
    })
  })

  describe('requests that never answer', () => {
    const settle = (promise: Promise<unknown>) => promise.then(() => null, (problem: unknown) => problem)

    it('TestRF020_ASilentTokenEndpointIsAbortedAtTheDeadline', async () => {
      const { state } = await leaveForTheHub()
      const fetchMock = vi.fn((_input: RequestInfo | URL, init?: RequestInit) => silentUntilAborted(init))
      vi.stubGlobal('fetch', fetchMock)
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
      const outcome = settle(finishAuthorization(`?code=c&state=${state}`))
      const aborted = () => fetchMock.mock.calls[0]?.[1]?.signal?.aborted
      await vi.advanceTimersByTimeAsync(DEADLINE_MS - 1)
      expect(aborted()).toBe(false)
      await vi.advanceTimersByTimeAsync(1)
      expect(aborted()).toBe(true)
      expect(await outcome).toMatchObject({ kind: 'exchange' })
    })

    it('TestRF020_ASilentKeySetEndpointIsAbortedAtTheDeadline', async () => {
      const { state } = await leaveForTheHub()
      const access = await hub.mint(hub.claims(nowSeconds()))
      vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) =>
        String(input).endsWith('/oauth/token') ? reply(200, { access_token: access }) : silentUntilAborted(init)))
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
      const outcome = settle(finishAuthorization(`?code=c&state=${state}`))
      await vi.advanceTimersByTimeAsync(0)
      await vi.advanceTimersByTimeAsync(DEADLINE_MS)
      expect(await outcome).toMatchObject({ kind: 'hub_down' })
    })

    it('TestRF020_AResponseBodyThatNeverArrivesIsCutOffToo', async () => {
      const { state } = await leaveForTheHub()
      vi.stubGlobal('fetch', vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
        const body = new ReadableStream({
          start(controller) { init?.signal?.addEventListener('abort', () => controller.error(new DOMException('aborted', 'AbortError'))) },
        })
        return new Response(body, { status: 200 })
      }))
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
      const outcome = settle(finishAuthorization(`?code=c&state=${state}`))
      await vi.advanceTimersByTimeAsync(DEADLINE_MS)
      expect(await outcome).toMatchObject({ kind: 'exchange' })
    })
  })
})
