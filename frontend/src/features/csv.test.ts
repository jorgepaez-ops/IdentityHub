import { afterEach, describe, expect, it, vi } from 'vitest'
import { csvCell, csvFileName, downloadCsv, toCsv } from './csv'

const BOM = '﻿'

afterEach(() => {
  vi.restoreAllMocks()
  Reflect.deleteProperty(URL, 'createObjectURL')
  Reflect.deleteProperty(URL, 'revokeObjectURL')
})

describe('csv helper', () => {
  it('TestRF010_CsvQuotesFieldsWithCommasQuotesAndLineBreaks', () => {
    expect(csvCell('plain')).toBe('plain')
    expect(csvCell('a,b')).toBe('"a,b"')
    expect(csvCell('say "hi"')).toBe('"say ""hi"""')
    expect(csvCell('two\nlines')).toBe('"two\nlines"')
    expect(csvCell('two\r\nlines')).toBe('"two\r\nlines"')
  })

  it('TestRF010_CsvUsesCrlfBetweenRecordsAndStartsWithBom', () => {
    const text = toCsv(['a', 'b'], [['1', '2'], ['3', '4']])
    expect(text).toBe(`${BOM}a,b\r\n1,2\r\n3,4\r\n`)
    expect(text.startsWith(BOM)).toBe(true)
  })

  it('TestRF010_CsvKeepsUnicodeAndEmptyCells', () => {
    expect(toCsv(['Descripción'], [['Papelería ñandú ✓'], [null], [undefined], ['']])).toBe(`${BOM}Descripción\r\nPapelería ñandú ✓\r\n\r\n\r\n\r\n`)
  })

  it('TestRF010_CsvWithNoRowsKeepsOnlyTheHeader', () => {
    expect(toCsv(['a', 'b'], [])).toBe(`${BOM}a,b\r\n`)
  })

  it.each(['=1+1', '+SUM(A1)', '-2+3', '@cmd', '\tTab', '\rReturn'])('TestRF010_CsvNeutralizesFormulaPrefix %j', (value) => {
    const cell = csvCell(value)
    expect(cell.replace(/^"/, '').startsWith(`'${value[0]}`)).toBe(true)
  })

  it('TestRF010_CsvNeutralizesBeforeQuoting', () => {
    expect(csvCell('=A1,B1')).toBe('"\'=A1,B1"')
  })

  it('TestRF010_CsvLeavesNumbersNumeric', () => {
    expect(csvCell(-1500)).toBe('-1500')
    expect(csvCell(42.5)).toBe('42.5')
    expect(csvCell(Number.NaN)).toBe('')
    expect(csvCell(Number.POSITIVE_INFINITY)).toBe('')
  })

  it('TestRF010_CsvFileNameUsesTheLocalDate', () => {
    expect(csvFileName('usuarios', new Date(2026, 9, 6, 23, 30))).toBe('usuarios-2026-10-06.csv')
    expect(csvFileName('movimientos', new Date(2026, 0, 5))).toBe('movimientos-2026-01-05.csv')
  })

  it('TestRF010_DownloadUsesABlobAnchorAndRevokesTheUrl', async () => {
    const created: Blob[] = []
    const createObjectURL = vi.fn((blob: Blob) => {
      created.push(blob)
      return 'blob:test'
    })
    const revokeObjectURL = vi.fn()
    Object.assign(URL, { createObjectURL, revokeObjectURL })
    const clicked: { href: string; download: string }[] = []
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clicked.push({ href: this.getAttribute('href') ?? '', download: this.download })
    })
    downloadCsv('x-2026-10-06.csv', 'hello')
    expect(clicked).toEqual([{ href: 'blob:test', download: 'x-2026-10-06.csv' }])
    expect(created[0]?.type).toContain('text/csv')
    expect(document.querySelector('a[download]')).toBeNull()
    await vi.waitFor(() => expect(revokeObjectURL).toHaveBeenCalledWith('blob:test'))
  })
})
