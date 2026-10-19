import { describe, expect, it } from 'vitest'
import { INITIAL_MOVEMENTS, INITIAL_PRODUCTS, applyMovement, nextMovementId } from './stock'

describe('demo warehouse', () => {
  it('TestRF020_EntryAddsToStockAndExitSubtracts', () => {
    const entry = applyMovement(INITIAL_PRODUCTS, 'P-1001', 'in', 5)
    expect(entry.ok && entry.products.find((p) => p.sku === 'P-1001')?.stock).toBe(125)
    const exit = applyMovement(INITIAL_PRODUCTS, 'P-1001', 'out', 20)
    expect(exit.ok && exit.products.find((p) => p.sku === 'P-1001')?.stock).toBe(100)
  })

  it('TestRF020_ExitCannotTakeStockBelowZero', () => {
    expect(applyMovement(INITIAL_PRODUCTS, 'P-1005', 'out', 10)).toEqual({ ok: false, reason: 'insufficient_stock' })
  })

  it.each([0, -3, 1.5, Number.NaN])('TestRF020_RejectsInvalidQuantity %s', (quantity) => {
    expect(applyMovement(INITIAL_PRODUCTS, 'P-1001', 'in', quantity)).toEqual({ ok: false, reason: 'invalid_quantity' })
  })

  it('TestRF020_RejectsUnknownProductAndLeavesInputUntouched', () => {
    expect(applyMovement(INITIAL_PRODUCTS, 'P-9999', 'in', 1)).toEqual({ ok: false, reason: 'unknown_product' })
    expect(INITIAL_PRODUCTS.find((p) => p.sku === 'P-1001')?.stock).toBe(120)
  })

  it('TestRF020_FoliosFollowTheLastMovement', () => {
    expect(nextMovementId(INITIAL_MOVEMENTS)).toBe('MV-004')
    expect(nextMovementId([])).toBe('MV-001')
  })
})
