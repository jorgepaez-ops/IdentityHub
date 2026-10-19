import { goTo } from '../browser'
import { CLIENT_ID, HUB_ORIGIN, callbackUrl } from '../hub'
import { randomToken, s256 } from './pkce'
import { verifyToken, type Identity, type KeySet } from './tokenVerifier'

/** Longest a Hub request may take, body included, before it is abandoned. */
export const DEADLINE_MS = 10_000

const STORED_STATE = 'inventario.login.state'
const STORED_VERIFIER = 'inventario.login.verifier'

export type FailureKind = 'state' | 'refused' | 'no_access' | 'exchange' | 'token' | 'hub_down'

export class LoginFailure extends Error {
  readonly kind: FailureKind
  constructor(kind: FailureKind) {
    super(`login failed: ${kind}`)
    this.kind = kind
  }
}

export interface SignedIn {
  accessToken: string
  identity: Identity
}

/** Sends the browser to the Hub with a fresh PKCE verifier and state kept for the way back. */
export async function startAuthorization(): Promise<void> {
  const verifier = randomToken(48)
  const state = randomToken(24)
  sessionStorage.setItem(STORED_VERIFIER, verifier)
  sessionStorage.setItem(STORED_STATE, state)
  const query = new URLSearchParams({
    response_type: 'code',
    client_id: CLIENT_ID,
    redirect_uri: callbackUrl(),
    code_challenge: await s256(verifier),
    code_challenge_method: 'S256',
    state,
  })
  goTo(`${HUB_ORIGIN}/oauth/authorize?${query.toString()}`)
}

/** Runs one Hub request and reads its JSON body under a single deadline. */
async function hubJson(path: string, init: RequestInit): Promise<unknown> {
  const abort = new AbortController()
  const timer = setTimeout(() => abort.abort(), DEADLINE_MS)
  try {
    const response = await fetch(`${HUB_ORIGIN}${path}`, { ...init, credentials: 'omit', signal: abort.signal })
    if (!response.ok) throw new Error(`HTTP ${response.status}`)
    return await response.json()
  } finally {
    clearTimeout(timer)
  }
}

/** Takes the stored state and verifier out of the session; they can be used once. */
function takeStored(): { state: string | null; verifier: string | null } {
  const stored = { state: sessionStorage.getItem(STORED_STATE), verifier: sessionStorage.getItem(STORED_VERIFIER) }
  sessionStorage.removeItem(STORED_STATE)
  sessionStorage.removeItem(STORED_VERIFIER)
  return stored
}

async function exchangeCode(code: string, verifier: string): Promise<string> {
  try {
    const reply = await hubJson('/oauth/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        code,
        code_verifier: verifier,
        client_id: CLIENT_ID,
        redirect_uri: callbackUrl(),
      }),
    })
    const token = (reply as { access_token?: unknown } | null)?.access_token
    if (typeof token !== 'string') throw new TypeError('no access_token')
    return token
  } catch {
    throw new LoginFailure('exchange')
  }
}

async function fetchKeySet(): Promise<KeySet> {
  try {
    const document = (await hubJson('/.well-known/jwks.json', { method: 'GET' })) as Partial<KeySet> | null
    if (!Array.isArray(document?.keys)) throw new TypeError('no keys')
    return { keys: document.keys }
  } catch {
    throw new LoginFailure('hub_down')
  }
}

/**
 * Handles the redirect back from the Hub: checks the state, trades the code for a token and verifies
 * it. The callback URL is replaced by the app root first, and the token is never persisted.
 */
export async function finishAuthorization(search: string): Promise<SignedIn> {
  const params = new URLSearchParams(search)
  const { state, verifier } = takeStored()
  // Success or failure, the code must not stay in the address bar or history, and a reload must start over.
  window.history.replaceState(null, '', '/')

  if (!state || !verifier || params.get('state') !== state) throw new LoginFailure('state')
  const hubError = params.get('error')
  if (hubError) throw new LoginFailure(hubError === 'access_denied' ? 'no_access' : 'refused')
  const code = params.get('code')
  if (!code) throw new LoginFailure('refused')

  const accessToken = await exchangeCode(code, verifier)
  const keySet = await fetchKeySet()
  try {
    const identity = await verifyToken(accessToken, keySet, { issuer: HUB_ORIGIN, audience: CLIENT_ID, now: Math.floor(Date.now() / 1000) })
    return { accessToken, identity }
  } catch {
    throw new LoginFailure('token')
  }
}
