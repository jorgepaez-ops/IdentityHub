// Example data only: Contabilidad has no ledger backend (T13). `mine` marks the
// rows that belong to the signed-in analyst in this demo.
export type Status = 'approved' | 'pending' | 'rejected'

export interface Movement {
  id: string
  date: string
  description: string
  category: string
  amount: number
  status: Status
  mine: boolean
}

export const INITIAL_MOVEMENTS: Movement[] = [
  { id: 'M-2041', date: '2026-09-15', description: 'Nómina quincenal', category: 'Nómina', amount: 18_450_000, status: 'approved', mine: false },
  { id: 'M-2042', date: '2026-09-03', description: 'Arriendo oficina Chapinero', category: 'Arriendo', amount: 6_200_000, status: 'approved', mine: false },
  { id: 'M-2043', date: '2026-09-22', description: 'Viáticos visita cliente Cali', category: 'Viáticos', amount: 480_000, status: 'pending', mine: true },
  { id: 'M-2044', date: '2026-09-24', description: 'Pago proveedor papelería Andina', category: 'Proveedores', amount: 312_500, status: 'pending', mine: true },
  { id: 'M-2045', date: '2026-09-10', description: 'Servicio de energía sede norte', category: 'Servicios', amount: 1_140_000, status: 'approved', mine: false },
  { id: 'M-2046', date: '2026-09-12', description: 'Viáticos capacitación interna', category: 'Viáticos', amount: 210_000, status: 'rejected', mine: true },
]

export const PERIOD = 'Septiembre 2026'
export const LAST_CLOSE = 'Agosto 2026'
export const USERS_WITH_ACCESS = 12

const COP = new Intl.NumberFormat('es-CO', { style: 'currency', currency: 'COP', maximumFractionDigits: 0 })
export const formatCop = (amount: number) => COP.format(amount)

export const STATUS_LABEL: Record<Status, string> = { approved: 'Aprobado', pending: 'Pendiente', rejected: 'Rechazado' }

export const visibleMovements = (movements: Movement[], level: 'full' | 'own') => (level === 'full' ? movements : movements.filter((movement) => movement.mine))

export const totalOf = (movements: Movement[], status: Status) => movements.filter((movement) => movement.status === status).reduce((sum, movement) => sum + movement.amount, 0)
export const countOf = (movements: Movement[], status: Status) => movements.filter((movement) => movement.status === status).length

const FOLIO_PREFIX = 'M-'
const INITIAL_FOLIO_NUMBER = 2040

export function nextId(movements: Movement[]): string {
  const highest = movements.reduce((max, movement) => {
    if (!movement.id.startsWith(FOLIO_PREFIX)) return max
    const number = Number(movement.id.slice(FOLIO_PREFIX.length))
    return Number.isInteger(number) ? Math.max(max, number) : max
  }, INITIAL_FOLIO_NUMBER)
  return `${FOLIO_PREFIX}${highest + 1}`
}
