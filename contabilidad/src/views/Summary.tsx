import type { Access } from '../access'
import { LockIcon } from '../icons'
import { recentMovements } from '../aggregate'
import { LAST_CLOSE, STATUS_LABEL, USERS_WITH_ACCESS, countOf, formatCop, inOpenPeriod, totalOf, visibleMovements, type Movement } from '../ledger'
import { CategoryChart } from './charts/CategoryChart'
import { MonthChart } from './charts/MonthChart'
import { StatusChart } from './charts/StatusChart'

const RECENT_LIMIT = 6

function Stat({ label, value, note }: Readonly<{ label: string; value: string; note?: string }>) {
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <strong className="stat-value">{value}</strong>
      {note && <span className="stat-note">{note}</span>}
    </div>
  )
}

export function SummaryView({ access, movements }: Readonly<{ access: Access; movements: Movement[] }>) {
  if (access.level === 'none') return null
  if (!access.canSeeReports) {
    return (
      <section aria-labelledby="view-title">
        <h1 id="view-title">Resumen</h1>
        <div className="card locked-card"><LockIcon aria-hidden="true" /><span>Requiere el permiso reportes.ver</span></div>
      </section>
    )
  }
  const rows = visibleMovements(movements, access.level)
  const own = access.level === 'own'
  // The stat cards describe the open period; the charts below cover the whole sample history.
  const period = inOpenPeriod(rows)
  const recent = recentMovements(rows, RECENT_LIMIT)
  return (
    <section aria-labelledby="view-title">
      <h1 id="view-title">Resumen</h1>
      <p className="lead">{own ? 'Aquí ves solo tus propios movimientos del periodo.' : 'Estado del periodo para toda la organización.'}</p>
      <div className="stats">
        {own ? (
          <>
            <Stat label="Mis aprobados" value={formatCop(totalOf(period, 'approved'))} note={`${countOf(period, 'approved')} movimientos`} />
            <Stat label="Mis pendientes" value={formatCop(totalOf(period, 'pending'))} note={`${countOf(period, 'pending')} movimientos`} />
            <Stat label="Mis registros del mes" value={String(period.length)} />
          </>
        ) : (
          <>
            <Stat label="Aprobado del mes" value={formatCop(totalOf(period, 'approved'))} note={`${countOf(period, 'approved')} movimientos`} />
            <Stat label="Pendiente de aprobación" value={formatCop(totalOf(period, 'pending'))} note={`${countOf(period, 'pending')} movimientos`} />
            <Stat label="Cerrado hasta" value={LAST_CLOSE} />
            <Stat label="Usuarios con acceso" value={String(USERS_WITH_ACCESS)} />
          </>
        )}
      </div>
      {rows.length === 0 && <p className="lead">Aún no hay movimientos para mostrar.</p>}
      <div className="charts">
        <StatusChart movements={rows} />
        <CategoryChart movements={rows} />
        <MonthChart movements={rows} />
      </div>
      <h2 className="section-title">Actividad reciente</h2>
      <div className="card recent">
        <ul className="recent-list" aria-label="Actividad reciente">
          {recent.map((movement) => (
            <li key={movement.id}>
              <span className="recent-main">{movement.description}<small>{movement.date} · {movement.category}</small></span>
              <span className="mono">{formatCop(movement.amount)}</span>
              <span className={`pill pill-${movement.status}`}>{STATUS_LABEL[movement.status]}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
