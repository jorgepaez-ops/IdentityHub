import { AUDIENCE, CLIENT_ID, HUB_ORIGIN, ISSUER, redirectUri } from '../config'
import { redirectTo } from '../navigation'
import { verifyAccessToken, type Jwks, type VerifiedToken } from './jwt'
import { challengeFor, generateState, generateVerifier } from './pkce'

// Only the PKCE verifier and state live in sessionStorage, for the length of
// the round trip through the Hub. They are removed as soon as the callback reads them.
const VERIFIER_KEY = 'contabilidad.pkce_verifier'
const STATE_KEY = 'contabilidad.oauth_state'

export type CallbackErrorKind = 'state' | 'denied' | 'exchange' | 'token' | 'hub_unavailable'

export class CallbackError extends Error {
  constructor(readonly kind: CallbackErrorKind, message: string) {
    super(message)
    this.name = 'CallbackError'
  }
}

export interface Session {
  accessToken: string
  verified: VerifiedToken
}

export async function beginLogin(): Promise<void> {
  const verifier = generateVerifier()
  const state = generateState()
  const challenge = await challengeFor(verifier)
  sessionStorage.setItem(VERIFIER_KEY, verifier)
  sessionStorage.setItem(STATE_KEY, state)
  const query = new URLSearchParams({
    client_id: CLIENT_ID,
    redirect_uri: redirectUri(),
    response_type: 'code',
    state,
    code_challenge: challenge,
    code_challenge_method: 'S256',
  })
  redirectTo(`${HUB_ORIGIN}/oauth/authorize?${query.toString()}`)
}

function takeStoredFlow() {
  const flow = { verifier: sessionStorage.getItem(VERIFIER_KEY), state: sessionStorage.getItem(STATE_KEY) }
  sessionStorage.removeItem(VERIFIER_KEY)
  sessionStorage.removeItem(STATE_KEY)
  return flow
}

/**
 * Finishes the authorization code flow. The authorization code and state are
 * removed from the address bar before anything asynchronous happens, and the
 * stored verifier/state are consumed (single use) whatever the outcome.
 */
export async function completeCallback(search: string): Promise<Session> {
  const params = new URLSearchParams(search)
  window.history.replaceState(window.history.state, '', '/')
  const stored = takeStoredFlow()

  const returnedState = params.get('state')
  if (!stored.state || !stored.verifier || !returnedState || returnedState !== stored.state) {
    throw new CallbackError('state', 'state mismatch')
  }
  if (params.get('error')) throw new CallbackError('denied', 'authorization denied')
  const code = params.get('code')
  if (!code) throw new CallbackError('exchange', 'missing authorization code')

  let accessToken: string
  try {
    const response = await fetch(`${HUB_ORIGIN}/oauth/token`, {
      method: 'POST',
      credentials: 'omit',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'authorization_code',
        code,
        redirect_uri: redirectUri(),
        client_id: CLIENT_ID,
        code_verifier: stored.verifier,
      }).toString(),
    })
    if (!response.ok) throw new Error(`token endpoint answered ${response.status}`)
    const body: unknown = await response.json()
    const candidate = (body as { access_token?: unknown } | null)?.access_token
    if (typeof candidate !== 'string' || candidate === '') throw new Error('token response without access_token')
    accessToken = candidate
  } catch (reason) {
    throw new CallbackError('exchange', reason instanceof Error ? reason.message : 'token exchange failed')
  }

  let jwks: Jwks
  try {
    const response = await fetch(`${HUB_ORIGIN}/.well-known/jwks.json`, { credentials: 'omit' })
    if (!response.ok) throw new Error(`jwks answered ${response.status}`)
    const body: unknown = await response.json()
    if (typeof body !== 'object' || body === null || !Array.isArray((body as { keys?: unknown }).keys)) {
      throw new Error('jwks response without keys array')
    }
    jwks = body as Jwks
  } catch (reason) {
    throw new CallbackError('hub_unavailable', reason instanceof Error ? reason.message : 'JWKS retrieval failed')
  }

  try {
    const verified = await verifyAccessToken(accessToken, jwks, { issuer: ISSUER, audience: AUDIENCE })
    return { accessToken, verified }
  } catch (reason) {
    throw new CallbackError('token', reason instanceof Error ? reason.message : 'token verification failed')
  }
}
