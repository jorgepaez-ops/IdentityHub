import { hubUrl } from './config'
import { extractCode, waitForMail } from './mailpit'

// Nginx allows 5 r/s (burst 5) on /api/v1/auth/. Keep calls at least 300 ms apart.
const PACE_MS = 300
let lastCall = 0

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

export interface ApiResult<T = unknown> {
  status: number
  body: T
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

/** Password login plus the emailed MFA code; returns the access token. */
export async function loginWithMfa(email: string, password: string): Promise<string> {
  const since = new Date()
  const login = await api<{ mfaToken: string }>('POST', '/api/v1/auth/login', { json: { email, password } })
  if (login.status !== 202) throw new Error(`login for ${email} returned ${login.status}`)
  const mail = await waitForMail(email, 'sign-in code', since)
  const verify = await api<{ accessToken: string }>('POST', '/api/v1/auth/mfa/verify', {
    json: { mfaToken: login.body.mfaToken, code: extractCode(mail.text) },
  })
  if (verify.status !== 200) throw new Error(`mfa verify for ${email} returned ${verify.status}`)
  return verify.body.accessToken
}
