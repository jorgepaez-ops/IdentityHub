export const ROLE_SENIOR = 'contabilidad.senior'
export const ROLE_ANALYST = 'contabilidad.analista'
export const ROLE_ADMIN = 'admin'

export type Level = 'full' | 'own' | 'none'

export interface Access {
  level: Level
  /** Label shown next to the user in the top bar. */
  label: string
  hint: string
  canApprove: boolean
  canClose: boolean
  showAdministration: boolean
}

const DENIED: Access = { level: 'none', label: 'Sin rol', hint: '', canApprove: false, canClose: false, showAdministration: false }

/**
 * Maps the roles of the verified access token to what this client shows. This
 * is presentation only: the Hub issues the roles and a real ledger backend would
 * have to enforce them again on the server (RF-009).
 */
export function accessFor(roles: readonly string[]): Access {
  const admin = roles.includes(ROLE_ADMIN)
  const senior = roles.includes(ROLE_SENIOR)
  const analyst = roles.includes(ROLE_ANALYST)
  if (admin) return { level: 'full', label: 'Administrador', hint: 'Acceso total, igual que en Identity Hub.', canApprove: true, canClose: true, showAdministration: true }
  if (senior) return { level: 'full', label: 'Contador senior', hint: 'Acceso completo dentro de Contabilidad.', canApprove: true, canClose: true, showAdministration: false }
  if (analyst) return { level: 'own', label: 'Analista contable', hint: 'Este rol solo ve y crea sus propios movimientos.', canApprove: false, canClose: false, showAdministration: false }
  return DENIED
}
