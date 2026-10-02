import { createHash, randomBytes } from 'node:crypto'
import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import { createActiveUser, seedAdmin, strongPassword } from '../support/accounts'
import { paced } from '../support/api'
import { contextApi, loginWithMfaContext } from '../support/auth'
import { contabilidadUrl, hubUrl } from '../support/config'
import { assertEmail, psql } from '../support/db'
import { extractCode, mailIds, waitForMail } from '../support/mailpit'

const clientID = 'contabilidad'
const redirectURI = `${contabilidadUrl}/oauth/callback`

type OAuthToken = { access_token: string; token_type: string; expires_in: number; refresh_token?: string }
type Claims = { aud?: string | string[]; roles?: string[] }

function newVerifier(): string {
  return randomBytes(32).toString('base64url')
}

function s256Challenge(verifier: string): string {
  return createHash('sha256').update(verifier).digest('base64url')
}

function authorizationPath(input: { state?: string; verifier: string; redirectUri?: string; method?: string }): string {
  const query = new URLSearchParams({
    client_id: clientID,
    redirect_uri: input.redirectUri ?? redirectURI,
    response_type: 'code',
    code_challenge: s256Challenge(input.verifier),
    code_challenge_method: input.method ?? 'S256',
  })
  if (input.state !== undefined) query.set('state', input.state)
  return `/oauth/authorize?${query.toString()}`
}

async function authorize(context: APIRequestContext, input: { state?: string; verifier: string; redirectUri?: string; method?: string }) {
  await paced()
  return context.get(authorizationPath(input), { maxRedirects: 0 })
}

async function exchange(context: APIRequestContext, code: string, verifier: string) {
  await paced()
  const response = await context.post('/oauth/token', {
    form: {
      grant_type: 'authorization_code',
      code,
      client_id: clientID,
      redirect_uri: redirectURI,
      code_verifier: verifier,
    },
  })
  const body = (await response.json()) as OAuthToken
  return { response, body }
}

function codeFromRedirect(responseLocation: string): string {
  const code = new URL(responseLocation).searchParams.get('code')
  if (!code) throw new Error('OAuth authorize redirect has no authorization code')
  return code
}

function decodeClaims(accessToken: string): Claims {
  const payload = accessToken.split('.')[1]
  if (!payload) throw new Error('OAuth access token has no payload')
  return JSON.parse(Buffer.from(payload, 'base64url').toString()) as Claims
}

async function contextSession(playwright: Parameters<typeof loginWithMfaContext>[0], roles: string[] = ['user']) {
  const admin = await seedAdmin()
  const email = await createActiveUser(admin, roles)
  return { email, ...(await loginWithMfaContext(playwright, email, strongPassword)) }
}

async function completeBrowserMfa(email: string, existingMailIds: ReadonlySet<string>, since: Date, page: Page) {
  await page.locator('#email').fill(email)
  await page.locator('#password').fill(strongPassword)
  await page.getByRole('button', { name: 'Iniciar sesión' }).click()
  await expect(page.locator('#mfa-code')).toBeVisible()
  const code = extractCode((await waitForMail(email, 'sign-in code', since, existingMailIds)).text)
  await page.locator('#mfa-code').fill(code)
  await page.getByRole('button', { name: 'Verificar' }).click()
}

function expectSafeCallbackAddress(address: string): void {
  const url = new URL(address)
  expect(url.searchParams.get('code')).toBeNull()
  expect(url.searchParams.get('state')).toBeNull()
  expect(address).not.toMatch(/(?:access|refresh|id)_?token=/)
}

async function expectNoClientStorage(page: Page): Promise<void> {
  await expect.poll(() => page.evaluate(() => ({ session: sessionStorage.length, local: localStorage.length }))).toEqual({ session: 0, local: 0 })
}

