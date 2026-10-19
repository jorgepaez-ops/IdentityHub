import { describe, expect, it } from 'vitest'
import { formatPesos, nextFolio, post, revise, seedWarehouse } from './ledger'

const stockOf = (warehouse: ReturnType<typeof seedWarehouse>, sku: string) => warehouse.items.find((item) => item.sku === sku)?.stock

describe('demo warehouse', () => {
  it('TestRF021_AnEntryAddsAndAnExitSubtractsWithoutTouchingTheInput', () => {
    const start = seedWarehouse()
    const afterEntry = post(start, 'P-1004', 'in', 6, '2026-10-19')
    expect(afterEntry.ok && stockOf(afterEntry.warehouse, 'P-1004')).toBe(20)
    const afterExit = afterEntry.ok ? post(afterEntry.warehouse, 'P-1004', 'out', 20, '2026-10-19') : afterEntry
    expect(afterExit.ok && stockOf(afterExit.warehouse, 'P-1004')).toBe(0)
    expect(stockOf(start, 'P-1004')).toBe(14)
    expect(start.entries).toHaveLength(3)
  })

  it('TestRF021_AnExitNeverLeavesTheStockBelowZero', () => {
    expect(post(seedWarehouse(), 'P-1005', 'out', 10, '2026-10-19')).toEqual({ ok: false, problem: 'not_enough_stock' })
  })

  it.each([0, -3, 1.5, Number.NaN])('TestRF021_QuantityOf%sIsRefused', (quantity) => {
    expect(post(seedWarehouse(), 'P-1001', 'in', quantity, '2026-10-19')).toEqual({ ok: false, problem: 'bad_quantity' })
  })

  it('TestRF021_AnUnknownProductIsRefused', () => {
    expect(post(seedWarehouse(), 'P-9999', 'in', 1, '2026-10-19')).toEqual({ ok: false, problem: 'unknown_item' })
  })

  it('TestRF021_FoliosFollowTheHighestOneIssued', () => {
    const start = seedWarehouse()
    expect(nextFolio(start.entries)).toBe('MV-004')
    expect(nextFolio([])).toBe('MV-001')
    const posted = post(start, 'P-1001', 'in', 1, '2026-10-19')
    expect(posted.ok && posted.entry).toMatchObject({ folio: 'MV-004', day: '2026-10-19', direction: 'in' })
  })

  it('TestRF021_ReviseChangesOnlyNameAndPriceOfThatProduct', () => {
    const revised = revise(seedWarehouse(), 'P-1002', 'Cinta reforzada', 7_500)
    expect(revised.items.find((item) => item.sku === 'P-1002')).toMatchObject({ name: 'Cinta reforzada', price: 7_500, stock: 340 })
    expect(revised.items.find((item) => item.sku === 'P-1001')?.name).toContain('Tornillo')
  })

  it('TestRF021_PricesAreShownInColombianPesos', () => {
    expect(formatPesos(24_500)).toMatch(/24\.500/)
  })
})
