import { totalsByMonth } from '../../aggregate'
import { formatCop, type Movement } from '../../ledger'
import { ChartFigure, ChartSvg, summarize } from './ChartFigure'
import { formatMillions } from './format'

const TITLE = 'Monto por mes'
const WIDTH = 480
const BASELINE = 170
const PLOT_HEIGHT = 130

export function MonthChart({ movements }: { movements: Movement[] }) {
  const totals = totalsByMonth(movements)
  const max = Math.max(1, ...totals.map((entry) => entry.total))
  const slot = WIDTH / Math.max(1, totals.length)
  const barWidth = Math.min(48, slot * 0.6)
  const label = totals.length === 0 ? summarize(TITLE, ['sin movimientos']) : summarize(TITLE, totals.map((entry) => `${entry.label} ${formatCop(entry.total)}`))
  return (
    <ChartFigure title={TITLE} caption="Evolución mensual del monto registrado" columns={['Mes', 'Movimientos', 'Monto']} rows={totals.map((entry) => [entry.label, String(entry.count), formatCop(entry.total)])}>
      <ChartSvg label={label} viewBox={`0 0 ${WIDTH} 200`}>
        <line className="chart-axis" x1="0" y1={BASELINE} x2={WIDTH} y2={BASELINE} />
        {totals.map((entry, index) => {
          const height = (entry.total / max) * PLOT_HEIGHT
          const center = slot * index + slot / 2
          return (
            <g key={entry.month}>
              <rect className="chart-bar" x={center - barWidth / 2} y={BASELINE - height} width={barWidth} height={height} rx="3" />
              <text className="chart-value" x={center} y={BASELINE - height - 6} textAnchor="middle">{formatMillions(entry.total)}</text>
              <text className="chart-label" x={center} y={BASELINE + 18} textAnchor="middle">{entry.label}</text>
            </g>
          )
        })}
      </ChartSvg>
    </ChartFigure>
  )
}
