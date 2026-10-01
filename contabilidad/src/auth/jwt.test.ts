import { beforeAll, describe, expect, it } from 'vitest'
import { encodeBase64Url } from './base64url'
import { verifyAccessToken } from './jwt'
import { AUDIENCE, ISSUER } from '../config'
import { NOW, makeKey, signToken, validClaims, type TestKey } from '../testing'

let key: TestKey
let other: TestKey
const expected = { issuer: ISSUER, audience: AUDIENCE, now: () => NOW }

beforeAll(async () => {
  key = await makeKey('hub-key')
  other = await makeKey('hub-key')
})

describe('access token verification against the Hub JWKS', () => {
  it('TestRF020_AcceptsValidTokenAndReturnsRoles', async () => {
    const token = await signToken(key, validClaims({ roles: ['contabilidad.analista'] }))
    const verified = await verifyAccessToken(token, { keys: [key.jwk] }, expected)
    expect(verified.roles).toEqual(['contabilidad.analista'])
    expect(verified.subject).toBe('3f2c1a9e-0000-4000-8000-000000000001')
    expect(verified.expiresAt).toBe(NOW + 900)
  })

  it('TestRF020_AcceptsStringAudience', async () => {
    const token = await signToken(key, validClaims({ aud: AUDIENCE }))
    await expect(verifyAccessToken(token, { keys: [key.jwk] }, expected)).resolves.toBeTruthy()
  })

  it('TestRF020_NullRolesClaimMeansNoRoles', async () => {
    // The backend serializes an empty role set as null (append to a nil slice).
    const token = await signToken(key, validClaims({ roles: null }))
    const verified = await verifyAccessToken(token, { keys: [key.jwk] }, expected)
    expect(verified.roles).toEqual([])
  })

  it('TestRF020_RejectsBadSignature', async () => {
    // Signed by a different key that reuses the same kid: the signature cannot verify.
    const token = await signToken(other, validClaims())
    await expect(verifyAccessToken(token, { keys: [key.jwk] }, expected)).rejects.toThrow(/signature/)
  })

  it('TestRF020_RejectsTamperedPayload', async () => {
    const token = await signToken(key, validClaims({ roles: ['contabilidad.analista'] }))
    const [header, , signature] = token.split('.')
    const forged = encodeBase64Url(new TextEncoder().encode(JSON.stringify(validClaims({ roles: ['contabilidad.senior'] }))))
    await expect(verifyAccessToken(`${header}.${forged}.${signature}`, { keys: [key.jwk] }, expected)).rejects.toThrow(/signature/)
  })

  it('TestRF020_RejectsWrongIssuer', async () => {
    const token = await signToken(key, validClaims({ iss: 'http://evil.localhost:8080' }))
    await expect(verifyAccessToken(token, { keys: [key.jwk] }, expected)).rejects.toThrow(/issuer/)
  })

  it('TestRF020_RejectsWrongAudience', async () => {
    const token = await signToken(key, validClaims({ aud: ['otra-app'] }))
    await expect(verifyAccessToken(token, { keys: [key.jwk] }, expected)).rejects.toThrow(/audience/)
  })

  it('TestRF020_RejectsExpiredToken', async () => {
    const token = await signToken(key, validClaims({ exp: NOW - 61 }))
    await expect(verifyAccessToken(token, { keys: [key.jwk] }, expected)).rejects.toThrow(/expired/)
  })

  it('TestRF020_AcceptsClockSkewAtExpiryAndNotBeforeBoundaries', async () => {
    const expAtLeeway = await signToken(key, validClaims({ exp: NOW - 60, nbf: NOW + 60 }))
    await expect(verifyAccessToken(expAtLeeway, { keys: [key.jwk] }, expected)).resolves.toBeTruthy()
    const expPastLeeway = await signToken(key, validClaims({ exp: NOW - 61 }))
    await expect(verifyAccessToken(expPastLeeway, { keys: [key.jwk] }, expected)).rejects.toThrow(/expired/)
    const nbfPastLeeway = await signToken(key, validClaims({ nbf: NOW + 61 }))
    await expect(verifyAccessToken(nbfPastLeeway, { keys: [key.jwk] }, expected)).rejects.toThrow(/not yet valid/)
  })

  it('TestRF020_RejectsMissingExpiry', async () => {
    const claims: Record<string, unknown> = validClaims()
    delete claims.exp
    await expect(verifyAccessToken(await signToken(key, claims), { keys: [key.jwk] }, expected)).rejects.toThrow(/expired/)
  })

  it('TestRF020_RejectsAlgorithmOtherThanEdDSA', async () => {
    const none = await signToken(key, validClaims(), { alg: 'none' })
    await expect(verifyAccessToken(none, { keys: [key.jwk] }, expected)).rejects.toThrow(/algorithm/)
    const hmac = await signToken(key, validClaims(), { alg: 'HS256' })
    await expect(verifyAccessToken(hmac, { keys: [key.jwk] }, expected)).rejects.toThrow(/algorithm/)
  })

  it('TestRF020_RejectsUnknownKid', async () => {
    const token = await signToken(key, validClaims(), { kid: 'rotated-away' })
    await expect(verifyAccessToken(token, { keys: [key.jwk] }, expected)).rejects.toThrow(/key/)
  })

  it('TestRF020_RejectsMalformedToken', async () => {
    await expect(verifyAccessToken('not-a-jwt', { keys: [key.jwk] }, expected)).rejects.toThrow(/malformed/)
    await expect(verifyAccessToken('a.b.c', { keys: [key.jwk] }, expected)).rejects.toThrow()
  })
})
