import { useEffect, useState, type ReactNode } from 'react'
import type { Access } from './access'
import { INITIAL_MOVEMENTS, PERIOD, nextId, type Movement } from './ledger'
import { ClosingIcon, LockIcon, LogoutIcon, SummaryIcon, TransactionsIcon } from './icons'
import { ThemeToggle } from './ThemeToggle'
import { Toast } from './Toast'
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
  // The id restarts the toast timer when the same message is shown twice in a row.
  const [toast, setToast] = useState<{ id: number; message: string } | null>(null)
  const notify = (message: string) => setToast((current) => ({ id: (current?.id ?? 0) + 1, message }))

  const decide = (id: string, status: 'approved' | 'rejected') => {
    setMovements((current) => current.map((movement) => (movement.id === id ? { ...movement, status } : movement)))
    notify(status === 'approved' ? `Movimiento ${id} aprobado.` : `Movimiento ${id} rechazado.`)
  }
  const register = (description: string, amount: number) => {
    setMovements((current) => [...current, { id: nextId(current), date: new Date().toISOString().slice(0, 10), description, category: 'Sin categoría', amount, status: 'pending', mine: true }])
    notify('Movimiento registrado, queda pendiente de aprobación.')
  }
  const close = () => {
    setClosed(true)
    notify(`Cierre de ${PERIOD} registrado (datos de ejemplo).`)
  }

  const items: { id: View; label: string; icon: ReactNode; locked?: boolean }[] = [
    { id: 'summary', label: 'Resumen', icon: <SummaryIcon />, locked: !access.canSeeReports },
    { id: 'transactions', label: 'Transacciones', icon: <TransactionsIcon /> },
    { id: 'closing', label: 'Cierre contable', icon: <ClosingIcon />, locked: !access.canClose },
  ]
  const current = items.find((item) => item.id === view)
  useEffect(() => {
    document.title = `${current?.label ?? 'Contabilidad'} · Contabilidad`
  }, [current?.label])
  const initials = access.label.split(' ').map((word) => word[0]).join('').slice(0, 2).toUpperCase()

  return (
    <div className="app">
      <a className="skip-link" href="#main-content">Saltar al contenido</a>
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark small" aria-hidden="true">C</span>
          <span>Contabilidad<small>Conectada a Identity Hub</small></span>
        </div>
        <div className="whoami">
          <span className="avatar" aria-hidden="true">{initials}</span>
          <span className="who"><strong>{access.label}</strong><small>Cuenta {subject.slice(0, 8)}</small></span>
          <ThemeToggle />
          <button className="pill-button" type="button" onClick={onLogout}><LogoutIcon />Cerrar sesión</button>
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
                <span className="rail-label">{item.icon}{item.label}</span>
                {item.locked && <LockIcon className="lock" role="img" aria-hidden={undefined} aria-label="Bloqueado" />}
              </button>
            ))}
          </nav>
          <p className="rail-hint">{access.hint}</p>
        </aside>
        <main className="content" id="main-content" tabIndex={-1}>
          {view === 'summary' && <SummaryView access={access} movements={movements} />}
          {view === 'transactions' && <TransactionsView access={access} movements={movements} onDecide={decide} onRegister={register} />}
          {view === 'closing' && <ClosingView access={access} movements={movements} closed={closed} onClose={close} />}
        </main>
      </div>
      {toast && <Toast key={toast.id} message={toast.message} onClose={() => setToast(null)} />}
    </div>
  )
}
