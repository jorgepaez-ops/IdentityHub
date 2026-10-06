import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { Toast } from '../../components/Toast'
import { DownloadIcon, SearchIcon, UsersIcon } from '../../components/icons'
import { type AdminUser, listUsers, resendInvitation } from '../../api/client'
import { csvFileName, downloadCsv, toCsv } from '../csv'
import { formatDate } from '../format'
import { type DrawerTarget, UserDrawer } from './UserDrawer'
import { Problems, RoleChips, StatusPill, adminProblems, initialsOf, isAuthFailure, parseStatusFilter, STATUS_FILTERS, STATUS_FILTER_LABEL, STATUS_LABEL, STATUS_PARAM } from './shared'

const PAGE_SIZE = 100
const SEARCH_DELAY_MS = 250

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

function emptyMessage(search: string, status: AdminUser['status'] | null): string {
  if (search) return 'No hay usuarios que coincidan con la búsqueda.'
  if (status) return 'No hay usuarios con este estado.'
  return 'Todavía no hay usuarios.'
}

export function UsersPage({ currentUserId, onSessionEnded }: { currentUserId: string; onSessionEnded: () => void }) {
  const [users, setUsers] = useState<AdminUser[] | null>(null)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [appliedSearch, setAppliedSearch] = useState('')
  const [params, setParams] = useSearchParams()
  const status = parseStatusFilter(params.get(STATUS_PARAM))
  const [loadingMore, setLoadingMore] = useState(false)
  const [drawer, setDrawer] = useState<DrawerTarget | null>(null)
  const [toast, setToast] = useState<{ text: string; id: number } | null>(null)
  const [resending, setResending] = useState<string | null>(null)
  const generation = useRef(0)
  const mounted = useRef(true)

  useEffect(() => () => {
    generation.current += 1
  }, [])
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const opener = useRef<HTMLElement | null>(null)
  // The id remounts the toast, so the same message shown twice restarts its timer.
  const showToast = useCallback((text: string) => setToast((current) => ({ text, id: (current?.id ?? 0) + 1 })), [])

  useEffect(() => {
    const timer = setTimeout(() => setAppliedSearch(search.trim()), SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [search])

  useEffect(() => {
    const mine = ++generation.current
    const current = () => mine === generation.current
    setLoadError(null)
    setNextCursor(null)
    setLoadingMore(false)
    void (async () => {
      try {
        const page = await listUsers({ limit: PAGE_SIZE, ...(appliedSearch ? { q: appliedSearch } : {}), ...(status ? { status } : {}) })
        if (!current()) return
        setUsers(page.items)
        setNextCursor(page.nextCursor ?? null)
      } catch (reason) {
        if (!current()) return
        if (isAuthFailure(reason)) onSessionEnded()
        else setLoadError('No fue posible cargar el directorio. Inténtalo de nuevo.')
      }
    })()
  }, [appliedSearch, status, onSessionEnded])

  const loadMore = async () => {
    if (!nextCursor) return
    const mine = generation.current
    setLoadingMore(true)
    try {
      const page = await listUsers({ limit: PAGE_SIZE, cursor: nextCursor, ...(appliedSearch ? { q: appliedSearch } : {}), ...(status ? { status } : {}) })
      if (mine !== generation.current) return
      setUsers((current) => [...(current ?? []), ...page.items])
      setNextCursor(page.nextCursor ?? null)
    } catch (reason) {
      if (mine !== generation.current) return
      if (isAuthFailure(reason)) onSessionEnded()
      else setLoadError('No fue posible cargar el directorio. Inténtalo de nuevo.')
    } finally {
      if (mine === generation.current) setLoadingMore(false)
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
      if (!mounted.current) return
      showToast(`Invitación reenviada a ${user.email}.`)
    } catch (reason) {
      if (!mounted.current) return
      if (isAuthFailure(reason)) onSessionEnded()
      else showToast(adminProblems(reason, RESEND_COPY, 'No fue posible reenviar la invitación. Inténtalo de nuevo.')[0] ?? '')
    } finally {
      if (mounted.current) setResending(null)
    }
  }

  // The status lives in the URL so a reload or a shared link keeps it; "all" removes the parameter.
  const chooseStatus = (next: AdminUser['status'] | null) => {
    setParams((current) => {
      const updated = new URLSearchParams(current)
      if (next) updated.set(STATUS_PARAM, next)
      else updated.delete(STATUS_PARAM)
      return updated
    }, { replace: true })
  }

  const list = users ?? []
  // Exports what is loaded (the filtered pages the table shows): fetching every page would contradict D3.
  const exportUsers = () => {
    const header = ['Correo', 'Nombre', 'Estado', 'Roles', 'MFA', 'Último acceso', 'Creado']
    const rows = list.map((user) => [user.email, user.displayName, STATUS_LABEL[user.status], user.roles.join('; '), user.mfaEnabled ? 'Sí' : 'No', user.lastLoginAt, user.createdAt])
    downloadCsv(csvFileName('usuarios'), toCsv(header, rows))
    showToast(`Exportados ${rows.length} registros`)
  }
  const count = (status: AdminUser['status']) => list.filter((user) => user.status === status).length

  return (
    <section>
      <div className="page-header">
        <div>
          <h1>Usuarios</h1>
          <p className="muted">Cuentas del Hub, sus roles y el estado de cada invitación.</p>
        </div>
        <button className="primary-button fit" onClick={() => openDrawer({ mode: 'create' })} type="button"><UsersIcon />Nuevo usuario</button>
      </div>
      {/* These counts come from the loaded list, so under a status filter they would report zeros for the other states. */}
      {status === null && (
        <>
          <div className="stat-grid">
            <StatTile label="Usuarios activos" value={count('active')} />
            <StatTile label="Cuentas bloqueadas" value={count('locked')} />
            <StatTile label="Invitaciones pendientes" value={count('pending_verification')} />
          </div>
          {nextCursor && <p className="hint">Los totales cuentan solo los usuarios cargados; hay más en el directorio.</p>}
        </>
      )}
      <section className="panel directory" aria-labelledby="directory-title">
        <div className="panel-header">
          <h2 id="directory-title">Directorio</h2>
          <div className="search-field">
            <label htmlFor="user-search">Buscar por correo o nombre</label>
            <div className="input-with-icon"><SearchIcon /><input id="user-search" type="search" value={search} maxLength={200} onChange={(event) => setSearch(event.target.value)} placeholder="Buscar…" /></div>
          </div>
        </div>
        <div className="filter-group" role="group" aria-label="Filtrar por estado">
          {STATUS_FILTERS.map((option) => (
            <button key={option ?? 'all'} className="filter-button" type="button" aria-pressed={status === option} onClick={() => chooseStatus(option)}>{STATUS_FILTER_LABEL[option ?? 'all']}</button>
          ))}
        </div>
        <div className="export-row">
          <button className="secondary-button fit" type="button" disabled={list.length === 0} onClick={exportUsers}><DownloadIcon />Exportar CSV</button>
          {nextCursor && list.length > 0 && <p className="hint">Exporta los {list.length} cargados; carga más para incluir el resto.</p>}
        </div>
        {loadError && <Problems messages={[loadError]} />}
        {users === null && !loadError && <LoadingSkeleton label="Cargando usuarios…" />}
        {users !== null && list.length === 0 && <p className="muted">{emptyMessage(appliedSearch, status)}</p>}
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
      {toast && <Toast key={toast.id} message={toast.text} onClose={() => setToast(null)} />}
    </section>
  )
}
