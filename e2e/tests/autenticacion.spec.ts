import { expect, test, type PlaywrightWorkerArgs } from '@playwright/test'
import { hubUrl } from '../support/config'
type Playwright = PlaywrightWorkerArgs['playwright']
import { createActiveUser, seedAdmin, strongPassword } from '../support/accounts'
import { api } from '../support/api'
import { contextApi, loginWithMfaContext } from '../support/auth'
import { assertEmail, psql } from '../support/db'
import { extractCode, incorrectCode, mailIds, waitForMail } from '../support/mailpit'

async function activeAccount(playwright: Playwright): Promise<string> {
  const admin = await seedAdmin()
  return createActiveUser(admin)
}

test('RF-003 Todo inicio de sesión exige MFA por correo', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const context = await playwright.request.newContext({ baseURL: hubUrl })
  const existingMailIds = await mailIds(email)
  const since = new Date()
  const login = await contextApi<{ mfaToken: string; accessToken?: string }>(context, 'POST', '/api/v1/auth/login', {
    data: { email, password: strongPassword },
  })
  expect(login.status).toBe(202)
  expect(login.body.mfaToken).toEqual(expect.any(String))
  expect(login.body.accessToken).toBeUndefined()
  expect(login.response.headers()['set-cookie']).toBeUndefined()
  expect((await context.storageState()).cookies.find((cookie) => cookie.name === 'refresh_token')).toBeUndefined()
  const mail = await waitForMail(email, 'sign-in code', since, existingMailIds)
  const verified = await contextApi<{ accessToken: string }>(context, 'POST', '/api/v1/auth/mfa/verify', {
    data: { mfaToken: login.body.mfaToken, code: extractCode(mail.text) },
  })
  expect(verified.status).toBe(200)
  const state = await context.storageState()
  const refresh = state.cookies.find((cookie) => cookie.name === 'refresh_token')
  expect(refresh).toMatchObject({ httpOnly: true, secure: true, sameSite: 'Strict', path: '/api/v1/auth' })
  const header = JSON.parse(Buffer.from(verified.body.accessToken.split('.')[0], 'base64url').toString()) as { alg?: string }
  expect(header.alg).toBe('EdDSA')
  expect(psql(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_user_id WHERE u.email = :'email' AND a.action = 'login_succeeded';`, { email: assertEmail(email) })).toBe('1')
  await context.dispose()
})

test('RF-014 Reenviar el código invalida el anterior', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const context = await playwright.request.newContext()
  const loginMailIds = await mailIds(email)
  const since = new Date()
  const login = await contextApi<{ mfaToken: string }>(context, 'POST', `${hubUrl}/api/v1/auth/login`, { data: { email, password: strongPassword } })
  expect(login.status).toBe(202)
  const first = await waitForMail(email, 'sign-in code', since, loginMailIds)
  psql(`UPDATE mfa_challenges SET last_sent_at = now() - interval '61 seconds' WHERE user_id = (SELECT id FROM users WHERE email = :'email') AND used_at IS NULL;`, { email: assertEmail(email) })
  const resendMailIds = await mailIds(email)
  const resendSince = new Date()
  const resent = await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/resend`, { data: { mfaToken: login.body.mfaToken } })
  expect(resent.status).toBe(202)
  const second = await waitForMail(email, 'sign-in code', resendSince, resendMailIds)
  expect(extractCode(second.text)).not.toBe(extractCode(first.text))
  const oldCode = await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: login.body.mfaToken, code: extractCode(first.text) } })
  expect(oldCode.status).toBe(401)
  await context.dispose()
})

