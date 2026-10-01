import { FormEvent, useCallback, useEffect, useState } from 'react'
import { BrowserRouter, Link, NavLink, Navigate, Route, Routes, useNavigate, useSearchParams } from 'react-router-dom'
import { safeContinueTarget } from './features/account/continueTarget'
import { redirectTo } from './navigation'
import { MyAccountPage } from './features/account/MyAccountPage'
import { AuditLogPage } from './features/admin/AuditLogPage'
import { UsersPage } from './features/admin/UsersPage'
import { AcceptInvitationPage, ForgotPasswordPage, ResetPasswordPage } from './features/account/PublicPages'
import { ApiProblemError, clearSession, type CurrentUser, type MfaChallenge, getCurrentUser, login, logout, refreshSession, resendMfaCode, verifyMfa } from './api/client'

type ErrorStep = 'credentials' | 'verify' | 'resend'

function messageFor(error: unknown, step: ErrorStep = 'credentials') {
  if (!(error instanceof ApiProblemError)) return 'No se pudo conectar con el servicio. Comprueba tu conexión.'
  if (step === 'verify' && error.status === 401) return 'El código no es válido o el desafío expiró.'
  if (step === 'resend') {
    if (error.status === 401) return 'El desafío expiró. Vuelve a iniciar sesión.'
    if (error.status === 429) return 'Se solicitaron demasiados códigos. Espera antes de reenviar otro.'
    if (error.status === 503) return 'No se pudo enviar el código. El código anterior sigue siendo válido.'
  }
  switch (error.status) {
    case 401: return 'Correo o contraseña incorrectos.'
    case 423: return 'La cuenta está bloqueada. Inténtalo más tarde.'
    case 429: return 'Demasiados intentos. Espera antes de volver a intentarlo.'
    case 503: return 'No fue posible enviar el código. Inténtalo de nuevo.'
    default: return 'No fue posible completar la solicitud. Inténtalo de nuevo.'
  }
}

function LoginPage({ onAuthenticated }: { onAuthenticated: (user: CurrentUser) => void }) {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  // RF-020: set when an OAuth client sent the user here (ADR 0009).
  const continueTarget = safeContinueTarget(searchParams.get('continue'))
  const [challenge, setChallenge] = useState<MfaChallenge | null>(null)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [canStartOver, setCanStartOver] = useState(false)

  const completeLogin = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    setPending(true)
    setError(null)
    setNotice(null)
    setCanStartOver(false)
    try {
      setChallenge(await login({ email: String(data.get('email')), password: String(data.get('password')) }))
    } catch (reason) {
      setError(messageFor(reason))
    } finally {
      setPending(false)
    }
  }

  const verify = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!challenge) return
    const code = String(new FormData(event.currentTarget).get('code'))
    setPending(true)
    setError(null)
    setNotice(null)
    setCanStartOver(false)
    let mfaVerified = false
    try {
      await verifyMfa({ mfaToken: challenge.mfaToken, code })
      mfaVerified = true
      if (continueTarget) {
        // The MFA response set the hub_session cookie; /oauth/authorize needs a
        // full top-level navigation to the Hub's own origin to read it.
        redirectTo(continueTarget)
        return
      }
      const user = await getCurrentUser()
      onAuthenticated(user)
      navigate(user.roles.includes('admin') ? '/usuarios' : '/me', { replace: true })
    } catch (reason) {
      if (mfaVerified) {
        clearSession()
        setChallenge(null)
        setError('No se pudo cargar tu sesión. Vuelve a iniciar sesión.')
        return
      }
      setError(messageFor(reason, 'verify'))
      setCanStartOver(reason instanceof ApiProblemError && reason.status === 401)
    } finally {
      setPending(false)
    }
  }

  const resend = async () => {
    if (!challenge) return
    setPending(true)
    setError(null)
    setNotice(null)
    setCanStartOver(false)
    try {
      await resendMfaCode({ mfaToken: challenge.mfaToken })
      setNotice('Te enviamos un código nuevo.')
    } catch (reason) {
      setError(messageFor(reason, 'resend'))
      setCanStartOver(reason instanceof ApiProblemError && reason.status === 401)
    } finally {
      setPending(false)
    }
  }

  const startOver = () => {
    setChallenge(null)
    setError(null)
    setNotice(null)
    setCanStartOver(false)
  }

  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="login-title">
        <div className="brand-mark" aria-hidden="true">IH</div>
        <h1 id="login-title">Identity Hub</h1>
        <p className="muted">Proveedor de identidad</p>
        {error && (
          <div className="error-box" role="alert">
            <p>{error}</p>
            {canStartOver && <button className="secondary-button" onClick={startOver} type="button">Volver a iniciar sesión</button>}
          </div>
        )}
        {notice && <div className="success-box" role="status"><p>{notice}</p></div>}
        {challenge ? (
          <form onSubmit={verify}>
            <p className="muted">Enviamos un código de seis dígitos a tu correo electrónico.</p>
            <label htmlFor="mfa-code">Código de verificación</label>
            <input id="mfa-code" name="code" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required />
            <button className="primary-button" disabled={pending} type="submit">{pending ? 'Validando…' : 'Verificar'}</button>
            <button className="secondary-button" disabled={pending} onClick={() => void resend()} type="button">Reenviar código</button>
          </form>
        ) : (
          <form onSubmit={completeLogin}>
            <label htmlFor="email">Correo electrónico</label>
            <input id="email" name="email" type="email" required />
            <label htmlFor="password">Contraseña</label>
            <input id="password" name="password" type="password" required />
            <button className="primary-button" disabled={pending} type="submit">{pending ? 'Validando…' : 'Iniciar sesión'}</button>
            <Link className="text-link" to="/forgot-password">¿Olvidaste tu contraseña?</Link>
          </form>
        )}
      </section>
    </main>
  )
}

