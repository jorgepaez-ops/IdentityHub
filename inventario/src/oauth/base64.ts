const URL_SAFE = /^[A-Za-z0-9_-]*$/

export function bytesToB64u(bytes: Uint8Array): string {
  let text = ''
  for (const byte of bytes) text += String.fromCharCode(byte)
  return btoa(text).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '')
}

export function b64uToBytes(encoded: string): Uint8Array<ArrayBuffer> {
  if (!URL_SAFE.test(encoded)) throw new Error('not base64url')
  const padded = encoded.padEnd(Math.ceil(encoded.length / 4) * 4, '=')
  const text = atob(padded.replaceAll('-', '+').replaceAll('_', '/'))
  return Uint8Array.from(text, (char) => char.codePointAt(0) ?? 0)
}
