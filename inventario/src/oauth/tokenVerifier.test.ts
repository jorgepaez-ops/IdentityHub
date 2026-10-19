import { beforeAll, describe, expect, it } from 'vitest'
import { FakeHub } from '../testkit/fakeHub'
import { bytesToB64u } from './base64'
import { LEEWAY_SECONDS, verifyToken, type Rejection } from './tokenVerifier'

const NOW = 1_800_000_000
const expected = { issuer: 'http://identityhub.localhost:8080', audience: 'inventario', now: NOW }

let hub: FakeHub
beforeAll(async () => { hub = await FakeHub.create('k1') })

const keySet = () => ({ keys: [hub.publicKey] })
const mintWith = (overrides: Record<string, unknown> = {}, header: Record<string, unknown> = {}) => hub.mint(hub.claims(NOW, overrides), header)

async function reasonOf(token: string): Promise<Rejection | undefined> {
  try {
    await verifyToken(token, keySet(), expected)
  } catch (problem) {
    return (problem as { reason?: Rejection }).reason
  }
  return undefined
}

describe('access token verification against the Hub keys', () => {
  it('TestRF020_AcceptsAValidTokenAndExposesIdentityRolesAndPermissions', async () => {
    const identity = await verifyToken(await mintWith(), keySet(), expected)
    expect(identity).toEqual({
      subject: '3f2c1a9e-0000-4000-8000-000000000001',
      roles: ['inventario.bodeguero'],
      permissions: ['productos.ver', 'productos.editar', 'movimientos.registrar'],
      expiresAt: NOW + 900,
    })
  })

  it('TestRF020_AcceptsAudienceGivenAsAPlainString', async () => {
    await expect(verifyToken(await mintWith({ aud: 'inventario' }), keySet(), expected)).resolves.toBeTruthy()
  })

  it.each([
    ['null', null],
    ['absent', undefined],
  ])('TestRF021_A%sRolesOrPermissionsClaimMeansNone', async (_label, value) => {
    const identity = await verifyToken(await mintWith({ roles: value, permissions: value }), keySet(), expected)
    expect(identity.roles).toEqual([])
    expect(identity.permissions).toEqual([])
  })

  it('TestRF021_RejectsAPermissionsClaimThatIsNotAListOfStrings', async () => {
    expect(await reasonOf(await mintWith({ permissions: 'productos.ver' }))).toBe('claims')
    expect(await reasonOf(await mintWith({ permissions: [1, 2] }))).toBe('claims')
  })

  it('TestRF020_RejectsASignatureMadeByAnotherKey', async () => {
    const stranger = await FakeHub.create('k1')
    expect(await reasonOf(await stranger.mint(stranger.claims(NOW)))).toBe('signature')
  })

  it('TestRF020_RejectsAPayloadEditedAfterSigning', async () => {
    const [head, , signature] = (await mintWith({ roles: ['inventario.consulta'] })).split('.')
    const forged = bytesToB64u(new TextEncoder().encode(JSON.stringify(hub.claims(NOW))))
    expect(await reasonOf(`${head}.${forged}.${signature}`)).toBe('signature')
  })

  it('TestRF020_RejectsAnotherIssuerAndAnotherAudience', async () => {
    expect(await reasonOf(await mintWith({ iss: 'http://evil.localhost:8080' }))).toBe('issuer')
    expect(await reasonOf(await mintWith({ aud: ['contabilidad'] }))).toBe('audience')
    expect(await reasonOf(await mintWith({ aud: undefined }))).toBe('audience')
  })

  it('TestRF020_ClockLeewayAppliesToExpiryAndNotBefore', async () => {
    expect(await reasonOf(await mintWith({ exp: NOW - LEEWAY_SECONDS, nbf: NOW + LEEWAY_SECONDS }))).toBeUndefined()
    expect(await reasonOf(await mintWith({ exp: NOW - LEEWAY_SECONDS - 1 }))).toBe('expired')
    expect(await reasonOf(await mintWith({ nbf: NOW + LEEWAY_SECONDS + 1 }))).toBe('not_yet_valid')
  })

  it('TestRF020_RejectsATokenWithoutExpiry', async () => {
    expect(await reasonOf(await mintWith({ exp: undefined }))).toBe('expired')
  })

  it('TestRF020_OnlyEdDSAIsAccepted', async () => {
    expect(await reasonOf(await mintWith({}, { alg: 'none' }))).toBe('algorithm')
    expect(await reasonOf(await mintWith({}, { alg: 'HS256' }))).toBe('algorithm')
  })

  it('TestRF020_RejectsAKeyIdTheHubDoesNotPublish', async () => {
    expect(await reasonOf(await mintWith({}, { kid: 'rotated-away' }))).toBe('unknown_key')
  })

  it('TestRF020_RejectsTokensThatAreNotThreeDecodableParts', async () => {
    expect(await reasonOf('not-a-jwt')).toBe('malformed')
    expect(await reasonOf('a.b.c')).toBe('malformed')
    expect(await reasonOf(`${bytesToB64u(new TextEncoder().encode('[]'))}.x.y`)).toBe('malformed')
  })
})
