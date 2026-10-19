import { bytesToB64u } from './base64'

/** Unguessable URL-safe string made of `size` random bytes. */
export function randomToken(size: number): string {
  return bytesToB64u(crypto.getRandomValues(new Uint8Array(size)))
}

/** The S256 code challenge of RFC 7636 section 4.2. */
export async function s256(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
  return bytesToB64u(new Uint8Array(digest))
}
