import { FormEvent, KeyboardEvent, useEffect, useId, useRef, useState } from 'react'
import { type AdminUser, type Application, type UserStatus, createUser, listApplications, updateUser } from '../../api/client'
import { DIRECTORY_ROLES, DIRECTORY_ROLE_DESCRIPTION, Problems, STATUS_LABEL, adminProblems, isAuthFailure } from './shared'
import { fieldValue } from '../../formData'

export type DrawerTarget = { mode: 'create' } | { mode: 'edit'; user: AdminUser }

const EDITABLE_STATUS: UserStatus[] = ['active', 'locked', 'disabled']
const FOCUSABLE = 'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled])'
const sameRoles = (a: readonly string[], b: readonly string[]) => a.length === b.length && a.every((role) => b.includes(role))

const CREATE_COPY: Record<number, string> = {
  400: 'Revisa los datos e inténtalo de nuevo.',
  403: 'No tienes permiso para crear usuarios.',
  409: 'No se pudo crear la cuenta: ese correo ya está registrado o no está disponible.',
}
const EDIT_COPY: Record<number, string> = {
  400: 'El cambio no está permitido: un admin no puede deshabilitarse ni asignarse roles a sí mismo, y siempre debe quedar un admin activo.',
  403: 'No tienes permiso para modificar usuarios.',
  404: 'La cuenta ya no existe.',
}

