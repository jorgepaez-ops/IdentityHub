import { describe, expect, it } from 'vitest'
import { nextId } from './ledger'

describe('next movement identifier', () => {
  it('TestLedgerNextId_IgnoresUnexpectedExistingIdentifiers', () => {
    const existing: Parameters<typeof nextId>[0] = [
      { id: 'not-a-folio', date: '2026-10-01', description: 'Unexpected', category: 'Other', amount: 0, status: 'pending', mine: false },
      { id: 'M-2044', date: '2026-10-01', description: 'Expected', category: 'Other', amount: 0, status: 'pending', mine: false },
    ]
    expect(nextId(existing)).toBe('M-2045')
  })
})
