import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { AuditIcon, ShieldIcon, UsersIcon } from '../../components/icons'
import { type AuditEvent, type UserStatus, listAuditLog, listUsers } from '../../api/client'
import { formatDate } from '../format'
import { AUDIT_FAILED_SIGN_IN_PATH, FAILED_SIGN_IN_ACTION, MFA_CODE_REJECTED_ACTION, Problems, STATUS_PARAM, isAuthFailure } from './shared'

// D3: the API has no totals endpoint, so a count is exact up to one full page and "N+" beyond it.
const COUNT_PAGE = 100
const RECENT_LIMIT = 10
const DAY_MS = 24 * 60 * 60 * 1000

const STATUS_ORDER: UserStatus[] = ['active', 'pending_verification', 'locked', 'disabled']
const STATUS_TILE_LABEL: Record<UserStatus, string> = {
  active: 'Activos',
  pending_verification: 'Pendientes',
  locked: 'Bloqueados',
  disabled: 'Deshabilitados',
}

type Count = { value: number; more: boolean }
type UserCounts = Record<UserStatus, Count>
// Each group loads on its own, so one failing request only blanks its own part of the page.
type Group<T> = { state: 'loading' } | { state: 'ready'; data: T } | { state: 'error' }
type Snapshot = { users: Group<UserCounts>; failedSignIns: Group<Count>; recent: Group<AuditEvent[]> }

const LOADING: Snapshot = { users: { state: 'loading' }, failedSignIns: { state: 'loading' }, recent: { state: 'loading' } }
const GROUP_ERROR = {
  users: 'No fue posible cargar los conteos de usuarios.',
  failedSignIns: 'No fue posible cargar los inicios de sesión fallidos.',
  recent: 'No fue posible cargar la actividad reciente.',
} as const

// A rejected MFA code is a failed sign-in too (specs/01-requirements.md), but the audit log records it as its own action.
const FAILED_SIGN_IN_ACTIONS = [FAILED_SIGN_IN_ACTION, MFA_CODE_REJECTED_ACTION] as const
const STATUS_TONE: Record<UserStatus, string> = { active: 'ok', pending_verification: 'warn', locked: 'danger', disabled: 'danger' }

const countOf = (page: { items: unknown[]; nextCursor?: string | null }): Count => ({ value: page.items.length, more: Boolean(page.nextCursor) })
const sumCounts = (counts: Count[]): Count => ({ value: counts.reduce((total, count) => total + count.value, 0), more: counts.some((count) => count.more) })
const countText = (count: Count) => (count.more ? `${count.value}+` : String(count.value))

async function loadUsers(): Promise<UserCounts> {
  const pages = await Promise.all(STATUS_ORDER.map((status) => listUsers({ status, limit: COUNT_PAGE })))
  return Object.fromEntries(STATUS_ORDER.map((status, index) => [status, countOf(pages[index] ?? { items: [] })])) as UserCounts
}

async function loadFailedSignIns(): Promise<Count> {
  const since = new Date(Date.now() - DAY_MS).toISOString()
  const pages = await Promise.all(FAILED_SIGN_IN_ACTIONS.map((action) => listAuditLog({ action, since, limit: COUNT_PAGE })))
  return sumCounts(pages.map(countOf))
}

async function loadRecent(): Promise<AuditEvent[]> {
  return (await listAuditLog({ limit: RECENT_LIMIT })).items
}


// A tile with `to` keeps its group name and count; its label becomes a link stretched over the whole tile.
function StatTile({ label, count, tone, to }: Readonly<{ label: string; count: Count; tone?: string; to?: string }>) {
  const classes = ['stat-tile', tone ? `stat-${tone}` : '', to ? 'stat-link' : ''].filter(Boolean).join(' ')
  return (
    <div className={classes} role="group" aria-label={label}>
      <strong>{countText(count)}</strong>
      {to ? <Link to={to}>{label}</Link> : <span>{label}</span>}
    </div>
  )
}

