import { encodeBase64Url } from './base64url'

function randomToken(byteLength: number): string {
  return encodeBase64Url(crypto.getRandomValues(new Uint8Array(byteLength)))
}

/** RFC 7636 section 4.1: 32 random bytes encode to a 43-character verifier. */
export function generateVerifier(): string {
  return randomToken(32)
}

/** Unguessable value that binds the callback to the browser tab that started the flow. */
export function generateState(): string {
  return randomToken(16)
}

/** RFC 7636 section 4.2, S256: BASE64URL(SHA256(ASCII(verifier))). */
export async function challengeFor(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
  return encodeBase64Url(new Uint8Array(digest))
}
