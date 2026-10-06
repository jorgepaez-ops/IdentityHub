// Captures the Grafana dashboard, Explore (Loki logs and Prometheus metrics) and the Prometheus
// targets page for the technical report.
//
//   make up-obs                       # the observability profile must be running
//   node capturas-grafana.mjs [--out DIR]
//
// Default output: a folder under the OS temp directory (binaries are copied into docs/informe/img/
// by hand once the captures are reviewed, so iCloud never syncs half-written files).
//
// Credentials: GRAFANA_ADMIN_USER / GRAFANA_ADMIN_PASSWORD from the environment, or, when unset,
// read from the running Grafana container. They are used only to open a session through the HTTP
// login endpoint, are never printed and never appear in a capture (the login form is not shown).

import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { chromium } from 'playwright-core'

const args = process.argv.slice(2)
const outIdx = args.indexOf('--out')
const OUT = outIdx >= 0 ? path.resolve(args[outIdx + 1]) : path.join(os.tmpdir(), 'informe-grafana')
const GRAFANA = process.env.GRAFANA_URL ?? 'http://localhost:3000'
const PROMETHEUS = process.env.PROMETHEUS_URL ?? 'http://localhost:9090'
const CONTAINER = process.env.GRAFANA_CONTAINER ?? 'identity-hub-grafana-1'
// Same convention as e2e/support/db.ts: the Docker CLI can be pinned to an absolute path.
const DOCKER = process.env.INFORME_DOCKER ?? 'docker'

function fromContainer(name) {
  return execFileSync(DOCKER, ['exec', CONTAINER, 'printenv', name], { encoding: 'utf8' }).trim()
}
const user = process.env.GRAFANA_ADMIN_USER ?? fromContainer('GF_SECURITY_ADMIN_USER')
// Read at run time from the environment or the container, never stored in this file.
const clave = process.env.GRAFANA_ADMIN_PASSWORD ?? fromContainer('GF_SECURITY_ADMIN_PASSWORD')

fs.mkdirSync(OUT, { recursive: true })
const browser = await chromium.launch()
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
  deviceScaleFactor: 1,
  locale: 'es-CO',
  colorScheme: 'light',
})

// The browser is closed in `finally`, so a failed login or capture never leaves Chromium running.
try {
  const login = await context.request.post(`${GRAFANA}/login`, { data: { user, password: clave } })
  if (!login.ok()) throw new Error(`Grafana login failed (${login.status()})`)

  const sources = await (await context.request.get(`${GRAFANA}/api/datasources`)).json()
  const uid = (type) => sources.find((s) => s.type === type)?.uid
  const lokiUid = uid('loki')
  const promUid = uid('prometheus')
  if (!lokiUid || !promUid) throw new Error('Loki or Prometheus data source is not provisioned')

  const page = await context.newPage()
  const shot = async (name) => {
    await page.waitForTimeout(3500)
    await page.screenshot({ path: path.join(OUT, name), fullPage: false })
    console.log('captured', name)
  }

  // 1. Provisioned dashboard "Identity Hub — Seguridad"
  await page.goto(`${GRAFANA}/d/identity-security?orgId=1&from=now-1h&to=now&refresh=`, { waitUntil: 'networkidle' })
  await shot('grafana-dashboard-seguridad.png')

  // 2. Explore with Loki: structured logs of the worker (asynchronous notifications)
  const explore = (ds, dsType, expr, extra = {}) =>
    `${GRAFANA}/explore?schemaVersion=1&orgId=1&panes=` +
    encodeURIComponent(
      JSON.stringify({
        a: {
          datasource: ds,
          queries: [{ refId: 'A', expr, datasource: { type: dsType, uid: ds }, ...extra }],
          range: { from: 'now-1h', to: 'now' },
        },
      }),
    )

  await page.goto(explore(lokiUid, 'loki', '{service="worker"} | json | msg="notificación entregada"'), { waitUntil: 'networkidle' })
  await shot('grafana-explore-loki.png')

  // 3. Explore with Prometheus: events consumed by the worker, by type
  await page.goto(
    explore(promUid, 'prometheus', 'sum by (event_type) (rate(identity_events_consumed_total[5m]))', { range: true, instant: false }),
    { waitUntil: 'networkidle' },
  )
  await shot('grafana-explore-prometheus.png')

  // 4. Prometheus: scrape targets
  await page.goto(`${PROMETHEUS}/targets`, { waitUntil: 'load' })
  await shot('prometheus-targets.png')
} finally {
  await browser.close()
}
