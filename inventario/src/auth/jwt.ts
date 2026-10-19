import { decodeBase64Url } from './base64url'

export const CLOCK_LEEWAY_SECONDS = 60

export interface Jwk {
  kty?: string
  crv?: string
  x?: string
  kid?: string
  alg?: string
  use?: string
}

export interface Jwks {
  keys: Jwk[]
}

export interface ExpectedToken {
  issuer: string
  audience: string
  /** Unix seconds; injectable so tests do not depend on the wall clock. */
  now?: () => number
}

export interface VerifiedToken {
  subject: string
  roles: string[]
  permissions: string[]
  expiresAt: number
}

export class TokenVerificationError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'TokenVerificationError'
  }
}

function parseJson(segment: string): Record<string, unknown> {
  try {
    const value: unknown = JSON.parse(new TextDecoder().decode(decodeBase64Url(segment)))
    if (typeof value === 'object' && value !== null && !Array.isArray(value)) return value as Record<string, unknown>
  } catch {
    // Falls through to the malformed error below.
  }
  throw new TokenVerificationError('malformed token')
}

/**
 * Verifies an Identity Hub access token entirely in the browser (ADR 0009: the
 * SPA has no backend). Only EdDSA is accepted and the algorithm comes from the
 * pinned expectation, never from the key set, so `none` and HMAC downgrades fail.
 */
export async function verifyAccessToken(token: string, jwks: Jwks, expected: ExpectedToken): Promise<VerifiedToken> {
  const parts = token.split('.')
  if (parts.length !== 3) throw new TokenVerificationError('malformed token')
  const [headerPart, payloadPart, signaturePart] = parts as [string, string, string]
  const header = parseJson(headerPart)
  if (header.alg !== 'EdDSA') throw new TokenVerificationError('unsupported algorithm')

  const jwk = jwks.keys.find((candidate) => candidate.kid === header.kid && candidate.kty === 'OKP' && candidate.crv === 'Ed25519' && typeof candidate.x === 'string')
  if (!jwk || typeof header.kid !== 'string') throw new TokenVerificationError('no matching signing key')

  let valid = false
  try {
    const publicKey = await crypto.subtle.importKey('jwk', { kty: 'OKP', crv: 'Ed25519', x: jwk.x }, { name: 'Ed25519' }, false, ['verify'])
    valid = await crypto.subtle.verify({ name: 'Ed25519' }, publicKey, decodeBase64Url(signaturePart), new TextEncoder().encode(`${headerPart}.${payloadPart}`))
  } catch {
    valid = false
  }
  if (!valid) throw new TokenVerificationError('invalid signature')

  const claims = parseJson(payloadPart)
  if (claims.iss !== expected.issuer) throw new TokenVerificationError('unexpected issuer')
  const audience = Array.isArray(claims.aud) ? claims.aud : [claims.aud]
  if (!audience.includes(expected.audience)) throw new TokenVerificationError('unexpected audience')
  const now = (expected.now ?? (() => Math.floor(Date.now() / 1000)))()
  if (typeof claims.exp !== 'number' || claims.exp < now - CLOCK_LEEWAY_SECONDS) throw new TokenVerificationError('token expired')
  if (typeof claims.nbf === 'number' && claims.nbf > now + CLOCK_LEEWAY_SECONDS) throw new TokenVerificationError('token not yet valid')
  if (typeof claims.sub !== 'string' || claims.sub === '') throw new TokenVerificationError('missing subject')

  // The Hub serializes an empty role set as null.
  const rawRoles = claims.roles ?? []
  if (!Array.isArray(rawRoles) || !rawRoles.every((role) => typeof role === 'string')) throw new TokenVerificationError('invalid roles claim')
  // Same for permissions; a token issued before permissions existed has no claim at all.
  const rawPermissions = claims.permissions ?? []
  if (!Array.isArray(rawPermissions) || !rawPermissions.every((permission) => typeof permission === 'string')) throw new TokenVerificationError('invalid permissions claim')
  return { subject: claims.sub, roles: rawRoles as string[], permissions: rawPermissions as string[], expiresAt: claims.exp }
}
