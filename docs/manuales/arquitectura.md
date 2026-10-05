# Manual de arquitectura

Identity Hub es un proveedor de identidad autocontenido y, sobre todo, el vehículo para demostrar un pipeline DevSecOps de extremo a extremo. Gestiona cuentas, autenticación, autorización y auditoría; no pretende ser una plataforma de identidad de alta disponibilidad. La fuente de verdad funcional es [`specs/00-vision.md`](../../specs/00-vision.md).

## Alcance

Incluye registro e invitaciones por correo, contraseñas Argon2id, JWT Ed25519 y JWKS, refresh tokens rotativos, MFA por correo, roles y auditoría inmutable. Quedan fuera la federación con proveedores externos, multi-tenancy efectivo, alta disponibilidad, escalado horizontal, TOTP y WebAuthn. El objetivo de despliegue actual es un solo host con Docker Compose.

## Componentes y responsabilidades

| Componente | Responsabilidad | Evidencia principal |
|---|---|---|
| `web` | Sirve las SPA del Hub y Contabilidad con Nginx; aplica cabeceras, límite de autenticación y proxy HTTP. | [`frontend/nginx/default.conf`](../../frontend/nginx/default.conf) |
| `api` | Expone la API HTTP, aplica autenticación/autorización y coordina dominio, persistencia y eventos. | [`backend/cmd/api/main.go`](../../backend/cmd/api/main.go), [`backend/internal/api/`](../../backend/internal/api/) |
| `worker` | Consume eventos y entrega notificaciones por SMTP; también publica métricas y salud. | [`backend/cmd/worker/main.go`](../../backend/cmd/worker/main.go) |
| PostgreSQL | Conserva usuarios, sesiones, auditoría y demás estado transaccional. | [`db/migrations/`](../../db/migrations/) |
| RabbitMQ | Transporta eventos persistentes desde la API al worker. | [`backend/internal/events/`](../../backend/internal/events/) |
| Mailpit | Captura el SMTP local para desarrollo y pruebas; no es el broker. | [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) |
| Prometheus, Loki, Alloy y Grafana | Recogen métricas y logs, y los presentan cuando se activa el perfil de observabilidad. | [`deploy/observability/`](../../deploy/observability/) |

## Comunicación entre dominios

| Flujo | Protocolo | Límite y propósito |
|---|---|---|
| Navegador ↔ `web` ↔ `api` | HTTP | Nginx entrega SPA y proxifica API; `/metrics` no se expone por Nginx. |
| `api` ↔ PostgreSQL | SQL | Persistencia transaccional mediante consultas generadas por `sqlc`. |
| `api` → RabbitMQ → `worker` | AMQP | Eventos persistentes; la publicación usa *publisher confirms* sin outbox transaccional. |
| `worker` → Mailpit | SMTP | Invitaciones, códigos MFA y avisos de seguridad en el entorno local. |
| Hub ↔ Contabilidad | OAuth 2.0 Authorization Code con PKCE sobre HTTP local | Son dos orígenes (`identityhub.localhost` y `contabilidad.localhost`); la aplicación canjea código y verifica JWT con JWKS. |

La implementación OAuth es mínima: Contabilidad es el cliente configurado, el `redirect_uri` se compara exactamente y PKCE S256 sustituye un secreto de cliente en la SPA. No hay registro dinámico de clientes ni federación externa.

## Panorama de datos

- **Identidad y sesiones:** usuarios, roles, invitaciones, restablecimientos, sesiones del Hub, códigos de autorización y familias de refresh viven en PostgreSQL.
- **Credenciales:** las contraseñas se guardan como Argon2id; los refresh tokens, códigos de autorización y secretos de desafío se almacenan como hashes, no en claro.
- **Auditoría:** los eventos de seguridad se agregan al audit log, protegido contra actualización y borrado para el rol de aplicación.
- **Mensajería:** RabbitMQ contiene eventos de notificación; Mailpit solo visualiza el correo local recibido.
- **Telemetría:** Prometheus consulta métricas; Alloy obtiene logs de contenedores para Loki; Grafana consulta ambas fuentes.

## Patrones sustentados

