import { FormEvent, useCallback, useEffect, useId, useRef, useState } from 'react'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { Toast } from '../../components/Toast'
import { WarningIcon } from '../../components/icons'
import {
  type CreatedServiceKey, type ServiceKey, type ServiceKeyScope,
  createServiceKey, listServiceKeys, revokeServiceKey,
} from '../../api/client'
import { formatDate } from '../format'
import { Problems, adminProblems, isAuthFailure } from './shared'

// The closed list the API accepts (RF-018); more scopes are added here and in the OpenAPI enum.
const SCOPES: { value: ServiceKeyScope; description: string }[] = [
  { value: 'audit-log:read', description: 'Leer el registro de auditoría (solo lectura).' },
]
const DEFAULT_DAYS = '90'
const MAX_DAYS = 365

const STATUS_COPY: Record<number, string> = {
  400: 'Revisa los datos e inténtalo de nuevo.',
  403: 'No tienes permiso para gestionar claves de servicio.',
  404: 'La clave ya no existe.',
}

type Notify = (message: string | null) => void
type Report = { onProblems: (messages: string[]) => void; onSessionEnded: () => void }

function stateOf(key: ServiceKey, now: number): { label: string; tone: string } {
  if (key.revokedAt) return { label: 'Revocada', tone: 'danger' }
  if (new Date(key.expiresAt).getTime() <= now) return { label: 'Expirada', tone: 'warn' }
  return { label: 'Vigente', tone: 'ok' }
}

function KeyRow({ serviceKey, report, onRevoked }: Readonly<{ serviceKey: ServiceKey; report: Report; onRevoked: (key: ServiceKey) => void }>) {
  const [confirming, setConfirming] = useState(false)
  const [pending, setPending] = useState(false)
  const state = stateOf(serviceKey, Date.now())

  const revoke = async () => {
    report.onProblems([])
    setPending(true)
    try {
      await revokeServiceKey(serviceKey.id)
      onRevoked(serviceKey)
    } catch (reason) {
      setConfirming(false)
      setPending(false)
      if (isAuthFailure(reason)) report.onSessionEnded()
      else report.onProblems(adminProblems(reason, STATUS_COPY, 'No fue posible revocar la clave. Inténtalo de nuevo.'))
    }
  }

  return (
    <tr>
      <th scope="row">{serviceKey.name}</th>
      <td><span className="mono">{serviceKey.prefix}</span></td>
      <td>{serviceKey.scopes.join(', ')}</td>
      <td>{formatDate(serviceKey.expiresAt)}</td>
      <td>{serviceKey.lastUsedAt ? formatDate(serviceKey.lastUsedAt) : <span className="muted">Nunca</span>}</td>
      <td><span className={`pill pill-${state.tone}`}>{state.label}</span></td>
      <td>
        {!serviceKey.revokedAt && (
          <div className="row-actions">
            {!confirming && <button className="link-button" onClick={() => setConfirming(true)} type="button" aria-label={`Revocar ${serviceKey.name}`}>Revocar</button>}
            {confirming && (
              <>
                <button className="danger-button fit" disabled={pending} onClick={() => void revoke()} type="button" aria-label={`Confirmar revocación de ${serviceKey.name}`}>Confirmar revocación</button>
                <button className="link-button" disabled={pending} onClick={() => setConfirming(false)} type="button" aria-label={`Cancelar revocación de ${serviceKey.name}`}>Cancelar</button>
              </>
            )}
          </div>
        )}
      </td>
    </tr>
  )
}

// The only place the full key is ever on screen: it is dropped from state as soon as it is dismissed.
function RevealedKey({ created, onDismiss, onNotify }: Readonly<{ created: CreatedServiceKey; onDismiss: () => void; onNotify: Notify }>) {
  const titleId = useId()
  const inputId = useId()
  const input = useRef<HTMLInputElement>(null)

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(created.key)
      onNotify('Clave copiada.')
    } catch {
      // No clipboard permission (or insecure context): select the text so Ctrl+C works.
      input.current?.select()
      onNotify('No fue posible copiar automáticamente: la clave quedó seleccionada, cópiala con Ctrl+C.')
    }
  }

  return (
    <div className="warn-box" role="alert" aria-labelledby={titleId}>
      <h3 id={titleId}><WarningIcon aria-hidden="true" /> Clave emitida</h3>
      <p><strong>Copia la clave ahora: no se volverá a mostrar.</strong> Guárdala en el gestor de secretos de la integración. Si la pierdes, revoca esta clave y emite otra.</p>
      <label htmlFor={inputId}>Clave de servicio completa</label>
      <input id={inputId} ref={input} className="mono key-secret" readOnly value={created.key} onFocus={(event) => event.target.select()} autoComplete="off" spellCheck={false} />
      <div className="drawer-actions">
        <button className="primary-button fit" onClick={() => void copy()} type="button">Copiar clave</button>
        <button className="secondary-button fit" onClick={onDismiss} type="button">Ya la copié</button>
      </div>
    </div>
  )
}

