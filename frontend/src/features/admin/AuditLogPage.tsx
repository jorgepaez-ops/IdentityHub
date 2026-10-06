import { FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { Toast } from '../../components/Toast'
import { DownloadIcon } from '../../components/icons'
import { type AuditEvent, type AuditLogQuery, getUser, listAuditLog } from '../../api/client'
import { csvFileName, downloadCsv, toCsv } from '../csv'
import { formatDate } from '../format'
import { AUDIT_PARAM, FAILED_SIGN_IN_ACTION, LAST_24H, MFA_CODE_REJECTED_ACTION, Problems, isAuthFailure } from './shared'

const PAGE_SIZE = 50
const DAY_MS = 24 * 60 * 60 * 1000
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const LOCAL_DATETIME = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/
const short = (id: string) => id.slice(0, 8)

type Filters = { action: string; actorId: string; since: string }
const NO_FILTERS: Filters = { action: '', actorId: '', since: '' }

// The API takes one action per query, so each shortcut sets exactly one action.
const ACTION_SHORTCUTS = [
  { label: 'Inicios fallidos', action: FAILED_SIGN_IN_ACTION },
  { label: 'Códigos MFA rechazados', action: MFA_CODE_REJECTED_ACTION },
  { label: 'Cambios de roles', action: 'role_changed' },
] as const

// Invalid values in a hand-edited URL are ignored instead of reaching the API.
function readFilters(params: URLSearchParams): Filters {
  const actorId = params.get(AUDIT_PARAM.actor) ?? ''
  const since = params.get(AUDIT_PARAM.since) ?? ''
  const validSince = since === LAST_24H || (LOCAL_DATETIME.test(since) && !Number.isNaN(new Date(since).getTime()))
  return {
    action: (params.get(AUDIT_PARAM.action) ?? '').slice(0, 100),
    actorId: UUID.test(actorId) ? actorId : '',
    since: validSince ? since : '',
  }
}

function writeFilters(filters: Filters): URLSearchParams {
  const params = new URLSearchParams()
  if (filters.action) params.set(AUDIT_PARAM.action, filters.action)
  if (filters.actorId) params.set(AUDIT_PARAM.actor, filters.actorId)
  if (filters.since) params.set(AUDIT_PARAM.since, filters.since)
  return params
}

function toQuery(filters: Filters): AuditLogQuery {
  const since = filters.since === LAST_24H ? new Date(Date.now() - DAY_MS) : filters.since ? new Date(filters.since) : null
  return {
    ...(filters.action ? { action: filters.action } : {}),
    ...(filters.actorId ? { actorId: filters.actorId } : {}),
    ...(since ? { since: since.toISOString() } : {}),
  }
}

const MAX_DEPTH = 4
function stringify(value: unknown): string {
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value) ?? String(value)
  } catch {
    return String(value)
  }
}

// Flattens nested objects into dotted keys; arrays of scalars are joined, anything deeper is stringified safely.
function metadataEntries(value: unknown, prefix = '', depth = 0): [string, string][] {
  if (value !== null && typeof value === 'object' && !Array.isArray(value) && depth < MAX_DEPTH) {
    const entries = Object.entries(value as Record<string, unknown>)
    return entries.flatMap(([key, inner]) => metadataEntries(inner, prefix ? `${prefix}.${key}` : key, depth + 1))
  }
  if (Array.isArray(value) && value.every((item) => item === null || typeof item !== 'object')) return [[prefix, value.map(String).join(', ')]]
  return [[prefix, stringify(value)]]
}

function MetadataList({ metadata }: { metadata: AuditEvent['metadata'] }) {
  const entries = Object.keys(metadata ?? {}).length === 0 ? [] : metadataEntries(metadata)
  if (entries.length === 0) return <span className="muted">—</span>
  return (
    <details>
      <summary>Ver</summary>
      <dl className="metadata-list">
        {/* Flattened keys can collide (a literal "a.b" and a nested a.b), so the position keeps React keys unique. */}
        {entries.map(([key, text], index) => <div className="metadata-pair" key={`${index}:${key}`}><dt>{key}</dt><dd>{text}</dd></div>)}
      </dl>
    </details>
  )
}

