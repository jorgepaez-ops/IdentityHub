import { readFileSync } from 'node:fs'
import { expect, test } from '@playwright/test'
import { hubUrl } from '../support/config'
import { createActiveUser, seedAdmin, strongPassword } from '../support/accounts'
import { api } from '../support/api'
import { contextApi } from '../support/auth'
import { linesLeaking, serviceLogs } from '../support/logs'
import { extractCode, mailIds, waitForMail } from '../support/mailpit'

const apiMetricsUrl = process.env.E2E_API_METRICS_URL ?? 'http://localhost:8081/metrics'
const workerMetricsUrl = process.env.E2E_WORKER_METRICS_URL ?? 'http://localhost:9091/metrics'
const dashboardPath = new URL('../../deploy/observability/grafana/dashboards/seguridad.json', import.meta.url)

async function metrics(url: string): Promise<string> {
  const response = await fetch(url, { signal: AbortSignal.timeout(10_000) })
  expect(response.status).toBe(200)
  return response.text()
}

/** Sum of every sample of `name` whose label set contains `label` (e.g. result="failed"). */
function counter(text: string, name: string, label: string): number {
  return text
    .split('\n')
    .filter((line) => line.startsWith(`${name}{`) && line.includes(label))
    .reduce((total, line) => total + Number(line.slice(line.lastIndexOf(' ') + 1)), 0)
}

const loginBody = (email: string, pw: string) => ({ email, password: pw })

function isJsonObject(line: string): boolean {
  try {
    const parsed: unknown = JSON.parse(line)
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
  } catch {
    return false
  }
}

test('RNF-007 La API y el worker exponen métricas Prometheus, logs JSON y el panel de Grafana', async () => {
  const email = await createActiveUser(await seedAdmin())

  // La API y el worker responden en formato de exposición de Prometheus.
  const before = await metrics(apiMetricsUrl)
  expect(before).toContain('# TYPE identity_http_requests_total counter')
  expect(before).toContain('# TYPE identity_http_request_duration_seconds histogram')

  // Un intento fallido real incrementa el contador que alimenta el panel de logins fallidos.
  const failedBefore = counter(before, 'identity_login_attempts_total', 'result="failed"')
  expect((await api('POST', '/api/v1/auth/login', { json: loginBody(email, `incorrecta-${strongPassword}`) })).status).toBe(401)
  // El contador es global al proceso de la API y no se reinicia entre archivos de prueba: se exige que suba
  // al menos 1 (no exactamente 1), así otro login fallido concurrente o rezagado no vuelve frágil la prueba;
  // si la métrica no reaccionara al intento, el valor se quedaría en failedBefore y esto fallaría.
  const failedAfter = counter(await metrics(apiMetricsUrl), 'identity_login_attempts_total', 'result="failed"')
  expect(failedAfter).toBeGreaterThanOrEqual(failedBefore + 1)

  // El worker consume el evento del correo de la invitación/MFA: su contador aparece en /metrics.
  await expect
    .poll(async () => counter(await metrics(workerMetricsUrl), 'identity_events_consumed_total', 'result='), { timeout: 20_000 })
    .toBeGreaterThan(0)

  // Los logs de ambos procesos son JSON estructurado, línea por línea.
  for (const service of ['api', 'worker'] as const) {
    const lines = serviceLogs(service)
    expect(lines.length, `${service} no emitió logs`).toBeGreaterThan(0)
    const plain = lines.filter((line) => !isJsonObject(line))
    expect(plain, `líneas de ${service} que no son JSON`).toEqual([])
  }

  // El panel de Grafana presenta las cuatro señales que pide el requisito.
  const panels = (JSON.parse(readFileSync(dashboardPath, 'utf8')) as { panels: { title: string; targets?: { expr: string }[] }[] }).panels
  const exprs = panels.flatMap((panel) => (panel.targets ?? []).map((target) => target.expr)).join('\n')
  expect(exprs).toContain('identity_login_attempts_total')
  expect(exprs).toContain('identity_http_request_duration_seconds_bucket')
  expect(exprs).toContain('notifications.dlq')
  expect(exprs).toContain('identity_refresh_reuse_detected_total')
})

test('RNF-012 Los logs no contienen contraseñas, tokens ni códigos tras un inicio de sesión', async ({ playwright }) => {
  const email = await createActiveUser(await seedAdmin())
  const context = await playwright.request.newContext({ baseURL: hubUrl })
  const existing = await mailIds(email)
  const since = new Date()

  // Un intento con contraseña incorrecta y otro completo (contraseña, código por correo y renovación).
  const wrongPassword = `incorrecta-${strongPassword}`
  expect((await contextApi(context, 'POST', '/api/v1/auth/login', { data: loginBody(email, wrongPassword) })).status).toBe(401)
  const login = await contextApi<{ mfaToken: string }>(context, 'POST', '/api/v1/auth/login', { data: loginBody(email, strongPassword) })
  expect(login.status).toBe(202)
  const code = extractCode((await waitForMail(email, 'sign-in code', since, existing)).text)
  const verified = await contextApi<{ accessToken: string }>(context, 'POST', '/api/v1/auth/mfa/verify', {
    data: { mfaToken: login.body.mfaToken, code },
  })
  expect(verified.status).toBe(200)
  const refreshToken = (await context.storageState()).cookies.find((cookie) => cookie.name === 'refresh_token')?.value ?? ''
  expect(refreshToken).not.toBe('')
  const renewed = await contextApi<{ accessToken: string }>(context, 'POST', '/api/v1/auth/refresh')
  expect(renewed.status).toBe(200)

  const secrets = [
    strongPassword,
    wrongPassword,
    login.body.mfaToken,
    verified.body.accessToken,
    renewed.body.accessToken,
    refreshToken,
    `"${code}"`,
  ]
  // Garantía de que el detector funciona: sin esto, un cambio en el formato de logs lo dejaría ciego.
  expect(linesLeaking([`{"msg":"prueba ${strongPassword}"}`], secrets)).toHaveLength(1)

  for (const service of ['api', 'worker'] as const) {
    const logs = serviceLogs(service)
    expect(logs.length, `${service} no emitió logs`).toBeGreaterThan(0)
    expect(linesLeaking(logs, secrets), `${service} registró un secreto`).toEqual([])
  }
  await context.dispose()
})
