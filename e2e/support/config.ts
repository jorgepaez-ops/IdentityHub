export const hubUrl = process.env.E2E_HUB_URL ?? 'http://identityhub.localhost:8080'
export const contabilidadUrl = process.env.E2E_CONTABILIDAD_URL ?? 'http://contabilidad.localhost:8080'
export const mailpitUrl = process.env.E2E_MAILPIT_URL ?? 'http://localhost:8025'
// Address that serves both virtual hosts; requests set the Host header explicitly.
export const nginxAddress = process.env.E2E_NGINX_URL ?? 'http://127.0.0.1:8080'