test('RF-020 Flujo OAuth correcto con PKCE S256', async ({ page }) => {
  const admin = await seedAdmin()
  const email = await createActiveUser(admin, ['contabilidad.analista'])
  const existingMailIds = await mailIds(email)
  const since = new Date()

  await page.goto(contabilidadUrl)
  const authorizeRequest = page.waitForRequest((request) => request.url().startsWith(`${hubUrl}/oauth/authorize?`))
  await page.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
  const requestedState = new URL((await authorizeRequest).url()).searchParams.get('state')
  expect(requestedState).toBeTruthy()
  const authorizeRedirect = page.waitForResponse((response) => response.url().startsWith(`${hubUrl}/oauth/authorize?`) && response.status() === 302)
  const tokenResponse = page.waitForResponse((response) => response.url() === `${hubUrl}/oauth/token` && response.request().method() === 'POST')
  await completeBrowserMfa(email, existingMailIds, since, page)

  const redirectLocation = (await authorizeRedirect).headers().location
  expect(redirectLocation).toBeTruthy()
  const redirect = new URL(redirectLocation)
  expect(redirect.origin + redirect.pathname).toBe(redirectURI)
  expect(redirect.searchParams.get('code')).toBeTruthy()
  expect(redirect.searchParams.get('state')).toBe(requestedState)
  const token = (await tokenResponse).json() as Promise<OAuthToken>
  const tokenBody = await token
  expect(tokenBody).toMatchObject({ token_type: 'Bearer', expires_in: expect.any(Number) })
  expect(tokenBody.access_token).toEqual(expect.any(String))
  expect(tokenBody.refresh_token).toBeUndefined()
  expect([decodeClaims(tokenBody.access_token).aud].flat()).toEqual([clientID])

  await expect(page.getByText('Analista contable', { exact: true })).toBeVisible()
  await expect(page).toHaveURL(`${contabilidadUrl}/`)
  expectSafeCallbackAddress(page.url())
  await expectNoClientStorage(page)
})

test('RF-020 Un segundo acceso usa la sesión SSO del Hub', async ({ page }) => {
  const admin = await seedAdmin()
  const email = await createActiveUser(admin, ['contabilidad.analista'])
  const initialMailIds = await mailIds(email)
  const since = new Date()

  await page.goto(contabilidadUrl)
  await page.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
  await completeBrowserMfa(email, initialMailIds, since, page)
  await expect(page.getByText('Analista contable', { exact: true })).toBeVisible()
  const mailAfterMfa = await mailIds(email)

  await page.goto(contabilidadUrl)
  await page.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
  await expect(page.getByText('Analista contable', { exact: true })).toBeVisible()
  await expect(page.locator('#password')).toHaveCount(0)
  expect([...await mailIds(email)].sort()).toEqual([...mailAfterMfa].sort())
  expectSafeCallbackAddress(page.url())
  await expectNoClientStorage(page)
})

test('RF-020 Una URI de retorno distinta se rechaza', async ({ playwright }) => {
  const session = await contextSession(playwright)
  const verifier = newVerifier()
  const response = await authorize(session.context, { verifier, state: 'redirect-mismatch', redirectUri: `${redirectURI}/other` })

  expect(response.status()).toBe(400)
  expect(response.headers().location).toBeUndefined()
  expect(await response.json()).toMatchObject({ status: 400 })
  await session.context.dispose()
})

test('RF-020 El estado es obligatorio', async ({ playwright }) => {
  const session = await contextSession(playwright)
  const response = await authorize(session.context, { verifier: newVerifier() })

  expect(response.status()).toBe(302)
  const redirect = new URL(response.headers().location ?? '')
  expect(redirect.origin + redirect.pathname).toBe(redirectURI)
  expect(redirect.searchParams.get('error')).toBe('invalid_request')
  expect(redirect.searchParams.get('state')).toBeNull()
  expect(redirect.searchParams.get('code')).toBeNull()
  await session.context.dispose()
})

test('RF-020 PKCE plain se rechaza', async ({ playwright }) => {
  const session = await contextSession(playwright)
  const state = 'plain-is-rejected'
  const response = await authorize(session.context, { verifier: newVerifier(), state, method: 'plain' })

  expect(response.status()).toBe(302)
  const redirect = new URL(response.headers().location ?? '')
  expect(redirect.origin + redirect.pathname).toBe(redirectURI)
  expect(redirect.searchParams.get('error')).toBe('invalid_request')
  expect(redirect.searchParams.get('state')).toBe(state)
  expect(redirect.searchParams.get('code')).toBeNull()
  await session.context.dispose()
})

