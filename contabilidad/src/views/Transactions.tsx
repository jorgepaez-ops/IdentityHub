import { FormEvent, useState } from 'react'
import type { Access } from '../access'
import { STATUS_LABEL, formatCop, visibleMovements, type Movement } from '../ledger'

interface Props {
  access: Access
  movements: Movement[]
  onDecide: (id: string, status: 'approved' | 'rejected') => void
  onRegister: (description: string, amount: number) => void
}

export function TransactionsView({ access, movements, onDecide, onRegister }: Props) {
  const [adding, setAdding] = useState(false)
  if (access.level === 'none') return null
  const rows = visibleMovements(movements, access.level)

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    const amount = Number(data.get('amount'))
    const description = String(data.get('description')).trim()
    if (!description || !Number.isFinite(amount) || amount <= 0) return
    onRegister(description, amount)
    setAdding(false)
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
          <button className="primary-button" type="submit">Registrar</button>
        </form>
      )}
      <div className="card table-card">
        <table>
          <thead>
            <tr><th>Folio</th><th>Descripción</th><th className="num">Monto</th><th>Estado</th><th>Acciones</th></tr>
          </thead>
          <tbody>
            {rows.map((movement) => (
              <tr key={movement.id}>
                <td><span className="mono">{movement.id}</span><small>{movement.date}</small></td>
                <td>{movement.description}<small>{movement.category}</small></td>
                <td className="num mono">{formatCop(movement.amount)}</td>
                <td><span className={`pill pill-${movement.status}`}>{STATUS_LABEL[movement.status]}</span></td>
                <td>
                  {movement.status === 'pending' && access.canApprove && (
                    <span className="actions">
                      <button className="small-button ok" type="button" aria-label={`Aprobar ${movement.id}`} onClick={() => onDecide(movement.id, 'approved')}>Aprobar</button>
                      <button className="small-button danger" type="button" aria-label={`Rechazar ${movement.id}`} onClick={() => onDecide(movement.id, 'rejected')}>Rechazar</button>
                    </span>
                  )}
                  {movement.status === 'pending' && !access.canApprove && <span className="muted">esperando aprobación</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
