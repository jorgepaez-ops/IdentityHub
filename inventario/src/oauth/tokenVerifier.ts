import { b64uToBytes } from './base64'

/** Tolerated difference between the Hub clock and this device, in seconds. */
export const LEEWAY_SECONDS = 60

export interface PublicKey {
  kty?: string
  crv?: string
  x?: string
  kid?: string
}
export interface KeySet {
  keys: PublicKey[]
}
export interface Expectations {
  issuer: string
  audience: string
  /** Current time in seconds since the epoch. */
  now: number
}
export interface Identity {
  subject: string
  roles: string[]
  permissions: string[]
  /** Seconds since the epoch. */
  expiresAt: number
}

export type Rejection =
  | 'malformed' | 'algorithm' | 'unknown_key' | 'signature'
  | 'issuer' | 'audience' | 'expired' | 'not_yet_valid' | 'claims'

export class TokenRejected extends Error {
  readonly reason: Rejection
  constructor(reason: Rejection) {
    super(`access token rejected: ${reason}`)
    this.reason = reason
  }
}

type Json = Record<string, unknown>

function parseSegment(segment: string): Json {
  try {
    const value: unknown = JSON.parse(new TextDecoder().decode(b64uToBytes(segment)))
    if (typeof value === 'object' && value !== null && !Array.isArray(value)) return value as Json
  } catch {
    // falls through to the rejection below
  }
  throw new TokenRejected('malformed')
}

async function signatureHolds(key: PublicKey, signed: string, signature: string): Promise<boolean> {
  try {
    const publicKey = await crypto.subtle.importKey('jwk', { kty: 'OKP', crv: 'Ed25519', x: key.x }, { name: 'Ed25519' }, false, ['verify'])
    return await crypto.subtle.verify({ name: 'Ed25519' }, publicKey, b64uToBytes(signature), new TextEncoder().encode(signed))
  } catch {
    return false
  }
}

/** A claim that is a list of strings; `null` or a missing claim means an empty list. */
function stringList(claim: unknown): string[] {
  if (claim === undefined || claim === null) return []
  if (Array.isArray(claim) && claim.every((entry) => typeof entry === 'string')) return claim
  throw new TokenRejected('claims')
}

/**
 * Checks an EdDSA-signed JWT against the Hub key set: algorithm, signature, issuer, audience and
 * validity window (with leeway). Anything else about the token is untrusted until this resolves.
 */
export async function verifyToken(token: string, keySet: KeySet, expected: Expectations): Promise<Identity> {
  const parts = token.split('.')
  if (parts.length !== 3) throw new TokenRejected('malformed')
  const [head = '', body = '', signature = ''] = parts
  const header = parseSegment(head)
  if (header.alg !== 'EdDSA') throw new TokenRejected('algorithm')

  const key = keySet.keys.find((candidate) => candidate.kid === header.kid && candidate.kty === 'OKP' && candidate.crv === 'Ed25519')
  if (!key) throw new TokenRejected('unknown_key')
  if (!(await signatureHolds(key, `${head}.${body}`, signature))) throw new TokenRejected('signature')

  const claims = parseSegment(body)
  if (claims.iss !== expected.issuer) throw new TokenRejected('issuer')
  const audiences = typeof claims.aud === 'string' ? [claims.aud] : claims.aud
  if (!Array.isArray(audiences) || !audiences.includes(expected.audience)) throw new TokenRejected('audience')

  const expiresAt = claims.exp
  if (typeof expiresAt !== 'number' || expiresAt + LEEWAY_SECONDS < expected.now) throw new TokenRejected('expired')
  if (typeof claims.nbf === 'number' && claims.nbf - LEEWAY_SECONDS > expected.now) throw new TokenRejected('not_yet_valid')
  if (typeof claims.sub !== 'string') throw new TokenRejected('claims')

  return { subject: claims.sub, roles: stringList(claims.roles), permissions: stringList(claims.permissions), expiresAt }
}
