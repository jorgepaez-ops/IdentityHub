import { expect, test, type Browser, type Page } from '@playwright/test'
import { seedAdmin, seedCompanyUser, strongPassword } from '../support/accounts'
import { api, paced } from '../support/api'
import { browserLoginWithMfa } from '../support/browser'
import { loginWithMfaContext } from '../support/auth'
import { contabilidadUrl, hubUrl } from '../support/config'
import { psql } from '../support/db'
import { mailIds } from '../support/mailpit'
import { organizations } from '../support/organizations'
import { ownId } from '../support/roles'

type Claims = { aud?: string | string[]; roles?: string[] }

function decodeClaims(accessToken: string): Claims {
  const payload = accessToken.split('.')[1]
  if (!payload) throw new Error('OAuth access token has no payload')
  return JSON.parse(Buffer.from(payload, 'base64url').toString()) as Claims
}

/** A global administrator signed in to the console in a browser of its own. */
async function consoleAsAdmin(browser: Browser): Promise<{ page: Page; close: () => Promise<void>; email: string; accessToken: string }> {
  const admin = await seedAdmin()
  const context = await browser.newContext()
  const page = await context.newPage()
  await page.goto(`${hubUrl}/login`)
  await browserLoginWithMfa(page, admin.email, strongPassword)
  await expect(page.getByRole('heading', { name: 'Inicio' })).toBeVisible()
  return { page, close: () => context.close(), email: admin.email, accessToken: admin.accessToken }
}

const chooseCompany = (page: Page, name: string) => page.getByLabel('Empresa', { exact: true }).selectOption({ label: name })

/** Emails of one company's accounts according to the database. */
function emailsOf(organizationId: string): Set<string> {
  const rows = psql(`SELECT email FROM users WHERE organization_id = :'organization'::uuid;`, { organization: organizationId })
  return new Set(rows.split('\n').filter(Boolean))
}

/** Emails shown by the directory table right now. */
const listedEmails = (page: Page) =>
  page.locator('table[aria-label="Directorio"] tbody tr td:first-child .muted').allTextContents()