export function UserDrawer({ target, currentUserId, onClose, onSaved, onSessionEnded }: Readonly<{
  target: DrawerTarget
  currentUserId: string
  onClose: () => void
  onSaved: (user: AdminUser, message: string) => void
  onSessionEnded: () => void
}>) {
  const editing = target.mode === 'edit' ? target.user : null
  const isSelf = editing?.id === currentUserId
  const idleLabel = editing ? 'Guardar cambios' : 'Crear y enviar invitación'
  const busyLabel = editing ? 'Guardando…' : 'Creando…'
  const titleId = useId()
  const dialogRef = useRef<HTMLElement>(null)
  const [status, setStatus] = useState<UserStatus>(editing?.status ?? 'active')
  // Held roles keep whatever the account already has, even a role the catalog did not return.
  const [roles, setRoles] = useState<string[]>(() => (editing ? [...editing.roles] : ['user']))
  const [applications, setApplications] = useState<Application[]>([])
  const [catalogError, setCatalogError] = useState(false)
  const [problems, setProblems] = useState<string[]>([])
  const [pending, setPending] = useState(false)

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    const field = dialog.querySelector<HTMLElement>('form input:not([readonly]):not([disabled]), form select:not([disabled])')
    ;(field ?? dialog.querySelector<HTMLElement>('button'))?.focus()
  }, [])

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const loaded = await listApplications()
        if (active) setApplications(loaded)
      } catch (reason) {
        if (!active) return
        if (isAuthFailure(reason)) onSessionEnded()
        else setCatalogError(true)
      }
    })()
    return () => { active = false }
  }, [onSessionEnded])

  useEffect(() => {
    const onKeyDown = (event: globalThis.KeyboardEvent) => { if (event.key === 'Escape') onClose() }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  const trapTab = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Tab' || !dialogRef.current) return
    const items = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(FOCUSABLE))
    const first = items[0]
    const last = items.at(-1)
    if (!first || !last) return
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  // Application roles come from the API (RF-021); system roles are the two directory ones above.
  const groups = applications
    .map((application) => ({ application, roles: application.roles.filter((role) => !role.system) }))
    .filter((group) => group.roles.length > 0)
  const catalog = [...DIRECTORY_ROLES as readonly string[], ...groups.flatMap((group) => group.roles.map((role) => role.name))]
  // Stable order: catalog order first, then roles the catalog does not know about.
  const ordered = (held: readonly string[]) => [...catalog.filter((role) => held.includes(role)), ...held.filter((role) => !catalog.includes(role))]
  const toggleRole = (role: string) =>
    setRoles((current) => ordered(current.includes(role) ? current.filter((item) => item !== role) : [...current, role]))

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setProblems([])
    if (editing) {
      const changes = {
        ...(status !== editing.status ? { status } : {}),
        ...(!sameRoles(roles, editing.roles) ? { roles: ordered(roles) } : {}),
      }
      if (Object.keys(changes).length === 0) {
        onClose()
        return
      }
      setPending(true)
      try {
        onSaved(await updateUser(editing.id, changes), 'Cambios guardados.')
      } catch (reason) {
        if (isAuthFailure(reason)) onSessionEnded()
        else setProblems(adminProblems(reason, EDIT_COPY, 'No fue posible guardar los cambios. Inténtalo de nuevo.'))
        setPending(false)
      }
      return
    }
    const data = new FormData(event.currentTarget)
    const displayName = fieldValue(data, 'displayName').trim()
    const email = fieldValue(data, 'email').trim()
    if (displayName === '') {
      setProblems(['El nombre para mostrar no puede estar vacío.'])
      return
    }
    setPending(true)
    try {
      const user = await createUser({ email, displayName, roles: ordered(roles) })
      onSaved(user, `Invitación enviada a ${user.email}.`)
    } catch (reason) {
      if (isAuthFailure(reason)) onSessionEnded()
      else setProblems(adminProblems(reason, CREATE_COPY, 'No fue posible crear la cuenta. Inténtalo de nuevo.'))
      setPending(false)
    }
  }

  return (
    <div className="drawer-layer">
      <div className="drawer-backdrop" onClick={onClose} aria-hidden="true" />
      <aside className="drawer" role="dialog" aria-modal="true" aria-labelledby={titleId} ref={dialogRef} onKeyDown={trapTab}>
        <header className="drawer-header">
          <h2 id={titleId}>{editing ? 'Editar usuario' : 'Nuevo usuario'}</h2>
          <button className="icon-button" onClick={onClose} type="button" aria-label="Cerrar">×</button>
        </header>
        <form onSubmit={(event) => void submit(event)} noValidate>
          <Problems messages={problems} />
          <label htmlFor="drawer-name">Nombre para mostrar</label>
          <input id="drawer-name" name="displayName" defaultValue={editing?.displayName ?? ''} maxLength={100} readOnly={editing !== null} autoComplete="off" required />
          <label htmlFor="drawer-email">Correo electrónico</label>
          <input id="drawer-email" name="email" type="email" defaultValue={editing?.email ?? ''} maxLength={254} readOnly={editing !== null} autoComplete="off" required />
          {editing && (
            <>
              <label htmlFor="drawer-status">Estado</label>
              <select id="drawer-status" value={status} disabled={isSelf} onChange={(event) => setStatus(event.target.value as UserStatus)}>
                {editing.status === 'pending_verification' && <option value="pending_verification">{STATUS_LABEL.pending_verification}</option>}
                {EDITABLE_STATUS.map((value) => <option key={value} value={value}>{STATUS_LABEL[value]}</option>)}
              </select>
              {isSelf && <p className="danger-note">Un admin no puede deshabilitarse a sí mismo (RF-010).</p>}
            </>
          )}
          <fieldset className="role-fieldset">
            <legend>Roles</legend>
            {DIRECTORY_ROLES.map((role) => (
              <RoleOption key={role} role={role} checked={roles.includes(role)} disabled={role === 'user' || isSelf} hint={DIRECTORY_ROLE_DESCRIPTION[role] ?? ''} onToggle={toggleRole} />
            ))}
            {catalogError && <p className="danger-note" role="alert">No fue posible cargar los roles de las aplicaciones. Los roles del Hub siguen disponibles.</p>}
            {groups.map(({ application, roles: applicationRoles }) => (
              <fieldset className="role-fieldset role-group" key={application.id}>
                <legend>{application.name}</legend>
                {applicationRoles.map((role) => (
                  <RoleOption
                    key={role.id}
                    role={role.name}
                    checked={roles.includes(role.name)}
                    disabled={isSelf}
                    hint={role.description}
                    permissions={role.permissionKeys.length > 0 ? `Permisos: ${role.permissionKeys.join(', ')}` : 'Sin permisos'}
                    onToggle={toggleRole}
                  />
                ))}
              </fieldset>
            ))}
            {isSelf && <p className="danger-note">Un admin no puede asignarse roles a sí mismo; otro admin debe hacerlo (RF-010).</p>}
          </fieldset>
          <div className="drawer-actions">
            <button className="primary-button fit" disabled={pending || isSelf} type="submit">
              {pending ? busyLabel : idleLabel}
            </button>
            <button className="secondary-button fit" onClick={onClose} type="button">Cancelar</button>
          </div>
        </form>
      </aside>
    </div>
  )
}

function RoleOption({ role, checked, disabled, hint, permissions, onToggle }: Readonly<{ role: string; checked: boolean; disabled: boolean; hint: string; permissions?: string; onToggle: (role: string) => void }>) {
  return (
    <div className="role-option">
      <input id={`role-${role}`} type="checkbox" value={role} checked={checked} disabled={disabled} aria-describedby={`role-${role}-help`} onChange={() => onToggle(role)} />
      <label htmlFor={`role-${role}`}>{role}</label>
      <p className="hint" id={`role-${role}-help`}>{hint}{permissions && <span className="block">{permissions}</span>}</p>
    </div>
  )
}
