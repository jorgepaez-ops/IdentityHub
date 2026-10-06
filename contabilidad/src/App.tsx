import { useCallback, useEffect, useRef, useState } from 'react'
import { accessFor } from './access'
import { beginLogin, completeCallback, type CallbackError, type Session } from './auth/flow'
import { CLOCK_LEEWAY_SECONDS } from './auth/jwt'
import { CALLBACK_PATH } from './config'
import { AccessDenied, LoginScreen, type LoginState } from './Login'
import { Shell } from './Shell'

export const MINIMUM_SESSION_LIFETIME_MS = CLOCK_LEEWAY_SECONDS * 1000

type CallbackMessageKind = CallbackError['kind'] | 'clock'

const CALLBACK_MESSAGES: Record<CallbackMessageKind, string> = {
  state: 'No se pudo validar el inicio de sesión (la respuesta no corresponde a esta solicitud). Inténtalo de nuevo.',
  denied: 'Identity Hub no autorizó el acceso. Inténtalo de nuevo.',
  exchange: 'No se pudo completar el inicio de sesión con Identity Hub. Inténtalo de nuevo.',
  token: 'La credencial recibida de Identity Hub no es válida. Inténtalo de nuevo.',
  hub_unavailable: 'Identity Hub no está disponible en este momento. Inténtalo de nuevo.',
  clock: 'No se pudo iniciar sesión por un problema de reloj. Verifica la hora de tu dispositivo e inténtalo de nuevo.',
}

export function App() {
  const returning = window.location.pathname === CALLBACK_PATH
  const [login, setLogin] = useState<LoginState>(returning ? { kind: 'validating' } : { kind: 'idle' })
  // The access token lives only in this component's memory: never in storage.
  const [session, setSession] = useState<Session | null>(null)
  // Survives the simulated remount of React.StrictMode: the code is single use.
  const callbackHandled = useRef(false)

  useEffect(() => {
    if (!returning || callbackHandled.current) return
    callbackHandled.current = true
    completeCallback(window.location.search).then(
      (result) => {
        if (result.verified.expiresAt * 1000 - Date.now() <= MINIMUM_SESSION_LIFETIME_MS) {
          setLogin({ kind: 'error', message: CALLBACK_MESSAGES.clock })
          return
        }
        setSession(result)
      },
      (reason: unknown) => setLogin({ kind: 'error', message: CALLBACK_MESSAGES[(reason as CallbackError)?.kind] ?? CALLBACK_MESSAGES.exchange }),
    )
  }, [returning])

  const expiresAt = session?.verified.expiresAt
  useEffect(() => {
    if (expiresAt === undefined) return
    // There is no refresh token by design (ADR 0009): when the access token
    // expires the app asks the Hub again, which answers silently while the Hub
    // session lives.
    const timer = window.setTimeout(() => { void beginLogin() }, Math.max(0, expiresAt * 1000 - Date.now()))
    return () => window.clearTimeout(timer)
  }, [expiresAt])

  const start = useCallback(() => {
    setLogin({ kind: 'validating' })
    void beginLogin()
  }, [])
  const logout = useCallback(() => {
    setSession(null)
    setLogin({ kind: 'idle' })
  }, [])

  if (!session) return <LoginScreen state={login} onContinue={start} />
  const access = accessFor(session.verified.permissions, session.verified.roles)
  if (access.level === 'none') return <AccessDenied onLogout={logout} />
  return <Shell access={access} subject={session.verified.subject} onLogout={logout} />
}
