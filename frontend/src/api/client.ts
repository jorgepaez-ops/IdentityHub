import type { components, operations } from './schema'

type LoginRequest = operations['login']['requestBody']['content']['application/json']
export type MfaChallenge = components['schemas']['MfaChallenge']
type MfaVerifyRequest = operations['verifyMfa']['requestBody']['content']['application/json']
type MfaResendRequest = operations['resendMfaCode']['requestBody']['content']['application/json']
type TokenPair = components['schemas']['TokenPair']
export type CurrentUser = components['schemas']['User']
export type Problem = components['schemas']['Problem']

const refreshLockName = 'identity-hub-refresh'
let accessToken: string | null = null
let refreshInFlight: Promise<void> | null = null
let tokenChannel: BroadcastChannel | null = null

export class ApiProblemError extends Error {
  readonly status: number
  readonly title: string
  readonly detail?: string

  constructor(problem: Problem, fallbackStatus: number) {
    super(problem.detail ?? problem.title)
    this.name = 'ApiProblemError'
    this.status = problem.status ?? fallbackStatus
    this.title = problem.title
    this.detail = problem.detail
  }
}

function getTokenChannel(): BroadcastChannel | null {
  if (tokenChannel || typeof BroadcastChannel === 'undefined') return tokenChannel
  tokenChannel = new BroadcastChannel(refreshLockName)
  tokenChannel.onmessage = (event: MessageEvent<{ token?: unknown }>) => {
    setAccessToken(typeof event.data.token === 'string' ? event.data.token : null, false)
  }
  return tokenChannel
}

function setAccessToken(token: string | null, announce = true) {
  accessToken = token
  if (announce) getTokenChannel()?.postMessage({ token })
}

async function parseError(response: Response): Promise<ApiProblemError> {
  let problem: Partial<Problem> = {}
  if (response.headers.get('content-type')?.includes('application/problem+json')) {
    try {
      problem = (await response.json()) as Problem
    } catch {
      // A proxy can replace the body; retain the HTTP status in that case.
    }
  }
  return new ApiProblemError(
    {
      type: problem.type ?? 'about:blank',
      title: problem.title ?? (response.statusText || 'Error de la API'),
      status: problem.status ?? response.status,
      ...(problem.detail ? { detail: problem.detail } : {}),
    },
    response.status,
  )
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, { ...init, headers, credentials: 'same-origin' })
  if (!response.ok) throw await parseError(response)
  if (response.status === 204 || !response.headers.get('content-type')?.includes('application/json')) return undefined as T
  const body = await response.text()
  if (!body.trim()) return undefined as T
  return JSON.parse(body) as T
}

async function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, { method: 'POST', ...(body === undefined ? {} : { body: JSON.stringify(body) }) })
}

async function runExclusive<T>(operation: () => Promise<T>): Promise<T> {
  if (navigator.locks) return navigator.locks.request(refreshLockName, operation)
  return operation()
}

export async function refreshSession(): Promise<void> {
  getTokenChannel()
  if (refreshInFlight) return refreshInFlight
  const tokenBeforeRefresh = accessToken
  const operation = runExclusive(async () => {
    // A sibling tab may have completed the rotation while this tab waited.
    if (accessToken !== tokenBeforeRefresh) return
    const pair = await post<TokenPair>('/api/v1/auth/refresh')
    setAccessToken(pair.accessToken)
  })
  refreshInFlight = operation
  try {
    await operation
  } catch (error) {
    setAccessToken(null)
    throw error
  } finally {
    if (refreshInFlight === operation) refreshInFlight = null
  }
}

export async function authenticatedRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const withToken = () => {
    const headers = new Headers(init.headers)
    if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`)
    return request<T>(path, { ...init, headers })
  }

  try {
    return await withToken()
  } catch (error) {
    if (!(error instanceof ApiProblemError) || error.status !== 401) throw error
    await refreshSession()
    return withToken()
  }
}

export const login = (input: LoginRequest) => post<MfaChallenge>('/api/v1/auth/login', input)

export async function verifyMfa(input: MfaVerifyRequest): Promise<void> {
  const pair = await post<TokenPair>('/api/v1/auth/mfa/verify', input)
  setAccessToken(pair.accessToken)
}

export const resendMfaCode = (input: MfaResendRequest) => post<void>('/api/v1/auth/mfa/resend', input)
export const getCurrentUser = () => authenticatedRequest<CurrentUser>('/api/v1/me')

export async function logout(): Promise<void> {
  try {
    await post<void>('/api/v1/auth/logout')
  } finally {
    setAccessToken(null)
  }
}

export function clearSession() {
  setAccessToken(null)
}

// Test-only reset; session state is module memory and never uses Web Storage.
export function resetSessionForTests() {
  accessToken = null
  refreshInFlight = null
  tokenChannel?.close()
  tokenChannel = null
}
