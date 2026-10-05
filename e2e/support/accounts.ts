import { createHash, randomBytes } from 'node:crypto'
import { api, loginWithMfa } from './api'
import { assertEmail, assertHex, psql } from './db'
import { extractLink, mailIds, waitForMail } from './mailpit'

// Random per run: every test creates its own accounts, so no fixed credential lives in the repo.
export const strongPassword = `e2e-${randomBytes(18).toString('base64url')}`

export function uniqueEmail(purpose: string): string {
  const slug = purpose.toLowerCase().replace(/[^a-z0-9]+/g, '-')
  return assertEmail(`e2e-${slug}-${Date.now()}-${randomBytes(3).toString('hex')}@example.test`)
}

export function tokenFromLink(link: string): string {
  const token = new URL(link).searchParams.get('token')
  if (!token) throw new Error('Invitation link has no token')
  return token
}

export interface Admin {
  email: string
  accessToken: string
}

/**
 * Creates a private admin: a pending user is seeded in SQL with an invitation token we know, then the
 * invitation is accepted through the real API and the admin signs in with MFA read from Mailpit.
 */
export async function seedAdmin(): Promise<Admin> {
  const email = uniqueEmail('admin')
  const raw = randomBytes(32)
  const hash = createHash('sha256').update(raw).digest('hex')
  psql(
    `WITH u AS (
       INSERT INTO users (email, password_hash, display_name)
       VALUES (:'email', '$argon2id$e2e-placeholder', 'E2E Admin') RETURNING id
     ), r AS (
       INSERT INTO user_roles (user_id, role_id)
       SELECT u.id, roles.id FROM u, roles WHERE roles.name IN ('admin', 'user') RETURNING 1
     )
     INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at)
     SELECT id, decode(:'hash', 'hex'), 'invitation', now() + interval '1 hour' FROM u;`,
    { email: assertEmail(email), hash: assertHex(hash) },
  )
  const accepted = await api('POST', '/api/v1/auth/invitations/accept', {
    json: { token: raw.toString('base64url'), password: strongPassword },
  })
  if (accepted.status !== 204) throw new Error(`seed admin invitation returned ${accepted.status}`)
  return { email, accessToken: await loginWithMfa(email, strongPassword) }
}

/** Invites a new user through the admin API and returns the raw token read from the emailed link. */
export async function inviteUser(
  admin: Admin,
  roles: string[] = ['user'],
  email = uniqueEmail('invitee'),
): Promise<{ id: string; email: string; token: string; link: string }> {
  const existingMailIds = await mailIds(email)
  const since = new Date()
  const created = await api<{ id: string }>('POST', '/api/v1/admin/users', {
    token: admin.accessToken,
    json: { email, displayName: 'E2E User', roles },
  })
  if (created.status !== 201) throw new Error(`invite returned ${created.status}`)
  const mail = await waitForMail(email, 'invited', since, existingMailIds)
  const link = extractLink(mail.text)
  return { id: created.body.id, email, token: tokenFromLink(link), link }
}

/** Creates an isolated active account through the actual invitation endpoint. */
export async function createActiveUser(admin: Admin, roles: string[] = ['user']): Promise<string> {
  const invitee = await inviteUser(admin, roles)
  const accepted = await api('POST', '/api/v1/auth/invitations/accept', {
    json: { token: invitee.token, password: strongPassword },
  })
  if (accepted.status !== 204) throw new Error(`invite acceptance returned ${accepted.status}`)
  return invitee.email
}
