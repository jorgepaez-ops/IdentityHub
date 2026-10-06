import { expect, test } from '@playwright/test'
import { createActiveUser, inviteUser, seedAdmin, strongPassword } from '../support/accounts'
import { api } from '../support/api'
import { browserLoginWithMfa } from '../support/browser'
import { contabilidadUrl, hubUrl } from '../support/config'
import { psql } from '../support/db'
import {
  contabilidadApplication, createRole, ownId, purgeRole, roleCount, uniqueRoleName,
  type ApplicationRoleBody,
} from '../support/roles'

const adminApi = '/api/v1/admin'
const rolePath = (applicationId: string, roleId: string) => `${adminApi}/applications/${applicationId}/roles/${roleId}`

test('RF-021 Admin crea auditor de Contabilidad con capacidades acotadas', async ({ browser }) => {
  test.setTimeout(180_000)
  const roleName = 'contabilidad.auditor'
  purgeRole(roleName) // a previous aborted run must not leave the demo role behind
  const admin = await seedAdmin()
  const employee = await createActiveUser(admin)
  const adminContext = await browser.newContext()
  const employeeContext = await browser.newContext()
  const adminPage = await adminContext.newPage()
  const employeePage = await employeeContext.newPage()

  try {
    await test.step('El admin crea el rol en la grilla de la consola', async () => {
      await adminPage.goto(`${hubUrl}/login`)
      await browserLoginWithMfa(adminPage, admin.email, strongPassword)
      await expect(adminPage.getByRole('heading', { name: 'Usuarios' })).toBeVisible()
      await adminPage.getByRole('link', { name: 'Roles', exact: true }).click()
      await expect(adminPage.getByRole('heading', { name: 'Roles y permisos' })).toBeVisible()
      await adminPage.getByRole('button', { name: 'Nuevo rol de Contabilidad' }).click()
      await adminPage.getByLabel('Nombre del rol').fill(roleName)
      const permissions = adminPage.getByRole('group', { name: 'Permisos del nuevo rol' })
      await permissions.getByLabel('reportes.ver', { exact: true }).check()
      await permissions.getByLabel('movimientos.ver_todos', { exact: true }).check()
      await adminPage.getByRole('button', { name: 'Crear rol' }).click()
      await expect(adminPage.getByRole('status')).toContainText(`Rol ${roleName} creado.`)
      const grid = adminPage.getByRole('table', { name: 'Permisos de roles de Contabilidad' })
      await expect(grid.getByRole('checkbox', { name: `reportes.ver para ${roleName}` })).toBeChecked()
      await expect(grid.getByRole('checkbox', { name: `movimientos.ver_todos para ${roleName}` })).toBeChecked()
      await expect(grid.getByRole('checkbox', { name: `cierre.ejecutar para ${roleName}` })).not.toBeChecked()
    })

    await test.step('El admin asigna el rol al empleado', async () => {
      await adminPage.getByRole('link', { name: 'Usuarios', exact: true }).click()
      await adminPage.getByLabel('Buscar por correo o nombre').fill(employee)
      await adminPage.getByRole('row', { name: new RegExp(employee) }).getByRole('button', { name: 'Editar a E2E User' }).click()
      const drawer = adminPage.getByRole('dialog', { name: 'Editar usuario' })
      await drawer.getByLabel(roleName, { exact: true }).check()
      await drawer.getByRole('button', { name: 'Guardar cambios' }).click()
      await expect(adminPage.getByRole('status')).toContainText('Cambios guardados')
    })

    await test.step('El empleado ve todo y el Resumen, pero no puede actuar', async () => {
      await employeePage.goto(contabilidadUrl)
      await employeePage.getByRole('button', { name: 'Continuar con Identity Hub' }).click()
      await browserLoginWithMfa(employeePage, employee, strongPassword)
      // Lands on the summary because the role carries reportes.ver.
      await expect(employeePage.getByRole('heading', { name: 'Resumen' })).toBeVisible()
      await expect(employeePage.getByText('Aprobado del mes')).toBeVisible()
      // The hint lists exactly the capabilities of the token: nothing else was granted.
      await expect(employeePage.getByText('Tu rol permite: ver todos los movimientos, ver el resumen.')).toBeVisible()

      await employeePage.getByRole('button', { name: 'Transacciones' }).click()
      for (const folio of ['M-2041', 'M-2042', 'M-2043', 'M-2044', 'M-2045', 'M-2046']) {
        await expect(employeePage.getByText(folio, { exact: true })).toBeVisible()
      }
      await expect(employeePage.getByRole('button', { name: '+ Registrar movimiento' })).toHaveCount(0)
      await expect(employeePage.getByRole('button', { name: /^Aprobar / })).toHaveCount(0)
      await expect(employeePage.getByRole('button', { name: /^Rechazar / })).toHaveCount(0)
      const closing = employeePage.getByRole('button', { name: /Cierre contable/ })
      await expect(closing).toBeDisabled()
      await expect(closing.getByRole('img', { name: 'Bloqueado' })).toBeVisible()
    })
  } finally {
    await adminContext.close()
    await employeeContext.close()
    purgeRole(roleName)
  }
})

