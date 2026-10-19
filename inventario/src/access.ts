export const PERMISSION_VIEW = 'productos.ver'
export const PERMISSION_EDIT = 'productos.editar'
export const PERMISSION_MOVE = 'movimientos.registrar'

/** Every role of this application is namespaced; roles of other applications (admin, user) do not grant membership. */
export const ROLE_PREFIX = 'inventario.'

export interface Access {
  /** True when the token carries an Inventario role and at least one Inventario permission. */
  granted: boolean
  canView: boolean
  canEdit: boolean
  canMove: boolean
  /** Label shown next to the user in the top bar. */
  label: string
  hint: string
}

const DENIED: Access = { granted: false, canView: false, canEdit: false, canMove: false, label: 'Sin rol', hint: '' }

const SEEDED_LABELS: Record<string, string> = {
  'inventario.consulta': 'Consulta de inventario',
  'inventario.bodeguero': 'Bodeguero',
}

/**
 * Maps the verified access token to what this client shows. Membership needs an
 * Inventario role; what the user can do comes from the `permissions` claim, not
 * from role names. This is presentation only: the Hub issues roles and
 * permissions and a real stock backend would have to enforce them again (RF-009, RF-021).
 */
export function accessFor(permissions: readonly string[], allRoles: readonly string[]): Access {
  const roles = allRoles.filter((role) => role.startsWith(ROLE_PREFIX))
  if (roles.length === 0) return DENIED
  const canView = permissions.includes(PERMISSION_VIEW)
  const canEdit = permissions.includes(PERMISSION_EDIT)
  const canMove = permissions.includes(PERMISSION_MOVE)
  if (!canView && !canEdit && !canMove) return DENIED

  const capabilities = [
    canView && 'consultar el catálogo',
    canEdit && 'editar productos',
    canMove && 'registrar entradas y salidas de bodega',
  ].filter((item): item is string => typeof item === 'string')
  const seeded = roles.find((role) => role in SEEDED_LABELS)
  const label = seeded ? (SEEDED_LABELS[seeded] as string) : roles.join(', ')
  return { granted: true, canView, canEdit, canMove, label, hint: `Tu rol permite: ${capabilities.join(', ')}.` }
}
