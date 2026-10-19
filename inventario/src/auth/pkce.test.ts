import { describe, expect, it } from 'vitest'
import { challengeFor, generateState, generateVerifier } from './pkce'

const BASE64URL = /^[A-Za-z0-9_-]+$/

describe('PKCE (RFC 7636)', () => {
  it('TestRF020_ChallengeMatchesRFC7636AppendixBVector', async () => {
    // RFC 7636 Appendix B: the only normative S256 test vector.
    expect(await challengeFor('dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk')).toBe('E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM')
  })

  it('TestRF020_VerifierHasAtLeast32RandomBytesInBase64url', () => {
    const first = generateVerifier()
    const second = generateVerifier()
    expect(first).toMatch(BASE64URL)
    expect(first.length).toBeGreaterThanOrEqual(43)
    expect(first.length).toBeLessThanOrEqual(128)
    expect(first).not.toBe(second)
  })

  it('TestRF020_StateIsRandomBase64url', () => {
    const first = generateState()
    expect(first).toMatch(BASE64URL)
    expect(first.length).toBeGreaterThanOrEqual(22)
    expect(first).not.toBe(generateState())
  })
})
