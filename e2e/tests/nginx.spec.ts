import { expect, request, test } from '@playwright/test'
import { nginxAddress } from '../support/config'

const hosts = [
  { host: 'identityhub.localhost', connect: "connect-src 'self';" },
  { host: 'contabilidad.localhost', connect: "connect-src 'self' http://identityhub.localhost:8080;" },
]

const staticHeaders = {
  'strict-transport-security': 'max-age=31536000; includeSubDomains',
  'x-content-type-options': 'nosniff',
  'x-frame-options': 'DENY',
  'referrer-policy': 'strict-origin-when-cross-origin',
}

function expectSecurityHeaders(headers: Record<string, string>, connect: string) {
  expect(headers['content-security-policy']).toContain("default-src 'self'")
  expect(headers['content-security-policy']).toContain(connect)
  expect(headers['content-security-policy']).toContain("frame-ancestors 'none'")
  for (const [name, value] of Object.entries(staticHeaders)) expect(headers[name]).toBe(value)
}

async function assetPath(host: string): Promise<string> {
  const ctx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: host } })
  const html = await (await ctx.get('/')).text()
  await ctx.dispose()
  const match = /\/assets\/[^"']+/.exec(html)
  if (!match) throw new Error(`No /assets/ reference in ${host} index`)
  return match[0]
}

for (const { host, connect } of hosts) {
  test.describe(host, () => {
    test(`RNF-009 ${host} sends the five security headers on the SPA`, async () => {
      const ctx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: host } })
      const response = await ctx.get('/')
      expect(response.status()).toBe(200)
      expectSecurityHeaders(response.headers(), connect)
      expect(response.headers()['server']).toBe('nginx')
      await ctx.dispose()
    })

    test(`RNF-009 ${host} keeps the security headers on /assets/ files`, async () => {
      const ctx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: host } })
      const response = await ctx.get(await assetPath(host))
      expect(response.status()).toBe(200)
      expectSecurityHeaders(response.headers(), connect)
      expect(response.headers()['cache-control']).toContain('immutable')
      await ctx.dispose()
    })
  })
}

test('RNF-009 the Hub proxies /oauth/ to the API instead of the SPA index', async () => {
  const ctx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: 'identityhub.localhost' } })
  // Without client_id/redirect_uri the API answers a 400 problem; the SPA would answer 200 HTML.
  const response = await ctx.get('/oauth/authorize', { maxRedirects: 0 })
  expect(response.status()).toBe(400)
  expect(response.headers()['content-type'] ?? '').toContain('application/problem+json')
  await ctx.dispose()
})

test('RNF-009 Contabilidad serves its own SPA, distinct from the Hub', async () => {
  const hub = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: 'identityhub.localhost' } })
  const conta = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: 'contabilidad.localhost' } })
  const hubIndex = await (await hub.get('/')).text()
  const contaIndex = await (await conta.get('/oauth/callback')).text()
  expect(contaIndex).toContain('<div id="root"')
  expect(contaIndex).not.toBe(hubIndex)
  await hub.dispose()
  await conta.dispose()
})