| Patrón | Cómo se aplica | Sustento |
|---|---|---|
| Contrato primero | OpenAPI 3.0.3 genera la interfaz Go y los tipos TypeScript; no se edita el código generado. | [ADR 0003](../../specs/adr/0003-openapi-303-como-fuente-de-verdad.md) |
| API y worker separados | Un módulo Go produce `cmd/api` y `cmd/worker`, desacoplando HTTP de la entrega de correo. | [ADR 0001](../../specs/adr/0001-stack-y-contenerizacion.md) |
| Mensajería asíncrona con confirmación | La API confirma al broker antes de responder; el outbox es trabajo futuro deliberado. | [ADR 0006](../../specs/adr/0006-publicacion-directa-sin-outbox.md) |
| Token opaco y rotación por familia | El refresh se revoca y rota por familia para detectar reuso. | [ADR 0005](../../specs/adr/0005-refresh-tokens-rotativos-con-familia.md) |

## Decisiones de arquitectura

| ADR | Resumen |
|---|---|
| [0001](../../specs/adr/0001-stack-y-contenerizacion.md) | Define React/TypeScript/Nginx, Go con API y worker, PostgreSQL, RabbitMQ, Mailpit y Docker Compose; Go reduce superficie de imagen y aporta criptografía y `govulncheck`. |
| [0002](../../specs/adr/0002-idp-propio-en-lugar-de-keycloak.md) | Implementa el IdP en Go para que el pipeline analice riesgos reales del dominio de identidad. |
| [0003](../../specs/adr/0003-openapi-303-como-fuente-de-verdad.md) | Fija OpenAPI 3.0.3 como contrato generable y vinculante para backend y frontend. |
| [0004](../../specs/adr/0004-argon2id-para-contrasenas.md) | Usa Argon2id calibrado y permite rehash cuando cambian sus parámetros. |
| [0005](../../specs/adr/0005-refresh-tokens-rotativos-con-familia.md) | Define refresh opaco, almacenado como hash, rotativo y revocable por familia ante reuso. |
| [0006](../../specs/adr/0006-publicacion-directa-sin-outbox.md) | Publica directamente al broker con confirmación y deja el outbox transaccional fuera de esta entrega. |
| [0007](../../specs/adr/0007-linea-base-vulnerable-deliberada.md) | Conserva una línea base vulnerable para demostrar detección y remediación verificables. |
| [0008](../../specs/adr/0008-github-actions-y-ghcr.md) | Elige GitHub Actions, SARIF y Cosign keyless; su decisión de registry fue sustituida parcialmente. |
| [0009](../../specs/adr/0009-sso-entre-dominios-con-authorization-code-y-pkce.md) | Implementa SSO entre Hub y Contabilidad mediante authorization code con PKCE. |
| [0010](../../specs/adr/0010-mfa-por-codigo-enviado-por-correo.md) | Sustituye TOTP por MFA obligatorio con código de un solo uso enviado al correo verificado. |
| [0011](../../specs/adr/0011-docker-hub-como-registry.md) | Sustituye GHCR por Docker Hub para publicar imágenes versionadas, SBOM y firma; la implementación llega en T17. |
| [0012](../../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md) | Arquitectura de producción de referencia en AWS (ECS Fargate, ALB, RDS, RabbitMQ en ECS, Secrets Manager) en Terraform, validada con LocalStack; la implementación llega en T8-T10. |

El README documenta la elección técnica de Go y sus razones para el proyecto. Sin embargo, `ANALISIS-REQUISITOS.md` identifica Go como una desviación frente a Python/Node y pide validación escrita del docente; la pregunta Q4 de la semana 4 permanece abierta y prevé registrar esa evidencia en un ADR si existe. El repositorio no identifica un aprobador ni aporta esa validación escrita.

## Diagramas y modelo de amenazas

- [Casos de uso](../diagramas/uml/casos-de-uso.md)
- [Componentes](../diagramas/uml/componentes.md)
- [Despliegue local](../diagramas/uml/despliegue.md)
- [Secuencia de login con MFA](../diagramas/uml/secuencia-login-mfa.md)
- [Modelo Threat Dragon: DFD nivel 0 y 1](../threat-model/README.md)
