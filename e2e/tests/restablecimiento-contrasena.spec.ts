import { expect, test } from '@playwright/test'
import { createActiveUser, seedAdmin, strongPassword, uniqueEmail } from '../support/accounts'
import { api } from '../support/api'
import { contextApi, loginWithMfaContext } from '../support/auth'
import { assertEmail, psql } from '../support/db'
import { extractLink, mailIds, waitForMail } from '../support/mailpit'

const replacementPassword = 'new-correct-horse-battery'

async function resetLink(email: string): Promise<string> {
  const existingMailIds = await mailIds(email)
  const since = new Date()
  expect((await api('POST', '/api/v1/auth/password-reset/request', { json: { email } })).status).toBe(202)
  return extractLink((await waitForMail(email, 'password', since, existingMailIds)).text)
}

test('RF-015 La solicitud no enumera cuentas', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  const existing = await api('POST', '/api/v1/auth/password-reset/request', { json: { email } })
  const missing = await api('POST', '/api/v1/auth/password-reset/request', { json: { email: uniqueEmail('missing') } })
  expect(existing.status).toBe(202)
  expect(missing.status).toBe(202)
  expect(existing.body).toEqual(missing.body)
})

test('RF-015 Restablecer revoca las sesiones anteriores', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  const first = await loginWithMfaContext(playwright, email, strongPassword)
  const second = await loginWithMfaContext(playwright, email, strongPassword)
  const link = await resetLink(email)
  const token = new URL(link).searchParams.get('token')
  expect(token).toBeTruthy()
  const confirmed = await api('POST', '/api/v1/auth/password-reset/confirm', { json: { token, password: replacementPassword } })
  expect(confirmed.status).toBe(204)
  expect((await contextApi(first.context, 'POST', '/api/v1/auth/refresh')).status).toBe(401)
  expect((await contextApi(second.context, 'POST', '/api/v1/auth/refresh')).status).toBe(401)
  expect((await api('POST', '/api/v1/auth/password-reset/confirm', { json: { token, password: replacementPassword } })).status).toBe(410)
  await first.context.dispose()
  await second.context.dispose()
})

test('RF-015 Restablecer desbloquea la cuenta sin que un fallo la vuelva a bloquear', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  psql(`INSERT INTO audit_log (actor_user_id, action, metadata)
        SELECT id, 'login_failed', '{}'::jsonb FROM users WHERE email = :'email';
        INSERT INTO audit_log (actor_user_id, action, metadata)
        SELECT id, 'login_failed', '{}'::jsonb FROM users WHERE email = :'email';
        INSERT INTO audit_log (actor_user_id, action, metadata)
        SELECT id, 'login_failed', '{}'::jsonb FROM users WHERE email = :'email';
        INSERT INTO audit_log (actor_user_id, action, metadata)
        SELECT id, 'login_failed', '{}'::jsonb FROM users WHERE email = :'email';
        INSERT INTO audit_log (actor_user_id, action, metadata)
        SELECT id, 'login_failed', '{}'::jsonb FROM users WHERE email = :'email';`, { email: assertEmail(email) })
  psql(`UPDATE users SET status = 'locked', locked_until = now() + interval '30 minutes' WHERE email = :'email';`, { email: assertEmail(email) })
  const link = await resetLink(email)
  const token = new URL(link).searchParams.get('token')
  expect((await api('POST', '/api/v1/auth/password-reset/confirm', { json: { token, password: replacementPassword } })).status).toBe(204)
  expect(psql(`SELECT status FROM users WHERE email = :'email';`, { email: assertEmail(email) })).toBe('active')
  const session = await loginWithMfaContext(playwright, email, replacementPassword)
  expect((await api('POST', '/api/v1/auth/login', { json: { email, password: 'incorrect-password-value' } })).status).toBe(401)
  expect(psql(`SELECT status FROM users WHERE email = :'email';`, { email: assertEmail(email) })).toBe('active')
  await session.context.dispose()
})
