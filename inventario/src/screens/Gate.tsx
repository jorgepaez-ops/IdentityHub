import type { ReactNode } from 'react'
import { HUB_ORIGIN } from '../hub'
import { usePageTitle } from '../pageTitle'

/** Centered card shared by the screens shown before the workspace. */
function GateCard({ title, children }: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <main className="gate">
      <section className="gate__panel" aria-labelledby="gate-title">
        <span className="gate__logo" aria-hidden="true">I</span>
        <h1 id="gate-title">{title}</h1>
        {children}
      </section>
    </main>
  )
}

interface SignInProps {
  checking: boolean
  problem: string | null
  onContinue: () => void
}

export function SignInScreen({ checking, problem, onContinue }: Readonly<SignInProps>) {
  usePageTitle('Iniciar sesión · Inventario')
  return (
    <GateCard title="Inventario">
      {checking ? (
        <output className="gate__wait">Validando con Identity Hub…</output>
      ) : (
        <>
          <p className="gate__text">Inicia sesión con tu cuenta de Identity Hub. Esta aplicación no tiene usuarios propios.</p>
          {problem !== null && <p className="gate__problem" role="alert">{problem}</p>}
          <button className="btn btn--solid btn--block" type="button" onClick={onContinue}>
            {problem === null ? 'Continuar con Identity Hub' : 'Reintentar'}
          </button>
        </>
      )}
    </GateCard>
  )
}

export function NoAccessScreen({ onSignOut }: Readonly<{ onSignOut: () => void }>) {
  usePageTitle('Sin acceso · Inventario')
  return (
    <GateCard title="Sin acceso a Inventario">
      <p className="gate__text">
        Tu cuenta se autenticó, pero no tiene acceso a Inventario: pertenece a otra empresa o no tiene un rol de esta aplicación.
        Pide a un administrador que te lo asigne en Identity Hub.
      </p>
      <a className="btn btn--solid btn--block" href={HUB_ORIGIN}>Ir a Identity Hub</a>
      <button className="btn btn--outline btn--block" type="button" onClick={onSignOut}>Cerrar sesión</button>
    </GateCard>
  )
}