// Resolves each distinct actor id on screen to an email once per page load. It never blocks the table:
// until a lookup settles (or when it fails, e.g. a deleted account) the short id is shown.
function useActorEmails(events: AuditEvent[] | null): Record<string, string> {
  const [emails, setEmails] = useState<Record<string, string>>({})
  const requested = useRef(new Set<string>())
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  useEffect(() => {
    const ids = new Set((events ?? []).flatMap((item) => (item.actorUserId ? [item.actorUserId] : [])))
    for (const id of ids) {
      if (requested.current.has(id)) continue
      requested.current.add(id)
      void getUser(id).then(
        (user) => {
          if (mounted.current) setEmails((current) => ({ ...current, [id]: user.email }))
        },
        () => undefined,
      )
    }
  }, [events])
  return emails
}

export function AuditLogPage({ onSessionEnded }: { onSessionEnded: () => void }) {
  const [params, setParams] = useSearchParams()
  const paramsKey = params.toString()
  const applied = useMemo(() => readFilters(new URLSearchParams(paramsKey)), [paramsKey])
  const [draft, setDraft] = useState<Filters>(applied)
  const [events, setEvents] = useState<AuditEvent[] | null>(null)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [filterError, setFilterError] = useState<string | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)
  // Bumped by "Filtrar" so re-submitting unchanged filters still reloads (and re-resolves a relative window).
  const [reloads, setReloads] = useState(0)
  const generation = useRef(0)
  // The query sent for the first page is reused for "Cargar más", so a relative window never shifts mid-pagination.
  const query = useRef<AuditLogQuery>({})
  const actorEmails = useActorEmails(events)
  const [toast, setToast] = useState<{ text: string; id: number } | null>(null)

  useEffect(() => () => {
    generation.current += 1
  }, [])

  // The URL is the source of truth: the form mirrors it whenever it changes (shortcuts, back/forward, clear).
  useEffect(() => {
    setDraft({ ...applied, since: applied.since === LAST_24H ? '' : applied.since })
  }, [applied])

  useEffect(() => {
    const mine = ++generation.current
    query.current = toQuery(applied)
    setError(null)
    setNextCursor(null)
    setLoadingMore(false)
    void (async () => {
      try {
        const page = await listAuditLog({ limit: PAGE_SIZE, ...query.current })
        if (mine !== generation.current) return
        setEvents(page.items)
        setNextCursor(page.nextCursor ?? null)
      } catch (reason) {
        if (mine !== generation.current) return
        if (isAuthFailure(reason)) onSessionEnded()
        else setError('No fue posible cargar el registro de auditoría. Inténtalo de nuevo.')
      }
    })()
  }, [applied, reloads, onSessionEnded])

  // Changing any filter rewrites the URL (replace, like the user status filter); the effect above then reloads from the first page.
  const apply = (next: Filters) => {
    setFilterError(null)
    setParams(writeFilters(next), { replace: true })
  }

  const filter = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const next = { action: draft.action.trim(), actorId: draft.actorId.trim(), since: draft.since || (applied.since === LAST_24H ? LAST_24H : '') }
    if (next.actorId && !UUID.test(next.actorId)) {
      setFilterError('El ID del actor debe ser un UUID.')
      return
    }
    // An unchanged URL would not re-run the load effect, so force it; a changed URL reloads on its own.
    if (writeFilters(next).toString() !== paramsKey) return apply(next)
    setFilterError(null)
    setReloads((count) => count + 1)
  }

  const clear = () => apply(NO_FILTERS)

  const toggleAction = (action: string) => apply({ ...applied, action: applied.action === action ? '' : action })
  const toggleLast24h = () => apply({ ...applied, since: applied.since === LAST_24H ? '' : LAST_24H })

  const loadMore = async () => {
    if (!nextCursor) return
    const mine = generation.current
    setLoadingMore(true)
    try {
      const page = await listAuditLog({ limit: PAGE_SIZE, cursor: nextCursor, ...query.current })
      if (mine !== generation.current) return
      setEvents((current) => [...(current ?? []), ...page.items])
      setNextCursor(page.nextCursor ?? null)
    } catch (reason) {
      if (mine !== generation.current) return
      if (isAuthFailure(reason)) onSessionEnded()
      else setError('No fue posible cargar el registro de auditoría. Inténtalo de nuevo.')
    } finally {
      if (mine === generation.current) setLoadingMore(false)
    }
  }

  // Exports what is loaded under the current filters: fetching every page would contradict D3.
  const exportEvents = () => {
    const header = ['Fecha', 'Acción', 'Actor', 'Tipo de recurso', 'ID de recurso', 'IP', 'Metadatos']
    const rows = (events ?? []).map((item) => [
      item.createdAt,
      item.action,
      item.actorUserId ? actorEmails[item.actorUserId] ?? item.actorUserId : 'Sistema',
      item.resourceType,
      item.resourceId,
      item.ip,
      Object.keys(item.metadata ?? {}).length === 0 ? '' : metadataEntries(item.metadata).map(([key, text]) => `${key}=${text}`).join('; '),
    ])
    downloadCsv(csvFileName('auditoria'), toCsv(header, rows))
    setToast((current) => ({ text: `Exportados ${rows.length} registros`, id: (current?.id ?? 0) + 1 }))
  }

  const set = (key: keyof Filters) => (event: React.ChangeEvent<HTMLInputElement>) => setDraft((current) => ({ ...current, [key]: event.target.value }))

  return (
    <section>
      <h1>Auditoría</h1>
      <p className="muted">Eventos de seguridad del Hub, del más reciente al más antiguo. El registro es de solo lectura.</p>
      <section className="panel directory" aria-labelledby="audit-title">
        <h2 id="audit-title">Registro</h2>
        <form className="filter-form" onSubmit={filter} noValidate>
          <div><label htmlFor="audit-action">Acción</label><input id="audit-action" value={draft.action} maxLength={100} onChange={set('action')} placeholder="login_failed" /></div>
          <div><label htmlFor="audit-actor">Actor (ID)</label><input id="audit-actor" value={draft.actorId} onChange={set('actorId')} placeholder="UUID del usuario" /></div>
          <div><label htmlFor="audit-since">Desde</label><input id="audit-since" type="datetime-local" value={draft.since} onChange={set('since')} /></div>
          <div className="filter-actions">
            <button className="primary-button fit" type="submit">Filtrar</button>
            <button className="secondary-button fit" onClick={clear} type="button">Limpiar</button>
          </div>
        </form>
        <div className="filter-group" role="group" aria-label="Atajos de filtro">
          {ACTION_SHORTCUTS.map((item) => (
            <button key={item.action} className="filter-button" type="button" aria-pressed={applied.action === item.action} onClick={() => toggleAction(item.action)}>{item.label}</button>
          ))}
          <button className="filter-button" type="button" aria-pressed={applied.since === LAST_24H} onClick={toggleLast24h}>Últimas 24 h</button>
        </div>
        <div className="export-row">
          <button className="secondary-button fit" type="button" disabled={!events?.length} onClick={exportEvents}><DownloadIcon />Exportar CSV</button>
          {nextCursor && events && events.length > 0 && <p className="hint">Exporta los {events.length} cargados; carga más para incluir el resto.</p>}
        </div>
        {filterError && <Problems messages={[filterError]} />}
        {error && <Problems messages={[error]} />}
        {events === null && !error && <LoadingSkeleton label="Cargando auditoría…" />}
        {events !== null && events.length === 0 && <p className="muted">No hay eventos para los filtros elegidos.</p>}
        {events !== null && events.length > 0 && (
          <div className="table-scroll">
            <table className="data-table" aria-label="Registro de auditoría">
              <thead>
                <tr><th scope="col">Fecha</th><th scope="col">Actor</th><th scope="col">Acción</th><th scope="col">Recurso</th><th scope="col">IP</th><th scope="col">Metadatos</th></tr>
              </thead>
              <tbody>
                {events.map((event) => {
                  const actorId = event.actorUserId
                  const email = actorId ? actorEmails[actorId] : undefined
                  return (
                    <tr key={event.id}>
                      <td>{formatDate(event.createdAt)}</td>
                      <td>
                        {actorId ? (
                          <div className="actor-cell">
                            {email ? <span title={actorId}>{email}</span> : <span className="mono" title={actorId}>{short(actorId)}</span>}
                            <button className="link-button" type="button" aria-label={`Filtrar por este actor: ${email ?? short(actorId)}`} onClick={() => apply({ ...applied, actorId })}>Filtrar por este actor</button>
                          </div>
                        ) : <span className="muted">Sistema</span>}
                      </td>
                      <td><code>{event.action}</code></td>
                      <td>
                        {event.resourceType && event.resourceId
                          ? <span className="mono" title={event.resourceId}>{`${event.resourceType} · ${short(event.resourceId)}`}</span>
                          : event.resourceType ?? <span className="muted">—</span>}
                      </td>
                      <td>{event.ip ?? <span className="muted">—</span>}</td>
                      <td><MetadataList metadata={event.metadata} /></td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
        {nextCursor && <button className="secondary-button fit" disabled={loadingMore} onClick={() => void loadMore()} type="button">{loadingMore ? 'Cargando…' : 'Cargar más'}</button>}
      </section>
      {toast && <Toast key={toast.id} message={toast.text} onClose={() => setToast(null)} />}
    </section>
  )
}