test('RF-022 La grilla de roles muestra solo las aplicaciones y roles de la empresa seleccionada', async ({ browser }) => {
  const hubConsole = await consoleAsAdmin(browser)
  const { page } = hubConsole
  try {
    await page.getByRole('link', { name: 'Roles', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Roles y permisos' })).toBeVisible()

    await chooseCompany(page, 'Empresa B')

    await expect(page.getByRole('table', { name: 'Permisos de roles de Inventario' })).toBeVisible()
    await expect(page.getByRole('rowheader', { name: 'inventario.consulta', exact: true })).toBeVisible()
    await expect(page.getByRole('rowheader', { name: 'inventario.bodeguero', exact: true })).toBeVisible()
    await expect(page.getByRole('table', { name: 'Permisos de roles de Contabilidad' })).toHaveCount(0)
    await expect(page.getByText(/contabilidad\./)).toHaveCount(0)
    // Creating a role only offers the company on screen.
    await expect(page.getByRole('button', { name: /^Nuevo rol de / })).toHaveText(['Nuevo rol de Inventario'])

    await chooseCompany(page, 'Contabilidad S.A.')

    await expect(page.getByRole('table', { name: 'Permisos de roles de Contabilidad' })).toBeVisible()
    await expect(page.getByRole('rowheader', { name: 'contabilidad.senior', exact: true })).toBeVisible()
    await expect(page.getByRole('table', { name: 'Permisos de roles de Inventario' })).toHaveCount(0)
    await expect(page.getByText(/inventario\./)).toHaveCount(0)
  } finally {
    await hubConsole.close()
  }
})

test('RF-022 El listado de usuarios muestra solo los usuarios de la empresa seleccionada', async ({ browser }) => {
  const conta = await seedCompanyUser(organizations.contabilidad, ['contabilidad.analista'])
  const empresaB = await seedCompanyUser(organizations.empresaB, ['inventario.consulta'])
  const hubConsole = await consoleAsAdmin(browser)
  const { page } = hubConsole
  try {
    await page.getByRole('link', { name: 'Usuarios', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Usuarios' })).toBeVisible()

    const search = page.getByLabel('Buscar por correo o nombre')
    const companyB = emailsOf(organizations.empresaB.id)

    await chooseCompany(page, 'Empresa B')

    // The directory pages by 100 and E2E runs pile up accounts, so each check searches for its own account.
    await search.fill(empresaB.email)
    await expect(page.getByRole('row', { name: new RegExp(empresaB.email) })).toBeVisible()
    expect((await listedEmails(page)).filter((email) => !companyB.has(email))).toEqual([])
    await search.fill(conta.email)
    await expect(page.getByText('No hay usuarios que coincidan con la búsqueda.')).toBeVisible()
    expect(await listedEmails(page)).not.toContain(conta.email)

    await chooseCompany(page, 'Contabilidad S.A.')

    await search.fill(conta.email)
    await expect(page.getByRole('row', { name: new RegExp(conta.email) })).toBeVisible()
    await search.fill(empresaB.email)
    await expect(page.getByText('No hay usuarios que coincidan con la búsqueda.')).toBeVisible()
  } finally {
    await hubConsole.close()
  }
})

test('RF-022 Al asignar un rol solo se ofrecen roles de la empresa del usuario', async ({ browser }) => {
  const empresaB = await seedCompanyUser(organizations.empresaB, ['inventario.consulta'])
  const hubConsole = await consoleAsAdmin(browser)
  const { page } = hubConsole
  try {
    await page.getByRole('link', { name: 'Usuarios', exact: true }).click()
    await chooseCompany(page, 'Empresa B')
    await page.getByLabel('Buscar por correo o nombre').fill(empresaB.email)
    await page.getByRole('row', { name: new RegExp(empresaB.email) }).getByRole('button', { name: /^Editar a / }).click()

    const drawer = page.getByRole('dialog', { name: 'Editar usuario' })
    await expect(drawer.getByLabel('inventario.bodeguero', { exact: true })).toBeVisible()
    const offered = await drawer.getByRole('checkbox').evaluateAll((boxes) => boxes.map((box) => (box as HTMLInputElement).value))
    expect(offered).toEqual(expect.arrayContaining(['inventario.consulta', 'inventario.bodeguero']))
    expect(offered.filter((role) => role.startsWith('contabilidad.'))).toEqual([])
  } finally {
    await hubConsole.close()
  }
})

test('RF-022 El backend rechaza asignar un rol de otra empresa', async () => {
  const admin = await seedAdmin()
  const adminId = await ownId(admin.accessToken)
  const empresaB = await seedCompanyUser(organizations.empresaB, ['inventario.consulta'])
  const rolesOf = () => psql(`SELECT string_agg(r.name, ',' ORDER BY r.name) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = :'id'::uuid;`, { id: empresaB.id })
  const before = rolesOf()

  const assigned = await api('PATCH', `/api/v1/admin/users/${empresaB.id}`, {
    token: admin.accessToken,
    json: { roles: ['user', 'contabilidad.analista'] },
  })

  expect(assigned.status).toBe(400)
  expect(rolesOf()).toBe(before)
  expect(before).toBe('inventario.consulta,user')
  expect(psql(`SELECT count(*) FROM audit_log
               WHERE action = 'role_assignment_rejected' AND actor_user_id = :'actor'::uuid
                 AND resource_id = :'target' AND metadata->>'reason' = 'organization_mismatch';`, { actor: adminId, target: empresaB.id })).toBe('1')
})

test('RF-022 Un usuario de Contabilidad S.A. entra a Contabilidad', async ({ page }) => {
  const employee = await seedCompanyUser(organizations.contabilidad, ['contabilidad.analista'])
  const existingMailIds = await mailIds(employee.email)
  const since = new Date()

  await page.goto(contabilidadUrl)
  const authorizeRequest = page.waitForRequest((request) => request.url().startsWith(`${hubUrl}/oauth/authorize?`))
  await page.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
  const requestedState = new URL((await authorizeRequest).url()).searchParams.get('state')
  const authorizeRedirect = page.waitForResponse((response) => response.url().startsWith(`${hubUrl}/oauth/authorize?`) &&
    response.status() === 302 &&
    (response.headers().location ?? '').startsWith(`${contabilidadUrl}/oauth/callback?`))
  const tokenResponse = page.waitForResponse((response) => response.url() === `${hubUrl}/oauth/token` && response.request().method() === 'POST')
  await browserLoginWithMfa(page, employee.email, strongPassword, { existingMailIds, since })

  const redirect = new URL((await authorizeRedirect).headers().location)
  expect(redirect.searchParams.get('code')).toBeTruthy()
  expect(redirect.searchParams.get('state')).toBe(requestedState)
  const token = (await (await tokenResponse).json()) as { access_token: string }
  expect([decodeClaims(token.access_token).aud].flat()).toEqual(['contabilidad'])
  await expect(page.getByText('Analista contable', { exact: true })).toBeVisible()
})

test('RF-022 Un usuario de Empresa B que abre Contabilidad es rechazado', async ({ page }) => {
  const employee = await seedCompanyUser(organizations.empresaB, ['inventario.consulta'])
  const existingMailIds = await mailIds(employee.email)
  const since = new Date()
  const authorizeRedirects: string[] = []
  const tokenRequests: string[] = []
  page.on('response', (response) => {
    if (new URL(response.url()).pathname === '/oauth/authorize' && response.status() === 302) {
      authorizeRedirects.push(response.headers().location ?? '')
    }
  })
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/oauth/token') tokenRequests.push(request.url())
  })

  await page.goto(contabilidadUrl)
  await page.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
  await browserLoginWithMfa(page, employee.email, strongPassword, { existingMailIds, since })

  await expect(page.getByRole('heading', { name: 'Sin acceso a Contabilidad', exact: true })).toBeVisible()
  expect(authorizeRedirects.some((location) => location.includes('error=access_denied'))).toBe(true)
  expect(authorizeRedirects.some((location) => location.includes('code='))).toBe(false)
  expect(tokenRequests).toHaveLength(0)
  expect(psql(`SELECT count(*) FROM audit_log a
               JOIN users u ON u.id = a.actor_user_id
               WHERE u.email = :'email' AND a.action = 'oauth_authorization_rejected'
                 AND a.metadata->>'reason' = 'organization_mismatch' AND a.metadata->>'client_id' = 'contabilidad';`, { email: employee.email })).toBe('1')
})

test('RF-022 Un usuario sin rol en una aplicación de su empresa es rechazado', async ({ playwright }) => {
  const employee = await seedCompanyUser(organizations.empresaB)
  const session = await loginWithMfaContext(playwright, employee.email, strongPassword)
  const redirectUri = 'http://inventario.localhost:8080/oauth/callback'
  const query = new URLSearchParams({
    client_id: 'inventario',
    redirect_uri: redirectUri,
    response_type: 'code',
    code_challenge: 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM',
    code_challenge_method: 'S256',
    state: 'e2e-state',
  })
  try {
    await paced()
    const response = await session.context.get(`/oauth/authorize?${query.toString()}`, { maxRedirects: 0 })

    expect(response.status()).toBe(302)
    const location = new URL(response.headers().location ?? '')
    expect(location.origin + location.pathname).toBe(redirectUri)
    expect(location.searchParams.get('error')).toBe('access_denied')
    expect(location.searchParams.get('state')).toBe('e2e-state')
    expect(location.searchParams.get('code')).toBeNull()
    expect(psql(`SELECT count(*) FROM audit_log a
                 JOIN users u ON u.id = a.actor_user_id
                 WHERE u.email = :'email' AND a.action = 'oauth_authorization_rejected'
                   AND a.metadata->>'reason' = 'no_application_role' AND a.metadata->>'client_id' = 'inventario';`, { email: employee.email })).toBe('1')
  } finally {
    await session.context.dispose()
  }
})

test('RF-022 Tras el login se listan solo las aplicaciones de la empresa del usuario', async ({ page }) => {
  const employee = await seedCompanyUser(organizations.contabilidad, ['contabilidad.analista'])

  await page.goto(`${hubUrl}/login`)
  await browserLoginWithMfa(page, employee.email, strongPassword)

  // With exactly one application the Hub sends the user straight to it...
  await page.waitForURL(`${contabilidadUrl}/**`)
  await expect(page.getByRole('button', { name: 'Continuar con Identity Hub' })).toBeVisible()

  // ...and the launcher of the Hub lists that application and no other company's.
  await page.goto(`${hubUrl}/me`)
  const apps = page.locator('.connected-apps')
  await expect(apps.getByRole('link', { name: 'Contabilidad', exact: true })).toBeVisible()
  await expect(apps.getByRole('link', { name: 'Inventario' })).toHaveCount(0)
})
