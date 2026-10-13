// RNF-010 — Latencia: el p95 de los endpoints de autenticación se mantiene por debajo de
// 300 ms en local, excluido el coste deliberado de Argon2id.
//
// Qué se mide (y por qué):
//   · POST /api/v1/auth/refresh   renovación de sesión con rotación de la cookie refresh
//   · GET  /api/v1/me             validación del JWT de acceso en cada petición autenticada
//   · GET  /.well-known/jwks.json clave pública con la que los clientes validan el JWT
// El inicio de sesión (POST /api/v1/auth/login) NO se mide: verifica una contraseña con
// Argon2id, cuyo coste (~250 ms) es deliberado y el requisito lo excluye. Solo se hace una
// vez en setup() para obtener la sesión con la que corren los demás endpoints.
//
// Nginx limita /api/v1/auth/ a 5 r/s por IP (burst 5, AM-017): el escenario de refresh va a
// ~3 r/s a propósito para medir la latencia del endpoint y no la del limitador. Ningún login
// falla, así que no hay bloqueo por fuerza bruta (RF-017) que esquivar.
//
// Los umbrales hacen fallar la ejecución (k6 sale con código distinto de cero).
import http from 'k6/http'
import { check, fail, sleep } from 'k6'

const HUB = __ENV.HUB_URL || 'http://host.docker.internal:8080'
const MAILPIT = __ENV.MAILPIT_URL || 'http://host.docker.internal:8025'
const EMAIL = __ENV.LOAD_EMAIL
const PW = __ENV.LOAD_PW
const INVITATION = __ENV.LOAD_INVITATION_TOKEN
const P95_MS = Number(__ENV.P95_MS || 300) // RNF-010
const DURATION = __ENV.DURATION || '20s'

// Espera del código MFA en Mailpit: 40 intentos x 0,5 s = 20 s máximo, el mismo margen que los 20 s de
// espera de correo de las pruebas E2E (el worker publica y envía el correo de forma asíncrona).
const MAIL_ATTEMPTS = 40
const MAIL_POLL_SECONDS = 0.5
// `since` se toma en el reloj de k6 (contenedor) y `Created` viene del reloj de Mailpit (otro proceso, a
// veces otra máquina virtual): se acepta un correo creado hasta 2 s antes de `since` para absorber esa
// diferencia de relojes. No admite correos viejos: la cuenta es nueva por ejecución y solo recibe este código.
const MAIL_CLOCK_SKEW_MS = 2000

const json = { headers: { 'Content-Type': 'application/json' } }

export const options = {
  scenarios: {
    jwks: {
      executor: 'constant-arrival-rate',
      rate: 50,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 10,
      maxVUs: 50,
      exec: 'jwks',
    },
    me: {
      executor: 'constant-arrival-rate',
      rate: 50,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 10,
      maxVUs: 50,
      exec: 'me',
    },
    // Una sola sesión que rota su refresh token: la cookie vigente es la de la última respuesta.
    refresh: {
      executor: 'constant-vus',
      vus: 1,
      duration: DURATION,
      exec: 'refresh',
    },
  },
  thresholds: {
    'http_req_duration{endpoint:jwks}': [`p(95)<${P95_MS}`],
    'http_req_duration{endpoint:me}': [`p(95)<${P95_MS}`],
    'http_req_duration{endpoint:refresh}': [`p(95)<${P95_MS}`],
    // Una respuesta de error rápida no puede hacer pasar la prueba.
    'checks{endpoint:jwks}': ['rate==1'],
    'checks{endpoint:me}': ['rate==1'],
    'checks{endpoint:refresh}': ['rate==1'],
  },
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
}

function mailCode(since) {
  for (let attempt = 0; attempt < MAIL_ATTEMPTS; attempt++) {
    const found = http.get(`${MAILPIT}/api/v1/search?query=${encodeURIComponent(`to:${EMAIL}`)}`)
    const message = (found.json('messages') || []).find(
      (m) => m.Subject.includes('sign-in code') && new Date(m.Created).getTime() >= since - MAIL_CLOCK_SKEW_MS,
    )
    if (message) {
      const match = /\b(\d{6})\b/.exec(http.get(`${MAILPIT}/api/v1/message/${message.ID}`).json('Text'))
      if (match) return match[1]
    }
    sleep(MAIL_POLL_SECONDS)
  }
  return fail('no llegó el código MFA')
}

// Login completo (con Argon2id) fuera de la medición.
export function setup() {
  if (!EMAIL || !PW || !INVITATION) fail('faltan LOAD_EMAIL, LOAD_PW o LOAD_INVITATION_TOKEN (usa scripts/load-test.sh)')
  const accepted = http.post(`${HUB}/api/v1/auth/invitations/accept`, JSON.stringify({ token: INVITATION, password: PW }), json)
  if (accepted.status !== 204) fail(`aceptar la invitación devolvió ${accepted.status}`)
  const since = Date.now()
  const login = http.post(`${HUB}/api/v1/auth/login`, JSON.stringify({ email: EMAIL, password: PW }), json)
  if (login.status !== 202) fail(`login devolvió ${login.status}`)
  const verify = http.post(
    `${HUB}/api/v1/auth/mfa/verify`,
    JSON.stringify({ mfaToken: login.json('mfaToken'), code: mailCode(since) }),
    json,
  )
  if (verify.status !== 200) fail(`mfa/verify devolvió ${verify.status}`)
  const cookie = verify.cookies.refresh_token
  if (!cookie || !cookie.length) fail('mfa/verify no devolvió la cookie refresh_token')
  return { accessToken: verify.json('accessToken'), refreshToken: cookie[0].value }
}

export function jwks() {
  const res = http.get(`${HUB}/.well-known/jwks.json`, { tags: { endpoint: 'jwks' } })
  check(res, { 'jwks 200 con claves': (r) => r.status === 200 && r.json('keys').length > 0 }, { endpoint: 'jwks' })
}

export function me(data) {
  const res = http.get(`${HUB}/api/v1/me`, {
    headers: { Authorization: `Bearer ${data.accessToken}` },
    tags: { endpoint: 'me' },
  })
  check(res, { 'me 200 con el correo': (r) => r.status === 200 && r.json('email') === EMAIL }, { endpoint: 'me' })
}

// Estado por VU: el scenario refresh tiene un solo VU, así que la cadena de rotación es única.
let current

export function refresh(data) {
  current = current || data.refreshToken
  const res = http.post(`${HUB}/api/v1/auth/refresh`, null, {
    headers: { Cookie: `refresh_token=${current}` },
    tags: { endpoint: 'refresh' },
  })
  const ok = check(res, { 'refresh 200 con token nuevo': (r) => r.status === 200 && !!r.json('accessToken') }, { endpoint: 'refresh' })
  const rotated = res.cookies.refresh_token
  if (ok && rotated && rotated.length) current = rotated[0].value
  sleep(0.3)
}
