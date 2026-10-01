import { HUB_ORIGIN } from '../config'

export function AdministrationView() {
  return (
    <section aria-labelledby="view-title">
      <h1 id="view-title">Administración</h1>
      <p className="lead">La gestión de usuarios y roles vive en Identity Hub, no aquí; Contabilidad solo la consume.</p>
      <a className="banner" href={HUB_ORIGIN}>Abrir la consola de Identity Hub</a>
    </section>
  )
}
