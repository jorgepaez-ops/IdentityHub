import { randomBytes } from 'node:crypto'
import { api } from './api'
import type { Admin } from './accounts'
import { psql } from './db'

export interface ApplicationRoleBody {
  id: string
  name: string
  permissionKeys: string[]
  assignedCount: number
}

export interface ApplicationBody {
  id: string
  clientId: string
  permissions: { key: string }[]
  roles: ApplicationRoleBody[]
}

const ROLE_NAME = /^contabilidad\.[a-z0-9][a-z0-9_-]{1,40}$/
// Seeded roles the demo and the rest of the suite rely on; cleanup must never remove them.
const SEEDED_ROLES = new Set(['contabilidad.senior', 'contabilidad.analista'])

/** A role name no other test uses, so a failed run never collides with the next one. */
export function uniqueRoleName(purpose: string): string {
  return `contabilidad.e2e-${purpose}-${randomBytes(3).toString('hex')}`
}

export async function contabilidadApplication(admin: Admin): Promise<ApplicationBody> {
  const listed = await api<ApplicationBody[]>('GET', '/api/v1/admin/applications', { token: admin.accessToken })
  if (listed.status !== 200) throw new Error(`list applications returned ${listed.status}`)
  const application = listed.body.find((item) => item.clientId === 'contabilidad')
  if (!application) throw new Error('the contabilidad application is not registered')
  return application
}

export async function createRole(
  admin: Admin,
  application: ApplicationBody,
  name: string,
  permissionKeys: string[],
): Promise<ApplicationRoleBody> {
  const created = await api<ApplicationRoleBody>('POST', `/api/v1/admin/applications/${application.id}/roles`, {
    token: admin.accessToken,
    json: { name, description: 'Rol de prueba E2E', permissionKeys },
  })
  if (created.status !== 201) throw new Error(`create role ${name} returned ${created.status}`)
  return created.body
}

/** Id of the account the token belongs to. */
export async function ownId(token: string): Promise<string> {
  const found = await api<{ id: string }>('GET', '/api/v1/me', { token })
  if (found.status !== 200) throw new Error(`/me returned ${found.status}`)
  return found.body.id
}

/** Removes a test role and its assignments straight from the database (cleanup only). */
export function purgeRole(name: string): void {
  if (!ROLE_NAME.test(name) || SEEDED_ROLES.has(name)) throw new Error(`Refusing to purge a role that is not a test contabilidad role: ${name}`)
  psql(
    `WITH r AS (SELECT id FROM roles WHERE name = :'name'),
          a AS (DELETE FROM user_roles WHERE role_id IN (SELECT id FROM r)),
          p AS (DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM r))
     DELETE FROM roles WHERE id IN (SELECT id FROM r);`,
    { name },
  )
}

export function roleCount(name: string): number {
  return Number(psql(`SELECT count(*) FROM roles WHERE name = :'name';`, { name }))
}
