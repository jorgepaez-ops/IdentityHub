import { useCallback, useEffect, useReducer, useRef } from 'react'
import { CALLBACK_PATH } from './hub'
import { finishAuthorization, LoginFailure, startAuthorization, type FailureKind, type SignedIn } from './oauth/hubLogin'
import { LEEWAY_SECONDS } from './oauth/tokenVerifier'

export type Phase =
  | { tag: 'signed_out' }
  | { tag: 'checking' }
  | { tag: 'failed'; message: string }
  | { tag: 'no_access' }
  | { tag: 'active'; session: SignedIn }

type Action =
  | { type: 'checking' }
  | { type: 'failed'; message: string }
  | { type: 'no_access' }
  | { type: 'active'; session: SignedIn }
  | { type: 'signed_out' }

export function phaseReducer(_phase: Phase, action: Action): Phase {
  switch (action.type) {
    case 'checking': return { tag: 'checking' }
    case 'failed': return { tag: 'failed', message: action.message }
    case 'no_access': return { tag: 'no_access' }
    case 'active': return { tag: 'active', session: action.session }
    case 'signed_out': return { tag: 'signed_out' }
  }
}

const CLOCK_PROBLEM = 'No se pudo iniciar sesión por un problema de reloj. Verifica la hora de tu dispositivo e inténtalo de nuevo.'
const GENERIC_PROBLEM = 'No se pudo completar el inicio de sesión con Identity Hub. Inténtalo de nuevo.'

const FAILURE_TEXT: Record<Exclude<FailureKind, 'no_access'>, string> = {
  state: 'No se pudo validar el inicio de sesión (la respuesta no corresponde a esta solicitud). Inténtalo de nuevo.',
  refused: 'Identity Hub no autorizó el acceso. Inténtalo de nuevo.',
  exchange: GENERIC_PROBLEM,
  token: 'La credencial recibida de Identity Hub no es válida. Inténtalo de nuevo.',
  hub_down: 'Identity Hub no está disponible en este momento. Inténtalo de nuevo.',
}

/** A token that is about to expire would loop through the Hub forever, so it counts as a clock problem. */
export const MIN_REMAINING_MS = LEEWAY_SECONDS * 1000

type Dispatch = (action: Action) => void

async function settleReturn(dispatch: Dispatch): Promise<void> {
  try {
    const session = await finishAuthorization(window.location.search)
    const remaining = session.identity.expiresAt * 1000 - Date.now()
    dispatch(remaining <= MIN_REMAINING_MS ? { type: 'failed', message: CLOCK_PROBLEM } : { type: 'active', session })
  } catch (problem) {
    const kind = problem instanceof LoginFailure ? problem.kind : 'exchange'
    dispatch(kind === 'no_access' ? { type: 'no_access' } : { type: 'failed', message: FAILURE_TEXT[kind] })
  }
}

/**
 * Owns the sign-in life cycle. The access token only ever lives in this hook's state, and there is no
 * refresh token (ADR 0009): when the token expires the Hub is asked again, which answers without a
 * prompt while its own session is alive.
 */
export function useHubSession() {
  const returning = useRef(window.location.pathname === CALLBACK_PATH)
  const settled = useRef(false)
  const [phase, dispatch] = useReducer(phaseReducer, returning.current ? { tag: 'checking' } : { tag: 'signed_out' })

  useEffect(() => {
    // The code is single use, so the simulated remount of StrictMode must not process it twice.
    if (!returning.current || settled.current) return
    settled.current = true
    void settleReturn(dispatch)
  }, [])

  const signIn = useCallback(async () => {
    dispatch({ type: 'checking' })
    try {
      await startAuthorization()
    } catch {
      dispatch({ type: 'failed', message: GENERIC_PROBLEM })
    }
  }, [])

  const expiresAt = phase.tag === 'active' ? phase.session.identity.expiresAt : null
  useEffect(() => {
    if (expiresAt === null) return
    const timer = window.setTimeout(() => { void signIn() }, Math.max(0, expiresAt * 1000 - Date.now()))
    return () => window.clearTimeout(timer)
  }, [expiresAt, signIn])

  const signOut = useCallback(() => dispatch({ type: 'signed_out' }), [])
  return { phase, signIn, signOut }
}
