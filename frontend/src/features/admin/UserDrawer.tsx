import { FormEvent, KeyboardEvent, useEffect, useId, useRef, useState } from 'react'
import { type AdminUser, type Role, type UserStatus, createUser, updateUser } from '../../api/client'
import { ROLES, ROLE_CATALOG, Problems, STATUS_LABEL, adminProblems, isAuthFailure } from './shared'

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

export function UserDrawer({ target, currentUserId, onClose, onSaved, onSessionEnded }: {
  target: DrawerTarget
  currentUserId: string
  onClose: () => void
  onSaved: (user: AdminUser, message: string) => void
  onSessionEnded: () => void
}) {
  const editing = target.mode === 'edit' ? target.user : null
  const isSelf = editing?.id === currentUserId
  const titleId = useId()
  const dialogRef = useRef<HTMLElement>(null)
  const [status, setStatus] = useState<UserStatus>(editing?.status ?? 'active')
  const [roles, setRoles] = useState<Role[]>(() => ROLES.filter((role) => (editing ? editing.roles.includes(role) : role === 'user')))
  const [problems, setProblems] = useState<string[]>([])
  const [pending, setPending] = useState(false)

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    const field = dialog.querySelector<HTMLElement>('form input:not([readonly]):not([disabled]), form select:not([disabled])')
    ;(field ?? dialog.querySelector<HTMLElement>('button'))?.focus()
  }, [])

  useEffect(() => {
    const onKeyDown = (event: globalThis.KeyboardEvent) => { if (event.key === 'Escape') onClose() }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  const trapTab = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Tab' || !dialogRef.current) return
    const items = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(FOCUSABLE))
    const first = items[0]
    const last = items[items.length - 1]
    if (!first || !last) return
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  const toggleRole = (role: Role) =>
    setRoles((current) => ROLES.filter((item) => (item === role ? !current.includes(item) : current.includes(item))))

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setProblems([])
    if (editing) {
      const changes = {
        ...(status !== editing.status ? { status } : {}),
        ...(!sameRoles(roles, editing.roles) ? { roles } : {}),
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
    const displayName = String(data.get('displayName')).trim()
    const email = String(data.get('email')).trim()
    if (displayName === '') {
      setProblems(['El nombre para mostrar no puede estar vacío.'])
      return
    }
    setPending(true)
    try {
      const user = await createUser({ email, displayName, roles })
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
            {ROLES.map((role) => {
              const locked = role === 'user' || isSelf
              return (
                <div className="role-option" key={role}>
                  <input id={`role-${role}`} type="checkbox" value={role} checked={roles.includes(role)} disabled={locked} aria-describedby={`role-${role}-help`} onChange={() => toggleRole(role)} />
                  <label htmlFor={`role-${role}`}>{role}</label>
                  <p className="hint" id={`role-${role}-help`}>{ROLE_CATALOG[role]}</p>
                </div>
              )
            })}
            {isSelf && <p className="danger-note">Un admin no puede asignarse roles a sí mismo; otro admin debe hacerlo (RF-010).</p>}
          </fieldset>
          <div className="drawer-actions">
            <button className="primary-button fit" disabled={pending || isSelf} type="submit">
              {editing ? (pending ? 'Guardando…' : 'Guardar cambios') : (pending ? 'Creando…' : 'Crear y enviar invitación')}
            </button>
            <button className="secondary-button fit" onClick={onClose} type="button">Cancelar</button>
          </div>
        </form>
      </aside>
    </div>
  )
}
