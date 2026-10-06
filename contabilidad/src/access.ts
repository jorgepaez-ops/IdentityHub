export const ROLE_SENIOR = 'contabilidad.senior'
export const ROLE_ANALYST = 'contabilidad.analista'

export const PERMISSION_REGISTER = 'movimientos.registrar'
export const PERMISSION_VIEW_ALL = 'movimientos.ver_todos'
export const PERMISSION_APPROVE = 'movimientos.aprobar'
export const PERMISSION_CLOSE = 'cierre.ejecutar'
export const PERMISSION_REPORTS = 'reportes.ver'

/** Every role of this application is namespaced; roles of other applications (admin, user) do not grant membership. */
export const ROLE_PREFIX = 'contabilidad.'

export type Level = 'full' | 'own' | 'none'

export interface Access {
  level: Level
  /** Label shown next to the user in the top bar. */
  label: string
  hint: string
  canRegister: boolean
  canApprove: boolean
  canClose: boolean
  canSeeReports: boolean
}

const DENIED: Access = { level: 'none', label: 'Sin rol', hint: '', canRegister: false, canApprove: false, canClose: false, canSeeReports: false }

const SEEDED_LABELS: Record<string, string> = {
  [ROLE_SENIOR]: 'Contador senior',
  [ROLE_ANALYST]: 'Analista contable',
}

function labelFor(roles: readonly string[]): string {
  const seeded = roles.find((role) => role in SEEDED_LABELS)
  if (seeded) return SEEDED_LABELS[seeded] as string
  return roles.join(', ')
}

const SEEDED_HINTS: Record<string, { permissions: readonly string[]; hint: string }> = {
  [ROLE_SENIOR]: { permissions: [PERMISSION_REGISTER, PERMISSION_VIEW_ALL, PERMISSION_APPROVE, PERMISSION_CLOSE, PERMISSION_REPORTS], hint: 'Acceso completo dentro de Contabilidad.' },
  [ROLE_ANALYST]: { permissions: [PERMISSION_REGISTER, PERMISSION_REPORTS], hint: 'Este rol solo ve y crea sus propios movimientos.' },
}

/** The seeded roles keep their familiar hint while they carry exactly their seeded permissions. */
function seededHintFor(roles: readonly string[], permissions: readonly string[]): string | undefined {
  const seeded = roles.find((role) => role in SEEDED_HINTS)
  if (!seeded || roles.length !== 1) return undefined
  const expected = (SEEDED_HINTS[seeded] as { permissions: readonly string[]; hint: string })
  const same = expected.permissions.length === permissions.length && expected.permissions.every((permission) => permissions.includes(permission))
  return same ? expected.hint : undefined
}

/**
 * Maps the verified access token to what this client shows. Membership is still
 * decided by holding at least one role of this application; what the user can do
 * comes from the `permissions` claim, not from role names. This is presentation
 * only: the Hub issues roles and permissions and a real ledger backend would have
 * to enforce them again on the server (RF-009, RF-021).
 */
export function accessFor(permissions: readonly string[], allRoles: readonly string[]): Access {
  const roles = allRoles.filter((role) => role.startsWith(ROLE_PREFIX))
  if (roles.length === 0) return DENIED
  const has = (permission: string) => permissions.includes(permission)
  const canRegister = has(PERMISSION_REGISTER)
  const canApprove = has(PERMISSION_APPROVE)
  const canClose = has(PERMISSION_CLOSE)
  const canSeeReports = has(PERMISSION_REPORTS)
  const level: Level = has(PERMISSION_VIEW_ALL) ? 'full' : 'own'

  const capabilities = [
    level === 'full' ? 'ver todos los movimientos' : 'ver tus propios movimientos',
    canRegister && 'registrar movimientos',
    canApprove && 'aprobar o rechazar movimientos',
    canClose && 'ejecutar el cierre contable',
    canSeeReports && 'ver el resumen',
  ].filter((item): item is string => typeof item === 'string')
  const seededHint = seededHintFor(roles, permissions)
  const hint = seededHint ?? `Tu rol permite: ${capabilities.join(', ')}.`

  return { level, label: labelFor(roles), hint, canRegister, canApprove, canClose, canSeeReports }
}
