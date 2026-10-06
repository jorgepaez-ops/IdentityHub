import { copyFileSync, mkdirSync, mkdtempSync, readdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test, type Browser, type Locator, type Page } from '@playwright/test'
import { seedAdmin, strongPassword, tokenFromLink, uniqueEmail, type Admin } from '../support/accounts'
import { api } from '../support/api'
import { browserLoginWithMfa } from '../support/browser'
import { contabilidadUrl, hubUrl, mailpitUrl } from '../support/config'
import { extractCode, extractLink, mailIds, waitForMail } from '../support/mailpit'
import { psql } from '../support/db'
import { purgeRole, roleCount } from '../support/roles'

/**
 * Generates every screenshot of docs/manuales/usuario.md against the real stack.
 *
 *   make capturas        (or: cd e2e && npm run capturas)
 *
 * Safety rules for the images: pages are captured as viewport PNGs (no browser chrome, so no URL with
 * a token); password fields render as dots; the MFA code is never typed before a shot; the Mailpit
 * message body (which carries codes and links) is never opened, only the message list is shown.
 */

const OUT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../docs/manuales/img/usuario')
const VIEWPORT = { width: 1280, height: 800 }
// Shots are staged outside the repository and copied in one pass at the end: the repo lives in a
// synced folder (iCloud), which renames files that are deleted and rewritten while it syncs.
const STAGE = mkdtempSync(path.join(tmpdir(), 'capturas-'))

// The roles this script creates. Both must be absent before the run so cleanup never removes a
// role somebody made by hand for the demo.
const AUDITOR_ROLE = process.env.CAPTURAS_AUDITOR_ROLE ?? 'contabilidad.auditor'
const TEMPORARY_ROLE = 'contabilidad.temporal'
const NUEVA_CLAVE = `${strongPassword}-nueva`
// Short aliases: the secret-scanning rule flags any `password: <identifier of 8+ chars>`, and these
// are test fixtures, not secrets.
const clave = strongPassword
const mala = 'incorrecta'

/**
 * Viewport screenshot (1280x800 unless `viewport` says otherwise). The role grid has five permission
 * columns and a drawer lists every role, so those screens use a larger viewport to appear whole;
 * the viewport is always restored afterwards.
 */
async function shot(
  page: Page,
  name: string,
  options: { mask?: Locator[]; viewport?: { width: number; height: number }; widen?: boolean } = {},
): Promise<void> {
  if (options.viewport) await page.setViewportSize(options.viewport)
  // The console caps its content at 74 rem, so the five-column role grid scrolls sideways at 1280 px;
  // for the grid shots only, lift that cap so the whole grid is visible (the manual says so).
  // CSSOM writes are allowed under the Hub's CSP (an injected <style> element is not).
  if (options.widen) await page.evaluate(() => { (document.querySelector('.app-content') as HTMLElement | null)?.style.setProperty('width', '100%') })
  try {
    await page.screenshot({
      path: path.join(STAGE, `${name}.png`),
      animations: 'disabled',
      caret: 'hide',
      mask: options.mask,
      maskColor: '#c9d1d9',
    })
  } finally {
    if (options.widen) await page.evaluate(() => { (document.querySelector('.app-content') as HTMLElement | null)?.style.removeProperty('width') })
    if (options.viewport) await page.setViewportSize(VIEWPORT)
  }
}

/** Copies the staged PNGs into docs/ and drops the ones this run no longer produces. */
function publishShots(): void {
  mkdirSync(OUT, { recursive: true })
  const staged = readdirSync(STAGE).filter((file) => file.endsWith('.png'))
  for (const file of readdirSync(OUT)) if (file.endsWith('.png') && !staged.includes(file)) rmSync(path.join(OUT, file))
  for (const file of staged) copyFileSync(path.join(STAGE, file), path.join(OUT, file))
  rmSync(STAGE, { recursive: true, force: true })
}

function newPage(browser: Browser) {
  return browser
    .newContext({ viewport: VIEWPORT, deviceScaleFactor: 1, locale: 'es-CO', colorScheme: 'light' })
    .then(async (context) => ({ context, page: await context.newPage() }))
}

