import { totalsByStatus } from '../../aggregate'
import { formatCop, type Movement } from '../../ledger'
import { ChartFigure, ChartSvg, summarize } from './ChartFigure'

const TITLE = 'Monto por estado'
const RADIUS = 40
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

export function StatusChart({ movements }: Readonly<{ movements: Movement[] }>) {
  const totals = totalsByStatus(movements)
  const grand = totals.reduce((sum, entry) => sum + entry.total, 0)
  let offset = 0
  const arcs = totals.map((entry) => {
    const length = grand === 0 ? 0 : (entry.total / grand) * CIRCUMFERENCE
    const arc = { ...entry, length, offset }
    offset += length
    return arc
  })
  const label = movements.length === 0
    ? summarize(TITLE, ['sin movimientos'])
    : summarize(TITLE, totals.map((entry) => `${entry.label} ${formatCop(entry.total)} en ${entry.count} movimientos`))
  return (
    <ChartFigure title={TITLE} caption="Monto por estado de aprobación" columns={['Estado', 'Movimientos', 'Monto']} rows={totals.map((entry) => [entry.label, String(entry.count), formatCop(entry.total)])}>
      <div className="chart-split">
        <ChartSvg label={label} viewBox="0 0 120 120">
          <circle className="chart-track" cx="60" cy="60" r={RADIUS} fill="none" strokeWidth="16" />
          {arcs.filter((arc) => arc.length > 0).map((arc) => (
            <circle key={arc.status} className={`chart-arc chart-arc-${arc.status}`} cx="60" cy="60" r={RADIUS} fill="none" strokeWidth="16" strokeDasharray={`${arc.length} ${CIRCUMFERENCE - arc.length}`} strokeDashoffset={-arc.offset} transform="rotate(-90 60 60)" />
          ))}
          <text className="chart-center" x="60" y="64" textAnchor="middle">{movements.length}</text>
        </ChartSvg>
        <ul className="chart-legend" aria-hidden="true">
          {totals.map((entry) => (
            <li key={entry.status}><span className={`chart-swatch chart-swatch-${entry.status}`} />{entry.label}<span className="mono">{formatCop(entry.total)}</span></li>
          ))}
        </ul>
      </div>
    </ChartFigure>
  )
}
