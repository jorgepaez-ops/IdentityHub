export const ROLE_SENIOR = 'contabilidad.senior'
export const ROLE_ANALYST = 'contabilidad.analista'

export type Level = 'full' | 'own' | 'none'

export interface Access {
  level: Level
  /** Label shown next to the user in the top bar. */
  label: string
  hint: string
  canApprove: boolean
  canClose: boolean
}

const DENIED: Access = { level: 'none', label: 'Sin rol', hint: '', canApprove: false, canClose: false }

/**
 * Maps the roles of the verified access token to what this client shows. This
 * is presentation only: the Hub issues the roles and a real ledger backend would
 * have to enforce them again on the server (RF-009).
 */
export function accessFor(roles: readonly string[]): Access {
  const senior = roles.includes(ROLE_SENIOR)
  const analyst = roles.includes(ROLE_ANALYST)
  if (senior) return { level: 'full', label: 'Contador senior', hint: 'Acceso completo dentro de Contabilidad.', canApprove: true, canClose: true }
  if (analyst) return { level: 'own', label: 'Analista contable', hint: 'Este rol solo ve y crea sus propios movimientos.', canApprove: false, canClose: false }
  return DENIED
}
