import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { accessFor } from '../access'
import { INITIAL_MOVEMENTS } from '../ledger'
import { ClosingView } from './Closing'
import { SummaryView } from './Summary'

// The rail disables a locked section, so the shell never mounts these branches for a user who lacks
// the permission; rendering the views directly is the only way to prove the copy and the guard.
const auditor = accessFor(['movimientos.ver_todos', 'reportes.ver'], ['contabilidad.auditor'])
const withoutReports = accessFor(['movimientos.ver_todos'], ['contabilidad.auditor'])

describe('locked views name the missing permission', () => {
  it('TestRF021_ClosingViewIsLockedAndNamesCierreEjecutarWithoutThePermission', () => {
    render(<ClosingView access={auditor} movements={INITIAL_MOVEMENTS} closed={false} onClose={() => undefined} />)
    expect(screen.getByRole('heading', { name: 'Cierre contable' })).toBeInTheDocument()
    expect(screen.getByText('Requiere el permiso cierre.ejecutar')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Cerrar mes de/ })).not.toBeInTheDocument()
  })

  it('TestRF021_SummaryViewIsLockedAndNamesReportesVerWithoutThePermission', () => {
    render(<SummaryView access={withoutReports} movements={INITIAL_MOVEMENTS} />)
    expect(screen.getByRole('heading', { name: 'Resumen' })).toBeInTheDocument()
    expect(screen.getByText('Requiere el permiso reportes.ver')).toBeInTheDocument()
    expect(screen.queryByText('Aprobado del mes')).not.toBeInTheDocument()
  })

  it('TestRF021_SummaryViewShowsTheStatsWithThePermission', () => {
    render(<SummaryView access={auditor} movements={INITIAL_MOVEMENTS} />)
    expect(screen.getByText('Aprobado del mes')).toBeInTheDocument()
    expect(screen.queryByText('Requiere el permiso reportes.ver')).not.toBeInTheDocument()
  })
})
