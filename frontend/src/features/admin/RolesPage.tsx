import { FormEvent, useCallback, useEffect, useId, useRef, useState } from 'react'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { Toast } from '../../components/Toast'
import {
  ApiProblemError, type Application, type ApplicationRole,
  createApplicationRole, deleteApplicationRole, listApplications, updateApplicationRole,
} from '../../api/client'
import { connectionMessage } from '../account/PublicPages'
import { Problems, isAuthFailure } from './shared'

// Same rule as the backend: <client_id>.<name> with name matching ^[a-z0-9][a-z0-9_-]{1,40}$.
const ROLE_SLUG = /^[a-z0-9][a-z0-9_-]{1,40}$/

const STATUS_COPY: Record<number, string> = {
  400: 'Revisa los datos e inténtalo de nuevo.',
  403: 'No tienes permiso para realizar esta operación sobre el rol.',
  404: 'El rol o la aplicación ya no existe.',
  409: 'La operación entra en conflicto con el estado actual del rol.',
}

// The API explains each refusal in the problem detail (control 2, 3, 4); field errors come first.
function roleProblems(reason: unknown, fallback: string): string[] {
  if (!(reason instanceof ApiProblemError)) return [connectionMessage]
  const fields = reason.errors.filter((item) => item.message).map((item) => `${item.field}: ${item.message}`)
  if (fields.length > 0) return fields
  return [reason.detail ?? STATUS_COPY[reason.status] ?? fallback]
}

const sameKeys = (a: ReadonlySet<string>, b: readonly string[]) => a.size === b.length && b.every((key) => a.has(key))
const users = (count: number) => (count === 1 ? '1 usuario' : `${count} usuarios`)

type Report = { onProblems: (messages: string[]) => void; onSessionEnded: () => void }

function RoleRow({ application, role, held, report, onUpdated, onDeleted }: {
  application: Application
  role: ApplicationRole
  held: boolean
  report: Report
  onUpdated: (role: ApplicationRole) => void
  onDeleted: (role: ApplicationRole) => void
}) {
  const [keys, setKeys] = useState<Set<string>>(() => new Set(role.permissionKeys))
  const [description, setDescription] = useState(role.description)
  const [confirming, setConfirming] = useState(false)
  const [pending, setPending] = useState(false)
  const noteId = useId()
  const dirty = !sameKeys(keys, role.permissionKeys) || description !== role.description
  const canDelete = !held && role.assignedCount === 0

  const toggle = (key: string) => setKeys((current) => {
    const next = new Set(current)
    if (!next.delete(key)) next.add(key)
    return next
  })

  const fail = (reason: unknown, fallback: string) => {
    if (isAuthFailure(reason)) report.onSessionEnded()
    else report.onProblems(roleProblems(reason, fallback))
    setPending(false)
  }

  const save = async () => {
    report.onProblems([])
    setPending(true)
    const catalog = application.permissions.map((permission) => permission.key)
    // Catalog order first, then any key the catalog does not list, so nothing is dropped silently.
    const permissionKeys = [...catalog.filter((key) => keys.has(key)), ...[...keys].filter((key) => !catalog.includes(key))]
    try {
      onUpdated(await updateApplicationRole(application.id, role.id, {
        ...(description !== role.description ? { description } : {}),
        ...(!sameKeys(keys, role.permissionKeys) ? { permissionKeys } : {}),
      }))
      // Do not rely on the parent remounting the row: if the server returns the same role, the
      // row would stay disabled.
      setPending(false)
    } catch (reason) {
      fail(reason, 'No fue posible guardar el rol. Inténtalo de nuevo.')
    }
  }

  const remove = async () => {
    report.onProblems([])
    setPending(true)
    try {
      await deleteApplicationRole(application.id, role.id)
      onDeleted(role)
    } catch (reason) {
      setConfirming(false)
      fail(reason, 'No fue posible eliminar el rol. Inténtalo de nuevo.')
    }
  }

  return (
    <tr>
      <th scope="row">{role.name}</th>
      <td>
        <input aria-label={`Descripción de ${role.name}`} value={description} maxLength={200} disabled={held || pending} onChange={(event) => setDescription(event.target.value)} />
      </td>
      {application.permissions.map((permission) => (
        <td className="grid-cell" key={permission.key}>
          <input type="checkbox" aria-label={`${permission.key} para ${role.name}`} checked={keys.has(permission.key)} disabled={held || pending} onChange={() => toggle(permission.key)} />
        </td>
      ))}
      <td>
        <div className="row-actions role-actions">
          {!held && <button className="secondary-button fit" disabled={!dirty || pending} onClick={() => void save()} type="button">{pending ? 'Guardando…' : 'Guardar'}<span className="sr-only"> {role.name}</span></button>}
          {!confirming && <button className="link-button" disabled={!canDelete || pending} aria-describedby={canDelete ? undefined : noteId} onClick={() => setConfirming(true)} type="button" aria-label={`Eliminar ${role.name}`}>Eliminar</button>}
          {confirming && (
            <>
              <button className="danger-button fit" disabled={pending} onClick={() => void remove()} type="button" aria-label={`Confirmar eliminación de ${role.name}`}>Confirmar eliminación</button>
              <button className="link-button" disabled={pending} onClick={() => setConfirming(false)} type="button" aria-label={`Cancelar eliminación de ${role.name}`}>Cancelar</button>
            </>
          )}
        </div>
        {held && <p className="hint" id={noteId}>Tienes este rol: solo otro admin puede cambiar sus permisos o eliminarlo.</p>}
        {!held && role.assignedCount > 0 && <p className="hint" id={noteId}>Asignado a {users(role.assignedCount)}: quita la asignación antes de eliminarlo.</p>}
      </td>
    </tr>
  )
}

