import { STATUS_LABEL, type Movement } from './ledger'

// The movements ledger export: one column table drives both the header and the rows.

interface Column {
  title: string
  /** Free text can come from users, so it is neutralized; ids, dates and amounts are ours. */
  value: (movement: Movement) => string | number
  untrusted?: boolean
}

const COLUMNS: readonly Column[] = [
  { title: 'Folio', value: (m) => m.id },
  { title: 'Fecha', value: (m) => m.date },
  { title: 'Descripción', value: (m) => m.description, untrusted: true },
  { title: 'Categoría', value: (m) => m.category, untrusted: true },
  { title: 'Monto', value: (m) => m.amount },
  { title: 'Estado', value: (m) => STATUS_LABEL[m.status] },
]

const BYTE_ORDER_MARK = '\uFEFF'
const LINE_END = '\r\n'
const REVOKE_AFTER_MS = 30_000

// A spreadsheet runs a cell as a formula when it opens with one of these (OWASP CSV injection).
const FORMULA_LEAD = /^(?=[=+\-@\t\r])/
// Anything that forces RFC 4180 quoting; the quote itself is doubled in the same pass.
const SPECIAL = /[",\r\n]/

function encodeField(raw: string | number, untrusted: boolean): string {
  // A non-finite amount (NaN, Infinity) is not a value a spreadsheet can use, so the cell stays empty.
  if (typeof raw === 'number' && !Number.isFinite(raw)) return ''
  const text = untrusted ? String(raw).replace(FORMULA_LEAD, "'") : String(raw)
  return SPECIAL.test(text) ? `"${text.replaceAll('"', '""')}"` : text
}

export function movementsCsv(movements: readonly Movement[]): string {
  const header = COLUMNS.map((column) => encodeField(column.title, false)).join(',')
  const body = movements.map((movement) => COLUMNS.map((column) => encodeField(column.value(movement), column.untrusted === true)).join(','))
  return BYTE_ORDER_MARK + [header, ...body].map((line) => line + LINE_END).join('')
}

export function exportFileName(now: Date): string {
  const [month, day] = [now.getMonth() + 1, now.getDate()].map((part) => `${part}`.padStart(2, '0'))
  return `movimientos-${now.getFullYear()}-${month}-${day}.csv`
}

/** Hands the file to the browser through a Blob URL, which keeps the CSP free of data: URLs. */
export function exportMovements(movements: readonly Movement[], now: Date = new Date()): void {
  const url = URL.createObjectURL(new Blob([movementsCsv(movements)], { type: 'text/csv;charset=utf-8' }))
  const anchor = Object.assign(document.createElement('a'), { href: url, download: exportFileName(now), hidden: true })
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  // Browsers may begin the download after click() returns, so keep the URL alive for a while.
  window.setTimeout(() => URL.revokeObjectURL(url), REVOKE_AFTER_MS)
}
