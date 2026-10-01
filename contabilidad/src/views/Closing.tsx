import type { Access } from '../access'
import { LAST_CLOSE, PERIOD, countOf, type Movement } from '../ledger'

export function ClosingView({ access, movements, closed, onClose }: { access: Access; movements: Movement[]; closed: boolean; onClose: () => void }) {
  if (!access.canClose) {
    return (
      <section aria-labelledby="view-title">
        <h1 id="view-title">Cierre contable</h1>
        <div className="card locked-card">Requiere rol contador senior o admin</div>
      </section>
    )
  }
  return (
    <section aria-labelledby="view-title">
      <h1 id="view-title">Cierre contable</h1>
      <div className="card closing-card">
        <dl>
          <div><dt>Periodo</dt><dd>{PERIOD}</dd></div>
          <div><dt>Movimientos aprobados</dt><dd>{countOf(movements, 'approved')}</dd></div>
          <div><dt>Movimientos pendientes</dt><dd>{countOf(movements, 'pending')}</dd></div>
          <div><dt>Último cierre</dt><dd>{closed ? PERIOD : LAST_CLOSE}</dd></div>
        </dl>
        <button className="primary-button" type="button" disabled={closed} onClick={onClose}>{closed ? 'Mes cerrado' : `Cerrar mes de ${PERIOD}`}</button>
      </div>
    </section>
  )
}
