import { expect, test } from '@playwright/test'
import { api } from '../support/api'
import { type Admin, inviteUser, seedAdmin, strongPassword } from '../support/accounts'

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