export function HomePage({ onSessionEnded }: Readonly<{ onSessionEnded: () => void }>) {
  const [snapshot, setSnapshot] = useState<Snapshot>(LOADING)
  const [attempt, setAttempt] = useState(0)
  const generation = useRef(0)

  useEffect(() => {
    const mine = ++generation.current
    let ended = false
    // Each group renders as soon as it settles, so a slow or failing one never holds the others back.
    const run = <K extends keyof Snapshot>(key: K, load: () => Promise<Extract<Snapshot[K], { state: 'ready' }>['data']>) => {
      load().then(
        (data) => {
          if (mine === generation.current) setSnapshot((current) => ({ ...current, [key]: { state: 'ready', data } }))
        },
        (reason: unknown) => {
          if (mine !== generation.current) return
          // A 401 that survived the client refresh in any group ends the session, once.
          if (isAuthFailure(reason)) {
            if (!ended) onSessionEnded()
            ended = true
            return
          }
          setSnapshot((current) => ({ ...current, [key]: { state: 'error' } }))
        },
      )
    }
    run('users', loadUsers)
    run('failedSignIns', loadFailedSignIns)
    run('recent', loadRecent)
    return () => {
      generation.current += 1
    }
  }, [onSessionEnded, attempt])

  const retry = () => {
    setSnapshot(LOADING)
    setAttempt((current) => current + 1)
  }
  const failed = (Object.keys(GROUP_ERROR) as (keyof typeof GROUP_ERROR)[]).filter((key) => snapshot[key].state === 'error')
  const loading = snapshot.users.state === 'loading' && snapshot.failedSignIns.state === 'loading' && snapshot.recent.state === 'loading'
  const { users, failedSignIns, recent } = snapshot

  return (
    <section>
      <h1>Inicio</h1>
      <p className="muted">Resumen del Hub con los datos actuales de la API. Los conteos grandes se muestran como «100+».</p>
      {failed.length > 0 && (
        <>
          <Problems messages={failed.map((key) => GROUP_ERROR[key])} />
          <button className="secondary-button" type="button" onClick={retry}>Reintentar</button>
        </>
      )}
      {loading && <LoadingSkeleton rows={4} label="Cargando indicadores…" />}
      {!loading && (
        <>
          <div className="stat-grid" aria-label="Indicadores">
            {users.state === 'ready' && STATUS_ORDER.map((status) => <StatTile key={status} label={STATUS_TILE_LABEL[status]} count={users.data[status]} tone={STATUS_TONE[status]} to={`/usuarios?${STATUS_PARAM}=${status}`} />)}
            {(users.state === 'loading' || failedSignIns.state === 'loading') && <p className="muted">Cargando conteos de usuarios…</p>}
            {failedSignIns.state === 'ready' && <StatTile label="Inicios de sesión fallidos (24 h)" count={failedSignIns.data} tone={failedSignIns.data.value > 0 ? 'danger' : 'ok'} to={AUDIT_FAILED_SIGN_IN_PATH} />}
          </div>
          <div className="home-grid">
            <section className="panel" aria-labelledby="home-activity-title">
              <h2 id="home-activity-title">Actividad reciente</h2>
              {recent.state === 'ready' && (recent.data.length === 0
                ? <p className="muted">Todavía no hay actividad registrada.</p>
                : (
                  <ul className="activity-list" aria-label="Actividad reciente">
                    {recent.data.map((event) => (
                      <li key={event.id}><code>{event.action}</code><time dateTime={event.createdAt}>{formatDate(event.createdAt)}</time></li>
                    ))}
                  </ul>
                ))}
              {recent.state === 'loading' && <p className="muted">Cargando actividad…</p>}
            </section>
            <section className="panel shortcuts" aria-label="Accesos directos">
              <h2>Accesos directos</h2>
              <Link to="/usuarios"><UsersIcon />Usuarios<small>Directorio, invitaciones y estados</small></Link>
              <Link to="/roles"><ShieldIcon />Roles<small>Permisos de las aplicaciones</small></Link>
              <Link to="/auditoria"><AuditIcon />Auditoría<small>Eventos de seguridad</small></Link>
            </section>
          </div>
        </>
      )}
    </section>
  )
}
