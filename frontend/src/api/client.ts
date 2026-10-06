import type { components, operations } from './schema'

type LoginRequest = operations['login']['requestBody']['content']['application/json']
export type MfaChallenge = components['schemas']['MfaChallenge']
type MfaVerifyRequest = operations['verifyMfa']['requestBody']['content']['application/json']
type MfaResendRequest = operations['resendMfaCode']['requestBody']['content']['application/json']
type TokenPair = components['schemas']['TokenPair']
type InvitationAcceptRequest = operations['acceptInvitation']['requestBody']['content']['application/json']
type EmailRequest = operations['requestPasswordReset']['requestBody']['content']['application/json']
type PasswordResetConfirmRequest = operations['confirmPasswordReset']['requestBody']['content']['application/json']
type UpdateProfileRequest = operations['updateCurrentUser']['requestBody']['content']['application/json']
export type Session = components['schemas']['Session']
export type CurrentUser = components['schemas']['User']
export type Problem = components['schemas']['Problem']
export type Role = components['schemas']['Role']
export type Application = components['schemas']['Application']
export type ApplicationRole = components['schemas']['ApplicationRole']
export type Permission = components['schemas']['Permission']
type CreateApplicationRoleRequest = components['schemas']['CreateApplicationRoleRequest']
type UpdateApplicationRoleRequest = components['schemas']['UpdateApplicationRoleRequest']
export type UserStatus = components['schemas']['UserStatus']
export type AdminUser = components['schemas']['User']
export type UserPage = components['schemas']['UserPage']
export type AuditEvent = components['schemas']['AuditEvent']
export type AuditLogPage = components['schemas']['AuditLogPage']
type CreateUserRequest = operations['createEmployee']['requestBody']['content']['application/json']
type UpdateUserRequest = operations['updateUser']['requestBody']['content']['application/json']
export type ListUsersQuery = NonNullable<operations['listUsers']['parameters']['query']>
export type AuditLogQuery = NonNullable<operations['listAuditLog']['parameters']['query']>

const refreshLockName = 'identity-hub-refresh'
let accessToken: string | null = null
let refreshInFlight: Promise<void> | null = null
let tokenChannel: BroadcastChannel | null = null

export class ApiProblemError extends Error {
  readonly status: number
  readonly title: string
  readonly detail?: string
  readonly errors: NonNullable<Problem['errors']>

  constructor(problem: Problem, fallbackStatus: number) {
    super(problem.detail ?? problem.title)
    this.name = 'ApiProblemError'
    this.status = problem.status ?? fallbackStatus
    this.title = problem.title
    this.detail = problem.detail
    this.errors = problem.errors ?? []
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
      ...(Array.isArray(problem.errors) ? { errors: problem.errors } : {}),
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
  // No-content responses (204/205 or an empty body) carry nothing to parse.
  if (response.status === 204 || response.status === 205) return undefined as T
  const body = await response.text()
  if (!body.trim()) return undefined as T
  // A body that is not JSON (e.g. a proxy error page) must not look like success.
  if (!response.headers.get('content-type')?.includes('application/json')) {
    throw new Error('Respuesta inesperada del servidor: el cuerpo no es JSON.')
  }
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

export const acceptInvitation = (input: InvitationAcceptRequest) => post<void>('/api/v1/auth/invitations/accept', input)
export const requestPasswordReset = (input: EmailRequest) => post<void>('/api/v1/auth/password-reset/request', input)
export const confirmPasswordReset = (input: PasswordResetConfirmRequest) => post<void>('/api/v1/auth/password-reset/confirm', input)
export const updateCurrentUser = (input: UpdateProfileRequest) =>
  authenticatedRequest<CurrentUser>('/api/v1/me', { method: 'PATCH', body: JSON.stringify(input) })
export const listSessions = () => authenticatedRequest<Session[]>('/api/v1/me/sessions')
export const revokeSession = (sessionId: string) =>
  authenticatedRequest<void>(`/api/v1/me/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' })

function withQuery(path: string, query: Record<string, string | number | undefined>): string {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== '') params.set(key, String(value))
  }
  const text = params.toString()
  return text ? `${path}?${text}` : path
}

export const listUsers = (query: ListUsersQuery = {}) => authenticatedRequest<UserPage>(withQuery('/api/v1/admin/users', query))
export const createUser = (input: CreateUserRequest) =>
  authenticatedRequest<AdminUser>('/api/v1/admin/users', { method: 'POST', body: JSON.stringify(input) })
export const updateUser = (userId: string, input: UpdateUserRequest) =>
  authenticatedRequest<AdminUser>(`/api/v1/admin/users/${encodeURIComponent(userId)}`, { method: 'PATCH', body: JSON.stringify(input) })
export const resendInvitation = (userId: string) =>
  authenticatedRequest<void>(`/api/v1/admin/users/${encodeURIComponent(userId)}/invitation`, { method: 'POST' })
export const listAuditLog = (query: AuditLogQuery = {}) => authenticatedRequest<AuditLogPage>(withQuery('/api/v1/admin/audit-log', query))

export const listApplications = () => authenticatedRequest<Application[]>('/api/v1/admin/applications')
const rolesPath = (applicationId: string) => `/api/v1/admin/applications/${encodeURIComponent(applicationId)}/roles`
export const createApplicationRole = (applicationId: string, input: CreateApplicationRoleRequest) =>
  authenticatedRequest<ApplicationRole>(rolesPath(applicationId), { method: 'POST', body: JSON.stringify(input) })
export const updateApplicationRole = (applicationId: string, roleId: string, input: UpdateApplicationRoleRequest) =>
  authenticatedRequest<ApplicationRole>(`${rolesPath(applicationId)}/${encodeURIComponent(roleId)}`, { method: 'PATCH', body: JSON.stringify(input) })
export const deleteApplicationRole = (applicationId: string, roleId: string) =>
  authenticatedRequest<void>(`${rolesPath(applicationId)}/${encodeURIComponent(roleId)}`, { method: 'DELETE' })

export async function logout(): Promise<void> {
  try {
    await post<void>('/api/v1/auth/logout')
  } finally {
    setAccessToken(null)
  }
}

// Clears only this tab's token. It does not announce null: the failure that led
// here (e.g. a profile fetch) is local and must not sign the other tabs out.
export function clearSession() {
  setAccessToken(null, false)
}

// Test-only reset; session state is module memory and never uses Web Storage.
export function resetSessionForTests() {
  accessToken = null
  refreshInFlight = null
  tokenChannel?.close()
  tokenChannel = null
}
