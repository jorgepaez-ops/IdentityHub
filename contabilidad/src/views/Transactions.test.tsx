import { fireEvent, render, screen, within } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { accessFor } from '../access'
import { INITIAL_MOVEMENTS, type Movement } from '../ledger'
import { TransactionsView } from './Transactions'

const senior = accessFor(['movimientos.registrar', 'movimientos.ver_todos', 'movimientos.aprobar'], ['contabilidad.senior'])
const analyst = accessFor(['movimientos.registrar', 'reportes.ver'], ['contabilidad.analista'])

interface Spies { onDecide: (id: string, status: 'approved' | 'rejected') => void; onRegister: (description: string, amount: number, category: string) => void }

function renderView(access = senior, movements: Movement[] = INITIAL_MOVEMENTS, spies: Partial<Spies> = {}) {
  const onDecide = spies.onDecide ?? vi.fn()
  const onRegister = spies.onRegister ?? vi.fn()
  render(<TransactionsView access={access} movements={movements} onDecide={onDecide} onRegister={onRegister} />)
  return { onDecide, onRegister }
}

const search = (value: string) => fireEvent.change(screen.getByLabelText('Buscar'), { target: { value } })
const pick = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } })
const bodyRows = () => within(screen.getByRole('table', { name: 'Movimientos del periodo' })).getAllByRole('row').slice(1)
const folios = () => bodyRows().map((row) => within(row).getAllByRole('cell')[1]?.textContent)
const amounts = () => bodyRows().map((row) => within(row).getAllByRole('cell')[3]?.textContent)
const dates = () => bodyRows().map((row) => within(row).getAllByRole('cell')[0]?.textContent)
const header = (name: string) => screen.getByRole('columnheader', { name })

describe('movements search', () => {
  it('TestRF021_SearchMatchesFolioDescriptionAndCategory', () => {
    renderView()
    search('M-2043')
    expect(folios()).toEqual(['M-2043'])
    search('papelería')
    expect(folios().every((folio) => ['M-2044', 'M-1005', 'M-1019', 'M-1033'].includes(folio ?? ''))).toBe(true)
    expect(folios()).toHaveLength(4)
    search('impuestos')
    expect(folios()).toHaveLength(INITIAL_MOVEMENTS.filter((movement) => movement.category === 'Impuestos').length)
  })

  it('TestRF021_SearchIgnoresCaseAndAccents', () => {
    renderView()
    search('NOMINA')
    expect(folios()).toHaveLength(INITIAL_MOVEMENTS.filter((movement) => movement.category === 'Nómina').length)
    search('viaticos')
    expect(folios()).toHaveLength(INITIAL_MOVEMENTS.filter((movement) => movement.category === 'Viáticos').length)
    search('Papeleria ANDINA')
    expect(folios()).toHaveLength(4)
  })

  it('TestRF021_ResultCountIsAnnouncedPoliteWithoutAddingAStatusRegion', () => {
    renderView()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.getByText(`${INITIAL_MOVEMENTS.length} movimientos`).closest('[aria-live="polite"]')).not.toBeNull()
    search('M-2043')
    expect(screen.getByText('1 movimiento').closest('[aria-live="polite"]')).not.toBeNull()
  })
})

