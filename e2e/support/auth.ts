import type { APIRequestContext, APIResponse, PlaywrightWorkerArgs } from '@playwright/test'

type Playwright = PlaywrightWorkerArgs['playwright']
import { hubUrl } from './config'
import { paced, passwordThenMfa } from './api'

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
  const accessToken = await passwordThenMfa(email, password, (path, data) =>
    contextApi(context, 'POST', path, { data }),
  )
  return { context, accessToken }
}

export async function refreshCookie(context: APIRequestContext): Promise<string> {
  const state = await context.storageState()
  const cookie = state.cookies.find((item) => item.name === 'refresh_token')
  if (!cookie?.value) throw new Error('refresh_token cookie is absent')
  return cookie.value
}
