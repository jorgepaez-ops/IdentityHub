import { vi } from 'vitest'
import { CLIENT_ID, HUB_ORIGIN } from '../hub'
import { bytesToB64u } from '../oauth/base64'
import type { PublicKey } from '../oauth/tokenVerifier'

const utf8 = (value: unknown) => new TextEncoder().encode(JSON.stringify(value))

/** A stand-in for the Hub: owns an Ed25519 key pair, mints tokens and answers fetch calls. */
export class FakeHub {
  private constructor(
    readonly kid: string,
    private readonly privateKey: CryptoKey,
    readonly publicKey: PublicKey,
  ) {}

  static async create(kid = 'hub-key'): Promise<FakeHub> {
    const pair = (await crypto.subtle.generateKey({ name: 'Ed25519' }, true, ['sign', 'verify'])) as CryptoKeyPair
    const jwk = await crypto.subtle.exportKey('jwk', pair.publicKey)
    return new FakeHub(kid, pair.privateKey, { kty: 'OKP', crv: 'Ed25519', x: jwk.x, kid })
  }

  /** Claims of a healthy Inventario token, valid for 15 minutes from `now` (seconds). */
  claims(now: number, overrides: Record<string, unknown> = {}): Record<string, unknown> {
    return {
      iss: HUB_ORIGIN,
      sub: '3f2c1a9e-0000-4000-8000-000000000001',
      aud: [CLIENT_ID],
      iat: now,
      exp: now + 900,
      roles: ['inventario.bodeguero'],
      permissions: ['productos.ver', 'productos.editar', 'movimientos.registrar'],
      ...overrides,
    }
  }

  async mint(claims: Record<string, unknown>, header: Record<string, unknown> = {}): Promise<string> {
    const head = bytesToB64u(utf8({ alg: 'EdDSA', typ: 'JWT', kid: this.kid, ...header }))
    const signed = `${head}.${bytesToB64u(utf8(claims))}`
    const signature = await crypto.subtle.sign({ name: 'Ed25519' }, this.privateKey, new TextEncoder().encode(signed))
    return `${signed}.${bytesToB64u(new Uint8Array(signature))}`
  }

  /**
   * Replaces `fetch` with a Hub double. `token` decides what the token endpoint answers; the key set
   * endpoint always serves this hub's public key. Returns the requests seen so far.
   */
  serve(token: () => Response | Promise<Response>) {
    const seen: { url: string; init?: RequestInit }[] = []
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      seen.push({ url, init })
      if (url === `${HUB_ORIGIN}/oauth/token`) return token()
      if (url === `${HUB_ORIGIN}/.well-known/jwks.json`) return reply(200, { keys: [this.publicKey] })
      throw new Error(`unexpected request to ${url}`)
    }))
    return seen
  }

  /** Like `serve`, but the token endpoint hands out a token with these claim overrides. */
  serveToken(overrides: Record<string, unknown> = {}) {
    return this.serve(async () => {
      const now = Math.floor(Date.now() / 1000)
      return reply(200, { access_token: await this.mint(this.claims(now, overrides)), token_type: 'Bearer', expires_in: 900 })
    })
  }
}

export const reply = (status: number, body: unknown): Response =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

/** A fetch that only settles when aborted, like a Hub that accepts the connection and goes quiet. */
export const silentUntilAborted = (init?: RequestInit) =>
  new Promise<Response>((_resolve, reject) => {
    init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
  })
