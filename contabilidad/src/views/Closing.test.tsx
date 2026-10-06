import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { accessFor } from '../access'
import { INITIAL_MOVEMENTS, countOf, inOpenPeriod } from '../ledger'
import { ClosingView } from './Closing'

const closer = accessFor(['movimientos.ver_todos', 'cierre.ejecutar'], ['contabilidad.admin'])
const valueOf = (term: string) => screen.getByText(term).nextElementSibling?.textContent

describe('closing view', () => {
  it('TestRF021_ClosingCountsOnlyTheOpenPeriod', () => {
    render(<ClosingView access={closer} movements={INITIAL_MOVEMENTS} closed={false} onClose={() => undefined} />)
    const open = inOpenPeriod(INITIAL_MOVEMENTS)
    expect(countOf(INITIAL_MOVEMENTS, 'approved')).toBeGreaterThan(countOf(open, 'approved'))
    expect(valueOf('Movimientos aprobados')).toBe(String(countOf(open, 'approved')))
    expect(valueOf('Movimientos pendientes')).toBe(String(countOf(open, 'pending')))
  })
})