function NewRoleForm({ application, report, onCreated, onCancel }: {
  application: Application
  report: Report
  onCreated: (role: ApplicationRole) => void
  onCancel: () => void
}) {
  const prefix = `${application.clientId}.`
  const ids = useId()
  const [name, setName] = useState(prefix)
  const [description, setDescription] = useState('')
  const [keys, setKeys] = useState<Set<string>>(new Set())
  const [pending, setPending] = useState(false)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const trimmed = name.trim()
    if (!trimmed.startsWith(prefix) || !ROLE_SLUG.test(trimmed.slice(prefix.length))) {
      report.onProblems([`El nombre debe ser ${prefix}<nombre>, con minúsculas, números, guion o guion bajo (de 2 a 41 caracteres, empezando por letra o número).`])
      return
    }
    report.onProblems([])
    setPending(true)
    try {
      onCreated(await createApplicationRole(application.id, {
        name: trimmed,
        ...(description.trim() ? { description: description.trim() } : {}),
        permissionKeys: application.permissions.map((permission) => permission.key).filter((key) => keys.has(key)),
      }))
    } catch (reason) {
      if (isAuthFailure(reason)) report.onSessionEnded()
      else report.onProblems(roleProblems(reason, 'No fue posible crear el rol. Inténtalo de nuevo.'))
      setPending(false)
    }
  }

  return (
    <form className="new-role-form" onSubmit={(event) => void submit(event)} noValidate>
      <h3>Nuevo rol de {application.name}</h3>
      <label htmlFor={`${ids}-name`}>Nombre del rol</label>
      <input id={`${ids}-name`} value={name} maxLength={80} autoComplete="off" required onChange={(event) => setName(event.target.value)} />
      <label htmlFor={`${ids}-description`}>Descripción del nuevo rol</label>
      <input id={`${ids}-description`} value={description} maxLength={200} autoComplete="off" onChange={(event) => setDescription(event.target.value)} />
      <fieldset className="role-fieldset">
        <legend>Permisos del nuevo rol</legend>
        {application.permissions.map((permission) => (
          <div className="role-option" key={permission.key}>
            <input id={`${ids}-${permission.key}`} type="checkbox" checked={keys.has(permission.key)} aria-describedby={`${ids}-${permission.key}-help`} onChange={() => setKeys((current) => {
              const next = new Set(current)
              if (!next.delete(permission.key)) next.add(permission.key)
              return next
            })} />
            <label htmlFor={`${ids}-${permission.key}`}>{permission.key}</label>
            <p className="hint" id={`${ids}-${permission.key}-help`}>{permission.description}</p>
          </div>
        ))}
      </fieldset>
      <div className="drawer-actions">
        <button className="primary-button fit" disabled={pending} type="submit">{pending ? 'Creando…' : 'Crear rol'}</button>
        <button className="secondary-button fit" disabled={pending} onClick={onCancel} type="button">Cancelar</button>
      </div>
    </form>
  )
}

