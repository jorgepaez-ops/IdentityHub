import { ApiProblemError, type AdminUser } from '../../api/client'
import { connectionMessage } from '../account/PublicPages'

// Only the two directory roles are described locally; application roles and their permissions
// come from GET /admin/applications (RF-021).
export const DIRECTORY_ROLES = ['admin', 'user'] as const
export const DIRECTORY_ROLE_DESCRIPTION: Record<string, string> = {
  admin: 'Control total del Hub y de todas las aplicaciones conectadas.',
  user: 'Rol base de toda cuenta: acceso a Mi cuenta.',
}

const chipTone = (role: string) => (role === 'admin' ? 'accent' : role === 'contabilidad.senior' ? 'warn' : 'neutral')

export function RoleChips({ roles }: { roles: readonly string[] }) {
  return (
    <ul className="chips" aria-label="Roles">
      {roles.map((role) => <li className={`chip chip-${chipTone(role)}`} key={role}>{role}</li>)}
    </ul>
  )
}

export const STATUS_LABEL: Record<AdminUser['status'], string> = {
  active: 'Activo',
  locked: 'Bloqueado',
  disabled: 'Deshabilitado',
  pending_verification: 'Pendiente',
}
const statusTone: Record<AdminUser['status'], string> = { active: 'ok', locked: 'danger', disabled: 'danger', pending_verification: 'warn' }

export function StatusPill({ status }: { status: AdminUser['status'] }) {
  return <span className={`pill pill-${statusTone[status]}`}>{STATUS_LABEL[status]}</span>
}

// A 401 that reaches the page already went through the refresh-and-retry in authenticatedRequest,
// so the session is really gone.
export const isAuthFailure = (reason: unknown) => reason instanceof ApiProblemError && reason.status === 401

const FIELD_LABEL: Record<string, string> = {
  email: 'Correo electrónico',
  displayName: 'Nombre para mostrar',
  roles: 'Roles',
  status: 'Estado',
}

export const initialsOf = (user: Pick<AdminUser, 'displayName' | 'email'>) => {
  const words = user.displayName.trim().split(/\s+/).filter(Boolean)
  const letters = words.length > 1 ? `${words[0]?.[0] ?? ''}${words[1]?.[0] ?? ''}` : (words[0] ?? user.email).slice(0, 2)
  return letters.toUpperCase()
}

// Messages for one failed admin call. `byStatus` holds the copy for the statuses the OpenAPI declares;
// field errors from problem+json win over it, prefixed with a Spanish field label.
export function adminProblems(reason: unknown, byStatus: Record<number, string>, fallback: string): string[] {
  if (!(reason instanceof ApiProblemError)) return [connectionMessage]
  const fields = reason.errors.filter((item) => item.message).map((item) => `${FIELD_LABEL[item.field] ?? item.field}: ${item.message}`)
  if (fields.length > 0) return fields
  return [byStatus[reason.status] ?? fallback]
}

export function Problems({ messages }: { messages: string[] }) {
  if (messages.length === 0) return null
  return <div className="error-box" role="alert">{messages.map((message, index) => <p key={index}>{message}</p>)}</div>
}

// Directory status filter: `null` is "all" and sends no `status` parameter. It is mirrored in the URL as ?estado=.
export const STATUS_PARAM = 'estado'
export const STATUS_FILTERS: (AdminUser['status'] | null)[] = [null, 'active', 'pending_verification', 'locked', 'disabled']
export const STATUS_FILTER_LABEL: Record<AdminUser['status'] | 'all', string> = {
  all: 'Todos',
  active: 'Activos',
  pending_verification: 'Pendientes',
  locked: 'Bloqueados',
  disabled: 'Deshabilitados',
}
export const parseStatusFilter = (value: string | null): AdminUser['status'] | null =>
  STATUS_FILTERS.find((option) => option !== null && option === value) ?? null
