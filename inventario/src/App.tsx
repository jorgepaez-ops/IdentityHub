import { grantsFor } from './permissions'
import { NoAccessScreen, SignInScreen } from './screens/Gate'
import { Workspace } from './screens/Workspace'
import { useHubSession } from './session'

/** Picks the screen that matches where the person is in the sign-in life cycle. */
export function App() {
  const { phase, signIn, signOut } = useHubSession()

  switch (phase.tag) {
    case 'no_access':
      return <NoAccessScreen onSignOut={signOut} />
    case 'active': {
      const { identity } = phase.session
      const grants = grantsFor(identity.roles, identity.permissions)
      if (!grants) return <NoAccessScreen onSignOut={signOut} />
      return <Workspace grants={grants} subject={identity.subject} onSignOut={signOut} />
    }
    default:
      return <SignInScreen checking={phase.tag === 'checking'} problem={phase.tag === 'failed' ? phase.message : null} onContinue={() => { void signIn() }} />
  }
}
