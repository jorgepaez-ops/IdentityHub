import { expect, test } from '@playwright/test'
import { type Admin, createActiveUser, seedAdmin, strongPassword } from '../support/accounts'
import { api, loginWithMfa } from '../support/api'
import { browserLoginWithMfa } from '../support/browser'
import { hubUrl } from '../support/config'
import { psql } from '../support/db'

const keysPath = '/api/v1/admin/service-keys'
const auditPath = '/api/v1/admin/audit-log'
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/

interface IssuedKey {
  id: string
  key: string
  prefix: string
}

const secretOf = (key: string) => key.slice(key.indexOf('_', 'ihk_'.length) + 1)

async function issueKey(admin: Admin, name: string, expiresInDays = 30): Promise<IssuedKey> {
  const created = await api<IssuedKey>('POST', keysPath, {
    token: admin.accessToken,
    json: { name, scopes: ['audit-log:read'], expiresInDays },
  })
  expect(created.status).toBe(201)
  return created.body
}

test('RF-018 Admin emite una clave, la usa para leer el audit log y la revoca', async ({ browser }) => {
  test.setTimeout(180_000)
  const admin = await seedAdmin()
  const context = await browser.newContext()
  const page = await context.newPage()
  const name = `SIEM e2e ${Date.now()}`

  try {
    await page.goto(`${hubUrl}/login`)
    await browserLoginWithMfa(page, admin.email, strongPassword)
    await expect(page.getByRole('heading', { name: 'Inicio' })).toBeVisible()
    await page.getByRole('link', { name: 'Claves de servicio', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Claves de servicio' })).toBeVisible()

    await page.getByLabel('Nombre de la clave').fill(name)
    await expect(page.getByRole('checkbox', { name: 'audit-log:read' })).toBeChecked()
    await page.getByLabel('Vigencia en días').fill('30')
    await page.getByRole('button', { name: 'Emitir clave' }).click()

    const reveal = page.getByRole('alert', { name: 'Clave emitida' })
    await expect(reveal).toContainText('no se volverá a mostrar')
    const key = await reveal.getByLabel('Clave de servicio completa').inputValue()
    expect(key).toMatch(/^ihk_[0-9a-f]{16}_[A-Za-z0-9_-]{43}$/)

    // The SIEM reads the audit log with the key alone: no session, no cookie.
    const used = await api<{ items: { action: string }[] }>('GET', `${auditPath}?action=service_key_created`, { token: key })
    expect(used.status).toBe(200)
    expect(used.body.items.some((item) => item.action === 'service_key_created')).toBe(true)

    await reveal.getByRole('button', { name: 'Ya la copié' }).click()
    await expect(page.getByLabel('Clave de servicio completa')).toHaveCount(0)
    await page.reload()
    await expect(page.getByRole('table', { name: 'Claves de servicio' }).getByText(name)).toBeVisible()
    await expect(page.getByText(key)).toHaveCount(0)

    await page.getByRole('button', { name: `Revocar ${name}` }).click()
    await page.getByRole('button', { name: `Confirmar revocación de ${name}` }).click()
    await expect(page.getByRole('status')).toContainText(`Clave ${name} revocada.`)

    expect((await api('GET', auditPath, { token: key })).status).toBe(401)
  } finally {
    await context.close()
  }
})

test('RF-018 El listado de claves no expone el secreto', async () => {
  const admin = await seedAdmin()
  const issued = await issueKey(admin, 'Listado e2e')

  const list = await api<Record<string, unknown>[]>('GET', keysPath, { token: admin.accessToken })
  expect(list.status).toBe(200)
  const row = list.body.find((item) => item.id === issued.id)
  expect(row).toBeDefined()
  expect(row?.prefix).toBe(issued.prefix)
  expect(Object.keys(row ?? {}).sort()).toEqual(['createdAt', 'createdBy', 'expiresAt', 'id', 'name', 'prefix', 'scopes'])
  expect(JSON.stringify(list.body)).not.toContain(secretOf(issued.key))
})

test('RF-018 Una clave de servicio solo abre las operaciones que declaran su alcance', async () => {
  const admin = await seedAdmin()
  const issued = await issueKey(admin, 'Alcance e2e')

  expect((await api('GET', '/api/v1/admin/users', { token: issued.key })).status).toBe(401)
  expect((await api('GET', '/api/v1/me', { token: issued.key })).status).toBe(401)
  expect((await api('GET', keysPath, { token: issued.key })).status).toBe(401)
  expect((await api('POST', keysPath, { token: issued.key, json: { name: 'x', scopes: ['audit-log:read'], expiresInDays: 1 } })).status).toBe(401)
  expect((await api('DELETE', `${keysPath}/${issued.id}`, { token: issued.key })).status).toBe(401)
  // The key itself still works where its scope is declared.
  expect((await api('GET', auditPath, { token: issued.key })).status).toBe(200)
})

test('RF-018 Una clave inválida, expirada o revocada recibe la misma respuesta', async () => {
  const admin = await seedAdmin()
  const valid = await issueKey(admin, 'Valida e2e')
  const expired = await issueKey(admin, 'Expirada e2e')
  const revoked = await issueKey(admin, 'Revocada e2e')
  expect(UUID.test(expired.id)).toBe(true)
  psql(`UPDATE service_keys SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day' WHERE id = :'id'::uuid;`, { id: expired.id })
  expect((await api('DELETE', `${keysPath}/${revoked.id}`, { token: admin.accessToken })).status).toBe(204)
  // Revoking again is harmless.
  expect((await api('DELETE', `${keysPath}/${revoked.id}`, { token: admin.accessToken })).status).toBe(204)

  const publicId = valid.prefix.slice('ihk_'.length)
  const attempts: Record<string, string> = {
    'wrong value': `ihk_${publicId}_${'A'.repeat(43)}`,
    'unknown id': `ihk_0000000000000000_${secretOf(valid.key)}`,
    expired: expired.key,
    revoked: revoked.key,
  }
  let reference: unknown
  for (const [label, key] of Object.entries(attempts)) {
    const response = await api('GET', auditPath, { token: key })
    expect(response.status, label).toBe(401)
    reference ??= response.body
    expect(response.body, label).toEqual(reference)
  }

  const rejected = await api<{ items: { metadata: { publicId?: string; reason?: string } }[] }>(
    'GET', `${auditPath}?action=service_key_rejected&limit=100`, { token: admin.accessToken },
  )
  const reasons = rejected.body.items.map((item) => item.metadata.reason)
  // Only rejections tied to an existing key are audited; an unknown id is anonymous junk and goes to the logs.
  for (const reason of ['bad_secret', 'expired', 'revoked']) expect(reasons).toContain(reason)
  expect(reasons).not.toContain('unknown_key')
  expect(JSON.stringify(rejected.body)).not.toContain(secretOf(valid.key))
})

test('RF-018 Solo un administrador gestiona claves de servicio', async () => {
  const admin = await seedAdmin()
  const issued = await issueKey(admin, 'Gestion e2e')
  const email = await createActiveUser(admin)
  const userToken = await loginWithMfa(email, strongPassword)

  expect((await api('POST', keysPath, { token: userToken, json: { name: 'x', scopes: ['audit-log:read'], expiresInDays: 1 } })).status).toBe(403)
  expect((await api('GET', keysPath, { token: userToken })).status).toBe(403)
  expect((await api('DELETE', `${keysPath}/${issued.id}`, { token: userToken })).status).toBe(403)
  expect((await api('GET', auditPath, { token: issued.key })).status).toBe(200)
})
