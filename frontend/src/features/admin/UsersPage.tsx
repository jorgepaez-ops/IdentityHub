import { useCallback, useEffect, useRef, useState } from 'react'
import { type AdminUser, listUsers, resendInvitation } from '../../api/client'
import { formatDate } from '../format'
import { type DrawerTarget, UserDrawer } from './UserDrawer'
import { Problems, RoleChips, StatusPill, adminProblems, initialsOf, isAuthFailure } from './shared'

const PAGE_SIZE = 100
const SEARCH_DELAY_MS = 250
const TOAST_MS = 6000

const RESEND_COPY: Record<number, string> = {
  403: 'No tienes permiso para reenviar invitaciones.',
  404: 'La cuenta ya no existe.',
  409: 'La cuenta ya no está pendiente: no hace falta reenviar la invitación.',
}

function StatTile({ label, value }: { label: string; value: number }) {
  return (
    <div className="stat-tile" role="group" aria-label={label}>
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  )
}

export function UsersPage({ currentUserId, onSessionEnded }: { currentUserId: string; onSessionEnded: () => void }) {
  const [users, setUsers] = useState<AdminUser[] | null>(null)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [appliedSearch, setAppliedSearch] = useState('')
  const [loadingMore, setLoadingMore] = useState(false)
  const [drawer, setDrawer] = useState<DrawerTarget | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const [resending, setResending] = useState<string | null>(null)
  const opener = useRef<HTMLElement | null>(null)
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const showToast = useCallback((message: string) => {
    setToast(message)
    if (toastTimer.current) clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setToast(null), TOAST_MS)
  }, [])
  useEffect(() => () => { if (toastTimer.current) clearTimeout(toastTimer.current) }, [])

  useEffect(() => {
    const timer = setTimeout(() => setAppliedSearch(search.trim()), SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [search])

  useEffect(() => {
    let active = true
    setLoadError(null)
    void (async () => {
      try {
        const page = await listUsers({ limit: PAGE_SIZE, ...(appliedSearch ? { q: appliedSearch } : {}) })
        if (!active) return
        setUsers(page.items)
        setNextCursor(page.nextCursor ?? null)
      } catch (reason) {
        if (!active) return
        if (isAuthFailure(reason)) onSessionEnded()
        else setLoadError('No fue posible cargar el directorio. Inténtalo de nuevo.')
      }
    })()
    return () => { active = false }
  }, [appliedSearch, onSessionEnded])

  const loadMore = async () => {
    if (!nextCursor) return
    setLoadingMore(true)
    try {
      const page = await listUsers({ limit: PAGE_SIZE, cursor: nextCursor, ...(appliedSearch ? { q: appliedSearch } : {}) })
      setUsers((current) => [...(current ?? []), ...page.items])
      setNextCursor(page.nextCursor ?? null)
    } catch (reason) {
      if (isAuthFailure(reason)) onSessionEnded()
      else setLoadError('No fue posible cargar el directorio. Inténtalo de nuevo.')
    } finally {
      setLoadingMore(false)
    }
  }

  const openDrawer = (target: DrawerTarget) => {
    opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    setDrawer(target)
  }
  const closeDrawer = useCallback(() => {
    setDrawer(null)
    opener.current?.focus()
  }, [])

  const saved = (user: AdminUser, message: string) => {
    setUsers((current) => {
      const list = current ?? []
      return list.some((item) => item.id === user.id) ? list.map((item) => (item.id === user.id ? user : item)) : [user, ...list]
    })
    closeDrawer()
    showToast(message)
  }

  const resend = async (user: AdminUser) => {
    setResending(user.id)
    try {
      await resendInvitation(user.id)
      showToast(`Invitación reenviada a ${user.email}.`)
    } catch (reason) {
      if (isAuthFailure(reason)) onSessionEnded()
      else showToast(adminProblems(reason, RESEND_COPY, 'No fue posible reenviar la invitación. Inténtalo de nuevo.')[0] ?? '')
    } finally {
      setResending(null)
    }
  }

  const list = users ?? []
  const count = (status: AdminUser['status']) => list.filter((user) => user.status === status).length

  return (
    <section>
      <div className="page-header">
        <div>
          <h1>Usuarios</h1>
          <p className="muted">Cuentas del Hub, sus roles y el estado de cada invitación.</p>
        </div>
        <button className="primary-button fit" onClick={() => openDrawer({ mode: 'create' })} type="button">Nuevo usuario</button>
      </div>
      <div className="stat-grid">
        <StatTile label="Usuarios activos" value={count('active')} />
        <StatTile label="Cuentas bloqueadas" value={count('locked')} />
        <StatTile label="Invitaciones pendientes" value={count('pending_verification')} />
      </div>
      {nextCursor && <p className="hint">Los totales cuentan solo los usuarios cargados; hay más en el directorio.</p>}
      <section className="panel directory" aria-labelledby="directory-title">
        <div className="panel-header">
          <h2 id="directory-title">Directorio</h2>
          <div className="search-field">
            <label htmlFor="user-search">Buscar por correo o nombre</label>
            <input id="user-search" type="search" value={search} maxLength={200} onChange={(event) => setSearch(event.target.value)} placeholder="Buscar…" />
          </div>
        </div>
        {loadError && <Problems messages={[loadError]} />}
        {users === null && !loadError && <p className="muted">Cargando…</p>}
        {users !== null && list.length === 0 && <p className="muted">{appliedSearch ? 'No hay usuarios que coincidan con la búsqueda.' : 'Todavía no hay usuarios.'}</p>}
        {list.length > 0 && (
          <div className="table-scroll">
            <table className="data-table" aria-label="Directorio">
              <thead>
                <tr><th scope="col">Usuario</th><th scope="col">Estado</th><th scope="col">Roles</th><th scope="col">Último acceso</th><th scope="col"><span className="sr-only">Acciones</span></th></tr>
              </thead>
              <tbody>
                {list.map((user) => (
                  <tr key={user.id}>
                    <td>
                      <div className="user-cell">
                        <span className="avatar" aria-hidden="true">{initialsOf(user)}</span>
                        <div><strong>{user.displayName}</strong><span className="muted block">{user.email}</span></div>
                      </div>
                    </td>
                    <td><StatusPill status={user.status} /></td>
                    <td><RoleChips roles={user.roles} /></td>
                    <td>{user.lastLoginAt ? formatDate(user.lastLoginAt) : <span className="muted">Sin accesos</span>}</td>
                    <td>
                      <div className="row-actions">
                        <button className="link-button" onClick={() => openDrawer({ mode: 'edit', user })} type="button" aria-label={`Editar a ${user.displayName}`}>Editar</button>
                        {user.status === 'pending_verification' && (
                          <button className="link-button" disabled={resending !== null} onClick={() => void resend(user)} type="button" aria-label={`Reenviar invitación a ${user.displayName}`}>Reenviar invitación</button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {nextCursor && <button className="secondary-button fit" disabled={loadingMore} onClick={() => void loadMore()} type="button">{loadingMore ? 'Cargando…' : 'Cargar más'}</button>}
      </section>
      {drawer && <UserDrawer target={drawer} currentUserId={currentUserId} onClose={closeDrawer} onSaved={saved} onSessionEnded={onSessionEnded} />}
      {toast && <div className="toast" role="status">{toast}</div>}
    </section>
  )
}
