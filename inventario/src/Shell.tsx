import { useEffect, useState, type ReactNode } from 'react'
import type { Access } from './access'
import { COMPANY_NAME } from './config'
import { CatalogIcon, LockIcon, LogoutIcon, MovementsIcon } from './icons'
import { INITIAL_MOVEMENTS, INITIAL_PRODUCTS, KIND_LABEL, applyMovement, nextMovementId, type MovementKind, type Product, type StockMovement } from './stock'
import { ThemeToggle } from './ThemeToggle'
import { Toast } from './Toast'
import { CatalogView } from './views/Catalog'
import { MovementsView } from './views/Movements'

type View = 'catalog' | 'movements'

interface Props {
  access: Access
  subject: string
  onLogout: () => void
}

export function Shell({ access, subject, onLogout }: Readonly<Props>) {
  const [view, setView] = useState<View>(access.canMove && !access.canView ? 'movements' : 'catalog')
  const [products, setProducts] = useState<Product[]>(INITIAL_PRODUCTS)
  const [movements, setMovements] = useState<StockMovement[]>(INITIAL_MOVEMENTS)
  // The id restarts the toast timer when the same message is shown twice in a row.
  const [toast, setToast] = useState<{ id: number; message: string } | null>(null)
  const notify = (message: string) => setToast((current) => ({ id: (current?.id ?? 0) + 1, message }))

  const edit = (sku: string, name: string, price: number) => {
    setProducts((current) => current.map((product) => (product.sku === sku ? { ...product, name, price } : product)))
    notify(`Producto ${sku} actualizado (datos de ejemplo).`)
  }
  const register = (sku: string, kind: MovementKind, quantity: number): boolean => {
    const result = applyMovement(products, sku, kind, quantity)
    if (!result.ok) {
      notify(result.reason === 'insufficient_stock' ? 'No hay existencias suficientes para esa salida.' : 'No se pudo registrar el movimiento.')
      return false
    }
    setProducts(result.products)
    setMovements((current) => [...current, { id: nextMovementId(current), date: new Date().toISOString().slice(0, 10), sku, kind, quantity }])
    notify(`${KIND_LABEL[kind]} de ${quantity} unidades registrada.`)
    return true
  }

  const items: { id: View; label: string; icon: ReactNode; locked: boolean }[] = [
    { id: 'catalog', label: 'Catálogo', icon: <CatalogIcon />, locked: !access.canView },
    { id: 'movements', label: 'Movimientos', icon: <MovementsIcon />, locked: !access.canMove },
  ]
  const current = items.find((item) => item.id === view)
  useEffect(() => {
    document.title = `${current?.label ?? 'Inventario'} · Inventario`
  }, [current?.label])
  const initials = access.label.split(' ').map((word) => word[0]).join('').slice(0, 2).toUpperCase()

  return (
    <div className="app">
      <a className="skip-link" href="#main-content">Saltar al contenido</a>
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark small" aria-hidden="true">I</span>
          <span>Inventario<small>{COMPANY_NAME} · Conectada a Identity Hub</small></span>
        </div>
        <div className="whoami">
          <span className="avatar" aria-hidden="true">{initials}</span>
          <span className="who"><strong>{access.label}</strong><small>{COMPANY_NAME} · Cuenta {subject.slice(0, 8)}</small></span>
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
          {view === 'catalog' && <CatalogView access={access} products={products} onEdit={edit} />}
          {view === 'movements' && <MovementsView access={access} products={products} movements={movements} onRegister={register} />}
        </main>
      </div>
      {toast && <Toast key={toast.id} message={toast.message} onClose={() => setToast(null)} />}
    </div>
  )
}
