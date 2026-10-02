import { expect, request, test } from '@playwright/test'
import { nginxAddress } from '../support/config'

const hub = { host: 'identityhub.localhost', connect: "connect-src 'self';" }
const contabilidad = { host: 'contabilidad.localhost', connect: "connect-src 'self' http://identityhub.localhost:8080;" }

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

async function expectSpaHeaders(host: string, connect: string) {
  const ctx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: host } })
  const response = await ctx.get('/')
  expect(response.status()).toBe(200)
  expectSecurityHeaders(response.headers(), connect)
  expect(response.headers()['server']).toBe('nginx')
  await ctx.dispose()
}

async function expectAssetHeaders(host: string, connect: string) {
  const ctx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: host } })
  const response = await ctx.get(await assetPath(host))
  expect(response.status()).toBe(200)
  expectSecurityHeaders(response.headers(), connect)
  expect(response.headers()['cache-control']).toContain('immutable')
  await ctx.dispose()
}

test('RNF-009 identityhub.localhost sends the five security headers on the SPA', async () => {
  await expectSpaHeaders(hub.host, hub.connect)
})

test('RNF-009 identityhub.localhost keeps the security headers on /assets/ files', async () => {
  await expectAssetHeaders(hub.host, hub.connect)
})

test('RNF-009 contabilidad.localhost sends the five security headers on the SPA', async () => {
  await expectSpaHeaders(contabilidad.host, contabilidad.connect)
})

test('RNF-009 contabilidad.localhost keeps the security headers on /assets/ files', async () => {
  await expectAssetHeaders(contabilidad.host, contabilidad.connect)
})

test('RNF-009 the Hub proxies /oauth/ to the API instead of the SPA index', async () => {
  const ctx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: 'identityhub.localhost' } })
  // Without client_id/redirect_uri the API answers a 400 problem; the SPA would answer 200 HTML.
  const response = await ctx.get('/oauth/authorize', { maxRedirects: 0 })
  expect(response.status()).toBe(400)
  expect(response.headers()['content-type'] ?? '').toContain('application/problem+json')
  await ctx.dispose()
})

test('RNF-009 Contabilidad serves its own SPA, distinct from the Hub', async () => {
  const hubCtx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: hub.host } })
  const contaCtx = await request.newContext({ baseURL: nginxAddress, extraHTTPHeaders: { Host: contabilidad.host } })
  const hubIndex = await (await hubCtx.get('/')).text()
  const contaResponse = await contaCtx.get('/oauth/callback')
  const contaIndex = await contaResponse.text()
  expect(contaResponse.status()).toBe(200)
  expect(contaIndex).toContain('<div id="root"')
  // Specific to the Contabilidad build, not just "different bytes".
  expect(contaIndex).toContain('<title>Contabilidad</title>')
  expect(hubIndex).toContain('<title>Identity Hub</title>')
  expect(contaIndex).not.toBe(hubIndex)
  await hubCtx.dispose()
  await contaCtx.dispose()
})
