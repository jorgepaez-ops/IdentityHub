import type { APIRequestContext, APIResponse, PlaywrightWorkerArgs } from '@playwright/test'

type Playwright = PlaywrightWorkerArgs['playwright']
import { hubUrl } from './config'
import { LoginRejectedError, paced } from './api'
import { extractCode, mailIds, waitForMail } from './mailpit'

export interface ContextResult<T = unknown> {
  status: number
  body: T
  response: APIResponse
}

export async function contextApi<T = unknown>(
  context: APIRequestContext,
  method: 'GET' | 'POST' | 'DELETE',
  path: string,
  options: { data?: unknown; headers?: Record<string, string> } = {},
): Promise<ContextResult<T>> {
  await paced()
  const response = await context.fetch(path, {
    method,
    data: options.data,
    headers: options.headers,
  })
  const raw = await response.text()
  let body: unknown = raw
  try {
    body = raw ? JSON.parse(raw) : null
  } catch {
    // RFC 7807 is JSON, but retain a raw body if an upstream proxy does not comply.
  }
  return { status: response.status(), body: body as T, response }
}

export async function loginWithMfaContext(
  playwright: Playwright,
  email: string,
  password: string,
): Promise<{ context: APIRequestContext; accessToken: string }> {
  const context = await playwright.request.newContext({ baseURL: hubUrl })
  const existingMailIds = await mailIds(email)
  const since = new Date()
  const login = await contextApi<{ mfaToken: string }>(context, 'POST', '/api/v1/auth/login', {
    data: { email, password },
  })
  if (login.status !== 202) throw new LoginRejectedError(login.status, email)
  const mail = await waitForMail(email, 'sign-in code', since, existingMailIds)
  const verified = await contextApi<{ accessToken: string }>(context, 'POST', '/api/v1/auth/mfa/verify', {
    data: { mfaToken: login.body.mfaToken, code: extractCode(mail.text) },
  })
  if (verified.status !== 200) throw new Error(`MFA verification for ${email} returned ${verified.status}`)
  return { context, accessToken: verified.body.accessToken }
}

export async function refreshCookie(context: APIRequestContext): Promise<string> {
  const state = await context.storageState()
  const cookie = state.cookies.find((item) => item.name === 'refresh_token')
  if (!cookie?.value) throw new Error('refresh_token cookie is absent')
  return cookie.value
}
