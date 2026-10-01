import { FormEvent, ReactNode, useId, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ApiProblemError, acceptInvitation, confirmPasswordReset, requestPasswordReset } from '../../api/client'

// Bounds come from InvitationAcceptRequest / PasswordResetConfirmRequest in specs/03-api/openapi.yaml.
const PASSWORD_MIN = 12
const PASSWORD_MAX = 128

export const connectionMessage = 'No se pudo conectar con el servicio. Comprueba tu conexión.'

export function problemMessages(error: unknown, fallback: string): string[] {
  if (!(error instanceof ApiProblemError)) return [connectionMessage]
  const fields = error.errors.map((item) => item.message).filter(Boolean)
  return fields.length > 0 ? fields : [fallback]
}

function AuthCard({ title, lead, children }: { title: string; lead?: string; children: ReactNode }) {
  const titleId = useId()
  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby={titleId}>
        <div className="brand-mark" aria-hidden="true">IH</div>
        <h1 id={titleId}>{title}</h1>
        {lead && <p className="muted">{lead}</p>}
        {children}
      </section>
    </main>
  )
}

function Problems({ messages }: { messages: string[] }) {
  if (messages.length === 0) return null
  return <div className="error-box" role="alert">{messages.map((message, index) => <p key={index}>{message}</p>)}</div>
}

function LoginLink() {
  return <Link className="secondary-button button-link" to="/login">Ir a iniciar sesión</Link>
}

function NewPasswordForm({ submitLabel, pending, problems, onSubmit }: {
  submitLabel: string
  pending: boolean
  problems: string[]
  onSubmit: (password: string) => void
}) {
  const [localProblem, setLocalProblem] = useState<string | null>(null)
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    const password = String(data.get('password'))
    if (password.length < PASSWORD_MIN || password.length > PASSWORD_MAX) {
      setLocalProblem(`La contraseña debe tener entre ${PASSWORD_MIN} y ${PASSWORD_MAX} caracteres.`)
      return
    }
    if (password !== String(data.get('confirmation'))) {
      setLocalProblem('Las contraseñas no coinciden.')
      return
    }
    setLocalProblem(null)
    onSubmit(password)
  }
  return (
    <form onSubmit={submit} noValidate>
      <Problems messages={localProblem ? [localProblem] : problems} />
      <label htmlFor="new-password">Contraseña nueva</label>
      <input id="new-password" name="password" type="password" autoComplete="new-password" required />
      <p className="hint">Entre {PASSWORD_MIN} y {PASSWORD_MAX} caracteres.</p>
      <label htmlFor="password-confirmation">Confirma la contraseña</label>
      <input id="password-confirmation" name="confirmation" type="password" autoComplete="new-password" required />
      <button className="primary-button" disabled={pending} type="submit">{pending ? 'Guardando…' : submitLabel}</button>
    </form>
  )
}

type TokenFlow = 'form' | 'done' | 'expired'

function useTokenFlow(call: (token: string, password: string) => Promise<void>, badRequestFallback: string) {
  const [params] = useSearchParams()
  const token = params.get('token')
  const [flow, setFlow] = useState<TokenFlow>('form')
  const [pending, setPending] = useState(false)
  const [problems, setProblems] = useState<string[]>([])
  const submit = async (password: string) => {
    if (!token) return
    setPending(true)
    setProblems([])
    try {
      await call(token, password)
      setFlow('done')
    } catch (reason) {
      if (reason instanceof ApiProblemError && reason.status === 410) setFlow('expired')
      else if (reason instanceof ApiProblemError && reason.status === 400) setProblems(problemMessages(reason, badRequestFallback))
      else setProblems(problemMessages(reason, 'No fue posible completar la solicitud. Inténtalo de nuevo.'))
    } finally {
      setPending(false)
    }
  }
  return { token, flow, pending, problems, submit }
}

