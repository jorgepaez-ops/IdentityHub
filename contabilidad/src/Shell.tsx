import { useState } from 'react'
import type { Access } from './access'
import { INITIAL_MOVEMENTS, PERIOD, nextId, type Movement } from './ledger'
import { ClosingView } from './views/Closing'
import { SummaryView } from './views/Summary'
import { TransactionsView } from './views/Transactions'

type View = 'summary' | 'transactions' | 'closing'

interface Props {
  access: Access
  subject: string
  onLogout: () => void
}

export function Shell({ access, subject, onLogout }: Props) {
  const [view, setView] = useState<View>(access.canSeeReports ? 'summary' : 'transactions')
  const [movements, setMovements] = useState<Movement[]>(INITIAL_MOVEMENTS)
  const [closed, setClosed] = useState(false)
  const [toast, setToast] = useState<string | null>(null)

  const decide = (id: string, status: 'approved' | 'rejected') => {
    setMovements((current) => current.map((movement) => (movement.id === id ? { ...movement, status } : movement)))
    setToast(status === 'approved' ? `Movimiento ${id} aprobado.` : `Movimiento ${id} rechazado.`)
  }
  const register = (description: string, amount: number) => {
    setMovements((current) => [...current, { id: nextId(current), date: new Date().toISOString().slice(0, 10), description, category: 'Sin categoría', amount, status: 'pending', mine: true }])
    setToast('Movimiento registrado, queda pendiente de aprobación.')
  }
  const close = () => {
    setClosed(true)
    setToast(`Cierre de ${PERIOD} registrado (datos de ejemplo).`)
  }

  const items: { id: View; label: string; locked?: boolean }[] = [
    { id: 'summary', label: 'Resumen', locked: !access.canSeeReports },
    { id: 'transactions', label: 'Transacciones' },
    { id: 'closing', label: 'Cierre contable', locked: !access.canClose },
  ]
  const initials = access.label.split(' ').map((word) => word[0]).join('').slice(0, 2).toUpperCase()

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark small" aria-hidden="true">C</span>
          <span>Contabilidad<small>Conectada a Identity Hub</small></span>
        </div>
        <div className="whoami">
          <span className="avatar" aria-hidden="true">{initials}</span>
          <span className="who"><strong>{access.label}</strong><small>Cuenta {subject.slice(0, 8)}</small></span>
          <button className="pill-button" type="button" onClick={onLogout}>Cerrar sesión</button>
        </div>
      </header>
      <div className="layout">
        <aside className="rail">
          <nav aria-label="Secciones">
            {items.map((item) => (
              <button
                key={item.id}
                type="button"
                className={view === item.id ? 'rail-item active' : 'rail-item'}
                aria-current={view === item.id ? 'page' : undefined}
                disabled={item.locked}
                onClick={() => setView(item.id)}
              >
                {item.label}
                {item.locked && <span className="lock" role="img" aria-label="Bloqueado">&#128274;</span>}
              </button>
            ))}
          </nav>
          <p className="rail-hint">{access.hint}</p>
        </aside>
        <main className="content">
          {view === 'summary' && <SummaryView access={access} movements={movements} />}
          {view === 'transactions' && <TransactionsView access={access} movements={movements} onDecide={decide} onRegister={register} />}
          {view === 'closing' && <ClosingView access={access} movements={movements} closed={closed} onClose={close} />}
        </main>
      </div>
      {toast && <div className="toast" role="status">{toast}</div>}
    </div>
  )
}
