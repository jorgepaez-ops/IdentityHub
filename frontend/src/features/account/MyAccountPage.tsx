import { FormEvent, useEffect, useState } from 'react'
import { type CurrentUser, type Session, listSessions, revokeSession, updateCurrentUser } from '../../api/client'
import { problemMessages } from './PublicPages'

const dateFormat = new Intl.DateTimeFormat('es', { dateStyle: 'medium', timeStyle: 'short' })
const formatDate = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : dateFormat.format(date)
}

function ProfileCard({ user, onUserChange }: { user: CurrentUser; onUserChange: (user: CurrentUser) => void }) {
  const [pending, setPending] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [problems, setProblems] = useState<string[]>([])
  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const displayName = String(new FormData(event.currentTarget).get('displayName')).trim()
    setPending(true)
    setNotice(null)
    setProblems([])
    try {
      onUserChange(await updateCurrentUser({ displayName }))
      setNotice('Perfil actualizado.')
    } catch (reason) {
      setProblems(problemMessages(reason, 'No fue posible guardar el perfil. Inténtalo de nuevo.'))
    } finally {
      setPending(false)
    }
  }
  return (
    <section className="panel" aria-labelledby="profile-title">
      <h2 id="profile-title">Perfil</h2>
      {problems.length > 0 && <div className="error-box" role="alert">{problems.map((message) => <p key={message}>{message}</p>)}</div>}
      {notice && <div className="success-box" role="status"><p>{notice}</p></div>}
      <form onSubmit={(event) => void save(event)} noValidate>
        <label htmlFor="profile-name">Nombre para mostrar</label>
        <input id="profile-name" name="displayName" defaultValue={user.displayName} maxLength={100} required />
        <label htmlFor="profile-email">Correo electrónico</label>
        <input id="profile-email" value={user.email} readOnly />
        <p className="field-label" id="roles-label">Roles</p>
        <ul className="chips" aria-labelledby="roles-label">
          {user.roles.map((role) => <li className={`chip chip-${role === 'admin' ? 'accent' : role === 'contabilidad.senior' ? 'warn' : 'neutral'}`} key={role}>{role}</li>)}
        </ul>
        <button className="primary-button fit" disabled={pending} type="submit">{pending ? 'Guardando…' : 'Guardar cambios'}</button>
      </form>
    </section>
  )
}

function SessionsCard({ onCurrentSessionRevoked }: { onCurrentSessionRevoked: () => void }) {
  const [sessions, setSessions] = useState<Session[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const items = await listSessions()
        if (active) setSessions(items)
      } catch {
        if (active) setError('No fue posible cargar tus sesiones. Inténtalo de nuevo.')
      }
    })()
    return () => { active = false }
  }, [])

  const revoke = async (session: Session) => {
    setBusyId(session.id)
    setError(null)
    try {
      await revokeSession(session.id)
      if (session.current) {
        onCurrentSessionRevoked()
        return
      }
      setSessions((items) => (items ?? []).filter((item) => item.id !== session.id))
    } catch {
      setError('No fue posible revocar la sesión. Inténtalo de nuevo.')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <section className="panel" aria-labelledby="sessions-title">
      <h2 id="sessions-title">Sesiones activas</h2>
      <p className="muted">Revoca las sesiones que no reconozcas. Si revocas la actual, se cerrará aquí mismo.</p>
      {error && <div className="error-box" role="alert"><p>{error}</p></div>}
      {sessions === null && !error && <p className="muted">Validando…</p>}
      {sessions && (
        <ul className="session-list" aria-label="Sesiones activas">
          {sessions.map((session) => (
            <li className="session-row" key={session.id} aria-label={`Sesión ${session.userAgent || session.ip}`}>
              <div>
                <p className="session-agent">{session.userAgent || 'Dispositivo desconocido'}{session.current && <span className="chip chip-ok">Esta sesión</span>}</p>
                <p className="muted"><span>{session.ip}</span></p>
                <p className="muted">Creada {formatDate(session.createdAt)} · Último uso {formatDate(session.lastUsedAt)}</p>
              </div>
              <button className="secondary-button fit" disabled={busyId !== null} onClick={() => void revoke(session)} type="button">Revocar</button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

export function MyAccountPage({ user, onUserChange, onSessionEnded }: {
  user: CurrentUser
  onUserChange: (user: CurrentUser) => void
  onSessionEnded: () => void
}) {
  return (
    <section>
      <h1>Mi cuenta</h1>
      <p className="muted">Tu perfil y las sesiones abiertas con tu cuenta.</p>
      <div className="account-grid">
        <ProfileCard user={user} onUserChange={onUserChange} />
        <SessionsCard onCurrentSessionRevoked={onSessionEnded} />
      </div>
    </section>
  )
}
