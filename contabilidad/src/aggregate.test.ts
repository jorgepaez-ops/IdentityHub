import { describe, expect, it, vi } from 'vitest'
import { monthLabel, recentMovements, totalsByCategory, totalsByMonth, totalsByStatus } from './aggregate'
import { INITIAL_MOVEMENTS, type Movement } from './ledger'

const row = (id: string, date: string, category: string, amount: number, status: Movement['status']): Movement => ({ id, date, description: `Row ${id}`, category, amount, status, mine: false })

const SAMPLE: Movement[] = [
  row('A', '2026-01-10', 'Nómina', 100, 'approved'),
  row('B', '2026-01-20', 'Ventas', 50, 'pending'),
  row('C', '2026-03-05', 'Nómina', 30, 'rejected'),
  row('D', '2026-03-05', 'Ventas', 70, 'approved'),
]

describe('ledger aggregation', () => {
  it('TestRF021_AggregateByStatusSumsAndCountsEveryStatusInFixedOrder', () => {
    expect(totalsByStatus(SAMPLE)).toEqual([
      { status: 'approved', label: 'Aprobado', count: 2, total: 170 },
      { status: 'pending', label: 'Pendiente', count: 1, total: 50 },
      { status: 'rejected', label: 'Rechazado', count: 1, total: 30 },
    ])
  })

  it('TestRF021_AggregateByCategorySortsByTotalDescendingThenName', () => {
    expect(totalsByCategory(SAMPLE)).toEqual([
      { category: 'Nómina', count: 2, total: 130 },
      { category: 'Ventas', count: 2, total: 120 },
    ])
    const tied = [row('X', '2026-01-01', 'Beta', 10, 'approved'), row('Y', '2026-01-01', 'Alfa', 10, 'approved')]
    expect(totalsByCategory(tied).map((entry) => entry.category)).toEqual(['Alfa', 'Beta'])
  })

  it('TestRF021_AggregateByMonthIsChronologicalAndFillsEmptyMonthsWithZero', () => {
    expect(totalsByMonth([...SAMPLE].reverse())).toEqual([
      { month: '2026-01', label: 'ene 2026', count: 2, total: 150 },
      { month: '2026-02', label: 'feb 2026', count: 0, total: 0 },
      { month: '2026-03', label: 'mar 2026', count: 2, total: 100 },
    ])
  })

  it('TestRF021_TotalsByMonthIgnoresMalformedDatesInsteadOfLooping', () => {
    const months = totalsByMonth([row('X', '', 'Ventas', 5, 'pending'), row('Y', 'not-a-date', 'Ventas', 6, 'pending'), row('Z', '2026-13-01', 'Ventas', 7, 'pending'), row('A', '2026-02-10', 'Ventas', 9, 'approved')])
    expect(months.map((month) => [month.month, month.total])).toEqual([['2026-02', 9]])
    expect(totalsByMonth([row('X', '', 'Ventas', 5, 'pending')])).toEqual([])
  })

  it('TestRF021_AggregateMonthsCrossYearBoundaries', () => {
    const months = totalsByMonth([row('P', '2025-11-30', 'Ventas', 1, 'approved'), row('Q', '2026-02-01', 'Ventas', 2, 'approved')])
    expect(months.map((entry) => entry.month)).toEqual(['2025-11', '2025-12', '2026-01', '2026-02'])
  })

  it('TestRF021_AggregatesOfEmptyInputAreEmptyOrZero', () => {
    expect(totalsByStatus([]).map((entry) => [entry.count, entry.total])).toEqual([[0, 0], [0, 0], [0, 0]])
    expect(totalsByCategory([])).toEqual([])
    expect(totalsByMonth([])).toEqual([])
    expect(recentMovements([], 5)).toEqual([])
  })

  it('TestRF021_RecentMovementsAreNewestFirstWithIdAsTieBreakAndDoNotMutateTheInput', () => {
    const input = [...SAMPLE]
    expect(recentMovements(input, 3).map((entry) => entry.id)).toEqual(['D', 'C', 'B'])
    expect(input).toEqual(SAMPLE)
    expect(recentMovements(SAMPLE, 99)).toHaveLength(SAMPLE.length)
  })

  it('TestRF021_MonthLabelIsSpanishAndIndependentOfTheRuntimeLocale', () => {
    expect(monthLabel('2026-09')).toBe('sep 2026')
    expect(monthLabel('2026-12')).toBe('dic 2026')
  })
})

describe('sample ledger', () => {
  it('TestRF021_SampleLedgerIsRealisticDeterministicAndConsistent', () => {
    expect(INITIAL_MOVEMENTS.length).toBeGreaterThanOrEqual(30)
    expect(INITIAL_MOVEMENTS.length).toBeLessThanOrEqual(60)
    expect(new Set(INITIAL_MOVEMENTS.map((movement) => movement.id)).size).toBe(INITIAL_MOVEMENTS.length)
    expect(new Set(INITIAL_MOVEMENTS.map((movement) => movement.date.slice(0, 7))).size).toBeGreaterThanOrEqual(4)
    expect(new Set(INITIAL_MOVEMENTS.map((movement) => movement.category)).size).toBeGreaterThanOrEqual(5)
    for (const movement of INITIAL_MOVEMENTS) {
      expect(movement.category).not.toBe('')
      expect(movement.date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
      expect(movement.amount).toBeGreaterThan(0)
    }
    // Months before the open period are already decided.
    expect(INITIAL_MOVEMENTS.filter((movement) => movement.date < '2026-09-01' && movement.status === 'pending')).toEqual([])
    // The original folios keep their meaning for the access tests.
    expect(INITIAL_MOVEMENTS.filter((movement) => movement.mine && movement.id.startsWith('M-204')).map((movement) => movement.id)).toEqual(['M-2043', 'M-2044', 'M-2046'])
  })

  it('TestRF021_SampleLedgerDoesNotDependOnTheClockOrRandomness', async () => {
    const first = JSON.stringify(INITIAL_MOVEMENTS)
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2031-03-04T00:00:00Z'))
    vi.spyOn(Math, 'random').mockReturnValue(0.123)
    vi.resetModules()
    try {
      const fresh = await import('./ledger')
      expect(JSON.stringify(fresh.INITIAL_MOVEMENTS)).toBe(first)
    } finally {
      vi.useRealTimers()
      vi.restoreAllMocks()
    }
  })
})
