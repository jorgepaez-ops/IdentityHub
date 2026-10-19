import { useCallback, useEffect, useState } from 'react'
import { COMPANY } from '../hub'
import type { Grants } from '../permissions'
import { usePageTitle } from '../pageTitle'
import { localDay } from '../warehouse/day'
import { DIRECTION_NAME, post, revise, seedWarehouse, type Direction } from '../warehouse/ledger'
import { CatalogPanel } from './CatalogPanel'
import { MovementsPanel } from './MovementsPanel'

type Section = 'catalog' | 'movements'

interface Props {
  grants: Grants
  subject: string
  onSignOut: () => void
}

type Change =
  | { kind: 'revise'; sku: string; name: string; price: number }
  | { kind: 'post'; sku: string; direction: Direction; quantity: number }

const NOTICE_MS = 5000

export function Workspace({ grants, subject, onSignOut }: Readonly<Props>) {
  const [section, setSection] = useState<Section>(grants.move && !grants.view ? 'movements' : 'catalog')
  const [warehouse, setWarehouse] = useState(seedWarehouse)
  // The id makes a repeated message count as new, so its timer restarts.
  const [notice, setNotice] = useState<{ id: number; text: string } | null>(null)
  const say = (text: string) => setNotice((last) => ({ id: (last?.id ?? 0) + 1, text }))
  const [noticeShown, setNoticeShown] = useState(false)

  useEffect(() => {
    if (!notice) return
    setNoticeShown(true)
    const timer = window.setTimeout(() => setNoticeShown(false), NOTICE_MS)
    return () => window.clearTimeout(timer)
  }, [notice])

  const apply = useCallback((change: Change): boolean => {
    if (change.kind === 'revise') {
      setWarehouse((current) => revise(current, change.sku, change.name, change.price))
      say(`Producto ${change.sku} actualizado (datos de ejemplo).`)
      return true
    }
    const outcome = post(warehouse, change.sku, change.direction, change.quantity, localDay(new Date()))
    if (!outcome.ok) {
      say(outcome.problem === 'not_enough_stock' ? 'No hay existencias suficientes para esa salida.' : 'No se pudo registrar el movimiento.')
      return false
    }
    setWarehouse(outcome.warehouse)
    say(`${DIRECTION_NAME[change.direction]} de ${change.quantity} unidades registrada.`)
    return true
  }, [warehouse])

  const tabs: { id: Section; label: string; allowed: boolean }[] = [
    { id: 'catalog', label: 'Catálogo', allowed: grants.view },
    { id: 'movements', label: 'Movimientos', allowed: grants.move },
  ]
  usePageTitle(`${tabs.find((tab) => tab.id === section)?.label ?? 'Inventario'} · Inventario`)

  return (
    <div className="shell">
      <a className="shell__skip" href="#workspace-main">Saltar al contenido</a>
      <header className="shell__bar">
        <p className="shell__brand">
          <strong>Inventario</strong>
          <span>{COMPANY} · Conectada a Identity Hub</span>
        </p>
        <div className="shell__user">
          <p>
            <strong>{grants.roleTitle}</strong>
            <span>{COMPANY} · Cuenta {subject.slice(0, 8)}</span>
          </p>
          <button className="btn btn--outline btn--small" type="button" onClick={onSignOut}>Cerrar sesión</button>
        </div>
      </header>
      <nav className="tabs" aria-label="Secciones">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            className={tab.id === section ? 'tabs__tab tabs__tab--current' : 'tabs__tab'}
            aria-current={tab.id === section ? 'page' : undefined}
            disabled={!tab.allowed}
            onClick={() => setSection(tab.id)}
          >
            {tab.label}
            {!tab.allowed && <span className="tag tag--muted">Bloqueado</span>}
          </button>
        ))}
        <p className="tabs__summary">{grants.summary}</p>
      </nav>
      <main id="workspace-main" className="shell__main" tabIndex={-1}>
        {section === 'catalog' && (
          <CatalogPanel grants={grants} items={warehouse.items} onRevise={(sku, name, price) => { apply({ kind: 'revise', sku, name, price }) }} />
        )}
        {section === 'movements' && (
          <MovementsPanel
            grants={grants}
            items={warehouse.items}
            entries={warehouse.entries}
            onPost={(sku, direction, quantity) => apply({ kind: 'post', sku, direction, quantity })}
          />
        )}
      </main>
      {notice && noticeShown && (
        <div className="notice" role="status">
          <span>{notice.text}</span>
          <button className="notice__close" type="button" aria-label="Cerrar aviso" onClick={() => setNoticeShown(false)}>×</button>
        </div>
      )}
    </div>
  )
}
