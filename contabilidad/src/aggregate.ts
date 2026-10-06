import { STATUS_LABEL, type Movement, type Status } from './ledger'

export interface StatusTotal { status: Status; label: string; count: number; total: number }
export interface CategoryTotal { category: string; count: number; total: number }
export interface MonthTotal { month: string; label: string; count: number; total: number }

const STATUS_ORDER: readonly Status[] = ['approved', 'pending', 'rejected']
const MONTH_NAMES = ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic']

/** Spanish short month label for a `YYYY-MM` key, independent of the runtime locale data. */
export function monthLabel(month: string): string {
  const [year, number] = month.split('-')
  return `${MONTH_NAMES[Number(number) - 1] ?? number} ${year}`
}

export function totalsByStatus(movements: Movement[]): StatusTotal[] {
  return STATUS_ORDER.map((status) => {
    const matching = movements.filter((movement) => movement.status === status)
    return { status, label: STATUS_LABEL[status], count: matching.length, total: matching.reduce((sum, movement) => sum + movement.amount, 0) }
  })
}

/** Largest total first; equal totals fall back to the category name so the order is stable. */
export function totalsByCategory(movements: Movement[]): CategoryTotal[] {
  const byCategory = new Map<string, CategoryTotal>()
  for (const movement of movements) {
    const entry = byCategory.get(movement.category) ?? { category: movement.category, count: 0, total: 0 }
    entry.count += 1
    entry.total += movement.amount
    byCategory.set(movement.category, entry)
  }
  return [...byCategory.values()].sort((a, b) => b.total - a.total || a.category.localeCompare(b.category, 'es'))
}

const nextMonth = (month: string): string => {
  const [year, number] = month.split('-').map(Number) as [number, number]
  return number === 12 ? `${year + 1}-01` : `${year}-${String(number + 1).padStart(2, '0')}`
}

/** Chronological totals; months without movements between the first and last one appear with zero. */
export function totalsByMonth(movements: Movement[]): MonthTotal[] {
  if (movements.length === 0) return []
  const keys = movements.map((movement) => movement.date.slice(0, 7)).sort()
  const first = keys[0] as string
  const last = keys[keys.length - 1] as string
  const result: MonthTotal[] = []
  for (let month = first; month <= last; month = nextMonth(month)) {
    const matching = movements.filter((movement) => movement.date.startsWith(month))
    result.push({ month, label: monthLabel(month), count: matching.length, total: matching.reduce((sum, movement) => sum + movement.amount, 0) })
  }
  return result
}

/** Newest first; rows on the same date are ordered by folio, highest first. Does not mutate the input. */
export function recentMovements(movements: Movement[], limit: number): Movement[] {
  return [...movements].sort((a, b) => b.date.localeCompare(a.date) || b.id.localeCompare(a.id)).slice(0, limit)
}
