import { act, renderHook, waitFor } from '@testing-library/react'
import { StrictMode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import * as browser from './browser'
import { HUB_ORIGIN } from './hub'
import { startAuthorization } from './oauth/hubLogin'
import { MIN_REMAINING_MS, phaseReducer, useHubSession, type Phase } from './session'
import { installFakeHubEnvironment } from './testkit/hubEnvironment'

const fakeHub = installFakeHubEnvironment()

const identity = { subject: 's', roles: ['inventario.bodeguero'], permissions: ['productos.ver'], expiresAt: 2_000_000_000 }
const session = { accessToken: 'a.b.c', identity }

describe('phaseReducer', () => {
  const phases: Phase[] = [{ tag: 'signed_out' }, { tag: 'checking' }, { tag: 'no_access' }]

  it.each(phases)('TestRF020_EveryActionReplacesThePhase_%o', (from) => {
    expect(phaseReducer(from, { type: 'checking' })).toEqual({ tag: 'checking' })
    expect(phaseReducer(from, { type: 'failed', message: 'x' })).toEqual({ tag: 'failed', message: 'x' })
    expect(phaseReducer(from, { type: 'no_access' })).toEqual({ tag: 'no_access' })
    expect(phaseReducer(from, { type: 'active', session })).toEqual({ tag: 'active', session })
    expect(phaseReducer(from, { type: 'signed_out' })).toEqual({ tag: 'signed_out' })
  })

  it('TestRF020_SigningOutOfAnActiveSessionDropsTheToken', () => {
    const next = phaseReducer({ tag: 'active', session }, { type: 'signed_out' })
    expect(JSON.stringify(next)).not.toContain('a.b.c')
  })
})

/** Plays the way back from the Hub and mounts the hook on the callback URL. */
async function returnFromHub(token: Record<string, unknown> = {}, wrapper?: typeof StrictMode) {
  await startAuthorization()
  const state = sessionStorage.getItem('inventario.login.state') ?? ''
  const seen = fakeHub().serveToken(token)
  window.history.replaceState(null, '', `/oauth/callback?code=c&state=${state}`)
  return { seen, view: renderHook(() => useHubSession(), { wrapper }) }
}

describe('useHubSession', () => {
  it('TestRF020_StartsSignedOutOutsideTheCallback', () => {
    const { result } = renderHook(() => useHubSession())
    expect(result.current.phase).toEqual({ tag: 'signed_out' })
  })

  it('TestRF020_StartsCheckingOnTheCallbackThenBecomesActive', async () => {
    const { view } = await returnFromHub()
    expect(view.result.current.phase.tag).toBe('checking')
    await waitFor(() => expect(view.result.current.phase.tag).toBe('active'))
  })

  it('TestRF020_AStrictModeRemountProcessesTheSingleUseCodeOnce', async () => {
    const { seen, view } = await returnFromHub({}, StrictMode)
    await waitFor(() => expect(view.result.current.phase.tag).toBe('active'))
    expect(seen.filter((call) => call.url === `${HUB_ORIGIN}/oauth/token`)).toHaveLength(1)
  })

  it('TestRF020_AForgedStateEndsFailed', async () => {
    await startAuthorization()
    fakeHub().serveToken()
    window.history.replaceState(null, '', '/oauth/callback?code=c&state=forged')
    const { result } = renderHook(() => useHubSession())
    await waitFor(() => expect(result.current.phase.tag).toBe('failed'))
  })

  it('TestRF022_AccessDeniedEndsWithNoAccess', async () => {
    await startAuthorization()
    const state = sessionStorage.getItem('inventario.login.state')
    window.history.replaceState(null, '', `/oauth/callback?error=access_denied&state=${state}`)
    const { result } = renderHook(() => useHubSession())
    await waitFor(() => expect(result.current.phase.tag).toBe('no_access'))
  })

  it('TestRF020_ATokenAboutToExpireIsAClockProblemNotASession', async () => {
    const exp = Math.floor(Date.now() / 1000) + MIN_REMAINING_MS / 1000 - 30
    const { view } = await returnFromHub({ exp })
    await waitFor(() => expect(view.result.current.phase).toMatchObject({ tag: 'failed' }))
    expect(JSON.stringify(view.result.current.phase)).toContain('reloj')
  })

  it('TestRF020_SigningOutReturnsToSignedOut', async () => {
    const { view } = await returnFromHub()
    await waitFor(() => expect(view.result.current.phase.tag).toBe('active'))
    act(() => view.result.current.signOut())
    expect(view.result.current.phase).toEqual({ tag: 'signed_out' })
  })

  it('TestRF020_WhenTheTokenExpiresTheHubIsAskedAgainAndTheTokenIsDropped', async () => {
    const timers = vi.spyOn(window, 'setTimeout')
    const { view } = await returnFromHub()
    await waitFor(() => expect(view.result.current.phase.tag).toBe('active'))
    const expiresAt = (view.result.current.phase as Extract<Phase, { tag: 'active' }>).session.identity.expiresAt
    vi.mocked(browser.goTo).mockClear()

    // The expiry timer is the one scheduled for the moment the token runs out.
    const due = timers.mock.calls.find(([, delay]) => typeof delay === 'number' && Math.abs(delay - (expiresAt * 1000 - Date.now())) < 5000)
    expect(due).toBeDefined()
    await act(async () => { (due?.[0] as () => void)() })

    expect(view.result.current.phase.tag).toBe('checking')
    await waitFor(() => expect(browser.goTo).toHaveBeenCalledTimes(1))
    expect(String(vi.mocked(browser.goTo).mock.calls[0]?.[0])).toContain(`${HUB_ORIGIN}/oauth/authorize`)
  })

  it('TestRF020_SigningOutCancelsTheExpiryTimer', async () => {
    const cleared = vi.spyOn(window, 'clearTimeout')
    const { view } = await returnFromHub()
    await waitFor(() => expect(view.result.current.phase.tag).toBe('active'))
    const before = cleared.mock.calls.length
    act(() => view.result.current.signOut())
    expect(cleared.mock.calls.length).toBeGreaterThan(before)
  })
})
