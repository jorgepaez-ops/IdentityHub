import { describe, expect, it } from 'vitest'
import { b64uToBytes, bytesToB64u } from './base64'
import { randomToken, s256 } from './pkce'

describe('PKCE helpers (RFC 7636)', () => {
  it('TestRF020_S256MatchesTheVectorOfAppendixB', async () => {
    expect(await s256('dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk')).toBe('E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM')
  })

  it('TestRF020_RandomTokensAreLongUrlSafeAndDifferent', () => {
    const first = randomToken(48)
    expect(first).toMatch(/^[A-Za-z0-9_-]{64}$/)
    expect(randomToken(48)).not.toBe(first)
  })

  it('TestRF020_Base64UrlRoundTripsBytesThatNeedPaddingAndSpecialCharacters', () => {
    const bytes = Uint8Array.from([251, 255, 190, 1, 2])
    const text = bytesToB64u(bytes)
    expect(text).not.toMatch(/[+/=]/)
    expect(Array.from(b64uToBytes(text))).toEqual(Array.from(bytes))
  })

  it('TestRF020_Base64UrlRefusesCharactersOutsideItsAlphabet', () => {
    expect(() => b64uToBytes('ab+/')).toThrow()
  })
})
