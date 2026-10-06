import type { Access } from '../access'
import { LockIcon } from '../icons'
import { LAST_CLOSE, PERIOD, countOf, inOpenPeriod, type Movement } from '../ledger'

export function ClosingView({ access, movements, closed, onClose }: Readonly<{ access: Access; movements: Movement[]; closed: boolean; onClose: () => void }>) {
  if (!access.canClose) {
    return (
      <section aria-labelledby="view-title">
        <h1 id="view-title">Cierre contable</h1>
        <div className="card locked-card"><LockIcon aria-hidden="true" /><span>Requiere el permiso cierre.ejecutar</span></div>
      </section>
    )
  }
  // The close covers only the open period; earlier months were closed already.
  const period = inOpenPeriod(movements)
  return (
    <section aria-labelledby="view-title">
      <h1 id="view-title">Cierre contable</h1>
      <div className="card closing-card">
        <dl>
          <div><dt>Periodo</dt><dd>{PERIOD}</dd></div>
          <div><dt>Movimientos aprobados</dt><dd>{countOf(period, 'approved')}</dd></div>
          <div><dt>Movimientos pendientes</dt><dd>{countOf(period, 'pending')}</dd></div>
          <div><dt>Último cierre</dt><dd>{closed ? PERIOD : LAST_CLOSE}</dd></div>
        </dl>
        <button className="primary-button" type="button" disabled={closed} onClick={onClose}>{closed ? 'Mes cerrado' : `Cerrar mes de ${PERIOD}`}</button>
      </div>
    </section>
  )
}
