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

const m = (id: string, date: string, description: string, category: string, amount: number, status: Status, mine = false): Movement => ({ id, date, description, category, amount, status, mine })

// Deterministic sample ledger (no Math.random, no Date.now) spanning April to September 2026. Past
// months hold only decided rows; September keeps pending rows for the approval flow. The folios
// M-2041..M-2046 are referenced by tests and E2E specs, and the M-1xxx folios stay below
// INITIAL_FOLIO_NUMBER so newly registered movements keep numbering from M-2047.
export const INITIAL_MOVEMENTS: Movement[] = [
  { id: 'M-2041', date: '2026-09-15', description: 'Nómina quincenal', category: 'Nómina', amount: 18_450_000, status: 'approved', mine: false },
  { id: 'M-2042', date: '2026-09-03', description: 'Arriendo oficina Chapinero', category: 'Arriendo', amount: 6_200_000, status: 'approved', mine: false },
  { id: 'M-2043', date: '2026-09-22', description: 'Viáticos visita cliente Cali', category: 'Viáticos', amount: 480_000, status: 'pending', mine: true },
  { id: 'M-2044', date: '2026-09-24', description: 'Pago proveedor papelería Andina', category: 'Proveedores', amount: 312_500, status: 'pending', mine: true },
  { id: 'M-2045', date: '2026-09-10', description: 'Servicio de energía sede norte', category: 'Servicios', amount: 1_140_000, status: 'approved', mine: false },
  { id: 'M-2046', date: '2026-09-12', description: 'Viáticos capacitación interna', category: 'Viáticos', amount: 210_000, status: 'rejected', mine: true },
  m('M-1001', '2026-04-03', 'Arriendo oficina Chapinero', 'Arriendo', 6_200_000, 'approved'),
  m('M-1002', '2026-04-05', 'Ventas de contado semana 1', 'Ventas', 9_850_000, 'approved'),
  m('M-1003', '2026-04-10', 'Servicio de energía sede norte', 'Servicios', 1_090_000, 'approved'),
  m('M-1004', '2026-04-15', 'Nómina quincenal', 'Nómina', 17_900_000, 'approved'),
  m('M-1005', '2026-04-18', 'Pago proveedor papelería Andina', 'Proveedores', 286_000, 'approved', true),
  m('M-1006', '2026-04-22', 'Viáticos visita cliente Medellín', 'Viáticos', 390_000, 'approved', true),
  m('M-1007', '2026-04-30', 'Retención en la fuente abril', 'Impuestos', 2_340_000, 'approved'),
  m('M-1008', '2026-05-03', 'Arriendo oficina Chapinero', 'Arriendo', 6_200_000, 'approved'),
  m('M-1009', '2026-05-08', 'Ventas de contado semana 2', 'Ventas', 11_200_000, 'approved'),
  m('M-1010', '2026-05-10', 'Servicio de internet y telefonía', 'Servicios', 745_000, 'approved'),
  m('M-1011', '2026-05-15', 'Nómina quincenal', 'Nómina', 17_900_000, 'approved'),
  m('M-1012', '2026-05-20', 'Pago proveedor aseo Brillo', 'Proveedores', 520_000, 'approved'),
  m('M-1013', '2026-05-24', 'Viáticos capacitación externa', 'Viáticos', 640_000, 'rejected', true),
  m('M-1014', '2026-05-31', 'IVA bimestral', 'Impuestos', 3_180_000, 'approved'),
  m('M-1015', '2026-06-03', 'Arriendo oficina Chapinero', 'Arriendo', 6_200_000, 'approved'),
  m('M-1016', '2026-06-06', 'Ventas de contado semana 1', 'Ventas', 10_400_000, 'approved'),
  m('M-1017', '2026-06-10', 'Servicio de energía sede norte', 'Servicios', 1_120_000, 'approved'),
  m('M-1018', '2026-06-15', 'Nómina quincenal', 'Nómina', 18_100_000, 'approved'),
  m('M-1019', '2026-06-19', 'Pago proveedor papelería Andina', 'Proveedores', 341_000, 'approved', true),
  m('M-1020', '2026-06-25', 'Viáticos visita cliente Bogotá', 'Viáticos', 150_000, 'approved', true),
  m('M-1021', '2026-06-30', 'Prima de servicios', 'Nómina', 9_600_000, 'approved'),
  m('M-1022', '2026-07-03', 'Arriendo oficina Chapinero', 'Arriendo', 6_200_000, 'approved'),
  m('M-1023', '2026-07-07', 'Ventas de contado semana 1', 'Ventas', 12_750_000, 'approved'),
  m('M-1024', '2026-07-10', 'Servicio de acueducto', 'Servicios', 410_000, 'approved'),
  m('M-1025', '2026-07-15', 'Nómina quincenal', 'Nómina', 18_100_000, 'approved'),
  m('M-1026', '2026-07-21', 'Pago proveedor tecnología Nexo', 'Proveedores', 2_450_000, 'rejected'),
  m('M-1027', '2026-07-27', 'Viáticos feria sectorial', 'Viáticos', 870_000, 'approved', true),
  m('M-1028', '2026-07-31', 'Impuesto de industria y comercio', 'Impuestos', 1_760_000, 'approved'),
  m('M-1029', '2026-08-03', 'Arriendo oficina Chapinero', 'Arriendo', 6_200_000, 'approved'),
  m('M-1030', '2026-08-06', 'Ventas de contado semana 1', 'Ventas', 13_300_000, 'approved'),
  m('M-1031', '2026-08-10', 'Servicio de energía sede norte', 'Servicios', 1_150_000, 'approved'),
  m('M-1032', '2026-08-15', 'Nómina quincenal', 'Nómina', 18_450_000, 'approved'),
  m('M-1033', '2026-08-20', 'Pago proveedor papelería Andina', 'Proveedores', 298_000, 'approved', true),
  m('M-1034', '2026-08-26', 'Viáticos visita cliente Cali', 'Viáticos', 455_000, 'approved', true),
  m('M-1035', '2026-08-31', 'Retención en la fuente agosto', 'Impuestos', 2_610_000, 'approved'),
  m('M-1036', '2026-09-05', 'Ventas de contado semana 1', 'Ventas', 13_900_000, 'approved'),
  m('M-1037', '2026-09-17', 'Pago proveedor aseo Brillo', 'Proveedores', 540_000, 'approved'),
  m('M-1038', '2026-09-19', 'Servicio de internet y telefonía', 'Servicios', 760_000, 'pending'),
  m('M-1039', '2026-09-26', 'Ventas de contado semana 4', 'Ventas', 14_600_000, 'pending'),
  m('M-1040', '2026-09-27', 'Viáticos transporte interno', 'Viáticos', 95_000, 'pending', true),
  m('M-1041', '2026-09-29', 'Retención en la fuente septiembre', 'Impuestos', 2_790_000, 'pending'),
]

export const PERIOD = 'Septiembre 2026'
/** First day of the open period: everything on or after it has not been closed yet. */
export const OPEN_PERIOD_START = '2026-09-01'
export const LAST_CLOSE = 'Agosto 2026'
export const USERS_WITH_ACCESS = 12

const COP = new Intl.NumberFormat('es-CO', { style: 'currency', currency: 'COP', maximumFractionDigits: 0 })
export const formatCop = (amount: number) => COP.format(amount)

export const STATUS_LABEL: Record<Status, string> = { approved: 'Aprobado', pending: 'Pendiente', rejected: 'Rechazado' }

export const visibleMovements = (movements: Movement[], level: 'full' | 'own') => (level === 'full' ? movements : movements.filter((movement) => movement.mine))

export const totalOf = (movements: Movement[], status: Status) => movements.filter((movement) => movement.status === status).reduce((sum, movement) => sum + movement.amount, 0)
export const inOpenPeriod = (movements: Movement[]) => movements.filter((movement) => movement.date >= OPEN_PERIOD_START)
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
