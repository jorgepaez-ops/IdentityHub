import { createContext, useContext, useEffect, useMemo, useReducer, type ReactNode } from 'react'
import { paint, rememberAppearance, storedAppearance, systemAppearance, systemQuery, type Appearance } from './colorScheme'

interface State {
  appearance: Appearance
  /** True once the person picked a side; from then on the operating system is ignored. */
  pinned: boolean
}

type Action = { type: 'system'; appearance: Appearance } | { type: 'pick'; appearance: Appearance }

function reduce(state: State, action: Action): State {
  if (action.type === 'pick') return { appearance: action.appearance, pinned: true }
  return state.pinned ? state : { ...state, appearance: action.appearance }
}

function initialState(): State {
  const stored = storedAppearance()
  return stored ? { appearance: stored, pinned: true } : { appearance: systemAppearance(), pinned: false }
}

interface AppearanceApi {
  appearance: Appearance
  toggle: () => void
}

const AppearanceContext = createContext<AppearanceApi | null>(null)

export function AppearanceProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [state, dispatch] = useReducer(reduce, undefined, initialState)

  useEffect(() => paint(state.appearance), [state.appearance])

  // Only listen to the system while nobody has chosen.
  useEffect(() => {
    const query = systemQuery()
    if (state.pinned || !query?.addEventListener) return
    const onChange = (event: { matches: boolean }) => dispatch({ type: 'system', appearance: event.matches ? 'dark' : 'light' })
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [state.pinned])

  const api = useMemo<AppearanceApi>(() => ({
    appearance: state.appearance,
    toggle: () => {
      const next = state.appearance === 'dark' ? 'light' : 'dark'
      rememberAppearance(next)
      dispatch({ type: 'pick', appearance: next })
    },
  }), [state.appearance])

  return <AppearanceContext.Provider value={api}>{children}</AppearanceContext.Provider>
}

export function useAppearance(): AppearanceApi {
  const api = useContext(AppearanceContext)
  if (!api) throw new Error('useAppearance needs an AppearanceProvider')
  return api
}