describe('movements filters', () => {
  it('TestRF021_StatusFilterKeepsOnlyThatStatus', () => {
    renderView()
    for (const [value, label] of [['pending', 'Pendiente'], ['approved', 'Aprobado'], ['rejected', 'Rechazado']] as const) {
      pick('Filtrar por estado', value)
      const expected = INITIAL_MOVEMENTS.filter((movement) => movement.status === value).length
      expect(bodyRows()).toHaveLength(expected)
      for (const row of bodyRows()) expect(within(row).getByText(label)).toBeInTheDocument()
    }
    pick('Filtrar por estado', 'all')
    expect(bodyRows()).toHaveLength(INITIAL_MOVEMENTS.length)
  })

  it('TestRF021_CategoryFilterIsBuiltFromTheDataAndKeepsOnlyThatCategory', () => {
    renderView()
    const select = screen.getByLabelText('Filtrar por categoría')
    const options = within(select).getAllByRole('option').map((option) => option.textContent)
    expect(options).toEqual(['Todas', ...[...new Set(INITIAL_MOVEMENTS.map((movement) => movement.category))].sort((a, b) => a.localeCompare(b, 'es'))])
    pick('Filtrar por categoría', 'Arriendo')
    expect(bodyRows()).toHaveLength(INITIAL_MOVEMENTS.filter((movement) => movement.category === 'Arriendo').length)
  })

  it('TestRF021_FiltersCombineWithSearch', () => {
    renderView()
    pick('Filtrar por estado', 'pending')
    pick('Filtrar por categoría', 'Viáticos')
    expect(folios().sort()).toEqual(['M-1040', 'M-2043'])
    search('cali')
    expect(folios()).toEqual(['M-2043'])
  })

  it('TestRF021_EmptyStateOffersClearFiltersAndRestoresTheRows', () => {
    renderView()
    pick('Filtrar por estado', 'rejected')
    search('zzz-no-existe')
    expect(bodyRows()).toHaveLength(0)
    expect(screen.getByText('Ningún movimiento coincide con la búsqueda y los filtros.')).toBeInTheDocument()
    expect(screen.getByText('0 movimientos')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Limpiar filtros' }))
    expect(bodyRows()).toHaveLength(INITIAL_MOVEMENTS.length)
    expect(screen.getByLabelText('Buscar')).toHaveValue('')
    expect(screen.getByLabelText('Filtrar por estado')).toHaveValue('all')
    expect(screen.queryByText('Ningún movimiento coincide con la búsqueda y los filtros.')).not.toBeInTheDocument()
  })
})

describe('movements sorting', () => {
  it('TestRF021_DefaultOrderIsNewestFirstAndDateHeaderSaysDescending', () => {
    renderView()
    const sorted = [...dates()].sort().reverse()
    expect(dates()).toEqual(sorted)
    expect(header('Fecha')).toHaveAttribute('aria-sort', 'descending')
    expect(header('Folio')).not.toHaveAttribute('aria-sort')
    expect(header('Monto')).not.toHaveAttribute('aria-sort')
  })

  it('TestRF021_DateHeaderTogglesToOldestFirst', () => {
    renderView()
    fireEvent.click(within(header('Fecha')).getByRole('button'))
    expect(dates()).toEqual([...dates()].sort())
    expect(header('Fecha')).toHaveAttribute('aria-sort', 'ascending')
    fireEvent.click(within(header('Fecha')).getByRole('button'))
    expect(header('Fecha')).toHaveAttribute('aria-sort', 'descending')
  })

  it('TestRF021_AmountHeaderSortsByValueBothWays', () => {
    renderView()
    fireEvent.click(within(header('Monto')).getByRole('button'))
    const values = INITIAL_MOVEMENTS.map((movement) => movement.amount).sort((a, b) => b - a)
    expect(bodyRows().map((row) => row.querySelector('td.num')?.textContent)).toHaveLength(values.length)
    expect(folios()[0]).toBe(INITIAL_MOVEMENTS.find((movement) => movement.amount === values[0])?.id)
    expect(header('Monto')).toHaveAttribute('aria-sort', 'descending')
    expect(header('Fecha')).not.toHaveAttribute('aria-sort')
    fireEvent.click(within(header('Monto')).getByRole('button'))
    expect(header('Monto')).toHaveAttribute('aria-sort', 'ascending')
    expect(folios()[0]).toBe(INITIAL_MOVEMENTS.find((movement) => movement.amount === Math.min(...values))?.id)
    expect(amounts()).toHaveLength(values.length)
  })

  it('TestRF021_FolioHeaderSortsNumericallyAscendingThenDescending', () => {
    renderView()
    fireEvent.click(within(header('Folio')).getByRole('button'))
    expect(header('Folio')).toHaveAttribute('aria-sort', 'ascending')
    expect(folios()[0]).toBe('M-1001')
    expect(folios()[folios().length - 1]).toBe('M-2046')
    fireEvent.click(within(header('Folio')).getByRole('button'))
    expect(header('Folio')).toHaveAttribute('aria-sort', 'descending')
    expect(folios()[0]).toBe('M-2046')
  })

  it('TestRF021_SortingAppliesToTheFilteredRows', () => {
    renderView()
    pick('Filtrar por estado', 'pending')
    fireEvent.click(within(header('Monto')).getByRole('button'))
    expect(folios()).toEqual(['M-1039', 'M-1041', 'M-1038', 'M-2043', 'M-2044', 'M-1040'])
  })
})

describe('register with category', () => {
  const open = () => fireEvent.click(screen.getByRole('button', { name: '+ Registrar movimiento' }))
  const fill = (description: string, amount: string) => {
    fireEvent.change(screen.getByLabelText('Descripción'), { target: { value: description } })
    fireEvent.change(screen.getByLabelText('Monto'), { target: { value: amount } })
  }

  it('TestRF021_RegisterFormHasARequiredCategorySelectWithKnownCategories', () => {
    renderView()
    open()
    const select = screen.getByLabelText('Categoría')
    expect(select).toBeRequired()
    const options = within(select).getAllByRole('option').map((option) => option.textContent)
    expect(options).toContain('Viáticos')
    expect(options).not.toContain('Sin categoría')
  })

  it('TestRF021_RegisterDoesNotSubmitWithoutACategory', () => {
    const { onRegister } = renderView()
    open()
    fill('Taxi', '85000')
    fireEvent.click(screen.getByRole('button', { name: 'Registrar' }))
    expect(onRegister).not.toHaveBeenCalled()
  })

  it('TestRF021_RegisterPassesTheChosenCategory', () => {
    const { onRegister } = renderView()
    open()
    fill('Taxi aeropuerto', '85000')
    fireEvent.change(screen.getByLabelText('Categoría'), { target: { value: 'Viáticos' } })
    fireEvent.click(screen.getByRole('button', { name: 'Registrar' }))
    expect(onRegister).toHaveBeenCalledWith('Taxi aeropuerto', 85000, 'Viáticos')
  })

  it('TestRF021_NewRowCarriesTheCategoryAndSurvivesFilters', () => {
    function Harness() {
      const [rows, setRows] = useState<Movement[]>(INITIAL_MOVEMENTS)
      return <TransactionsView access={senior} movements={rows} onDecide={() => undefined} onRegister={(description, amount, category) => setRows((current) => [...current, { id: 'M-2047', date: '2026-10-01', description, category, amount, status: 'pending', mine: true }])} />
    }
    render(<Harness />)
    open()
    fill('Taxi aeropuerto', '85000')
    fireEvent.change(screen.getByLabelText('Categoría'), { target: { value: 'Viáticos' } })
    fireEvent.click(screen.getByRole('button', { name: 'Registrar' }))
    const row = screen.getByText('M-2047').closest('tr') as HTMLElement
    expect(within(row).getByText('Viáticos')).toBeInTheDocument()
    expect(screen.queryByText('Sin categoría')).not.toBeInTheDocument()
  })
})

describe('confirmation before deciding', () => {
  const rowOf = (folio: string) => screen.getByText(folio).closest('tr') as HTMLElement
  const ask = (folio: string, name: RegExp) => fireEvent.click(within(rowOf(folio)).getByRole('button', { name }))

  it('TestRF021_ApproveOpensAModalDialogNamingFolioAndAmountWithoutDeciding', () => {
    const { onDecide } = renderView()
    ask('M-2043', /Aprobar/)
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(dialog).toHaveAccessibleName(/Aprobar/)
    expect(dialog).toHaveTextContent('M-2043')
    expect(dialog).toHaveTextContent(/480\.000/)
    expect(onDecide).not.toHaveBeenCalled()
    expect(dialog.contains(document.activeElement)).toBe(true)
  })

  it('TestRF021_ConfirmingApproveDecidesOnceAndClosesTheDialog', () => {
    const { onDecide } = renderView()
    ask('M-2043', /Aprobar/)
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar aprobación' }))
    expect(onDecide).toHaveBeenCalledTimes(1)
    expect(onDecide).toHaveBeenCalledWith('M-2043', 'approved')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('TestRF021_ConfirmingRejectDecidesRejected', () => {
    const { onDecide } = renderView()
    ask('M-2044', /Rechazar/)
    expect(screen.getByRole('dialog')).toHaveTextContent('M-2044')
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar rechazo' }))
    expect(onDecide).toHaveBeenCalledWith('M-2044', 'rejected')
  })

  function DecidingHarness({ onDecide = () => undefined }: { onDecide?: (id: string, status: 'approved' | 'rejected') => void }) {
    const [rows, setRows] = useState<Movement[]>(INITIAL_MOVEMENTS)
    const decide = (id: string, status: 'approved' | 'rejected') => {
      onDecide(id, status)
      setRows((current) => current.map((row) => (row.id === id ? { ...row, status } : row)))
    }
    return (
      <>
        <button type="button" onClick={() => setRows((current) => current.map((row) => (row.id === 'M-2043' ? { ...row, status: 'approved' } : row)))}>Decidir por fuera</button>
        <TransactionsView access={senior} movements={rows} onDecide={decide} onRegister={() => undefined} />
      </>
    )
  }

  it('TestRF021_ConfirmingMovesFocusToTheTableInsteadOfTheBody', () => {
    render(<DecidingHarness />)
    ask('M-2043', /Aprobar/)
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar aprobación' }))
    expect(document.activeElement).toBe(screen.getByRole('table', { name: 'Movimientos del periodo' }))
  })

  it('TestRF021_ConfirmDoesNotDecideAMovementThatIsNoLongerPending', () => {
    const onDecide = vi.fn()
    render(<DecidingHarness onDecide={onDecide} />)
    ask('M-2043', /Aprobar/)
    fireEvent.click(screen.getByRole('button', { name: 'Decidir por fuera' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar aprobación' }))
    expect(onDecide).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('TestRF021_BackdropClickCancelsButAClickInsideTheDialogDoesNot', () => {
    const { onDecide } = renderView()
    ask('M-2043', /Aprobar/)
    const dialog = screen.getByRole('dialog')
    fireEvent.mouseDown(dialog)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.mouseDown(dialog.parentElement as HTMLElement)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(onDecide).not.toHaveBeenCalled()
  })

  it('TestRF021_CancelChangesNothingAndRestoresFocusToTheTrigger', () => {
    const { onDecide } = renderView()
    const trigger = within(rowOf('M-2043')).getByRole('button', { name: /Aprobar/ })
    trigger.focus()
    fireEvent.click(trigger)
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    expect(onDecide).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('TestRF021_EscapeCancelsTheDialog', () => {
    const { onDecide } = renderView()
    ask('M-2043', /Rechazar/)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onDecide).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('TestRF021_TabKeepsFocusInsideTheDialog', () => {
    renderView()
    ask('M-2043', /Aprobar/)
    const dialog = screen.getByRole('dialog')
    const buttons = within(dialog).getAllByRole('button')
    const last = buttons[buttons.length - 1] as HTMLElement
    last.focus()
    fireEvent.keyDown(dialog, { key: 'Tab' })
    expect(buttons[0]).toHaveFocus()
    fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true })
    expect(last).toHaveFocus()
  })

  it('TestRF021_AnalystWithoutApprovePermissionSeesNoDecisionButtons', () => {
    renderView(analyst)
    expect(screen.queryByRole('button', { name: /Aprobar/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Rechazar/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(within(rowOf('M-2043')).getByText('esperando aprobación')).toBeInTheDocument()
  })

  it('TestRF021_AnalystOnlySearchesAndFiltersTheirOwnRows', () => {
    renderView(analyst)
    search('nómina')
    expect(bodyRows()).toHaveLength(0)
    const options = within(screen.getByLabelText('Filtrar por categoría')).getAllByRole('option').map((option) => option.textContent)
    expect(options).not.toContain('Nómina')
  })
})

describe('table accessibility', () => {
  it('TestRF021_TableKeepsItsLabelAndColumnScopes', () => {
    renderView()
    const table = screen.getByRole('table', { name: 'Movimientos del periodo' })
    const headers = within(table).getAllByRole('columnheader')
    expect(headers.map((cell) => cell.textContent?.replace(/[↑↓↕]/g, ''))).toEqual(['Fecha', 'Folio', 'Descripción', 'Monto', 'Estado', 'Acciones'])
    for (const cell of headers) expect(cell).toHaveAttribute('scope', 'col')
  })
})
