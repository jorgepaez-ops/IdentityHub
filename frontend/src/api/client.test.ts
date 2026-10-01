import { afterEach, describe, expect, it, vi } from 'vitest'
import { authenticatedRequest, refreshSession, resendMfaCode, resetSessionForTests } from './client'

const json = (status: number, body: unknown, contentType = 'application/json') =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': contentType } })

describe('API client', () => {
  afterEach(() => {
    resetSessionForTests()
    vi.unstubAllGlobals()
  })

  it('TestRF003_ParsesProblemJSON', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(423, { type: 'locked', title: 'Locked', status: 423, detail: 'Retry later' }, 'application/problem+json')))

    await expect(authenticatedRequest('/api/v1/me')).rejects.toEqual(
      expect.objectContaining({ status: 423, title: 'Locked', detail: 'Retry later' }),
    )
  })

  it('TestRF005_RetriesOnceAfterRefresh', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(json(200, { accessToken: 'first', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }, 'application/problem+json'))
      .mockResolvedValueOnce(json(200, { accessToken: 'second', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(200, { id: 'user', email: 'user@example.test' }))
    vi.stubGlobal('fetch', fetch)

    await refreshSession()
    await expect(authenticatedRequest('/api/v1/me')).resolves.toEqual({ id: 'user', email: 'user@example.test' })
    expect(fetch).toHaveBeenCalledTimes(4)
  })

  it('TestRF006_SerializesConcurrentRefreshes', async () => {
    const fetch = vi.fn(async () => json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }))
    vi.stubGlobal('fetch', fetch)

    await Promise.all([refreshSession(), refreshSession()])
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('TestRF014_AcceptsEmptyResendResponse', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 202 })))

    await expect(resendMfaCode({ mfaToken: 'challenge' })).resolves.toBeUndefined()
  })

  it('TestRF014_AcceptsSuccessfulResponsesWithoutConsumableJSON', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 202, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response('queued', { status: 202, headers: { 'Content-Type': 'text/plain' } }))
    vi.stubGlobal('fetch', fetch)

    await expect(resendMfaCode({ mfaToken: 'challenge' })).resolves.toBeUndefined()
    await expect(resendMfaCode({ mfaToken: 'challenge' })).resolves.toBeUndefined()
    await expect(resendMfaCode({ mfaToken: 'challenge' })).resolves.toBeUndefined()
  })

  it('TestRF005_ParsesSuccessfulJSONResponse', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => json(200, { id: 'user', email: 'user@example.test' })))

    await expect(authenticatedRequest('/api/v1/me')).resolves.toEqual({ id: 'user', email: 'user@example.test' })
  })

  it('TestRF005_ClearsTokenAndRethrowsWhenRefreshFails', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }, 'application/problem+json'))
      .mockResolvedValueOnce(json(200, { id: 'user', email: 'user@example.test' }))
    vi.stubGlobal('fetch', fetch)

    await refreshSession()
    await expect(refreshSession()).rejects.toEqual(expect.objectContaining({ status: 401 }))
    await expect(authenticatedRequest('/api/v1/me')).resolves.toEqual({ id: 'user', email: 'user@example.test' })
    const finalRequest = fetch.mock.calls.at(-1)?.[1] as RequestInit | undefined
    expect(new Headers(finalRequest?.headers).get('Authorization')).toBeNull()
  })

  it('TestRF005_RethrowsWhenRefreshAfterUnauthorizedFails', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 }))
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }, 'application/problem+json'))
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }, 'application/problem+json'))
    vi.stubGlobal('fetch', fetch)

    await refreshSession()
    await expect(authenticatedRequest('/api/v1/me')).rejects.toEqual(expect.objectContaining({ status: 401 }))
    expect(fetch).toHaveBeenCalledTimes(3)
  })

  it('TestRF005_ReusesAndClosesTheSessionBroadcastChannel', async () => {
    class TestBroadcastChannel {
      static instances: TestBroadcastChannel[] = []
      readonly name: string
      onmessage: ((event: MessageEvent<{ token?: unknown }>) => void) | null = null
      close = vi.fn()
      postMessage = vi.fn()

      constructor(name: string) {
        this.name = name
        TestBroadcastChannel.instances.push(this)
      }
    }
    vi.stubGlobal('BroadcastChannel', TestBroadcastChannel)
    vi.resetModules()
    const session = await import('./client')
    vi.stubGlobal('fetch', vi.fn(async () => json(200, { accessToken: 'access', tokenType: 'Bearer', expiresIn: 900 })))

    await session.refreshSession()
    await session.refreshSession()
    expect(TestBroadcastChannel.instances).toHaveLength(1)
    session.resetSessionForTests()
    expect(TestBroadcastChannel.instances[0]?.close).toHaveBeenCalledOnce()
  })

  it('TestRF005_ReceivesTokenBroadcastBeforeLocalPublication', async () => {
    class TestBroadcastChannel {
      static instances: TestBroadcastChannel[] = []
      readonly name: string
      onmessage: ((event: MessageEvent<{ token?: unknown }>) => void) | null = null
      close = vi.fn()
      postMessage = vi.fn()

      constructor(name: string) {
        this.name = name
        TestBroadcastChannel.instances.push(this)
      }
    }
    vi.stubGlobal('BroadcastChannel', TestBroadcastChannel)
    vi.resetModules()
    const session = await import('./client')
    const fetch = vi.fn()
      .mockResolvedValueOnce(json(401, { title: 'Unauthorized', status: 401 }, 'application/problem+json'))
      .mockResolvedValueOnce(json(200, { id: 'user' }))
    vi.stubGlobal('fetch', fetch)

    await expect(session.refreshSession()).rejects.toEqual(expect.objectContaining({ status: 401 }))
    TestBroadcastChannel.instances[0]?.onmessage?.({ data: { token: 'remote-access' } } as MessageEvent<{ token: unknown }>)
    await expect(session.authenticatedRequest('/api/v1/me')).resolves.toEqual({ id: 'user' })
    const finalRequest = fetch.mock.calls.at(-1)?.[1] as RequestInit | undefined
    expect(new Headers(finalRequest?.headers).get('Authorization')).toBe('Bearer remote-access')
    session.resetSessionForTests()
  })
})
