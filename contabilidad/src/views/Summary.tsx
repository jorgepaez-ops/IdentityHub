import type { Access } from '../access'
import { LAST_CLOSE, USERS_WITH_ACCESS, countOf, formatCop, totalOf, visibleMovements, type Movement } from '../ledger'

function Stat({ label, value, note }: { label: string; value: string; note?: string }) {
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <strong className="stat-value">{value}</strong>
      {note && <span className="stat-note">{note}</span>}
    </div>
  )
}

export function SummaryView({ access, movements }: { access: Access; movements: Movement[] }) {
  if (access.level === 'none') return null
  if (!access.canSeeReports) {
    return (
      <section aria-labelledby="view-title">
        <h1 id="view-title">Resumen</h1>
        <div className="card locked-card">Requiere el permiso reportes.ver</div>
      </section>
    )
  }
  const rows = visibleMovements(movements, access.level)
  const own = access.level === 'own'
  return (
    <section aria-labelledby="view-title">
      <h1 id="view-title">Resumen</h1>
      <p className="lead">{own ? 'Aquí ves solo tus propios movimientos del periodo.' : 'Estado del periodo para toda la organización.'}</p>
      <div className="stats">
        {own ? (
          <>
            <Stat label="Mis aprobados" value={formatCop(totalOf(rows, 'approved'))} note={`${countOf(rows, 'approved')} movimientos`} />
            <Stat label="Mis pendientes" value={formatCop(totalOf(rows, 'pending'))} note={`${countOf(rows, 'pending')} movimientos`} />
            <Stat label="Mis registros del mes" value={String(rows.length)} />
          </>
        ) : (
          <>
            <Stat label="Aprobado del mes" value={formatCop(totalOf(rows, 'approved'))} note={`${countOf(rows, 'approved')} movimientos`} />
            <Stat label="Pendiente de aprobación" value={formatCop(totalOf(rows, 'pending'))} note={`${countOf(rows, 'pending')} movimientos`} />
            <Stat label="Cerrado hasta" value={LAST_CLOSE} />
            <Stat label="Usuarios con acceso" value={String(USERS_WITH_ACCESS)} />
          </>
        )}
      </div>
    </section>
  )
}
