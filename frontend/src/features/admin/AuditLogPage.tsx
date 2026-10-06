import { FormEvent, useEffect, useRef, useState } from 'react'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { type AuditEvent, type AuditLogQuery, listAuditLog } from '../../api/client'
import { formatDate } from '../format'
import { Problems, isAuthFailure } from './shared'

const PAGE_SIZE = 50
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const short = (id: string) => id.slice(0, 8)

type Filters = { action: string; actorId: string; since: string }
const NO_FILTERS: Filters = { action: '', actorId: '', since: '' }

function toQuery(filters: Filters): AuditLogQuery {
  return {
    ...(filters.action ? { action: filters.action } : {}),
    ...(filters.actorId ? { actorId: filters.actorId } : {}),
    ...(filters.since ? { since: new Date(filters.since).toISOString() } : {}),
  }
}

function metadataText(event: AuditEvent): string | null {
  const metadata = event.metadata ?? {}
  return Object.keys(metadata).length === 0 ? null : JSON.stringify(metadata)
}

export function AuditLogPage({ onSessionEnded }: { onSessionEnded: () => void }) {
  const [draft, setDraft] = useState<Filters>(NO_FILTERS)
  const [applied, setApplied] = useState<Filters>(NO_FILTERS)
  const [events, setEvents] = useState<AuditEvent[] | null>(null)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [filterError, setFilterError] = useState<string | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)
  const generation = useRef(0)

  useEffect(() => () => {
    generation.current += 1
  }, [])

  useEffect(() => {
    const mine = ++generation.current
    setError(null)
    setNextCursor(null)
    setLoadingMore(false)
    void (async () => {
      try {
        const page = await listAuditLog({ limit: PAGE_SIZE, ...toQuery(applied) })
        if (mine !== generation.current) return
        setEvents(page.items)
        setNextCursor(page.nextCursor ?? null)
      } catch (reason) {
        if (mine !== generation.current) return
        if (isAuthFailure(reason)) onSessionEnded()
        else setError('No fue posible cargar el registro de auditoría. Inténtalo de nuevo.')
      }
    })()
  }, [applied, onSessionEnded])

  const filter = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const next = { action: draft.action.trim(), actorId: draft.actorId.trim(), since: draft.since }
    if (next.actorId && !UUID.test(next.actorId)) {
      setFilterError('El ID del actor debe ser un UUID.')
      return
    }
    setFilterError(null)
    setApplied(next)
  }

  const clear = () => {
    setDraft(NO_FILTERS)
    setFilterError(null)
    setApplied(NO_FILTERS)
  }

  const loadMore = async () => {
    if (!nextCursor) return
    const mine = generation.current
    setLoadingMore(true)
    try {
      const page = await listAuditLog({ limit: PAGE_SIZE, cursor: nextCursor, ...toQuery(applied) })
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
                  const metadata = metadataText(event)
                  return (
                    <tr key={event.id}>
                      <td>{formatDate(event.createdAt)}</td>
                      <td>{event.actorUserId ? <span className="mono" title={event.actorUserId}>{short(event.actorUserId)}</span> : <span className="muted">Sistema</span>}</td>
                      <td><code>{event.action}</code></td>
                      <td>
                        {event.resourceType && event.resourceId
                          ? <span className="mono" title={event.resourceId}>{`${event.resourceType} · ${short(event.resourceId)}`}</span>
                          : event.resourceType ?? <span className="muted">—</span>}
                      </td>
                      <td>{event.ip ?? <span className="muted">—</span>}</td>
                      <td>{metadata ? <details><summary>Ver</summary><pre className="metadata">{metadata}</pre></details> : <span className="muted">—</span>}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
        {nextCursor && <button className="secondary-button fit" disabled={loadingMore} onClick={() => void loadMore()} type="button">{loadingMore ? 'Cargando…' : 'Cargar más'}</button>}
      </section>
    </section>
  )
}
