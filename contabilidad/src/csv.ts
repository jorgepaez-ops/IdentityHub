// Pure CSV helpers (RFC 4180) shared by the exports. The Hub and Contabilidad keep identical copies.

export type CsvCell = string | number | null | undefined

const BOM = '\uFEFF'
const EOL = '\r\n'
// Cells starting with these characters can run as formulas in a spreadsheet (OWASP CSV injection).
const FORMULA_START = /^[=+\-@\t\r]/

const numberText = (value: number) => (Number.isFinite(value) ? String(value) : '')
const neutralize = (value: string) => (FORMULA_START.test(value) ? `'${value}` : value)

/**
 * Text cells get a leading single quote when they could run as a formula. Numbers are written as plain
 * digits and never neutralized, so negative amounts stay numeric.
 */
export function csvCell(value: CsvCell): string {
  if (value === null || value === undefined) return ''
  const text = typeof value === 'number' ? numberText(value) : neutralize(value)
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

/** Builds the whole file: UTF-8 BOM (so Excel reads accents), a header row and CRLF-separated records. */
export function toCsv(header: readonly string[], rows: readonly (readonly CsvCell[])[]): string {
  return BOM + [header, ...rows].map((row) => row.map(csvCell).join(',')).join(EOL) + EOL
}

const pad = (value: number) => String(value).padStart(2, '0')

/** For example `usuarios-2026-10-06.csv`, using the local calendar date. */
export const csvFileName = (prefix: string, date: Date = new Date()) => `${prefix}-${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}.csv`

/** Downloads through a Blob URL (no data: URL, so the CSP is untouched) and revokes the URL afterwards. */
export function downloadCsv(fileName: string, content: string): void {
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.hidden = true
  document.body.appendChild(link)
  link.click()
  link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}