function ApplicationSection({ application, currentRoles, onSessionEnded, onNotice }: { application: Application; currentRoles: readonly string[]; onSessionEnded: () => void; onNotice: (message: string) => void }) {
  const titleId = useId()
  // System roles (admin, user) are never edited here; only the application's own roles.
  const [roles, setRoles] = useState<ApplicationRole[]>(() => application.roles.filter((role) => !role.system))
  const [problems, setProblems] = useState<string[]>([])
  const [creating, setCreating] = useState(false)
  const report: Report = {
    onProblems: setProblems,
    onSessionEnded,
  }

  return (
    <section className="panel role-section" aria-labelledby={titleId}>
      <div className="panel-header">
        <div>
          <h2 id={titleId}>{application.name}</h2>
          <p className="muted">Aplicación <span className="mono">{application.clientId}</span>. Un cambio de permisos rige desde el siguiente token (hasta 15 minutos).</p>
        </div>
        {!creating && <button className="primary-button fit" onClick={() => setCreating(true)} type="button">Nuevo rol de {application.name}</button>}
      </div>
      <Problems messages={problems} />
      {roles.length === 0 ? <p className="muted">Esta aplicación todavía no tiene roles configurables.</p> : (
        <div className="table-scroll">
          <table className="data-table role-grid" aria-label={`Permisos de roles de ${application.name}`}>
            <thead>
              <tr>
                <th scope="col">Rol</th>
                <th scope="col">Descripción</th>
                {application.permissions.map((permission) => <th scope="col" key={permission.key} title={permission.description}>{permission.key}</th>)}
                <th scope="col">Acciones</th>
              </tr>
            </thead>
            <tbody>
              {roles.map((role) => (
                <RoleRow
                  key={`${role.id}:${role.description}:${role.permissionKeys.join(',')}`}
                  application={application}
                  role={role}
                  held={currentRoles.includes(role.name)}
                  report={report}
                  onUpdated={(updated) => { setRoles((current) => current.map((item) => (item.id === updated.id ? updated : item))); onNotice(`Rol ${updated.name} guardado.`) }}
                  onDeleted={(removed) => { setRoles((current) => current.filter((item) => item.id !== removed.id)); onNotice(`Rol ${removed.name} eliminado.`) }}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
      <dl className="permission-legend">
        {application.permissions.map((permission) => (
          <div key={permission.key}><dt className="mono">{permission.key}</dt><dd className="hint">{permission.description}</dd></div>
        ))}
      </dl>
      {creating && (
        <NewRoleForm
          application={application}
          report={report}
          onCancel={() => { setCreating(false); setProblems([]) }}
          onCreated={(created) => { setRoles((current) => [...current, created]); setCreating(false); onNotice(`Rol ${created.name} creado.`) }}
        />
      )}
    </section>
  )
}

export function RolesPage({ currentRoles, onSessionEnded }: { currentRoles: readonly string[]; onSessionEnded: () => void }) {
  const [applications, setApplications] = useState<Application[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const active = useRef(true)

  const load = useCallback(async () => {
    try {
      const loaded = await listApplications()
      if (active.current) setApplications(loaded)
    } catch (reason) {
      if (!active.current) return
      if (isAuthFailure(reason)) onSessionEnded()
      else setLoadError('No fue posible cargar las aplicaciones. Inténtalo de nuevo.')
    }
  }, [onSessionEnded])

  useEffect(() => {
    active.current = true
    void load()
    return () => { active.current = false }
  }, [load])

  return (
    <section>
      <div className="page-header">
        <div>
          <h1>Roles y permisos</h1>
          <p className="muted">Cada aplicación declara sus permisos; aquí decides qué permisos lleva cada rol. Los roles admin y user son del sistema y no se editan.</p>
        </div>
      </div>
      {loadError && <Problems messages={[loadError]} />}
      {applications === null && !loadError && <LoadingSkeleton label="Cargando roles…" />}
      {applications?.length === 0 && <p className="muted">No hay aplicaciones conectadas.</p>}
      {applications?.map((application) => <ApplicationSection application={application} currentRoles={currentRoles} key={application.id} onSessionEnded={onSessionEnded} onNotice={setNotice} />)}
      {notice && <Toast message={notice} onClose={() => setNotice(null)} />}
    </section>
  )
}
