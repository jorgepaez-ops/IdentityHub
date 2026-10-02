import { expect, test } from '@playwright/test'
import { api } from '../support/api'
import { type Admin, createActiveUser, inviteUser, seedAdmin, strongPassword, uniqueEmail } from '../support/accounts'
import { assertEmail, psql } from '../support/db'
import { extractLink, mailIds, waitForMail } from '../support/mailpit'

let admin: Admin

test.beforeAll(async () => {
  admin = await seedAdmin()
})

test('RF-002 Aceptar la invitación verifica el correo y fija la contraseña', async ({ page }) => {
  const invitee = await inviteUser(admin, ['user'])

  await page.goto(invitee.link)
  await page.locator('#new-password').fill(strongPassword)
  await page.locator('#password-confirmation').fill(strongPassword)
  await page.getByRole('button', { name: 'Definir contraseña' }).click()
  await expect(page.getByRole('status')).toContainText('Tu cuenta quedó activada')

  // The account can start the MFA challenge with that password.
  const login = await api('POST', '/api/v1/auth/login', { json: { email: invitee.email, password: strongPassword } })
  expect(login.status).toBe(202)
})

test('RF-001 Alta administrativa con datos válidos', async () => {
  const email = uniqueEmail('administrative-create')
  const existingMailIds = await mailIds(email)
  const since = new Date()
  const created = await api<{ id: string; email: string; status: string; roles: string[] }>('POST', '/api/v1/admin/users', {
    token: admin.accessToken,
    json: { email, displayName: 'E2E Accountant', roles: ['contabilidad.analista'] },
  })

  expect(created.status).toBe(201)
  expect(created.body).toMatchObject({ email, status: 'pending_verification', roles: ['user', 'contabilidad.analista'] })
  expect(created.body.id).toEqual(expect.any(String))
  await expect.poll(() => psql(`SELECT status FROM users WHERE email = :'email';`, { email: assertEmail(email) })).toBe('pending_verification')
  await waitForMail(email, 'invited', since, existingMailIds)
})

test('RF-001 El autorregistro público no está disponible', async () => {
  const result = await api('POST', '/api/v1/auth/register', {
    json: { email: uniqueEmail('public-register'), password: strongPassword },
  })

  expect(result.status).toBe(404)
})

test('RF-001 Un correo no puede darse de alta dos veces', async () => {
  const email = await createActiveUser(admin)
  const duplicate = await api('POST', '/api/v1/admin/users', {
    token: admin.accessToken,
    json: { email, displayName: 'Duplicate E2E User', roles: ['user'] },
  })

  expect(duplicate.status).toBe(409)
  expect(JSON.stringify(duplicate.body)).not.toContain(email)
})

test('RF-002 La invitación es de un solo uso', async () => {
  const invitee = await inviteUser(admin)
  const first = await api('POST', '/api/v1/auth/invitations/accept', {
    json: { token: invitee.token, password: strongPassword },
  })
  const second = await api('POST', '/api/v1/auth/invitations/accept', {
    json: { token: invitee.token, password: strongPassword },
  })

  expect(first.status).toBe(204)
  expect(second.status).toBe(410)
})

test('RF-002 Una cuenta que no aceptó la invitación no puede iniciar sesión', async () => {
  const invitee = await inviteUser(admin)
  const login = await api('POST', '/api/v1/auth/login', {
    json: { email: invitee.email, password: strongPassword },
  })

  expect(login.status).toBe(401)
})

test('RF-001 Un admin reenvía una invitación pendiente', async () => {
  const invitee = await inviteUser(admin)
  psql(`UPDATE verification_tokens
        SET expires_at = now() - interval '1 minute'
        WHERE user_id = :'user_id' AND purpose = 'invitation' AND used_at IS NULL;`, { user_id: invitee.id })
  const existingMailIds = await mailIds(invitee.email)
  const since = new Date()
  const resent = await api('POST', `/api/v1/admin/users/${invitee.id}/invitation`, { token: admin.accessToken })

  expect(resent.status).toBe(204)
  expect((await api('POST', '/api/v1/auth/invitations/accept', {
    json: { token: invitee.token, password: strongPassword },
  })).status).toBe(410)
  const replacement = await waitForMail(invitee.email, 'invited', since, existingMailIds)
  expect(new URL(extractLink(replacement.text)).searchParams.get('token')).not.toBe(invitee.token)
  expect(psql(`SELECT (expires_at > now() + interval '23 hours 59 minutes'
                       AND expires_at < now() + interval '24 hours 1 minute')::text
               FROM verification_tokens
               WHERE user_id = :'user_id' AND purpose = 'invitation' AND used_at IS NULL;`, { user_id: invitee.id })).toBe('true')
})
