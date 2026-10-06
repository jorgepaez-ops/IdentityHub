import { encodeBase64Url } from './auth/base64url'
import { AUDIENCE, ISSUER } from './config'

export const NOW = 1_800_000_000

export interface TestKey {
  kid: string
  privateKey: CryptoKey
  jwk: { kty: string; crv: string; x: string; kid: string; use: string; alg: string }
}

export async function makeKey(kid = 'test-key'): Promise<TestKey> {
  const pair = (await crypto.subtle.generateKey({ name: 'Ed25519' }, true, ['sign', 'verify'])) as CryptoKeyPair
  const exported = await crypto.subtle.exportKey('jwk', pair.publicKey)
  return { kid, privateKey: pair.privateKey, jwk: { kty: 'OKP', crv: 'Ed25519', x: String(exported.x), kid, use: 'sig', alg: 'EdDSA' } }
}

const encodeJson = (value: unknown) => encodeBase64Url(new TextEncoder().encode(JSON.stringify(value)))

export async function signToken(key: TestKey, claims: Record<string, unknown>, header: Record<string, unknown> = {}): Promise<string> {
  const signingInput = `${encodeJson({ alg: 'EdDSA', typ: 'JWT', kid: key.kid, ...header })}.${encodeJson(claims)}`
  const signature = await crypto.subtle.sign({ name: 'Ed25519' }, key.privateKey, new TextEncoder().encode(signingInput))
  return `${signingInput}.${encodeBase64Url(new Uint8Array(signature))}`
}

export const ALL_PERMISSIONS = ['movimientos.registrar', 'movimientos.ver_todos', 'movimientos.aprobar', 'cierre.ejecutar', 'reportes.ver']

/** Permissions the seeded roles carry in the Hub (migration of the role grid). */
export const SEEDED_PERMISSIONS: Record<string, string[]> = {
  'contabilidad.senior': ALL_PERMISSIONS,
  'contabilidad.analista': ['movimientos.registrar', 'reportes.ver'],
}

export const validClaims = (overrides: Record<string, unknown> = {}) => ({
  iss: ISSUER,
  sub: '3f2c1a9e-0000-4000-8000-000000000001',
  aud: [AUDIENCE],
  exp: NOW + 900,
  iat: NOW,
  jti: 'abc',
  roles: ['contabilidad.senior'],
  permissions: ALL_PERMISSIONS,
  ...overrides,
})
