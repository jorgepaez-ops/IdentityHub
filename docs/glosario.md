# Glosario

Los términos técnicos de esta documentación, explicados en simple. Cada uno dice dónde aparece en Identity Hub. Vuelva al [índice de `docs/`](README.md) o al [README principal](../README.md) cuando termine.

**Contenido:** [Identidad y acceso](#identidad-y-acceso) · [Seguridad en el pipeline](#seguridad-en-el-pipeline) · [Análisis de amenazas y decisiones](#análisis-de-amenazas-y-decisiones) · [Infraestructura y operación](#infraestructura-y-operación)

## Identidad y acceso

| Término | Qué significa | Dónde aparece en este proyecto |
|---|---|---|
| **IdP** (proveedor de identidad) | El sistema que guarda las cuentas y comprueba quién es cada persona, para que las demás aplicaciones no tengan que hacerlo. | Identity Hub es un IdP. Por qué se escribió uno propio: [ADR 0002](../specs/adr/0002-idp-propio-en-lugar-de-keycloak.md). |
| **SSO** (inicio de sesión único) | Iniciar sesión una sola vez y entrar a varias aplicaciones sin repetirlo. | El Hub y Contabilidad: [flujo SSO](diagramas/02-flujo-sso.html) y [ADR 0009](../specs/adr/0009-sso-entre-dominios-con-authorization-code-y-pkce.md). |
| **OAuth 2.0** | Un estándar para que una aplicación obtenga acceso en nombre de una persona sin conocer su contraseña. | El Hub implementa el flujo *Authorization Code*: [guía de integración](manuales/integracion-terceros.md). |
| **Authorization code** (código de autorización) | Un ticket de un solo uso y muy corto (1 minuto) que el Hub entrega por el navegador y que la aplicación cambia por el token. | [Guía de integración](manuales/integracion-terceros.md), sección «Flujo paso a paso». |
| **PKCE** | Una comprobación extra para que, si alguien roba el código de autorización, no pueda canjearlo: solo quien inició el flujo conoce el secreto que lo desbloquea. | [Secuencia SSO con PKCE](diagramas/uml/secuencia-sso-pkce.md) y [ADR 0009](../specs/adr/0009-sso-entre-dominios-con-authorization-code-y-pkce.md). |
| **OIDC** (OpenID Connect) | Una capa estándar sobre OAuth 2.0 que añade un formato común para identificar a la persona. **Identity Hub no la implementa**: usa OAuth 2.0 con PKCE en versión mínima. | Los límites están en la sección 9 de la [guía de integración](manuales/integracion-terceros.md); implementar OIDC es trabajo futuro. |
| **JWT** (token web JSON) | Un token firmado que dice quién es la persona (`sub`), para qué aplicación sirve (`aud`), qué roles y permisos tiene y cuándo vence. Cualquiera puede leerlo, pero nadie puede alterarlo sin que la firma falle. | El *access token* del Hub es un JWT firmado con Ed25519 y vive 15 minutos: [guía de integración](manuales/integracion-terceros.md), sección «Contenido del token». |
| **JWKS** | La lista pública de claves con las que una aplicación comprueba la firma de los tokens del Hub. | Se publica en `/.well-known/jwks.json` ([OpenAPI](../specs/03-api/openapi.yaml)). |
| **Claim** | Un dato dentro de un token, como el usuario o sus roles. | Los roles se leen de la base de datos en cada petición, no del claim que envía el cliente: amenaza AM-007 en el [modelo de amenazas](../specs/05-security/threat-model.md). |
| **Refresh token** (token de renovación) | Una credencial de larga duración que permite pedir un nuevo access token sin volver a escribir la contraseña. | En el Hub rota en cada uso y se detecta su reutilización: [ADR 0005](../specs/adr/0005-refresh-tokens-rotativos-con-familia.md). |
| **MFA** (segundo factor) | Pedir algo más que la contraseña. | En el Hub es un código enviado por correo (visible en Mailpit en local): [ADR 0010](../specs/adr/0010-mfa-por-codigo-enviado-por-correo.md). |
| **Argon2id** | Un algoritmo diseñado para guardar contraseñas de forma que sean muy costosas de adivinar si se filtra la base de datos. | Las contraseñas del Hub usan Argon2id: [ADR 0004](../specs/adr/0004-argon2id-para-contrasenas.md). |
| **RBAC / grilla de roles** | Control de acceso por roles: a cada persona se le asigna un rol y cada rol tiene permisos. | Cada aplicación declara sus permisos (columnas) y el administrador decide qué permisos lleva cada rol (filas): [ADR 0013](../specs/adr/0013-roles-y-permisos-configurables-por-aplicacion.md) y [manual de usuario](manuales/usuario.md). |
| **Registro de auditoría** | La bitácora de quién hizo qué en el sistema, que no se puede modificar ni borrar. | Se consulta y se exporta (CSV y JSON) desde el Hub: [manual de usuario](manuales/usuario.md). |
| **Clave de servicio** | Una credencial para que un sistema (no una persona) consulte datos del Hub. | Alcance `audit-log:read`: [manual](manuales/claves-de-servicio.md) y [ADR 0014](../specs/adr/0014-claves-de-servicio-para-integraciones-maquina.md). |
| **SIEM** | Un sistema que reúne y analiza eventos de seguridad de muchas fuentes. | Es el caso de uso previsto de las claves de servicio: [manual](manuales/claves-de-servicio.md). |

## Seguridad en el pipeline

El *pipeline* es la cadena automática (en GitHub Actions) que comprueba cada cambio antes de aceptarlo. Se describe en el [manual de seguridad](manuales/seguridad.md) y en [`.github/`](../.github/workflows/README.md).

| Término | Qué significa | Dónde aparece en este proyecto |
|---|---|---|
| **SAST** | Análisis estático: una herramienta lee el código fuente buscando fallos de seguridad sin ejecutarlo. | CodeQL y Semgrep en CI: [manual de seguridad](manuales/seguridad.md). |
| **SCA** | Análisis de composición: busca vulnerabilidades conocidas en las dependencias que usa el proyecto. | govulncheck, npm audit y osv-scanner: [manual de seguridad](manuales/seguridad.md). |
| **DAST** | Análisis dinámico: ataca la aplicación ya en marcha, como lo haría un atacante externo. | OWASP ZAP (`make scan-dast`); informes en [`security/`](../security/README.md). |
| **CVE** | El identificador público de una vulnerabilidad conocida, por ejemplo `CVE-2026-85091`. | Los escáneres de imágenes (Trivy) bloquean según su severidad; ejemplos en la [bitácora](BITACORA.md). |
| **SBOM** | La «lista de ingredientes» de una imagen de contenedor: todo el software que contiene. | Cada imagen publicada en Docker Hub lleva su SBOM adjunto: [ADR 0011](../specs/adr/0011-docker-hub-como-registry.md). |
| **Cosign** | Herramienta que firma imágenes de contenedor y comprueba esas firmas, para saber que una imagen la publicó este proyecto y no fue alterada. | `release.yml` firma y verifica con Cosign antes de mover `latest`: [manual de despliegue](manuales/despliegue-y-operacion.md). |
| **Digest** | La huella (`sha256:...`) que identifica de forma única el contenido exacto de una imagen. | La producción simulada usa imágenes por digest: [`deploy/`](../deploy/README.md). |
| **Vulnerabilidad VULN-NNN** | Un hallazgo del proyecto con ficha propia, evidencia «antes» y «después» y remediación. | [`security/findings/`](../security/findings/) y [evidencia](evidencia/README.md). |

## Análisis de amenazas y decisiones

| Término | Qué significa | Dónde aparece en este proyecto |
|---|---|---|
| **STRIDE** | Un método para pensar en amenazas según seis categorías (suplantación, manipulación, repudio, divulgación, denegación de servicio y elevación de privilegios). | [Modelo de amenazas](../specs/05-security/threat-model.md). |
| **DFD** | Diagrama de flujo de datos: muestra qué componentes intercambian datos y dónde cambia la confianza. | Niveles 0 y 1 en [`docs/threat-model/`](threat-model/README.md), hechos con OWASP Threat Dragon. |
| **ADR** | Registro de una decisión de arquitectura: contexto, decisión y consecuencias. | [Índice de ADR](../specs/adr/README.md). |
| **Gherkin / escenario de aceptación** | Una forma de escribir el comportamiento esperado en frases (dado, cuando, entonces). | [`specs/06-acceptance/`](../specs/06-acceptance/README.md); cada escenario tiene una prueba E2E con el mismo nombre. |

## Infraestructura y operación

| Término | Qué significa | Dónde aparece en este proyecto |
|---|---|---|
| **IaC** (infraestructura como código) | Describir los servidores y redes en archivos versionados en lugar de crearlos a mano. | [`infraestructura/`](../infraestructura/README.md) (Terraform) y los Dockerfile, revisados con Checkov. |
| **Terraform** | La herramienta con la que se escribe esa infraestructura. | La referencia en AWS: [ADR 0012](../specs/adr/0012-arquitectura-de-referencia-en-aws-con-localstack.md). |
| **Docker Compose** | La forma de levantar varios contenedores (API, base de datos, correo...) con un solo archivo y un comando. | [`deploy/`](../deploy/README.md) y `make up`. |
| **Mailpit** | Una bandeja de correo falsa para desarrollo: recibe los correos que envía el sistema sin enviarlos a nadie. | Invitaciones y códigos MFA en <http://localhost:8025>. |
| **RabbitMQ / cola de mensajes** | Un intermediario donde la API deja tareas (como «enviar este correo») para que otro proceso, el worker, las haga después. | [Topología](../specs/04-events/topology.md). |
| **DLQ** (cola de mensajes fallidos) | La cola donde van a parar los mensajes que no se lograron procesar. | [Runbook de la cola de notificaciones](runbooks/dlq-notificaciones.md). |
| **Outbox** | Un patrón para que guardar un dato y publicar su evento no puedan quedar a medias. | **No está implementado**; es trabajo futuro deliberado: [ADR 0006](../specs/adr/0006-publicacion-directa-sin-outbox.md). |
| **Falco** | Un detector que vigila lo que hacen los contenedores en ejecución y avisa si ven algo anómalo. | Cuatro reglas propias y una alerta en Grafana: [runbook](runbooks/falco-alerta.md). |
| **Observabilidad** | Poder ver qué pasa dentro del sistema mediante métricas, logs y alertas. | Prometheus (métricas), Loki y Alloy (logs) y Grafana (paneles): [manual de despliegue](manuales/despliegue-y-operacion.md). |
