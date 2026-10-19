import { LockIcon } from '../icons'

export function Locked({ title, message }: Readonly<{ title: string; message: string }>) {
  return (
    <section>
      <div className="view-head"><h1>{title}</h1></div>
      <div className="card locked-card"><LockIcon />{message}</div>
    </section>
  )
}

export const DemoNotice = () => <p className="lead">Datos de ejemplo: no hay almacén real detrás y los cambios se pierden al recargar la página.</p>
