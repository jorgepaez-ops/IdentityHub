import { expect, test, type PlaywrightWorkerArgs } from '@playwright/test'
import { createActiveUser, inviteUser, seedAdmin, strongPassword } from '../support/accounts'
import { api } from '../support/api'
import { browserLoginWithMfa } from '../support/browser'
import { contextApi, loginWithMfaContext } from '../support/auth'
import { contabilidadUrl, hubUrl } from '../support/config'
import { psql } from '../support/db'
import { mailIds } from '../support/mailpit'

type Playwright = PlaywrightWorkerArgs['playwright']

async function acceptInvitation(token: string): Promise<void> {
  expect((await api('POST', '/api/v1/auth/invitations/accept', {
    json: { token, password: strongPassword },
  })).status).toBe(204)
}

test('RF-009 Un usuario corriente no accede a la administración', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  const session = await loginWithMfaContext(playwright, email, strongPassword)

  expect((await contextApi(session.context, 'GET', '/api/v1/admin/users', {
    headers: { Authorization: `Bearer ${session.accessToken}` },
  })).status).toBe(403)
  await session.context.dispose()
})

test('RF-009 Añadir el rol admin al token no concede privilegios', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  const session = await loginWithMfaContext(playwright, email, strongPassword)
  const [header, payload, signature] = session.accessToken.split('.')
  const claims = JSON.parse(Buffer.from(payload, 'base64url').toString()) as { roles?: string[] }
  const altered = `${header}.${Buffer.from(JSON.stringify({ ...claims, roles: [...(claims.roles ?? []), 'admin'] })).toString('base64url')}.${signature}`

  expect((await api('GET', '/api/v1/admin/users', { token: altered })).status).toBe(401)
  await session.context.dispose()
})

test('RF-009 Un empleado sin rol de aplicación no accede a Contabilidad', async ({ page }) => {
  const admin = await seedAdmin()
  const email = await createActiveUser(admin)
  const existingMailIds = await mailIds(email)
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
  await browserLoginWithMfa(page, email, strongPassword, { existingMailIds, since })

  await expect(page.getByRole('heading', { name: 'Sin acceso a Contabilidad', exact: true })).toHaveText('Sin acceso a Contabilidad')
  // The Hub refused at authorize: the browser came back with access_denied, never a code,
  // and Contabilidad never asked for a token.
  const rejection = authorizeRedirects.find((location) => location.includes('error=access_denied'))
  expect(rejection).toBeDefined()
  expect(authorizeRedirects.some((location) => location.includes('code='))).toBe(false)
  expect(tokenRequests).toHaveLength(0)
  expect(psql(`SELECT count(*) FROM audit_log a
               JOIN users u ON u.id = a.actor_user_id
               WHERE u.email = :'email' AND a.action = 'oauth_authorization_rejected'
                 AND a.metadata->>'reason' = 'no_application_role' AND a.metadata->>'client_id' = 'contabilidad';`, { email })).toBe('1')
})

test('RF-010 Un administrador deshabilita una cuenta', async () => {
  const admin = await seedAdmin()
  const target = await inviteUser(admin)
  await acceptInvitation(target.token)
  const disabled = await api('PATCH', `/api/v1/admin/users/${target.id}`, {
    token: admin.accessToken,
    json: { status: 'disabled' },
  })

  expect(disabled.status).toBe(200)
  expect((await api('POST', '/api/v1/auth/login', {
    json: { email: target.email, password: strongPassword },
  })).status).toBe(401)
  expect(psql(`SELECT count(*) FROM audit_log a
               JOIN users actor ON actor.id = a.actor_user_id
               WHERE actor.email = :'email' AND a.action = 'user_disabled' AND a.resource_id = :'resource_id';`, {
    email: admin.email,
    resource_id: target.id,
  })).toBe('1')
})

test('RF-010 Un administrador no puede deshabilitarse a sí mismo', async () => {
  const admin = await seedAdmin()
  const current = await api<{ id: string }>('GET', '/api/v1/me', { token: admin.accessToken })

  expect(current.status).toBe(200)
  expect((await api('PATCH', `/api/v1/admin/users/${current.body.id}`, {
    token: admin.accessToken,
    json: { status: 'disabled' },
  })).status).toBe(400)
})

