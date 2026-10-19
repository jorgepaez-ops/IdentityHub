import { Fragment, type ReactNode, createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { LoadingSkeleton } from '../../components/LoadingSkeleton'
import { type Organization, listOrganizations } from '../../api/client'
import { Problems, isAuthFailure } from './shared'

// The company the administrator is working on (RF-022, ADR 0015 point 6). It is a per-viewer convenience:
// kept in localStorage, never in the URL or the API, and the page works without it.
export const COMPANY_STORAGE_KEY = 'identityhub.admin.company'

function readStoredCompanyId(): string | null {
  try {
    return window.localStorage.getItem(COMPANY_STORAGE_KEY)
  } catch {
    return null
  }
}

function storeCompanyId(id: string) {
  try {
    window.localStorage.setItem(COMPANY_STORAGE_KEY, id)
  } catch {
    // Storage can be blocked; the selection then lasts until the page closes.
  }
}

/** The stored company if it still exists, else the first non-operator company, else the first one. */
export function initialCompany(organizations: readonly Organization[], storedId: string | null): Organization | null {
  return organizations.find((item) => item.id === storedId)
    ?? organizations.find((item) => !item.operator)
    ?? organizations[0]
    ?? null
}

// The seeded operator already carries "(operador)" in its name; the label adds it only when missing.
export const companyLabel = (organization: Organization) => (organization.operator && !/operador/i.test(organization.name) ? `${organization.name} (operador)` : organization.name)

type CompanyState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; organizations: Organization[]; selectedId: string | null }

type CompanyContextValue = {
  state: CompanyState
  select: (id: string) => void
  retry: () => void
}

const CompanyContext = createContext<CompanyContextValue | null>(null)

export function CompanyProvider({ onSessionEnded, children }: Readonly<{ onSessionEnded: () => void; children: ReactNode }>) {
  const [state, setState] = useState<CompanyState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const organizations = await listOrganizations()
        if (active) setState({ status: 'ready', organizations, selectedId: initialCompany(organizations, readStoredCompanyId())?.id ?? null })
      } catch (reason) {
        if (!active) return
        if (isAuthFailure(reason)) onSessionEnded()
        else setState({ status: 'error' })
      }
    })()
    return () => { active = false }
  }, [attempt, onSessionEnded])

  const select = useCallback((id: string) => {
    storeCompanyId(id)
    setState((current) => (current.status === 'ready' ? { ...current, selectedId: id } : current))
  }, [])
  const retry = useCallback(() => {
    setState({ status: 'loading' })
    setAttempt((current) => current + 1)
  }, [])
  const value = useMemo(() => ({ state, select, retry }), [state, select, retry])
  return <CompanyContext.Provider value={value}>{children}</CompanyContext.Provider>
}

function useCompanyContext(): CompanyContextValue {
  const context = useContext(CompanyContext)
  if (!context) throw new Error('CompanyScope needs a CompanyProvider')
  return context
}

export function CompanySelector({ organizations, selectedId, onSelect }: Readonly<{ organizations: readonly Organization[]; selectedId: string; onSelect: (id: string) => void }>) {
  return (
    <div className="company-bar">
      <label htmlFor="company-select">Empresa</label>
      <select id="company-select" value={selectedId} onChange={(event) => onSelect(event.target.value)}>
        {organizations.map((organization) => <option key={organization.id} value={organization.id}>{companyLabel(organization)}</option>)}
      </select>
    </div>
  )
}

/**
 * Wraps an administration page with the company selector and renders it for the selected company only.
 * The page is remounted when the company changes, so nothing from the previous company stays on screen.
 */
export function CompanyScope({ children }: Readonly<{ children: (organization: Organization, organizations: readonly Organization[]) => ReactNode }>) {
  const { state, select, retry } = useCompanyContext()
  if (state.status === 'loading') return <LoadingSkeleton label="Cargando empresas…" />
  if (state.status === 'error') {
    return (
      <>
        <Problems messages={['No fue posible cargar las empresas. Inténtalo de nuevo.']} />
        <button className="secondary-button fit" type="button" onClick={retry}>Reintentar</button>
      </>
    )
  }
  const organization = state.organizations.find((item) => item.id === state.selectedId)
  if (!organization) return <p className="muted">No hay empresas registradas.</p>
  return (
    <>
      <CompanySelector organizations={state.organizations} selectedId={organization.id} onSelect={select} />
      <Fragment key={organization.id}>{children(organization, state.organizations)}</Fragment>
    </>
  )
}