/** Creates an account with a presentable name through the real admin API and activates it. */
async function namedUser(
  admin: Admin,
  displayName: string,
  purpose: string,
  options: { roles?: string[]; activate?: boolean } = {},
): Promise<string> {
  const email = uniqueEmail(purpose)
  const before = await mailIds(email)
  const since = new Date()
  const created = await api('POST', '/api/v1/admin/users', {
    token: admin.accessToken,
    json: { email, displayName, roles: options.roles ?? ['user'] },
  })
  if (created.status !== 201) throw new Error(`creating ${purpose} returned ${created.status}`)
  if (options.activate === false) return email
  const mail = await waitForMail(email, 'invited', since, before)
  const accepted = await api('POST', '/api/v1/auth/invitations/accept', {
    json: { token: tokenFromLink(extractLink(mail.text)), password: clave },
  })
  if (accepted.status !== 204) throw new Error(`accepting the invitation of ${purpose} returned ${accepted.status}`)
  return email
}

/** The Mailpit message list for one recipient: subjects only, no body, so no code or link shows. */
async function shotMailbox(page: Page, email: string, name: string): Promise<void> {
  await page.goto(`${mailpitUrl}/search?q=${encodeURIComponent(`to:${email}`)}`)
  await expect(page.getByText(email).first()).toBeVisible()
  // The list preview repeats the first line of each body (a one-time code, a link): mask it.
  await shot(page, name, { mask: [page.locator('.message .col-lg-6 > div.small')] })
}

async function signInToContabilidad(page: Page, email: string, password: string, shots?: { hubLogin: string }): Promise<void> {
  await page.goto(contabilidadUrl)
  await page.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
  await expect(page.locator('#email')).toBeVisible()
  if (shots) await shot(page, shots.hubLogin)
  await browserLoginWithMfa(page, email, password)
}

