import { hubUrl } from './config'
import { extractCode, mailIds, waitForMail } from './mailpit'

// Upper bound for any single HTTP call, so a hung API or Mailpit cannot stall a test forever.
export const FETCH_TIMEOUT_MS = 15_000

// Nginx allows 5 r/s (burst 5) on /api/v1/auth/. Keep calls at least 300 ms apart.
const PACE_MS = 300
let lastCall = 0

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

export interface ApiResult<T = unknown> {
  status: number
  body: T
}

export class LoginRejectedError extends Error {
  constructor(
    public readonly status: number,
    email: string,
  ) {
    super(`login for ${email} returned ${status}`)
    this.name = 'LoginRejectedError'
  }
}

export async function paced(): Promise<void> {
  const wait = lastCall + PACE_MS - Date.now()
  if (wait > 0) await sleep(wait)
  lastCall = Date.now()
}

export async function api<T = unknown>(
  method: string,
  path: string,
  options: { json?: unknown; token?: string } = {},
): Promise<ApiResult<T>> {
  await paced()
  const headers: Record<string, string> = {}
  if (options.json !== undefined) headers['Content-Type'] = 'application/json'
  if (options.token) headers.Authorization = `Bearer ${options.token}`
  const response = await fetch(`${hubUrl}${path}`, {
    method,
    headers,
    body: options.json === undefined ? undefined : JSON.stringify(options.json),
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  })
  const raw = await response.text()
  let body: unknown = raw
  try {
    body = raw ? JSON.parse(raw) : null
  } catch {
    // keep the raw text
  }
  return { status: response.status, body: body as T }
}

type Post = (path: string, json: unknown) => Promise<{ status: number; body: unknown }>

/**
 * The shared login sequence: password login, wait for the emailed code (ignoring mails that existed
 * before), then MFA verification. `post` decides how requests are sent (plain fetch or a cookie-keeping
 * Playwright context).
 */
export async function passwordThenMfa(email: string, password: string, post: Post): Promise<string> {
  const existingMailIds = await mailIds(email)
  const since = new Date()
  const login = await post('/api/v1/auth/login', { email, password })
  if (login.status !== 202) throw new LoginRejectedError(login.status, email)
  const mail = await waitForMail(email, 'sign-in code', since, existingMailIds)
  const verify = await post('/api/v1/auth/mfa/verify', {
    mfaToken: (login.body as { mfaToken: string }).mfaToken,
    code: extractCode(mail.text),
  })
  if (verify.status !== 200) throw new Error(`mfa verify for ${email} returned ${verify.status}`)
  return (verify.body as { accessToken: string }).accessToken
}

/** Password login plus the emailed MFA code; returns the access token. */
export function loginWithMfa(email: string, password: string): Promise<string> {
  return passwordThenMfa(email, password, (path, json) => api('POST', path, { json }))
}