export function AcceptInvitationPage() {
  const { token, flow, pending, problems, submit } = useTokenFlow(
    (value, password) => acceptInvitation({ token: value, password }),
    'La contraseña no cumple los requisitos.',
  )
  if (!token) {
    return (
      <AuthCard title="Define tu contraseña">
        <div className="error-box" role="alert"><p>El enlace no es válido o está incompleto. Abre de nuevo el enlace de tu correo de invitación.</p></div>
      </AuthCard>
    )
  }
  return (
    <AuthCard title="Define tu contraseña" lead="Elige una contraseña para activar tu cuenta.">
      {flow === 'form' && <NewPasswordForm submitLabel="Definir contraseña" pending={pending} problems={problems} onSubmit={(password) => void submit(password)} />}
      {flow === 'done' && (
        <>
          <div className="success-box" role="status"><p>Tu cuenta quedó activada. Ya puedes iniciar sesión.</p></div>
          <LoginLink />
        </>
      )}
      {flow === 'expired' && (
        <div className="error-box" role="alert"><p>La invitación venció o ya se usó. Pide a un administrador que te reenvíe la invitación.</p></div>
      )}
    </AuthCard>
  )
}

export function ForgotPasswordPage() {
  const [pending, setPending] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const email = String(new FormData(event.currentTarget).get('email'))
    setPending(true)
    setError(null)
    try {
      await requestPasswordReset({ email })
      setSent(true)
    } catch (reason) {
      // Any answer from the API (including 400, 429 and 503) shows the same notice so the page never
      // reveals whether the account exists or whether delivery worked. Only a network failure differs.
      if (reason instanceof ApiProblemError) setSent(true)
      else setError(connectionMessage)
    } finally {
      setPending(false)
    }
  }
  return (
    <AuthCard title="Restablecer contraseña" lead="Escribe tu correo y te enviaremos un enlace para elegir una contraseña nueva.">
      {error && <div className="error-box" role="alert"><p>{error}</p></div>}
      {sent && <div className="success-box" role="status"><p>Si la cuenta existe, enviamos un enlace para restablecer la contraseña. Revisa tu correo.</p></div>}
      <form onSubmit={(event) => void submit(event)}>
        <label htmlFor="reset-email">Correo electrónico</label>
        <input id="reset-email" name="email" type="email" autoComplete="email" required />
        <button className="primary-button" disabled={pending} type="submit">{pending ? 'Enviando…' : 'Enviar enlace'}</button>
      </form>
      <Link className="secondary-button button-link" to="/login">Volver a iniciar sesión</Link>
    </AuthCard>
  )
}

export function ResetPasswordPage() {
  const { token, flow, pending, problems, submit } = useTokenFlow(
    (value, password) => confirmPasswordReset({ token: value, password }),
    'La contraseña no cumple los requisitos.',
  )
  if (!token) {
    return (
      <AuthCard title="Elige una contraseña nueva">
        <div className="error-box" role="alert"><p>El enlace no es válido o está incompleto. Abre de nuevo el enlace de tu correo.</p></div>
        <Link className="secondary-button button-link" to="/forgot-password">Solicitar un enlace nuevo</Link>
      </AuthCard>
    )
  }
  return (
    <AuthCard title="Elige una contraseña nueva">
      {flow === 'form' && <NewPasswordForm submitLabel="Restablecer contraseña" pending={pending} problems={problems} onSubmit={(password) => void submit(password)} />}
      {flow === 'done' && (
        <>
          <div className="success-box" role="status"><p>Tu contraseña se actualizó y cerramos tus sesiones activas. Inicia sesión con la nueva.</p></div>
          <LoginLink />
        </>
      )}
      {flow === 'expired' && (
        <>
          <div className="error-box" role="alert"><p>El enlace venció o ya se usó.</p></div>
          <Link className="secondary-button button-link" to="/forgot-password">Solicitar un enlace nuevo</Link>
        </>
      )}
    </AuthCard>
  )
}
