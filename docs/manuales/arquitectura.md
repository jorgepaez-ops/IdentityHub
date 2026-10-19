# Manual de arquitectura

Identity Hub es un proveedor de identidad autocontenido y, sobre todo, el vehículo para demostrar un pipeline DevSecOps de extremo a extremo. Gestiona cuentas, autenticación, autorización y auditoría; no pretende ser una plataforma de identidad de alta disponibilidad. La fuente de verdad funcional es [`specs/00-vision.md`](../../specs/00-vision.md).

## Alcance

Incluye registro e invitaciones por correo, contraseñas Argon2id, JWT Ed25519 y JWKS, refresh tokens rotativos, MFA por correo, roles y auditoría inmutable. Incluye también el aislamiento por empresa (cada aplicación y cada usuario pertenecen a una empresa; [ADR 0015](../../specs/adr/0015-empresas-y-aislamiento-de-roles.md)). Quedan fuera la federación con proveedores externos, la multi-tenencia completa (administrador y auditoría por empresa), alta disponibilidad, escalado horizontal, TOTP y WebAuthn. El objetivo de despliegue actual es un solo host con Docker Compose.

## Componentes y responsabilidades

| Componente | Responsabilidad | Evidencia principal |
|---|---|---|
| `web` | Sirve las SPA del Hub, Contabilidad e Inventario con Nginx; aplica cabeceras, límite de autenticación y proxy HTTP. | [`frontend/nginx/default.conf`](../../frontend/nginx/default.conf) |
| `api` | Expone la API HTTP, aplica autenticación/autorización y coordina dominio, persistencia y eventos. | [`backend/cmd/api/main.go`](../../backend/cmd/api/main.go), [`backend/internal/api/`](../../backend/internal/api/) |
| `worker` | Consume eventos y entrega notificaciones por SMTP; también publica métricas y salud. | [`backend/cmd/worker/main.go`](../../backend/cmd/worker/main.go) |
| PostgreSQL | Conserva usuarios, sesiones, auditoría y demás estado transaccional. | [`db/migrations/`](../../db/migrations/) |
| RabbitMQ | Transporta eventos persistentes desde la API al worker. | [`backend/internal/events/`](../../backend/internal/events/) |
| Mailpit | Captura el SMTP local para desarrollo y pruebas, con STARTTLS y autenticación obligatorios; no es el broker. | [`deploy/docker-compose.yml`](../../deploy/docker-compose.yml) |
| Prometheus, Loki, Alloy y Grafana | Recogen métricas y logs, y los presentan cuando se activa el perfil de observabilidad. | [`deploy/observability/`](../../deploy/observability/) |
| Falco | Detección de comportamiento anómalo en tiempo de ejecución (driver modern eBPF) con cuatro reglas propias sobre `api`, `worker` y `web`; sus alertas llegan a Loki por Alloy y a Grafana ([runbook](../runbooks/falco-alerta.md)). Solo en el perfil de observabilidad. | [`deploy/observability/falco/`](../../deploy/observability/falco/) |

## Comunicación entre dominios

| Flujo | Protocolo | Límite y propósito |
|---|---|---|
| Navegador ↔ `web` ↔ `api` | HTTP | Nginx entrega SPA y proxifica API; `/metrics` no se expone por Nginx. |
| `api` ↔ PostgreSQL | SQL | Persistencia transaccional mediante consultas generadas por `sqlc`. |
| `api` → RabbitMQ → `worker` | AMQP | Eventos persistentes; la publicación usa *publisher confirms* sin outbox transaccional. |
| `worker` → Mailpit | SMTP con STARTTLS y autenticación | Invitaciones, códigos MFA y avisos de seguridad. El worker exige STARTTLS (TLS 1.2 como mínimo) antes de autenticarse y no envía nada si el servidor no lo ofrece; en local, Mailpit usa la CA y las credenciales que genera `make certs`. |
| Hub ↔ Contabilidad e Inventario | OAuth 2.0 Authorization Code con PKCE sobre HTTP local | Son tres orígenes (`identityhub.localhost`, `contabilidad.localhost` e `inventario.localhost`); cada aplicación canjea código y verifica JWT con JWKS. |

La implementación OAuth es mínima: los clientes (Contabilidad e Inventario) son filas de la tabla `applications` que la API lee al arrancar, el `redirect_uri` se compara exactamente contra el del cliente solicitado y PKCE S256 sustituye un secreto de cliente en la SPA. Cada cliente pertenece a una empresa: `/oauth/authorize` rechaza con `access_denied` a quien sea de otra empresa o no tenga rol en la aplicación, y el token lleva solo los roles de esa aplicación. No hay registro dinámico de clientes (se agregan por migración) ni federación externa; la guía de [integración de terceros](integracion-terceros.md) lo detalla.

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
| [0011](../../specs/adr/0011-docker-hub-como-registry.md) | Sustituye GHCR por Docker Hub para publicar imágenes versionadas, SBOM y firma; implementado en `release.yml` (tarea C3), validado con el tag `v0.9.0`. |
| [0012](../../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md) | Arquitectura de producción de referencia en AWS (ECS Fargate, ALB, RDS, RabbitMQ en ECS, Secrets Manager) en Terraform, validada con LocalStack; la implementación llega en T8-T10. |
| [0013](../../specs/adr/0013-roles-y-permisos-configurables-por-aplicacion.md) | Roles y permisos configurables por aplicación desde el Hub, sin límite de roles de aplicación. |
| [0014](../../specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md) | Claves de servicio `ihk_<publicId>_<secreto>` para integraciones máquina (RF-018): solo se guarda el SHA-256 del secreto, alcance `audit-log:read` y un único 401 para toda causa ([manual](claves-de-servicio.md)). |
| [0015](../../specs/adr/0015-empresas-y-aislamiento-de-roles.md) | Empresas ligeras: cada aplicación y usuario pertenecen a una empresa, los roles no se mezclan, la consola trabaja una empresa a la vez y el Hub rechaza el acceso entre empresas. |

El README y el ADR 0001 documentan la elección técnica de Go y sus razones para el proyecto. El enunciado deja al equipo la libertad de elegir el stack siempre que se cumplan los requisitos de DevSecOps y de gestor de identidad; Python y Node.js eran sugerencias del material del curso, no una restricción, así que la elección no requiere autorización adicional.

## Diagramas y modelo de amenazas

- [Casos de uso](../diagramas/uml/casos-de-uso.md)
- [Componentes](../diagramas/uml/componentes.md)
- [Despliegue local](../diagramas/uml/despliegue.md)
- [Secuencia de login con MFA](../diagramas/uml/secuencia-login-mfa.md)
- [Secuencia del SSO con PKCE](../diagramas/uml/secuencia-sso-pkce.md)
- [Estados de la cuenta de usuario](../diagramas/uml/estados-usuario.md)
- Vistas de conjunto en HTML con SVG en línea, sin JavaScript y con el tema del sistema: [arquitectura](../diagramas/01-arquitectura.html), [flujo SSO](../diagramas/02-flujo-sso.html) y [ciclo de vida del usuario](../diagramas/03-ciclo-de-vida-usuario.html)
- [Modelo Threat Dragon: DFD nivel 0 y 1](../threat-model/README.md)
