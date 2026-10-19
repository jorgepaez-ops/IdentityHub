export const CAN_VIEW = 'productos.ver'
export const CAN_EDIT = 'productos.editar'
export const CAN_MOVE = 'movimientos.registrar'

const ROLE_NAMESPACE = 'inventario.'

const ROLE_TITLES: Record<string, string> = {
  'inventario.consulta': 'Consulta de inventario',
  'inventario.bodeguero': 'Bodeguero',
}

export interface Grants {
  view: boolean
  edit: boolean
  move: boolean
  /** Name of the role, shown next to the user. */
  roleTitle: string
  /** One sentence listing what the role allows. */
  summary: string
}

/**
 * What this client lets the person do, taken from the verified token. Membership needs a role of this
 * application; the actions come from the `permissions` claim, never from role names. It only shapes
 * the UI: a real stock backend would have to enforce the same permissions again (RF-009, RF-021).
 * Returns `null` when the person has no access at all.
 */
export function grantsFor(roles: readonly string[], permissions: readonly string[]): Grants | null {
  const own = roles.filter((role) => role.startsWith(ROLE_NAMESPACE))
  const view = permissions.includes(CAN_VIEW)
  const edit = permissions.includes(CAN_EDIT)
  const move = permissions.includes(CAN_MOVE)
  if (own.length === 0 || !(view || edit || move)) return null

  const actions: string[] = []
  if (view) actions.push('consultar el catálogo')
  if (edit) actions.push('editar productos')
  if (move) actions.push('registrar entradas y salidas de bodega')
  const known = own.find((role) => role in ROLE_TITLES)
  return {
    view,
    edit,
    move,
    roleTitle: known ? ROLE_TITLES[known] ?? known : own.join(', '),
    summary: `Tu rol permite: ${actions.join(', ')}.`,
  }
}
