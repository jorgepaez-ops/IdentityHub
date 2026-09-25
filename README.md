# Identity Hub

Plataforma de gestión de identidad contenerizada, construida como vehículo de un
**pipeline de DevSecOps de ciclo completo**.

Proyecto final de materia · Licencia Apache-2.0

---

> ### ⚠️ Este repositorio contiene código inseguro a propósito
>
> El tag `v0.0.0-vuln-baseline` es una **línea base deliberadamente vulnerable**:
> imágenes base antiguas, dependencias con CVE publicado, secretos incrustados,
> SQL concatenado, contenedores como root y cabeceras de seguridad ausentes.
>
> Existe para demostrar que cada gate del pipeline detecta lo que dice detectar.
> Un pipeline cuyos controles nunca han fallado es indistinguible de uno mal
> configurado que deja pasar todo.
>
> **Nunca se despliega.** La rama `main` es la versión remediada.
> Ver [`specs/adr/0007`](specs/adr/0007-linea-base-vulnerable-deliberada.md).

---

## Qué es esto

El requisito del curso pide *"un pipeline de DevSecOps de ciclo completo para una
aplicación contenerizada de libre uso"*. El entregable principal **es el
pipeline**; la aplicación es lo que le da algo real que analizar.

Se eligió el dominio de identidad porque concentra riesgo genuino —criptografía,
manejo de secretos, control de acceso, auditoría— y por tanto produce hallazgos
auténticos en los escáneres. Un pipeline sobre un "hola mundo" no demuestra nada.

## Metodología: Spec-Driven Development

**Ningún endpoint, evento o tabla existe si no está antes en `specs/`.** Los
specs no son documentación posterior: son artefactos de los que se genera código
y contra los que CI valida.

| Fuente de verdad | Genera |
|---|---|
| `specs/03-api/openapi.yaml` | Tipos e interfaz del servidor Go · tipos del cliente TypeScript |
| `specs/04-events/asyncapi.yaml` | Contrato de mensajería; validado en pruebas |
| `db/migrations/*.sql` | Capa de acceso a datos tipada (sqlc) |
| `specs/06-acceptance/*.feature` | Escenarios de Playwright |
| Todo lo anterior | `specs/07-traceability.md`, generado por `scripts/traceability.py` |

El job `spec-drift` regenera en CI y falla si el resultado difiere de lo
commiteado. Eso convierte "los specs mandan" en algo verificable y no en una
norma que se respeta por costumbre.

## Arquitectura

```
navegador ──► web (Nginx + React) ──► api (Go, el IdP) ──┬─► db (PostgreSQL)
                                                          └─► broker (RabbitMQ)
                                                                    │
                                                              worker (Go) ──► mailpit (SMTP)

observabilidad: prometheus · loki + alloy · grafana
```

- **api** — proveedor de identidad: Argon2id, JWT Ed25519 con JWKS público,
  refresh rotativo con detección de reuso, TOTP, RBAC y auditoría append-only.
- **worker** — consume eventos del broker y entrega notificaciones por SMTP, con
  reintentos, *dead-letter queue* e idempotencia por `eventId`.
- **web** — SPA en React servida por Nginx, que además aloja las cabeceras de
  seguridad y el rate limiting de los endpoints de autenticación.

## Puesta en marcha

Requisitos: solo Docker.

```bash
git clone <repo> && cd ProyectoFinalMateria
cp .env.example .env
# Rellena los secretos de .env; genera JWT_SIGNING_KEY con: openssl rand -base64 32
make up
```

| Servicio | URL | Credenciales |
|---|---|---|
| Aplicación | http://localhost:8080 | — |
| API | http://localhost:8081/healthz | — |
| RabbitMQ | http://localhost:15672 | `identity` / `rabbit_admin_2024` |
| Mailpit | http://localhost:8025 | — |
| Grafana | http://localhost:3000 (con `make up-obs`) | `admin` / `admin` |

`make help` lista todo lo disponible.

## El pipeline

| # | Gate | Herramienta | Rompe la build por |
|---|---|---|---|
| 1 | Deriva de specs | validador OpenAPI, `traceability.py` | código divergente del contrato |
| 2 | Lint y tipos | golangci-lint (+gosec), ESLint, tsc, Hadolint | cualquier hallazgo |
| 3 | Secretos | Gitleaks (historial completo) | cualquier secreto |
| 4 | SAST | CodeQL, Semgrep | severidad ERROR |
| 5 | Dependencias | govulncheck, npm audit, osv-scanner | vulnerabilidad con parche |
| 6 | Pruebas | go test, Vitest | fallo o cobertura < 70 % |
| 8 | Configuración | Trivy config | misconfiguración HIGH |
| 9-10 | Imágenes | buildx + Trivy image | CVE HIGH/CRITICAL corregible |

Pendientes de la semana 3: SBOM (Syft), firma (Cosign *keyless*), publicación en
GHCR, E2E con Playwright y DAST con OWASP ZAP.

Los mismos controles corren en local con `make scan`, con idénticas versiones de
las herramientas. Si el pipeline y la máquina de quien desarrolla divergen, el
pipeline deja de ser una red de seguridad y pasa a ser una sorpresa.

## Estructura