function NewKeyForm({ report, onCreated }: Readonly<{ report: Report; onCreated: (created: CreatedServiceKey) => void }>) {
  const ids = useId()
  const [name, setName] = useState('')
  const [days, setDays] = useState(DEFAULT_DAYS)
  const [scopes, setScopes] = useState<Set<ServiceKeyScope>>(() => new Set(SCOPES.map((scope) => scope.value)))
  const [pending, setPending] = useState(false)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const trimmed = name.trim()
    const expiresInDays = Number(days)
    const problems: string[] = []
    if (!trimmed) problems.push('El nombre es obligatorio (hasta 100 caracteres).')
    if (!Number.isInteger(expiresInDays) || expiresInDays < 1 || expiresInDays > MAX_DAYS) problems.push(`La vigencia debe ser un número entero entre 1 y ${MAX_DAYS} días.`)
    if (scopes.size === 0) problems.push('Elige al menos un alcance.')
    if (problems.length > 0) {
      report.onProblems(problems)
      return
    }
    report.onProblems([])
    setPending(true)
    try {
      const created = await createServiceKey({ name: trimmed, scopes: SCOPES.map((scope) => scope.value).filter((value) => scopes.has(value)), expiresInDays })
      setName('')
      setDays(DEFAULT_DAYS)
      setPending(false)
      onCreated(created)
    } catch (reason) {
      setPending(false)
      if (isAuthFailure(reason)) report.onSessionEnded()
      else report.onProblems(adminProblems(reason, STATUS_COPY, 'No fue posible emitir la clave. Inténtalo de nuevo.'))
    }
  }

  return (
    <form className="new-role-form" onSubmit={(event) => void submit(event)} noValidate>
      <h2>Emitir una clave</h2>
      <label htmlFor={`${ids}-name`}>Nombre de la clave</label>
      <input id={`${ids}-name`} value={name} maxLength={100} autoComplete="off" placeholder="Por ejemplo: SIEM de seguridad" required onChange={(event) => setName(event.target.value)} />
      <fieldset className="role-fieldset">
        <legend>Alcances</legend>
        {SCOPES.map((scope) => (
          <div className="role-option" key={scope.value}>
            <input id={`${ids}-${scope.value}`} type="checkbox" checked={scopes.has(scope.value)} aria-describedby={`${ids}-${scope.value}-help`} onChange={() => setScopes((current) => {
              const next = new Set(current)
              if (!next.delete(scope.value)) next.add(scope.value)
              return next
            })} />
            <label htmlFor={`${ids}-${scope.value}`}>{scope.value}</label>
            <p className="hint" id={`${ids}-${scope.value}-help`}>{scope.description}</p>
          </div>
        ))}
      </fieldset>
      <label htmlFor={`${ids}-days`}>Vigencia en días</label>
      <input id={`${ids}-days`} type="number" inputMode="numeric" min={1} max={MAX_DAYS} value={days} aria-describedby={`${ids}-days-help`} onChange={(event) => setDays(event.target.value)} />
      <p className="hint" id={`${ids}-days-help`}>De 1 a {MAX_DAYS}. Las claves siempre vencen; emite una nueva antes de que expire la actual.</p>
      <div className="drawer-actions">
        <button className="primary-button fit" disabled={pending} type="submit">{pending ? 'Emitiendo…' : 'Emitir clave'}</button>
      </div>
    </form>
  )
}

export function ServiceKeysPage({ onSessionEnded }: Readonly<{ onSessionEnded: () => void }>) {
  const [keys, setKeys] = useState<ServiceKey[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [problems, setProblems] = useState<string[]>([])
  const [revealed, setRevealed] = useState<CreatedServiceKey | null>(null)
  const [notice, setNotice] = useState<{ text: string; id: number } | null>(null)
  const showNotice = useCallback<Notify>((text) => setNotice((current) => (text === null ? null : { text, id: (current?.id ?? 0) + 1 })), [])
  const active = useRef(true)
  const report: Report = {
    onProblems: (messages) => { setProblems(messages); if (messages.length > 0) showNotice(null) },
    onSessionEnded,
  }

  const load = useCallback(async () => {
    try {
      const loaded = await listServiceKeys()
      if (active.current) setKeys(loaded)
    } catch (reason) {
      if (!active.current) return
      if (isAuthFailure(reason)) onSessionEnded()
      else setLoadError('No fue posible cargar las claves de servicio. Inténtalo de nuevo.')
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
          <h1>Claves de servicio</h1>
          <p className="muted">Credenciales para integraciones máquina a máquina, por ejemplo un SIEM que lee el registro de auditoría. Cada clave tiene alcance mínimo, vence y se puede revocar al instante.</p>
        </div>
      </div>
      {loadError && <Problems messages={[loadError]} />}
      <Problems messages={problems} />
      {revealed && <RevealedKey created={revealed} onDismiss={() => setRevealed(null)} onNotify={showNotice} />}
      <section className="panel role-section" aria-label="Claves emitidas">
        {keys === null && !loadError && <LoadingSkeleton label="Cargando claves…" />}
        {keys?.length === 0 && <p className="muted">Todavía no hay claves de servicio.</p>}
        {keys !== null && keys.length > 0 && (
          <div className="table-scroll">
            <table className="data-table" aria-label="Claves de servicio">
              <thead>
                <tr>
                  <th scope="col">Nombre</th>
                  <th scope="col">Prefijo</th>
                  <th scope="col">Alcances</th>
                  <th scope="col">Vence</th>
                  <th scope="col">Último uso</th>
                  <th scope="col">Estado</th>
                  <th scope="col">Acciones</th>
                </tr>
              </thead>
              <tbody>
                {keys.map((key) => (
                  <KeyRow key={`${key.id}:${key.revokedAt ?? ''}`} serviceKey={key} report={report}
                    onRevoked={(revoked) => {
                      setKeys((current) => (current ?? []).map((item) => (item.id === revoked.id ? { ...item, revokedAt: new Date().toISOString() } : item)))
                      showNotice(`Clave ${revoked.name} revocada.`)
                    }} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <section className="panel role-section" aria-label="Emitir una clave">
        <NewKeyForm report={report} onCreated={(created) => {
          const { key: _secret, ...metadata } = created
          void _secret
          setKeys((current) => [metadata, ...(current ?? [])])
          setRevealed(created)
          showNotice(null)
        }} />
      </section>
      {notice && <Toast key={notice.id} message={notice.text} onClose={() => setNotice(null)} />}
    </section>
  )
}
