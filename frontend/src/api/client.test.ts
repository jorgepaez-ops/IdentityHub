import { afterEach, describe, expect, it, vi } from 'vitest'
import { authenticatedRequest, refreshSession, resetSessionForTests } from './client'

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
})
