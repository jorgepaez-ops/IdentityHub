import type { ReactNode } from 'react'

interface Props {
  title: string
  caption: string
  /** Column headers of the data alternative; the first one names the row label. */
  columns: [string, string, string]
  rows: [string, string, string][]
  children: ReactNode
}

/** Shared frame: figure + caption, an SVG named for assistive tech and the same data as a table. */
export function ChartFigure({ title, caption, columns, rows, children }: Readonly<Props>) {
  return (
    <figure className="card chart">
      <figcaption>{caption}</figcaption>
      {children}
      <details className="chart-data">
        <summary>Ver datos de «{title}»</summary>
        <table aria-label={`Datos de ${title}`}>
          <thead><tr><th scope="col">{columns[0]}</th><th scope="col" className="num">{columns[1]}</th><th scope="col" className="num">{columns[2]}</th></tr></thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row[0]}><th scope="row">{row[0]}</th><td className="num mono">{row[1]}</td><td className="num mono">{row[2]}</td></tr>
            ))}
          </tbody>
        </table>
      </details>
    </figure>
  )
}

interface SvgProps {
  /** Accessible name: the chart title followed by a summary of the data. */
  label: string
  viewBox: string
  children: ReactNode
}

/** Responsive drawing: width follows the container through the viewBox, never the `icon` size rule. */
export function ChartSvg({ label, viewBox, children }: Readonly<SvgProps>) {
  return <svg className="chart-svg" role="img" aria-label={label} viewBox={viewBox} preserveAspectRatio="xMidYMid meet">{children}</svg>
}

export const summarize = (title: string, parts: string[]) => `${title}: ${parts.join('; ')}.`
