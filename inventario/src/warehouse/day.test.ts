import { describe, expect, it } from 'vitest'
import { localDay } from './day'

describe('localDay', () => {
  it('TestRF021_LateInTheEveningStillReportsTheLocalDate', () => {
    // 23:30 in Bogota (UTC-5) is already the next day in UTC.
    expect(localDay(new Date(2026, 9, 19, 23, 30))).toBe('2026-10-19')
  })

  it('TestRF021_EarlyInTheMorningStillReportsTheLocalDate', () => {
    expect(localDay(new Date(2026, 0, 5, 0, 15))).toBe('2026-01-05')
  })

  it('TestRF021_MonthAndDayArePaddedToTwoDigits', () => {
    expect(localDay(new Date(2026, 2, 7, 12))).toBe('2026-03-07')
  })
})