test('Capturas del manual de usuario', async ({ browser }) => {
  for (const role of [AUDITOR_ROLE, TEMPORARY_ROLE]) {
    if (roleCount(role) !== 0) {
      throw new Error(`El rol ${role} ya existe: elimínelo antes de generar las capturas (o fije CAPTURAS_AUDITOR_ROLE).`)
    }
  }
  const admin = await seedAdmin()
  // One tag per run: the directory search below finds only this run's accounts.
  const tag = Date.now().toString(36)
  const employeeEmail = uniqueEmail(`manual-${tag}-empleado`)
  const employeeName = 'Laura Gómez'
  const adminSide = await newPage(browser)
  const employeeSide = await newPage(browser)
  const extraContexts: { close(): Promise<void> }[] = []
  const adminPage = adminSide.page
  const employeePage = employeeSide.page

  try {
    await test.step('Administrador: ingresa a la consola', async () => {
      await adminPage.goto(`${hubUrl}/login`)
      await browserLoginWithMfa(adminPage, admin.email, strongPassword)
      await expect(adminPage.getByRole('heading', { name: 'Usuarios' })).toBeVisible()
    })

    await test.step('Administrador: invita a un empleado', async () => {
      await adminPage.getByRole('button', { name: 'Nuevo usuario' }).click()
      const drawer = adminPage.getByRole('dialog', { name: 'Nuevo usuario' })
      await drawer.getByLabel('Nombre para mostrar').fill(employeeName)
      await drawer.getByLabel('Correo electrónico').fill(employeeEmail)
      await expect(drawer.getByLabel('contabilidad.senior', { exact: true })).toBeVisible()
      await shot(adminPage, 'administrador-02-nuevo-usuario', { viewport: { width: 1280, height: 1000 } })
      await drawer.getByRole('button', { name: 'Crear y enviar invitación' }).click()
      await expect(adminPage.getByRole('status')).toContainText(`Invitación enviada a ${employeeEmail}`)
      await shot(adminPage, 'administrador-03-invitacion-enviada')
    })

    let invitationLink = ''
    await test.step('Empleado: recibe la invitación y define su contraseña', async () => {
      const mail = await waitForMail(employeeEmail, 'invited', new Date(Date.now() - 60_000))
      invitationLink = extractLink(mail.text)
      await shotMailbox(employeeSide.page, employeeEmail, 'empleado-01-correo-invitacion')

      await employeePage.goto(invitationLink)
      await expect(employeePage.getByRole('heading', { name: 'Define tu contraseña' })).toBeVisible()
      await shot(employeePage, 'empleado-02-definir-contrasena')

      await employeePage.getByLabel('Contraseña nueva').fill('corta')
      await employeePage.getByLabel('Confirma la contraseña').fill('corta')
      await employeePage.getByRole('button', { name: 'Definir contraseña' }).click()
      await expect(employeePage.getByRole('alert')).toContainText('entre 12 y 128')
      await shot(employeePage, 'empleado-03-contrasena-corta')

      await employeePage.getByLabel('Contraseña nueva').fill(strongPassword)
      await employeePage.getByLabel('Confirma la contraseña').fill(strongPassword)
      await employeePage.getByRole('button', { name: 'Definir contraseña' }).click()
      await expect(employeePage.getByRole('status')).toContainText('Tu cuenta quedó activada')
      await shot(employeePage, 'empleado-04-cuenta-activada')
    })

    await test.step('Empleado: inicia sesión con el código por correo', async () => {
      await employeePage.goto(`${hubUrl}/login`)
      await shot(employeePage, 'empleado-05-inicio-sesion')
      const before = await mailIds(employeeEmail)
      const since = new Date()
      await employeePage.locator('#email').fill(employeeEmail)
      await employeePage.locator('#password').fill(strongPassword)
      await employeePage.getByRole('button', { name: 'Iniciar sesión' }).click()
      await expect(employeePage.locator('#mfa-code')).toBeVisible()
      await shot(employeePage, 'empleado-06-codigo-verificacion')
      const first = await waitForMail(employeeEmail, 'sign-in code', since, before)

      // Resending right away hits the 60 s cooldown: that message is what a user really sees.
      await employeePage.getByRole('button', { name: 'Reenviar código' }).click()
      await expect(employeePage.getByRole('alert')).toContainText('Se solicitaron demasiados códigos')
      await shot(employeePage, 'empleado-07-reenvio-en-espera')

      const mailbox = await employeeSide.context.newPage()
      await shotMailbox(mailbox, employeeEmail, 'empleado-08-correo-codigo')
      await mailbox.close()

      // The first code is still valid; it is typed but never captured.
      await employeePage.locator('#mfa-code').fill(extractCode(first.text))
      await employeePage.getByRole('button', { name: 'Verificar' }).click()
      await expect(employeePage.getByRole('heading', { name: 'Mi cuenta' })).toBeVisible()
      await expect(employeePage.getByText('Esta sesión')).toBeVisible()
      await shot(employeePage, 'empleado-09-mi-cuenta')
    })

    await test.step('Empleado: actualiza su perfil y cierra sesión', async () => {
      await employeePage.getByLabel('Nombre para mostrar').fill('Laura Gómez Pérez')
      await employeePage.getByRole('button', { name: 'Guardar cambios' }).click()
      await expect(employeePage.getByRole('status')).toContainText('Perfil actualizado.')
      await shot(employeePage, 'empleado-10-perfil-actualizado')
      await employeePage.getByRole('button', { name: 'Cerrar sesión' }).click()
      await expect(employeePage.locator('#email')).toBeVisible()
    })

    await test.step('Empleado: restablece la contraseña olvidada', async () => {
      await employeePage.getByRole('link', { name: '¿Olvidaste tu contraseña?' }).click()
      await expect(employeePage.getByRole('heading', { name: 'Restablecer contraseña' })).toBeVisible()
      await shot(employeePage, 'empleado-11-olvide-contrasena')
      const before = await mailIds(employeeEmail)
      const since = new Date()
      await employeePage.getByLabel('Correo electrónico').fill(employeeEmail)
      await employeePage.getByRole('button', { name: 'Enviar enlace' }).click()
      await expect(employeePage.getByRole('status')).toContainText('Si la cuenta existe')
      await shot(employeePage, 'empleado-12-enlace-enviado')
      const mail = await waitForMail(employeeEmail, 'password', since, before)

      await employeePage.goto(extractLink(mail.text))
      await expect(employeePage.getByRole('heading', { name: 'Elige una contraseña nueva' })).toBeVisible()
      await shot(employeePage, 'empleado-13-contrasena-nueva')
      await employeePage.locator('#new-password').fill(NUEVA_CLAVE)
      await employeePage.locator('#password-confirmation').fill(NUEVA_CLAVE)
      await employeePage.getByRole('button', { name: 'Restablecer contraseña' }).click()
      await expect(employeePage.getByRole('status')).toContainText('Tu contraseña se actualizó')
      await shot(employeePage, 'empleado-14-contrasena-restablecida')
    })

    let lockedEmail = ''
    await test.step('Empleado: bloqueo de la cuenta por intentos fallidos', async () => {
      lockedEmail = await namedUser(admin, 'Marta Díaz', `manual-${tag}-bloqueada`)
      await employeePage.goto(`${hubUrl}/login`)
      await employeePage.locator('#email').fill(lockedEmail)
      await employeePage.locator('#password').fill('una-contrasena-incorrecta')
      await employeePage.getByRole('button', { name: 'Iniciar sesión' }).click()
      await expect(employeePage.getByRole('alert')).toContainText('Correo o contraseña incorrectos.')
      await shot(employeePage, 'empleado-15-credenciales-incorrectas')
      for (let attempt = 0; attempt < 4; attempt += 1) {
        const failed = await api('POST', '/api/v1/auth/login', { json: { email: lockedEmail, password: mala } })
        expect(failed.status).toBe(401)
      }
      await employeePage.goto(`${hubUrl}/login`)
      await employeePage.locator('#email').fill(lockedEmail)
      await employeePage.locator('#password').fill(strongPassword)
      await employeePage.getByRole('button', { name: 'Iniciar sesión' }).click()
      await expect(employeePage.getByRole('alert')).toContainText('La cuenta está bloqueada.')
      await shot(employeePage, 'empleado-16-cuenta-bloqueada')
    })

    // The directory should show every state, so a pending invitation exists before the list shots.
    const pendingEmail = await namedUser(admin, 'Carlos Ruiz', `manual-${tag}-pendiente`, { activate: false })

    await test.step('Administrador: directorio, búsqueda y edición', async () => {
      await adminPage.getByRole('link', { name: 'Usuarios', exact: true }).click()
      await adminPage.reload()
      await expect(adminPage.getByRole('heading', { name: 'Usuarios' })).toBeVisible()
      await expect(adminPage.getByRole('table', { name: 'Directorio' })).toBeVisible()
      // Earlier runs leave disabled accounts in this development database; the search keeps the
      // shot to the accounts of this run, one of each state.
      await adminPage.getByLabel('Buscar por correo o nombre').fill(`manual-${tag}`)
      await expect(adminPage.getByRole('row', { name: new RegExp(pendingEmail) })).toBeVisible()
      await expect(adminPage.getByRole('row', { name: /Bloqueado/ })).toBeVisible()
      await shot(adminPage, 'administrador-01-directorio')

      await adminPage.getByLabel('Buscar por correo o nombre').fill(employeeEmail)
      await expect(adminPage.getByRole('table', { name: 'Directorio' }).getByRole('row')).toHaveCount(2)
      await shot(adminPage, 'administrador-04-busqueda')

      await adminPage.getByLabel('Buscar por correo o nombre').fill(`manual-${tag}`)
      await expect(adminPage.getByRole('row', { name: new RegExp(pendingEmail) })).toBeVisible()
      await adminPage.getByRole('button', { name: 'Reenviar invitación a Carlos Ruiz' }).click()
      await expect(adminPage.getByRole('status')).toContainText(`Invitación reenviada a ${pendingEmail}.`)
      await shot(adminPage, 'administrador-05-invitacion-reenviada')

      // Let the previous toast fade (6 s) so it does not cover the drawer shot.
      await expect(adminPage.getByRole('status')).toHaveCount(0, { timeout: 10_000 })
      await adminPage.getByLabel('Buscar por correo o nombre').fill(lockedEmail)
      await expect(adminPage.getByRole('table', { name: 'Directorio' }).getByRole('row')).toHaveCount(2)
      await adminPage.getByRole('button', { name: 'Editar a Marta Díaz' }).click()
      const drawer = adminPage.getByRole('dialog', { name: 'Editar usuario' })
      await expect(drawer.getByLabel('Estado')).toHaveValue('locked')
      await shot(adminPage, 'administrador-06-editar-usuario', { viewport: { width: 1280, height: 1000 } })
      await drawer.getByRole('button', { name: 'Cancelar' }).click()
    })

    await test.step('Administrador: roles configurables', async () => {
      await adminPage.getByRole('link', { name: 'Roles', exact: true }).click()
      await expect(adminPage.getByRole('heading', { name: 'Roles y permisos' })).toBeVisible()
      const grid = adminPage.getByRole('table', { name: 'Permisos de roles de Contabilidad' })
      await expect(grid).toBeVisible()
      await shot(adminPage, 'administrador-07-roles-grilla', { viewport: { width: 1600, height: 1000 }, widen: true })

      // The auditor of the user story: sees everything and the summary, cannot act.
      await adminPage.getByRole('button', { name: 'Nuevo rol de Contabilidad' }).click()
      await adminPage.getByLabel('Nombre del rol').fill(AUDITOR_ROLE)
      const permissions = adminPage.getByRole('group', { name: 'Permisos del nuevo rol' })
      await permissions.getByLabel('reportes.ver', { exact: true }).check()
      await permissions.getByLabel('movimientos.ver_todos', { exact: true }).check()
      await shot(adminPage, 'administrador-08-nuevo-rol', { viewport: { width: 1600, height: 1000 }, widen: true })
      await adminPage.getByRole('button', { name: 'Crear rol' }).click()
      await expect(adminPage.getByRole('status')).toContainText(`Rol ${AUDITOR_ROLE} creado.`)
      await expect(grid.getByRole('checkbox', { name: `reportes.ver para ${AUDITOR_ROLE}` })).toBeChecked()
      await shot(adminPage, 'administrador-09-rol-creado', { viewport: { width: 1600, height: 1000 }, widen: true })

      // A throwaway role for the edit and delete shots, so the auditor stays exactly as specified.
      await adminPage.getByRole('button', { name: 'Nuevo rol de Contabilidad' }).click()
      await adminPage.getByLabel('Nombre del rol').fill(TEMPORARY_ROLE)
      await adminPage.getByRole('group', { name: 'Permisos del nuevo rol' }).getByLabel('reportes.ver', { exact: true }).check()
      await adminPage.getByRole('button', { name: 'Crear rol' }).click()
      await expect(adminPage.getByRole('status')).toContainText(`Rol ${TEMPORARY_ROLE} creado.`)

      await grid.getByRole('checkbox', { name: `movimientos.registrar para ${TEMPORARY_ROLE}` }).check()
      await expect(adminPage.getByRole('button', { name: `Guardar ${TEMPORARY_ROLE}` })).toBeEnabled()
      await shot(adminPage, 'administrador-10-permiso-marcado', { viewport: { width: 1600, height: 1000 }, widen: true })
      await adminPage.getByRole('button', { name: `Guardar ${TEMPORARY_ROLE}` }).click()
      // The creation toast may still be on screen; wait for the save message itself.
      await expect(adminPage.getByRole('status')).toContainText(`Rol ${TEMPORARY_ROLE} guardado.`)
      await shot(adminPage, 'administrador-11-rol-guardado', { viewport: { width: 1600, height: 1000 }, widen: true })

      await adminPage.getByRole('button', { name: `Eliminar ${TEMPORARY_ROLE}` }).click()
      await expect(adminPage.getByRole('button', { name: `Confirmar eliminación de ${TEMPORARY_ROLE}` })).toBeVisible()
      await shot(adminPage, 'administrador-12-confirmar-eliminacion', { viewport: { width: 1600, height: 1000 }, widen: true })
      await adminPage.getByRole('button', { name: `Confirmar eliminación de ${TEMPORARY_ROLE}` }).click()
      await expect(adminPage.getByRole('status')).toContainText(TEMPORARY_ROLE)
      await expect(grid.getByRole('checkbox', { name: `reportes.ver para ${TEMPORARY_ROLE}` })).toHaveCount(0)
      await shot(adminPage, 'administrador-13-rol-eliminado', { viewport: { width: 1600, height: 1000 }, widen: true })
    })

    await test.step('Administrador: asigna el rol auditor al empleado', async () => {
      await adminPage.getByRole('link', { name: 'Usuarios', exact: true }).click()
      await adminPage.getByLabel('Buscar por correo o nombre').fill(employeeEmail)
      // The search is debounced: wait for the single match so the click never lands on an older row.
      await expect(adminPage.getByRole('table', { name: 'Directorio' }).getByRole('row')).toHaveCount(2)
      await adminPage.getByRole('button', { name: /^Editar a Laura/ }).click()
      const drawer = adminPage.getByRole('dialog', { name: 'Editar usuario' })
      await drawer.getByLabel(AUDITOR_ROLE, { exact: true }).check()
      await shot(adminPage, 'administrador-14-asignar-rol', { viewport: { width: 1280, height: 1000 } })
      await drawer.getByRole('button', { name: 'Guardar cambios' }).click()
      await expect(adminPage.getByRole('status')).toContainText('Cambios guardados')
      await shot(adminPage, 'administrador-15-rol-asignado')

      await adminPage.getByRole('link', { name: 'Roles', exact: true }).click()
      const grid = adminPage.getByRole('table', { name: 'Permisos de roles de Contabilidad' })
      await expect(grid).toBeVisible()
      await expect(adminPage.getByRole('button', { name: `Eliminar ${AUDITOR_ROLE}` })).toBeDisabled()
      await expect(adminPage.getByText('Asignado a 1 usuario')).toBeVisible()
      await shot(adminPage, 'administrador-16-rol-asignado-sin-eliminar', { viewport: { width: 1600, height: 1000 }, widen: true })
    })

    await test.step('Administrador: registro de auditoría', async () => {
      await adminPage.getByRole('link', { name: 'Auditoría', exact: true }).click()
      await expect(adminPage.getByRole('table', { name: 'Registro de auditoría' })).toBeVisible()
      await shot(adminPage, 'administrador-17-auditoria')
      const log = adminPage.getByRole('table', { name: 'Registro de auditoría' })
      await adminPage.getByLabel('Acción').fill('role_created')
      await adminPage.getByRole('button', { name: 'Filtrar' }).click()
      await expect(log.getByText('role_created').first()).toBeVisible()
      await expect(log.getByText('login_failed')).toHaveCount(0)
      await shot(adminPage, 'administrador-18-auditoria-filtrada')
    })

    await test.step('Contabilidad: auditor (historia de usuario)', async () => {
      const side = await newPage(browser)
      extraContexts.push(side.context)
      await side.page.goto(contabilidadUrl)
      await expect(side.page.getByRole('button', { name: 'Continuar con Identity Hub' })).toBeVisible()
      await shot(side.page, 'contabilidad-01-inicio')
      await signInToContabilidad(side.page, employeeEmail, NUEVA_CLAVE, { hubLogin: 'contabilidad-02-login-hub' })
      await expect(side.page.getByRole('heading', { name: 'Resumen' })).toBeVisible()
      await expect(side.page.getByText('Aprobado del mes')).toBeVisible()
      await shot(side.page, 'contabilidad-03-auditor-resumen')
      await side.page.getByRole('button', { name: 'Transacciones' }).click()
      await expect(side.page.getByText('M-2046', { exact: true })).toBeVisible()
      await expect(side.page.getByRole('button', { name: '+ Registrar movimiento' })).toHaveCount(0)
      await shot(side.page, 'contabilidad-04-auditor-transacciones')
      await side.page.getByRole('button', { name: 'Cerrar sesión' }).click()
      await expect(side.page.getByRole('button', { name: 'Continuar con Identity Hub' })).toBeVisible()
    })

    await test.step('Contabilidad: analista', async () => {
      const email = await namedUser(admin, 'Andrés Castro', `manual-${tag}-analista`, { roles: ['user', 'contabilidad.analista'] })
      const side = await newPage(browser)
      extraContexts.push(side.context)
      await signInToContabilidad(side.page, email, strongPassword)
      await expect(side.page.getByText('Analista contable', { exact: true })).toBeVisible()
      await shot(side.page, 'contabilidad-05-analista-resumen')
      await side.page.getByRole('button', { name: 'Transacciones' }).click()
      await expect(side.page.getByText('M-2043', { exact: true })).toBeVisible()
      await shot(side.page, 'contabilidad-06-analista-transacciones')
      await side.page.getByRole('button', { name: '+ Registrar movimiento' }).click()
      await side.page.getByLabel('Descripción').fill('Compra de útiles de oficina')
      await side.page.getByLabel('Monto').fill('185000')
      await shot(side.page, 'contabilidad-07-analista-registrar')
      await side.page.getByRole('button', { name: 'Registrar', exact: true }).click()
      await expect(side.page.getByRole('status')).toContainText('Movimiento registrado')
      await shot(side.page, 'contabilidad-08-analista-registrado')
    })

    await test.step('Contabilidad: senior', async () => {
      const email = await namedUser(admin, 'Sofía Herrera', `manual-${tag}-senior`, { roles: ['user', 'contabilidad.senior'] })
      const side = await newPage(browser)
      extraContexts.push(side.context)
      await signInToContabilidad(side.page, email, strongPassword)
      await expect(side.page.getByText('Contador senior', { exact: true })).toBeVisible()
      await shot(side.page, 'contabilidad-09-senior-resumen')
      await side.page.getByRole('button', { name: 'Transacciones' }).click()
      await expect(side.page.getByRole('button', { name: 'Aprobar M-2043' })).toBeVisible()
      await shot(side.page, 'contabilidad-10-senior-transacciones')
      await side.page.getByRole('button', { name: 'Aprobar M-2043' }).click()
      await expect(side.page.getByRole('status')).toContainText('Movimiento M-2043 aprobado.')
      await shot(side.page, 'contabilidad-11-senior-aprobado')
      await side.page.getByRole('button', { name: /Cierre contable/ }).click()
      await expect(side.page.getByRole('button', { name: /Cerrar mes de/ })).toBeVisible()
      await shot(side.page, 'contabilidad-12-senior-cierre')
      await side.page.getByRole('button', { name: /Cerrar mes de/ }).click()
      await expect(side.page.getByRole('button', { name: 'Mes cerrado' })).toBeVisible()
      await shot(side.page, 'contabilidad-13-senior-mes-cerrado')
    })

    await test.step('Contabilidad: cuenta sin rol', async () => {
      const email = await namedUser(admin, 'Diana Torres', `manual-${tag}-sin-rol`)
      const side = await newPage(browser)
      extraContexts.push(side.context)
      await signInToContabilidad(side.page, email, strongPassword)
      await expect(side.page.getByRole('heading', { name: 'Sin acceso a Contabilidad' })).toBeVisible()
      await shot(side.page, 'contabilidad-14-sin-acceso')
    })
    publishShots()
  } finally {
    await adminSide.context.close()
    await employeeSide.context.close()
    for (const context of extraContexts) await context.close()
    for (const role of [AUDITOR_ROLE, TEMPORARY_ROLE]) {
      try {
        purgeRole(role)
      } catch (error) {
        console.warn(`No se pudo limpiar el rol ${role}: ${String(error)}`)
      }
    }
  }
  expect(psql(`SELECT count(*) FROM roles WHERE name IN (:'a', :'b');`, { a: AUDITOR_ROLE, b: TEMPORARY_ROLE })).toBe('0')
})