test('RF-014 El reenvío respeta la ventana mínima', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const context = await playwright.request.newContext()
  const login = await contextApi<{ mfaToken: string }>(context, 'POST', `${hubUrl}/api/v1/auth/login`, { data: { email, password: strongPassword } })
  expect(login.status).toBe(202)
  const resent = await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/resend`, { data: { mfaToken: login.body.mfaToken } })
  expect(resent.status).toBe(429)
  await context.dispose()
})

test('RF-014 Agotar los intentos MFA anula el desafío', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const context = await playwright.request.newContext()
  const existingMailIds = await mailIds(email)
  const since = new Date()
  const login = await contextApi<{ mfaToken: string }>(context, 'POST', `${hubUrl}/api/v1/auth/login`, { data: { email, password: strongPassword } })
  expect(login.status).toBe(202)
  const correctCode = extractCode((await waitForMail(email, 'sign-in code', since, existingMailIds)).text)
  expect(psql(`SELECT attempts_left FROM mfa_challenges
               WHERE user_id = (SELECT id FROM users WHERE email = :'email')
                 AND attempts_left > 0 AND used_at IS NULL;`, { email: assertEmail(email) })).toBe('5')
  // Challenge and account limits are both five, so reducing this isolates challenge exhaustion without locking the account.
  psql(`UPDATE mfa_challenges
        SET attempts_left = 1
        WHERE user_id = (SELECT id FROM users WHERE email = :'email')
          AND attempts_left > 0
          AND used_at IS NULL;`, { email: assertEmail(email) })
  const wrongCode = incorrectCode(correctCode)
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: login.body.mfaToken, code: wrongCode } })).status).toBe(401)
  // The exhausted challenge must reject even the correct code: the mfaToken is no longer valid.
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: login.body.mfaToken, code: correctCode } })).status).toBe(401)
  expect(psql(`SELECT status FROM users WHERE email = :'email';`, { email: assertEmail(email) })).toBe('active')
  expect(psql(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_user_id WHERE u.email = :'email' AND a.action = 'mfa_code_rejected';`, { email: assertEmail(email) })).toBe('1')
  expect(psql(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_user_id WHERE u.email = :'email' AND a.action = 'mfa_challenge_exhausted';`, { email: assertEmail(email) })).toBe('1')
  await context.dispose()
})

test('RF-014 Adivinar códigos MFA en desafíos sucesivos bloquea la cuenta', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const context = await playwright.request.newContext()
  const firstMailIds = await mailIds(email)
  const firstSince = new Date()
  const first = await contextApi<{ mfaToken: string }>(context, 'POST', `${hubUrl}/api/v1/auth/login`, { data: { email, password: strongPassword } })
  expect(first.status).toBe(202)
  const firstWrongCode = incorrectCode(extractCode((await waitForMail(email, 'sign-in code', firstSince, firstMailIds)).text))
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: first.body.mfaToken, code: firstWrongCode } })).status).toBe(401)
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: first.body.mfaToken, code: firstWrongCode } })).status).toBe(401)
  const secondMailIds = await mailIds(email)
  const secondSince = new Date()
  const second = await contextApi<{ mfaToken: string }>(context, 'POST', `${hubUrl}/api/v1/auth/login`, { data: { email, password: strongPassword } })
  expect(second.status).toBe(202)
  const secondWrongCode = incorrectCode(extractCode((await waitForMail(email, 'sign-in code', secondSince, secondMailIds)).text))
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: second.body.mfaToken, code: secondWrongCode } })).status).toBe(401)
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: second.body.mfaToken, code: secondWrongCode } })).status).toBe(401)
  const thirdMailIds = await mailIds(email)
  const since = new Date()
  const third = await contextApi<{ mfaToken: string }>(context, 'POST', `${hubUrl}/api/v1/auth/login`, { data: { email, password: strongPassword } })
  expect(third.status).toBe(202)
  const correctCode = extractCode((await waitForMail(email, 'sign-in code', since, thirdMailIds)).text)
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: third.body.mfaToken, code: incorrectCode(correctCode) } })).status).toBe(401)
  expect(psql(`SELECT status FROM users WHERE email = :'email';`, { email: assertEmail(email) })).toBe('locked')
  expect((await contextApi(context, 'POST', `${hubUrl}/api/v1/auth/mfa/verify`, { data: { mfaToken: third.body.mfaToken, code: correctCode } })).status).toBe(401)
  expect(psql(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_user_id WHERE u.email = :'email' AND a.action = 'account_locked';`, { email: assertEmail(email) })).toBe('1')
  await context.dispose()
})

test('RF-003 Contraseña incorrecta', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const context = await playwright.request.newContext()
  const result = await contextApi<{ type?: string; title?: string; status?: number; detail?: string }>(context, 'POST', `${hubUrl}/api/v1/auth/login`, {
    data: { email, password: 'una-contraseña-cualquiera' },
  })
  expect(result.status).toBe(401)
  expect(result.response.headers()['content-type']).toBe('application/problem+json; charset=utf-8')
  expect(result.body).toEqual({
    type: 'https://identity.local/problems/invalid-credentials',
    title: 'Unauthorized',
    status: 401,
    detail: 'Invalid email or password.',
  })
  expect(psql(`SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_user_id WHERE u.email = :'email' AND a.action = 'login_failed';`, { email: assertEmail(email) })).toBe('1')
  await context.dispose()
})

test('RF-004 Un token con el algoritmo alterado se rechaza', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const session = await loginWithMfaContext(playwright, email, strongPassword)
  const [header, payload, signature] = session.accessToken.split('.')
  const altered = `${Buffer.from(JSON.stringify({ ...JSON.parse(Buffer.from(header, 'base64url').toString()), alg: 'none' })).toString('base64url')}.${payload}.${signature}`
  const result = await api('GET', '/api/v1/me', { token: altered })
  expect(result.status).toBe(401)
  await session.context.dispose()
})

test('RF-017 Bloqueo tras intentos fallidos repetidos', async ({ playwright }) => {
  const email = await activeAccount(playwright)
  const existingMailIds = await mailIds(email)
  const since = new Date()
  for (let attempt = 0; attempt < 5; attempt += 1) {
    expect((await api('POST', '/api/v1/auth/login', { json: { email, password: 'una-contraseña-cualquiera' } })).status).toBe(401)
  }
  expect((await api('POST', '/api/v1/auth/login', { json: { email, password: strongPassword } })).status).toBe(423)
  await expect.poll(() => psql(`SELECT status FROM users WHERE email = :'email';`, { email: assertEmail(email) })).toBe('locked')
  await waitForMail(email, 'account temporarily locked', since, existingMailIds)
})
