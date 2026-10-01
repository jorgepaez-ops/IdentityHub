import { HUB_ORIGIN } from './config'

export type LoginState = { kind: 'idle' } | { kind: 'validating' } | { kind: 'error'; message: string }

export function LoginScreen({ state, onContinue }: { state: LoginState; onContinue: () => void }) {
  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="login-title">
        <div className="brand-mark" aria-hidden="true">C</div>
        <h1 id="login-title">Contabilidad</h1>
        {state.kind === 'validating' ? (
          <p className="validating" role="status"><span className="pulse" aria-hidden="true" />Validando con Identity Hub…</p>
        ) : (
          <>
            <p className="muted">Inicia sesión con tu cuenta de Identity Hub. Esta aplicación no tiene usuarios propios.</p>
            {state.kind === 'error' && (
              <div className="error-box" role="alert"><p>{state.message}</p></div>
            )}
            <button className="primary-button wide" type="button" onClick={onContinue}>
              {state.kind === 'error' ? 'Reintentar' : 'Continuar con Identity Hub'}
            </button>
          </>
        )}
      </section>
    </main>
  )
}

export function AccessDenied({ onLogout }: { onLogout: () => void }) {
  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="denied-title">
        <div className="brand-mark" aria-hidden="true">C</div>
        <h1 id="denied-title">Sin acceso a Contabilidad</h1>
        <p className="muted">Tu cuenta se autenticó, pero no tiene un rol de Contabilidad. Pide a un administrador que te lo asigne en Identity Hub.</p>
        <a className="primary-button wide" href={HUB_ORIGIN}>Ir a Identity Hub</a>
        <button className="secondary-button wide" type="button" onClick={onLogout}>Cerrar sesión</button>
      </section>
    </main>
  )
}