function AppShell({ user, onLogout, onUserChange, onSessionEnded }: { user: CurrentUser; onLogout: () => Promise<void>; onUserChange: (user: CurrentUser) => void; onSessionEnded: () => void }) {
  const navigate = useNavigate()
  const isAdmin = user.roles.includes('admin')
  const home = isAdmin ? '/usuarios' : '/me'
  const closeSession = async () => {
    try {
      await onLogout()
    } catch {
      // Local session state must close even if the revoke request is unavailable.
    } finally {
      navigate('/login', { replace: true })
    }
  }
  return (
    <div className="app-shell">
      <aside className="left-rail">
        <div className="rail-brand"><span className="small-mark">IH</span><span>Identity Hub<small>Proveedor de identidad</small></span></div>
        <nav aria-label="Navegación principal">
          {isAdmin && <NavLink to="/usuarios">Usuarios</NavLink>}
          {isAdmin && <NavLink to="/auditoria">Auditoría</NavLink>}
          <NavLink to="/me">Mi cuenta</NavLink>
        </nav>
        <div className="connected-apps"><strong>Aplicaciones conectadas</strong><a href="http://contabilidad.localhost:8080">Contabilidad</a></div>
        <button className="logout-button" onClick={() => void closeSession()} type="button">Cerrar sesión</button>
      </aside>
      <main className="app-content">
        <Routes>
          <Route path="/usuarios" element={isAdmin ? <UsersPage currentUserId={user.id} onSessionEnded={onSessionEnded} /> : <Navigate to="/me" replace />} />
          <Route path="/auditoria" element={isAdmin ? <AuditLogPage onSessionEnded={onSessionEnded} /> : <Navigate to="/me" replace />} />
          <Route path="/me" element={<MyAccountPage user={user} onUserChange={onUserChange} onSessionEnded={onSessionEnded} />} />
          <Route path="*" element={<Navigate to={home} replace />} />
        </Routes>
      </main>
    </div>
  )
}

// A signed-in console user lands on /login?continue= only when /oauth/authorize found no
// hub_session: show the login form so the MFA step can create one instead of looping.
const hasContinueTarget = () => safeContinueTarget(new URLSearchParams(window.location.search).get('continue')) !== null

function AppRoutes() {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [restoring, setRestoring] = useState(true)
  // Stable identity: admin pages list it as an effect dependency.
  const endSession = useCallback(() => { clearSession(); setUser(null) }, [])

  useEffect(() => {
    let active = true
    void (async () => {
      try {
        await refreshSession()
        const current = await getCurrentUser()
        if (active) setUser(current)
      } catch {
        // Missing or expired refresh cookies are normal before a user signs in.
      } finally {
        if (active) setRestoring(false)
      }
    })()
    return () => { active = false }
  }, [])

  if (restoring) return <main className="login-page"><p className="muted">Validando…</p></main>

  return (
    <Routes>
      <Route path="/invitations/accept" element={<AcceptInvitationPage />} />
      <Route path="/forgot-password" element={<ForgotPasswordPage />} />
      <Route path="/password-reset" element={<ResetPasswordPage />} />
      <Route path="/login" element={user && !hasContinueTarget() ? <Navigate to={user.roles.includes('admin') ? '/usuarios' : '/me'} replace /> : <LoginPage onAuthenticated={setUser} />} />
      <Route path="/*" element={user ? <AppShell user={user} onLogout={async () => { try { await logout() } finally { setUser(null) } }} onUserChange={setUser} onSessionEnded={endSession} /> : <Navigate to="/login" replace />} />
    </Routes>
  )
}

export function App() {
  return <BrowserRouter><AppRoutes /></BrowserRouter>
}
