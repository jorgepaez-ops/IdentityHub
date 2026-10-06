import { FormEvent, useState } from 'react'
import type { Access } from '../access'
import { categoriesOf, filterMovements, sortMovements, type SortDirection, type SortKey, type StatusFilter } from '../aggregate'
import { CheckIcon, CloseIcon } from '../icons'
import { KNOWN_CATEGORIES, STATUS_LABEL, formatCop, visibleMovements, type Movement } from '../ledger'
import { ConfirmDialog } from './ConfirmDialog'

interface Props {
  access: Access
  movements: Movement[]
  onDecide: (id: string, status: 'approved' | 'rejected') => void
  onRegister: (description: string, amount: number, category: string) => void
}

type Decision = 'approved' | 'rejected'

// Date and amount start newest/largest first; folios start at the lowest number.
const FIRST_DIRECTION: Record<SortKey, SortDirection> = { date: 'descending', amount: 'descending', folio: 'ascending' }

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'Todos' },
  { value: 'pending', label: STATUS_LABEL.pending },
  { value: 'approved', label: STATUS_LABEL.approved },
  { value: 'rejected', label: STATUS_LABEL.rejected },
]

export function TransactionsView({ access, movements, onDecide, onRegister }: Props) {
  const [adding, setAdding] = useState(false)
  const [text, setText] = useState('')
  const [status, setStatus] = useState<StatusFilter>('all')
  const [category, setCategory] = useState('')
  const [sort, setSort] = useState<{ key: SortKey; direction: SortDirection }>({ key: 'date', direction: 'descending' })
  const [deciding, setDeciding] = useState<{ movement: Movement; decision: Decision } | null>(null)
  if (access.level === 'none') return null
  const visible = visibleMovements(movements, access.level)
  const rows = sortMovements(filterMovements(visible, { text, status, category }), sort.key, sort.direction)
  const filtering = text.trim() !== '' || status !== 'all' || category !== ''
  const count = `${rows.length} ${rows.length === 1 ? 'movimiento' : 'movimientos'}`

  const clearFilters = () => {
    setText('')
    setStatus('all')
    setCategory('')
  }
  const sortBy = (key: SortKey) => setSort((current) => ({ key, direction: current.key === key ? (current.direction === 'ascending' ? 'descending' : 'ascending') : FIRST_DIRECTION[key] }))
  const ariaSort = (key: SortKey) => (sort.key === key ? sort.direction : undefined)
  const sortHeader = (key: SortKey, label: string, className?: string) => (
    <th scope="col" className={className} aria-sort={ariaSort(key)}>
      <button className="sort-button" type="button" onClick={() => sortBy(key)}>{label}<span className="sort-arrow" aria-hidden="true">{sort.key === key ? (sort.direction === 'ascending' ? '↑' : '↓') : '↕'}</span></button>
    </th>
  )

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    const amount = Number(data.get('amount'))
    const description = String(data.get('description')).trim()
    const chosen = String(data.get('category'))
    if (!description || !Number.isFinite(amount) || amount <= 0 || !KNOWN_CATEGORIES.includes(chosen)) return
    onRegister(description, amount, chosen)
    setAdding(false)
  }

  const confirm = () => {
    if (!deciding) return
    onDecide(deciding.movement.id, deciding.decision)
    setDeciding(null)
  }

  return (
    <section aria-labelledby="view-title">
      <div className="view-head">
        <h1 id="view-title">Transacciones</h1>
        {access.canRegister && <button className="primary-button" type="button" onClick={() => setAdding((open) => !open)}>+ Registrar movimiento</button>}
      </div>
      {adding && access.canRegister && (
        <form className="card movement-form" onSubmit={submit}>
          <label htmlFor="movement-description">Descripción</label>
          <input id="movement-description" name="description" required maxLength={80} />
          <label htmlFor="movement-amount">Monto</label>
          <input id="movement-amount" name="amount" type="number" min="1" step="1" required />
          <label htmlFor="movement-category">Categoría</label>
          <select id="movement-category" name="category" required defaultValue="">
            <option value="" disabled>Selecciona una categoría</option>
            {KNOWN_CATEGORIES.map((known) => <option key={known} value={known}>{known}</option>)}
          </select>
          <button className="primary-button" type="submit">Registrar</button>
        </form>
      )}
      <div className="card movement-filters">
        <div className="field">
          <label htmlFor="movement-search">Buscar</label>
          <input id="movement-search" type="search" value={text} placeholder="Folio, descripción o categoría" onChange={(event) => setText(event.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="movement-status-filter">Filtrar por estado</label>
          <select id="movement-status-filter" value={status} onChange={(event) => setStatus(event.target.value as StatusFilter)}>
            {STATUS_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </div>
        <div className="field">
          <label htmlFor="movement-category-filter">Filtrar por categoría</label>
          <select id="movement-category-filter" value={category} onChange={(event) => setCategory(event.target.value)}>
            <option value="">Todas</option>
            {categoriesOf(visible).map((known) => <option key={known} value={known}>{known}</option>)}
          </select>
        </div>
        <p className="result-count muted" aria-live="polite" aria-atomic="true">{count}</p>
      </div>
      <div className="card table-card">
        <table className="movements-table" aria-label="Movimientos del periodo">
          <thead>
            <tr>
              {sortHeader('date', 'Fecha')}
              {sortHeader('folio', 'Folio')}
              <th scope="col">Descripción</th>
              {sortHeader('amount', 'Monto', 'num')}
              <th scope="col">Estado</th>
              <th scope="col">Acciones</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((movement) => (
              <tr key={movement.id}>
                <td className="mono date">{movement.date}</td>
                <td><span className="mono">{movement.id}</span></td>
                <td>{movement.description}<small>{movement.category}</small></td>
                <td className="num mono">{formatCop(movement.amount)}</td>
                <td><span className={`pill pill-${movement.status}`}>{STATUS_LABEL[movement.status]}</span></td>
                <td>
                  {movement.status === 'pending' && access.canApprove && (
                    <span className="actions">
                      <button className="small-button ok" type="button" aria-label={`Aprobar ${movement.id}`} onClick={() => setDeciding({ movement, decision: 'approved' })}><CheckIcon />Aprobar</button>
                      <button className="small-button danger" type="button" aria-label={`Rechazar ${movement.id}`} onClick={() => setDeciding({ movement, decision: 'rejected' })}><CloseIcon />Rechazar</button>
                    </span>
                  )}
                  {movement.status === 'pending' && !access.canApprove && <span className="muted">esperando aprobación</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && (
          <div className="empty-state">
            {filtering ? (
              <>
                <p>Ningún movimiento coincide con la búsqueda y los filtros.</p>
                <button className="secondary-button" type="button" onClick={clearFilters}>Limpiar filtros</button>
              </>
            ) : <p>Aún no hay movimientos para mostrar.</p>}
          </div>
        )}
      </div>
      {deciding && (
        <ConfirmDialog
          title={`${deciding.decision === 'approved' ? 'Aprobar' : 'Rechazar'} movimiento ${deciding.movement.id}`}
          confirmLabel={deciding.decision === 'approved' ? 'Confirmar aprobación' : 'Confirmar rechazo'}
          tone={deciding.decision === 'approved' ? 'ok' : 'danger'}
          onConfirm={confirm}
          onCancel={() => setDeciding(null)}
        >
          {`Vas a ${deciding.decision === 'approved' ? 'aprobar' : 'rechazar'} el movimiento ${deciding.movement.id} por ${formatCop(deciding.movement.amount)}. Esta decisión queda registrada.`}
        </ConfirmDialog>
      )}
    </section>
  )
}