test('RF-020 Un código de autorización no puede reutilizarse', async ({ playwright }) => {
  const session = await contextSession(playwright)
  const verifier = newVerifier()
  const authorized = await authorize(session.context, { verifier, state: 'single-use' })
  expect(authorized.status()).toBe(302)
  const code = codeFromRedirect(authorized.headers().location ?? '')

  const first = await exchange(session.context, code, verifier)
  expect(first.response.status()).toBe(200)
  const reused = await exchange(session.context, code, verifier)
  expect(reused.response.status()).toBe(400)
  expect(reused.body).toMatchObject({ status: 400 })
  expect(psql(`SELECT count(*) FROM audit_log a
               JOIN users u ON u.id = a.actor_user_id
               WHERE u.email = :'email' AND a.action = 'authorization_code_reused';`, { email: assertEmail(session.email) })).toBe('1')
  await session.context.dispose()
})

test('RF-009 El token contiene solo los roles de Contabilidad', async ({ playwright }) => {
  const session = await contextSession(playwright, ['admin', 'contabilidad.senior'])
  const user = await contextApi<{ roles: string[] }>(session.context, 'GET', '/api/v1/me', {
    headers: { Authorization: `Bearer ${session.accessToken}` },
  })
  expect(user.status).toBe(200)
  expect([...user.body.roles].sort()).toEqual(['admin', 'contabilidad.senior', 'user'])
  const verifier = newVerifier()
  const authorized = await authorize(session.context, { verifier, state: 'app-roles-only' })
  expect(authorized.status()).toBe(302)

  const token = await exchange(session.context, codeFromRedirect(authorized.headers().location ?? ''), verifier)
  expect(token.response.status()).toBe(200)
  expect(decodeClaims(token.body.access_token).roles).toEqual(['contabilidad.senior'])
  expect([decodeClaims(token.body.access_token).aud].flat()).toEqual([clientID])
  await session.context.dispose()
})

test('RF-020 Sin sesión en el Hub se pasa por el login y se vuelve a la autorización', async ({ page }) => {
  const admin = await seedAdmin()
  const email = await createActiveUser(admin, ['contabilidad.analista'])
  const existingMailIds = await mailIds(email)
  const since = new Date()

  await page.goto(contabilidadUrl)
  await page.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
  await expect(page).toHaveURL(new RegExp(`^${hubUrl.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/login\\?`))
  const continueTarget = new URL(page.url()).searchParams.get('continue')
  expect(continueTarget).toMatch(/^\/oauth\/authorize\?/)
  const continueParams = new URL(continueTarget ?? '', hubUrl).searchParams
  expect(continueParams.get('client_id')).toBe(clientID)
  expect(continueParams.get('redirect_uri')).toBe(redirectURI)
  expect(continueParams.get('state')).toBeTruthy()

  await completeBrowserMfa(email, existingMailIds, since, page)
  await expect(page.getByText('Analista contable', { exact: true })).toBeVisible()
  await expect(page).toHaveURL(`${contabilidadUrl}/`)
  expectSafeCallbackAddress(page.url())
  await expectNoClientStorage(page)
})

test('RF-020 Un "continue" absoluto o externo no provoca una redirección abierta', async ({ page }) => {
  const admin = await seedAdmin()
  const email = await createActiveUser(admin)
  const existingMailIds = await mailIds(email)
  const since = new Date()
  const external = 'https://sitio-malicioso.example/robo'

  await page.goto(`${hubUrl}/login?continue=${encodeURIComponent(external)}`)
  await completeBrowserMfa(email, existingMailIds, since, page)
  await expect(page.getByRole('heading', { name: 'Mi cuenta' })).toBeVisible()
  expect(page.url()).toMatch(new RegExp(`^${hubUrl.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`))
  expect(page.url()).not.toContain(external)
  expectSafeCallbackAddress(page.url())
})
