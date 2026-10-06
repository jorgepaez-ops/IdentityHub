import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { accessFor } from '../access'
import { recentMovements, totalsByCategory, totalsByMonth, totalsByStatus } from '../aggregate'
import { INITIAL_MOVEMENTS, formatCop, visibleMovements } from '../ledger'
import { SummaryView } from './Summary'

const auditor = accessFor(['movimientos.ver_todos', 'reportes.ver'], ['contabilidad.auditor'])
const analyst = accessFor(['movimientos.registrar', 'reportes.ver'], ['contabilidad.analista'])

const chart = (name: RegExp) => screen.getByRole('img', { name })
const figureOf = (name: RegExp) => chart(name).closest('figure') as HTMLElement
const dataTable = (figure: HTMLElement) => within(figure).getByRole('table')
const cells = (table: HTMLElement) => within(table).getAllByRole('row').slice(1).map((row) => [...row.querySelectorAll('th,td')].map((cell) => cell.textContent))

describe('summary charts', () => {
  it('TestRF021_SummaryRendersThreeChartsEachWithACaptionAndAnAccessibleName', () => {
    render(<SummaryView access={auditor} movements={INITIAL_MOVEMENTS} />)
    for (const name of [/Monto por estado/, /Monto por categoría/, /Monto por mes/]) {
      const figure = figureOf(name)
      expect(figure.querySelector('figcaption')).not.toBeNull()
      expect(chart(name).getAttribute('aria-label')).toMatch(/\d/)
    }
  })

  it('TestRF021_ChartSvgsAreNotIconsAndCarryNoInlineStyles', () => {
    const { container } = render(<SummaryView access={auditor} movements={INITIAL_MOVEMENTS} />)
    const svgs = [...container.querySelectorAll('figure svg')]
    expect(svgs).toHaveLength(3)
    for (const svg of svgs) expect(svg.classList.contains('icon')).toBe(false)
    expect(container.querySelectorAll('[style]')).toHaveLength(0)
    expect(svgs.every((svg) => svg.getAttribute('viewBox'))).toBe(true)
  })

  it('TestRF021_StatusChartDataAlternativeMatchesTheAggregates', () => {
    render(<SummaryView access={auditor} movements={INITIAL_MOVEMENTS} />)
    const expected = totalsByStatus(INITIAL_MOVEMENTS).map((entry) => [entry.label, String(entry.count), formatCop(entry.total)])
    expect(cells(dataTable(figureOf(/Monto por estado/)))).toEqual(expected)
  })

  it('TestRF021_CategoryChartDataAlternativeMatchesTheAggregates', () => {
    render(<SummaryView access={auditor} movements={INITIAL_MOVEMENTS} />)
    const expected = totalsByCategory(INITIAL_MOVEMENTS).map((entry) => [entry.category, String(entry.count), formatCop(entry.total)])
    expect(cells(dataTable(figureOf(/Monto por categoría/)))).toEqual(expected)
  })

  it('TestRF021_TimelineChartDataAlternativeMatchesTheAggregates', () => {
    render(<SummaryView access={auditor} movements={INITIAL_MOVEMENTS} />)
    const expected = totalsByMonth(INITIAL_MOVEMENTS).map((entry) => [entry.label, String(entry.count), formatCop(entry.total)])
    expect(expected.length).toBeGreaterThanOrEqual(4)
    expect(cells(dataTable(figureOf(/Monto por mes/)))).toEqual(expected)
  })

  it('TestRF021_AnalystChartsOnlyAggregateTheirOwnMovements', () => {
    render(<SummaryView access={analyst} movements={INITIAL_MOVEMENTS} />)
    const own = visibleMovements(INITIAL_MOVEMENTS, 'own')
    expect(own.length).toBeLessThan(INITIAL_MOVEMENTS.length)
    const expected = totalsByStatus(own).map((entry) => [entry.label, String(entry.count), formatCop(entry.total)])
    expect(cells(dataTable(figureOf(/Monto por estado/)))).toEqual(expected)
  })

  it('TestRF021_ChartsHandleAnEmptyLedger', () => {
    render(<SummaryView access={auditor} movements={[]} />)
    expect(chart(/Monto por categoría/).getAttribute('aria-label')).toMatch(/sin movimientos/i)
    expect(screen.getByText('Aún no hay movimientos para mostrar.')).toBeInTheDocument()
  })
})

describe('recent activity', () => {
  it('TestRF021_RecentActivityListsTheSixNewestMovementsInOrderWithTheirDetails', () => {
    render(<SummaryView access={auditor} movements={INITIAL_MOVEMENTS} />)
    const list = screen.getByRole('list', { name: 'Actividad reciente' })
    const items = within(list).getAllByRole('listitem')
    const expected = recentMovements(INITIAL_MOVEMENTS, 6)
    expect(items).toHaveLength(6)
    items.forEach((item, index) => {
      const movement = expected[index]!
      expect(item).toHaveTextContent(movement.description)
      expect(item).toHaveTextContent(movement.date)
      expect(item).toHaveTextContent(formatCop(movement.amount).replace(/\u00a0/g, ' '))
    })
    expect(items[0]).toHaveTextContent('2026-09-29')
  })

  it('TestRF021_RecentActivityOfAnAnalystOnlyShowsTheirOwnMovements', () => {
    render(<SummaryView access={analyst} movements={INITIAL_MOVEMENTS} />)
    const items = within(screen.getByRole('list', { name: 'Actividad reciente' })).getAllByRole('listitem')
    const own = recentMovements(visibleMovements(INITIAL_MOVEMENTS, 'own'), 6)
    expect(items).toHaveLength(own.length)
    expect(items[0]).toHaveTextContent(own[0]!.description)
  })
})