test('RF-021 Se rechaza crear un rol de directorio desde la grilla', async () => {
  const admin = await seedAdmin()
  const application = await contabilidadApplication(admin)
  const before = psql(`SELECT count(*) || ':' || coalesce(bool_and(application_id IS NULL)::text, '') FROM roles WHERE name = 'admin';`)

  for (const name of ['admin', 'user']) {
    const created = await api('POST', `${adminApi}/applications/${application.id}/roles`, {
      token: admin.accessToken,
      json: { name, permissionKeys: [] },
    })
    expect([400, 403]).toContain(created.status)
  }
  expect(psql(`SELECT count(*) || ':' || coalesce(bool_and(application_id IS NULL)::text, '') FROM roles WHERE name = 'admin';`)).toBe(before)
  expect(before).toBe('1:true')
})

test('RF-021 Un administrador no edita permisos de un rol que posee', async () => {
  const holder = await seedAdmin()
  const other = await seedAdmin()
  const application = await contabilidadApplication(holder)
  const roleName = uniqueRoleName('propio')
  let role: ApplicationRoleBody | undefined
  try {
    role = await createRole(other, application, roleName, ['reportes.ver'])
    // Another admin gives the role to the first one; nobody can assign roles to themselves.
    const assigned = await api('PATCH', `${adminApi}/users/${await ownId(holder.accessToken)}`, {
      token: other.accessToken,
      json: { roles: ['user', 'admin', roleName] },
    })
    expect(assigned.status).toBe(200)

    const updated = await api('PATCH', rolePath(application.id, role.id), {
      token: holder.accessToken,
      json: { permissionKeys: ['reportes.ver', 'cierre.ejecutar'] },
    })
    expect(updated.status).toBe(403)
    const after = (await contabilidadApplication(other)).roles.find((item) => item.id === role?.id)
    expect(after?.permissionKeys).toEqual(['reportes.ver'])
  } finally {
    purgeRole(roleName)
  }
})

test('RF-021 Se rechaza un permiso de otra aplicación', async () => {
  const admin = await seedAdmin()
  const application = await contabilidadApplication(admin)
  const roleName = uniqueRoleName('ajeno')
  try {
    const role = await createRole(admin, application, roleName, ['reportes.ver'])
    const updated = await api('PATCH', rolePath(application.id, role.id), {
      token: admin.accessToken,
      json: { permissionKeys: ['reportes.ver', 'otra-app.usuarios.ver'] },
    })
    expect(updated.status).toBe(400)
    const after = (await contabilidadApplication(admin)).roles.find((item) => item.id === role.id)
    expect(after?.permissionKeys).toEqual(['reportes.ver'])
  } finally {
    purgeRole(roleName)
  }
})

test('RF-021 No se elimina un rol asignado', async () => {
  const admin = await seedAdmin()
  const employee = await inviteUser(admin)
  const application = await contabilidadApplication(admin)
  const roleName = uniqueRoleName('asignado')
  try {
    const role = await createRole(admin, application, roleName, ['reportes.ver'])
    expect((await api('PATCH', `${adminApi}/users/${employee.id}`, {
      token: admin.accessToken,
      json: { roles: ['user', roleName] },
    })).status).toBe(200)

    expect((await api('DELETE', rolePath(application.id, role.id), { token: admin.accessToken })).status).toBe(409)
    expect(roleCount(roleName)).toBe(1)
    expect(psql(`SELECT count(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE r.name = :'name';`, { name: roleName })).toBe('1')
  } finally {
    purgeRole(roleName)
  }
})

test('RF-021 Cada cambio de rol queda auditado', async () => {
  const admin = await seedAdmin()
  const application = await contabilidadApplication(admin)
  const roleName = uniqueRoleName('auditado')
  try {
    const role = await createRole(admin, application, roleName, ['reportes.ver'])
    expect((await api('PATCH', rolePath(application.id, role.id), {
      token: admin.accessToken,
      json: { permissionKeys: ['reportes.ver', 'movimientos.ver_todos'] },
    })).status).toBe(200)
    expect((await api('DELETE', rolePath(application.id, role.id), { token: admin.accessToken })).status).toBe(204)

    for (const action of ['role_created', 'role_updated', 'role_deleted']) {
      expect(psql(`SELECT count(*) FROM audit_log a
                   JOIN users actor ON actor.id = a.actor_user_id
                   WHERE actor.email = :'email' AND a.action = :'action';`, {
        email: admin.email,
        action,
      })).toBe('1')
    }
  } finally {
    purgeRole(roleName)
  }
})