```
specs/          fuente de verdad: requisitos, contratos, amenazas, aceptación, ADR
backend/        módulo Go: cmd/api, cmd/worker, internal/… (detalle abajo)
frontend/       React + TypeScript + Nginx
db/migrations/  esquema versionado
deploy/         docker-compose y configuración de observabilidad
e2e/            pruebas de extremo a extremo (Playwright)
security/       fichas de hallazgos, evidencia de escaneos, .trivyignore
scripts/        generadores y utilidades del pipeline
docs/           arquitectura, runbook e informe de seguridad
```

### `backend/`

Dos binarios (`cmd/`) y los paquetes de dominio que comparten (`internal/`),
organizados en capas: `internal/api` (HTTP) llama a los casos de uso de
`internal/auth/*`, que a su vez dependen de `internal/store` (datos) y
`internal/events` (mensajería) a través de interfaces — nunca al revés.

| Paquete | Qué hace |
|---|---|
| `cmd/api` | Binario de la API: carga la configuración, conecta Postgres/RabbitMQ, compone todos los servicios de `internal/auth/*` y monta el router HTTP generado. |
| `cmd/worker` | Binario del worker: consume la cola `notifications` de RabbitMQ y entrega notificaciones por SMTP, con reintentos, *dead-letter queue* e idempotencia por `eventId`. |
| `internal/api` | Adaptadores HTTP: implementa el `ServerInterface` generado desde `specs/03-api/openapi.yaml`, más `RequireAuth`/`RequireRole` (RBAC) y `ClientIP` (resolución de IP confiable, T6). |
| `internal/auth/registration` | Alta de cuentas (RF-001): hashea la contraseña, genera el token de verificación de correo y publica `user.registered`. |
| `internal/auth/verification` | Consume el token de verificación de correo y activa la cuenta (RF-002). |
| `internal/auth/login` | Autenticación por contraseña (RF-003): emite el par de tokens, y aplica el bloqueo por fuerza bruta por cuenta y por IP (RF-017). |
| `internal/auth/token` | Emisión y validación de JWT Ed25519 y el endpoint `/.well-known/jwks.json` (RF-004). |
| `internal/auth/refresh` | Rotación del refresh token con detección de reuso y revocación de toda la familia (RF-005/RF-006). |
| `internal/auth/logout` | Revoca el refresh token presentado (RF-007). |
| `internal/auth/password` | Hashing Argon2id, verificación en tiempo constante y contraseña señuelo para no revelar si un correo existe (AM-004). |
| `internal/auth/admin` | Administración de usuarios (RF-010): listar/editar/deshabilitar, con protección transaccional para que nunca quede el sistema sin un admin activo. |
| `internal/auth/auditlog` | Lectura paginada del registro de auditoría para el panel de administración (RF-011); separado de `internal/audit` para evitar un ciclo de imports con `internal/api`. |
| `internal/audit` | Escritor *append-only* del registro de auditoría: valida la acción y filtra cualquier campo sensible del metadata antes de insertar. |
| `internal/store` | Adaptadores de datos: envuelven el código generado por sqlc y traducen entre los tipos de cada paquete de dominio y las columnas reales. |
| `internal/events` | Contrato de mensajería (sobres de evento, nombres de canal, topología de RabbitMQ) y el cliente que declara exchanges/colas de forma idempotente. |
| `internal/notify` | Renderiza asunto y cuerpo de cada correo a partir del payload del evento, leyendo solo una lista explícita de campos (nunca el payload crudo). |
| `internal/config` | Carga y valida toda la configuración desde variables de entorno; un secreto ausente es un error de arranque, no un valor por defecto (RNF-003). |
| `internal/observability` | Logger estructurado (`slog`) compartido por ambos binarios. |
| `internal/testdb` | Aprovisiona una base PostgreSQL descartable por prueba de integración (crea, migra y borra). |

## Documentos que conviene leer primero

1. [`specs/00-vision.md`](specs/00-vision.md) — qué es esto y qué no.
2. [`specs/05-security/threat-model.md`](specs/05-security/threat-model.md) —
   STRIDE por componente, y qué gate protege cada mitigación.
3. [`specs/adr/`](specs/adr/) — las decisiones y sus contrapartidas.
4. [`specs/07-traceability.md`](specs/07-traceability.md) — qué está realmente
   verificado (generado, no escrito a mano).
5. [`docs/guia-desarrollo.md`](docs/guia-desarrollo.md) — cómo correr el
   backend fuera de Docker, qué hace `make gen` por dentro, comandos de
   prueba de Go y configuración de GoLand.

## Estado

Semana 2 de 4: núcleo del IdP completo (registro, verificación, login, JWT,
refresh rotativo, logout, RBAC, administración de usuarios, auditoría y
bloqueo por fuerza bruta), compuesto de punta a punta en `cmd/api` y
verificado con un flujo real completo (Postgres/RabbitMQ/Mailpit reales, no
solo pruebas aisladas). Ver `specs/07-traceability.md` para el detalle
requisito por requisito.

**CI sigue en rojo a propósito.** El repositorio está en la línea base
vulnerable y los gates encuentran lo que se sembró para ellos. Pasará a verde
durante la remediación de la semana 3 (Fase 3), y ese cambio de color es el
entregable. `make up` todavía no construye las imágenes de `api`/`worker`: usan
la línea base deliberada (Debian 11 / Go 1.22), que no cumple el `go 1.25` que
exige el módulo desde T6 — se remedia junto con esas vulnerabilidades, no antes.