test('RF-010 Un administrador no puede asignarse roles a sí mismo', async () => {
  const admin = await seedAdmin()
  const current = await api<{ id: string }>('GET', '/api/v1/me', { token: admin.accessToken })

  expect(current.status).toBe(200)
  expect((await api('PATCH', `/api/v1/admin/users/${current.body.id}`, {
    token: admin.accessToken,
    json: { roles: ['user', 'admin', 'contabilidad.senior'] },
  })).status).toBe(400)
  // Exactly one: seedAdmin() creates a brand-new admin on every call, so no earlier test can have
  // produced this event for this actor.
  expect(psql(`SELECT count(*) FROM audit_log a
               JOIN users actor ON actor.id = a.actor_user_id
               WHERE actor.email = :'email' AND a.action = 'role_assignment_rejected' AND a.resource_id = :'resource_id';`, {
    email: admin.email,
    resource_id: current.body.id,
  })).toBe('1')
})

test('RF-009 El rol base "user" no se puede quitar', async () => {
  const admin = await seedAdmin()
  const target = await inviteUser(admin, ['contabilidad.analista'])
  await acceptInvitation(target.token)
  const changed = await api('PATCH', `/api/v1/admin/users/${target.id}`, {
    token: admin.accessToken,
    json: { roles: ['contabilidad.senior'] },
  })

  expect(changed.status).toBe(400)
  const after = await api<{ roles: string[] }>('GET', `/api/v1/admin/users/${target.id}`, { token: admin.accessToken })
  expect(after.status).toBe(200)
  expect([...after.body.roles].sort()).toEqual(['contabilidad.analista', 'user'])
})

test('RF-011 El registro de auditoría no se puede alterar', async () => {
  await seedAdmin()
  // The scenario is about the application: switch to its role (identity_app), which has UPDATE and
  // DELETE revoked on audit_log, so both fail with insufficient_privilege ("permission denied").
  // The append-only triggers additionally stop the table owner; that is covered by migrations.
  expect(psql(`SET ROLE identity_app;
DO $$
DECLARE audit_id bigint;
BEGIN
  SELECT id INTO audit_id FROM audit_log ORDER BY id LIMIT 1;
  IF audit_id IS NULL THEN RAISE EXCEPTION 'audit_log unexpectedly has no rows'; END IF;
  BEGIN
    UPDATE audit_log SET metadata = metadata WHERE id = audit_id;
    RAISE EXCEPTION 'audit_log UPDATE unexpectedly succeeded';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  BEGIN
    DELETE FROM audit_log WHERE id = audit_id;
    RAISE EXCEPTION 'audit_log DELETE unexpectedly succeeded';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
END $$;`)).toBe('')
})

test('RF-019 Un administrador exporta el registro de auditoría filtrado', async () => {
  const admin = await seedAdmin()
  // A login_failed row gives the filter something to find, next to the admin's own successful login events.
  psql(`INSERT INTO audit_log (action, metadata) VALUES ('login_failed', '{}'::jsonb)`)

  const response = await fetch(`${hubUrl}/api/v1/admin/audit-log/export?format=csv&action=login_failed`, {
    headers: { Authorization: `Bearer ${admin.accessToken}` },
  })
  expect(response.status).toBe(200)
  expect(response.headers.get('content-type')).toContain('text/csv')
  expect(response.headers.get('content-disposition')).toMatch(/^attachment; filename="audit-log-\d{8}T\d{6}Z\.csv"$/)
  const lines = (await response.text()).replace(/^\uFEFF/, '').trim().split('\r\n')
  expect(lines[0]).toBe('id,created_at,actor_user_id,action,resource_type,resource_id,ip,user_agent,metadata')
  expect(lines.length).toBeGreaterThan(1)
  for (const line of lines.slice(1)) expect(line.split(',')[3]).toBe('login_failed')

  const recorded = psql(`SELECT count(*) FROM audit_log WHERE action = 'audit_log_exported' AND metadata->>'format' = 'csv' AND metadata->'filters'->>'action' = 'login_failed'`)
  expect(Number(recorded)).toBeGreaterThanOrEqual(1)
})

test('RF-019 Un usuario corriente no puede exportar el registro de auditoría', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  const session = await loginWithMfaContext(playwright, email, strongPassword)

  expect((await contextApi(session.context, 'GET', '/api/v1/admin/audit-log/export?format=json', {
    headers: { Authorization: `Bearer ${session.accessToken}` },
  })).status).toBe(403)
  await session.context.dispose()
})
