import { expect, test, type PlaywrightWorkerArgs } from '@playwright/test'
import { hubUrl } from '../support/config'
type Playwright = PlaywrightWorkerArgs['playwright']
import { createActiveUser, seedAdmin, strongPassword } from '../support/accounts'
import { contextApi, loginWithMfaContext, refreshCookie } from '../support/auth'
import { assertEmail, psql } from '../support/db'
import { mailIds, waitForMail } from '../support/mailpit'

async function signedIn(playwright: Playwright) {
  const email = await createActiveUser(await seedAdmin())
  return { email, ...(await loginWithMfaContext(playwright, email, strongPassword)) }
}

test('RF-005 La renovación rota la cookie refresh', async ({ playwright }) => {
  const session = await signedIn(playwright)
  const oldCookie = await refreshCookie(session.context)
  const renewed = await contextApi<{ accessToken: string }>(session.context, 'POST', '/api/v1/auth/refresh')
  expect(renewed.status).toBe(200)
  expect(renewed.body.accessToken).not.toBe(session.accessToken)
  expect(await refreshCookie(session.context)).not.toBe(oldCookie)
  const replay = await playwright.request.newContext({ extraHTTPHeaders: { Cookie: `refresh_token=${oldCookie}` } })
  expect((await contextApi(replay, 'POST', `${hubUrl}/api/v1/auth/refresh`)).status).toBe(401)
  await replay.dispose()
  await session.context.dispose()
})

test('RF-006 Reutilizar un refresh token rotado revoca toda la familia', async ({ playwright }) => {
  const session = await signedIn(playwright)
  const oldCookie = await refreshCookie(session.context)
  expect((await contextApi(session.context, 'POST', '/api/v1/auth/refresh')).status).toBe(200)
  const existingMailIds = await mailIds(session.email)
  const since = new Date()
  const thief = await playwright.request.newContext({ extraHTTPHeaders: { Cookie: `refresh_token=${oldCookie}` } })
  expect((await contextApi(thief, 'POST', `${hubUrl}/api/v1/auth/refresh`)).status).toBe(401)
  expect((await contextApi(session.context, 'POST', '/api/v1/auth/refresh')).status).toBe(401)
  expect(psql(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_user_id WHERE u.email = :'email' AND a.action = 'refresh_reuse_detected';`, { email: assertEmail(session.email) })).toBe('1')
  await waitForMail(session.email, 'refresh token reuse detected', since, existingMailIds)
  await thief.dispose()
  await session.context.dispose()
})

test('RF-007 Cierre de sesión', async ({ playwright }) => {
  const session = await signedIn(playwright)
  const oldCookie = await refreshCookie(session.context)
  const logout = await contextApi(session.context, 'POST', '/api/v1/auth/logout')
  expect(logout.status).toBe(204)
  const clearedRefresh = (await logout.response.headersArray()).find((header) => header.name.toLowerCase() === 'set-cookie' && header.value.startsWith('refresh_token='))
  expect(clearedRefresh?.value).toContain('Path=/api/v1/auth')
  expect(clearedRefresh?.value).toContain('Max-Age=0')
  expect(clearedRefresh?.value).toContain('HttpOnly')
  expect(clearedRefresh?.value).toContain('Secure')
  expect(clearedRefresh?.value).toContain('SameSite=Strict')
  expect((await session.context.storageState()).cookies.find((cookie) => cookie.name === 'refresh_token')).toBeUndefined()
  const replay = await playwright.request.newContext({ extraHTTPHeaders: { Cookie: `refresh_token=${oldCookie}` } })
  expect((await contextApi(replay, 'POST', `${hubUrl}/api/v1/auth/refresh`)).status).toBe(401)
  await replay.dispose()
  await session.context.dispose()
})

test('RF-016 Revocar una sesión concreta desde otro dispositivo', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  const first = await loginWithMfaContext(playwright, email, strongPassword)
  const second = await loginWithMfaContext(playwright, email, strongPassword)
  const sessions = await contextApi<Array<{ id: string; current: boolean }>>(second.context, 'GET', '/api/v1/me/sessions', { headers: { Authorization: `Bearer ${second.accessToken}` } })
  expect(sessions.status).toBe(200)
  const firstSession = sessions.body.find((item) => !item.current)
  expect(firstSession).toBeDefined()
  expect((await contextApi(second.context, 'DELETE', `/api/v1/me/sessions/${firstSession?.id}`, { headers: { Authorization: `Bearer ${second.accessToken}` } })).status).toBe(204)
  expect((await contextApi(first.context, 'POST', '/api/v1/auth/refresh')).status).toBe(401)
  expect((await contextApi(second.context, 'POST', '/api/v1/auth/refresh')).status).toBe(200)
  await first.context.dispose()
  await second.context.dispose()
})
