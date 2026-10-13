export function LoadingSkeleton({ rows = 3, label = 'Cargando…' }: Readonly<{ rows?: number; label?: string }>) {
  return <output className="loading-skeleton" aria-busy="true"><span className="sr-only">{label}</span>{Array.from({ length: rows }, (_, index) => <span className="skeleton-line" key={index} />)}</output>
}
