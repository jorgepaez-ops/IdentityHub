import type { ReactNode } from 'react'

export function Panel({ id, title, children }: Readonly<{ id: string; title: string; children: ReactNode }>) {
  return (
    <section className="panel" aria-labelledby={id}>
      <h1 id={id} className="panel__title">{title}</h1>
      {children}
    </section>
  )
}

export function LockedPanel({ id, title, reason }: Readonly<{ id: string; title: string; reason: string }>) {
  return (
    <Panel id={id} title={title}>
      <p className="panel__locked">{reason}</p>
    </Panel>
  )
}

export const DemoDataNote = () => (
  <p className="panel__note">Datos de ejemplo: no hay almacén real detrás y los cambios se pierden al recargar la página.</p>
)
