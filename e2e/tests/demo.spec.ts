import { expect, test } from '@playwright/test'
import { seedAdmin, strongPassword, uniqueEmail } from '../support/accounts'
import { browserLoginWithMfa } from '../support/browser'
import { contabilidadUrl, hubUrl } from '../support/config'
import { extractLink, mailIds, waitForMail } from '../support/mailpit'

const analystRole = 'contabilidad.analista'
const seniorRole = 'contabilidad.senior'
const continueButton = 'Continuar con Identity Hub'

test('DEMO Guion de la demo en vivo: alta, invitación, MFA, SSO y vistas por rol', async ({ browser }) => {
  test.setTimeout(180_000)
  const admin = await seedAdmin()
  const employee = uniqueEmail('demo-empleado')
  const displayName = `Demo ${employee.split('-').slice(-2).join('-').split('@')[0]}`
  const adminContext = await browser.newContext()
  const employeeContext = await browser.newContext()
  const adminPage = await adminContext.newPage()
  const employeePage = await employeeContext.newPage()

  try {
    await test.step('1. El admin entra a la consola del Hub con MFA', async () => {
      await adminPage.goto(`${hubUrl}/login`)
      await browserLoginWithMfa(adminPage, admin.email, strongPassword)
      await expect(adminPage.getByRole('heading', { name: 'Usuarios' })).toBeVisible()
    })

    const invitationLink = await test.step('2. El admin da de alta a un empleado con el rol analista', async () => {
      const before = await mailIds(employee)
      const since = new Date()
      await adminPage.getByRole('button', { name: 'Nuevo usuario' }).click()
      const drawer = adminPage.getByRole('dialog', { name: 'Nuevo usuario' })
      await drawer.getByLabel('Nombre para mostrar').fill(displayName)
      await drawer.getByLabel('Correo electrónico').fill(employee)
      await drawer.getByLabel(analystRole, { exact: true }).check()
      await drawer.getByRole('button', { name: 'Crear y enviar invitación' }).click()
      await expect(adminPage.getByRole('status')).toContainText(`Invitación enviada a ${employee}`)
      // The invitation arrives in Mailpit, the classroom mailbox.
      const mail = await waitForMail(employee, 'invited', since, before)
      return extractLink(mail.text)
    })

    await test.step('3. El empleado abre la invitación y define su contraseña', async () => {
      await employeePage.goto(invitationLink)
      await employeePage.getByLabel('Contraseña nueva').fill(strongPassword)
      await employeePage.getByLabel('Confirma la contraseña').fill(strongPassword)
      await employeePage.getByRole('button', { name: 'Definir contraseña' }).click()
      await expect(employeePage.getByRole('status')).toContainText('Tu cuenta quedó activada')
    })

    await test.step('4. El empleado inicia sesión en el Hub con el código MFA', async () => {
      await employeePage.goto(`${hubUrl}/login`)
      await browserLoginWithMfa(employeePage, employee, strongPassword)
      await expect(employeePage).toHaveURL(`${hubUrl}/me`)
    })

    const mailAfterLogin = await mailIds(employee)

    await test.step('5. Contabilidad redirige al Hub y vuelve sin pedir contraseña', async () => {
      await employeePage.goto(contabilidadUrl)
      await employeePage.getByRole('button', { name: continueButton }).click()
      await expect(employeePage.getByText('Analista contable', { exact: true })).toBeVisible()
      await expect(employeePage).toHaveURL(`${contabilidadUrl}/`)
      await expect(employeePage.locator('#password')).toHaveCount(0)
      expect([...(await mailIds(employee))].sort()).toEqual([...mailAfterLogin].sort())
    })

    await test.step('6. La vista del analista bloquea el cierre contable', async () => {
      const closing = employeePage.getByRole('button', { name: /Cierre contable/ })
      await expect(closing).toBeDisabled()
      await expect(closing.getByRole('img', { name: 'Bloqueado' })).toBeVisible()
    })

    await test.step('7. El admin cambia el rol a senior y el siguiente acceso lo refleja', async () => {
      await adminPage.getByLabel('Buscar por correo o nombre').fill(employee)
      await adminPage.getByRole('button', { name: `Editar a ${displayName}` }).click()
      const drawer = adminPage.getByRole('dialog', { name: 'Editar usuario' })
      await drawer.getByLabel(analystRole, { exact: true }).uncheck()
      await drawer.getByLabel(seniorRole, { exact: true }).check()
      await drawer.getByRole('button', { name: 'Guardar cambios' }).click()
      await expect(adminPage.getByRole('status')).toContainText('Cambios guardados')

      // The access token lives in memory, so a fresh visit to Contabilidad is the "next login".
      await employeePage.goto(contabilidadUrl)
      await employeePage.getByRole('button', { name: continueButton }).click()
      await expect(employeePage.getByText('Contador senior', { exact: true })).toBeVisible()
      await expect(employeePage.getByRole('button', { name: /Cierre contable/ })).toBeEnabled()
      await expect(employeePage.locator('#password')).toHaveCount(0)
    })
  } finally {
    await adminContext.close()
    await employeeContext.close()
  }
})
