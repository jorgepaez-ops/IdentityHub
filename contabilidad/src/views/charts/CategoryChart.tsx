import { totalsByCategory } from '../../aggregate'
import { formatCop, type Movement } from '../../ledger'
import { ChartFigure, ChartSvg, summarize } from './ChartFigure'
import { formatMillions } from './format'

const TITLE = 'Monto por categoría'
const ROW = 30
const LABEL_WIDTH = 110
const BAR_MAX = 290

export function CategoryChart({ movements }: Readonly<{ movements: Movement[] }>) {
  const totals = totalsByCategory(movements)
  const max = Math.max(1, ...totals.map((entry) => entry.total))
  const label = totals.length === 0 ? summarize(TITLE, ['sin movimientos']) : summarize(TITLE, totals.map((entry) => `${entry.category} ${formatCop(entry.total)}`))
  return (
    <ChartFigure title={TITLE} caption="Monto acumulado por categoría" columns={['Categoría', 'Movimientos', 'Monto']} rows={totals.map((entry) => [entry.category, String(entry.count), formatCop(entry.total)])}>
      <ChartSvg label={label} viewBox={`0 0 480 ${Math.max(1, totals.length) * ROW + 6}`}>
        {totals.map((entry, index) => {
          const width = Math.max(2, (entry.total / max) * BAR_MAX)
          const y = index * ROW + 4
          return (
            <g key={entry.category}>
              <text className="chart-label" x={LABEL_WIDTH - 8} y={y + 16} textAnchor="end">{entry.category}</text>
              <rect className="chart-bar" x={LABEL_WIDTH} y={y} width={width} height="20" rx="3" />
              <text className="chart-value" x={LABEL_WIDTH + width + 6} y={y + 15}>{formatMillions(entry.total)}</text>
            </g>
          )
        })}
      </ChartSvg>
    </ChartFigure>
  )
}
