import { afterEach, describe, expect, it, vi } from 'vitest'
import { exportFileName, exportMovements, movementsCsv } from './exportMovements'
import type { Movement } from './ledger'

const movement = (overrides: Partial<Movement> = {}): Movement => ({
  id: 'M-1', date: '2026-10-01', description: 'Papel', category: 'Papelería', amount: 100, status: 'approved', mine: true, ...overrides,
})

const lines = (csv: string) => csv.replace(/^\uFEFF/, '').split('\r\n')

describe('movements export content', () => {
  it('TestRF021_ExportHeaderAndStatusLabelsAreInSpanish', () => {
    const csv = movementsCsv([movement({ status: 'pending' })])
    expect(lines(csv)).toEqual(['Folio,Fecha,Descripción,Categoría,Monto,Estado', 'M-1,2026-10-01,Papel,Papelería,100,Pendiente', ''])
  })

  it('TestRF021_ExportStartsWithBomAndUsesCrlfOnly', () => {
    const csv = movementsCsv([movement(), movement({ id: 'M-2' })])
    expect(csv.charCodeAt(0)).toBe(0xfeff)
    expect(csv.endsWith('\r\n')).toBe(true)
    expect(csv.replace(/\r\n/g, '')).not.toMatch(/[\r\n]/)
  })

  it('TestRF021_ExportQuotesCommasQuotesAndLineBreaks', () => {
    const csv = movementsCsv([movement({ description: 'Café, "premium"\nlinea 2' })])
    expect(csv).toContain('"Café, ""premium""\nlinea 2"')
  })

  it.each(['=SUM(A1)', '+1', '-2', '@cmd', '\tcell', '\rcell'])('TestRF021_ExportNeutralizesFormulaStart_%j', (text) => {
    const csv = movementsCsv([movement({ description: text, category: text })])
    // Description and category are adjacent columns; each must come out as the neutralized, quoted-if-needed field.
    const neutralized = `'${text}`
    const field = /[",\r\n]/.test(neutralized) ? `"${neutralized}"` : neutralized
    expect(csv).toContain(`,${field},${field},`)
  })

  it('TestRF021_ExportLeavesNonFiniteAmountsEmpty', () => {
    const [, nan, infinite] = lines(movementsCsv([movement({ amount: Number.NaN }), movement({ id: 'M-2', amount: Number.POSITIVE_INFINITY })]))
    expect(nan).not.toContain('NaN')
    expect(infinite).not.toContain('Infinity')
    expect(nan).toMatch(/,,[^,]+$/)
  })

  it('TestRF021_ExportLeavesAmountsAsPlainNumbers', () => {
    const csv = movementsCsv([movement({ amount: -300 }), movement({ id: 'M-2', amount: 1234567.5 })])
    const [, first, second] = lines(csv)
    expect(first).toContain(',-300,')
    expect(second).toContain(',1234567.5,')
    expect(first).not.toContain("'-300")
  })

  it('TestRF021_ExportOfNoMovementsIsOnlyTheHeader', () => {
    expect(lines(movementsCsv([]))).toEqual(['Folio,Fecha,Descripción,Categoría,Monto,Estado', ''])
  })
})

describe('movements export download', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    Reflect.deleteProperty(URL, 'createObjectURL')
    Reflect.deleteProperty(URL, 'revokeObjectURL')
  })

  it('TestRF021_ExportFileNameCarriesTheLocalDate', () => {
    expect(exportFileName(new Date(2026, 0, 5))).toBe('movimientos-2026-01-05.csv')
    expect(exportFileName(new Date(2026, 11, 31, 23, 59))).toBe('movimientos-2026-12-31.csv')
  })

  it('TestRF021_ExportDownloadsABlobAndRevokesItAfterThirtySeconds', () => {
    vi.useFakeTimers()
    const createObjectURL = vi.fn(() => 'blob:abc')
    const revokeObjectURL = vi.fn()
    Object.assign(URL, { createObjectURL, revokeObjectURL })
    let clicked: { name: string; href: string; hidden: boolean } | undefined
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clicked = { name: this.download, href: this.getAttribute('href') ?? '', hidden: this.hidden }
      expect(document.body.contains(this)).toBe(true)
    })

    exportMovements([movement()], new Date(2026, 9, 6))

    const blob = (createObjectURL.mock.calls[0] as unknown as [Blob])[0]
    expect(blob.type).toBe('text/csv;charset=utf-8')
    expect(clicked).toEqual({ name: 'movimientos-2026-10-06.csv', href: 'blob:abc', hidden: true })
    expect(document.querySelector('a[download]')).toBeNull()
    vi.advanceTimersByTime(29_999)
    expect(revokeObjectURL).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:abc')
  })
})
