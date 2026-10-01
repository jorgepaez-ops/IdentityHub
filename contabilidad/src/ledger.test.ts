import { describe, expect, it } from 'vitest'
import { nextId } from './ledger'

describe('next movement identifier', () => {
  it('TestRF009_IgnoresUnexpectedExistingIdentifiers', () => {
    expect(nextId([{ id: 'not-a-folio' }, { id: 'M-2044' }] as never)).toBe('M-2045')
  })
})
