import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { AuditIcon, ShieldIcon, UsersIcon } from '../../components/icons'
import { type AuditEvent, type UserStatus, listAuditLog, listUsers } from '../../api/client'
import { formatDate } from '../format'
import { AUDIT_FAILED_SIGN_IN_PATH, FAILED_SIGN_IN_ACTION, Problems, STATUS_PARAM, isAuthFailure } from './shared'

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
type Snapshot = { users: Record<UserStatus, Count>; failedSignIns: Count; recent: AuditEvent[] }

// A rejected MFA code is a failed sign-in too (specs/01-requirements.md), but the audit log records it as its own action.
const FAILED_SIGN_IN_ACTIONS = [FAILED_SIGN_IN_ACTION, 'mfa_code_rejected'] as const
const STATUS_TONE: Record<UserStatus, string> = { active: 'ok', pending_verification: 'warn', locked: 'danger', disabled: 'danger' }

const countOf = (page: { items: unknown[]; nextCursor?: string | null }): Count => ({ value: page.items.length, more: Boolean(page.nextCursor) })
const sumCounts = (counts: Count[]): Count => ({ value: counts.reduce((total, count) => total + count.value, 0), more: counts.some((count) => count.more) })
const countText = (count: Count) => (count.more ? `${count.value}+` : String(count.value))

async function loadSnapshot(): Promise<Snapshot> {
  const since = new Date(Date.now() - DAY_MS).toISOString()
  const [pages, failed, recent] = await Promise.all([
    Promise.all(STATUS_ORDER.map((status) => listUsers({ status, limit: COUNT_PAGE }))),
    Promise.all(FAILED_SIGN_IN_ACTIONS.map((action) => listAuditLog({ action, since, limit: COUNT_PAGE }))),
    listAuditLog({ limit: RECENT_LIMIT }),
  ])
  const users = Object.fromEntries(STATUS_ORDER.map((status, index) => [status, countOf(pages[index] ?? { items: [] })])) as Record<UserStatus, Count>
  return { users, failedSignIns: sumCounts(failed.map(countOf)), recent: recent.items }
}

// A tile with `to` keeps its group name and count; its label becomes a link stretched over the whole tile.
function StatTile({ label, count, tone, to }: { label: string; count: Count; tone?: string; to?: string }) {
  const classes = ['stat-tile', tone ? `stat-${tone}` : '', to ? 'stat-link' : ''].filter(Boolean).join(' ')
  return (
    <div className={classes} role="group" aria-label={label}>
      <strong>{countText(count)}</strong>
      {to ? <Link to={to}>{label}</Link> : <span>{label}</span>}
    </div>
  )
}

export function HomePage({ onSessionEnded }: { onSessionEnded: () => void }) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [error, setError] = useState<string | null>(null)
  const generation = useRef(0)

  useEffect(() => {
    const mine = ++generation.current
    void (async () => {
      try {
        const data = await loadSnapshot()
        if (mine === generation.current) setSnapshot(data)
      } catch (reason) {
        if (mine !== generation.current) return
        if (isAuthFailure(reason)) onSessionEnded()
        else setError('No fue posible cargar los indicadores. Inténtalo de nuevo.')
      }
    })()
    return () => {
      generation.current += 1
    }
  }, [onSessionEnded])

  return (
    <section>
      <h1>Inicio</h1>
      <p className="muted">Resumen del Hub con los datos actuales de la API. Los conteos grandes se muestran como «100+».</p>
      {error && <Problems messages={[error]} />}
      {snapshot === null && !error && <LoadingSkeleton rows={4} label="Cargando indicadores…" />}
      {snapshot && (
        <>
          <div className="stat-grid" aria-label="Indicadores">
            {STATUS_ORDER.map((status) => <StatTile key={status} label={STATUS_TILE_LABEL[status]} count={snapshot.users[status]} tone={STATUS_TONE[status]} to={`/usuarios?${STATUS_PARAM}=${status}`} />)}
            <StatTile label="Inicios de sesión fallidos (24 h)" count={snapshot.failedSignIns} tone={snapshot.failedSignIns.value > 0 ? 'danger' : 'ok'} to={AUDIT_FAILED_SIGN_IN_PATH} />
          </div>
          <div className="home-grid">
            <section className="panel" aria-labelledby="home-activity-title">
              <h2 id="home-activity-title">Actividad reciente</h2>
              {snapshot.recent.length === 0
                ? <p className="muted">Todavía no hay actividad registrada.</p>
                : (
                  <ul className="activity-list" aria-label="Actividad reciente">
                    {snapshot.recent.map((event) => (
                      <li key={event.id}><code>{event.action}</code><time dateTime={event.createdAt}>{formatDate(event.createdAt)}</time></li>
                    ))}
                  </ul>
                )}
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
